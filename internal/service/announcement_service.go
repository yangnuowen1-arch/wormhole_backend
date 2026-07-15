package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

const (
	maxAnnouncementTitleLength   = 128
	maxAnnouncementContentLength = 10000
)

// AnnouncementService 公告业务接口。
type AnnouncementService interface {
	ListVisible(ctx context.Context) ([]dto.AnnouncementResponse, error)
	AdminList(ctx context.Context, status *int16) ([]dto.AnnouncementResponse, error)
	Create(ctx context.Context, req dto.CreateAnnouncementRequest) (dto.AnnouncementResponse, error)
	Update(ctx context.Context, id int64, req dto.UpdateAnnouncementRequest) (dto.AnnouncementResponse, error)
	UpdateStatus(ctx context.Context, id int64, status int16) (dto.AnnouncementResponse, error)
}

type announcementService struct {
	repo     repository.AnnouncementRepository
	userRepo UserRoleFinder
}

// NewAnnouncementService 构造 AnnouncementService。
func NewAnnouncementService(repo repository.AnnouncementRepository, userRepo UserRoleFinder) AnnouncementService {
	return &announcementService{repo: repo, userRepo: userRepo}
}

// ListVisible 返回当前登录用户可见的已发布且未过期公告。
func (s *announcementService) ListVisible(ctx context.Context) ([]dto.AnnouncementResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}
	announcements, err := s.repo.List(ctx, repository.AnnouncementListFilter{VisibleOnly: true})
	if err != nil {
		return nil, err
	}
	return toAnnouncementResponses(announcements), nil
}

// AdminList 返回管理员可管理的全部公告。
func (s *announcementService) AdminList(ctx context.Context, status *int16) ([]dto.AnnouncementResponse, error) {
	if _, err := requireAdminRole(ctx, s.userRepo); err != nil {
		return nil, err
	}
	if status != nil && !isValidStatus(*status) {
		return nil, ErrInvalidStatus
	}
	announcements, err := s.repo.List(ctx, repository.AnnouncementListFilter{Status: status})
	if err != nil {
		return nil, err
	}
	return toAnnouncementResponses(announcements), nil
}

// Create 创建公告；status 默认为已发布，publishedAt 为空时立即发布。
func (s *announcementService) Create(ctx context.Context, req dto.CreateAnnouncementRequest) (dto.AnnouncementResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}

	title, err := normalizeAnnouncementTitle(req.Title)
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}
	content, err := normalizeAnnouncementContent(req.Content)
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}

	status := statusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if !isValidStatus(status) {
		return dto.AnnouncementResponse{}, ErrInvalidStatus
	}

	now := time.Now().UTC()
	publishedAt := now
	if strings.TrimSpace(req.PublishedAt) != "" {
		parsed, parseErr := parseOptionalTime(req.PublishedAt)
		if parseErr != nil || parsed == nil {
			return dto.AnnouncementResponse{}, ErrInvalidAnnouncement
		}
		publishedAt = *parsed
	}
	expiresAt, parseErr := parseOptionalTime(req.ExpiresAt)
	if parseErr != nil || (expiresAt != nil && !expiresAt.After(publishedAt)) {
		return dto.AnnouncementResponse{}, ErrInvalidAnnouncement
	}

	announcement := &model.AnnouncementRecord{
		Title:       title,
		Content:     content,
		IsPinned:    req.IsPinned,
		Status:      status,
		PublishedAt: publishedAt,
		ExpiresAt:   expiresAt,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, announcement); err != nil {
		return dto.AnnouncementResponse{}, err
	}
	return toAnnouncementResponse(*announcement), nil
}

