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
	// ErrResourceCategoryNotFound 资源分类不存在。
	ErrResourceCategoryNotFound = errors.New("resource category not found")
	// ErrInvalidResource 资源参数不合法。
	ErrInvalidResource = errors.New("invalid resource")
	// ErrInvalidResourceCategory 资源分类参数不合法。
	ErrInvalidResourceCategory = errors.New("invalid resource category")
	// ErrInvalidSearchQuery 搜索词为空或不合法。
	ErrInvalidSearchQuery = errors.New("invalid search query")
)

// ResourceListOptions 资源列表查询参数。
type ResourceListOptions struct {
	CategoryID   *int32
	CategoryCode string
	Featured     *bool
	Status       *int16
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
	AdminListCategories(ctx context.Context, status *int16) ([]dto.ResourceCategoryResponse, error)
	CreateCategory(ctx context.Context, req dto.CreateResourceCategoryRequest) (dto.ResourceCategoryResponse, error)
	UpdateCategory(ctx context.Context, id int32, req dto.UpdateResourceCategoryRequest) (dto.ResourceCategoryResponse, error)
	SortCategories(ctx context.Context, req dto.SortResourceCategoriesRequest) error
	UpdateCategoryStatus(ctx context.Context, id int32, status int16) (dto.ResourceCategoryResponse, error)
	DeleteCategory(ctx context.Context, id int32) error
	ListResources(ctx context.Context, options ResourceListOptions) (ResourcePage, error)
	AdminListResources(ctx context.Context, options ResourceListOptions) (ResourcePage, error)
	CreateResource(ctx context.Context, req dto.CreateResourceRequest) (dto.ResourceResponse, error)
	UpdateResource(ctx context.Context, id int64, req dto.UpdateResourceRequest) (dto.ResourceResponse, error)
	SortResources(ctx context.Context, req dto.SortResourcesRequest) error
	UpdateResourceStatus(ctx context.Context, id int64, status int16) (dto.ResourceResponse, error)
	DeleteResource(ctx context.Context, id int64) error
	SearchResources(ctx context.Context, query string, page, pageSize int) (ResourcePage, error)
	GetResource(ctx context.Context, identifier string) (dto.ResourceResponse, error)
}

type resourceService struct {
	repo     repository.ResourceRepository
	userRepo UserRoleFinder
}

// NewResourceService 构造 ResourceService。
func NewResourceService(repo repository.ResourceRepository, userRepo ...UserRoleFinder) ResourceService {
	svc := &resourceService{repo: repo}
	if len(userRepo) > 0 {
		svc.userRepo = userRepo[0]
	}
	return svc
}

func (s *resourceService) ListCategories(ctx context.Context) ([]dto.ResourceCategoryResponse, error) {
	categories, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	return toResourceCategoryResponses(categories), nil
}

func (s *resourceService) AdminListCategories(ctx context.Context, status *int16) ([]dto.ResourceCategoryResponse, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if status != nil && !isValidStatus(*status) {
		return nil, ErrInvalidStatus
	}
	categories, err := s.repo.AdminListCategories(ctx, status)
	if err != nil {
		return nil, err
	}
	return toResourceCategoryResponses(categories), nil
}

func (s *resourceService) CreateCategory(ctx context.Context, req dto.CreateResourceCategoryRequest) (dto.ResourceCategoryResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.ResourceCategoryResponse{}, err
	}
	status := statusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if !isValidStatus(status) {
		return dto.ResourceCategoryResponse{}, ErrInvalidStatus
	}
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return dto.ResourceCategoryResponse{}, ErrInvalidResourceCategory
	}
	now := time.Now().UTC()
	category := &model.ResourceCategory{
		Code:        code,
		Name:        name,
		Description: optionalStr(strings.TrimSpace(req.Description)),
		SortOrder:   req.SortOrder,
		Status:      &status,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return dto.ResourceCategoryResponse{}, err
	}
	return toResourceCategoryResponse(*category), nil
}

func (s *resourceService) UpdateCategory(ctx context.Context, id int32, req dto.UpdateResourceCategoryRequest) (dto.ResourceCategoryResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.ResourceCategoryResponse{}, err
	}
	if id <= 0 {
		return dto.ResourceCategoryResponse{}, ErrResourceCategoryNotFound
	}
	updates := map[string]any{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return dto.ResourceCategoryResponse{}, ErrInvalidResourceCategory
		}
		updates["code"] = code
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return dto.ResourceCategoryResponse{}, ErrInvalidResourceCategory
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = optionalStr(strings.TrimSpace(*req.Description))
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.ResourceCategoryResponse{}, ErrInvalidStatus
		}
		updates["status"] = *req.Status
	}
	category, err := s.repo.UpdateCategory(ctx, id, updates)
	if errors.Is(err, repository.ErrResourceCategoryNotFound) {
		return dto.ResourceCategoryResponse{}, ErrResourceCategoryNotFound
	}
	if err != nil {
		return dto.ResourceCategoryResponse{}, err
	}
	return toResourceCategoryResponse(*category), nil
}

