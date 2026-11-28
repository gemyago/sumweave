package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	eb "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
)

type bankAuthorizationService struct {
	store       *persistence.Store
	persistence *persistence.BankAuthorizationStore
	cipher      connectionSecretCipher
	writer      *bankConnectionSecretWriter
	publisher   TransactionalSemanticCommandPublisher
	now         func() time.Time
	newID       func() string
}

func bankConnectionRef(c domain.BankConnection) domain.ProviderConnectionRef {
	return domain.ProviderConnectionRef{
		ConnectionID:      c.ID,
		ProviderID:        domain.ProviderID(c.Provider),
		ConnectorID:       c.ConnectorID,
		ProviderReference: c.ProviderReference,
	}
}

func (s *bankAuthorizationService) ValidateUpdate(
	ctx context.Context,
	start domain.PendingBankConnectionLinkStart,
) error {
	target := start.StartResult.UpdateTarget
	c, err := s.store.GetBankConnection(ctx, target.ConnectionID)
	if err != nil {
		return fmt.Errorf("load update target: %w", err)
	}
	if c.TenantID != start.TenantID || c.Provider != start.Provider || c.ConnectorID != start.ConnectorID ||
		c.ProviderReference != target.ExpectedSession {
		return domain.ErrBankAuthorizationStale
	}
	if c.State != domain.BankConnectionStateActive && c.State != domain.BankConnectionStateReauthRequired {
		return domain.ErrBankAuthorizationStale
	}
	return nil
}

func decodeBankAuthorization(
	cipher connectionSecretCipher,
	secret domain.ConnectionSecret,
) (*domain.BankAuthorization, error) {
	plaintext, err := cipher.OpenString(secret.Envelope)
	if err != nil {
		return nil, fmt.Errorf("open bank authorization: %w", err)
	}
	if plaintext == "" {
		//nolint:nilnil // Empty legacy payload explicitly requests saved-identity bootstrap.
		return nil, nil
	}
	var authorization domain.BankAuthorization
	if decodeErr := json.Unmarshal([]byte(plaintext), &authorization); decodeErr != nil {
		return nil, fmt.Errorf("decode bank authorization: %w", decodeErr)
	}
	if validateErr := authorization.Validate(); validateErr != nil {
		return nil, validateErr
	}
	return &authorization, nil
}

func (s *bankAuthorizationService) ReplaceLink(
	ctx context.Context,
	start domain.PendingBankConnectionLinkStart,
	result providers.LinkResult,
) (domain.BankConnection, error) {
	if result.Authorization == nil || result.ConnectionSnapshot == nil ||
		result.State != domain.BankConnectionStateActive ||
		result.ProviderReference == "" {
		return domain.BankConnection{}, domain.ErrBankAuthorizationInvalid
	}
	target := start.StartResult.UpdateTarget
	if result.ProviderReference == target.ExpectedSession {
		return domain.BankConnection{}, domain.ErrBankAuthorizationInvalid
	}
	expected := domain.ProviderConnectionRef{
		ConnectionID:      target.ConnectionID,
		ProviderID:        domain.ProviderID(start.Provider),
		ConnectorID:       start.ConnectorID,
		ProviderReference: target.ExpectedSession,
	}
	var saved domain.BankConnection
	err := s.persistence.WithConnection(ctx, expected, func(tx *persistence.BankAuthorizationTransaction) error {
		if tx.Connection.TenantID != start.TenantID ||
			(tx.Connection.State != domain.BankConnectionStateActive && tx.Connection.State != domain.BankConnectionStateReauthRequired) {
			return domain.ErrBankAuthorizationStale
		}
		now := s.now()
		secret, replay, prepareErr := s.prepareReplacement(ctx, tx, result)
		if prepareErr != nil {
			return prepareErr
		}

		snapshot := domain.ProviderSnapshot{
			ID:               s.newID(),
			TenantID:         start.TenantID,
			ConnectionID:     target.ConnectionID,
			Subject:          domain.ProviderSnapshotSubjectConnection,
			Kind:             domain.ProviderSnapshotKindConnection,
			ProviderObjectID: result.ConnectionSnapshot.ProviderObjectID,
			DocumentJSON:     result.ConnectionSnapshot.DocumentJSON,
			CapturedAt:       result.ConnectionSnapshot.CapturedAt,
		}
		if replaceErr := tx.Replace(ctx, secret, snapshot); replaceErr != nil {
			return replaceErr
		}
		tx.Connection.ProviderReference, tx.Connection.SecretID, tx.Connection.State = result.ProviderReference, secret.ID, domain.BankConnectionStateActive
		tx.Connection.Reauth, tx.Connection.LastSyncError, tx.Connection.UpdatedAt = nil, "", now
		if replay {
			if publishErr := s.publishHistorySync(ctx, tx, start); publishErr != nil {
				return publishErr
			}
		}

		saved = tx.Connection
		return nil
	})
	if err != nil {
		return domain.BankConnection{}, fmt.Errorf("replace bank link: %w", err)
	}
	return saved, nil
}