// Update 编辑公告；到期时间必须晚于发布时间。
func (s *announcementService) Update(ctx context.Context, id int64, req dto.UpdateAnnouncementRequest) (dto.AnnouncementResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}
	if id <= 0 {
		return dto.AnnouncementResponse{}, ErrAnnouncementNotFound
	}

	current, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrAnnouncementNotFound) {
		return dto.AnnouncementResponse{}, ErrAnnouncementNotFound
	}
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}

	updates := map[string]any{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	publishedAt := current.PublishedAt
	expiresAt := current.ExpiresAt
	if req.Title != nil {
		title, normalizeErr := normalizeAnnouncementTitle(*req.Title)
		if normalizeErr != nil {
			return dto.AnnouncementResponse{}, normalizeErr
		}
		updates["title"] = title
	}
	if req.Content != nil {
		content, normalizeErr := normalizeAnnouncementContent(*req.Content)
		if normalizeErr != nil {
			return dto.AnnouncementResponse{}, normalizeErr
		}
		updates["content"] = content
	}
	if req.IsPinned != nil {
		updates["is_pinned"] = *req.IsPinned
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.AnnouncementResponse{}, ErrInvalidStatus
		}
		updates["status"] = *req.Status
	}
	if req.PublishedAt != nil {
		if strings.TrimSpace(*req.PublishedAt) == "" {
			return dto.AnnouncementResponse{}, ErrInvalidAnnouncement
		}
		parsed, parseErr := parseOptionalTime(*req.PublishedAt)
		if parseErr != nil || parsed == nil {
			return dto.AnnouncementResponse{}, ErrInvalidAnnouncement
		}
		publishedAt = *parsed
		updates["published_at"] = publishedAt
	}
	if req.ExpiresAt != nil {
		parsed, parseErr := parseOptionalTime(*req.ExpiresAt)
		if parseErr != nil {
			return dto.AnnouncementResponse{}, ErrInvalidAnnouncement
		}
		expiresAt = parsed
		if expiresAt == nil {
			updates["expires_at"] = nil
		} else {
			updates["expires_at"] = *expiresAt
		}
	}
	if expiresAt != nil && !expiresAt.After(publishedAt) {
		return dto.AnnouncementResponse{}, ErrInvalidAnnouncement
	}

	announcement, err := s.repo.Update(ctx, id, updates)
	if errors.Is(err, repository.ErrAnnouncementNotFound) {
		return dto.AnnouncementResponse{}, ErrAnnouncementNotFound
	}
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}
	return toAnnouncementResponse(*announcement), nil
}

// UpdateStatus 启用或下架公告。
func (s *announcementService) UpdateStatus(ctx context.Context, id int64, status int16) (dto.AnnouncementResponse, error) {
	userID, err := requireAdminRole(ctx, s.userRepo)
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}
	if id <= 0 {
		return dto.AnnouncementResponse{}, ErrAnnouncementNotFound
	}
	if !isValidStatus(status) {
		return dto.AnnouncementResponse{}, ErrInvalidStatus
	}

	announcement, err := s.repo.Update(ctx, id, map[string]any{
		"status":     status,
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	})
	if errors.Is(err, repository.ErrAnnouncementNotFound) {
		return dto.AnnouncementResponse{}, ErrAnnouncementNotFound
	}
	if err != nil {
		return dto.AnnouncementResponse{}, err
	}
	return toAnnouncementResponse(*announcement), nil
}

func normalizeAnnouncementTitle(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxAnnouncementTitleLength {
		return "", ErrInvalidAnnouncement
	}
	return value, nil
}

func normalizeAnnouncementContent(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxAnnouncementContentLength {
		return "", ErrInvalidAnnouncement
	}
	return value, nil
}

func toAnnouncementResponses(announcements []model.AnnouncementRecord) []dto.AnnouncementResponse {
	resp := make([]dto.AnnouncementResponse, 0, len(announcements))
	for _, announcement := range announcements {
		resp = append(resp, toAnnouncementResponse(announcement))
	}
	return resp
}

func toAnnouncementResponse(announcement model.AnnouncementRecord) dto.AnnouncementResponse {
	return dto.AnnouncementResponse{
		ID:          announcement.ID,
		Title:       announcement.Title,
		Content:     announcement.Content,
		IsPinned:    announcement.IsPinned,
		Status:      announcement.Status,
		PublishedAt: formatTime(&announcement.PublishedAt),
		ExpiresAt:   formatTime(announcement.ExpiresAt),
		CreatedAt:   formatTime(&announcement.CreatedAt),
		UpdatedAt:   formatTime(&announcement.UpdatedAt),
	}
}