func (s *resourceService) SortCategories(ctx context.Context, req dto.SortResourceCategoriesRequest) error {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return err
	}
	items := make([]repository.ResourceCategorySortItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ID <= 0 {
			continue
		}
		items = append(items, repository.ResourceCategorySortItem{
			ID:        item.ID,
			SortOrder: item.SortOrder,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return s.repo.UpdateCategorySortOrders(ctx, items, userID)
}

func (s *resourceService) UpdateCategoryStatus(ctx context.Context, id int32, status int16) (dto.ResourceCategoryResponse, error) {
	if !isValidStatus(status) {
		return dto.ResourceCategoryResponse{}, ErrInvalidStatus
	}
	return s.UpdateCategory(ctx, id, dto.UpdateResourceCategoryRequest{Status: &status})
}

// DeleteCategory 删除资源分类。关联资源会由数据库外键自动保留并解除分类。
func (s *resourceService) DeleteCategory(ctx context.Context, id int32) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	if id <= 0 {
		return ErrResourceCategoryNotFound
	}
	if err := s.repo.DeleteCategory(ctx, id); errors.Is(err, repository.ErrResourceCategoryNotFound) {
		return ErrResourceCategoryNotFound
	} else {
		return err
	}
}

func toResourceCategoryResponses(categories []model.ResourceCategory) []dto.ResourceCategoryResponse {
	resp := make([]dto.ResourceCategoryResponse, 0, len(categories))
	for _, category := range categories {
		resp = append(resp, toResourceCategoryResponse(category))
	}
	return resp
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

func (s *resourceService) AdminListResources(ctx context.Context, options ResourceListOptions) (ResourcePage, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return ResourcePage{}, err
	}
	if options.Status != nil && !isValidStatus(*options.Status) {
		return ResourcePage{}, ErrInvalidStatus
	}
	page, pageSize := normalizePagination(options.Page, options.PageSize)
	records, total, err := s.repo.AdminListResources(ctx, repository.ResourceFilter{
		CategoryID:   options.CategoryID,
		CategoryCode: strings.TrimSpace(options.CategoryCode),
		Featured:     options.Featured,
		Status:       options.Status,
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

func (s *resourceService) CreateResource(ctx context.Context, req dto.CreateResourceRequest) (dto.ResourceResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.ResourceResponse{}, err
	}
	status := statusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if !isValidStatus(status) {
		return dto.ResourceResponse{}, ErrInvalidStatus
	}
	categoryID, err := s.normalizedCategoryID(ctx, req.CategoryID)
	if err != nil {
		return dto.ResourceResponse{}, err
	}
	slug := strings.TrimSpace(req.Slug)
	name := strings.TrimSpace(req.Name)
	resourceType := strings.TrimSpace(req.ResourceType)
	if resourceType == "" {
		resourceType = "tool"
	}
	if slug == "" || name == "" || req.ModelCount < 0 || req.FollowerCount < 0 {
		return dto.ResourceResponse{}, ErrInvalidResource
	}
	tags := stringArrayJSONValue(normalizeStringList(req.Tags))
	metadata, err := jsonObjectString(req.Metadata)
	if err != nil {
		return dto.ResourceResponse{}, ErrInvalidResource
	}
	now := time.Now().UTC()
	resource := &model.Resource{
		CategoryID:    categoryID,
		Slug:          slug,
		Name:          name,
		IconURL:       optionalStr(strings.TrimSpace(req.IconURL)),
		IconText:      optionalStr(strings.TrimSpace(req.IconText)),
		WebsiteURL:    optionalStr(strings.TrimSpace(req.WebsiteURL)),
		Summary:       optionalStr(strings.TrimSpace(req.Summary)),
		Description:   optionalStr(strings.TrimSpace(req.Description)),
		ResourceType:  &resourceType,
		Provider:      optionalStr(strings.TrimSpace(req.Provider)),
		ModelCount:    req.ModelCount,
		FollowerCount: req.FollowerCount,
		Badge:         optionalStr(strings.TrimSpace(req.Badge)),
		Tags:          &tags,
		Metadata:      &metadata,
		IsFeatured:    req.IsFeatured,
		SortOrder:     req.SortOrder,
		Status:        &status,
		CreatedBy:     &userID,
		UpdatedBy:     &userID,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := s.repo.CreateResource(ctx, resource); err != nil {
		return dto.ResourceResponse{}, err
	}
	record, err := s.repo.FindResourceByIDAnyStatus(ctx, resource.ID)
	if err != nil {
		return dto.ResourceResponse{}, err
	}
	return toResourceResponse(record), nil
}

func (s *resourceService) UpdateResource(ctx context.Context, id int64, req dto.UpdateResourceRequest) (dto.ResourceResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.ResourceResponse{}, err
	}
	if id <= 0 {
		return dto.ResourceResponse{}, ErrResourceNotFound
	}
	updates := map[string]any{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.CategoryID != nil {
		categoryID, err := s.normalizedCategoryID(ctx, req.CategoryID)
		if err != nil {
			return dto.ResourceResponse{}, err
		}
		updates["category_id"] = categoryID
	}
	if req.Slug != nil {
		slug := strings.TrimSpace(*req.Slug)
		if slug == "" {
			return dto.ResourceResponse{}, ErrInvalidResource
		}
		updates["slug"] = slug
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return dto.ResourceResponse{}, ErrInvalidResource
		}
		updates["name"] = name
	}
	if req.IconURL != nil {
		updates["icon_url"] = optionalStr(strings.TrimSpace(*req.IconURL))
	}
	if req.IconText != nil {
		updates["icon_text"] = optionalStr(strings.TrimSpace(*req.IconText))
	}
	if req.WebsiteURL != nil {
		updates["website_url"] = optionalStr(strings.TrimSpace(*req.WebsiteURL))
	}
	if req.Summary != nil {
		updates["summary"] = optionalStr(strings.TrimSpace(*req.Summary))
	}
	if req.Description != nil {
		updates["description"] = optionalStr(strings.TrimSpace(*req.Description))
	}
	if req.ResourceType != nil {
		resourceType := strings.TrimSpace(*req.ResourceType)
		if resourceType == "" {
			return dto.ResourceResponse{}, ErrInvalidResource
		}
		updates["resource_type"] = resourceType
	}
	if req.Provider != nil {
		updates["provider"] = optionalStr(strings.TrimSpace(*req.Provider))
	}
	if req.ModelCount != nil {
		if *req.ModelCount < 0 {
			return dto.ResourceResponse{}, ErrInvalidResource
		}
		updates["model_count"] = *req.ModelCount
	}
	if req.FollowerCount != nil {
		if *req.FollowerCount < 0 {
			return dto.ResourceResponse{}, ErrInvalidResource
		}
		updates["follower_count"] = *req.FollowerCount
	}
	if req.Badge != nil {
		updates["badge"] = optionalStr(strings.TrimSpace(*req.Badge))
	}
	if req.Tags != nil {
		updates["tags"] = stringArrayJSONValue(normalizeStringList(req.Tags))
	}
	if req.Metadata != nil {
		metadata, err := jsonObjectString(*req.Metadata)
		if err != nil {
			return dto.ResourceResponse{}, ErrInvalidResource
		}
		updates["metadata"] = metadata
	}
	if req.IsFeatured != nil {
		updates["is_featured"] = *req.IsFeatured
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.ResourceResponse{}, ErrInvalidStatus
		}
		updates["status"] = *req.Status
	}
	record, err := s.repo.UpdateResource(ctx, id, updates)
	if errors.Is(err, repository.ErrResourceNotFound) {
		return dto.ResourceResponse{}, ErrResourceNotFound
	}
	if err != nil {
		return dto.ResourceResponse{}, err
	}
	return toResourceResponse(record), nil
}

func (s *resourceService) SortResources(ctx context.Context, req dto.SortResourcesRequest) error {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return err
	}
	items := make([]repository.ResourceSortItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ID <= 0 {
			continue
		}
		items = append(items, repository.ResourceSortItem{
			ID:        item.ID,
			SortOrder: item.SortOrder,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return s.repo.UpdateResourceSortOrders(ctx, items, userID)
}

func (s *resourceService) UpdateResourceStatus(ctx context.Context, id int64, status int16) (dto.ResourceResponse, error) {
	if !isValidStatus(status) {
		return dto.ResourceResponse{}, ErrInvalidStatus
	}
	return s.UpdateResource(ctx, id, dto.UpdateResourceRequest{Status: &status})
}

// DeleteResource 删除资源。关联常用工具会级联删除，推荐项会由数据库外键解除资源关联。
func (s *resourceService) DeleteResource(ctx context.Context, id int64) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	if id <= 0 {
		return ErrResourceNotFound
	}
	if err := s.repo.DeleteResource(ctx, id); errors.Is(err, repository.ErrResourceNotFound) {
		return ErrResourceNotFound
	} else {
		return err
	}
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
		Status:      derefInt16(category.Status),
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
		Status:        derefInt16(record.Status),
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

func (s *resourceService) requireAdmin(ctx context.Context) (int64, error) {
	if s.userRepo == nil {
		return 0, ErrForbidden
	}
	return requireAdminRole(ctx, s.userRepo)
}

func (s *resourceService) normalizedCategoryID(ctx context.Context, categoryID *int32) (*int32, error) {
	if categoryID == nil || *categoryID <= 0 {
		return nil, nil
	}
	if _, err := s.repo.FindCategoryByID(ctx, *categoryID); err != nil {
		if errors.Is(err, repository.ErrResourceCategoryNotFound) {
			return nil, ErrResourceCategoryNotFound
		}
		return nil, err
	}
	return categoryID, nil
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func jsonObjectString(values map[string]any) (string, error) {
	if values == nil {
		return "{}", nil
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
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
