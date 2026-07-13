package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100

	defaultSearchHistoryLimit = 4
	maxSearchHistoryLimit     = 20
	maxSearchQueryLength      = 128
)

var (
	// ErrResourceNotFound 资源不存在。
	ErrResourceNotFound = errors.New("resource not found")
	// ErrInvalidSearchQuery 搜索词为空或不合法。
	ErrInvalidSearchQuery = errors.New("invalid search query")
)

// ResourceListOptions 资源列表查询参数。
type ResourceListOptions struct {
	CategoryID   *int32
	CategoryCode string
	Featured     *bool
	Page         int
	PageSize     int
}

// ResourcePage 资源分页结果。
type ResourcePage struct {
	Items    []dto.ResourceResponse
	Total    int64
	Page     int
	PageSize int
}

// ResourceService 资源中心业务接口。
type ResourceService interface {
	ListCategories(ctx context.Context) ([]dto.ResourceCategoryResponse, error)
	ListResources(ctx context.Context, options ResourceListOptions) (ResourcePage, error)
	SearchResources(ctx context.Context, query string, page, pageSize int) (ResourcePage, error)
	GetResource(ctx context.Context, identifier string) (dto.ResourceResponse, error)
}

type resourceService struct {
	repo repository.ResourceRepository
}

// NewResourceService 构造 ResourceService。
func NewResourceService(repo repository.ResourceRepository) ResourceService {
	return &resourceService{repo: repo}
}

func (s *resourceService) ListCategories(ctx context.Context) ([]dto.ResourceCategoryResponse, error) {
	categories, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ResourceCategoryResponse, 0, len(categories))
	for _, category := range categories {
		resp = append(resp, toResourceCategoryResponse(category))
	}
	return resp, nil
}

func (s *resourceService) ListResources(ctx context.Context, options ResourceListOptions) (ResourcePage, error) {
	page, pageSize := normalizePagination(options.Page, options.PageSize)
	records, total, err := s.repo.ListResources(ctx, repository.ResourceFilter{
		CategoryID:   options.CategoryID,
		CategoryCode: strings.TrimSpace(options.CategoryCode),
		Featured:     options.Featured,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		return ResourcePage{}, err
	}
	return ResourcePage{
		Items:    toResourceResponses(records),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *resourceService) SearchResources(ctx context.Context, query string, page, pageSize int) (ResourcePage, error) {
	query = normalizeSearchQuery(query)
	if query == "" {
		return ResourcePage{}, ErrInvalidSearchQuery
	}

	page, pageSize = normalizePagination(page, pageSize)
	records, total, err := s.repo.ListResources(ctx, repository.ResourceFilter{
		Keyword:  query,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return ResourcePage{}, err
	}
	return ResourcePage{
		Items:    toResourceResponses(records),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *resourceService) GetResource(ctx context.Context, identifier string) (dto.ResourceResponse, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return dto.ResourceResponse{}, ErrResourceNotFound
	}

	var (
		record repository.ResourceRecord
		err    error
	)
	if id, parseErr := strconv.ParseInt(identifier, 10, 64); parseErr == nil && id > 0 {
		record, err = s.repo.FindResourceByID(ctx, id)
	} else {
		record, err = s.repo.FindResourceBySlug(ctx, identifier)
	}
	if errors.Is(err, repository.ErrResourceNotFound) {
		return dto.ResourceResponse{}, ErrResourceNotFound
	}
	if err != nil {
		return dto.ResourceResponse{}, err
	}
	return toResourceResponse(record), nil
}

// SearchHistoryService 搜索历史业务接口。
type SearchHistoryService interface {
	Record(ctx context.Context, req dto.RecordSearchHistoryRequest) (dto.SearchHistoryResponse, error)
	ListRecent(ctx context.Context, limit int) ([]dto.SearchHistoryResponse, error)
	Clear(ctx context.Context) (int64, error)
}

type searchHistoryService struct {
	repo repository.SearchHistoryRepository
}

// NewSearchHistoryService 构造 SearchHistoryService。
func NewSearchHistoryService(repo repository.SearchHistoryRepository) SearchHistoryService {
	return &searchHistoryService{repo: repo}
}

func (s *searchHistoryService) Record(ctx context.Context, req dto.RecordSearchHistoryRequest) (dto.SearchHistoryResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return dto.SearchHistoryResponse{}, err
	}
	query := normalizeSearchQuery(req.Query)
	if query == "" {
		return dto.SearchHistoryResponse{}, ErrInvalidSearchQuery
	}
	lastResultCount := req.LastResultCount
	if lastResultCount < 0 {
		lastResultCount = 0
	}

	history, err := s.repo.Upsert(ctx, userID, query, lastResultCount)
	if err != nil {
		return dto.SearchHistoryResponse{}, err
	}
	return toSearchHistoryResponse(*history), nil
}

func (s *searchHistoryService) ListRecent(ctx context.Context, limit int) ([]dto.SearchHistoryResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit = normalizeSearchHistoryLimit(limit)
	histories, err := s.repo.ListRecent(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.SearchHistoryResponse, 0, len(histories))
	for _, history := range histories {
		resp = append(resp, toSearchHistoryResponse(history))
	}
	return resp, nil
}

func (s *searchHistoryService) Clear(ctx context.Context) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}
	return s.repo.Clear(ctx, userID)
}

