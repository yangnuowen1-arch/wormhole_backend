package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrResourceNotFound 资源不存在。
var ErrResourceNotFound = errors.New("resource not found")

// ErrResourceCategoryNotFound 资源分类不存在。
var ErrResourceCategoryNotFound = errors.New("resource category not found")

// ResourceFilter 资源列表查询条件。
type ResourceFilter struct {
	CategoryID   *int32
	CategoryCode string
	Keyword      string
	Featured     *bool
	Status       *int16
	AllStatuses  bool
	Page         int
	PageSize     int
}

// ResourceCategorySortItem 资源分类排序项。
type ResourceCategorySortItem struct {
	ID        int32
	SortOrder int32
}

// ResourceSortItem 资源排序项。
type ResourceSortItem struct {
	ID        int64
	SortOrder int32
}

// ResourceRecord 是资源与分类的扁平查询结果。
type ResourceRecord struct {
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
}

// ResourceRepository 资源中心数据访问接口。
type ResourceRepository interface {
	ListCategories(ctx context.Context) ([]model.ResourceCategory, error)
	AdminListCategories(ctx context.Context, status *int16) ([]model.ResourceCategory, error)
	FindCategoryByID(ctx context.Context, id int32) (*model.ResourceCategory, error)
	CreateCategory(ctx context.Context, category *model.ResourceCategory) error
	UpdateCategory(ctx context.Context, id int32, updates map[string]any) (*model.ResourceCategory, error)
	UpdateCategorySortOrders(ctx context.Context, items []ResourceCategorySortItem, updatedBy int64) error
	DeleteCategory(ctx context.Context, id int32) error
	ListResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error)
	AdminListResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error)
	FindResourceByID(ctx context.Context, id int64) (ResourceRecord, error)
	FindResourceByIDAnyStatus(ctx context.Context, id int64) (ResourceRecord, error)
	FindResourceBySlug(ctx context.Context, slug string) (ResourceRecord, error)
	CreateResource(ctx context.Context, resource *model.Resource) error
	UpdateResource(ctx context.Context, id int64, updates map[string]any) (ResourceRecord, error)
	UpdateResourceSortOrders(ctx context.Context, items []ResourceSortItem, updatedBy int64) error
	DeleteResource(ctx context.Context, id int64) error
}

type resourceRepository struct {
	db *gorm.DB
}

// NewResourceRepository 构造 ResourceRepository。
func NewResourceRepository(db *gorm.DB) ResourceRepository {
	return &resourceRepository{db: db}
}

func (r *resourceRepository) ListCategories(ctx context.Context) ([]model.ResourceCategory, error) {
	status := int16(1)
	return r.AdminListCategories(ctx, &status)
}

func (r *resourceRepository) AdminListCategories(ctx context.Context, status *int16) ([]model.ResourceCategory, error) {
	var categories []model.ResourceCategory
	query := r.db.WithContext(ctx).Model(&model.ResourceCategory{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	err := query.Order("sort_order ASC, id ASC").Find(&categories).Error
	return categories, err
}

func (r *resourceRepository) FindCategoryByID(ctx context.Context, id int32) (*model.ResourceCategory, error) {
	var category model.ResourceCategory
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&category).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrResourceCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *resourceRepository) CreateCategory(ctx context.Context, category *model.ResourceCategory) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *resourceRepository) UpdateCategory(ctx context.Context, id int32, updates map[string]any) (*model.ResourceCategory, error) {
	result := r.db.WithContext(ctx).Model(&model.ResourceCategory{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrResourceCategoryNotFound
	}
	return r.FindCategoryByID(ctx, id)
}

func (r *resourceRepository) UpdateCategorySortOrders(ctx context.Context, items []ResourceCategorySortItem, updatedBy int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Model(&model.ResourceCategory{}).
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

func (r *resourceRepository) DeleteCategory(ctx context.Context, id int32) error {
	result := r.db.WithContext(ctx).Delete(&model.ResourceCategory{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrResourceCategoryNotFound
	}
	return nil
}

func (r *resourceRepository) ListResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error) {
	filter.AllStatuses = false
	status := int16(1)
	filter.Status = &status
	return r.listResources(ctx, filter)
}

func (r *resourceRepository) AdminListResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error) {
	filter.AllStatuses = true
	return r.listResources(ctx, filter)
}

func (r *resourceRepository) listResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error) {
	var total int64
	if err := r.resourceQuery(ctx, filter).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []ResourceRecord
	err := r.resourceQuery(ctx, filter).
		Select(resourceSelectColumns()).
		Order("r.sort_order ASC, r.id ASC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *resourceRepository) FindResourceByID(ctx context.Context, id int64) (ResourceRecord, error) {
	status := int16(1)
	var record ResourceRecord
	err := r.resourceQuery(ctx, ResourceFilter{Status: &status}).
		Select(resourceSelectColumns()).
		Where("r.id = ?", id).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResourceRecord{}, ErrResourceNotFound
	}
	return record, err
}

