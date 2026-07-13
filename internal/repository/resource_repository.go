package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrResourceNotFound 资源不存在。
var ErrResourceNotFound = errors.New("resource not found")

// ResourceFilter 资源列表查询条件。
type ResourceFilter struct {
	CategoryID   *int32
	CategoryCode string
	Keyword      string
	Featured     *bool
	Page         int
	PageSize     int
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
	ListResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error)
	FindResourceByID(ctx context.Context, id int64) (ResourceRecord, error)
	FindResourceBySlug(ctx context.Context, slug string) (ResourceRecord, error)
}

type resourceRepository struct {
	db *gorm.DB
}

// NewResourceRepository 构造 ResourceRepository。
func NewResourceRepository(db *gorm.DB) ResourceRepository {
	return &resourceRepository{db: db}
}

func (r *resourceRepository) ListCategories(ctx context.Context) ([]model.ResourceCategory, error) {
	var categories []model.ResourceCategory
	err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Order("sort_order ASC, id ASC").
		Find(&categories).Error
	return categories, err
}

func (r *resourceRepository) ListResources(ctx context.Context, filter ResourceFilter) ([]ResourceRecord, int64, error) {
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
	var record ResourceRecord
	err := r.resourceQuery(ctx, ResourceFilter{}).
		Select(resourceSelectColumns()).
		Where("r.id = ?", id).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResourceRecord{}, ErrResourceNotFound
	}
	return record, err
}

func (r *resourceRepository) FindResourceBySlug(ctx context.Context, slug string) (ResourceRecord, error) {
	var record ResourceRecord
	err := r.resourceQuery(ctx, ResourceFilter{}).
		Select(resourceSelectColumns()).
		Where("r.slug = ?", slug).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResourceRecord{}, ErrResourceNotFound
	}
	return record, err
}

func (r *resourceRepository) resourceQuery(ctx context.Context, filter ResourceFilter) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table(model.TableNameResource+" AS r").
		Joins("LEFT JOIN "+model.TableNameResourceCategory+" AS c ON c.id = r.category_id").
		Where("r.status = ?", 1)

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
