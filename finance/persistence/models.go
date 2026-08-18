package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
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
	ID              string     `gorm:"column:id;size:255;not null;primaryKey"`
	Name            string     `gorm:"column:name;size:255;not null"`
	DisplayCurrency string     `gorm:"column:display_currency;size:16;not null"`
	ArchivedAt      *time.Time `gorm:"column:archived_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;index:idx_finance_tenants_created_order"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (tenantModel) TableName() string { return "finance_tenants" }

type tenantMembershipModel struct {
	TenantID  string    `gorm:"column:tenant_id;size:255;not null;primaryKey;index:idx_finance_tenant_memberships_joined_order,priority:1"`
	UserID    string    `gorm:"column:user_id;size:255;not null;primaryKey"`
	JoinedAt  time.Time `gorm:"column:joined_at;not null;index:idx_finance_tenant_memberships_joined_order,priority:2"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (tenantMembershipModel) TableName() string { return "finance_tenant_memberships" }

type tenantInviteModel struct {
	ID               string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID         string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_tenant_invites_created_order,priority:1"`
	Code             string     `gorm:"column:code;size:255;not null;uniqueIndex:idx_finance_tenant_invites_code"`
	Recipient        string     `gorm:"column:recipient;size:255;not null"`
	CreatedByUserID  string     `gorm:"column:created_by_user_id;size:255;not null"`
	AcceptedByUserID *string    `gorm:"column:accepted_by_user_id;size:255"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;index:idx_finance_tenant_invites_created_order,priority:2"`
	AcceptedAt       *time.Time `gorm:"column:accepted_at"`
}

func (tenantInviteModel) TableName() string { return "finance_tenant_invites" }

type accountModel struct {
	ID                string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID          string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_accounts_created_order,priority:1"`
	Name              string     `gorm:"column:name;size:255;not null"`
	Currency          string     `gorm:"column:currency;size:16;not null"`
	Kind              string     `gorm:"column:kind;size:64;not null"`
	Provider          *string    `gorm:"column:provider;size:255"`
	ProviderAccountID *string    `gorm:"column:provider_account_id;size:255"`
	HiddenAt          *time.Time `gorm:"column:hidden_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null;index:idx_finance_accounts_created_order,priority:2"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null"`
}

func (accountModel) TableName() string { return "finance_accounts" }

type categoryModel struct {
	ID            string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID      string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_categories_created_order,priority:1"`
	Name          string     `gorm:"column:name;size:255;not null"`
	Kind          string     `gorm:"column:kind;size:64;not null"`
	SeededDefault bool       `gorm:"column:seeded_default;not null;index:idx_finance_categories_created_order,priority:2"`
	HiddenAt      *time.Time `gorm:"column:hidden_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;index:idx_finance_categories_created_order,priority:3"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null"`
}

func (categoryModel) TableName() string { return "finance_categories" }

type tagModel struct {
	ID        string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID  string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_tags_created_order,priority:1"`
	Name      string     `gorm:"column:name;size:255;not null"`
	HiddenAt  *time.Time `gorm:"column:hidden_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;index:idx_finance_tags_created_order,priority:2"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (tagModel) TableName() string { return "finance_tags" }

type transactionModel struct {
	ID                  string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID            string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_transactions_list_order,priority:1"`
	AccountID           string     `gorm:"column:account_id;size:255;not null"`
	Source              string     `gorm:"column:source;size:64;not null"`
	Status              string     `gorm:"column:status;size:64;not null"`
	Kind                string     `gorm:"column:kind;size:64;not null"`
	AmountMinor         int64      `gorm:"column:amount_minor;not null"`
	Currency            string     `gorm:"column:currency;size:16;not null"`
	Description         string     `gorm:"column:description;type:text;not null"`
	EffectiveAt         time.Time  `gorm:"column:effective_at;not null;index:idx_finance_transactions_provider_window,priority:1;index:idx_finance_transactions_list_order,priority:2"`
	CategoryID          *string    `gorm:"column:category_id;size:255"`
	TransferGroupID     *string    `gorm:"column:transfer_group_id;size:255"`
	TransferMatchedAt   *time.Time `gorm:"column:transfer_matched_at"`
	HiddenAt            *time.Time `gorm:"column:hidden_at"`
	OriginalAmountMinor *int64     `gorm:"column:original_amount_minor"`
	OriginalCurrency    *string    `gorm:"column:original_currency;size:16"`
	OriginalDescription *string    `gorm:"column:original_description;type:text"`
	OriginalEffectiveAt *time.Time `gorm:"column:original_effective_at"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null;index:idx_finance_transactions_provider_window,priority:2;index:idx_finance_transactions_list_order,priority:3"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null"`
}

