package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

const defaultAutoplaySeconds int32 = 5

// RecommendationItemService 今日推荐业务接口。
type RecommendationItemService interface {
	ListVisible(ctx context.Context) ([]dto.RecommendationItemResponse, error)
	AdminList(ctx context.Context, status *int16) ([]dto.RecommendationItemResponse, error)
	Create(ctx context.Context, req dto.CreateRecommendationItemRequest) (dto.RecommendationItemResponse, error)
	Update(ctx context.Context, id int64, req dto.UpdateRecommendationItemRequest) (dto.RecommendationItemResponse, error)
	Sort(ctx context.Context, req dto.SortRecommendationItemsRequest) error
	UpdateStatus(ctx context.Context, id int64, status int16) (dto.RecommendationItemResponse, error)
}

type recommendationItemService struct {
	repo         repository.RecommendationItemRepository
	userRepo     repository.UserRepository
	resourceRepo repository.ResourceRepository
}

// NewRecommendationItemService 构造 RecommendationItemService。
func NewRecommendationItemService(repo repository.RecommendationItemRepository, userRepo repository.UserRepository, resourceRepo repository.ResourceRepository) RecommendationItemService {
	return &recommendationItemService{repo: repo, userRepo: userRepo, resourceRepo: resourceRepo}
}

func (s *recommendationItemService) ListVisible(ctx context.Context) ([]dto.RecommendationItemResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	codes, err := roleCodes(ctx, s.userRepo, userID)
	if err != nil {
		return nil, err
	}
	status := statusEnabled
	items, err := s.repo.List(ctx, repository.RecommendationItemListFilter{
		RoleCodes: codes,
		Status:    &status,
	})
	if err != nil {
		return nil, err
	}
	return toRecommendationItemResponses(items), nil
}

func (s *recommendationItemService) AdminList(ctx context.Context, status *int16) ([]dto.RecommendationItemResponse, error) {
	if _, err := requireAdminRole(ctx, s.userRepo); err != nil {
		return nil, err
	}
	if status != nil && !isValidStatus(*status) {
		return nil, ErrInvalidStatus
	}
	items, err := s.repo.List(ctx, repository.RecommendationItemListFilter{Status: status})
	if err != nil {
		return nil, err
	}
	return toRecommendationItemResponses(items), nil
}

