package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BankAuthorizationStore serializes replacement and sync projections on the
// same connection row as ledger apply. No provider call runs under this lock.
const bankConnectionUpdateLock = "UPDATE"

type BankAuthorizationStore struct{ store *Store }

func NewBankAuthorizationStore(store *Store) *BankAuthorizationStore {
	return &BankAuthorizationStore{store: store}
}

type BankAuthorizationTransaction struct {
	*Store

	Connection domain.BankConnection
}

func lockBankConnection(ctx context.Context, db *gorm.DB, id string) (*domain.BankConnection, error) {
	var model bankConnectionModel
	err := db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.Locking{Strength: bankConnectionUpdateLock}).
		Where("id = ?", id).
		First(&model).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBankConnectionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock bank connection: %w", err)
	}
	connection := bankConnectionFromModel(model)
	return &connection, nil
}

func (s *BankAuthorizationStore) WithConnection(
	ctx context.Context,
	expected domain.ProviderConnectionRef,
	fn func(*BankAuthorizationTransaction) error,
) error {
	return s.store.WithTransaction(ctx, func(store *Store) error {
		connection, err := lockBankConnection(ctx, store.db, expected.ConnectionID)
		if err != nil {
			return err
		}
		if connection.ProviderReference != expected.ProviderReference ||
			connection.Provider != string(expected.ProviderID) ||
			connection.ConnectorID != expected.ConnectorID {
			return domain.ErrBankAuthorizationStale
		}
		return fn(&BankAuthorizationTransaction{Store: store, Connection: *connection})
	})
}

func (t *BankAuthorizationTransaction) SQLTransaction() (*sql.Tx, error) {
	return (&providerWindowSyncApplyStore{Store: t.Store}).SQLTransaction()
}

func (t *BankAuthorizationTransaction) Replace(
	ctx context.Context,
	secret domain.ConnectionSecret,
	snapshot domain.ProviderSnapshot,
) error {
	model := newConnectionSecretModel(secret)
	if err := t.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("create replacement secret: %w", err)
	}
	if err := t.db.WithContext(ctx).
		Table((bankConnectionModel{}).TableName()).
		Where("id = ?", t.Connection.ID).
		Updates(map[string]any{
			"provider_reference": secret.Reference,
			"secret_id":          secret.ID,
			columnState:          string(domain.BankConnectionStateActive),
			"reauth_required_at": nil,
			"reauth_reason":      "",
			"last_sync_error":    "",
			columnUpdatedAt:      secret.UpdatedAt,
		}).
		Error; err != nil {
		return fmt.Errorf("replace bank authorization: %w", err)
	}
	if _, err := NewProviderSnapshotStoreFromStore(t.Store).SaveProviderSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("save replacement snapshot: %w", err)
	}
	if err := t.DeleteConnectionSecret(ctx, t.Connection.SecretID); err != nil {
		return fmt.Errorf("delete replaced connection secret: %w", err)
	}
	return nil
}

func (t *BankAuthorizationTransaction) SaveSecretEnvelope(
	ctx context.Context,
	envelope credentials.Envelope,
	now time.Time,
) error {
	if err := t.db.WithContext(ctx).
		Table((connectionSecretModel{}).TableName()).
		Where("id = ?", t.Connection.SecretID).
		Updates(map[string]any{"key_version": envelope.KeyVersion, "algorithm": envelope.Algorithm, columnNonce: envelope.Nonce, columnCiphertext: envelope.Ciphertext, columnUpdatedAt: now}).
		Error; err != nil {
		return fmt.Errorf("save authorization replay: %w", err)
	}
	return nil
}

type BankSyncProjection struct {
	Connection domain.ProviderConnectionRef
	JobID      string
	At         time.Time
	Phase      string
	Error      string
	Reauth     bool
}

