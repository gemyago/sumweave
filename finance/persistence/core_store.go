package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	columnName              = "name"
	columnHiddenAt          = "hidden_at"
	columnArchivedAt        = "archived_at"
	columnKind              = "kind"
	columnTenantID          = "tenant_id"
	columnCurrency          = "currency"
	columnUpdatedAt         = "updated_at"
	columnProvider          = "provider"
	columnProviderReference = "provider_reference"
	columnState             = "state"
	columnConnectionID      = "connection_id"
	columnProviderAccountID = "provider_account_id"
	columnEffectiveAt       = "effective_at"
	columnFingerprint       = "fingerprint"
	columnScope             = "scope"
	columnProviderObjectID  = "provider_object_id"
	columnPayloadJSON       = "payload_json"
	columnCapturedAt        = "captured_at"
	columnNonce             = "nonce"
	columnCiphertext        = "ciphertext"
)

var (
	ErrTenantInviteNotFound = errors.New("tenant invite not found")
	ErrAccountNotFound      = errors.New("account not found")
	ErrCategoryNotFound     = errors.New("category not found")
	ErrTagNotFound          = errors.New("tag not found")
	ErrTransactionNotFound  = errors.New("transaction not found")
)

func (s *Store) SaveTenant(ctx context.Context, tenant domain.Tenant) (domain.Tenant, error) {
	model := newTenantModel(tenant)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnName,
				"display_currency",
				columnArchivedAt,
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.Tenant{}, fmt.Errorf("save tenant: %w", err)
	}
	return tenantFromModel(model), nil
}

func (s *Store) GetTenant(ctx context.Context, tenantID string) (*domain.Tenant, error) {
	var model tenantModel
	err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", tenantID).
		First(&model).
		Error
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	tenant := tenantFromModel(model)
	return &tenant, nil
}

func (s *Store) SaveTenantMembership(
	ctx context.Context,
	membership domain.TenantMembership,
) (domain.TenantMembership, error) {
	model := newTenantMembershipModel(membership)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model).Error; err != nil {
		return domain.TenantMembership{}, fmt.Errorf("save tenant membership: %w", err)
	}
	return tenantMembershipFromModel(model), nil
}

func (s *Store) ListTenantsForUser(
	ctx context.Context,
	userID string,
) ([]domain.TenantMembershipView, error) {
	var rows []struct {
		TenantID          string
		Name              string
		DisplayCurrency   string
		TenantArchivedAt  *time.Time
		TenantCreatedAt   time.Time
		TenantUpdatedAt   time.Time
		MembershipJoined  time.Time
		MembershipCreated time.Time
	}
	err := s.db.WithContext(ctx).
		Table("finance_tenant_memberships m").
		Select(
			"t.id AS tenant_id, t.name, t.display_currency, t.archived_at AS tenant_archived_at, "+
				"t.created_at AS tenant_created_at, t.updated_at AS tenant_updated_at, "+
				"m.joined_at AS membership_joined, m.created_at AS membership_created",
		).
		Joins("JOIN finance_tenants t ON t.id = m.tenant_id").
		Where("m.user_id = ?", userID).
		Where("t.archived_at IS NULL").
		Order("t.created_at ASC").
		Order("t.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list tenants for user: %w", err)
	}
	views := make([]domain.TenantMembershipView, 0, len(rows))
	for _, row := range rows {
		views = append(views, domain.TenantMembershipView{
			Tenant: domain.Tenant{
				ID:              row.TenantID,
				Name:            row.Name,
				DisplayCurrency: row.DisplayCurrency,
				CreatedAt:       row.TenantCreatedAt,
				UpdatedAt:       row.TenantUpdatedAt,
			},
			Membership: domain.TenantMembership{
				TenantID:  row.TenantID,
				UserID:    userID,
				JoinedAt:  row.MembershipJoined,
				CreatedAt: row.MembershipCreated,
			},
		})
	}
	return views, nil
}

func (s *Store) IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Table((tenantMembershipModel{}).TableName()).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check tenant membership: %w", err)
	}
	return count > 0, nil
}

