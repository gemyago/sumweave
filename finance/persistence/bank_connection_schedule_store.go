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

// BankConnectionScheduleStore owns persistence operations for authoritative
// bank-connection sync schedule state.
type BankConnectionScheduleStore struct {
	db *gorm.DB
}

// BankConnectionScheduleTransaction is the narrow transaction seam required
// to atomically advance finance schedule state and publish a durable command.
type BankConnectionScheduleTransaction struct {
	db    *gorm.DB
	sqlTx *sql.Tx
}

func NewBankConnectionScheduleStore(database *Database) *BankConnectionScheduleStore {
	return &BankConnectionScheduleStore{db: database.db}
}

func (s *BankConnectionScheduleStore) Save(
	ctx context.Context,
	schedule domain.BankConnectionSchedule,
) error {
	return saveBankConnectionSchedule(ctx, s.db, schedule)
}

func (s *BankConnectionScheduleStore) Get(
	ctx context.Context,
	connectionID string,
) (*domain.BankConnectionSchedule, error) {
	return getBankConnectionSchedule(ctx, s.db, connectionID)
}

// ListDue returns enabled schedules whose persisted occurrence is due.
func (s *BankConnectionScheduleStore) ListDue(
	ctx context.Context,
	now time.Time,
) ([]domain.BankConnectionSchedule, error) {
	var models []bankConnectionScheduleModel
	if err := s.db.WithContext(ctx).
		Table((bankConnectionScheduleModel{}).TableName()).
		Where(columnEnabled+" = ? AND "+columnNextRunAt+" IS NOT NULL AND "+columnNextRunAt+" <= ?", true, now).
		Order(columnNextRunAt + " ASC, connection_id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list due bank connection schedules: %w", err)
	}
	items := make([]domain.BankConnectionSchedule, 0, len(models))
	for _, model := range models {
		items = append(items, bankConnectionScheduleFromModel(model))
	}
	return items, nil
}

// WithTransaction provides the shared application SQL transaction to the
// app-side publisher while keeping schedule state operations finance-owned.
func (s *BankConnectionScheduleStore) WithTransaction(
	ctx context.Context,
	fn func(*BankConnectionScheduleTransaction) error,
) error {
	transaction := s.db.WithContext(ctx).Begin()
	if transaction.Error != nil {
		return fmt.Errorf("begin bank connection schedule transaction: %w", transaction.Error)
	}
	defer func() { _ = transaction.Rollback().Error }()

	sqlTx, ok := transaction.Statement.ConnPool.(*sql.Tx)
	if !ok {
		return errors.New("bank connection schedule transaction is not a SQL transaction")
	}
	if err := fn(&BankConnectionScheduleTransaction{db: transaction, sqlTx: sqlTx}); err != nil {
		return err
	}
	if err := transaction.Commit().Error; err != nil {
		return fmt.Errorf("commit bank connection schedule transaction: %w", err)
	}
	return nil
}

// SQLTransaction returns the application transaction for a collaborating
// durable publisher. Finance does not depend on the publisher implementation.
func (t *BankConnectionScheduleTransaction) SQLTransaction() *sql.Tx {
	return t.sqlTx
}

// Get returns schedule state from the transaction that will claim its due
// occurrence.
func (t *BankConnectionScheduleTransaction) Get(
	ctx context.Context,
	connectionID string,
) (*domain.BankConnectionSchedule, error) {
	return getBankConnectionSchedule(ctx, t.db, connectionID)
}

// ClaimDue advances one expected due occurrence before its command is
// published. The conditional update makes a stale scheduler a no-op.
func (t *BankConnectionScheduleTransaction) ClaimDue(
	ctx context.Context,
	connectionID string,
	dueAt time.Time,
	nextRunAt time.Time,
	now time.Time,
) (bool, error) {
	result := t.db.WithContext(ctx).
		Table((bankConnectionScheduleModel{}).TableName()).
		Where(columnConnectionID+" = ? AND "+columnEnabled+" = ? AND "+columnNextRunAt+" = ? AND "+columnNextRunAt+" <= ?", connectionID, true, dueAt, now).
		Updates(map[string]any{columnNextRunAt: nextRunAt, columnUpdatedAt: now})
	if result.Error != nil {
		return false, fmt.Errorf("claim due bank connection schedule: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// FinalizeClaim records the command reference only for the occurrence claimed
// in this transaction.
func (t *BankConnectionScheduleTransaction) FinalizeClaim(
	ctx context.Context,
	connectionID string,
	dueAt time.Time,
	nextRunAt time.Time,
	messageID string,
	now time.Time,
) error {
	result := t.db.WithContext(ctx).
		Table((bankConnectionScheduleModel{}).TableName()).
		Where(columnConnectionID+" = ? AND "+columnEnabled+" = ? AND "+columnNextRunAt+" = ?", connectionID, true, nextRunAt).
		Updates(map[string]any{
			columnLastScheduledAt: dueAt,
			columnLastJobID:       messageID,
			columnUpdatedAt:       now,
		})
	if result.Error != nil {
		return fmt.Errorf("finalize claimed bank connection schedule: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.New("claimed bank connection schedule changed before finalization")
	}
	return nil
}

func (t *BankConnectionScheduleTransaction) Save(
	ctx context.Context,
	schedule domain.BankConnectionSchedule,
) error {
	return saveBankConnectionSchedule(ctx, t.db, schedule)
}

func saveBankConnectionSchedule(
	ctx context.Context,
	db *gorm.DB,
	schedule domain.BankConnectionSchedule,
) error {
	model := newBankConnectionScheduleModel(schedule)
	if err := db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: columnConnectionID}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnIntervalSeconds, columnNextRunAt, columnLastScheduledAt, "last_started_at",
				"last_completed_at", columnLastJobID, columnEnabled, columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return fmt.Errorf("save bank connection schedule: %w", err)
	}
	return nil
}

func getBankConnectionSchedule(
	ctx context.Context,
	db *gorm.DB,
	connectionID string,
) (*domain.BankConnectionSchedule, error) {
	var model bankConnectionScheduleModel
	if err := db.WithContext(ctx).
		Table(model.TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBankConnectionScheduleNotFound
		}
		return nil, fmt.Errorf("get bank connection schedule: %w", err)
	}
	schedule := bankConnectionScheduleFromModel(model)
	return &schedule, nil
}