func (r *resourceRepository) FindResourceByIDAnyStatus(ctx context.Context, id int64) (ResourceRecord, error) {
	var record ResourceRecord
	err := r.resourceQuery(ctx, ResourceFilter{AllStatuses: true}).
		Select(resourceSelectColumns()).
		Where("r.id = ?", id).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResourceRecord{}, ErrResourceNotFound
	}
	return record, err
}

func (r *resourceRepository) FindResourceBySlug(ctx context.Context, slug string) (ResourceRecord, error) {
	status := int16(1)
	var record ResourceRecord
	err := r.resourceQuery(ctx, ResourceFilter{Status: &status}).
		Select(resourceSelectColumns()).
		Where("r.slug = ?", slug).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResourceRecord{}, ErrResourceNotFound
	}
	return record, err
}

func (r *resourceRepository) CreateResource(ctx context.Context, resource *model.Resource) error {
	return r.db.WithContext(ctx).Create(resource).Error
}

func (r *resourceRepository) UpdateResource(ctx context.Context, id int64, updates map[string]any) (ResourceRecord, error) {
	result := r.db.WithContext(ctx).Model(&model.Resource{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return ResourceRecord{}, result.Error
	}
	if result.RowsAffected == 0 {
		return ResourceRecord{}, ErrResourceNotFound
	}
	return r.FindResourceByIDAnyStatus(ctx, id)
}

func (r *resourceRepository) UpdateResourceSortOrders(ctx context.Context, items []ResourceSortItem, updatedBy int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Model(&model.Resource{}).
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

func (r *resourceRepository) DeleteResource(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&model.Resource{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrResourceNotFound
	}
	return nil
}

func (r *resourceRepository) resourceQuery(ctx context.Context, filter ResourceFilter) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table(model.TableNameResource + " AS r").
		Joins("LEFT JOIN " + model.TableNameResourceCategory + " AS c ON c.id = r.category_id")

	if filter.AllStatuses {
		if filter.Status != nil {
			query = query.Where("r.status = ?", *filter.Status)
		}
	} else {
		query = query.Where("r.status = ?", 1)
	}
	if filter.CategoryID != nil {
		query = query.Where("r.category_id = ?", *filter.CategoryID)
	}
	if code := strings.TrimSpace(filter.CategoryCode); code != "" && code != "all" {
		query = query.Where("c.code = ?", code)
	}
	if filter.Featured != nil {
		query = query.Where("r.is_featured = ?", *filter.Featured)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			`to_tsvector('simple', coalesce(r.name, '') || ' ' || coalesce(r.summary, '') || ' ' || coalesce(r.provider, '') || ' ' || coalesce(r.description, '')) @@ plainto_tsquery('simple', ?)
			 OR r.name ILIKE ?
			 OR r.summary ILIKE ?
			 OR r.provider ILIKE ?`,
			keyword, like, like, like,
		)
	}
	return query
}

func resourceSelectColumns() string {
	return `
		r.id,
		r.category_id,
		c.code AS category_code,
		c.name AS category_name,
		r.slug,
		r.name,
		r.icon_url,
		r.icon_text,
		r.website_url,
		r.summary,
		r.description,
		r.resource_type,
		r.provider,
		r.model_count,
		r.follower_count,
		r.badge,
		r.tags,
		r.metadata,
		r.is_featured,
		r.sort_order,
		r.status`
}