func toResourceCategoryResponse(category model.ResourceCategory) dto.ResourceCategoryResponse {
	return dto.ResourceCategoryResponse{
		ID:          category.ID,
		Code:        category.Code,
		Name:        category.Name,
		Description: derefStr(category.Description),
		SortOrder:   category.SortOrder,
	}
}

func toResourceResponses(records []repository.ResourceRecord) []dto.ResourceResponse {
	resp := make([]dto.ResourceResponse, 0, len(records))
	for _, record := range records {
		resp = append(resp, toResourceResponse(record))
	}
	return resp
}

func toResourceResponse(record repository.ResourceRecord) dto.ResourceResponse {
	var category *dto.ResourceCategorySummary
	if record.CategoryID != nil {
		category = &dto.ResourceCategorySummary{
			ID:   *record.CategoryID,
			Code: derefStr(record.CategoryCode),
			Name: derefStr(record.CategoryName),
		}
	}
	return dto.ResourceResponse{
		ID:            record.ID,
		Category:      category,
		Slug:          record.Slug,
		Name:          record.Name,
		IconURL:       derefStr(record.IconURL),
		IconText:      derefStr(record.IconText),
		WebsiteURL:    derefStr(record.WebsiteURL),
		Summary:       derefStr(record.Summary),
		Description:   derefStr(record.Description),
		ResourceType:  derefStr(record.ResourceType),
		Provider:      derefStr(record.Provider),
		ModelCount:    record.ModelCount,
		FollowerCount: record.FollowerCount,
		Badge:         derefStr(record.Badge),
		Tags:          jsonStringArray(record.Tags),
		Metadata:      jsonObject(record.Metadata),
		IsFeatured:    record.IsFeatured,
		SortOrder:     record.SortOrder,
	}
}

func toSearchHistoryResponse(history model.SearchHistory) dto.SearchHistoryResponse {
	return dto.SearchHistoryResponse{
		ID:              history.ID,
		Query:           history.Query,
		SearchCount:     derefInt32(history.SearchCount),
		LastResultCount: history.LastResultCount,
		LastSearchedAt:  formatTime(history.LastSearchedAt),
	}
}

func normalizePagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = defaultPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func normalizeSearchHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchHistoryLimit
	}
	if limit > maxSearchHistoryLimit {
		return maxSearchHistoryLimit
	}
	return limit
}

func normalizeSearchQuery(query string) string {
	query = strings.TrimSpace(query)
	if len([]rune(query)) > maxSearchQueryLength {
		query = string([]rune(query)[:maxSearchQueryLength])
	}
	return query
}

func jsonStringArray(raw *string) []string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(*raw), &values); err != nil {
		return []string{}
	}
	return values
}

func jsonObject(raw *string) map[string]any {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return map[string]any{}
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(*raw), &values); err != nil {
		return map[string]any{}
	}
	return values
}

func derefInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func formatTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
