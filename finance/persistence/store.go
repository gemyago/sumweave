package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrConnectionSecretNotFound = errors.New("connection secret not found")

var ErrCSVImportNotFound = errors.New("csv import not found")

type Store struct {
	db  *gorm.DB
	now func() time.Time
}

func NewStore(database *Database) *Store {
	return newStore(database.db, time.Now)
}

func (s *Store) WithTransaction(ctx context.Context, fn func(*Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(newStore(tx, s.now))
	})
}

func newStore(db *gorm.DB, now func() time.Time) *Store {
	return &Store{
		db:  db,
		now: now,
	}
}

func (s *Store) DeleteConnectionSecret(ctx context.Context, secretID string) error {
	if err := s.db.WithContext(ctx).
		Table((connectionSecretModel{}).TableName()).
		Where("id = ?", strings.TrimSpace(secretID)).
		Delete(&connectionSecretModel{}).Error; err != nil {
		return fmt.Errorf("delete connection secret: %w", err)
	}
	return nil
}

func (s *Store) SaveConnectionSecret(
	ctx context.Context,
	secret domain.ConnectionSecret,
) (domain.ConnectionSecret, error) {
	model := newConnectionSecretModel(secret)
	if model.CreatedAt.IsZero() {
		model.CreatedAt = s.now()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnProvider,
				"reference",
				"key_version",
				"algorithm",
				"nonce",
				"ciphertext",
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.ConnectionSecret{}, fmt.Errorf("save connection secret: %w", err)
	}
	return connectionSecretFromModel(model), nil
}

func (s *Store) GetConnectionSecret(
	ctx context.Context,
	secretID string,
) (*domain.ConnectionSecret, error) {
	var model connectionSecretModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", strings.TrimSpace(secretID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConnectionSecretNotFound
		}
		return nil, fmt.Errorf("get connection secret: %w", err)
	}
	secret := connectionSecretFromModel(model)
	return &secret, nil
}

func (s *Store) CreateFixtureBootstrapRun(
	ctx context.Context,
	run domain.FixtureBootstrapRun,
) error {
	model := newFixtureBootstrapRunModel(run)
	if model.StartedAt.IsZero() {
		model.StartedAt = s.now()
	}
	if err := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return fmt.Errorf("create fixture bootstrap run: %w", err)
	}
	return nil
}

func (s *Store) CreateFixtureScenarioRecord(
	ctx context.Context,
	runID string,
	record domain.FixtureScenarioRecord,
) error {
	model := newFixtureScenarioRecordModel(runID, record)
	if model.OccurredAt.IsZero() {
		model.OccurredAt = s.now()
	}
	if err := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return fmt.Errorf("create fixture scenario record: %w", err)
	}
	return nil
}

func (s *Store) SaveCSVImport(
	ctx context.Context,
	record domain.CSVImportRecord,
) (domain.CSVImportRecord, error) {
	model := newCSVImportModel(record)
	if model.CreatedAt.IsZero() {
		model.CreatedAt = s.now()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(&model).Error; err != nil {
		return domain.CSVImportRecord{}, fmt.Errorf("save csv import: %w", err)
	}
	return csvImportFromModel(model), nil
}

func (s *Store) GetCSVImport(ctx context.Context, importID string) (*domain.CSVImportRecord, error) {
	var model csvImportModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", strings.TrimSpace(importID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCSVImportNotFound
		}
		return nil, fmt.Errorf("get csv import: %w", err)
	}
	record := csvImportFromModel(model)
	return &record, nil
}
