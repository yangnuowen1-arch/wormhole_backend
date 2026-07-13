package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrQuickEntryNotFound 快速入口不存在。
var ErrQuickEntryNotFound = errors.New("quick entry not found")

// QuickEntryListFilter 快速入口列表查询条件。
type QuickEntryListFilter struct {
	RoleCodes []string
	Status    *int16
}

// QuickEntrySortItem 快速入口排序项。
type QuickEntrySortItem struct {
	ID        int32
	SortOrder int32
}

// QuickEntryRepository 快速入口数据访问接口。
type QuickEntryRepository interface {
	List(ctx context.Context, filter QuickEntryListFilter) ([]model.QuickEntry, error)
	FindByID(ctx context.Context, id int32) (*model.QuickEntry, error)
	Create(ctx context.Context, entry *model.QuickEntry) error
	Update(ctx context.Context, id int32, updates map[string]any) (*model.QuickEntry, error)
	UpdateSortOrders(ctx context.Context, items []QuickEntrySortItem, updatedBy int64) error
}

type quickEntryRepository struct {
	db *gorm.DB
}

// NewQuickEntryRepository 构造 QuickEntryRepository。
func NewQuickEntryRepository(db *gorm.DB) QuickEntryRepository {
	return &quickEntryRepository{db: db}
}

func (r *quickEntryRepository) List(ctx context.Context, filter QuickEntryListFilter) ([]model.QuickEntry, error) {
	var entries []model.QuickEntry
	query := r.db.WithContext(ctx).Model(&model.QuickEntry{})
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.RoleCodes != nil {
		condition, args := visibleRoleCondition(filter.RoleCodes)
		query = query.Where(condition, args...)
	}
	err := query.Order("sort_order ASC, id ASC").Find(&entries).Error
	return entries, err
}

func (r *quickEntryRepository) FindByID(ctx context.Context, id int32) (*model.QuickEntry, error) {
	var entry model.QuickEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrQuickEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *quickEntryRepository) Create(ctx context.Context, entry *model.QuickEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *quickEntryRepository) Update(ctx context.Context, id int32, updates map[string]any) (*model.QuickEntry, error) {
	result := r.db.WithContext(ctx).Model(&model.QuickEntry{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrQuickEntryNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *quickEntryRepository) UpdateSortOrders(ctx context.Context, items []QuickEntrySortItem, updatedBy int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Model(&model.QuickEntry{}).
				Where("id = ?", item.ID).
				Updates(map[string]any{
					"sort_order": item.SortOrder,
					"updated_by": updatedBy,
					"updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
}

func visibleRoleCondition(roleCodes []string) (string, []any) {
	parts := []string{"jsonb_array_length(visible_role_codes) = 0"}
	args := make([]any, 0, len(roleCodes))
	seen := make(map[string]struct{}, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleCode = strings.TrimSpace(roleCode)
		if roleCode == "" {
			continue
		}
		if _, ok := seen[roleCode]; ok {
			continue
		}
		seen[roleCode] = struct{}{}
		payload, _ := json.Marshal([]string{roleCode})
		parts = append(parts, "visible_role_codes @> ?::jsonb")
		args = append(args, string(payload))
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