func (s *Store) SaveTenantInvite(
	ctx context.Context,
	invite domain.TenantInvite,
) (domain.TenantInvite, error) {
	model := newTenantInviteModel(invite)
	if err := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return domain.TenantInvite{}, fmt.Errorf("save tenant invite: %w", err)
	}
	return tenantInviteFromModel(model), nil
}

func (s *Store) GetTenantInviteByCode(
	ctx context.Context,
	code string,
) (*domain.TenantInvite, error) {
	var model tenantInviteModel
	err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("code = ?", code).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantInviteNotFound
		}
		return nil, fmt.Errorf("get tenant invite by code: %w", err)
	}
	invite := tenantInviteFromModel(model)
	return &invite, nil
}

func (s *Store) UpdateTenantInvite(
	ctx context.Context,
	invite domain.TenantInvite,
) (domain.TenantInvite, error) {
	model := newTenantInviteModel(invite)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", model.ID).
		Updates(map[string]any{
			"accepted_by_user_id": model.AcceptedByUserID,
			"accepted_at":         model.AcceptedAt,
		}).Error; err != nil {
		return domain.TenantInvite{}, fmt.Errorf("update tenant invite: %w", err)
	}
	return tenantInviteFromModel(model), nil
}

func (s *Store) ListTenantInvites(
	ctx context.Context,
	tenantID string,
) ([]domain.TenantInvite, error) {
	var models []tenantInviteModel
	if err := s.db.WithContext(ctx).
		Table((tenantInviteModel{}).TableName()).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC, id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list tenant invites: %w", err)
	}
	items := make([]domain.TenantInvite, 0, len(models))
	for _, model := range models {
		items = append(items, tenantInviteFromModel(model))
	}
	return items, nil
}

func (s *Store) ListTenantMembers(
	ctx context.Context,
	tenantID string,
) ([]domain.TenantMember, error) {
	var models []tenantMembershipModel
	if err := s.db.WithContext(ctx).
		Table((tenantMembershipModel{}).TableName()).
		Where("tenant_id = ?", tenantID).
		Order("joined_at ASC, user_id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list tenant members: %w", err)
	}
	members := make([]domain.TenantMember, 0, len(models))
	for _, model := range models {
		members = append(members, domain.TenantMember{
			TenantID: model.TenantID,
			UserID:   model.UserID,
			JoinedAt: model.JoinedAt,
		})
	}
	return members, nil
}

func (s *Store) SaveAccount(ctx context.Context, account domain.Account) (domain.Account, error) {
	model := newAccountModel(account)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnTenantID,
				columnName,
				columnCurrency,
				columnKind,
				columnProvider,
				columnProviderAccountID,
				columnHiddenAt,
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.Account{}, fmt.Errorf("save account: %w", err)
	}
	return accountFromModel(model), nil
}

func (s *Store) GetAccount(ctx context.Context, accountID string) (*domain.Account, error) {
	var model accountModel
	err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", accountID).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	account := accountFromModel(model)
	return &account, nil
}

func (s *Store) ListAccounts(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Account, error) {
	var models []accountModel
	query := s.db.WithContext(ctx).
		Table((accountModel{}).TableName()).
		Where("tenant_id = ?", tenantID)
	if !includeHidden {
		query = query.Where("hidden_at IS NULL")
	}
	if err := query.Order("created_at ASC, id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	items := make([]domain.Account, 0, len(models))
	for _, model := range models {
		items = append(items, accountFromModel(model))
	}
	return items, nil
}

func (s *Store) SaveCategory(
	ctx context.Context,
	category domain.Category,
) (domain.Category, error) {
	model := newCategoryModel(category)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnTenantID,
				columnName,
				columnKind,
				"seeded_default",
				columnHiddenAt,
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.Category{}, fmt.Errorf("save category: %w", err)
	}
	return categoryFromModel(model), nil
}

func (s *Store) GetCategory(ctx context.Context, categoryID string) (*domain.Category, error) {
	var model categoryModel
	err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", categoryID).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	category := categoryFromModel(model)
	return &category, nil
}