func (s *recommendationItemService) Create(ctx context.Context, req dto.CreateRecommendationItemRequest) (dto.RecommendationItemResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	status := statusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if !isValidStatus(status) {
		return dto.RecommendationItemResponse{}, ErrInvalidStatus
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return dto.RecommendationItemResponse{}, ErrInvalidRecommendationItem
	}
	if err := s.ensureResourceExists(ctx, req.ResourceID); err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	publishedAt, err := parseOptionalTime(req.PublishedAt)
	if err != nil {
		return dto.RecommendationItemResponse{}, ErrInvalidTimeRange
	}
	now := time.Now().UTC()
	visibleRoleCodes := stringArrayJSONValue(normalizeRoleCodes(req.VisibleRoleCodes))
	item := &model.RecommendationItem{
		ResourceID:       req.ResourceID,
		Title:            title,
		Subtitle:         optionalStr(strings.TrimSpace(req.Subtitle)),
		SourceName:       optionalStr(strings.TrimSpace(req.SourceName)),
		SourceURL:        optionalStr(strings.TrimSpace(req.SourceURL)),
		IconURL:          optionalStr(strings.TrimSpace(req.IconURL)),
		IconText:         optionalStr(strings.TrimSpace(req.IconText)),
		TargetURL:        optionalStr(strings.TrimSpace(req.TargetURL)),
		PublishedAt:      publishedAt,
		VisibleRoleCodes: &visibleRoleCodes,
		SortOrder:        req.SortOrder,
		Status:           &status,
		CreatedBy:        &userID,
		UpdatedBy:        &userID,
		CreatedAt:        &now,
		UpdatedAt:        &now,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	return toRecommendationItemResponse(*item), nil
}

func (s *recommendationItemService) Update(ctx context.Context, id int64, req dto.UpdateRecommendationItemRequest) (dto.RecommendationItemResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	if id <= 0 {
		return dto.RecommendationItemResponse{}, ErrRecommendationItemNotFound
	}
	updates := map[string]any{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.ResourceID != nil {
		if err := s.ensureResourceExists(ctx, req.ResourceID); err != nil {
			return dto.RecommendationItemResponse{}, err
		}
		updates["resource_id"] = req.ResourceID
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return dto.RecommendationItemResponse{}, ErrInvalidRecommendationItem
		}
		updates["title"] = title
	}
	if req.Subtitle != nil {
		updates["subtitle"] = optionalStr(strings.TrimSpace(*req.Subtitle))
	}
	if req.SourceName != nil {
		updates["source_name"] = optionalStr(strings.TrimSpace(*req.SourceName))
	}
	if req.SourceURL != nil {
		updates["source_url"] = optionalStr(strings.TrimSpace(*req.SourceURL))
	}
	if req.IconURL != nil {
		updates["icon_url"] = optionalStr(strings.TrimSpace(*req.IconURL))
	}
	if req.IconText != nil {
		updates["icon_text"] = optionalStr(strings.TrimSpace(*req.IconText))
	}
	if req.TargetURL != nil {
		updates["target_url"] = optionalStr(strings.TrimSpace(*req.TargetURL))
	}
	if req.PublishedAt != nil {
		publishedAt, err := parseOptionalTime(*req.PublishedAt)
		if err != nil {
			return dto.RecommendationItemResponse{}, ErrInvalidTimeRange
		}
		updates["published_at"] = publishedAt
	}
	if req.VisibleRoleCodes != nil {
		updates["visible_role_codes"] = stringArrayJSONValue(normalizeRoleCodes(req.VisibleRoleCodes))
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.RecommendationItemResponse{}, ErrInvalidStatus
		}
		updates["status"] = *req.Status
	}
	item, err := s.repo.Update(ctx, id, updates)
	if errors.Is(err, repository.ErrRecommendationItemNotFound) {
		return dto.RecommendationItemResponse{}, ErrRecommendationItemNotFound
	}
	if err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	return toRecommendationItemResponse(*item), nil
}