func (transactionModel) TableName() string { return "finance_transactions" }

type transactionTagModel struct {
	TransactionID string `gorm:"column:transaction_id;size:255;not null;primaryKey;index:idx_finance_transaction_tags_tag_id,priority:2"`
	TagID         string `gorm:"column:tag_id;size:255;not null;primaryKey;index:idx_finance_transaction_tags_tag_id,priority:1"`
}

func (transactionTagModel) TableName() string { return "finance_transaction_tags" }

type currentFXRateModel struct {
	ID                      string    `gorm:"column:id;size:255;not null;primaryKey"`
	Provider                string    `gorm:"column:provider;size:64;not null;index:idx_finance_current_fx_rates_pair,unique,priority:1"`
	BaseCurrency            string    `gorm:"column:base_currency;size:16;not null;index:idx_finance_current_fx_rates_pair,unique,priority:2"`
	QuoteCurrency           string    `gorm:"column:quote_currency;size:16;not null;index:idx_finance_current_fx_rates_pair,unique,priority:3"`
	EffectiveAt             time.Time `gorm:"column:effective_at;not null"`
	LastSuccessfulRefreshAt time.Time `gorm:"column:last_successful_refresh_at;not null"`
	RateValue               float64   `gorm:"column:rate_value;not null"`
	CreatedAt               time.Time `gorm:"column:created_at;not null"`
	UpdatedAt               time.Time `gorm:"column:updated_at;not null"`
}

func (currentFXRateModel) TableName() string { return "finance_current_fx_rates" }

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
	ImportableCount       int64      `gorm:"column:importable_count;not null;default:0"`
	WouldCreateAccounts   string     `gorm:"column:would_create_accounts_json;type:text;not null"`
	WouldCreateCategories string     `gorm:"column:would_create_categories_json;type:text;not null"`
	WouldCreateTags       string     `gorm:"column:would_create_tags_json;type:text;not null"`
	AccountOptionsJSON    string     `gorm:"column:account_options_json;type:text;not null;default:'[]'"`
	SelectedAccountsJSON  string     `gorm:"column:selected_account_names_json;type:text;not null;default:'[]'"`
	JobID                 string     `gorm:"column:job_id;size:255;not null;default:''"`
	ConfirmedByUserID     string     `gorm:"column:confirmed_by_user_id;size:255;not null;default:''"`
	ConfirmedAt           *time.Time `gorm:"column:confirmed_at"`
	CompletedAt           *time.Time `gorm:"column:completed_at"`
	ImportedCount         int64      `gorm:"column:imported_count;not null"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null"`
}

func (csvImportModel) TableName() string { return "finance_csv_imports" }

