//nolint:funlen,gocognit // realistic fixture scenario stays intentionally linear and service-backed.
package fixtures

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
)

type financeScenarioService interface {
	CreateTenant(context.Context, financepkg.CreateTenantParams) (domain.Tenant, error)
	CreateTenantInvite(
		context.Context,
		financepkg.CreateTenantInviteParams,
	) (domain.TenantInvite, error)
	AcceptTenantInvite(
		context.Context,
		financepkg.AcceptTenantInviteParams,
	) (domain.TenantMembership, error)
	CreateAccount(context.Context, financepkg.CreateAccountParams) (domain.Account, error)
	ListCategories(context.Context, financepkg.ListCategoriesParams) ([]domain.Category, error)
	ListTags(context.Context, financepkg.ListTagsParams) ([]domain.Tag, error)
	PreviewCSVImport(
		context.Context,
		financepkg.PreviewCSVImportParams,
	) (financepkg.CSVImportPreview, error)
	RecordTransaction(
		context.Context,
		financepkg.RecordTransactionParams,
	) (domain.Transaction, error)
	HideTransaction(context.Context, financepkg.HideTransactionParams) error
	LinkTransfers(context.Context, financepkg.LinkTransfersParams) error
	LinkTokenBankConnection(
		context.Context,
		financepkg.LinkTokenBankConnectionParams,
	) (domain.BankConnection, error)
	UpsertBankConnectionSchedule(
		context.Context,
		financepkg.UpsertBankConnectionScheduleParams,
	) (domain.BankConnectionSchedule, error)
	SyncFXRates(context.Context, financepkg.SyncFXRatesParams) (financepkg.SyncFXRatesResult, error)
}

const (
	fixtureCurrencyUSD                = "USD"
	fixtureCurrencyEUR                = "EUR"
	fixtureFXProvider                 = financepkg.FXProviderFrankfurter
	fixtureConnectionScheduleInterval = 15 * time.Minute
	fixtureHoursPerDay                = 24
)

func RealisticScenarioOwnerUserID(seed int64) string {
	return fmt.Sprintf("fixture-owner-%d", seed)
}

func RealisticScenarioMemberUserID(seed int64) string {
	return fmt.Sprintf("fixture-member-%d", seed)
}

func RealisticScenarioWindow(anchor time.Time) (time.Time, time.Time) {
	return anchor.AddDate(0, -11, 0), anchor
}

//nolint:mnd // Deterministic fixture rates intentionally use simple fixed coefficients.
func RealisticScenarioStaticFXRates(provider string, anchor time.Time) []domain.FXRate {
	startDate := anchor.AddDate(0, -11, 0)
	endDate := anchor
	rateDates := []time.Time{startDate}
	for monthStart := startDate; !monthStart.After(endDate); monthStart = monthStart.AddDate(0, 1, 0) {
		for _, dayOffset := range []int{0, 9, 26} {
			date := monthStart.AddDate(0, 0, dayOffset)
			if date.After(endDate) {
				continue
			}
			rateDates = append(rateDates, date)
		}
	}
	rates := make([]domain.FXRate, 0, len(rateDates))
	seenDates := make(map[string]struct{}, len(rateDates))
	for _, date := range rateDates {
		dateKey := date.Format(time.RFC3339Nano)
		if _, seen := seenDates[dateKey]; seen {
			continue
		}
		seenDates[dateKey] = struct{}{}
		monthComponent := float64(int(date.Month())-1) * 0.004
		dayComponent := float64(date.Day()-1) * 0.0005
		rates = append(rates, domain.FXRate{
			Provider:      strings.TrimSpace(provider),
			BaseCurrency:  fixtureCurrencyEUR,
			QuoteCurrency: fixtureCurrencyUSD,
			RateDate:      date,
			Rate:          1.05 + monthComponent + dayComponent,
		})
	}
	return rates
}

