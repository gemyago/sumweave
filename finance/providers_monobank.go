package finance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	monobankclient "github.com/gemyago/signal-foundry/finance/internal/monobank/client"
)

const monobankStatementChunkRange = 31*24*time.Hour + time.Hour

const (
	monobankCurrencyUAH = 980
	monobankCurrencyEUR = 978
	monobankCurrencyUSD = 840
	currencyEUR         = "EUR"
	currencyUSD         = "USD"
)

type MonobankProviderConfig struct {
	BaseURL              string
	HTTPClient           *http.Client
	SleepBetweenRequests time.Duration
	Sleep                func(time.Duration)
}

type MonobankProvider struct {
	client               *monobankclient.Client
	sleepBetweenRequests time.Duration
	sleep                func(time.Duration)
}

func NewMonobankProvider(config MonobankProviderConfig) *MonobankProvider {
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &MonobankProvider{
		client: monobankclient.NewClient(
			monobankclient.Args{BaseURL: config.BaseURL},
			monobankclient.WithHTTPClient(client),
		),
		sleepBetweenRequests: config.SleepBetweenRequests,
		sleep:                config.Sleep,
	}
}

func (p *MonobankProvider) Name() string { return bankProviderMonobank }

func (p *MonobankProvider) StartLink(
	context.Context,
	ProviderStartLinkParams,
) (ProviderLinkStart, error) {
	return ProviderLinkStart{}, errors.New("monobank redirect linking unsupported")
}

func (p *MonobankProvider) FinishLink(
	context.Context,
	ProviderFinishLinkParams,
) (ProviderLinkResult, error) {
	return ProviderLinkResult{}, errors.New("monobank redirect linking unsupported")
}

func (p *MonobankProvider) LinkToken(
	ctx context.Context,
	params ProviderTokenLinkParams,
) (ProviderTokenLinkResult, error) {
	response, err := p.client.GetPersonalClientInfo(
		ctx,
		monobankclient.GetPersonalClientInfoParams{Token: params.Token},
	)
	if err != nil {
		return ProviderTokenLinkResult{}, normalizeMonobankClientError(err)
	}
	body := response.ClientInfo
	return ProviderTokenLinkResult{
		DisplayName:       firstNonEmpty(body.Name, "monobank"),
		ProviderReference: firstNonEmpty(body.Name, "monobank"),
		ExternalID:        firstMonobankAccountID(body.Accounts),
		Secret:            strings.TrimSpace(params.Token),
		State:             domain.BankConnectionStateActive,
		RawPayloads: []ProviderRawPayload{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: firstNonEmpty(body.Name, "monobank"),
			PayloadJSON:      response.RawJSON,
		}},
	}, nil
}