type csvImportRowOutcomeModel struct {
	ImportID      string    `gorm:"column:import_id;size:255;not null;primaryKey"`
	RowNumber     int       `gorm:"column:row_number;not null;primaryKey"`
	TransactionID string    `gorm:"column:transaction_id;size:255;not null;default:''"`
	Status        string    `gorm:"column:status;size:64;not null"`
	Reason        string    `gorm:"column:reason;type:text;not null;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

func (csvImportRowOutcomeModel) TableName() string { return "finance_csv_import_row_outcomes" }

type bankConnectionModel struct {
	ID                   string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID             string     `gorm:"column:tenant_id;size:255;not null;index;index:idx_finance_bank_connections_created_order,priority:1;index:idx_finance_bank_connections_provider_reference,unique,priority:1,where:provider_reference <> ''"`
	Provider             string     `gorm:"column:provider;size:255;not null;index:idx_finance_bank_connections_provider_reference,unique,priority:2,where:provider_reference <> ''"`
	ConnectorID          string     `gorm:"column:connector_id;size:255;not null;default:'';index:idx_finance_bank_connections_provider_reference,unique,priority:3,where:provider_reference <> ''"`
	DisplayName          string     `gorm:"column:display_name;size:255;not null"`
	ProviderReference    string     `gorm:"column:provider_reference;size:255;not null;default:'';index:idx_finance_bank_connections_provider_reference,unique,priority:4,where:provider_reference <> ''"`
	SecretID             string     `gorm:"column:secret_id;size:255;not null"`
	State                string     `gorm:"column:state;size:64;not null"`
	ReauthRequiredAt     *time.Time `gorm:"column:reauth_required_at"`
	ReauthReason         string     `gorm:"column:reauth_reason;size:255;not null;default:''"`
	LastSyncJobID        string     `gorm:"column:last_sync_job_id;size:255;not null;default:''"`
	LastSyncStartedAt    *time.Time `gorm:"column:last_sync_started_at"`
	LastSuccessfulSyncAt *time.Time `gorm:"column:last_successful_sync_at"`
	LastSyncError        string     `gorm:"column:last_sync_error;size:255;not null;default:''"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null;index:idx_finance_bank_connections_created_order,priority:2"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null"`
}

func (bankConnectionModel) TableName() string { return "finance_bank_connections" }

type pendingBankConnectionLinkStartModel struct {
	ID                string     `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID          string     `gorm:"column:tenant_id;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:1"`
	ActorUserID       string     `gorm:"column:actor_user_id;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:2"`
	Provider          string     `gorm:"column:provider;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:3;index:idx_finance_pending_bank_link_starts_state_created,priority:1"`
	ConnectorID       string     `gorm:"column:connector_id;size:255;not null;default:'';index:idx_finance_pending_bank_link_starts_lookup,unique,priority:4"`
	State             string     `gorm:"column:state;size:255;not null;index:idx_finance_pending_bank_link_starts_lookup,unique,priority:5;index:idx_finance_pending_bank_link_starts_state_created,priority:2"`
	CallbackURL       string     `gorm:"column:callback_url;type:text;not null"`
	AuthorizationURL  string     `gorm:"column:authorization_url;type:text;not null"`
	ProviderReference string     `gorm:"column:provider_reference;size:255;not null;default:''"`
	StartResultJSON   string     `gorm:"column:start_result_json;type:text;not null;default:'{}'"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;not null;index:idx_finance_pending_bank_link_starts_expires_at"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null;index:idx_finance_pending_bank_link_starts_state_created,priority:3"`
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
	ConnectionID         string     `gorm:"column:connection_id;size:255;not null;index:idx_finance_connection_provider_accounts_unique,unique,priority:1;index:idx_finance_connection_provider_accounts_created_order,priority:1"`
	ProviderAccountID    string     `gorm:"column:provider_account_id;size:255;not null;index:idx_finance_connection_provider_accounts_unique,unique,priority:2"`
	FinanceAccountID     string     `gorm:"column:finance_account_id;size:255;not null;default:''"`
	Name                 string     `gorm:"column:name;size:255;not null"`
	Currency             string     `gorm:"column:currency;size:16;not null"`
	IBAN                 string     `gorm:"column:iban;size:255;not null;default:''"`
	MaskedPAN            string     `gorm:"column:masked_pan;size:255;not null;default:''"`
	LastSuccessfulSyncAt *time.Time `gorm:"column:last_successful_sync_at"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null;index:idx_finance_connection_provider_accounts_created_order,priority:2"`
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

type providerSnapshotModel struct {
	ID                   string    `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID             string    `gorm:"column:tenant_id;size:255;not null;index:idx_finance_provider_snapshots_account_order,priority:1;index:idx_finance_provider_snapshots_transaction_order,priority:1;index:idx_finance_provider_snapshots_identity,unique,priority:1"`
	ConnectionID         string    `gorm:"column:connection_id;size:255;not null;index:idx_finance_provider_snapshots_connection_id,priority:1;index:idx_finance_provider_snapshots_identity,unique,priority:2"`
	FinanceAccountID     string    `gorm:"column:finance_account_id;size:255;not null;default:'';index:idx_finance_provider_snapshots_account_order,priority:3;index:idx_finance_provider_snapshots_identity,unique,priority:4"`
	FinanceTransactionID string    `gorm:"column:finance_transaction_id;size:255;not null;default:'';index:idx_finance_provider_snapshots_transaction_order,priority:3;index:idx_finance_provider_snapshots_identity,unique,priority:5"`
	Subject              string    `gorm:"column:subject;size:64;not null;index:idx_finance_provider_snapshots_account_order,priority:2;index:idx_finance_provider_snapshots_transaction_order,priority:2;index:idx_finance_provider_snapshots_identity,unique,priority:3"`
	Kind                 string    `gorm:"column:kind;size:64;not null;index:idx_finance_provider_snapshots_identity,unique,priority:6"`
	ProviderObjectID     string    `gorm:"column:provider_object_id;size:255;not null;index:idx_finance_provider_snapshots_identity,unique,priority:7"`
	DocumentJSON         string    `gorm:"column:document_json;type:text;not null"`
	CapturedAt           time.Time `gorm:"column:captured_at;not null;index:idx_finance_provider_snapshots_connection_id,priority:2;index:idx_finance_provider_snapshots_account_order,priority:4;index:idx_finance_provider_snapshots_transaction_order,priority:4"`
}

func (providerSnapshotModel) TableName() string { return "finance_provider_snapshots" }

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
	CreatedAccounts              int64      `gorm:"column:created_accounts;not null"`
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
	ConnectionID          string    `gorm:"column:connection_id;size:255;not null;index:idx_finance_provider_transaction_matches_provider_id,priority:1;index:idx_finance_provider_transaction_matches_fingerprint,priority:1;index:idx_finance_provider_transaction_matches_window_order,priority:1;index:idx_finance_provider_transaction_matches_updated_order,priority:1"`
	ProviderAccountID     string    `gorm:"column:provider_account_id;size:255;not null;index:idx_finance_provider_transaction_matches_provider_id,priority:2;index:idx_finance_provider_transaction_matches_fingerprint,priority:2;index:idx_finance_provider_transaction_matches_updated_order,priority:2"`
	ProviderTransactionID string    `gorm:"column:provider_transaction_id;size:255;not null;default:'';index:idx_finance_provider_transaction_matches_provider_id,priority:3"`
	Fingerprint           string    `gorm:"column:fingerprint;size:255;not null;default:'';index:idx_finance_provider_transaction_matches_fingerprint,priority:3;index:idx_finance_provider_transaction_matches_updated_order,priority:3"`
	TransactionID         string    `gorm:"column:transaction_id;size:255;not null"`
	Status                string    `gorm:"column:status;size:64;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;not null;index:idx_finance_provider_transaction_matches_window_order,priority:2"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null;index:idx_finance_provider_transaction_matches_updated_order,priority:4"`
}

func (providerTransactionMatchModel) TableName() string {
	return "finance_provider_transaction_matches"
}

type syntheticProviderStateModel struct {
	ProviderReference string    `gorm:"column:provider_reference;size:255;not null;primaryKey"`
	StateJSON         string    `gorm:"column:state_json;type:text;not null"`
	CreatedAt         time.Time `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null"`
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
		CreatedAt:  secret.CreatedAt,
		UpdatedAt:  secret.UpdatedAt,
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
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func newFixtureBootstrapRunModel(run domain.FixtureBootstrapRun) fixtureBootstrapRunModel {
	return fixtureBootstrapRunModel{
		ID:        run.ID,
		Seed:      run.Seed,
		Scenario:  run.Scenario,
		StartedAt: run.StartedAt,
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
		OccurredAt: record.OccurredAt,
	}
}

func makeFixtureScenarioRecordID(runID string, stableID string) string {
	hash := sha256.Sum256([]byte(runID + "\n" + stableID))
	return hex.EncodeToString(hash[:16])
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
		ImportableCount:       int64(record.ImportableCount),
		WouldCreateAccounts:   mustJSON(record.WouldCreateAccounts),
		WouldCreateCategories: mustJSON(record.WouldCreateCategories),
		WouldCreateTags:       mustJSON(record.WouldCreateTags),
		AccountOptionsJSON:    mustJSON(record.AccountOptions),
		SelectedAccountsJSON:  mustJSON(record.SelectedAccountNames),
		JobID:                 record.JobID,
		ConfirmedByUserID:     record.ConfirmedByUserID,
		ConfirmedAt:           record.ConfirmedAt,
		CompletedAt:           record.CompletedAt,
		ImportedCount:         int64(record.ImportedCount),
		CreatedAt:             record.CreatedAt,
		UpdatedAt:             record.UpdatedAt,
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
	var accountOptions []domain.CSVImportAccountOption
	var selectedAccountNames []string
	mustUnmarshalJSON(model.HeadersJSON, &headers)
	mustUnmarshalJSON(model.MappingJSON, &mapping)
	mustUnmarshalJSON(model.DuplicateRowsJSON, &duplicateRows)
	mustUnmarshalJSON(model.RejectedRowsJSON, &rejectedRows)
	mustUnmarshalJSON(model.WouldCreateAccounts, &wouldCreateAccounts)
	mustUnmarshalJSON(model.WouldCreateCategories, &wouldCreateCategories)
	mustUnmarshalJSON(model.WouldCreateTags, &wouldCreateTags)
	mustUnmarshalJSON(model.AccountOptionsJSON, &accountOptions)
	mustUnmarshalJSON(model.SelectedAccountsJSON, &selectedAccountNames)
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
		ImportableCount:       int(model.ImportableCount),
		WouldCreateAccounts:   wouldCreateAccounts,
		WouldCreateCategories: wouldCreateCategories,
		WouldCreateTags:       wouldCreateTags,
		AccountOptions:        accountOptions,
		SelectedAccountNames:  selectedAccountNames,
		JobID:                 model.JobID,
		ConfirmedByUserID:     model.ConfirmedByUserID,
		ConfirmedAt:           model.ConfirmedAt,
		CompletedAt:           model.CompletedAt,
		ImportedCount:         int(model.ImportedCount),
		CreatedAt:             model.CreatedAt,
		UpdatedAt:             model.UpdatedAt,
	}
}

func csvImportRowOutcomeFromModel(model csvImportRowOutcomeModel) domain.CSVImportRowOutcome {
	return domain.CSVImportRowOutcome{
		ImportID:      model.ImportID,
		RowNumber:     model.RowNumber,
		TransactionID: model.TransactionID,
		Status:        domain.CSVImportRowOutcomeStatus(model.Status),
		Reason:        model.Reason,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
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
		ArchivedAt:      tenant.ArchivedAt,
		CreatedAt:       tenant.CreatedAt,
		UpdatedAt:       tenant.UpdatedAt,
	}
}

func tenantFromModel(model tenantModel) domain.Tenant {
	return domain.Tenant{
		ID:              model.ID,
		Name:            model.Name,
		DisplayCurrency: model.DisplayCurrency,
		ArchivedAt:      model.ArchivedAt,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}

func newTenantMembershipModel(membership domain.TenantMembership) tenantMembershipModel {
	return tenantMembershipModel{
		TenantID:  membership.TenantID,
		UserID:    membership.UserID,
		JoinedAt:  membership.JoinedAt,
		CreatedAt: membership.CreatedAt,
	}
}

func tenantMembershipFromModel(model tenantMembershipModel) domain.TenantMembership {
	return domain.TenantMembership{
		TenantID:  model.TenantID,
		UserID:    model.UserID,
		JoinedAt:  model.JoinedAt,
		CreatedAt: model.CreatedAt,
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
		CreatedAt:        invite.CreatedAt,
		AcceptedAt:       invite.AcceptedAt,
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
		CreatedAt:        model.CreatedAt,
		AcceptedAt:       model.AcceptedAt,
	}
}

func newAccountModel(account domain.Account) accountModel {
	model := accountModel{
		ID:        account.ID,
		TenantID:  account.TenantID,
		Name:      account.Name,
		Currency:  account.Currency,
		Kind:      string(account.Kind),
		HiddenAt:  account.HiddenAt,
		CreatedAt: account.CreatedAt,
		UpdatedAt: account.UpdatedAt,
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
		HiddenAt:  model.HiddenAt,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
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
		HiddenAt:      category.HiddenAt,
		CreatedAt:     category.CreatedAt,
		UpdatedAt:     category.UpdatedAt,
	}
}

func categoryFromModel(model categoryModel) domain.Category {
	return domain.Category{
		ID:            model.ID,
		TenantID:      model.TenantID,
		Name:          model.Name,
		Kind:          domain.CategoryKind(model.Kind),
		SeededDefault: model.SeededDefault,
		HiddenAt:      model.HiddenAt,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func newTagModel(tag domain.Tag) tagModel {
	return tagModel{
		ID:        tag.ID,
		TenantID:  tag.TenantID,
		Name:      tag.Name,
		HiddenAt:  tag.HiddenAt,
		CreatedAt: tag.CreatedAt,
		UpdatedAt: tag.UpdatedAt,
	}
}

func tagFromModel(model tagModel) domain.Tag {
	return domain.Tag{
		ID:        model.ID,
		TenantID:  model.TenantID,
		Name:      model.Name,
		HiddenAt:  model.HiddenAt,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
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
		EffectiveAt:       transaction.EffectiveAt,
		CategoryID:        transaction.CategoryID,
		TransferGroupID:   transaction.TransferGroupID,
		TransferMatchedAt: transaction.TransferMatchedAt,
		HiddenAt:          transaction.HiddenAt,
		CreatedAt:         transaction.CreatedAt,
		UpdatedAt:         transaction.UpdatedAt,
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
		model.OriginalEffectiveAt = original.EffectiveAt
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
		EffectiveAt:       model.EffectiveAt,
		CategoryID:        model.CategoryID,
		TransferGroupID:   model.TransferGroupID,
		TransferMatchedAt: model.TransferMatchedAt,
		HiddenAt:          model.HiddenAt,
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
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
		transaction.ProviderOriginal.EffectiveAt = model.OriginalEffectiveAt
	}
	return transaction
}

func newCurrentFXRateModel(rate domain.FXRate) currentFXRateModel {
	return currentFXRateModel{
		ID:                      makeFXRateID(rate.Provider, rate.BaseCurrency, rate.QuoteCurrency),
		Provider:                rate.Provider,
		BaseCurrency:            rate.BaseCurrency,
		QuoteCurrency:           rate.QuoteCurrency,
		EffectiveAt:             rate.EffectiveAt,
		LastSuccessfulRefreshAt: rate.LastSuccessfulRefreshAt,
		RateValue:               rate.Rate,
		CreatedAt:               rate.CreatedAt,
		UpdatedAt:               rate.UpdatedAt,
	}
}

func currentFXRateFromModel(model currentFXRateModel) domain.FXRate {
	return domain.FXRate{
		Provider:                model.Provider,
		BaseCurrency:            model.BaseCurrency,
		QuoteCurrency:           model.QuoteCurrency,
		EffectiveAt:             model.EffectiveAt,
		RateDate:                model.EffectiveAt,
		LastSuccessfulRefreshAt: model.LastSuccessfulRefreshAt,
		Rate:                    model.RateValue,
		CreatedAt:               model.CreatedAt,
		UpdatedAt:               model.UpdatedAt,
	}
}

func makeFXRateID(
	provider string,
	baseCurrency string,
	quoteCurrency string,
) string {
	hash := sha256.Sum256([]byte(
		provider + "\n" + baseCurrency + "\n" + quoteCurrency,
	))
	return hex.EncodeToString(hash[:16])
}

func newBankConnectionModel(connection domain.BankConnection) bankConnectionModel {
	model := bankConnectionModel{
		ID:                   connection.ID,
		TenantID:             connection.TenantID,
		Provider:             connection.Provider,
		ConnectorID:          string(connection.ConnectorID),
		DisplayName:          connection.DisplayName,
		ProviderReference:    connection.ProviderReference,
		SecretID:             connection.SecretID,
		State:                string(connection.State),
		LastSyncJobID:        connection.LastSyncJobID,
		LastSyncStartedAt:    connection.LastSyncStartedAt,
		LastSuccessfulSyncAt: connection.LastSuccessfulSyncAt,
		LastSyncError:        connection.LastSyncError,
		CreatedAt:            connection.CreatedAt,
		UpdatedAt:            connection.UpdatedAt,
	}
	if connection.Reauth != nil {
		model.ReauthRequiredAt = connection.Reauth.RequiredAt
		model.ReauthReason = connection.Reauth.Reason
	}
	return model
}

func bankConnectionFromModel(model bankConnectionModel) domain.BankConnection {
	connection := domain.BankConnection{
		ID:                   model.ID,
		TenantID:             model.TenantID,
		Provider:             model.Provider,
		ConnectorID:          domain.ProviderConnectorID(model.ConnectorID),
		DisplayName:          model.DisplayName,
		ProviderReference:    model.ProviderReference,
		SecretID:             model.SecretID,
		State:                domain.BankConnectionState(model.State),
		LastSyncJobID:        model.LastSyncJobID,
		LastSyncStartedAt:    model.LastSyncStartedAt,
		LastSuccessfulSyncAt: model.LastSuccessfulSyncAt,
		LastSyncError:        model.LastSyncError,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
	}
	if model.ReauthRequiredAt != nil || model.ReauthReason != "" {
		connection.Reauth = &domain.ConnectionReauthMetadata{
			RequiredAt: model.ReauthRequiredAt,
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
		ConnectorID:       string(start.ConnectorID),
		State:             start.State,
		CallbackURL:       start.CallbackURL,
		AuthorizationURL:  start.AuthorizationURL,
		ProviderReference: start.ProviderReference,
		StartResultJSON:   mustJSON(start.StartResult),
		ExpiresAt:         start.ExpiresAt,
		ConsumedAt:        start.ConsumedAt,
		CreatedAt:         start.CreatedAt,
		UpdatedAt:         start.UpdatedAt,
	}
}

func pendingBankConnectionLinkStartFromModel(
	model pendingBankConnectionLinkStartModel,
) domain.PendingBankConnectionLinkStart {
	var startResult domain.PendingBankConnectionLinkStartResult
	mustUnmarshalJSON(model.StartResultJSON, &startResult)
	return domain.PendingBankConnectionLinkStart{
		ID:                model.ID,
		TenantID:          model.TenantID,
		ActorUserID:       model.ActorUserID,
		Provider:          model.Provider,
		ConnectorID:       domain.ProviderConnectorID(model.ConnectorID),
		State:             model.State,
		CallbackURL:       model.CallbackURL,
		AuthorizationURL:  model.AuthorizationURL,
		ProviderReference: model.ProviderReference,
		StartResult:       startResult,
		ExpiresAt:         model.ExpiresAt,
		ConsumedAt:        model.ConsumedAt,
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func newBankConnectionScheduleModel(
	schedule domain.BankConnectionSchedule,
) bankConnectionScheduleModel {
	return bankConnectionScheduleModel{
		ConnectionID:    schedule.ConnectionID,
		IntervalSeconds: int64(schedule.Interval / time.Second),
		NextRunAt:       schedule.NextRunAt,
		LastScheduledAt: schedule.LastScheduledAt,
		LastStartedAt:   schedule.LastStartedAt,
		LastCompletedAt: schedule.LastCompletedAt,
		LastJobID:       schedule.LastJobID,
		Enabled:         schedule.Enabled,
		CreatedAt:       schedule.CreatedAt,
		UpdatedAt:       schedule.UpdatedAt,
	}
}

func bankConnectionScheduleFromModel(
	model bankConnectionScheduleModel,
) domain.BankConnectionSchedule {
	return domain.BankConnectionSchedule{
		ConnectionID:    model.ConnectionID,
		Interval:        time.Duration(model.IntervalSeconds) * time.Second,
		NextRunAt:       model.NextRunAt,
		LastScheduledAt: model.LastScheduledAt,
		LastStartedAt:   model.LastStartedAt,
		LastCompletedAt: model.LastCompletedAt,
		LastJobID:       model.LastJobID,
		Enabled:         model.Enabled,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
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
		LastSuccessfulSyncAt: account.LastSuccessfulSyncAt,
		CreatedAt:            account.CreatedAt,
		UpdatedAt:            account.UpdatedAt,
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
		LastSuccessfulSyncAt: model.LastSuccessfulSyncAt,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
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
		CapturedAt:            snapshot.CapturedAt,
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
		CapturedAt:            model.CapturedAt,
	}
}

func newProviderSnapshotModel(snapshot domain.ProviderSnapshot) providerSnapshotModel {
	return providerSnapshotModel{
		ID:                   snapshot.ID,
		TenantID:             snapshot.TenantID,
		ConnectionID:         snapshot.ConnectionID,
		FinanceAccountID:     snapshot.FinanceAccountID,
		FinanceTransactionID: snapshot.FinanceTransactionID,
		Subject:              string(snapshot.Subject),
		Kind:                 string(snapshot.Kind),
		ProviderObjectID:     snapshot.ProviderObjectID,
		DocumentJSON:         string(snapshot.DocumentJSON),
		CapturedAt:           snapshot.CapturedAt,
	}
}

func providerSnapshotFromModel(model providerSnapshotModel) domain.ProviderSnapshot {
	return domain.ProviderSnapshot{
		ID:                   model.ID,
		TenantID:             model.TenantID,
		ConnectionID:         model.ConnectionID,
		FinanceAccountID:     model.FinanceAccountID,
		FinanceTransactionID: model.FinanceTransactionID,
		Subject:              domain.ProviderSnapshotSubject(model.Subject),
		Kind:                 domain.ProviderSnapshotKind(model.Kind),
		ProviderObjectID:     model.ProviderObjectID,
		DocumentJSON:         []byte(model.DocumentJSON),
		CapturedAt:           model.CapturedAt,
	}
}

func newProviderSyncStateJournalModel(
	state domain.ProviderSyncState,
	createdAt time.Time,
) providerSyncStateJournalModel {
	model := providerSyncStateJournalModel{
		ConnectionID:                 state.Connection.ConnectionID,
		AttemptedAt:                  state.AttemptedAt,
		SucceededAt:                  state.SucceededAt,
		WindowStart:                  state.Window.Start,
		WindowEnd:                    state.Window.End,
		RunID:                        state.RunID,
		JobID:                        state.JobID,
		ErrorSummary:                 state.ErrorSummary,
		ObservedAccounts:             int64(state.AggregateStats.ObservedAccounts),
		CreatedAccounts:              int64(state.AggregateStats.CreatedAccounts),
		ObservedTransactions:         int64(state.AggregateStats.ObservedTransactions),
		CreatedTransactions:          int64(state.AggregateStats.CreatedTransactions),
		UpdatedTransactions:          int64(state.AggregateStats.UpdatedTransactions),
		AmbiguousCreatedTransactions: int64(state.AggregateStats.AmbiguousCreatedTransactions),
		CreatedAt:                    createdAt,
	}
	return model
}

func providerSyncStateFromJournalModel(
	model providerSyncStateJournalModel,
	connection domain.ProviderConnectionRef,
) domain.ProviderSyncState {
	state := domain.ProviderSyncState{
		Connection:  connection,
		AttemptedAt: model.AttemptedAt,
		SucceededAt: model.SucceededAt,
		Window: domain.ProviderSyncWindow{
			Start: model.WindowStart,
			End:   model.WindowEnd,
		},
		RunID:        model.RunID,
		JobID:        model.JobID,
		ErrorSummary: model.ErrorSummary,
		AggregateStats: domain.ProviderSyncStats{
			ObservedAccounts:             int(model.ObservedAccounts),
			CreatedAccounts:              int(model.CreatedAccounts),
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
		CreatedAt:             match.CreatedAt,
		UpdatedAt:             match.UpdatedAt,
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
		CreatedAt:             model.CreatedAt,
		UpdatedAt:             model.UpdatedAt,
	}
}

func newSyntheticProviderStateModel(
	state domain.SyntheticProviderState,
) syntheticProviderStateModel {
	return syntheticProviderStateModel{
		ProviderReference: state.ProviderReference,
		StateJSON:         mustJSON(normalizeSyntheticProviderStateEnvelope(state.Envelope)),
		CreatedAt:         state.CreatedAt,
		UpdatedAt:         state.UpdatedAt,
	}
}

func syntheticProviderStateFromModel(
	model syntheticProviderStateModel,
) domain.SyntheticProviderState {
	envelope := domain.SyntheticProviderStateEnvelope{}
	mustUnmarshalJSON(model.StateJSON, &envelope)
	return domain.SyntheticProviderState{
		ProviderReference: model.ProviderReference,
		Envelope:          normalizeSyntheticProviderStateEnvelope(envelope),
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func normalizeSyntheticProviderStateEnvelope(
	envelope domain.SyntheticProviderStateEnvelope,
) domain.SyntheticProviderStateEnvelope {
	normalized := domain.SyntheticProviderStateEnvelope{
		Version:            envelope.Version,
		ConfiguredAccounts: append([]domain.SyntheticConfiguredAccount(nil), envelope.ConfiguredAccounts...),
		WindowHistory:      make([]domain.SyntheticWindowHistoryEntry, 0, len(envelope.WindowHistory)),
		SequenceCounters:   make([]domain.SyntheticAccountInstantSequenceCounter, 0, len(envelope.SequenceCounters)),
	}
	for _, entry := range envelope.WindowHistory {
		normalized.WindowHistory = append(normalized.WindowHistory, domain.SyntheticWindowHistoryEntry{
			Window: domain.SyntheticWindowKey{
				Start: entry.Window.Start,
				End:   entry.Window.End,
			},
			RepeatCount: entry.RepeatCount,
		})
	}
	for _, counter := range envelope.SequenceCounters {
		normalized.SequenceCounters = append(
			normalized.SequenceCounters,
			domain.SyntheticAccountInstantSequenceCounter{
				AccountKey:   counter.AccountKey,
				Instant:      counter.Instant,
				NextSequence: counter.NextSequence,
			},
		)
	}
	return normalized
}
