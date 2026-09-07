package financeapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appevents"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
)

const BankSyncWindowCompletedEventTopic = "finance.bank-sync-window-completed.v1"
const ClassificationConsumerGroup = "finance.classification.v1"
const TransferMatchingConsumerGroup = "finance.transfer-matching.v1"

// BankSyncWindowCompletedEvent is the transport representation of the
// finance-owned committed-window fact.
type BankSyncWindowCompletedEvent struct {
	TenantID            string    `json:"tenantId"`
	ConnectionID        string    `json:"connectionId"`
	RangeStart          time.Time `json:"rangeStart"`
	RangeEndExclusive   time.Time `json:"rangeEndExclusive"`
	SourceSyncMessageID string    `json:"sourceSyncMessageId"`
}

// RegisterAutomaticTransferMatchingHandler registers a job-free, independent
// committed-window consumer for transfer matching.
func RegisterAutomaticTransferMatchingHandler(
	router *appdispatch.Router,
	service transferMatchingJobService,
	logger *slog.Logger,
) error { // coverage-ignore
	if service == nil {
		return nil
	}
	if logger == nil {
		return errors.New("automatic transfer matching logger is required")
	}
	return appevents.RegisterMessageHandler(
		router,
		BankSyncWindowCompletedEvent{},
		func(ctx context.Context, message appdispatch.Message, event BankSyncWindowCompletedEvent) error {
			_, err := service.Match(
				ctx,
				financepkg.TransferMatchingParams{
					TenantID:            event.TenantID,
					RangeStart:          event.RangeStart,
					RangeEndExclusive:   event.RangeEndExclusive,
					MessageID:           message.ID,
					SourceSyncMessageID: event.SourceSyncMessageID,
				},
			)
			if err != nil {
				logger.ErrorContext(
					ctx,
					"automatic transfer matching failed",
					"messageId",
					message.ID,
					"sourceSyncMessageId",
					event.SourceSyncMessageID,
					"connectionId",
					event.ConnectionID,
					"error",
					err,
				)
				return fmt.Errorf("match completed bank sync window: %w", err)
			}
			return nil
		},
	)
}

func (BankSyncWindowCompletedEvent) Topic() string { // coverage-ignore
	return BankSyncWindowCompletedEventTopic
}

type bankSyncWindowEventPublisher struct{ publisher *appevents.Publisher }

func (p bankSyncWindowEventPublisher) PublishBankSyncWindowCompleted(
	ctx context.Context,
	tx *sql.Tx,
	fact domain.BankSyncWindowCompleted,
) error { // coverage-ignore
	if err := p.publisher.PublishInTx(ctx, tx, BankSyncWindowCompletedEvent{
		TenantID: fact.TenantID, ConnectionID: fact.ConnectionID,
		RangeStart: fact.RangeStart, RangeEndExclusive: fact.RangeEndExclusive,
		SourceSyncMessageID: fact.SourceSyncMessageID,
	}); err != nil {
		return fmt.Errorf("publish bank sync window completed event: %w", err)
	}
	return nil
}

// RegisterAutomaticClassificationHandler registers an ordinary appdispatch
// consumer. Automatic classification deliberately has no jobs projection.
func RegisterAutomaticClassificationHandler(
	router *appdispatch.Router,
	service classificationJobService,
	logger *slog.Logger,
) error { // coverage-ignore
	if service == nil {
		return nil
	}
	if logger == nil {
		return errors.New("automatic classification logger is required")
	}
	return appevents.RegisterMessageHandler(
		router,
		BankSyncWindowCompletedEvent{},
		func(ctx context.Context, message appdispatch.Message, event BankSyncWindowCompletedEvent) error {
			_, err := service.Classify(ctx, financepkg.ClassificationParams{
				TenantID: event.TenantID, RangeStart: event.RangeStart,
				RangeEndExclusive: event.RangeEndExclusive, MessageID: message.ID,
				SourceSyncMessageID: event.SourceSyncMessageID,
			})
			if err != nil {
				logger.ErrorContext(ctx, "automatic transaction classification failed",
					"messageId", message.ID, "sourceSyncMessageId", event.SourceSyncMessageID, "error", err)
				return fmt.Errorf("classify completed bank sync window: %w", err)
			}
			return nil
		},
	)
}
