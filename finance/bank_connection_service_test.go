//go:build postgres_test

package finance

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	internalenablebanking "github.com/gemyago/sumweave/finance/internal/enablebanking"
	enablebankingclient "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	internalmonobank "github.com/gemyago/sumweave/finance/internal/monobank"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBankConnectionService(t *testing.T) {
	type testBankConnectionServiceArgs struct {
		tenantMembershipStore accessGuardStore
		pendingStartLookup    bankConnectionPendingStartLookup
		connectionStore       bankConnectionMetadataStore
		linkCoordinator       bankConnectionLinkCoordinator
		logger                *slog.Logger
	}

	makeTestBankConnectionService := func(args testBankConnectionServiceArgs) *BankConnectionService {
		logger := args.logger
		if logger == nil {
			logger = slog.New(slog.DiscardHandler)
		}
		return &BankConnectionService{
			access:             newAccessGuard(args.tenantMembershipStore),
			pendingStartLookup: args.pendingStartLookup,
			connectionStore:    args.connectionStore,
			linkCoordinator:    args.linkCoordinator,
			logger:             logger.With("component", "bankConnectionService"),
		}
	}

	t.Run("enforces tenant access before coordinator calls", func(t *testing.T) {
		fake := faker.New()
		coordinator := &recordingBankConnectionLinkCoordinator{}
		service := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: false},
			pendingStartLookup:    recordingPendingStartLookup{},
			linkCoordinator:       coordinator,
		})

		_, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderMonobank,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID:        "actor-" + fake.UUID().V4(),
			TenantID:           "tenant-" + fake.UUID().V4(),
			Provider:           bankProviderPKO,
			RedirectURL:        "https://app.example.test/" + fake.UUID().V4(),
			BrowserCallbackURL: "http://localhost/callback/" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			State:       "state-" + fake.UUID().V4(),
			Code:        "code-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		assert.Zero(t, coordinator.tokenCalls)
		assert.Zero(t, coordinator.startCalls)
		assert.Zero(t, coordinator.finishCalls)
	})

	t.Run("renames only a connection owned by the authorized tenant", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		renamedName := "Renamed " + fake.Company().Name()
		store := newMockbankConnectionMetadataStore(t)
		store.EXPECT().UpdateBankConnectionDisplayName(
			mock.Anything,
			tenantID,
			connectionID,
			renamedName,
			now,
		).Return(nil)
		service := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{},
			connectionStore:       store,
			linkCoordinator:       &recordingBankConnectionLinkCoordinator{},
		})
		service.now = func() time.Time { return now }

		err := service.UpdateBankConnection(t.Context(), UpdateBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().
				V4(),
			TenantID: tenantID, ConnectionID: connectionID,
			Name: "  " + renamedName + "  ",
		})
		require.NoError(t, err)

		foreignStore := newMockbankConnectionMetadataStore(t)
		foreignStore.EXPECT().
			UpdateBankConnectionDisplayName(mock.Anything, tenantID, connectionID, renamedName, mock.Anything).
			Return(persistence.ErrBankConnectionNotFound)
		foreignService := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{},
			connectionStore:       foreignStore,
			linkCoordinator:       &recordingBankConnectionLinkCoordinator{},
		})
		foreignService.now = func() time.Time { return now }
		err = foreignService.UpdateBankConnection(t.Context(), UpdateBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().
				V4(),
			TenantID: tenantID, ConnectionID: connectionID,
			Name: renamedName,
		})
		require.ErrorIs(t, err, ErrBankConnectionNotFound)

		emptyNameStore := newMockbankConnectionMetadataStore(t)
		emptyNameService := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{},
			connectionStore:       emptyNameStore,
			linkCoordinator:       &recordingBankConnectionLinkCoordinator{},
		})
		err = emptyNameService.UpdateBankConnection(t.Context(), UpdateBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    tenantID, ConnectionID: connectionID, Name: " \t\n ",
		})
		require.ErrorIs(t, err, ErrBankConnectionNameRequired)
	})

	t.Run("wraps coordinator link errors with public configuration errors", func(t *testing.T) {
		fake := faker.New()
		unsupportedProvider := domain.ProviderID("unsupported-" + fake.UUID().V4())
		coordinator := &recordingBankConnectionLinkCoordinator{
			tokenErr:          internalproviders.ErrTokenLinkUnsupported,
			startErr:          internalproviders.ErrRedirectLinkUnsupported,
			finishErrProvider: unsupportedProvider,
			finishErr:         internalproviders.ErrProviderNotConfigured,
		}
		service := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{},
			linkCoordinator:       coordinator,
		})

		_, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, internalproviders.ErrTokenLinkUnsupported)

		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderMonobank,
			RedirectURL: "https://app.example.test/" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, internalproviders.ErrRedirectLinkUnsupported)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    string(unsupportedProvider),
			State:       "state-" + fake.UUID().V4(),
			Code:        "code-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrBankProviderNotConfigured)

		coordinator.tokenErr = internalproviders.ErrProviderNotConfigured
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrBankProviderNotConfigured)

		assert.Equal(t, 2, coordinator.tokenCalls)
		assert.Equal(t, 1, coordinator.startCalls)
		assert.Equal(t, 1, coordinator.finishCalls)
	})

	t.Run("maps wrapped Enable Banking client failures to public response errors", func(t *testing.T) {
		fake := faker.New()
		providerMessage := "provider-message-" + fake.UUID().V4()
		providerBody := []byte(`{"message":"` + providerMessage + `"}`)
		coordinator := &recordingBankConnectionLinkCoordinator{
			startErr: fmt.Errorf(
				"start redirect link: %w",
				&enablebankingclient.ResponseError{
					Operation:  "auth",
					StatusCode: http.StatusBadRequest,
					Code:       "REDIRECT_URI_NOT_ALLOWED",
					Message:    providerMessage,
					Body:       providerBody,
				},
			),
			finishErr: fmt.Errorf(
				"finish redirect link: %w",
				&enablebankingclient.ResponseError{
					Operation:  "sessions",
					StatusCode: http.StatusBadRequest,
					Code:       "INVALID_AUTHORIZATION_CODE",
					Message:    providerMessage,
					Body:       providerBody,
				},
			),
		}
		service := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{},
			linkCoordinator:       coordinator,
		})

		_, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			RedirectURL: "https://app.example.test/" + fake.UUID().V4(),
		})

		var providerResponseErr *ProviderResponseError
		require.ErrorAs(t, err, &providerResponseErr)
		assert.Equal(t, &ProviderResponseError{
			Provider:   bankConnectorEnableBanking,
			Operation:  "auth",
			StatusCode: http.StatusBadRequest,
			Code:       "REDIRECT_URI_NOT_ALLOWED",
			Message:    providerMessage,
		}, providerResponseErr)
		assert.Contains(t, err.Error(), providerMessage)
		var upstreamResponseErr *enablebankingclient.ResponseError
		assert.NotErrorAs(t, err, &upstreamResponseErr)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			State:       "state-" + fake.UUID().V4(),
			Code:        "code-" + fake.UUID().V4(),
		})
		require.ErrorAs(t, err, &providerResponseErr)
		assert.Equal(t, &ProviderResponseError{
			Provider:   bankConnectorEnableBanking,
			Operation:  "sessions",
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_AUTHORIZATION_CODE",
			Message:    providerMessage,
		}, providerResponseErr)
		assert.Contains(t, err.Error(), providerMessage)
		assert.NotErrorAs(t, err, &upstreamResponseErr)
	})

	t.Run(
		"delegates monobank token link and pko redirect flows through v2 coordination",
		func(t *testing.T) {
			fake := faker.New()
			now := time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC)
			startResult := internalproviders.StartLinkResult{
				State:            "state-" + fake.UUID().V4(),
				AuthorizationURL: "https://enable-banking.example.test/auth/" + fake.UUID().V4(),
			}
			tokenConnection := domain.BankConnection{
				ID:                "connection-mono-" + fake.UUID().V4(),
				TenantID:          "tenant-" + fake.UUID().V4(),
				Provider:          bankProviderMonobank,
				ConnectorID:       domain.ProviderConnectorIDMonobank,
				DisplayName:       "Monobank " + fake.Company().Name(),
				ProviderReference: "mono-ref-" + fake.UUID().V4(),
				SecretID:          "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			finishConnection := domain.BankConnection{
				ID:                "connection-pko-" + fake.UUID().V4(),
				TenantID:          "tenant-" + fake.UUID().V4(),
				Provider:          bankProviderPKO,
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "pko-ref-" + fake.UUID().V4(),
				SecretID:          "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			coordinator := &recordingBankConnectionLinkCoordinator{
				startResult:  startResult,
				tokenResult:  tokenConnection,
				finishResult: finishConnection,
			}
			service := makeTestBankConnectionService(testBankConnectionServiceArgs{
				tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
				pendingStartLookup:    recordingPendingStartLookup{},
				linkCoordinator:       coordinator,
			})

			tokenParams := LinkTokenBankConnectionParams{
				ActorUserID: "actor-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				Provider:    bankProviderMonobank,
				Token:       "token-" + fake.UUID().V4(),
			}
			tokenLinked, err := service.LinkTokenBankConnection(t.Context(), tokenParams)
			require.NoError(t, err)
			assert.Equal(t, tokenConnection, tokenLinked)
			assert.Equal(t, internalproviders.TokenLinkRequest{
				TenantID:    tokenParams.TenantID,
				ActorUserID: tokenParams.ActorUserID,
				ProviderID:  domain.ProviderIDMonobank,
				Token:       tokenParams.Token,
			}, coordinator.lastToken)

			startParams := StartBankConnectionLinkParams{
				ActorUserID: "actor-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				Provider:    bankProviderPKO,
				RedirectURL: "https://backend.example.test/callback/" + fake.UUID().V4(),
				BrowserCallbackURL: "http://localhost:5173/#/finance/connections/" + fake.UUID().
					V4(),
			}
			started, err := service.StartBankConnectionLink(t.Context(), startParams)
			require.NoError(t, err)
			assert.Equal(t, ProviderLinkStart{
				State:            startResult.State,
				AuthorizationURL: startResult.AuthorizationURL,
			}, started)
			assert.Equal(t, internalproviders.RedirectLinkStartRequest{
				TenantID:           startParams.TenantID,
				ActorUserID:        startParams.ActorUserID,
				ProviderID:         domain.ProviderIDPKO,
				RedirectURL:        startParams.RedirectURL,
				BrowserCallbackURL: startParams.BrowserCallbackURL,
			}, coordinator.lastStart)

			finishParams := FinishBankConnectionLinkParams{
				ActorUserID: "actor-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				Provider:    bankProviderPKO,
				State:       "state-" + fake.UUID().V4(),
				Code:        "code-" + fake.UUID().V4(),
				Start: ProviderLinkStart{
					State:             "tampered-state-" + fake.UUID().V4(),
					AuthorizationURL:  "https://evil.example.test/" + fake.UUID().V4(),
					ProviderReference: "tampered-ref-" + fake.UUID().V4(),
				},
			}
			finished, err := service.FinishBankConnectionLink(t.Context(), finishParams)
			require.NoError(t, err)
			assert.Equal(t, finishConnection, finished)
			assert.Equal(t, internalproviders.RedirectLinkFinishRequest{
				TenantID:    finishParams.TenantID,
				ActorUserID: finishParams.ActorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       finishParams.State,
				Code:        finishParams.Code,
			}, coordinator.lastFinish)
		},
	)

	t.Run(
		"composes real v2 connectors persistence secret writing and pending lookup",
		func(t *testing.T) {
			fake := faker.New()
			now := time.Date(2026, time.June, 30, 13, 0, 0, 0, time.UTC)
			database := openTestDatabase(t)
			store := persistence.NewStore(database)
			tenantID, actorUserID := seedTenantMembership(t, store, now)
			key := sha256.Sum256([]byte("key-" + fake.UUID().V4()))
			cipher, err := credentials.NewAESGCMCipher(key[:], "finance-bank-connections")
			require.NoError(t, err)
			monoSecretID := "mono-secret-" + fake.UUID().V4()
			monoConnectionID := "mono-connection-" + fake.UUID().V4()
			monoRawID := "mono-raw-" + fake.UUID().V4()
			pendingStartID := "pending-start-" + fake.UUID().V4()
			pkoSecretID := "pko-secret-" + fake.UUID().V4()
			pkoConnectionID := "pko-connection-" + fake.UUID().V4()
			pkoRawID := "pko-raw-" + fake.UUID().V4()
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
			require.NoError(t, err)
			privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
			privateKeyPEM := pem.EncodeToMemory(
				&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER},
			)
			require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))

			monobankServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", r.Method)
					}
					if r.URL.Path != "/personal/client-info" {
						t.Fatalf("expected /personal/client-info, got %s", r.URL.Path)
					}
					if r.Header.Get("X-Token") != "mono-token" {
						t.Fatalf("expected mono-token header, got %s", r.Header.Get("X-Token"))
					}
					writeJSON(t, w, map[string]any{
						"name": "Mono " + fake.Company().Name(),
						"accounts": []map[string]any{{
							"id":              "mono-account-" + fake.UUID().V4(),
							"currencyCode":    980,
							"cashbackType":    "None",
							"balance":         123,
							"creditLimit":     0,
							"maskedPan":       []string{"4444"},
							"type":            "black",
							"iban":            "UA" + fake.UUID().V4(),
							"sendId":          "send-" + fake.UUID().V4(),
							"currencyCodeA":   980,
							"currencyCodeB":   980,
							"isForCashback":   false,
							"maskedPanLength": 1,
						}},
					})
				}),
			)
			defer monobankServer.Close()

			enableBankingServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if got := r.Header.Get("Authorization"); got == "" {
						t.Fatalf("expected signed enable banking request")
					}
					switch r.URL.Path {
					case "/auth":
						if r.Method != http.MethodPost {
							t.Fatalf("expected POST auth, got %s", r.Method)
						}
						writeJSON(t, w, map[string]any{
							"url": "https://enable-banking.example.test/auth/" + fake.UUID().
								V4(),
							"authorization_id": "auth-ref-" + fake.UUID().V4(),
						})
					case "/sessions":
						if r.Method != http.MethodPost {
							t.Fatalf("expected POST sessions, got %s", r.Method)
						}
						writeJSON(t, w, map[string]any{
							"session_id": "session-" + fake.UUID().V4(),
							"status":     string(domain.BankConnectionStateActive),
							"aspsp": map[string]any{
								"name": fake.Company().Name(),
							},
						})
					default:
						t.Fatalf("unexpected enable banking path: %s", r.URL.Path)
					}
				}),
			)
			defer enableBankingServer.Close()

			service, err := newBankConnectionService(bankConnectionServiceArgs{
				Store:                  store,
				Logger:                 slog.New(slog.DiscardHandler),
				ConnectionSecretCipher: cipher,
				ConnectorRegistry: internalproviders.NewStaticConnectorRegistry(
					internalmonobank.NewConnector(
						internalmonobank.Args{BaseURL: monobankServer.URL},
					),
					internalenablebanking.NewConnector(
						internalenablebanking.Args{
							BaseURL:        enableBankingServer.URL,
							Logger:         slog.New(slog.DiscardHandler),
							AppID:          "app-" + fake.UUID().V4(),
							PrivateKeyPath: privateKeyPath,
							StateProvider:  func() (string, error) { return "pko-state", nil },
							Now:            func() time.Time { return now },
						},
					),
				),
				ProviderProfileRegistry: internalproviders.NewStaticProviderProfileRegistry(
					internalmonobank.Profile(),
					internalproviders.PKOProfile(),
				),
				Now: func() time.Time { return now },
				NewID: newSequentialIDGenerator(
					monoSecretID,
					monoConnectionID,
					monoRawID,
					pendingStartID,
					pkoSecretID,
					pkoConnectionID,
					pkoRawID,
				),
			})
			require.NoError(t, err)

			monobankConnection, err := service.LinkTokenBankConnection(
				t.Context(),
				LinkTokenBankConnectionParams{
					ActorUserID: actorUserID,
					TenantID:    tenantID,
					Provider:    bankProviderMonobank,
					Token:       "mono-token",
				},
			)
			require.NoError(t, err)
			assert.Equal(t, monoConnectionID, monobankConnection.ID)
			assert.Equal(t, domain.ProviderConnectorIDMonobank, monobankConnection.ConnectorID)
			assert.True(t, now.Equal(monobankConnection.CreatedAt))
			monobankSecret, err := store.GetConnectionSecret(
				t.Context(),
				monobankConnection.SecretID,
			)
			require.NoError(t, err)
			decryptedMonobankToken, err := cipher.OpenString(monobankSecret.Envelope)
			require.NoError(t, err)
			assert.Equal(t, "mono-token", decryptedMonobankToken)

			startResult, err := service.StartBankConnectionLink(
				t.Context(),
				StartBankConnectionLinkParams{
					ActorUserID:        actorUserID,
					TenantID:           tenantID,
					Provider:           bankProviderPKO,
					RedirectURL:        "https://backend.example.test/callback",
					BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
				},
			)
			require.NoError(t, err)
			assert.Equal(t, "pko-state", startResult.State)

			pendingStart, err := service.GetPendingBankConnectionLinkStartByState(
				t.Context(),
				GetPendingBankConnectionLinkStartByStateParams{
					Provider: bankProviderPKO,
					State:    "pko-state",
				},
			)
			require.NoError(t, err)
			assert.NotEmpty(t, pendingStart.ID)
			assert.Equal(t, domain.ProviderConnectorIDEnableBanking, pendingStart.ConnectorID)
			assert.True(t, now.Equal(pendingStart.CreatedAt))
			assert.Equal(t, "http://localhost:5173/#/finance/connections", pendingStart.CallbackURL)

			pkoConnection, err := service.FinishBankConnectionLink(
				t.Context(),
				FinishBankConnectionLinkParams{
					ActorUserID: actorUserID,
					TenantID:    tenantID,
					Provider:    bankProviderPKO,
					State:       "pko-state",
					Code:        "finish-code",
				},
			)
			require.NoError(t, err)
			assert.Equal(t, pkoConnectionID, pkoConnection.ID)
			assert.Equal(t, domain.ProviderConnectorIDEnableBanking, pkoConnection.ConnectorID)
			pkoSecret, err := store.GetConnectionSecret(t.Context(), pkoConnection.SecretID)
			require.NoError(t, err)
			decryptedPKOSecret, err := cipher.OpenString(pkoSecret.Envelope)
			require.NoError(t, err)
			assert.Empty(t, decryptedPKOSecret)
		},
	)

	t.Run("preserves persisted PKO sessions for distinct links and retries", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.August, 10, 15, 0, 0, 0, time.FixedZone("test", 2*60*60))
		store := persistence.NewStore(openTestDatabase(t))
		key := sha256.Sum256([]byte("key-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(key[:], "finance-bank-connections")
		require.NoError(t, err)
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
		require.NoError(t, os.WriteFile(
			privateKeyPath,
			pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}),
			0o600,
		))

		reference := "reference-" + fake.UUID().V4()
		externalID := "session-" + fake.UUID().V4()
		secondProviderReference := "session-second-" + fake.UUID().V4()
		thirdProviderReference := "session-third-" + fake.UUID().V4()
		retryReference := "reference-retry-" + fake.UUID().V4()
		retryProviderReference := "session-retry-" + fake.UUID().V4()
		sessionByCode := map[string]map[string]any{
			"first": {
				"session_id":         externalID,
				"provider_reference": reference,
				"status":             string(domain.BankConnectionStateActive),
			},
			"second": {
				"session_id":         secondProviderReference,
				"provider_reference": secondProviderReference,
				"status":             string(domain.BankConnectionStateActive),
			},
			"third": {
				"session_id":         thirdProviderReference,
				"provider_reference": thirdProviderReference,
				"status":             string(domain.BankConnectionStateActive),
			},
			"retry": {
				"session_id":         retryProviderReference,
				"provider_reference": retryReference,
				"status":             string(domain.BankConnectionStateActive),
			},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/auth":
				writeJSON(t, w, map[string]any{
					"url":              "https://enable-banking.example.test/auth/" + fake.UUID().V4(),
					"authorization_id": "auth-" + fake.UUID().V4(),
				})
			case "/sessions":
				var request struct {
					Code string `json:"code"`
				}
				if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
					t.Errorf("decode session request: %v", decodeErr)
					return
				}
				response, ok := sessionByCode[request.Code]
				if !ok {
					t.Errorf("unexpected session code %q", request.Code)
					return
				}
				writeJSON(t, w, response)
			default:
				t.Fatalf("unexpected enable banking path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		states := []string{
			"state-first-" + fake.UUID().V4(),
			"state-second-" + fake.UUID().V4(),
			"state-third-" + fake.UUID().V4(),
			"state-retry-" + fake.UUID().V4(),
		}
		stateIndex := 0
		connector := internalenablebanking.NewConnector(internalenablebanking.Args{
			BaseURL:        server.URL,
			Logger:         slog.New(slog.DiscardHandler),
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
			StateProvider: func() (string, error) {
				state := states[stateIndex]
				stateIndex++
				return state, nil
			},
			Now: func() time.Time { return now },
		})
		linkPersistence := persistence.NewProviderLinkPersistence(store)
		coordinator, err := internalproviders.NewLinkCoordinator(internalproviders.LinkCoordinatorArgs{
			ProviderProfileRegistry: internalproviders.NewStaticProviderProfileRegistry(internalproviders.PKOProfile()),
			ConnectorRegistry:       internalproviders.NewStaticConnectorRegistry(connector),
			PendingStartStore:       linkPersistence,
			ConnectionSecretWriter: newBankConnectionSecretWriter(
				store,
				cipher,
				func() time.Time { return now },
				uuid.NewString,
			),
			ConnectionStore: linkPersistence,
			Logger:          slog.New(slog.DiscardHandler),
			Now:             func() time.Time { return now },
			NewID:           uuid.NewString,
			PendingStartTTL: time.Minute,
		})
		require.NoError(t, err)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		start := func() internalproviders.StartLinkResult {
			result, startErr := coordinator.StartRedirectLink(t.Context(), internalproviders.RedirectLinkStartRequest{
				TenantID:           tenantID,
				ActorUserID:        actorUserID,
				ProviderID:         domain.ProviderIDPKO,
				RedirectURL:        "https://backend.example.test/callback",
				BrowserCallbackURL: "https://app.example.test/connections",
			})
			require.NoError(t, startErr)
			return result
		}
		finish := func(state string, code string) (domain.BankConnection, error) {
			return coordinator.FinishRedirectLink(t.Context(), internalproviders.RedirectLinkFinishRequest{
				TenantID:    tenantID,
				ActorUserID: actorUserID,
				ProviderID:  domain.ProviderIDPKO,
				State:       state,
				Code:        code,
			})
		}

		firstStart := start()
		first, err := finish(firstStart.State, "first")
		require.NoError(t, err)
		firstStored, err := store.GetBankConnection(t.Context(), first.ID)
		require.NoError(t, err)

		secondStart := start()
		second, err := finish(secondStart.State, "second")
		require.NoError(t, err)
		thirdStart := start()
		third, err := finish(thirdStart.State, "third")
		require.NoError(t, err)
		assert.NotEqual(t, first.ID, second.ID)
		assert.NotEqual(t, first.ID, third.ID)
		assert.NotEqual(t, first.SecretID, second.SecretID)
		assert.NotEqual(t, first.SecretID, third.SecretID)
		loadedFirst, err := store.GetBankConnection(t.Context(), first.ID)
		require.NoError(t, err)
		assert.Equal(t, firstStored, loadedFirst)

		retryStart := start()
		retry, err := finish(retryStart.State, "retry")
		require.NoError(t, err)
		assert.Equal(t, retryProviderReference, retry.ProviderReference)
	})

	t.Run("wraps constructor and service errors", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		key := sha256.Sum256([]byte("key-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(key[:], "finance-bank-connections")
		require.NoError(t, err)

		_, err = newBankConnectionService(bankConnectionServiceArgs{
			Store:                  store,
			Logger:                 slog.New(slog.DiscardHandler),
			ConnectionSecretCipher: cipher,
			ProviderProfileRegistry: internalproviders.NewStaticProviderProfileRegistry(
				internalproviders.PKOProfile(),
			),
			Now:   time.Now,
			NewID: uuid.NewString,
		})
		require.ErrorIs(t, err, internalproviders.ErrConnectorRegistryRequired)

		membershipErrService := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{
				err: errors.New("membership failed"),
			},
			pendingStartLookup: recordingPendingStartLookup{},
			linkCoordinator:    &recordingBankConnectionLinkCoordinator{},
		})
		_, err = membershipErrService.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: "actor-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				Provider:    bankProviderMonobank,
				Token:       "token-" + fake.UUID().V4(),
			},
		)
		require.ErrorContains(t, err, "check tenant membership")

		service := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{err: errors.New("lookup failed")},
			linkCoordinator: &recordingBankConnectionLinkCoordinator{
				startErr:  errors.New("start failed"),
				tokenErr:  errors.New("token failed"),
				finishErr: internalproviders.ErrPendingStartNotFound,
			},
		})

		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderMonobank,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorContains(t, err, "link token bank connection")

		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			RedirectURL: "https://app.example.test/" + fake.UUID().V4(),
		})
		require.ErrorContains(t, err, "start bank connection link")

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    bankProviderPKO,
			State:       "state-" + fake.UUID().V4(),
			Code:        "code-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)

		_, err = service.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			GetPendingBankConnectionLinkStartByStateParams{
				Provider: bankProviderPKO,
				State:    "state-" + fake.UUID().V4(),
			},
		)
		require.ErrorContains(t, err, "get pending bank connection link start by state")

		notFoundLookupService := makeTestBankConnectionService(testBankConnectionServiceArgs{
			tenantMembershipStore: recordingTenantMembershipStore{allowed: true},
			pendingStartLookup:    recordingPendingStartLookup{},
			linkCoordinator:       &recordingBankConnectionLinkCoordinator{},
		})

		_, err = notFoundLookupService.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			GetPendingBankConnectionLinkStartByStateParams{
				Provider: "unsupported-" + fake.UUID().V4(),
				State:    "state-" + fake.UUID().V4(),
			},
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
	})

	t.Run("prepares and persists encrypted connection secrets", func(t *testing.T) {
		fake := faker.New()
		store := persistence.NewStore(openTestDatabase(t))
		key := sha256.Sum256([]byte("connection-secret-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(key[:], "connection-secret")
		require.NoError(t, err)
		now := time.Now()
		writer := newBankConnectionSecretWriter(
			store,
			cipher,
			func() time.Time { return now },
			func() string { return "secret-" + fake.UUID().V4() },
		)
		prepared, err := writer.PrepareConnectionSecret(
			bankProviderMonobank,
			"reference-"+fake.UUID().V4(),
			"plaintext-"+fake.UUID().V4(),
		)
		require.NoError(t, err)
		assert.True(t, now.Equal(prepared.CreatedAt))
		secretID, err := writer.SaveConnectionSecret(
			t.Context(),
			prepared.Provider,
			prepared.Reference,
			"plaintext-"+fake.UUID().V4(),
		)
		require.NoError(t, err)
		persisted, err := store.GetConnectionSecret(t.Context(), secretID)
		require.NoError(t, err)
		assert.NotEmpty(t, persisted.Envelope.Ciphertext)
	})
}

type recordingTenantMembershipStore struct {
	allowed bool
	err     error
}

func (s recordingTenantMembershipStore) IsTenantMember(
	context.Context,
	string,
	string,
) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowed, nil
}