func (s *Store) ListCategories(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Category, error) {
	var models []categoryModel
	query := s.db.WithContext(ctx).
		Table((categoryModel{}).TableName()).
		Where("tenant_id = ?", tenantID)
	if !includeHidden {
		query = query.Where("hidden_at IS NULL")
	}
	if err := query.Order("seeded_default DESC, created_at ASC, id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	items := make([]domain.Category, 0, len(models))
	for _, model := range models {
		items = append(items, categoryFromModel(model))
	}
	return items, nil
}

func (s *Store) SaveTag(ctx context.Context, tag domain.Tag) (domain.Tag, error) {
	model := newTagModel(tag)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnTenantID,
				columnName,
				columnHiddenAt,
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.Tag{}, fmt.Errorf("save tag: %w", err)
	}
	return tagFromModel(model), nil
}

func (s *Store) GetTag(ctx context.Context, tagID string) (*domain.Tag, error) {
	var model tagModel
	err := s.db.WithContext(ctx).Table(model.TableName()).Where("id = ?", tagID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTagNotFound
		}
		return nil, fmt.Errorf("get tag: %w", err)
	}
	tag := tagFromModel(model)
	return &tag, nil
}

func (s *Store) ListTags(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Tag, error) {
	var models []tagModel
	query := s.db.WithContext(ctx).Table((tagModel{}).TableName()).Where("tenant_id = ?", tenantID)
	if !includeHidden {
		query = query.Where("hidden_at IS NULL")
	}
	if err := query.Order("created_at ASC, id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	items := make([]domain.Tag, 0, len(models))
	for _, model := range models {
		items = append(items, tagFromModel(model))
	}
	return items, nil
}

func (s *Store) SaveTransaction(
	ctx context.Context,
	transaction domain.Transaction,
) (domain.Transaction, error) {
	model := newTransactionModel(transaction)
	if err := s.saveTransactionWithDB(s.db.WithContext(ctx), model); err != nil {
		return domain.Transaction{}, fmt.Errorf("save transaction: %w", err)
	}
	return transactionFromModel(model), nil
}

func (s *Store) SaveLinkedTransferPair(
	ctx context.Context,
	firstTransaction domain.Transaction,
	secondTransaction domain.Transaction,
) error {
	firstModel := newTransactionModel(firstTransaction)
	secondModel := newTransactionModel(secondTransaction)
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.saveTransactionWithDB(tx, firstModel); err != nil {
			return err
		}
		if err := s.saveTransactionWithDB(tx, secondModel); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("save linked transfer pair: %w", err)
	}
	return nil
}

func (s *Store) saveTransactionWithDB(db *gorm.DB, model transactionModel) error {
	return saveTransactionModel(db, model)
}

func (s *Store) GetTransaction(
	ctx context.Context,
	transactionID string,
) (*domain.Transaction, error) {
	var model transactionModel
	err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", transactionID).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	transaction := transactionFromModel(model)
	return &transaction, nil
}

func (s *Store) ListTransactions(
	ctx context.Context,
	tenantID string,
	accountID string,
	source domain.TransactionSource,
	status domain.TransactionStatus,
	includeHidden bool,
	page ...ListTransactionsPage,
) ([]domain.Transaction, error) {
	var models []transactionModel
	query := s.db.WithContext(ctx).
		Table((transactionModel{}).TableName()).
		Where("tenant_id = ?", tenantID)
	if accountID != "" {
		query = query.Where("account_id = ?", accountID)
	}
	if source != "" {
		query = query.Where("source = ?", string(source))
	}
	if status != "" {
		query = query.Where("status = ?", string(status))
	}
	if !includeHidden {
		query = query.Where("hidden_at IS NULL")
	}
	if len(page) > 0 {
		if page[0].Limit > 0 {
			query = query.Limit(dbPageInt(page[0].Limit))
		}
		if page[0].Offset > 0 {
			query = query.Offset(dbPageInt(page[0].Offset))
		}
	}
	if err := query.Order("effective_at DESC, created_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	items := make([]domain.Transaction, 0, len(models))
	for _, model := range models {
		items = append(items, transactionFromModel(model))
	}
	return items, nil
}

type ListTransactionsPage struct {
	Limit  int64
	Offset int64
}

func dbPageInt(value int64) int {
	maxInt := int64(int(^uint(0) >> 1))
	if value > maxInt {
		return int(maxInt)
	}
	return int(value)
}
