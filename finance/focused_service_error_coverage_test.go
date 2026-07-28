package finance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestFocusedServiceErrorCoverage(t *testing.T) {
	makeStore := func(t *testing.T) *persistence.Store {
		t.Helper()
		return persistence.NewStore(openTestDatabase(t))
	}
	makeCipher := func(t *testing.T) *credentials.AESGCMCipher {
		t.Helper()
		cipher, err := credentials.NewAESGCMCipher([]byte("0123456789abcdef0123456789abcdef"), "test-key")
		require.NoError(t, err)
		return cipher
	}
	makeOtherCipher := func(t *testing.T) *credentials.AESGCMCipher {
		t.Helper()
		cipher, err := credentials.NewAESGCMCipher([]byte("abcdef0123456789abcdef0123456789"), "other-test-key")
		require.NoError(t, err)
		return cipher
	}
	makeTenant := func(t *testing.T, service *TenantService, userID string) domain.Tenant {
		t.Helper()
		fake := faker.New()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     userID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		return tenant
	}

	t.Run("root service delegates keep bank sync missing-store errors explicit", func(t *testing.T) {
		service := NewService(stubStore{})
		_, err := service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{})
		require.ErrorContains(t, err, "bank sync store is required")
		_, err = service.PauseBankConnectionSchedule(t.Context(), PauseBankConnectionScheduleParams{})
		require.ErrorContains(t, err, "bank sync store is required")
		_, err = service.ResumeBankConnectionSchedule(t.Context(), ResumeBankConnectionScheduleParams{})
		require.ErrorContains(t, err, "bank sync store is required")
		_, err = service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{})
		require.ErrorContains(t, err, "bank sync store is required")
		err = service.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{})
		require.ErrorContains(t, err, "bank sync store is required")
		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{})
		require.ErrorContains(t, err, "bank sync store is required")
		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{})
		require.ErrorContains(t, err, "bank sync store is required")
	})

	t.Run("root service still supports legacy token link path", func(t *testing.T) {
		store := makeStore(t)
		cipher := makeCipher(t)
		provider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				ExternalID:        "external",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		}
		service := NewService(store, WithConnectionSecretCipher(cipher), WithBankProviders(provider))
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant := makeTenant(t, NewTenantService(store), ownerUserID)
		_, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    provider.name,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
	})

	t.Run("csv import service covers focused error branches", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		catalogService := NewCatalogService(store)
		ledgerService := NewLedgerService(store)
		service := NewCSVImportService(store, catalogService, ledgerService)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		outsiderUserID := "outsider-" + fake.UUID().V4()
		tenant := makeTenant(t, tenantService, ownerUserID)

		_, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			CSV:         "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n",
		})
		require.Error(t, err)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			CSV:         "name,currency,kind\nwallet,USD,manual\n",
		})
		require.NoError(t, err)
		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: outsiderUserID,
			ImportID:    preview.ImportID,
		})
		require.Error(t, err)
		_, err = service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: ownerUserID,
			TenantID:    "other-tenant",
			ImportID:    preview.ImportID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = service.ListRecentCSVImportAudits(t.Context(), ListRecentCSVImportAuditsParams{
			ActorUserID:        outsiderUserID,
			TenantID:           tenant.ID,
			ExpectedImportType: CSVImportTypeTransactions,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = service.ListRecentCSVImportAudits(t.Context(), ListRecentCSVImportAuditsParams{
			ActorUserID:        ownerUserID,
			TenantID:           tenant.ID,
			ExpectedImportType: CSVImportTypeTransactions,
		})
		require.ErrorContains(t, err, "csv import row store is required")
	})

	t.Run("csv import focused service propagates preview confirm and run persistence failures", func(t *testing.T) {
		previewErr := errors.New("list accounts failed")
		previewService := NewCSVImportService(
			stubStore{
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
				listAccountsFn: func(context.Context, string, bool) ([]domain.Account, error) {
					return nil, previewErr
				},
			},
			&CatalogService{},
			&LedgerService{},
		)
		_, err := previewService.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			ImportType:  CSVImportTypeTransactions,
			CSV:         "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n",
		})
		require.ErrorIs(t, err, previewErr)

		previewSaveErr := errors.New("save preview failed")
		previewSaveService := NewCSVImportService(
			stubStore{
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
				saveCSVImportFn: func(context.Context, domain.CSVImportRecord) (domain.CSVImportRecord, error) {
					return domain.CSVImportRecord{}, previewSaveErr
				},
			},
			&CatalogService{},
			&LedgerService{},
		)
		_, err = previewSaveService.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			ImportType:  CSVImportTypeTransactions,
			CSV:         "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n",
		})
		require.ErrorIs(t, err, previewSaveErr)

		previewTagsErr := errors.New("list tags failed")
		previewTagsService := NewCSVImportService(
			stubStore{
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
				listTagsFn: func(context.Context, string, bool) ([]domain.Tag, error) {
					return nil, previewTagsErr
				},
			},
			&CatalogService{},
			&LedgerService{},
		)
		_, err = previewTagsService.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			ImportType:  CSVImportTypeTransactions,
			CSV:         "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n",
		})
		require.ErrorIs(t, err, previewTagsErr)

		previewCategoriesErr := errors.New("list categories failed")
		previewCategoriesService := NewCSVImportService(
			stubStore{
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
				listCategoriesFn: func(context.Context, string, bool) ([]domain.Category, error) {
					return nil, previewCategoriesErr
				},
			},
			&CatalogService{},
			&LedgerService{},
		)
		_, err = previewCategoriesService.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			ImportType:  CSVImportTypeTransactions,
			CSV:         "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n",
		})
		require.ErrorIs(t, err, previewCategoriesErr)

		previewTransactionsErr := errors.New("list transactions failed")
		previewTransactionsService := NewCSVImportService(
			stubStore{
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
				listTransactionsFn: func(context.Context, string, string, domain.TransactionSource, domain.TransactionStatus, bool) ([]domain.Transaction, error) {
					return nil, previewTransactionsErr
				},
			},
			&CatalogService{},
			&LedgerService{},
		)
		_, err = previewTransactionsService.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			ImportType:  CSVImportTypeTransactions,
			CSV:         "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n",
		})
		require.ErrorIs(t, err, previewTransactionsErr)

		_, err = NewCSVImportService(stubStore{}, &CatalogService{}, &LedgerService{}).ConfirmCSVImport(
			t.Context(),
			ConfirmCSVImportParams{ActorUserID: "actor-1", ImportID: "missing-import"},
		)
		require.Error(t, err)

		auditErr := errors.New("load csv import audit failed")
		auditService := NewCSVImportService(
			stubStore{getCSVImportFn: func(context.Context, string) (*domain.CSVImportRecord, error) {
				return nil, auditErr
			}},
			&CatalogService{},
			&LedgerService{},
		)
		_, err = auditService.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{ImportID: "missing-import"})
		require.ErrorIs(t, err, auditErr)

		invalidStatusService := NewCSVImportService(
			stubStore{
				getCSVImportFn: func(context.Context, string) (*domain.CSVImportRecord, error) {
					return &domain.CSVImportRecord{
						ID:       "import-invalid-status",
						TenantID: "tenant-1",
						Type:     domain.CSVImportTypeAccounts,
						Status:   "mystery-status",
					}, nil
				},
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			},
			&CatalogService{},
			&LedgerService{},
			WithCSVImportServiceJobEnqueuer(&recordingCSVJobEnqueuer{
				jobID:   "job-1",
				jobType: CSVImportJobTypeAccounts,
			}),
		)
		_, err = invalidStatusService.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: "actor-1",
			ImportID:    "import-invalid-status",
		})
		require.ErrorContains(t, err, "csv import is not confirmable")

		saveErr := errors.New("save csv import failed")
		saveService := NewCSVImportService(
			stubStore{
				getCSVImportFn: func(context.Context, string) (*domain.CSVImportRecord, error) {
					return &domain.CSVImportRecord{
						ID:       "import-save-failure",
						TenantID: "tenant-1",
						Type:     domain.CSVImportTypeAccounts,
						Status:   domain.CSVImportStatusPreviewed,
						Headers:  []string{"name", "currency", "kind"},
						Mapping: map[string]string{
							csvImportFieldName:     "name",
							csvImportFieldCurrency: "currency",
							csvImportFieldKind:     "kind",
						},
					}, nil
				},
				saveCSVImportFn: func(context.Context, domain.CSVImportRecord) (domain.CSVImportRecord, error) {
					return domain.CSVImportRecord{}, saveErr
				},
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			},
			&CatalogService{},
			&LedgerService{},
			WithCSVImportServiceJobEnqueuer(&recordingCSVJobEnqueuer{
				jobID:   "job-2",
				jobType: CSVImportJobTypeAccounts,
			}),
		)
		_, err = saveService.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: "actor-1",
			ImportID:    "import-save-failure",
		})
		require.ErrorIs(t, err, saveErr)

		runService := NewCSVImportService(
			stubStore{
				getCSVImportFn: func(context.Context, string) (*domain.CSVImportRecord, error) {
					return &domain.CSVImportRecord{
						ID:       "import-bad-csv",
						TenantID: "tenant-1",
						Type:     domain.CSVImportTypeAccounts,
						RawCSV:   "\"bad csv",
					}, nil
				},
			},
			&CatalogService{},
			&LedgerService{},
		)
		_, err = runService.RunCSVImportJob(t.Context(), RunCSVImportJobParams{ImportID: "import-bad-csv"})
		require.Error(t, err)
	})

	t.Run("bank sync service direct helpers cover focused error branches", func(t *testing.T) {
		store := makeStore(t)
		cipher := makeCipher(t)
		provider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				ExternalID:        "external",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
			syncResults: []ProviderSyncResult{{SyncKey: "sync-1"}},
		}
		failingStore := &failingProviderSyncStore{Store: store}
		service := NewBankSyncService(
			failingStore,
			WithBankSyncServiceConnectionSecretCipher(cipher),
			WithBankSyncServiceProviders(provider),
		)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant := makeTenant(t, NewTenantService(store), ownerUserID)

		sealed, err := cipher.SealString("secret")
		require.NoError(t, err)
		_, err = store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-1",
			Provider:  provider.name,
			Reference: "ref",
			Envelope:  sealed,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-1",
			TenantID:          tenant.ID,
			Provider:          provider.name,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			DisplayName:       "display",
			ProviderReference: "ref",
			ExternalID:        "external",
			SecretID:          "secret-1",
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)

		_, err = service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: connection.ID,
			Reason:       BankConnectionSyncReasonManual,
		})
		require.ErrorContains(t, err, "bank sync job enqueuer is required")

		failingStore.getScheduleErr = errors.New("schedule read failed")
		_, _, err = service.makeScheduledRunMetadata(
			t.Context(),
			connection,
			RunBankConnectionSyncParams{Reason: BankConnectionSyncReasonScheduled},
			time.Now().UTC(),
		)
		require.Error(t, err)
		failingStore.getScheduleErr = nil

		failingStore.saveBankConnectionErr = errors.New("save bank connection failed")
		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: connection.ID,
			Result:       ProviderSyncResult{},
		})
		require.Error(t, err)
	})

	t.Run("bank sync focused service covers decrypt sync-run and schedule failure branches", func(t *testing.T) {
		store := makeStore(t)
		cipher := makeCipher(t)
		otherCipher := makeOtherCipher(t)
		now := time.Now().UTC()

		serviceWithoutCipher := NewBankSyncService(store)
		_, err := serviceWithoutCipher.decryptConnectionSecret(t.Context(), "secret-1")
		require.ErrorContains(t, err, "connection secret cipher is required")

		failingStore := &failingProviderSyncStore{
			Store:                  store,
			getConnectionSecretErr: errors.New("get secret failed"),
			getSyncRunErr:          errors.New("get sync run failed"),
			saveSyncRunErr:         errors.New("claim sync run failed"),
		}
		service := NewBankSyncService(failingStore, WithBankSyncServiceConnectionSecretCipher(cipher))
		_, err = service.decryptConnectionSecret(t.Context(), "secret-1")
		require.ErrorContains(t, err, "get connection secret")

		secretEnvelope, err := cipher.SealString("secret-value")
		require.NoError(t, err)
		_, err = store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-open-failure",
			Provider:  bankProviderMonobank,
			Reference: "ref-open-failure",
			Envelope:  secretEnvelope,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)

		serviceWithDifferentCipher := NewBankSyncService(store, WithBankSyncServiceConnectionSecretCipher(otherCipher))
		_, err = serviceWithDifferentCipher.decryptConnectionSecret(t.Context(), "secret-open-failure")
		require.ErrorContains(t, err, "open connection secret")

		_, err = service.syncRunAlreadyApplied(t.Context(), "connection-1", "sync-1")
		require.ErrorContains(t, err, "apply provider sync result")

		claimed, err := service.claimSyncRun(t.Context(), "connection-1", "", "job-1", now)
		require.NoError(t, err)
		require.True(t, claimed)

		claimed, err = service.claimSyncRun(t.Context(), "connection-1", "sync-1", "job-1", now)
		require.ErrorContains(t, err, "apply provider sync result")
		require.False(t, claimed)
	})

	t.Run("bank sync focused service wraps mark and record schedule failures", func(t *testing.T) {
		store := makeStore(t)
		cipher := makeCipher(t)
		failingStore := &failingProviderSyncStore{Store: store}
		service := NewBankSyncService(failingStore, WithBankSyncServiceConnectionSecretCipher(cipher))
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant := makeTenant(t, NewTenantService(store), ownerUserID)

		sealed, err := cipher.SealString("secret")
		require.NoError(t, err)
		_, err = store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-schedule-1",
			Provider:  bankProviderMonobank,
			Reference: "ref-schedule-1",
			Envelope:  sealed,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-schedule-1",
			TenantID:          tenant.ID,
			Provider:          bankProviderMonobank,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			DisplayName:       "display",
			ProviderReference: "ref-schedule-1",
			ExternalID:        "external-schedule-1",
			SecretID:          "secret-schedule-1",
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)
		nextRunAt := time.Now().UTC().Add(time.Hour)
		_, err = store.SaveBankConnectionSchedule(t.Context(), domain.BankConnectionSchedule{
			ConnectionID: connection.ID,
			Interval:     time.Hour,
			NextRunAt:    &nextRunAt,
			Enabled:      true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		})
		require.NoError(t, err)

		failingStore.saveScheduleErr = errors.New("save schedule failed")
		err = service.markBankConnectionSyncStarted(
			t.Context(),
			&connection,
			RunBankConnectionSyncParams{JobID: "job-1"},
			time.Now().UTC(),
			&ProviderScheduledRunMetadata{ScheduledAt: time.Now().UTC()},
		)
		require.ErrorContains(t, err, "save bank connection schedule")

		failingStore.saveScheduleErr = nil
		failingStore.getScheduleErr = persistence.ErrBankConnectionScheduleNotFound
		err = service.recordBankConnectionSyncFailure(
			t.Context(),
			&connection,
			RunBankConnectionSyncParams{JobID: "job-2"},
			time.Now().UTC(),
			&ProviderScheduledRunMetadata{ScheduledAt: time.Now().UTC()},
			errors.New("sync failed"),
		)
		require.ErrorContains(t, err, "run bank connection sync")

		failingStore.getScheduleErr = nil
		failingStore.saveScheduleErr = errors.New("save schedule failed again")
		err = service.recordBankConnectionSyncFailure(
			t.Context(),
			&connection,
			RunBankConnectionSyncParams{JobID: "job-3"},
			time.Now().UTC(),
			&ProviderScheduledRunMetadata{ScheduledAt: time.Now().UTC()},
			errors.New("sync failed again"),
		)
		require.ErrorContains(t, err, "save bank connection schedule")
	})

	t.Run("bank sync focused service keeps tenant and provider lookups explicit", func(t *testing.T) {
		store := makeStore(t)
		provider := &stubBankProvider{name: bankProviderMonobank}
		service := NewBankSyncService(store, WithBankSyncServiceProviders(provider))
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant := makeTenant(t, NewTenantService(store), ownerUserID)
		otherTenant := makeTenant(t, NewTenantService(store), "other-"+fake.UUID().V4())
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-other-tenant",
			TenantID:          otherTenant.ID,
			Provider:          bankProviderMonobank,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			DisplayName:       "display",
			ProviderReference: "ref-other-tenant",
			ExternalID:        "external-other-tenant",
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)

		_, err = service.bankProvider(bankProviderMonobank)
		require.NoError(t, err)
		_, err = service.requireTenantBankConnection(t.Context(), tenant.ID, ownerUserID, connection.ID)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
	})

	t.Run("root provider sync helpers cover remaining writer secret and schedule error branches", func(t *testing.T) {
		writeErr := errors.New("write schedule failed")
		writerService := NewService(
			stubStore{},
			WithBankConnectionSyncScheduleWriter(&capturedBankSyncScheduleWriter{err: writeErr}),
		)
		err := writerService.writeBankConnectionSyncSchedule(t.Context(), BankConnectionSyncSchedule{})
		require.ErrorContains(t, err, "write bank connection sync schedule")

		store := makeStore(t)
		cipher := makeCipher(t)
		otherCipher := makeOtherCipher(t)
		failingStore := &failingProviderSyncStore{Store: store}
		service := NewService(
			failingStore,
			WithConnectionSecretCipher(cipher),
			WithBankProviders(
				&stubBankProvider{name: bankConnectorEnableBanking},
				&stubBankProvider{name: bankProviderMonobank},
			),
		)

		failingStore.saveConnectionSecretErr = errors.New("save secret failed")
		_, err = service.encryptAndSaveConnectionSecret(t.Context(), bankProviderMonobank, "ref-1", "secret-1")
		require.ErrorContains(t, err, "save connection secret")
		failingStore.saveConnectionSecretErr = nil

		sealed, err := cipher.SealString("secret-root")
		require.NoError(t, err)
		_, err = store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "root-secret-open-failure",
			Provider:  bankProviderMonobank,
			Reference: "ref-root-open-failure",
			Envelope:  sealed,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
		serviceWithDifferentCipher := NewService(store, WithConnectionSecretCipher(otherCipher))
		_, err = serviceWithDifferentCipher.decryptConnectionSecret(t.Context(), "root-secret-open-failure")
		require.ErrorContains(t, err, "open connection secret")

		providerRef, providerErr := service.bankProvider(bankProviderMonobank)
		require.NoError(t, providerErr)
		require.Equal(t, bankProviderMonobank, providerRef.bankID)

		failingStore.getSyncRunErr = errors.New("get root sync run failed")
		_, err = service.syncRunAlreadyApplied(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: failingStore},
			"connection-1",
			"sync-1",
		)
		require.ErrorContains(t, err, "apply provider sync result")
		failingStore.getSyncRunErr = nil

		failingStore.listBankConnectionsErr = errors.New("list bank connections failed")
		_, err = service.saveLinkedBankConnection(
			t.Context(),
			"tenant-1",
			bankProviderPKO,
			domain.ProviderConnectorIDEnableBanking,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref-2",
				Secret:            "secret-2",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.ErrorContains(t, err, "list bank connections")
		failingStore.listBankConnectionsErr = nil

		failingStore.saveRawPayloadErr = errors.New("save raw payload failed")
		_, err = service.saveLinkedBankConnection(
			t.Context(),
			"tenant-1",
			bankProviderMonobank,
			domain.ProviderConnectorIDMonobank,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref-3",
				Secret:            "secret-3",
				State:             domain.BankConnectionStateActive,
				RawPayloads: []ProviderRawPayload{{
					Scope:            domain.RawPayloadScopeConnection,
					ProviderObjectID: "payload-1",
					PayloadJSON:      []byte(`{"ok":true}`),
				}},
			},
		)
		require.ErrorContains(t, err, "save raw payload")
		failingStore.saveRawPayloadErr = nil

		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "root-schedule-connection-1",
			TenantID:          "tenant-1",
			Provider:          bankProviderMonobank,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			DisplayName:       "display",
			ProviderReference: "ref-root-schedule-1",
			ExternalID:        "external-root-schedule-1",
			SecretID:          "root-secret-open-failure",
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)

		failingStore.getScheduleErr = persistence.ErrBankConnectionScheduleNotFound
		err = service.recordBankConnectionSyncFailure(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: failingStore},
			&connection,
			RunBankConnectionSyncParams{JobID: "job-root-1"},
			time.Now().UTC(),
			&ProviderScheduledRunMetadata{ScheduledAt: time.Now().UTC()},
			errors.New("root sync failed"),
		)
		require.ErrorContains(t, err, "run bank connection sync")
	})

	t.Run("root provider sync helpers cover link and sync state failure branches", func(t *testing.T) {
		store := makeStore(t)
		cipher := makeCipher(t)
		providerErr := errors.New("provider link failed")
		provider := &stubBankProvider{
			name:    bankProviderMonobank,
			linkErr: providerErr,
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				ExternalID:        "external",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		}
		failingStore := &failingProviderSyncStore{Store: store}
		service := NewService(failingStore, WithConnectionSecretCipher(cipher), WithBankProviders(provider))
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant := makeTenant(t, NewTenantService(store), ownerUserID)

		_, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderMonobank,
			Token:       "token-1",
		})
		require.ErrorContains(t, err, "link token bank connection")

		provider.linkErr = nil
		failingStore.saveBankConnectionErr = errors.New("save bank connection failed")
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderMonobank,
			Token:       "token-2",
		})
		require.ErrorContains(t, err, "save bank connection")

		failingStore.saveBankConnectionErr = nil
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-root-sync-state",
			TenantID:          tenant.ID,
			Provider:          bankProviderMonobank,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			DisplayName:       "display",
			ProviderReference: "ref-root-sync-state",
			ExternalID:        "external-root-sync-state",
			SecretID:          "",
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)
		nextRunAt := time.Now().UTC().Add(time.Hour)
		_, err = store.SaveBankConnectionSchedule(t.Context(), domain.BankConnectionSchedule{
			ConnectionID: connection.ID,
			Interval:     time.Hour,
			NextRunAt:    &nextRunAt,
			Enabled:      true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		})
		require.NoError(t, err)

		failingStore.saveBankConnectionErr = errors.New("save bank connection failed again")
		err = service.markBankConnectionSyncStarted(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: failingStore},
			&connection,
			RunBankConnectionSyncParams{JobID: "job-root-2"},
			time.Now().UTC(),
			&ProviderScheduledRunMetadata{ScheduledAt: time.Now().UTC()},
		)
		require.ErrorContains(t, err, "save bank connection")

		err = service.recordBankConnectionSyncFailure(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: failingStore},
			&connection,
			RunBankConnectionSyncParams{JobID: "job-root-3"},
			time.Now().UTC(),
			&ProviderScheduledRunMetadata{ScheduledAt: time.Now().UTC()},
			errors.New("sync failed"),
		)
		require.ErrorContains(t, err, "save bank connection")
	})

	t.Run("root provider sync helpers cover membership secret-store and sync-run branches", func(t *testing.T) {
		cipher := makeCipher(t)
		provider := &stubBankProvider{
			name: bankProviderMonobank,
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				ExternalID:        "external",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		}
		serviceWithoutStores := NewService(
			stubStore{},
			WithConnectionSecretCipher(cipher),
			WithBankProviders(provider),
		)
		_, err := serviceWithoutStores.encryptAndSaveConnectionSecret(
			t.Context(),
			bankProviderMonobank,
			"ref",
			"secret",
		)
		require.ErrorContains(t, err, "connection secret store is required")
		_, err = serviceWithoutStores.decryptConnectionSecret(t.Context(), "secret-id")
		require.ErrorContains(t, err, "connection secret store is required")
		_, err = serviceWithoutStores.saveLinkedBankConnection(
			t.Context(),
			"tenant-1",
			bankProviderMonobank,
			domain.ProviderConnectorIDMonobank,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.ErrorContains(t, err, "connection secret store is required")

		store := makeStore(t)
		service := NewService(store, WithConnectionSecretCipher(cipher), WithBankProviders(provider))
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		outsiderUserID := "outsider-" + fake.UUID().V4()
		tenant := makeTenant(t, NewTenantService(store), ownerUserID)
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderMonobank,
			Token:       "token-3",
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		sealed, err := cipher.SealString("secret-success")
		require.NoError(t, err)
		_, err = store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "root-secret-success",
			Provider:  bankProviderMonobank,
			Reference: "ref-success",
			Envelope:  sealed,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
		plaintext, err := service.decryptConnectionSecret(t.Context(), "root-secret-success")
		require.NoError(t, err)
		require.Equal(t, "secret-success", plaintext)

		_, err = store.SaveBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
			ID:           "sync-run-1",
			ConnectionID: "connection-1",
			SyncKey:      "sync-key-1",
			JobID:        "job-1",
			CreatedAt:    time.Now().UTC(),
		})
		require.NoError(t, err)
		alreadyApplied, err := service.syncRunAlreadyApplied(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: store},
			"connection-1",
			"sync-key-1",
		)
		require.NoError(t, err)
		require.True(t, alreadyApplied)

		claimed, err := service.claimSyncRun(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: store},
			"connection-2",
			"sync-key-2",
			"job-2",
			time.Now().UTC(),
		)
		require.NoError(t, err)
		require.True(t, claimed)
	})

	t.Run("bank sync option helpers apply focused dependencies", func(t *testing.T) {
		service := &BankSyncService{}
		WithBankSyncServiceNow(func() time.Time { return time.Unix(1, 0).UTC() })(service)
		WithBankSyncServiceIDGenerator(func() string { return "id-1" })(service)
		WithBankSyncServiceConnectionSecretCipher(makeCipher(t))(service)
		WithBankSyncServiceProviders(&stubBankProvider{name: "monobank"})(service)
		WithBankSyncServiceJobEnqueuer(&capturedBankSyncJobEnqueuer{})(service)
		WithBankSyncServiceScheduleWriter(&capturedBankSyncScheduleWriter{})(service)
		WithBankSyncServiceLogger(defaultLogger())(service)
		require.NotNil(t, service.now)
		require.NotNil(t, service.newID)
		require.NotNil(t, service.connectionSecretCipher)
		require.NotNil(t, service.bankProviders)
		require.NotNil(t, service.bankSyncJobEnqueuer)
		require.NotNil(t, service.bankSyncScheduleWriter)
		require.NotNil(t, service.logger)
	})

	t.Run("reporting and fx focused option helpers apply explicit config", func(t *testing.T) {
		reporting := NewReportingService(makeStore(t), WithReportingServiceDefaultFXProvider("custom"))
		require.Equal(t, "custom", reporting.defaultFXProvider)

		fxService := NewFXService(makeStore(t), WithFXServiceDefaultProvider("custom"))
		_, err := fxService.resolveFXProviderName("")
		require.Error(t, err)
	})

	t.Run("csv import job returns read errors through focused service", func(t *testing.T) {
		service := NewCSVImportService(stubStore{}, &CatalogService{}, &LedgerService{})
		_, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{ImportID: "missing"})
		require.Error(t, err)
	})
}
