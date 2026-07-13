package repository

import (
	"context"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// CommonToolSortItem 常用工具排序项。
type CommonToolSortItem struct {
	ResourceID int64
	SortOrder  int32
}

// CommonToolRecord 是用户常用工具与资源详情的扁平查询结果。
type CommonToolRecord struct {
	ID            int64
	CategoryID    *int32
	CategoryCode  *string
	CategoryName  *string
	Slug          string
	Name          string
	IconURL       *string
	IconText      *string
	WebsiteURL    *string
	Summary       *string
	Description   *string
	ResourceType  *string
	Provider      *string
	ModelCount    int32
	FollowerCount int32
	Badge         *string
	Tags          *string
	Metadata      *string
	IsFeatured    bool
	SortOrder     int32
	Status        *int16

	CommonSortOrder int32
	StarredAt       *time.Time
}

// CommonToolRepository 用户常用工具数据访问接口。
type CommonToolRepository interface {
	Add(ctx context.Context, userID, resourceID int64, sortOrder int32) error
	Remove(ctx context.Context, userID, resourceID int64) (int64, error)
	List(ctx context.Context, userID int64) ([]CommonToolRecord, error)
	UpdateSortOrders(ctx context.Context, userID int64, items []CommonToolSortItem) error
}

type commonToolRepository struct {
	db *gorm.DB
}

// NewCommonToolRepository 构造 CommonToolRepository。
func NewCommonToolRepository(db *gorm.DB) CommonToolRepository {
	return &commonToolRepository{db: db}
}

func (r *commonToolRepository) Add(ctx context.Context, userID, resourceID int64, sortOrder int32) error {
	return r.db.WithContext(ctx).Exec(`
INSERT INTO user_common_tools (user_id, resource_id, sort_order, created_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, resource_id) DO UPDATE SET
	sort_order = EXCLUDED.sort_order`,
		userID, resourceID, sortOrder,
	).Error
}

func (r *commonToolRepository) Remove(ctx context.Context, userID, resourceID int64) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND resource_id = ?", userID, resourceID).
		Delete(&model.UserCommonTool{})
	return result.RowsAffected, result.Error
}

func (r *commonToolRepository) List(ctx context.Context, userID int64) ([]CommonToolRecord, error) {
	var records []CommonToolRecord
	err := r.db.WithContext(ctx).
		Table(model.TableNameUserCommonTool+" AS uct").
		Joins("JOIN "+model.TableNameResource+" AS r ON r.id = uct.resource_id").
		Joins("LEFT JOIN "+model.TableNameResourceCategory+" AS c ON c.id = r.category_id").
		Where("uct.user_id = ? AND r.status = ?", userID, 1).
		Select(commonToolSelectColumns()).
		Order("uct.sort_order ASC, uct.created_at DESC, r.id ASC").
		Scan(&records).Error
	return records, err
}

func (r *commonToolRepository) UpdateSortOrders(ctx context.Context, userID int64, items []CommonToolSortItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&model.UserCommonTool{}).
				Where("user_id = ? AND resource_id = ?", userID, item.ResourceID).
				Update("sort_order", item.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func commonToolSelectColumns() string {
	return resourceSelectColumns() + `,
		uct.sort_order AS common_sort_order,
		uct.created_at AS starred_at`
}
