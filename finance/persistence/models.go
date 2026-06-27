package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
)

type connectionSecretModel struct {
	ID         string    `gorm:"column:id;size:255;not null;primaryKey"`
	Provider   string    `gorm:"column:provider;size:255;not null"`
	Reference  string    `gorm:"column:reference;size:255;not null"`
	KeyVersion string    `gorm:"column:key_version;size:255;not null"`
	Algorithm  string    `gorm:"column:algorithm;size:64;not null"`
	Nonce      string    `gorm:"column:nonce;type:text;not null"`
	Ciphertext string    `gorm:"column:ciphertext;type:text;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

func (connectionSecretModel) TableName() string { return "finance_connection_secrets" }

type fixtureBootstrapRunModel struct {
	ID        string    `gorm:"column:id;size:255;not null;primaryKey"`
	Seed      int64     `gorm:"column:seed;not null"`
	Scenario  string    `gorm:"column:scenario;size:255;not null"`
	StartedAt time.Time `gorm:"column:started_at;not null"`
}

func (fixtureBootstrapRunModel) TableName() string { return "finance_fixture_bootstrap_runs" }

type fixtureScenarioRecordModel struct {
	ID         string    `gorm:"column:id;size:255;not null;primaryKey"`
	RunID      string    `gorm:"column:run_id;size:255;not null"`
	Name       string    `gorm:"column:name;size:255;not null"`
	StableID   string    `gorm:"column:stable_id;size:255;not null"`
	OccurredAt time.Time `gorm:"column:occurred_at;not null"`
}

func (fixtureScenarioRecordModel) TableName() string { return "finance_fixture_scenario_records" }

type tenantModel struct {
	ID              string    `gorm:"column:id;size:255;not null;primaryKey"`
	Name            string    `gorm:"column:name;size:255;not null"`
	DisplayCurrency string    `gorm:"column:display_currency;size:16;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (tenantModel) TableName() string { return "finance_tenants" }

type tenantMembershipModel struct {
	TenantID  string    `gorm:"column:tenant_id;size:255;not null;primaryKey"`
	UserID    string    `gorm:"column:user_id;size:255;not null;primaryKey"`
	JoinedAt  time.Time `gorm:"column:joined_at;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (tenantMembershipModel) TableName() string { return "finance_tenant_memberships" }

type tenantInviteModel struct {
	ID               string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID         string     `gorm:"column:tenant_id;size:255;not null"`
	Code             string     `gorm:"column:code;size:255;not null;uniqueIndex:idx_finance_tenant_invites_code"`
	Recipient        string     `gorm:"column:recipient;size:255;not null"`
	CreatedByUserID  string     `gorm:"column:created_by_user_id;size:255;not null"`
	AcceptedByUserID *string    `gorm:"column:accepted_by_user_id;size:255"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null"`
	AcceptedAt       *time.Time `gorm:"column:accepted_at"`
}

func (tenantInviteModel) TableName() string { return "finance_tenant_invites" }

type accountModel struct {
	ID                string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID          string     `gorm:"column:tenant_id;size:255;not null"`
	Name              string     `gorm:"column:name;size:255;not null"`
	Currency          string     `gorm:"column:currency;size:16;not null"`
	Kind              string     `gorm:"column:kind;size:64;not null"`
	Provider          *string    `gorm:"column:provider;size:255"`
	ProviderAccountID *string    `gorm:"column:provider_account_id;size:255"`
	HiddenAt          *time.Time `gorm:"column:hidden_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null"`
}

func (accountModel) TableName() string { return "finance_accounts" }

type categoryModel struct {
	ID            string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID      string     `gorm:"column:tenant_id;size:255;not null"`
	Name          string     `gorm:"column:name;size:255;not null"`
	Kind          string     `gorm:"column:kind;size:64;not null"`
	SeededDefault bool       `gorm:"column:seeded_default;not null"`
	HiddenAt      *time.Time `gorm:"column:hidden_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null"`
}

func (categoryModel) TableName() string { return "finance_categories" }

type tagModel struct {
	ID        string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID  string     `gorm:"column:tenant_id;size:255;not null"`
	Name      string     `gorm:"column:name;size:255;not null"`
	HiddenAt  *time.Time `gorm:"column:hidden_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (tagModel) TableName() string { return "finance_tags" }

type transactionModel struct {
	ID                  string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID            string     `gorm:"column:tenant_id;size:255;not null"`
	AccountID           string     `gorm:"column:account_id;size:255;not null"`
	Source              string     `gorm:"column:source;size:64;not null"`
	Status              string     `gorm:"column:status;size:64;not null"`
	Kind                string     `gorm:"column:kind;size:64;not null"`
	AmountMinor         int64      `gorm:"column:amount_minor;not null"`
	Currency            string     `gorm:"column:currency;size:16;not null"`
	Description         string     `gorm:"column:description;type:text;not null"`
	EffectiveAt         time.Time  `gorm:"column:effective_at;not null"`
	CategoryID          *string    `gorm:"column:category_id;size:255"`
	TransferGroupID     *string    `gorm:"column:transfer_group_id;size:255"`
	TransferMatchedAt   *time.Time `gorm:"column:transfer_matched_at"`
	HiddenAt            *time.Time `gorm:"column:hidden_at"`
	OriginalAmountMinor *int64     `gorm:"column:original_amount_minor"`
	OriginalCurrency    *string    `gorm:"column:original_currency;size:16"`
	OriginalDescription *string    `gorm:"column:original_description;type:text"`
	OriginalEffectiveAt *time.Time `gorm:"column:original_effective_at"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null"`
}

func (transactionModel) TableName() string { return "finance_transactions" }

type fxRateModel struct {
	ID            string    `gorm:"column:id;size:255;not null;primaryKey"`
	Provider      string    `gorm:"column:provider;size:64;not null;index:idx_finance_fx_rates_pair_date,unique,priority:1"`
	BaseCurrency  string    `gorm:"column:base_currency;size:16;not null;index:idx_finance_fx_rates_pair_date,unique,priority:2"`
	QuoteCurrency string    `gorm:"column:quote_currency;size:16;not null;index:idx_finance_fx_rates_pair_date,unique,priority:3"`
	RateDate      time.Time `gorm:"column:rate_date;not null;index:idx_finance_fx_rates_pair_date,unique,priority:4"`
	RateValue     float64   `gorm:"column:rate_value;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

func (fxRateModel) TableName() string { return "finance_fx_rates" }

type csvImportModel struct {
	ID                    string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID              string     `gorm:"column:tenant_id;size:255;not null;index"`
	Type                  string     `gorm:"column:type;size:64;not null"`
	Status                string     `gorm:"column:status;size:64;not null"`
	FileName              string     `gorm:"column:file_name;size:255;not null"`
	RawCSV                string     `gorm:"column:raw_csv;type:text;not null"`
	HeadersJSON           string     `gorm:"column:headers_json;type:text;not null"`
	MappingJSON           string     `gorm:"column:mapping_json;type:text;not null"`
	DuplicateRowsJSON     string     `gorm:"column:duplicate_rows_json;type:text;not null"`
	RejectedRowsJSON      string     `gorm:"column:rejected_rows_json;type:text;not null"`
	WouldCreateAccounts   string     `gorm:"column:would_create_accounts_json;type:text;not null"`
	WouldCreateCategories string     `gorm:"column:would_create_categories_json;type:text;not null"`
	WouldCreateTags       string     `gorm:"column:would_create_tags_json;type:text;not null"`
	JobID                 string     `gorm:"column:job_id;size:255;not null;default:''"`
	ConfirmedByUserID     string     `gorm:"column:confirmed_by_user_id;size:255;not null;default:''"`
	ConfirmedAt           *time.Time `gorm:"column:confirmed_at"`
	CompletedAt           *time.Time `gorm:"column:completed_at"`
	ImportedCount         int64      `gorm:"column:imported_count;not null"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null"`
}

func (csvImportModel) TableName() string { return "finance_csv_imports" }

type bankConnectionModel struct {
	ID                   string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID             string     `gorm:"column:tenant_id;size:255;not null;index"`
	Provider             string     `gorm:"column:provider;size:255;not null"`
	DisplayName          string     `gorm:"column:display_name;size:255;not null"`
	ProviderReference    string     `gorm:"column:provider_reference;size:255;not null"`
	ExternalID           string     `gorm:"column:external_id;size:255;not null;default:''"`
	SecretID             string     `gorm:"column:secret_id;size:255;not null"`
	State                string     `gorm:"column:state;size:64;not null"`
	ReauthRequiredAt     *time.Time `gorm:"column:reauth_required_at"`
	ReauthReason         string     `gorm:"column:reauth_reason;size:255;not null;default:''"`
	LastSyncJobID        string     `gorm:"column:last_sync_job_id;size:255;not null;default:''"`
	LastSyncStartedAt    *time.Time `gorm:"column:last_sync_started_at"`
	LastSuccessfulSyncAt *time.Time `gorm:"column:last_successful_sync_at"`
	LastSyncError        string     `gorm:"column:last_sync_error;size:255;not null;default:''"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null"`
}

func (bankConnectionModel) TableName() string { return "finance_bank_connections" }

type pendingBankConnectionLinkStartModel struct {
	ID                string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID          string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:1"`
	ActorUserID       string     `gorm:"column:actor_user_id;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:2"`
	Provider          string     `gorm:"column:provider;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:3"`
	State             string     `gorm:"column:state;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:4"`
	CallbackURL       string     `gorm:"column:callback_url;type:text;not null"`
	AuthorizationURL  string     `gorm:"column:authorization_url;type:text;not null"`
	ProviderReference string     `gorm:"column:provider_reference;size:255;not null;default:''"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;not null"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null"`
}

func (pendingBankConnectionLinkStartModel) TableName() string {
	return "finance_pending_bank_link_starts"
}

type bankConnectionScheduleModel struct {
	ConnectionID    string     `gorm:"column:connection_id;size:255;not null;primaryKey"`
	IntervalSeconds int64      `gorm:"column:interval_seconds;not null"`
	NextRunAt       *time.Time `gorm:"column:next_run_at"`
	LastScheduledAt *time.Time `gorm:"column:last_scheduled_at"`
	LastStartedAt   *time.Time `gorm:"column:last_started_at"`
	LastCompletedAt *time.Time `gorm:"column:last_completed_at"`
	LastJobID       string     `gorm:"column:last_job_id;size:255;not null;default:''"`
	Enabled         bool       `gorm:"column:enabled;not null"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (bankConnectionScheduleModel) TableName() string { return "finance_bank_connection_schedules" }

type connectionProviderAccountModel struct {
	ID                   string     `gorm:"column:id;size:255;not null;primaryKey"`
	ConnectionID         string     `gorm:"column:connection_id;size:255;not null;index:idx_finance_connection_provider_accounts_unique,unique,priority:1"`
	ProviderAccountID    string     `gorm:"column:provider_account_id;size:255;not null;index:idx_finance_connection_provider_accounts_unique,unique,priority:2"`
	FinanceAccountID     string     `gorm:"column:finance_account_id;size:255;not null;default:''"`
	Name                 string     `gorm:"column:name;size:255;not null"`
	Currency             string     `gorm:"column:currency;size:16;not null"`
	IBAN                 string     `gorm:"column:iban;size:255;not null;default:''"`
	MaskedPAN            string     `gorm:"column:masked_pan;size:255;not null;default:''"`
	LastSuccessfulSyncAt *time.Time `gorm:"column:last_successful_sync_at"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null"`
}

func (connectionProviderAccountModel) TableName() string {
	return "finance_connection_provider_accounts"
}

type balanceSnapshotModel struct {
	ID                    string    `gorm:"column:id;size:255;not null;primaryKey"`
	ConnectionID          string    `gorm:"column:connection_id;size:255;not null;index:idx_finance_balance_snapshots_connection_id,priority:1"`
	ProviderAccountID     string    `gorm:"column:provider_account_id;size:255;not null"`
	FinanceAccountID      string    `gorm:"column:finance_account_id;size:255;not null;default:''"`
	Currency              string    `gorm:"column:currency;size:16;not null"`
	CurrentBalanceMinor   int64     `gorm:"column:current_balance_minor;not null"`
	AvailableBalanceMinor *int64    `gorm:"column:available_balance_minor"`
	CapturedAt            time.Time `gorm:"column:captured_at;not null;index:idx_finance_balance_snapshots_connection_id,priority:2"`
}

func (balanceSnapshotModel) TableName() string { return "finance_balance_snapshots" }

type rawPayloadModel struct {
	ID               string    `gorm:"column:id;size:255;not null;primaryKey"`
	ConnectionID     string    `gorm:"column:connection_id;size:255;not null;index:idx_finance_raw_payloads_connection_id,priority:1"`
	Scope            string    `gorm:"column:scope;size:64;not null"`
	ProviderObjectID string    `gorm:"column:provider_object_id;size:255;not null;default:''"`
	PayloadJSON      string    `gorm:"column:payload_json;type:text;not null"`
	CapturedAt       time.Time `gorm:"column:captured_at;not null;index:idx_finance_raw_payloads_connection_id,priority:2"`
}

func (rawPayloadModel) TableName() string { return "finance_raw_payloads" }

type bankConnectionSyncRunModel struct {
	ID           string    `gorm:"column:id;size:255;not null;primaryKey"`
	ConnectionID string    `gorm:"column:connection_id;size:255;not null;index:idx_finance_connection_sync_runs_unique,unique,priority:1"`
	SyncKey      string    `gorm:"column:sync_key;size:255;not null;index:idx_finance_connection_sync_runs_unique,unique,priority:2"`
	JobID        string    `gorm:"column:job_id;size:255;not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

func (bankConnectionSyncRunModel) TableName() string { return "finance_bank_connection_sync_runs" }

type providerSyncStateJournalModel struct {
	JournalID                    int64      `gorm:"column:journal_id;not null;primaryKey;autoIncrement"`
	ConnectionID                 string     `gorm:"column:connection_id;size:255;not null;index:idx_finance_provider_sync_state_journal_latest,priority:1"`
	AttemptedAt                  *time.Time `gorm:"column:attempted_at"`
	SucceededAt                  *time.Time `gorm:"column:succeeded_at"`
	WindowStart                  time.Time  `gorm:"column:window_start;not null"`
	WindowEnd                    time.Time  `gorm:"column:window_end;not null"`
	RunID                        string     `gorm:"column:run_id;size:255;not null;default:''"`
	JobID                        string     `gorm:"column:job_id;size:255;not null;default:''"`
	ErrorSummary                 string     `gorm:"column:error_summary;type:text;not null;default:''"`
	ObservedAccounts             int64      `gorm:"column:observed_accounts;not null"`
	ObservedTransactions         int64      `gorm:"column:observed_transactions;not null"`
	CreatedTransactions          int64      `gorm:"column:created_transactions;not null"`
	UpdatedTransactions          int64      `gorm:"column:updated_transactions;not null"`
	AmbiguousCreatedTransactions int64      `gorm:"column:ambiguous_created_transactions;not null"`
	CreatedAt                    time.Time  `gorm:"column:created_at;not null;index:idx_finance_provider_sync_state_journal_latest,priority:2"`
}

func (providerSyncStateJournalModel) TableName() string {
	return "finance_provider_sync_state_journal_records"
}

type providerTransactionMatchModel struct {
	ID                    string    `gorm:"column:id;size:255;not null;primaryKey"`
	ConnectionID          string    `gorm:"column:connection_id;size:255;not null;index:idx_finance_provider_transaction_matches_provider_id,priority:1;index:idx_finance_provider_transaction_matches_fingerprint,priority:1"`
	ProviderAccountID     string    `gorm:"column:provider_account_id;size:255;not null;index:idx_finance_provider_transaction_matches_provider_id,priority:2;index:idx_finance_provider_transaction_matches_fingerprint,priority:2"`
	ProviderTransactionID string    `gorm:"column:provider_transaction_id;size:255;not null;default:'';index:idx_finance_provider_transaction_matches_provider_id,priority:3"`
	Fingerprint           string    `gorm:"column:fingerprint;size:255;not null;default:'';index:idx_finance_provider_transaction_matches_fingerprint,priority:3"`
	TransactionID         string    `gorm:"column:transaction_id;size:255;not null"`
	Status                string    `gorm:"column:status;size:64;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null"`
}

func (providerTransactionMatchModel) TableName() string {
	return "finance_provider_transaction_matches"
}

type syntheticProviderStateModel struct {
	ConnectionID string    `gorm:"column:connection_id;size:255;not null;primaryKey"`
	StateJSON    string    `gorm:"column:state_json;type:text;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (syntheticProviderStateModel) TableName() string {
	return "finance_synthetic_provider_states"
}

func newConnectionSecretModel(secret domain.ConnectionSecret) connectionSecretModel {
	return connectionSecretModel{
		ID:         secret.ID,
		Provider:   secret.Provider,
		Reference:  secret.Reference,
		KeyVersion: secret.Envelope.KeyVersion,
		Algorithm:  secret.Envelope.Algorithm,
		Nonce:      secret.Envelope.Nonce,
		Ciphertext: secret.Envelope.Ciphertext,
		CreatedAt:  normalizeUTC(secret.CreatedAt),
		UpdatedAt:  normalizeUTC(secret.UpdatedAt),
	}
}

func connectionSecretFromModel(model connectionSecretModel) domain.ConnectionSecret {
	return domain.ConnectionSecret{
		ID:        model.ID,
		Provider:  model.Provider,
		Reference: model.Reference,
		Envelope: credentials.Envelope{
			KeyVersion: model.KeyVersion,
			Algorithm:  model.Algorithm,
			Nonce:      model.Nonce,
			Ciphertext: model.Ciphertext,
		},
		CreatedAt: normalizeUTC(model.CreatedAt),
		UpdatedAt: normalizeUTC(model.UpdatedAt),
	}
}

func newFixtureBootstrapRunModel(run domain.FixtureBootstrapRun) fixtureBootstrapRunModel {
	return fixtureBootstrapRunModel{
		ID:        run.ID,
		Seed:      run.Seed,
		Scenario:  run.Scenario,
		StartedAt: normalizeUTC(run.StartedAt),
	}
}

func newFixtureScenarioRecordModel(
	runID string,
	record domain.FixtureScenarioRecord,
) fixtureScenarioRecordModel {
	return fixtureScenarioRecordModel{
		ID:         makeFixtureScenarioRecordID(runID, record.StableID),
		RunID:      runID,
		Name:       record.Name,
		StableID:   record.StableID,
		OccurredAt: normalizeUTC(record.OccurredAt),
	}
}

func makeFixtureScenarioRecordID(runID string, stableID string) string {
	hash := sha256.Sum256([]byte(runID + "\n" + stableID))
	return hex.EncodeToString(hash[:16])
}

func normalizeUTC(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return value.UTC()
}

func normalizeUTCPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func newCSVImportModel(record domain.CSVImportRecord) csvImportModel {
	return csvImportModel{
		ID:                    record.ID,
		TenantID:              record.TenantID,
		Type:                  string(record.Type),
		Status:                string(record.Status),
		FileName:              record.FileName,
		RawCSV:                record.RawCSV,
		HeadersJSON:           mustJSON(record.Headers),
		MappingJSON:           mustJSON(record.Mapping),
		DuplicateRowsJSON:     mustJSON(record.DuplicateRows),
		RejectedRowsJSON:      mustJSON(record.RejectedRows),
		WouldCreateAccounts:   mustJSON(record.WouldCreateAccounts),
		WouldCreateCategories: mustJSON(record.WouldCreateCategories),
		WouldCreateTags:       mustJSON(record.WouldCreateTags),
		JobID:                 record.JobID,
		ConfirmedByUserID:     record.ConfirmedByUserID,
		ConfirmedAt:           normalizeUTCPointer(record.ConfirmedAt),
		CompletedAt:           normalizeUTCPointer(record.CompletedAt),
		ImportedCount:         int64(record.ImportedCount),
		CreatedAt:             normalizeUTC(record.CreatedAt),
		UpdatedAt:             normalizeUTC(record.UpdatedAt),
	}
}

func csvImportFromModel(model csvImportModel) domain.CSVImportRecord {
	var headers []string
	var mapping map[string]string
	var duplicateRows []domain.CSVImportRejectedRow
	var rejectedRows []domain.CSVImportRejectedRow
	var wouldCreateAccounts []string
	var wouldCreateCategories []string
	var wouldCreateTags []string
	mustUnmarshalJSON(model.HeadersJSON, &headers)
	mustUnmarshalJSON(model.MappingJSON, &mapping)
	mustUnmarshalJSON(model.DuplicateRowsJSON, &duplicateRows)
	mustUnmarshalJSON(model.RejectedRowsJSON, &rejectedRows)
	mustUnmarshalJSON(model.WouldCreateAccounts, &wouldCreateAccounts)
	mustUnmarshalJSON(model.WouldCreateCategories, &wouldCreateCategories)
	mustUnmarshalJSON(model.WouldCreateTags, &wouldCreateTags)
	return domain.CSVImportRecord{
		ID:                    model.ID,
		TenantID:              model.TenantID,
		Type:                  domain.CSVImportType(model.Type),
		Status:                domain.CSVImportStatus(model.Status),
		FileName:              model.FileName,
		RawCSV:                model.RawCSV,
		Headers:               headers,
		Mapping:               mapping,
		DuplicateRows:         duplicateRows,
		RejectedRows:          rejectedRows,
		WouldCreateAccounts:   wouldCreateAccounts,
		WouldCreateCategories: wouldCreateCategories,
		WouldCreateTags:       wouldCreateTags,
		JobID:                 model.JobID,
		ConfirmedByUserID:     model.ConfirmedByUserID,
		ConfirmedAt:           normalizeUTCPointer(model.ConfirmedAt),
		CompletedAt:           normalizeUTCPointer(model.CompletedAt),
		ImportedCount:         int(model.ImportedCount),
		CreatedAt:             normalizeUTC(model.CreatedAt),
		UpdatedAt:             normalizeUTC(model.UpdatedAt),
	}
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func mustUnmarshalJSON(payload string, target any) {
	if payload == "" {
		return
	}
	if err := json.Unmarshal([]byte(payload), target); err != nil {
		panic(err)
	}
}

func newTenantModel(tenant domain.Tenant) tenantModel {
	return tenantModel{
		ID:              tenant.ID,
		Name:            tenant.Name,
		DisplayCurrency: tenant.DisplayCurrency,
		CreatedAt:       normalizeUTC(tenant.CreatedAt),
		UpdatedAt:       normalizeUTC(tenant.UpdatedAt),
	}
}

func tenantFromModel(model tenantModel) domain.Tenant {
	return domain.Tenant{
		ID:              model.ID,
		Name:            model.Name,
		DisplayCurrency: model.DisplayCurrency,
		CreatedAt:       normalizeUTC(model.CreatedAt),
		UpdatedAt:       normalizeUTC(model.UpdatedAt),
	}
}

func newTenantMembershipModel(membership domain.TenantMembership) tenantMembershipModel {
	return tenantMembershipModel{
		TenantID:  membership.TenantID,
		UserID:    membership.UserID,
		JoinedAt:  normalizeUTC(membership.JoinedAt),
		CreatedAt: normalizeUTC(membership.CreatedAt),
	}
}

func tenantMembershipFromModel(model tenantMembershipModel) domain.TenantMembership {
	return domain.TenantMembership{
		TenantID:  model.TenantID,
		UserID:    model.UserID,
		JoinedAt:  normalizeUTC(model.JoinedAt),
		CreatedAt: normalizeUTC(model.CreatedAt),
	}
}

func newTenantInviteModel(invite domain.TenantInvite) tenantInviteModel {
	return tenantInviteModel{
		ID:               invite.ID,
		TenantID:         invite.TenantID,
		Code:             invite.Code,
		Recipient:        invite.Recipient,
		CreatedByUserID:  invite.CreatedByUserID,
		AcceptedByUserID: invite.AcceptedByUserID,
		CreatedAt:        normalizeUTC(invite.CreatedAt),
		AcceptedAt:       normalizeUTCPointer(invite.AcceptedAt),
	}
}

func tenantInviteFromModel(model tenantInviteModel) domain.TenantInvite {
	return domain.TenantInvite{
		ID:               model.ID,
		TenantID:         model.TenantID,
		Code:             model.Code,
		Recipient:        model.Recipient,
		CreatedByUserID:  model.CreatedByUserID,
		AcceptedByUserID: model.AcceptedByUserID,
		CreatedAt:        normalizeUTC(model.CreatedAt),
		AcceptedAt:       normalizeUTCPointer(model.AcceptedAt),
	}
}

func newAccountModel(account domain.Account) accountModel {
	model := accountModel{
		ID:        account.ID,
		TenantID:  account.TenantID,
		Name:      account.Name,
		Currency:  account.Currency,
		Kind:      string(account.Kind),
		HiddenAt:  normalizeUTCPointer(account.HiddenAt),
		CreatedAt: normalizeUTC(account.CreatedAt),
		UpdatedAt: normalizeUTC(account.UpdatedAt),
	}
	if account.LinkedAccount != nil {
		provider := account.LinkedAccount.Provider
		providerAccountID := account.LinkedAccount.ProviderAccountID
		model.Provider = &provider
		model.ProviderAccountID = &providerAccountID
	}
	return model
}

func accountFromModel(model accountModel) domain.Account {
	account := domain.Account{
		ID:        model.ID,
		TenantID:  model.TenantID,
		Name:      model.Name,
		Currency:  model.Currency,
		Kind:      domain.AccountKind(model.Kind),
		HiddenAt:  normalizeUTCPointer(model.HiddenAt),
		CreatedAt: normalizeUTC(model.CreatedAt),
		UpdatedAt: normalizeUTC(model.UpdatedAt),
	}
	if model.Provider != nil || model.ProviderAccountID != nil {
		account.LinkedAccount = &domain.LinkedAccount{}
		if model.Provider != nil {
			account.LinkedAccount.Provider = *model.Provider
		}
		if model.ProviderAccountID != nil {
			account.LinkedAccount.ProviderAccountID = *model.ProviderAccountID
		}
	}
	return account
}

func newCategoryModel(category domain.Category) categoryModel {
	return categoryModel{
		ID:            category.ID,
		TenantID:      category.TenantID,
		Name:          category.Name,
		Kind:          string(category.Kind),
		SeededDefault: category.SeededDefault,
		HiddenAt:      normalizeUTCPointer(category.HiddenAt),
		CreatedAt:     normalizeUTC(category.CreatedAt),
		UpdatedAt:     normalizeUTC(category.UpdatedAt),
	}
}

func categoryFromModel(model categoryModel) domain.Category {
	return domain.Category{
		ID:            model.ID,
		TenantID:      model.TenantID,
		Name:          model.Name,
		Kind:          domain.CategoryKind(model.Kind),
		SeededDefault: model.SeededDefault,
		HiddenAt:      normalizeUTCPointer(model.HiddenAt),
		CreatedAt:     normalizeUTC(model.CreatedAt),
		UpdatedAt:     normalizeUTC(model.UpdatedAt),
	}
}

func newTagModel(tag domain.Tag) tagModel {
	return tagModel{
		ID:        tag.ID,
		TenantID:  tag.TenantID,
		Name:      tag.Name,
		HiddenAt:  normalizeUTCPointer(tag.HiddenAt),
		CreatedAt: normalizeUTC(tag.CreatedAt),
		UpdatedAt: normalizeUTC(tag.UpdatedAt),
	}
}

func tagFromModel(model tagModel) domain.Tag {
	return domain.Tag{
		ID:        model.ID,
		TenantID:  model.TenantID,
		Name:      model.Name,
		HiddenAt:  normalizeUTCPointer(model.HiddenAt),
		CreatedAt: normalizeUTC(model.CreatedAt),
		UpdatedAt: normalizeUTC(model.UpdatedAt),
	}
}

func newTransactionModel(transaction domain.Transaction) transactionModel {
	model := transactionModel{
		ID:                transaction.ID,
		TenantID:          transaction.TenantID,
		AccountID:         transaction.AccountID,
		Source:            string(transaction.Source),
		Status:            string(transaction.Status),
		Kind:              string(transaction.Kind),
		AmountMinor:       transaction.AmountMinor,
		Currency:          transaction.Currency,
		Description:       transaction.Description,
		EffectiveAt:       normalizeUTC(transaction.EffectiveAt),
		CategoryID:        transaction.CategoryID,
		TransferGroupID:   transaction.TransferGroupID,
		TransferMatchedAt: normalizeUTCPointer(transaction.TransferMatchedAt),
		HiddenAt:          normalizeUTCPointer(transaction.HiddenAt),
		CreatedAt:         normalizeUTC(transaction.CreatedAt),
		UpdatedAt:         normalizeUTC(transaction.UpdatedAt),
	}
	if transaction.ProviderOriginal != nil {
		original := transaction.ProviderOriginal
		model.OriginalAmountMinor = &original.AmountMinor
		if original.Currency != "" {
			currency := original.Currency
			model.OriginalCurrency = &currency
		}
		if original.Description != "" {
			description := original.Description
			model.OriginalDescription = &description
		}
		model.OriginalEffectiveAt = normalizeUTCPointer(original.EffectiveAt)
	}
	return model
}

func transactionFromModel(model transactionModel) domain.Transaction {
	transaction := domain.Transaction{
		ID:                model.ID,
		TenantID:          model.TenantID,
		AccountID:         model.AccountID,
		Source:            domain.TransactionSource(model.Source),
		Status:            domain.TransactionStatus(model.Status),
		Kind:              domain.TransactionKind(model.Kind),
		AmountMinor:       model.AmountMinor,
		Currency:          model.Currency,
		Description:       model.Description,
		EffectiveAt:       normalizeUTC(model.EffectiveAt),
		CategoryID:        model.CategoryID,
		TransferGroupID:   model.TransferGroupID,
		TransferMatchedAt: normalizeUTCPointer(model.TransferMatchedAt),
		HiddenAt:          normalizeUTCPointer(model.HiddenAt),
		CreatedAt:         normalizeUTC(model.CreatedAt),
		UpdatedAt:         normalizeUTC(model.UpdatedAt),
	}
	if model.OriginalAmountMinor != nil ||
		model.OriginalCurrency != nil ||
		model.OriginalDescription != nil ||
		model.OriginalEffectiveAt != nil {
		transaction.ProviderOriginal = &domain.ProviderTransactionOriginal{}
		if model.OriginalAmountMinor != nil {
			transaction.ProviderOriginal.AmountMinor = *model.OriginalAmountMinor
		}
		if model.OriginalCurrency != nil {
			transaction.ProviderOriginal.Currency = *model.OriginalCurrency
		}
		if model.OriginalDescription != nil {
			transaction.ProviderOriginal.Description = *model.OriginalDescription
		}
		transaction.ProviderOriginal.EffectiveAt = normalizeUTCPointer(model.OriginalEffectiveAt)
	}
	return transaction
}

func newFXRateModel(rate domain.FXRate) fxRateModel {
	rateDate := normalizeUTC(rate.RateDate)
	return fxRateModel{
		ID:            makeFXRateID(rate.Provider, rate.BaseCurrency, rate.QuoteCurrency, rateDate),
		Provider:      rate.Provider,
		BaseCurrency:  rate.BaseCurrency,
		QuoteCurrency: rate.QuoteCurrency,
		RateDate:      rateDate,
		RateValue:     rate.Rate,
		CreatedAt:     normalizeUTC(rate.CreatedAt),
		UpdatedAt:     normalizeUTC(rate.UpdatedAt),
	}
}

func fxRateFromModel(model fxRateModel) domain.FXRate {
	return domain.FXRate{
		Provider:      model.Provider,
		BaseCurrency:  model.BaseCurrency,
		QuoteCurrency: model.QuoteCurrency,
		RateDate:      normalizeUTC(model.RateDate),
		Rate:          model.RateValue,
		CreatedAt:     normalizeUTC(model.CreatedAt),
		UpdatedAt:     normalizeUTC(model.UpdatedAt),
	}
}

func makeFXRateID(
	provider string,
	baseCurrency string,
	quoteCurrency string,
	rateDate time.Time,
) string {
	hash := sha256.Sum256([]byte(
		provider + "\n" + baseCurrency + "\n" + quoteCurrency + "\n" + rateDate.Format(
			time.DateOnly,
		),
	))
	return hex.EncodeToString(hash[:16])
}

func newBankConnectionModel(connection domain.BankConnection) bankConnectionModel {
	model := bankConnectionModel{
		ID:                   connection.ID,
		TenantID:             connection.TenantID,
		Provider:             connection.Provider,
		DisplayName:          connection.DisplayName,
		ProviderReference:    connection.ProviderReference,
		ExternalID:           connection.ExternalID,
		SecretID:             connection.SecretID,
		State:                string(connection.State),
		LastSyncJobID:        connection.LastSyncJobID,
		LastSyncStartedAt:    normalizeUTCPointer(connection.LastSyncStartedAt),
		LastSuccessfulSyncAt: normalizeUTCPointer(connection.LastSuccessfulSyncAt),
		LastSyncError:        connection.LastSyncError,
		CreatedAt:            normalizeUTC(connection.CreatedAt),
		UpdatedAt:            normalizeUTC(connection.UpdatedAt),
	}
	if connection.Reauth != nil {
		model.ReauthRequiredAt = normalizeUTCPointer(connection.Reauth.RequiredAt)
		model.ReauthReason = connection.Reauth.Reason
	}
	return model
}

func bankConnectionFromModel(model bankConnectionModel) domain.BankConnection {
	connection := domain.BankConnection{
		ID:                   model.ID,
		TenantID:             model.TenantID,
		Provider:             model.Provider,
		DisplayName:          model.DisplayName,
		ProviderReference:    model.ProviderReference,
		ExternalID:           model.ExternalID,
		SecretID:             model.SecretID,
		State:                domain.BankConnectionState(model.State),
		LastSyncJobID:        model.LastSyncJobID,
		LastSyncStartedAt:    normalizeUTCPointer(model.LastSyncStartedAt),
		LastSuccessfulSyncAt: normalizeUTCPointer(model.LastSuccessfulSyncAt),
		LastSyncError:        model.LastSyncError,
		CreatedAt:            normalizeUTC(model.CreatedAt),
		UpdatedAt:            normalizeUTC(model.UpdatedAt),
	}
	if model.ReauthRequiredAt != nil || model.ReauthReason != "" {
		connection.Reauth = &domain.ConnectionReauthMetadata{
			RequiredAt: normalizeUTCPointer(model.ReauthRequiredAt),
			Reason:     model.ReauthReason,
		}
	}
	return connection
}

func newPendingBankConnectionLinkStartModel(
	start domain.PendingBankConnectionLinkStart,
) pendingBankConnectionLinkStartModel {
	return pendingBankConnectionLinkStartModel{
		ID:                start.ID,
		TenantID:          start.TenantID,
		ActorUserID:       start.ActorUserID,
		Provider:          start.Provider,
		State:             start.State,
		CallbackURL:       start.CallbackURL,
		AuthorizationURL:  start.AuthorizationURL,
		ProviderReference: start.ProviderReference,
		ExpiresAt:         normalizeUTC(start.ExpiresAt),
		ConsumedAt:        normalizeUTCPointer(start.ConsumedAt),
		CreatedAt:         normalizeUTC(start.CreatedAt),
		UpdatedAt:         normalizeUTC(start.UpdatedAt),
	}
}

func pendingBankConnectionLinkStartFromModel(
	model pendingBankConnectionLinkStartModel,
) domain.PendingBankConnectionLinkStart {
	return domain.PendingBankConnectionLinkStart{
		ID:                model.ID,
		TenantID:          model.TenantID,
		ActorUserID:       model.ActorUserID,
		Provider:          model.Provider,
		State:             model.State,
		CallbackURL:       model.CallbackURL,
		AuthorizationURL:  model.AuthorizationURL,
		ProviderReference: model.ProviderReference,
		ExpiresAt:         normalizeUTC(model.ExpiresAt),
		ConsumedAt:        normalizeUTCPointer(model.ConsumedAt),
		CreatedAt:         normalizeUTC(model.CreatedAt),
		UpdatedAt:         normalizeUTC(model.UpdatedAt),
	}
}

func newBankConnectionScheduleModel(
	schedule domain.BankConnectionSchedule,
) bankConnectionScheduleModel {
	return bankConnectionScheduleModel{
		ConnectionID:    schedule.ConnectionID,
		IntervalSeconds: int64(schedule.Interval / time.Second),
		NextRunAt:       normalizeUTCPointer(schedule.NextRunAt),
		LastScheduledAt: normalizeUTCPointer(schedule.LastScheduledAt),
		LastStartedAt:   normalizeUTCPointer(schedule.LastStartedAt),
		LastCompletedAt: normalizeUTCPointer(schedule.LastCompletedAt),
		LastJobID:       schedule.LastJobID,
		Enabled:         schedule.Enabled,
		CreatedAt:       normalizeUTC(schedule.CreatedAt),
		UpdatedAt:       normalizeUTC(schedule.UpdatedAt),
	}
}

func bankConnectionScheduleFromModel(
	model bankConnectionScheduleModel,
) domain.BankConnectionSchedule {
	return domain.BankConnectionSchedule{
		ConnectionID:    model.ConnectionID,
		Interval:        time.Duration(model.IntervalSeconds) * time.Second,
		NextRunAt:       normalizeUTCPointer(model.NextRunAt),
		LastScheduledAt: normalizeUTCPointer(model.LastScheduledAt),
		LastStartedAt:   normalizeUTCPointer(model.LastStartedAt),
		LastCompletedAt: normalizeUTCPointer(model.LastCompletedAt),
		LastJobID:       model.LastJobID,
		Enabled:         model.Enabled,
		CreatedAt:       normalizeUTC(model.CreatedAt),
		UpdatedAt:       normalizeUTC(model.UpdatedAt),
	}
}

func newConnectionProviderAccountModel(
	account domain.ConnectionProviderAccount,
) connectionProviderAccountModel {
	return connectionProviderAccountModel{
		ID:                   account.ID,
		ConnectionID:         account.ConnectionID,
		ProviderAccountID:    account.ProviderAccountID,
		FinanceAccountID:     account.FinanceAccountID,
		Name:                 account.Name,
		Currency:             account.Currency,
		IBAN:                 account.IBAN,
		MaskedPAN:            account.MaskedPAN,
		LastSuccessfulSyncAt: normalizeUTCPointer(account.LastSuccessfulSyncAt),
		CreatedAt:            normalizeUTC(account.CreatedAt),
		UpdatedAt:            normalizeUTC(account.UpdatedAt),
	}
}

func connectionProviderAccountFromModel(
	model connectionProviderAccountModel,
) domain.ConnectionProviderAccount {
	return domain.ConnectionProviderAccount{
		ID:                   model.ID,
		ConnectionID:         model.ConnectionID,
		ProviderAccountID:    model.ProviderAccountID,
		FinanceAccountID:     model.FinanceAccountID,
		Name:                 model.Name,
		Currency:             model.Currency,
		IBAN:                 model.IBAN,
		MaskedPAN:            model.MaskedPAN,
		LastSuccessfulSyncAt: normalizeUTCPointer(model.LastSuccessfulSyncAt),
		CreatedAt:            normalizeUTC(model.CreatedAt),
		UpdatedAt:            normalizeUTC(model.UpdatedAt),
	}
}

func newBalanceSnapshotModel(snapshot domain.BalanceSnapshot) balanceSnapshotModel {
	return balanceSnapshotModel{
		ID:                    snapshot.ID,
		ConnectionID:          snapshot.ConnectionID,
		ProviderAccountID:     snapshot.ProviderAccountID,
		FinanceAccountID:      snapshot.FinanceAccountID,
		Currency:              snapshot.Currency,
		CurrentBalanceMinor:   snapshot.CurrentBalanceMinor,
		AvailableBalanceMinor: snapshot.AvailableBalanceMinor,
		CapturedAt:            normalizeUTC(snapshot.CapturedAt),
	}
}

func balanceSnapshotFromModel(model balanceSnapshotModel) domain.BalanceSnapshot {
	return domain.BalanceSnapshot{
		ID:                    model.ID,
		ConnectionID:          model.ConnectionID,
		ProviderAccountID:     model.ProviderAccountID,
		FinanceAccountID:      model.FinanceAccountID,
		Currency:              model.Currency,
		CurrentBalanceMinor:   model.CurrentBalanceMinor,
		AvailableBalanceMinor: model.AvailableBalanceMinor,
		CapturedAt:            normalizeUTC(model.CapturedAt),
	}
}

func newRawPayloadModel(payload domain.RawPayload) rawPayloadModel {
	return rawPayloadModel{
		ID:               payload.ID,
		ConnectionID:     payload.ConnectionID,
		Scope:            string(payload.Scope),
		ProviderObjectID: payload.ProviderObjectID,
		PayloadJSON:      string(payload.PayloadJSON),
		CapturedAt:       normalizeUTC(payload.CapturedAt),
	}
}

func rawPayloadFromModel(model rawPayloadModel) domain.RawPayload {
	return domain.RawPayload{
		ID:               model.ID,
		ConnectionID:     model.ConnectionID,
		Scope:            domain.RawPayloadScope(model.Scope),
		ProviderObjectID: model.ProviderObjectID,
		PayloadJSON:      []byte(model.PayloadJSON),
		CapturedAt:       normalizeUTC(model.CapturedAt),
	}
}

func newBankConnectionSyncRunModel(run domain.BankConnectionSyncRun) bankConnectionSyncRunModel {
	return bankConnectionSyncRunModel{
		ID:           run.ID,
		ConnectionID: run.ConnectionID,
		SyncKey:      run.SyncKey,
		JobID:        run.JobID,
		CreatedAt:    normalizeUTC(run.CreatedAt),
	}
}

func bankConnectionSyncRunFromModel(model bankConnectionSyncRunModel) domain.BankConnectionSyncRun {
	return domain.BankConnectionSyncRun{
		ID:           model.ID,
		ConnectionID: model.ConnectionID,
		SyncKey:      model.SyncKey,
		JobID:        model.JobID,
		CreatedAt:    normalizeUTC(model.CreatedAt),
	}
}

func newProviderSyncStateJournalModel(
	state domain.ProviderSyncState,
	createdAt time.Time,
) providerSyncStateJournalModel {
	model := providerSyncStateJournalModel{
		ConnectionID:                 state.Connection.ConnectionID,
		AttemptedAt:                  normalizeUTCPointer(state.AttemptedAt),
		SucceededAt:                  normalizeUTCPointer(state.SucceededAt),
		WindowStart:                  normalizeUTC(state.Window.Start),
		WindowEnd:                    normalizeUTC(state.Window.End),
		RunID:                        state.RunID,
		JobID:                        state.JobID,
		ErrorSummary:                 state.ErrorSummary,
		ObservedAccounts:             int64(state.AggregateStats.ObservedAccounts),
		ObservedTransactions:         int64(state.AggregateStats.ObservedTransactions),
		CreatedTransactions:          int64(state.AggregateStats.CreatedTransactions),
		UpdatedTransactions:          int64(state.AggregateStats.UpdatedTransactions),
		AmbiguousCreatedTransactions: int64(state.AggregateStats.AmbiguousCreatedTransactions),
		CreatedAt:                    normalizeUTC(createdAt),
	}
	return model
}

func providerSyncStateFromJournalModel(
	model providerSyncStateJournalModel,
	connection domain.ProviderConnectionRef,
) domain.ProviderSyncState {
	state := domain.ProviderSyncState{
		Connection:  connection,
		AttemptedAt: normalizeUTCPointer(model.AttemptedAt),
		SucceededAt: normalizeUTCPointer(model.SucceededAt),
		Window: domain.ProviderSyncWindow{
			Start: normalizeUTC(model.WindowStart),
			End:   normalizeUTC(model.WindowEnd),
		},
		RunID:        model.RunID,
		JobID:        model.JobID,
		ErrorSummary: model.ErrorSummary,
		AggregateStats: domain.ProviderSyncStats{
			ObservedAccounts:             int(model.ObservedAccounts),
			ObservedTransactions:         int(model.ObservedTransactions),
			CreatedTransactions:          int(model.CreatedTransactions),
			UpdatedTransactions:          int(model.UpdatedTransactions),
			AmbiguousCreatedTransactions: int(model.AmbiguousCreatedTransactions),
		},
	}
	return state
}

func newProviderTransactionMatchModel(
	match domain.ProviderTransactionMatch,
) providerTransactionMatchModel {
	return providerTransactionMatchModel{
		ID:                    match.ID,
		ConnectionID:          match.ConnectionID,
		ProviderAccountID:     match.ProviderAccountID,
		ProviderTransactionID: match.ProviderTransactionID,
		Fingerprint:           match.Fingerprint,
		TransactionID:         match.TransactionID,
		Status:                string(match.Status),
		CreatedAt:             normalizeUTC(match.CreatedAt),
		UpdatedAt:             normalizeUTC(match.UpdatedAt),
	}
}

func providerTransactionMatchFromModel(
	model providerTransactionMatchModel,
) domain.ProviderTransactionMatch {
	return domain.ProviderTransactionMatch{
		ID:                    model.ID,
		ConnectionID:          model.ConnectionID,
		ProviderAccountID:     model.ProviderAccountID,
		ProviderTransactionID: model.ProviderTransactionID,
		Fingerprint:           model.Fingerprint,
		TransactionID:         model.TransactionID,
		Status:                domain.TransactionStatus(model.Status),
		CreatedAt:             normalizeUTC(model.CreatedAt),
		UpdatedAt:             normalizeUTC(model.UpdatedAt),
	}
}

func newSyntheticProviderStateModel(
	state domain.SyntheticProviderState,
) syntheticProviderStateModel {
	return syntheticProviderStateModel{
		ConnectionID: state.ConnectionID,
		StateJSON:    mustJSON(normalizeSyntheticProviderStateEnvelope(state.Envelope)),
		CreatedAt:    normalizeUTC(state.CreatedAt),
		UpdatedAt:    normalizeUTC(state.UpdatedAt),
	}
}

func syntheticProviderStateFromModel(
	model syntheticProviderStateModel,
) domain.SyntheticProviderState {
	envelope := domain.SyntheticProviderStateEnvelope{}
	mustUnmarshalJSON(model.StateJSON, &envelope)
	return domain.SyntheticProviderState{
		ConnectionID: model.ConnectionID,
		Envelope:     normalizeSyntheticProviderStateEnvelope(envelope),
		CreatedAt:    normalizeUTC(model.CreatedAt),
		UpdatedAt:    normalizeUTC(model.UpdatedAt),
	}
}

func normalizeSyntheticProviderStateEnvelope(
	envelope domain.SyntheticProviderStateEnvelope,
) domain.SyntheticProviderStateEnvelope {
	normalized := domain.SyntheticProviderStateEnvelope{
		Version:            envelope.Version,
		ConfiguredAccounts: append([]domain.SyntheticConfiguredAccount(nil), envelope.ConfiguredAccounts...),
		WindowHistory:      make([]domain.SyntheticWindowHistoryEntry, 0, len(envelope.WindowHistory)),
		SequenceCounters:   make([]domain.SyntheticAccountDaySequenceCounter, 0, len(envelope.SequenceCounters)),
	}
	for _, entry := range envelope.WindowHistory {
		normalized.WindowHistory = append(normalized.WindowHistory, domain.SyntheticWindowHistoryEntry{
			Window: domain.SyntheticWindowKey{
				NormalizedStartUTC:        normalizeUTC(entry.Window.NormalizedStartUTC),
				NormalizedEndExclusiveUTC: normalizeUTC(entry.Window.NormalizedEndExclusiveUTC),
			},
			RepeatCount: entry.RepeatCount,
		})
	}
	for _, counter := range envelope.SequenceCounters {
		normalized.SequenceCounters = append(
			normalized.SequenceCounters,
			domain.SyntheticAccountDaySequenceCounter{
				AccountKey:   counter.AccountKey,
				DayUTC:       normalizeUTC(counter.DayUTC),
				NextSequence: counter.NextSequence,
			},
		)
	}
	return normalized
}
