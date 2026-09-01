package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FXRefreshScheduleStore owns persistence for finance's FX refresh due state.
type FXRefreshScheduleStore struct {
	db *gorm.DB
}

// FXRefreshScheduleTransaction is the narrow transaction seam used to atomically
// publish a scheduled command and advance its finance-owned due state.
type FXRefreshScheduleTransaction struct {
	db    *gorm.DB
	sqlTx *sql.Tx
}

func NewFXRefreshScheduleStore(database *Database) *FXRefreshScheduleStore {
	return &FXRefreshScheduleStore{db: database.db}
}

func (s *FXRefreshScheduleStore) Save(ctx context.Context, schedule domain.FXRefreshSchedule) error {
	return saveFXRefreshSchedule(ctx, s.db, schedule)
}

func (s *FXRefreshScheduleStore) Get(ctx context.Context, scheduleID string) (*domain.FXRefreshSchedule, error) {
	var model fxRefreshScheduleModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("schedule_id = ?", strings.TrimSpace(scheduleID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFXRefreshScheduleNotFound
		}
		return nil, fmt.Errorf("get fx refresh schedule: %w", err)
	}
	schedule := fxRefreshScheduleFromModel(model)
	return &schedule, nil
}

// ListDue returns enabled FX refresh schedule occurrences that are due.
func (s *FXRefreshScheduleStore) ListDue(
	ctx context.Context,
	now time.Time,
) ([]domain.FXRefreshSchedule, error) {
	var models []fxRefreshScheduleModel
	if err := s.db.WithContext(ctx).
		Table((fxRefreshScheduleModel{}).TableName()).
		Where(columnEnabled+" = ? AND "+columnNextRunAt+" IS NOT NULL AND "+columnNextRunAt+" <= ?", true, now).
		Order(columnNextRunAt + " ASC, schedule_id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list due fx refresh schedules: %w", err)
	}
	items := make([]domain.FXRefreshSchedule, 0, len(models))
	for _, model := range models {
		items = append(items, fxRefreshScheduleFromModel(model))
	}
	return items, nil
}

// Ensure creates the initial schedule without changing a committed occurrence.
func (s *FXRefreshScheduleStore) Ensure(ctx context.Context, schedule domain.FXRefreshSchedule) error {
	model := newFXRefreshScheduleModel(schedule)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model).Error; err != nil {
		return fmt.Errorf("ensure fx refresh schedule: %w", err)
	}
	return nil
}

// WithTransaction exposes only the application SQL transaction required by a
// finance-owned scheduled-command publisher.
func (s *FXRefreshScheduleStore) WithTransaction(
	ctx context.Context,
	fn func(*FXRefreshScheduleTransaction) error,
) error {
	transaction := s.db.WithContext(ctx).Begin()
	if transaction.Error != nil {
		return fmt.Errorf("begin fx refresh schedule transaction: %w", transaction.Error)
	}
	defer func() { _ = transaction.Rollback().Error }()

	sqlTx, ok := transaction.Statement.ConnPool.(*sql.Tx)
	if !ok {
		return errors.New("fx refresh schedule transaction is not a SQL transaction")
	}
	if err := fn(&FXRefreshScheduleTransaction{db: transaction, sqlTx: sqlTx}); err != nil {
		return err
	}
	if err := transaction.Commit().Error; err != nil {
		return fmt.Errorf("commit fx refresh schedule transaction: %w", err)
	}
	return nil
}

// SQLTransaction returns the transaction used by the durable publisher.
func (t *FXRefreshScheduleTransaction) SQLTransaction() *sql.Tx { return t.sqlTx }

// Get returns schedule state from the transaction that will claim its due
// occurrence.
func (t *FXRefreshScheduleTransaction) Get(
	ctx context.Context,
	scheduleID string,
) (*domain.FXRefreshSchedule, error) {
	var model fxRefreshScheduleModel
	if err := t.db.WithContext(ctx).
		Table(model.TableName()).
		Where("schedule_id = ?", strings.TrimSpace(scheduleID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFXRefreshScheduleNotFound
		}
		return nil, fmt.Errorf("get fx refresh schedule: %w", err)
	}
	schedule := fxRefreshScheduleFromModel(model)
	return &schedule, nil
}

// ClaimDue advances one expected due occurrence before its command is
// published. The conditional update makes a stale scheduler a no-op.
func (t *FXRefreshScheduleTransaction) ClaimDue(
	ctx context.Context,
	scheduleID string,
	dueAt time.Time,
	nextRunAt time.Time,
	now time.Time,
) (bool, error) {
	result := t.db.WithContext(ctx).
		Table((fxRefreshScheduleModel{}).TableName()).
		Where("schedule_id = ? AND "+columnEnabled+" = ? AND "+columnNextRunAt+" = ? AND "+columnNextRunAt+" <= ?", scheduleID, true, dueAt, now).
		Updates(map[string]any{columnNextRunAt: nextRunAt, columnUpdatedAt: now})
	if result.Error != nil {
		return false, fmt.Errorf("claim due fx refresh schedule: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// FinalizeClaim records the command reference only for the occurrence claimed
// in this transaction.
func (t *FXRefreshScheduleTransaction) FinalizeClaim(
	ctx context.Context,
	scheduleID string,
	dueAt time.Time,
	nextRunAt time.Time,
	messageID string,
	now time.Time,
) error {
	result := t.db.WithContext(ctx).
		Table((fxRefreshScheduleModel{}).TableName()).
		Where("schedule_id = ? AND "+columnEnabled+" = ? AND "+columnNextRunAt+" = ?", scheduleID, true, nextRunAt).
		Updates(map[string]any{
			columnLastScheduledAt: dueAt,
			columnLastJobID:       messageID,
			columnUpdatedAt:       now,
		})
	if result.Error != nil {
		return fmt.Errorf("finalize claimed fx refresh schedule: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.New("claimed fx refresh schedule changed before finalization")
	}
	return nil
}

func (t *FXRefreshScheduleTransaction) Save(ctx context.Context, schedule domain.FXRefreshSchedule) error {
	return saveFXRefreshSchedule(ctx, t.db, schedule)
}

func saveFXRefreshSchedule(ctx context.Context, db *gorm.DB, schedule domain.FXRefreshSchedule) error {
	model := newFXRefreshScheduleModel(schedule)
	if err := db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "schedule_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"provider", columnIntervalSeconds, columnNextRunAt, columnLastScheduledAt, columnLastJobID,
				columnEnabled, columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return fmt.Errorf("save fx refresh schedule: %w", err)
	}
	return nil
}