func (s *bankAuthorizationService) loadAuthorization(
	ctx context.Context,
	tx *persistence.BankAuthorizationTransaction,
) (domain.BankAuthorization, error) {
	secret, err := tx.GetConnectionSecret(ctx, tx.Connection.SecretID)
	if err != nil {
		return domain.BankAuthorization{}, err
	}
	authorization, err := decodeBankAuthorization(s.cipher, *secret)
	if err != nil {
		return domain.BankAuthorization{}, err
	}
	if authorization != nil {
		return *authorization, nil
	}
	return bootstrapBankAuthorization(ctx, tx)
}

// Bootstrap only from exact persisted source identities, never account names,
// secondary hashes, or a search of historical sessions.
func bootstrapBankAuthorization(
	ctx context.Context,
	tx *persistence.BankAuthorizationTransaction,
) (domain.BankAuthorization, error) {
	sources, err := tx.LoadAuthorizationBootstrap(ctx)
	if err != nil {
		return domain.BankAuthorization{}, fmt.Errorf("%w: %w", domain.ErrBankAuthorizationInvalid, err)
	}
	snapshots, accounts := sources.Snapshots, sources.Accounts
	session, err := savedSessionIdentity(tx.Connection, snapshots)
	if err != nil {
		return domain.BankAuthorization{}, err
	}

	authorization := domain.BankAuthorization{
		Version:      1,
		ASPSPName:    session.ASPSP.Name,
		ASPSPCountry: session.ASPSP.Country,
	}
	for _, mapping := range accounts {
		binding, bindingErr := savedAccountBinding(tx.Connection, mapping, snapshots)
		if bindingErr != nil {
			return domain.BankAuthorization{}, bindingErr
		}
		local, loadErr := tx.GetAccount(ctx, mapping.FinanceAccountID)
		if loadErr != nil {
			return domain.BankAuthorization{}, loadErr
		}
		if local.TenantID != tx.Connection.TenantID {
			return domain.BankAuthorization{}, domain.ErrBankAuthorizationInvalid
		}
		for _, selected := range session.AccountsData {
			if selected.UID == binding.RemoteID && selected.IdentificationHash == binding.PrimaryHash {
				binding.Selected = true
			}
		}
		if !binding.Selected {
			return domain.BankAuthorization{}, fmt.Errorf(
				"%w: saved session membership missing",
				domain.ErrBankAuthorizationInvalid,
			)
		}
		authorization.Accounts = append(authorization.Accounts, binding)
	}
	if validateErr := authorization.Validate(); validateErr != nil {
		return domain.BankAuthorization{}, validateErr
	}
	return authorization, nil
}

func savedSessionIdentity(
	connection domain.BankConnection,
	snapshots []domain.ProviderSnapshot,
) (eb.SessionResponse, error) {
	for _, snapshot := range snapshots {
		if snapshot.TenantID != connection.TenantID || snapshot.Subject != domain.ProviderSnapshotSubjectConnection ||
			snapshot.Kind != domain.ProviderSnapshotKindConnection ||
			snapshot.ProviderObjectID != connection.ProviderReference {
			continue
		}
		var session eb.SessionResponse
		if err := json.Unmarshal(snapshot.DocumentJSON, &session); err != nil {
			return session, fmt.Errorf("decode installed session identity: %w", err)
		}
		return session, nil
	}
	return eb.SessionResponse{}, fmt.Errorf("%w: saved session identity missing", domain.ErrBankAuthorizationInvalid)
}

func savedAccountBinding(
	connection domain.BankConnection,
	mapping domain.ConnectionProviderAccount,
	snapshots []domain.ProviderSnapshot,
) (domain.BankAccountBinding, error) {
	for _, snapshot := range snapshots {
		if snapshot.TenantID != connection.TenantID || snapshot.Subject != domain.ProviderSnapshotSubjectAccount ||
			snapshot.Kind != domain.ProviderSnapshotKindAccount ||
			snapshot.FinanceAccountID != mapping.FinanceAccountID ||
			snapshot.ProviderObjectID != mapping.ProviderAccountID {
			continue
		}
		var account eb.Account
		if err := json.Unmarshal(snapshot.DocumentJSON, &account); err != nil {
			return domain.BankAccountBinding{}, fmt.Errorf("decode saved account identity: %w", err)
		}
		if account.UID == nil || *account.UID != mapping.ProviderAccountID || account.Currency != mapping.Currency {
			return domain.BankAccountBinding{}, fmt.Errorf(
				"%w: conflicting saved account identity",
				domain.ErrBankAuthorizationInvalid,
			)
		}
		return domain.BankAccountBinding{
			CanonicalID: mapping.ProviderAccountID,
			RemoteID:    mapping.ProviderAccountID,
			PrimaryHash: account.IdentificationHash,
			Currency:    mapping.Currency,
		}, nil
	}
	return domain.BankAccountBinding{}, fmt.Errorf(
		"%w: saved account identity missing",
		domain.ErrBankAuthorizationInvalid,
	)
}

