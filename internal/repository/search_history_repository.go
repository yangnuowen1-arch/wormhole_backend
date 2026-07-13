package repository

import (
	"context"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// SearchHistoryRepository 搜索历史数据访问接口。
type SearchHistoryRepository interface {
	Upsert(ctx context.Context, userID int64, query string, lastResultCount int32) (*model.SearchHistory, error)
	ListRecent(ctx context.Context, userID int64, limit int) ([]model.SearchHistory, error)
	Clear(ctx context.Context, userID int64) (int64, error)
}

type searchHistoryRepository struct {
	db *gorm.DB
}

// NewSearchHistoryRepository 构造 SearchHistoryRepository。
func NewSearchHistoryRepository(db *gorm.DB) SearchHistoryRepository {
	return &searchHistoryRepository{db: db}
}

func (r *searchHistoryRepository) Upsert(ctx context.Context, userID int64, query string, lastResultCount int32) (*model.SearchHistory, error) {
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Exec(`
INSERT INTO search_history (user_id, query, search_count, last_result_count, last_searched_at, created_at, updated_at)
VALUES (?, ?, 1, ?, ?, ?, ?)
ON CONFLICT (user_id, query) DO UPDATE SET
	search_count = search_history.search_count + 1,
	last_result_count = EXCLUDED.last_result_count,
	last_searched_at = EXCLUDED.last_searched_at,
	updated_at = EXCLUDED.updated_at`,
		userID, query, lastResultCount, now, now, now,
	).Error
	if err != nil {
		return nil, err
	}

	var history model.SearchHistory
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND query = ?", userID, query).
		First(&history).Error; err != nil {
		return nil, err
	}
	return &history, nil
}

func (r *searchHistoryRepository) ListRecent(ctx context.Context, userID int64, limit int) ([]model.SearchHistory, error) {
	var histories []model.SearchHistory
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_searched_at DESC, id DESC").
		Limit(limit).
		Find(&histories).Error
	return histories, err
}

func (r *searchHistoryRepository) Clear(ctx context.Context, userID int64) (int64, error) {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.SearchHistory{})
	return result.RowsAffected, result.Error
}