func (s *BankAuthorizationStore) ProjectSync(ctx context.Context, projection BankSyncProjection) error {
	return s.WithConnection(ctx, projection.Connection, func(tx *BankAuthorizationTransaction) error {
		if tx.Connection.State != domain.BankConnectionStateActive {
			return domain.ErrBankReauthRequired
		}
		fields := map[string]any{"last_sync_job_id": projection.JobID, columnUpdatedAt: projection.At}
		timing := map[string]any{columnLastJobID: projection.JobID, columnUpdatedAt: projection.At}
		switch projection.Phase {
		case "started":
			fields["last_sync_started_at"], fields["last_sync_error"] = projection.At, ""
			timing["last_started_at"] = projection.At
		case "succeeded":
			fields["last_successful_sync_at"], fields["last_sync_error"] = projection.At, ""
			timing["last_completed_at"] = projection.At
		case "failed":
			fields["last_sync_error"] = projection.Error
			timing["last_completed_at"] = projection.At
			if projection.Reauth {
				fields[columnState], fields["reauth_required_at"], fields["reauth_reason"] = string(
					domain.BankConnectionStateReauthRequired,
				), projection.At, "bank_session_unavailable"
			}
		default:
			return errors.New("invalid bank sync projection")
		}
		if err := tx.db.WithContext(ctx).
			Table((bankConnectionModel{}).TableName()).
			Where("id = ?", tx.Connection.ID).
			Updates(fields).
			Error; err != nil {
			return fmt.Errorf("project bank sync: %w", err)
		}
		if err := tx.db.WithContext(ctx).
			Table((bankConnectionScheduleModel{}).TableName()).
			Where("connection_id = ?", tx.Connection.ID).
			Updates(timing).
			Error; err != nil {
			return fmt.Errorf("project bank sync timing: %w", err)
		}
		return nil
	})
}

// BankAuthorizationBootstrap contains only the installed session and each
// mapping's exact original account source, not a historical snapshot scan.
type BankAuthorizationBootstrap struct {
	Accounts  []domain.ConnectionProviderAccount
	Snapshots []domain.ProviderSnapshot
}

func (t *BankAuthorizationTransaction) LoadAuthorizationBootstrap(
	ctx context.Context,
) (BankAuthorizationBootstrap, error) {
	accounts, err := t.ListConnectionProviderAccounts(ctx, t.Connection.ID)
	if err != nil {
		return BankAuthorizationBootstrap{}, err
	}
	result := BankAuthorizationBootstrap{Accounts: accounts}
	var session providerSnapshotModel
	err = t.db.WithContext(ctx).
		Table(session.TableName()).
		Where("tenant_id = ? AND connection_id = ? AND subject = ? AND kind = ? AND provider_object_id = ? AND finance_account_id = ? AND finance_transaction_id = ?", t.Connection.TenantID, t.Connection.ID, string(domain.ProviderSnapshotSubjectConnection), string(domain.ProviderSnapshotKindConnection), t.Connection.ProviderReference, "", "").
		First(&session).
		Error
	if err != nil {
		return BankAuthorizationBootstrap{}, fmt.Errorf("load installed session source: %w", err)
	}
	result.Snapshots = append(result.Snapshots, providerSnapshotFromModel(session))
	for _, mapping := range accounts {
		var account providerSnapshotModel
		loadErr := t.db.WithContext(ctx).
			Table(account.TableName()).
			Where("tenant_id = ? AND connection_id = ? AND subject = ? AND kind = ? AND provider_object_id = ? AND finance_account_id = ? AND finance_transaction_id = ?", t.Connection.TenantID, t.Connection.ID, string(domain.ProviderSnapshotSubjectAccount), string(domain.ProviderSnapshotKindAccount), mapping.ProviderAccountID, mapping.FinanceAccountID, "").
			First(&account).
			Error
		if loadErr != nil {
			return BankAuthorizationBootstrap{}, fmt.Errorf("load original account source: %w", loadErr)
		}
		result.Snapshots = append(result.Snapshots, providerSnapshotFromModel(account))
	}
	return result, nil
}

// DeleteConnection returns the row actually removed under PostgreSQL's delete
// lock, so owned-secret cleanup cannot use metadata captured before a renewal.
func (s *BankAuthorizationStore) DeleteConnection(ctx context.Context, id string) (*domain.BankConnection, error) {
	var model bankConnectionModel
	result := s.store.db.WithContext(ctx).Clauses(clause.Returning{}).Where("id = ?", id).Delete(&model)
	if result.Error != nil {
		return nil, fmt.Errorf("delete connection and capture authorization: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrBankConnectionNotFound
	}
	connection := bankConnectionFromModel(model)
	return &connection, nil
}