func (s *bankAuthorizationService) ClearReplay(
	ctx context.Context,
	connection domain.BankConnection,
	window domain.ProviderSyncWindow,
) error {
	return s.saveReplayProgress(ctx, connection, window, true)
}

func (s *bankAuthorizationService) AdvanceReplay(
	ctx context.Context,
	connection domain.BankConnection,
	window domain.ProviderSyncWindow,
) error {
	return s.saveReplayProgress(ctx, connection, window, false)
}

func (s *bankAuthorizationService) saveReplayProgress(
	ctx context.Context,
	connection domain.BankConnection,
	window domain.ProviderSyncWindow,
	complete bool,
) error {
	return s.persistence.WithConnection(
		ctx,
		bankConnectionRef(connection),
		func(tx *persistence.BankAuthorizationTransaction) error {
			if tx.Connection.State != domain.BankConnectionStateActive {
				return domain.ErrBankReauthRequired
			}
			secret, err := tx.GetConnectionSecret(ctx, tx.Connection.SecretID)
			if err != nil {
				return err
			}
			authorization, err := decodeBankAuthorization(s.cipher, *secret)
			if err != nil || authorization == nil {
				return err
			}
			for i, binding := range authorization.Accounts {
				if !binding.Selected || binding.ReplayFrom == nil || binding.ReplayFrom.Before(window.Start) ||
					binding.ReplayFrom.After(window.End) {
					continue
				}
				// Only contiguous committed coverage can advance a retained floor.
				authorization.Accounts[i].ReplayFrom = &window.End
				if complete {
					authorization.Accounts[i].ReplayFrom = nil
				}
			}
			document, err := json.Marshal(authorization)
			if err != nil {
				return fmt.Errorf("encode completed replay: %w", err)
			}
			envelope, err := s.cipher.SealString(string(document))
			if err != nil {
				return fmt.Errorf("seal completed replay: %w", err)
			}
			return tx.SaveSecretEnvelope(ctx, envelope, s.now())
		},
	)
}

func (s *bankAuthorizationService) prepareReplacement(
	ctx context.Context,
	tx *persistence.BankAuthorizationTransaction,
	result providers.LinkResult,
) (domain.ConnectionSecret, bool, error) {
	current, err := s.loadAuthorization(ctx, tx)
	if err != nil {
		return domain.ConnectionSecret{}, false, err
	}
	now := s.now()
	state, err := persistence.NewProviderSyncStateJournalStore(tx.Store).
		LoadLastState(ctx, bankConnectionRef(tx.Connection))
	if err != nil {
		return domain.ConnectionSecret{}, false, err
	}
	window, err := providers.NewCheckpointTargetWindowPolicy().
		Determine(providers.TargetWindowRequest{Now: now, State: state})
	if err != nil {
		return domain.ConnectionSecret{}, false, err
	}
	merged, replay, err := current.Merge(*result.Authorization, now, window.Start)
	if err != nil {
		return domain.ConnectionSecret{}, false, err
	}
	document, err := json.Marshal(merged)
	if err != nil { // coverage-ignore // Application authorization has only serializable fields and valid policy timestamps.
		return domain.ConnectionSecret{}, false, fmt.Errorf("encode replacement authorization: %w", err)
	}
	secret, err := s.writer.PrepareConnectionSecret(tx.Connection.Provider, result.ProviderReference, string(document))
	// A replacement can fence an in-flight replay from the preceding update.
	// Keep its outstanding work durably queued on the new authorization too.
	return secret, replay || merged.ReplayStart() != nil, err
}

func (s *bankAuthorizationService) publishHistorySync(
	ctx context.Context,
	tx *persistence.BankAuthorizationTransaction,
	start domain.PendingBankConnectionLinkStart,
) error {
	if s.publisher == nil {
		return errors.New("bank link update transactional publisher is required")
	}
	command, err := newSemanticCommand(
		BankConnectionSyncCommandTopic,
		BankConnectionSyncCommand{
			ConnectionID: tx.Connection.ID,
			Reason:       "bank_link_updated",
			Requester:    CommandRequester{UserID: start.ActorUserID, Source: CommandRequesterSourceOperator},
		},
		"bank-link-update:"+start.ID,
	)
	if err != nil { // coverage-ignore // Concrete bank sync command JSON encoding cannot fail.
		return err
	}
	sqlTx, err := tx.SQLTransaction()
	if err != nil { // coverage-ignore // WithConnection supplies the underlying SQL transaction.
		return err
	}
	reference, publishErr := s.publisher.PublishSemanticCommandInTx(ctx, sqlTx, command)
	if publishErr != nil {
		return fmt.Errorf("publish bank link history sync: %w", publishErr)
	}
	tx.Connection.LastSyncJobID = reference.MessageID
	if _, saveErr := tx.SaveBankConnection(ctx, tx.Connection); saveErr != nil {
		return fmt.Errorf("save bank link history job: %w", saveErr)
	}
	return nil
}