func (s *recommendationItemService) Sort(ctx context.Context, req dto.SortRecommendationItemsRequest) error {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return err
	}
	items := make([]repository.RecommendationItemSortItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ID <= 0 {
			continue
		}
		items = append(items, repository.RecommendationItemSortItem{
			ID:        item.ID,
			SortOrder: item.SortOrder,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return s.repo.UpdateSortOrders(ctx, items, userID)
}

func (s *recommendationItemService) UpdateStatus(ctx context.Context, id int64, status int16) (dto.RecommendationItemResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	if id <= 0 {
		return dto.RecommendationItemResponse{}, ErrRecommendationItemNotFound
	}
	if !isValidStatus(status) {
		return dto.RecommendationItemResponse{}, ErrInvalidStatus
	}
	item, err := s.repo.Update(ctx, id, map[string]any{
		"status":     status,
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	})
	if errors.Is(err, repository.ErrRecommendationItemNotFound) {
		return dto.RecommendationItemResponse{}, ErrRecommendationItemNotFound
	}
	if err != nil {
		return dto.RecommendationItemResponse{}, err
	}
	return toRecommendationItemResponse(*item), nil
}

func (s *recommendationItemService) ensureResourceExists(ctx context.Context, resourceID *int64) error {
	if resourceID == nil {
		return nil
	}
	if *resourceID <= 0 {
		return ErrResourceNotFound
	}
	if _, err := s.resourceRepo.FindResourceByID(ctx, *resourceID); err != nil {
		if errors.Is(err, repository.ErrResourceNotFound) {
			return ErrResourceNotFound
		}
		return err
	}
	return nil
}

// CarouselSlideService 幻灯片业务接口。
type CarouselSlideService interface {
	ListVisible(ctx context.Context) ([]dto.CarouselSlideResponse, error)
	AdminList(ctx context.Context, status *int16) ([]dto.CarouselSlideResponse, error)
	Create(ctx context.Context, req dto.CreateCarouselSlideRequest) (dto.CarouselSlideResponse, error)
	Update(ctx context.Context, id int64, req dto.UpdateCarouselSlideRequest) (dto.CarouselSlideResponse, error)
	Sort(ctx context.Context, req dto.SortCarouselSlidesRequest) error
	UpdateStatus(ctx context.Context, id int64, status int16) (dto.CarouselSlideResponse, error)
}

type carouselSlideService struct {
	repo     repository.CarouselSlideRepository
	userRepo repository.UserRepository
}

// NewCarouselSlideService 构造 CarouselSlideService。
func NewCarouselSlideService(repo repository.CarouselSlideRepository, userRepo repository.UserRepository) CarouselSlideService {
	return &carouselSlideService{repo: repo, userRepo: userRepo}
}

func (s *carouselSlideService) ListVisible(ctx context.Context) ([]dto.CarouselSlideResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	codes, err := roleCodes(ctx, s.userRepo, userID)
	if err != nil {
		return nil, err
	}
	status := statusEnabled
	slides, err := s.repo.List(ctx, repository.CarouselSlideListFilter{
		RoleCodes:  codes,
		Status:     &status,
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	return toCarouselSlideResponses(slides), nil
}

func (s *carouselSlideService) AdminList(ctx context.Context, status *int16) ([]dto.CarouselSlideResponse, error) {
	if _, err := requireAdminRole(ctx, s.userRepo); err != nil {
		return nil, err
	}
	if status != nil && !isValidStatus(*status) {
		return nil, ErrInvalidStatus
	}
	slides, err := s.repo.List(ctx, repository.CarouselSlideListFilter{Status: status})
	if err != nil {
		return nil, err
	}
	return toCarouselSlideResponses(slides), nil
}

func (s *carouselSlideService) Create(ctx context.Context, req dto.CreateCarouselSlideRequest) (dto.CarouselSlideResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	status := statusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if !isValidStatus(status) {
		return dto.CarouselSlideResponse{}, ErrInvalidStatus
	}
	code := strings.TrimSpace(req.Code)
	title := strings.TrimSpace(req.Title)
	if code == "" || title == "" {
		return dto.CarouselSlideResponse{}, ErrInvalidCarouselSlide
	}
	startsAt, endsAt, err := parseTimeRange(req.StartsAt, req.EndsAt)
	if err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	autoplaySeconds := defaultAutoplaySeconds
	if req.AutoplaySeconds != nil && *req.AutoplaySeconds > 0 {
		autoplaySeconds = *req.AutoplaySeconds
	}
	now := time.Now().UTC()
	visibleRoleCodes := stringArrayJSONValue(normalizeRoleCodes(req.VisibleRoleCodes))
	slide := &model.CarouselSlide{
		Code:             code,
		Title:            title,
		Subtitle:         optionalStr(strings.TrimSpace(req.Subtitle)),
		Description:      optionalStr(strings.TrimSpace(req.Description)),
		ImageURL:         optionalStr(strings.TrimSpace(req.ImageURL)),
		Background:       optionalStr(strings.TrimSpace(req.Background)),
		ButtonText:       optionalStr(strings.TrimSpace(req.ButtonText)),
		TargetURL:        optionalStr(strings.TrimSpace(req.TargetURL)),
		AutoplaySeconds:  &autoplaySeconds,
		StartsAt:         startsAt,
		EndsAt:           endsAt,
		VisibleRoleCodes: &visibleRoleCodes,
		SortOrder:        req.SortOrder,
		Status:           &status,
		CreatedBy:        &userID,
		UpdatedBy:        &userID,
		CreatedAt:        &now,
		UpdatedAt:        &now,
	}
	if err := s.repo.Create(ctx, slide); err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	return toCarouselSlideResponse(*slide), nil
}

func (s *carouselSlideService) Update(ctx context.Context, id int64, req dto.UpdateCarouselSlideRequest) (dto.CarouselSlideResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	if id <= 0 {
		return dto.CarouselSlideResponse{}, ErrCarouselSlideNotFound
	}
	updates := map[string]any{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return dto.CarouselSlideResponse{}, ErrInvalidCarouselSlide
		}
		updates["code"] = code
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return dto.CarouselSlideResponse{}, ErrInvalidCarouselSlide
		}
		updates["title"] = title
	}
	if req.Subtitle != nil {
		updates["subtitle"] = optionalStr(strings.TrimSpace(*req.Subtitle))
	}
	if req.Description != nil {
		updates["description"] = optionalStr(strings.TrimSpace(*req.Description))
	}
	if req.ImageURL != nil {
		updates["image_url"] = optionalStr(strings.TrimSpace(*req.ImageURL))
	}
	if req.Background != nil {
		updates["background"] = optionalStr(strings.TrimSpace(*req.Background))
	}
	if req.ButtonText != nil {
		updates["button_text"] = optionalStr(strings.TrimSpace(*req.ButtonText))
	}
	if req.TargetURL != nil {
		updates["target_url"] = optionalStr(strings.TrimSpace(*req.TargetURL))
	}
	if req.AutoplaySeconds != nil {
		if *req.AutoplaySeconds <= 0 {
			return dto.CarouselSlideResponse{}, ErrInvalidCarouselSlide
		}
		updates["autoplay_seconds"] = *req.AutoplaySeconds
	}
	if req.StartsAt != nil || req.EndsAt != nil {
		startsAt, endsAt, err := s.mergedSlideTimeRange(ctx, id, req.StartsAt, req.EndsAt)
		if err != nil {
			return dto.CarouselSlideResponse{}, err
		}
		updates["starts_at"] = startsAt
		updates["ends_at"] = endsAt
	}
	if req.VisibleRoleCodes != nil {
		updates["visible_role_codes"] = stringArrayJSONValue(normalizeRoleCodes(req.VisibleRoleCodes))
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.CarouselSlideResponse{}, ErrInvalidStatus
		}
		updates["status"] = *req.Status
	}
	slide, err := s.repo.Update(ctx, id, updates)
	if errors.Is(err, repository.ErrCarouselSlideNotFound) {
		return dto.CarouselSlideResponse{}, ErrCarouselSlideNotFound
	}
	if err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	return toCarouselSlideResponse(*slide), nil
}

func (s *carouselSlideService) Sort(ctx context.Context, req dto.SortCarouselSlidesRequest) error {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return err
	}
	items := make([]repository.CarouselSlideSortItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ID <= 0 {
			continue
		}
		items = append(items, repository.CarouselSlideSortItem{
			ID:        item.ID,
			SortOrder: item.SortOrder,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return s.repo.UpdateSortOrders(ctx, items, userID)
}

func (s *carouselSlideService) UpdateStatus(ctx context.Context, id int64, status int16) (dto.CarouselSlideResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	if id <= 0 {
		return dto.CarouselSlideResponse{}, ErrCarouselSlideNotFound
	}
	if !isValidStatus(status) {
		return dto.CarouselSlideResponse{}, ErrInvalidStatus
	}
	slide, err := s.repo.Update(ctx, id, map[string]any{
		"status":     status,
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	})
	if errors.Is(err, repository.ErrCarouselSlideNotFound) {
		return dto.CarouselSlideResponse{}, ErrCarouselSlideNotFound
	}
	if err != nil {
		return dto.CarouselSlideResponse{}, err
	}
	return toCarouselSlideResponse(*slide), nil
}

func (s *carouselSlideService) mergedSlideTimeRange(ctx context.Context, id int64, startsAtRaw, endsAtRaw *string) (*time.Time, *time.Time, error) {
	current, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrCarouselSlideNotFound) {
		return nil, nil, ErrCarouselSlideNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	startsAt := current.StartsAt
	endsAt := current.EndsAt
	if startsAtRaw != nil {
		startsAt, err = parseOptionalTime(*startsAtRaw)
		if err != nil {
			return nil, nil, ErrInvalidTimeRange
		}
	}
	if endsAtRaw != nil {
		endsAt, err = parseOptionalTime(*endsAtRaw)
		if err != nil {
			return nil, nil, ErrInvalidTimeRange
		}
	}
	if startsAt != nil && endsAt != nil && startsAt.After(*endsAt) {
		return nil, nil, ErrInvalidTimeRange
	}
	return startsAt, endsAt, nil
}

func toRecommendationItemResponses(items []model.RecommendationItem) []dto.RecommendationItemResponse {
	resp := make([]dto.RecommendationItemResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toRecommendationItemResponse(item))
	}
	return resp
}

func toRecommendationItemResponse(item model.RecommendationItem) dto.RecommendationItemResponse {
	return dto.RecommendationItemResponse{
		ID:               item.ID,
		ResourceID:       item.ResourceID,
		Title:            item.Title,
		Subtitle:         derefStr(item.Subtitle),
		SourceName:       derefStr(item.SourceName),
		SourceURL:        derefStr(item.SourceURL),
		IconURL:          derefStr(item.IconURL),
		IconText:         derefStr(item.IconText),
		TargetURL:        derefStr(item.TargetURL),
		PublishedAt:      formatTime(item.PublishedAt),
		VisibleRoleCodes: jsonStringArray(item.VisibleRoleCodes),
		SortOrder:        item.SortOrder,
		Status:           derefInt16(item.Status),
	}
}

func toCarouselSlideResponses(slides []model.CarouselSlide) []dto.CarouselSlideResponse {
	resp := make([]dto.CarouselSlideResponse, 0, len(slides))
	for _, slide := range slides {
		resp = append(resp, toCarouselSlideResponse(slide))
	}
	return resp
}

func toCarouselSlideResponse(slide model.CarouselSlide) dto.CarouselSlideResponse {
	return dto.CarouselSlideResponse{
		ID:               slide.ID,
		Code:             slide.Code,
		Title:            slide.Title,
		Subtitle:         derefStr(slide.Subtitle),
		Description:      derefStr(slide.Description),
		ImageURL:         derefStr(slide.ImageURL),
		Background:       derefStr(slide.Background),
		ButtonText:       derefStr(slide.ButtonText),
		TargetURL:        derefStr(slide.TargetURL),
		AutoplaySeconds:  derefInt32(slide.AutoplaySeconds),
		StartsAt:         formatTime(slide.StartsAt),
		EndsAt:           formatTime(slide.EndsAt),
		VisibleRoleCodes: jsonStringArray(slide.VisibleRoleCodes),
		SortOrder:        slide.SortOrder,
		Status:           derefInt16(slide.Status),
	}
}

func parseTimeRange(startsAtRaw, endsAtRaw string) (*time.Time, *time.Time, error) {
	startsAt, err := parseOptionalTime(startsAtRaw)
	if err != nil {
		return nil, nil, ErrInvalidTimeRange
	}
	endsAt, err := parseOptionalTime(endsAtRaw)
	if err != nil {
		return nil, nil, ErrInvalidTimeRange
	}
	if startsAt != nil && endsAt != nil && startsAt.After(*endsAt) {
		return nil, nil, ErrInvalidTimeRange
	}
	return startsAt, endsAt, nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