type recordingPendingStartLookup struct {
	start *domain.PendingBankConnectionLinkStart
	err   error
}

func (l recordingPendingStartLookup) GetPendingStartByState(
	context.Context,
	domain.ProviderID,
	string,
) (*domain.PendingBankConnectionLinkStart, error) {
	if l.err != nil {
		return nil, l.err
	}
	if l.start == nil {
		return nil, persistence.ErrPendingBankConnectionLinkStartNotFound
	}
	return l.start, nil
}

type recordingBankConnectionLinkCoordinator struct {
	startResult       internalproviders.StartLinkResult
	startErr          error
	tokenResult       domain.BankConnection
	tokenErr          error
	finishResult      domain.BankConnection
	finishErr         error
	finishErrProvider domain.ProviderID
	startCalls        int
	tokenCalls        int
	finishCalls       int
	lastStart         internalproviders.RedirectLinkStartRequest
	lastToken         internalproviders.TokenLinkRequest
	lastFinish        internalproviders.RedirectLinkFinishRequest
}

func (c *recordingBankConnectionLinkCoordinator) StartRedirectLink(
	_ context.Context,
	request internalproviders.RedirectLinkStartRequest,
) (internalproviders.StartLinkResult, error) {
	c.startCalls++
	c.lastStart = request
	if c.startErr != nil {
		return internalproviders.StartLinkResult{}, c.startErr
	}
	return c.startResult, nil
}