//nolint:funlen,gocognit,gocyclo,cyclop,goconst,mnd // Deterministic scenario coverage is intentionally explicit.
func GenerateRealisticScenario(
	ctx context.Context,
	bootstrap *Bootstrapper,
	service financeScenarioService,
	config Config,
) (Summary, error) {
	return bootstrap.Bootstrap(
		ctx,
		config,
		NamedScenario(
			"realistic-core",
			ScenarioBuilderFunc(func(scope ScenarioContext, handle *RunHandle) error {
				ownerUserID := strings.TrimSpace(scope.Config.OwnerUserID)
				if ownerUserID == "" {
					ownerUserID = RealisticScenarioOwnerUserID(scope.Config.Seed)
				}
				memberUserID := strings.TrimSpace(scope.Config.MemberUserID)
				if memberUserID == "" {
					memberUserID = RealisticScenarioMemberUserID(scope.Config.Seed)
				}
				connectionProvider := strings.TrimSpace(scope.Config.ConnectionProvider)
				if connectionProvider == "" {
					connectionProvider = "monobank"
				}
				startDate, endDate := RealisticScenarioWindow(scope.Config.Now)

				tenant, err := service.CreateTenant(ctx, financepkg.CreateTenantParams{
					ActorUserID:     ownerUserID,
					Name:            "Fixture Tenant",
					DisplayCurrency: fixtureCurrencyUSD,
				})
				if err != nil {
					return err
				}
				invite, err := service.CreateTenantInvite(ctx, financepkg.CreateTenantInviteParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
					Recipient:   "member@example.com",
				})
				if err != nil {
					return err
				}
				if _, acceptErr := service.AcceptTenantInvite(
					ctx,
					financepkg.AcceptTenantInviteParams{
						ActorUserID: memberUserID,
						Code:        invite.Code,
					},
				); acceptErr != nil {
					return acceptErr
				}

				categories, err := service.ListCategories(ctx, financepkg.ListCategoriesParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
				})
				if err != nil {
					return err
				}
				tags, err := service.ListTags(ctx, financepkg.ListTagsParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
				})
				if err != nil {
					return err
				}
				if len(tags) == 0 {
					return errors.New("realistic scenario requires seeded default tags")
				}

				categoriesByName := make(map[string]domain.Category, len(categories))
				for _, category := range categories {
					categoriesByName[category.Name] = category
				}

				checking, err := service.CreateAccount(ctx, financepkg.CreateAccountParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
					Name:        "Checking",
					Currency:    fixtureCurrencyUSD,
					Kind:        domain.AccountKindManual,
				})
				if err != nil {
					return err
				}
				savings, err := service.CreateAccount(ctx, financepkg.CreateAccountParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
					Name:        "Savings",
					Currency:    fixtureCurrencyEUR,
					Kind:        domain.AccountKindManual,
				})
				if err != nil {
					return err
				}
				importedCard, err := service.CreateAccount(ctx, financepkg.CreateAccountParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
					Name:        "Imported Card",
					Currency:    fixtureCurrencyUSD,
					Kind:        domain.AccountKindImported,
				})
				if err != nil {
					return err
				}
				reconciliation, err := service.CreateAccount(ctx, financepkg.CreateAccountParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
					Name:        "Reconciliation",
					Currency:    fixtureCurrencyUSD,
					Kind:        domain.AccountKindReconciliation,
				})
				if err != nil {
					return err
				}

				tagPreview, err := service.PreviewCSVImport(
					ctx,
					financepkg.PreviewCSVImportParams{
						ActorUserID: ownerUserID,
						TenantID:    tenant.ID,
						ImportType:  financepkg.CSVImportTypeTransactions,
						FileName:    "realistic-tag-preview.csv",
						CSV: "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n" +
							startDate.AddDate(0, 0, 8).Format("02.01.06") + "," + importedCard.Name +
							",Travel & Vacation,Travel,150.00,," + fixtureCurrencyUSD + ",travel import preview\n",
					},
				)
				if err != nil {
					return err
				}
				if slices.Contains(tagPreview.WouldCreateTags, "Travel") {
					return errors.New(
						"realistic scenario expected seeded tag to be reusable in csv preview: Travel",
					)
				}
				recordTransaction := func(
					accountID string,
					source domain.TransactionSource,
					status domain.TransactionStatus,
					kind domain.TransactionKind,
					amountMinor int64,
					currency string,
					description string,
					effectiveAt time.Time,
					categoryName string,
					providerOriginal *domain.ProviderTransactionOriginal,
					transferGroupID string,
				) (domain.Transaction, error) {
					params := financepkg.RecordTransactionParams{
						ActorUserID:      ownerUserID,
						TenantID:         tenant.ID,
						AccountID:        accountID,
						Source:           source,
						Status:           status,
						Kind:             kind,
						AmountMinor:      amountMinor,
						Currency:         currency,
						Description:      description,
						EffectiveAt:      effectiveAt,
						ProviderOriginal: providerOriginal,
						TransferGroupID:  transferGroupID,
					}
					if categoryName != "" {
						category, found := categoriesByName[categoryName]
						if !found {
							return domain.Transaction{}, fmt.Errorf(
								"realistic scenario missing seeded category: %s",
								categoryName,
							)
						}
						params.CategoryID = category.ID
					}
					return service.RecordTransaction(ctx, params)
				}

				if _, openingErr := recordTransaction(
					checking.ID,
					domain.TransactionSourceSystem,
					domain.TransactionStatusBooked,
					domain.TransactionKindOpeningBalance,
					420_000,
					fixtureCurrencyUSD,
					"opening balance checking",
					startDate,
					"",
					nil,
					"",
				); openingErr != nil {
					return openingErr
				}
				if _, openingErr := recordTransaction(
					savings.ID,
					domain.TransactionSourceSystem,
					domain.TransactionStatusBooked,
					domain.TransactionKindOpeningBalance,
					120_000,
					fixtureCurrencyEUR,
					"opening balance savings",
					startDate,
					"",
					nil,
					"",
				); openingErr != nil {
					return openingErr
				}

				for monthStart := startDate; !monthStart.After(endDate); monthStart = monthStart.AddDate(0, 1, 0) {
					monthIndex := (monthStart.Year()-startDate.Year())*12 +
						int(monthStart.Month()-startDate.Month())

					if _, paycheckErr := recordTransaction(
						checking.ID,
						domain.TransactionSourceManual,
						domain.TransactionStatusBooked,
						domain.TransactionKindRegular,
						320_000,
						fixtureCurrencyUSD,
						"monthly paycheck",
						monthStart,
						"Paycheck",
						nil,
						"",
					); paycheckErr != nil {
						return paycheckErr
					}
					if monthIndex%3 == 0 {
						if _, bonusErr := recordTransaction(
							checking.ID,
							domain.TransactionSourceManual,
							domain.TransactionStatusBooked,
							domain.TransactionKindRegular,
							45_000,
							fixtureCurrencyUSD,
							"quarterly bonus",
							monthStart.AddDate(0, 0, 1),
							"Bonus",
							nil,
							"",
						); bonusErr != nil {
							return bonusErr
						}
					}

					for _, item := range []struct {
						day          int
						accountID    string
						source       domain.TransactionSource
						status       domain.TransactionStatus
						kind         domain.TransactionKind
						amountMinor  int64
						currency     string
						description  string
						categoryName string
					}{
						{2, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -135_000, fixtureCurrencyUSD, "rent", "Housing"},
						{3, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -18_000, fixtureCurrencyUSD, "utilities", "Utilities"},
						{4, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -12_000, fixtureCurrencyUSD, "groceries week 1", "Groceries"},
						{11, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -11_500, fixtureCurrencyUSD, "groceries week 2", "Groceries"},
						{18, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -12_400, fixtureCurrencyUSD, "groceries week 3", "Groceries"},
						{25, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -11_900, fixtureCurrencyUSD, "groceries week 4", "Groceries"},
						{6, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -2_200, fixtureCurrencyUSD, "coffee 1", "Dining & Coffee"},
						{13, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -2_600, fixtureCurrencyUSD, "coffee 2", "Dining & Coffee"},
						{20, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -2_450, fixtureCurrencyUSD, "coffee 3", "Dining & Coffee"},
						{27, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -2_900, fixtureCurrencyUSD, "coffee 4", "Dining & Coffee"},
						{5, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -5_400, fixtureCurrencyUSD, "metro pass 1", "Transportation"},
						{12, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -5_700, fixtureCurrencyUSD, "metro pass 2", "Transportation"},
						{19, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -5_500, fixtureCurrencyUSD, "fuel 1", "Transportation"},
						{26, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -5_900, fixtureCurrencyUSD, "fuel 2", "Transportation"},
						{8, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -6_400, fixtureCurrencyUSD, "streaming and cinema", "Entertainment"},
						{22, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -7_100, fixtureCurrencyUSD, "concert tickets", "Entertainment"},
						{14, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -8_000, fixtureCurrencyUSD, "pharmacy", "Health & Medical"},
						{21, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -3_500, fixtureCurrencyUSD, "personal care", "Personal Care"},
						{17, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -9_200, fixtureCurrencyUSD, "household shopping", "Shopping"},
						{29, checking.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, -4_700, fixtureCurrencyUSD, "gift", "Gifts & Donations"},
						{7, importedCard.ID, domain.TransactionSourceProvider, domain.TransactionStatusBooked, domain.TransactionKindRegular, -9_100, fixtureCurrencyUSD, "card groceries", "Groceries"},
						{23, importedCard.ID, domain.TransactionSourceProvider, domain.TransactionStatusBooked, domain.TransactionKindRegular, -6_400, fixtureCurrencyUSD, "rideshare", "Transportation"},
						{9, importedCard.ID, domain.TransactionSourceCSV, domain.TransactionStatusBooked, domain.TransactionKindRegular, -15_000, fixtureCurrencyUSD, "travel import", "Travel & Vacation"},
						{24, importedCard.ID, domain.TransactionSourceCSV, domain.TransactionStatusBooked, domain.TransactionKindRegular, -5_300, fixtureCurrencyUSD, "subscription import", "Miscellaneous"},
						{16, importedCard.ID, domain.TransactionSourceProvider, domain.TransactionStatusBooked, domain.TransactionKindRefund, 1_800, fixtureCurrencyUSD, "merchant refund", "Shopping"},
						{28, reconciliation.ID, domain.TransactionSourceSystem, domain.TransactionStatusBooked, domain.TransactionKindReconciliation, -2_500, fixtureCurrencyUSD, "statement reconciliation", ""},
						{27, savings.ID, domain.TransactionSourceManual, domain.TransactionStatusPending, domain.TransactionKindRegular, -34_000, fixtureCurrencyEUR, "travel booking pending", "Travel & Vacation"},
						{30, savings.ID, domain.TransactionSourceManual, domain.TransactionStatusBooked, domain.TransactionKindRegular, 900, fixtureCurrencyEUR, "monthly interest", "Interest & Dividends"},
					} {
						var providerOriginal *domain.ProviderTransactionOriginal
						if item.source == domain.TransactionSourceProvider {
							originalAt := monthStart.AddDate(0, 0, item.day-1)
							providerOriginal = &domain.ProviderTransactionOriginal{
								AmountMinor: item.amountMinor - 300,
								Currency:    item.currency,
								Description: item.description + " original",
								EffectiveAt: &originalAt,
							}
						}
						if _, transactionErr := recordTransaction(
							item.accountID,
							item.source,
							item.status,
							item.kind,
							item.amountMinor,
							item.currency,
							item.description,
							monthStart.AddDate(0, 0, item.day-1),
							item.categoryName,
							providerOriginal,
							"",
						); transactionErr != nil {
							return transactionErr
						}
					}

					transferOut, transferOutErr := recordTransaction(
						checking.ID,
						domain.TransactionSourceManual,
						domain.TransactionStatusBooked,
						domain.TransactionKindTransfer,
						-90_000,
						fixtureCurrencyUSD,
						"move to savings",
						monthStart.AddDate(0, 0, 9),
						"",
						nil,
						"",
					)
					if transferOutErr != nil {
						return transferOutErr
					}
					transferIn, transferInErr := recordTransaction(
						savings.ID,
						domain.TransactionSourceManual,
						domain.TransactionStatusBooked,
						domain.TransactionKindTransfer,
						83_000,
						fixtureCurrencyEUR,
						"move from checking",
						monthStart.AddDate(0, 0, 9),
						"",
						nil,
						"",
					)
					if transferInErr != nil {
						return transferInErr
					}
					if linkErr := service.LinkTransfers(ctx, financepkg.LinkTransfersParams{
						ActorUserID:         ownerUserID,
						TenantID:            tenant.ID,
						FirstTransactionID:  transferOut.ID,
						SecondTransactionID: transferIn.ID,
					}); linkErr != nil {
						return linkErr
					}
					if _, transferErr := recordTransaction(
						checking.ID,
						domain.TransactionSourceCSV,
						domain.TransactionStatusBooked,
						domain.TransactionKindTransfer,
						-12_500,
						fixtureCurrencyUSD,
						"unmatched transfer export",
						monthStart.AddDate(0, 0, 27),
						"",
						nil,
						fmt.Sprintf("unmatched-%d", monthIndex),
					); transferErr != nil {
						return transferErr
					}
					hidden, hiddenErr := recordTransaction(
						importedCard.ID,
						domain.TransactionSourceCSV,
						domain.TransactionStatusBooked,
						domain.TransactionKindRegular,
						-2_100,
						fixtureCurrencyUSD,
						"duplicate import row",
						monthStart.AddDate(0, 0, 14),
						"Shopping",
						nil,
						"",
					)
					if hiddenErr != nil {
						return hiddenErr
					}
					if hideErr := service.HideTransaction(ctx, financepkg.HideTransactionParams{
						ActorUserID:   ownerUserID,
						TenantID:      tenant.ID,
						TransactionID: hidden.ID,
					}); hideErr != nil {
						return hideErr
					}
				}

				connection, err := service.LinkTokenBankConnection(
					ctx,
					financepkg.LinkTokenBankConnectionParams{
						ActorUserID: ownerUserID,
						TenantID:    tenant.ID,
						Provider:    connectionProvider,
						Token:       "fixture-token",
					},
				)
				if err != nil {
					return err
				}
				if _, scheduleErr := service.UpsertBankConnectionSchedule(
					ctx,
					financepkg.UpsertBankConnectionScheduleParams{
						ActorUserID:  ownerUserID,
						TenantID:     tenant.ID,
						ConnectionID: connection.ID,
						Interval:     fixtureConnectionScheduleInterval,
						NextRunAt:    scope.Config.Now.Add(-1 * time.Minute),
					},
				); scheduleErr != nil {
					return scheduleErr
				}
				if _, syncErr := service.SyncFXRates(
					ctx,
					financepkg.SyncFXRatesParams{
						Provider:       fixtureFXProvider,
						BaseCurrencies: []string{fixtureCurrencyEUR},
						QuoteCurrency:  fixtureCurrencyUSD,
						StartDate:      startDate,
						EndDate:        endDate,
					},
				); syncErr != nil {
					return syncErr
				}

				return handle.RecordScenario(
					ctx,
					ScenarioRecord{
						Name:       "realistic-core",
						StableID:   scope.NextStableID("realistic-core"),
						OccurredAt: scope.Config.Now,
					},
				)
			}),
		),
	)
}
