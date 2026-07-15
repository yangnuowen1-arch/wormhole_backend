package repository

import (
	"context"
	"errors"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrAnnouncementNotFound 公告不存在。
var ErrAnnouncementNotFound = errors.New("announcement not found")

// AnnouncementListFilter 公告列表查询条件。
type AnnouncementListFilter struct {
	Status      *int16
	VisibleOnly bool
}

// AnnouncementRepository 公告数据访问接口。
type AnnouncementRepository interface {
	List(ctx context.Context, filter AnnouncementListFilter) ([]model.AnnouncementRecord, error)
	FindByID(ctx context.Context, id int64) (*model.AnnouncementRecord, error)
	Create(ctx context.Context, announcement *model.AnnouncementRecord) error
	Update(ctx context.Context, id int64, updates map[string]any) (*model.AnnouncementRecord, error)
}

type announcementRepository struct {
	db *gorm.DB
}

// NewAnnouncementRepository 构造 AnnouncementRepository。
func NewAnnouncementRepository(db *gorm.DB) AnnouncementRepository {
	return &announcementRepository{db: db}
}

func (r *announcementRepository) List(ctx context.Context, filter AnnouncementListFilter) ([]model.AnnouncementRecord, error) {
	var announcements []model.AnnouncementRecord
	query := r.db.WithContext(ctx).Model(&model.AnnouncementRecord{})
	if filter.VisibleOnly {
		now := time.Now().UTC()
		query = query.
			Where("status = ?", 1).
			Where("published_at <= ?", now).
			Where("expires_at IS NULL OR expires_at > ?", now)
	} else if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	err := query.
		Order("is_pinned DESC, published_at DESC, id DESC").
		Find(&announcements).Error
	return announcements, err
}

func (r *announcementRepository) FindByID(ctx context.Context, id int64) (*model.AnnouncementRecord, error) {
	var announcement model.AnnouncementRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&announcement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAnnouncementNotFound
	}
	if err != nil {
		return nil, err
	}
	return &announcement, nil
}

func (r *announcementRepository) Create(ctx context.Context, announcement *model.AnnouncementRecord) error {
	return r.db.WithContext(ctx).Create(announcement).Error
}

func (r *announcementRepository) Update(ctx context.Context, id int64, updates map[string]any) (*model.AnnouncementRecord, error) {
	result := r.db.WithContext(ctx).Model(&model.AnnouncementRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrAnnouncementNotFound
	}
	return r.FindByID(ctx, id)
}