func (c *recordingBankConnectionLinkCoordinator) LinkToken(
	_ context.Context,
	request internalproviders.TokenLinkRequest,
) (domain.BankConnection, error) {
	c.tokenCalls++
	c.lastToken = request
	if c.tokenErr != nil {
		return domain.BankConnection{}, c.tokenErr
	}
	return c.tokenResult, nil
}

func (c *recordingBankConnectionLinkCoordinator) FinishRedirectLink(
	_ context.Context,
	request internalproviders.RedirectLinkFinishRequest,
) (domain.BankConnection, error) {
	c.finishCalls++
	c.lastFinish = request
	if c.finishErrProvider != "" && request.ProviderID == c.finishErrProvider {
		return domain.BankConnection{}, c.finishErr
	}
	if c.finishErr != nil {
		return domain.BankConnection{}, c.finishErr
	}
	return c.finishResult, nil
}

func seedTenantMembership(
	t *testing.T,
	store *persistence.Store,
	now time.Time,
) (string, string) {
	t.Helper()
	fake := faker.New()
	tenantID := "tenant-" + fake.UUID().V4()
	actorUserID := "actor-" + fake.UUID().V4()
	_, err := store.SaveTenant(t.Context(), domain.Tenant{
		ID:              tenantID,
		Name:            "Tenant " + fake.Company().Name(),
		DisplayCurrency: "USD",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	require.NoError(t, err)
	_, err = store.SaveTenantMembership(t.Context(), domain.TenantMembership{
		TenantID:  tenantID,
		UserID:    actorUserID,
		JoinedAt:  now,
		CreatedAt: now,
	})
	require.NoError(t, err)
	return tenantID, actorUserID
}

func writeJSON(t *testing.T, w http.ResponseWriter, body map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode JSON response: %v", err)
	}
}

func newSequentialIDGenerator(ids ...string) func() string {
	index := 0
	return func() string {
		if index >= len(ids) {
			return ids[len(ids)-1]
		}
		value := ids[index]
		index++
		return value
	}
}

var _ interface {
	StartRedirectLink(context.Context, internalproviders.RedirectLinkStartRequest) (internalproviders.StartLinkResult, error)
	FinishRedirectLink(context.Context, internalproviders.RedirectLinkFinishRequest) (domain.BankConnection, error)
	LinkToken(context.Context, internalproviders.TokenLinkRequest) (domain.BankConnection, error)
} = (*recordingBankConnectionLinkCoordinator)(nil)
