package repository

import (
	"context"
	"errors"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrRecommendationItemNotFound 今日推荐不存在。
var ErrRecommendationItemNotFound = errors.New("recommendation item not found")

// ErrCarouselSlideNotFound 幻灯片不存在。
var ErrCarouselSlideNotFound = errors.New("carousel slide not found")

// RecommendationItemListFilter 今日推荐列表查询条件。
type RecommendationItemListFilter struct {
	RoleCodes []string
	Status    *int16
}

// RecommendationItemSortItem 今日推荐排序项。
type RecommendationItemSortItem struct {
	ID        int64
	SortOrder int32
}

// RecommendationItemRepository 今日推荐数据访问接口。
type RecommendationItemRepository interface {
	List(ctx context.Context, filter RecommendationItemListFilter) ([]model.RecommendationItem, error)
	FindByID(ctx context.Context, id int64) (*model.RecommendationItem, error)
	Create(ctx context.Context, item *model.RecommendationItem) error
	Update(ctx context.Context, id int64, updates map[string]any) (*model.RecommendationItem, error)
	UpdateSortOrders(ctx context.Context, items []RecommendationItemSortItem, updatedBy int64) error
}

type recommendationItemRepository struct {
	db *gorm.DB
}

// NewRecommendationItemRepository 构造 RecommendationItemRepository。
func NewRecommendationItemRepository(db *gorm.DB) RecommendationItemRepository {
	return &recommendationItemRepository{db: db}
}

func (r *recommendationItemRepository) List(ctx context.Context, filter RecommendationItemListFilter) ([]model.RecommendationItem, error) {
	var items []model.RecommendationItem
	query := r.db.WithContext(ctx).Model(&model.RecommendationItem{})
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.RoleCodes != nil {
		condition, args := visibleRoleCondition(filter.RoleCodes)
		query = query.Where(condition, args...)
	}
	err := query.Order("sort_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *recommendationItemRepository) FindByID(ctx context.Context, id int64) (*model.RecommendationItem, error) {
	var item model.RecommendationItem
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRecommendationItemNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *recommendationItemRepository) Create(ctx context.Context, item *model.RecommendationItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *recommendationItemRepository) Update(ctx context.Context, id int64, updates map[string]any) (*model.RecommendationItem, error) {
	result := r.db.WithContext(ctx).Model(&model.RecommendationItem{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrRecommendationItemNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *recommendationItemRepository) UpdateSortOrders(ctx context.Context, items []RecommendationItemSortItem, updatedBy int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Model(&model.RecommendationItem{}).
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

// CarouselSlideListFilter 幻灯片列表查询条件。
type CarouselSlideListFilter struct {
	RoleCodes  []string
	Status     *int16
	ActiveOnly bool
}

// CarouselSlideSortItem 幻灯片排序项。
type CarouselSlideSortItem struct {
	ID        int64
	SortOrder int32
}

// CarouselSlideRepository 幻灯片数据访问接口。
type CarouselSlideRepository interface {
	List(ctx context.Context, filter CarouselSlideListFilter) ([]model.CarouselSlide, error)
	FindByID(ctx context.Context, id int64) (*model.CarouselSlide, error)
	Create(ctx context.Context, slide *model.CarouselSlide) error
	Update(ctx context.Context, id int64, updates map[string]any) (*model.CarouselSlide, error)
	UpdateSortOrders(ctx context.Context, items []CarouselSlideSortItem, updatedBy int64) error
}

type carouselSlideRepository struct {
	db *gorm.DB
}

// NewCarouselSlideRepository 构造 CarouselSlideRepository。
func NewCarouselSlideRepository(db *gorm.DB) CarouselSlideRepository {
	return &carouselSlideRepository{db: db}
}

func (r *carouselSlideRepository) List(ctx context.Context, filter CarouselSlideListFilter) ([]model.CarouselSlide, error) {
	var slides []model.CarouselSlide
	query := r.db.WithContext(ctx).Model(&model.CarouselSlide{})
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.ActiveOnly {
		now := time.Now().UTC()
		query = query.Where("(starts_at IS NULL OR starts_at <= ?) AND (ends_at IS NULL OR ends_at >= ?)", now, now)
	}
	if filter.RoleCodes != nil {
		condition, args := visibleRoleCondition(filter.RoleCodes)
		query = query.Where(condition, args...)
	}
	err := query.Order("sort_order ASC, id ASC").Find(&slides).Error
	return slides, err
}

func (r *carouselSlideRepository) FindByID(ctx context.Context, id int64) (*model.CarouselSlide, error) {
	var slide model.CarouselSlide
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&slide).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCarouselSlideNotFound
	}
	if err != nil {
		return nil, err
	}
	return &slide, nil
}

func (r *carouselSlideRepository) Create(ctx context.Context, slide *model.CarouselSlide) error {
	return r.db.WithContext(ctx).Create(slide).Error
}

func (r *carouselSlideRepository) Update(ctx context.Context, id int64, updates map[string]any) (*model.CarouselSlide, error) {
	result := r.db.WithContext(ctx).Model(&model.CarouselSlide{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrCarouselSlideNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *carouselSlideRepository) UpdateSortOrders(ctx context.Context, items []CarouselSlideSortItem, updatedBy int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Model(&model.CarouselSlide{}).
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
