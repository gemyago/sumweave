package sessions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/runtime/internal/gormsignalfoundry"
	"google.golang.org/adk/session"
	"google.golang.org/adk/session/database"
	"gorm.io/gorm"
)

// sessionMetadataModel is the GORM model for session metadata rows.
type sessionMetadataModel struct {
	SessionID string    `gorm:"column:session_id;primaryKey;size:255;index:idx_session_metadata_listing,priority:4"`
	AppName   string    `gorm:"column:app_name;size:255;not null;index:idx_session_metadata_listing,priority:1"`
	UserID    string    `gorm:"column:user_id;size:255;not null;index:idx_session_metadata_listing,priority:2"`
	Title     string    `gorm:"column:title;size:2048"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime:false;index:idx_session_metadata_listing,priority:3"`
}

func (sessionMetadataModel) TableName() string { return "session_metadata" }

func sessionMetadataToModel(m SessionMetadata) sessionMetadataModel {
	return sessionMetadataModel(m)
}

func sessionMetadataFromModel(m sessionMetadataModel) SessionMetadata {
	return SessionMetadata(m)
}

// DatabaseSessionMetadataStore persists session metadata in a relational database via GORM.
// Table names use the same prefix as other Signal Foundry-managed tables (see [gormsignalfoundry.GormSignalFoundryTablesOpts]).
type DatabaseSessionMetadataStore struct {
	db *gorm.DB
}

var _ SessionMetadataStore = (*DatabaseSessionMetadataStore)(nil)
var _ AutoMigratable = (*DatabaseSessionMetadataStore)(nil)

// NewDatabaseSessionMetadataStore opens a [DatabaseSessionMetadataStore] for the given DSN.
// opts configures GORM naming (table prefix) and error translation, consistent with
// [NewDatabaseSessionsStorage] and provider config DB services.
func NewDatabaseSessionMetadataStore(
	dsn string,
	opts gormsignalfoundry.GormSignalFoundryTablesOpts,
) (*DatabaseSessionMetadataStore, error) {
	if dsn == "" {
		return nil, errors.New("dsn is required")
	}
	opts.TranslateError = true
	gormCfg := gormsignalfoundry.NewGormConfigForSignalFoundryTables(opts)
	db, err := gorm.Open(gormsignalfoundry.NewGormDialector(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err = gormsignalfoundry.ApplySQLiteConnectionDefaults(db, dsn); err != nil {
		return nil, err
	}
	return &DatabaseSessionMetadataStore{db: db}, nil
}

// AutoMigrate creates or updates the session_metadata table schema.
func (s *DatabaseSessionMetadataStore) AutoMigrate() error {
	return s.db.AutoMigrate(&sessionMetadataModel{})
}

func (s *DatabaseSessionMetadataStore) Save(ctx context.Context, metadata SessionMetadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateMetadataForSave(metadata); err != nil {
		return err
	}
	row := sessionMetadataToModel(metadata)
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return fmt.Errorf("save session metadata: %w", err)
	}
	return nil
}

func (s *DatabaseSessionMetadataStore) List(
	ctx context.Context,
	params ListSessionMetadataParams,
) (*ListSessionMetadataResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateListParams(params); err != nil {
		return nil, err
	}

	var total int64
	countTx := s.db.WithContext(ctx).Model(&sessionMetadataModel{}).
		Where("app_name = ? AND user_id = ?", params.AppName, params.UserID)
	if err := countTx.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count session metadata: %w", err)
	}

	var rows []sessionMetadataModel
	findTx := s.db.WithContext(ctx).Where("app_name = ? AND user_id = ?", params.AppName, params.UserID).
		Order("updated_at DESC").
		Order("session_id DESC").
		Limit(params.Limit).
		Offset(params.Offset)
	if err := findTx.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list session metadata: %w", err)
	}

	out := make([]SessionMetadata, 0, len(rows))
	for i := range rows {
		out = append(out, sessionMetadataFromModel(rows[i]))
	}
	return &ListSessionMetadataResult{Sessions: out, Total: int(total)}, nil
}

func (s *DatabaseSessionMetadataStore) Delete(ctx context.Context, appName, userID, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if appName == "" || userID == "" {
		return errors.New("app_name and user_id are required")
	}
	if sessionID == "" {
		return errors.New("session_id is required")
	}
	res := s.db.WithContext(ctx).Where(
		"session_id = ? AND app_name = ? AND user_id = ?",
		sessionID, appName, userID,
	).Delete(&sessionMetadataModel{})
	if res.Error != nil {
		return fmt.Errorf("delete session metadata: %w", res.Error)
	}
	return nil
}

// DatabaseSessionsStorage unifies database-backed ADK session persistence and session metadata.
type DatabaseSessionsStorage struct {
	session.Service

	meta *DatabaseSessionMetadataStore
}

// NewDatabaseSessionsStorage returns concrete *DatabaseSessionsStorage.
func NewDatabaseSessionsStorage(
	dsn string,
	opts gormsignalfoundry.GormSignalFoundryTablesOpts,
) (*DatabaseSessionsStorage, error) {
	if dsn == "" {
		return nil, errors.New("dsn is required")
	}
	// database.NewSessionService returns [session.Service] only (no concrete type from ADK).
	svc, err := database.NewSessionService(
		gormsignalfoundry.NewGormDialector(dsn),
		gormsignalfoundry.NewGormConfigForSignalFoundryTables(opts),
	)
	if err != nil {
		return nil, fmt.Errorf("create ADK session service: %w", err)
	}
	meta, err := NewDatabaseSessionMetadataStore(dsn, opts)
	if err != nil {
		return nil, err
	}
	return &DatabaseSessionsStorage{
		Service: svc,
		meta:    meta,
	}, nil
}

func (s *DatabaseSessionsStorage) SaveMetadata(ctx context.Context, m SessionMetadata) error {
	return s.meta.Save(ctx, m)
}

func (s *DatabaseSessionsStorage) ListMetadata(
	ctx context.Context,
	p ListSessionMetadataParams,
) (*ListSessionMetadataResult, error) {
	return s.meta.List(ctx, p)
}

func (s *DatabaseSessionsStorage) DeleteMetadata(ctx context.Context, appName, userID, sessionID string) error {
	return s.meta.Delete(ctx, appName, userID, sessionID)
}

func (s *DatabaseSessionsStorage) AutoMigrate() error {
	if err := database.AutoMigrate(s.Service); err != nil {
		return fmt.Errorf("adk session migrate: %w", err)
	}
	return s.meta.AutoMigrate()
}

var _ SessionsStorage = (*DatabaseSessionsStorage)(nil)
var _ session.Service = (*DatabaseSessionsStorage)(nil)
var _ AutoMigratable = (*DatabaseSessionsStorage)(nil)