//nolint:funlen // Provider normalization intentionally keeps the HTTP-to-domain mapping in one place.
func (p *MonobankProvider) Sync(
	ctx context.Context,
	params ProviderSyncParams,
) (ProviderSyncResult, error) {
	clientInfoResponse, err := p.client.GetPersonalClientInfo(
		ctx,
		monobankclient.GetPersonalClientInfoParams{Token: params.Secret},
	)
	if err != nil {
		return ProviderSyncResult{}, normalizeMonobankClientError(err)
	}
	clientInfo := clientInfoResponse.ClientInfo
	accountItems := clientInfo.Accounts
	accounts := make([]ProviderNormalizedAccount, 0, len(accountItems))
	accountIDs := make([]string, 0, len(accountItems))
	transactions := make([]ProviderNormalizedTransaction, 0)
	rawPayloads := []ProviderRawPayload{
		{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: firstNonEmpty(params.ExternalID, "client-info"),
			PayloadJSON:      clientInfoResponse.RawJSON,
		},
	}
	for _, item := range accountItems {
		currentAccountID := item.ID
		accountIDs = append(accountIDs, currentAccountID)
		balance := item.Balance
		accounts = append(accounts, ProviderNormalizedAccount{
			ProviderAccountID: currentAccountID,
			Name:              firstNonEmpty(item.Type, currentAccountID),
			Currency:          monobankCurrencyCodeToISO(item.CurrencyCode),
			IBAN:              item.IBAN,
			CurrentBalanceMinor: func() *int64 {
				return &balance
			}(),
		})
	}
	chunks := make([]monobankChunk, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		chunks = append(chunks, makeMonobankChunks(accountID, params.WindowStart, params.WindowEnd)...)
	}
	for index, chunk := range chunks {
		if index > 0 && p.sleepBetweenRequests > 0 {
			if p.sleep != nil {
				p.sleep(p.sleepBetweenRequests)
			} else {
				timer := time.NewTimer(p.sleepBetweenRequests)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ProviderSyncResult{}, ctx.Err()
				case <-timer.C:
				}
			}
		}
		statementResponse, callErr := p.client.GetPersonalStatement(
			ctx,
			monobankclient.GetPersonalStatementParams{
				Token:   params.Secret,
				Account: chunk.accountID,
				From:    chunk.fromUnix,
				To:      chunk.toUnix,
			},
		)
		if callErr != nil {
			return ProviderSyncResult{}, normalizeMonobankClientError(callErr)
		}
		rawPayloads = append(
			rawPayloads,
			ProviderRawPayload{
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: chunk.accountID,
				PayloadJSON:      statementResponse.RawJSON,
			},
		)
		for _, item := range statementResponse.Items {
			effectiveAt := time.Unix(item.Time, 0).UTC()
			description := item.Description
			currency := monobankCurrencyCodeToISO(item.CurrencyCode)
			amount := item.Amount
			transactions = append(transactions, ProviderNormalizedTransaction{
				ProviderAccountID:     chunk.accountID,
				ProviderTransactionID: item.ID,
				Status:                bookedStatusFromMonobankHold(item.Hold),
				AmountMinor:           amount,
				Currency:              currency,
				Description:           description,
				EffectiveAt:           effectiveAt,
				Fingerprint: providerFingerprint(
					chunk.accountID,
					description,
					amount,
					currency,
					effectiveAt,
				),
				ProviderOriginal: &domain.ProviderTransactionOriginal{
					AmountMinor: amount,
					Currency:    currency,
					Description: description,
					EffectiveAt: &effectiveAt,
				},
				RawPayloadJSON: mustJSON(item),
			})
		}
	}
	return ProviderSyncResult{
		SyncKey: providerFingerprint(
			"monobank",
			mustJSON(clientInfo),
			int64(len(transactions)),
			strings.TrimSpace(params.ExternalID),
			params.WindowStart.UTC(),
			params.WindowEnd,
		),
		Accounts:     accounts,
		Transactions: transactions,
		RawPayloads:  rawPayloads,
	}, nil
}

type monobankChunk struct {
	accountID string
	fromUnix  int64
	toUnix    int64
}

func makeMonobankChunks(accountID string, from time.Time, to time.Time) []monobankChunk {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return []monobankChunk{{accountID: firstNonEmpty(accountID, "0"), fromUnix: 0, toUnix: 0}}
	}
	items := []monobankChunk{}
	chunkFrom := from.UTC()
	for {
		chunkTo := chunkFrom.Add(monobankStatementChunkRange)
		if chunkTo.After(to) {
			chunkTo = to.UTC()
		}
		items = append(
			items,
			monobankChunk{
				accountID: firstNonEmpty(accountID, "0"),
				fromUnix:  chunkFrom.Unix(),
				toUnix:    chunkTo.Unix(),
			},
		)
		if !chunkTo.Before(to.UTC()) {
			break
		}
		chunkFrom = chunkTo.Add(time.Second)
	}
	return items
}

func bookedStatusFromMonobankHold(hold bool) domain.TransactionStatus {
	if hold {
		return domain.TransactionStatusPending
	}
	return domain.TransactionStatusBooked
}

func firstMonobankAccountID(accounts []monobankclient.InfoAccount) string {
	if len(accounts) == 0 {
		return ""
	}

	return strings.TrimSpace(accounts[0].ID)
}

func normalizeMonobankClientError(err error) error {
	var apiErr *monobankclient.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return errors.New("monobank provider rate limit reached")
		}
		return fmt.Errorf("monobank provider request failed with status %d", apiErr.StatusCode)
	}

	return err
}

func monobankCurrencyCodeToISO(code int) string {
	switch code {
	case monobankCurrencyUAH:
		return "UAH"
	case monobankCurrencyEUR:
		return currencyEUR
	case monobankCurrencyUSD:
		return currencyUSD
	default:
		return ""
	}
}
