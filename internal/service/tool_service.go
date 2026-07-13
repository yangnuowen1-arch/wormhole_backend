package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

const (
	statusDisabled int16 = 0
	statusEnabled  int16 = 1
)

// CommonToolService 用户常用工具业务接口。
type CommonToolService interface {
	Add(ctx context.Context, req dto.AddCommonToolRequest) (dto.CommonToolResponse, error)
	Remove(ctx context.Context, resourceID int64) (int64, error)
	List(ctx context.Context) ([]dto.CommonToolResponse, error)
	Sort(ctx context.Context, req dto.SortCommonToolsRequest) error
}

type commonToolService struct {
	repo         repository.CommonToolRepository
	resourceRepo repository.ResourceRepository
}

// NewCommonToolService 构造 CommonToolService。
func NewCommonToolService(repo repository.CommonToolRepository, resourceRepo repository.ResourceRepository) CommonToolService {
	return &commonToolService{repo: repo, resourceRepo: resourceRepo}
}

func (s *commonToolService) Add(ctx context.Context, req dto.AddCommonToolRequest) (dto.CommonToolResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return dto.CommonToolResponse{}, err
	}
	if req.ResourceID <= 0 {
		return dto.CommonToolResponse{}, ErrResourceNotFound
	}
	if _, err := s.resourceRepo.FindResourceByID(ctx, req.ResourceID); err != nil {
		if errors.Is(err, repository.ErrResourceNotFound) {
			return dto.CommonToolResponse{}, ErrResourceNotFound
		}
		return dto.CommonToolResponse{}, err
	}
	if err := s.repo.Add(ctx, userID, req.ResourceID, req.SortOrder); err != nil {
		return dto.CommonToolResponse{}, err
	}
	tools, err := s.repo.List(ctx, userID)
	if err != nil {
		return dto.CommonToolResponse{}, err
	}
	for _, tool := range tools {
		if tool.ID == req.ResourceID {
			return toCommonToolResponse(tool), nil
		}
	}
	return dto.CommonToolResponse{}, ErrResourceNotFound
}

func (s *commonToolService) Remove(ctx context.Context, resourceID int64) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}
	if resourceID <= 0 {
		return 0, ErrResourceNotFound
	}
	return s.repo.Remove(ctx, userID, resourceID)
}

func (s *commonToolService) List(ctx context.Context) ([]dto.CommonToolResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	tools, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.CommonToolResponse, 0, len(tools))
	for _, tool := range tools {
		resp = append(resp, toCommonToolResponse(tool))
	}
	return resp, nil
}

func (s *commonToolService) Sort(ctx context.Context, req dto.SortCommonToolsRequest) error {
	userID, err := currentUserID(ctx)
	if err != nil {
		return err
	}
	items := make([]repository.CommonToolSortItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ResourceID <= 0 {
			continue
		}
		items = append(items, repository.CommonToolSortItem{
			ResourceID: item.ResourceID,
			SortOrder:  item.SortOrder,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return s.repo.UpdateSortOrders(ctx, userID, items)
}

// QuickEntryService 快速入口业务接口。
type QuickEntryService interface {
	ListVisible(ctx context.Context) ([]dto.QuickEntryResponse, error)
	AdminList(ctx context.Context, status *int16) ([]dto.QuickEntryResponse, error)
	Create(ctx context.Context, req dto.CreateQuickEntryRequest) (dto.QuickEntryResponse, error)
	Update(ctx context.Context, id int32, req dto.UpdateQuickEntryRequest) (dto.QuickEntryResponse, error)
	Sort(ctx context.Context, req dto.SortQuickEntriesRequest) error
	UpdateStatus(ctx context.Context, id int32, status int16) (dto.QuickEntryResponse, error)
}

type quickEntryService struct {
	repo     repository.QuickEntryRepository
	userRepo repository.UserRepository
}

// NewQuickEntryService 构造 QuickEntryService。
func NewQuickEntryService(repo repository.QuickEntryRepository, userRepo repository.UserRepository) QuickEntryService {
	return &quickEntryService{repo: repo, userRepo: userRepo}
}

func (s *quickEntryService) ListVisible(ctx context.Context) ([]dto.QuickEntryResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	roleCodes, err := s.roleCodes(ctx, userID)
	if err != nil {
		return nil, err
	}
	status := statusEnabled
	entries, err := s.repo.List(ctx, repository.QuickEntryListFilter{
		RoleCodes: roleCodes,
		Status:    &status,
	})
	if err != nil {
		return nil, err
	}
	return toQuickEntryResponses(entries), nil
}

func (s *quickEntryService) AdminList(ctx context.Context, status *int16) ([]dto.QuickEntryResponse, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if status != nil && !isValidStatus(*status) {
		return nil, ErrInvalidStatus
	}
	entries, err := s.repo.List(ctx, repository.QuickEntryListFilter{Status: status})
	if err != nil {
		return nil, err
	}
	return toQuickEntryResponses(entries), nil
}

func (s *quickEntryService) Create(ctx context.Context, req dto.CreateQuickEntryRequest) (dto.QuickEntryResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.QuickEntryResponse{}, err
	}
	status := statusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if !isValidStatus(status) {
		return dto.QuickEntryResponse{}, ErrInvalidStatus
	}
	code := strings.TrimSpace(req.Code)
	title := strings.TrimSpace(req.Title)
	targetURL := strings.TrimSpace(req.TargetURL)
	if code == "" || title == "" || targetURL == "" {
		return dto.QuickEntryResponse{}, ErrInvalidQuickEntry
	}
	now := time.Now().UTC()
	visibleRoleCodes := stringArrayJSONValue(normalizeRoleCodes(req.VisibleRoleCodes))
	entry := &model.QuickEntry{
		Code:             code,
		Title:            title,
		IconURL:          optionalStr(strings.TrimSpace(req.IconURL)),
		IconText:         optionalStr(strings.TrimSpace(req.IconText)),
		TargetURL:        targetURL,
		Description:      optionalStr(strings.TrimSpace(req.Description)),
		VisibleRoleCodes: &visibleRoleCodes,
		SortOrder:        req.SortOrder,
		Status:           &status,
		CreatedBy:        &userID,
		UpdatedBy:        &userID,
		CreatedAt:        &now,
		UpdatedAt:        &now,
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		return dto.QuickEntryResponse{}, err
	}
	return toQuickEntryResponse(*entry), nil
}

func (s *quickEntryService) Update(ctx context.Context, id int32, req dto.UpdateQuickEntryRequest) (dto.QuickEntryResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.QuickEntryResponse{}, err
	}
	if id <= 0 {
		return dto.QuickEntryResponse{}, ErrQuickEntryNotFound
	}

	updates := map[string]any{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return dto.QuickEntryResponse{}, ErrInvalidQuickEntry
		}
		updates["code"] = code
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return dto.QuickEntryResponse{}, ErrInvalidQuickEntry
		}
		updates["title"] = title
	}
	if req.IconURL != nil {
		updates["icon_url"] = optionalStr(strings.TrimSpace(*req.IconURL))
	}
	if req.IconText != nil {
		updates["icon_text"] = optionalStr(strings.TrimSpace(*req.IconText))
	}
	if req.TargetURL != nil {
		targetURL := strings.TrimSpace(*req.TargetURL)
		if targetURL == "" {
			return dto.QuickEntryResponse{}, ErrInvalidQuickEntry
		}
		updates["target_url"] = targetURL
	}
	if req.Description != nil {
		updates["description"] = optionalStr(strings.TrimSpace(*req.Description))
	}
	if req.VisibleRoleCodes != nil {
		updates["visible_role_codes"] = stringArrayJSONValue(normalizeRoleCodes(req.VisibleRoleCodes))
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.QuickEntryResponse{}, ErrInvalidStatus
		}
		updates["status"] = *req.Status
	}

	entry, err := s.repo.Update(ctx, id, updates)
	if errors.Is(err, repository.ErrQuickEntryNotFound) {
		return dto.QuickEntryResponse{}, ErrQuickEntryNotFound
	}
	if err != nil {
		return dto.QuickEntryResponse{}, err
	}
	return toQuickEntryResponse(*entry), nil
}

func (s *quickEntryService) Sort(ctx context.Context, req dto.SortQuickEntriesRequest) error {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return err
	}
	items := make([]repository.QuickEntrySortItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ID <= 0 {
			continue
		}
		items = append(items, repository.QuickEntrySortItem{
			ID:        item.ID,
			SortOrder: item.SortOrder,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return s.repo.UpdateSortOrders(ctx, items, userID)
}

func (s *quickEntryService) UpdateStatus(ctx context.Context, id int32, status int16) (dto.QuickEntryResponse, error) {
	userID, err := s.requireAdmin(ctx)
	if err != nil {
		return dto.QuickEntryResponse{}, err
	}
	if id <= 0 {
		return dto.QuickEntryResponse{}, ErrQuickEntryNotFound
	}
	if !isValidStatus(status) {
		return dto.QuickEntryResponse{}, ErrInvalidStatus
	}
	entry, err := s.repo.Update(ctx, id, map[string]any{
		"status":     status,
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	})
	if errors.Is(err, repository.ErrQuickEntryNotFound) {
		return dto.QuickEntryResponse{}, ErrQuickEntryNotFound
	}
	if err != nil {
		return dto.QuickEntryResponse{}, err
	}
	return toQuickEntryResponse(*entry), nil
}

func (s *quickEntryService) requireAdmin(ctx context.Context) (int64, error) {
	return requireAdminRole(ctx, s.userRepo)
}

func (s *quickEntryService) roleCodes(ctx context.Context, userID int64) ([]string, error) {
	return roleCodes(ctx, s.userRepo, userID)
}

func requireAdminRole(ctx context.Context, userRepo repository.UserRepository) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}
	codes, err := roleCodes(ctx, userRepo, userID)
	if err != nil {
		return 0, err
	}
	for _, code := range codes {
		if strings.EqualFold(code, "admin") {
			return userID, nil
		}
	}
	return 0, ErrForbidden
}

func roleCodes(ctx context.Context, userRepo repository.UserRepository, userID int64) ([]string, error) {
	roles, err := userRepo.FindRolesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		if code := strings.TrimSpace(role.Code); code != "" {
			codes = append(codes, code)
		}
	}
	return codes, nil
}

func toCommonToolResponse(record repository.CommonToolRecord) dto.CommonToolResponse {
	return dto.CommonToolResponse{
		Resource:  toResourceResponse(commonToolResourceRecord(record)),
		SortOrder: record.CommonSortOrder,
		StarredAt: formatTime(record.StarredAt),
	}
}

func commonToolResourceRecord(record repository.CommonToolRecord) repository.ResourceRecord {
	return repository.ResourceRecord{
		ID:            record.ID,
		CategoryID:    record.CategoryID,
		CategoryCode:  record.CategoryCode,
		CategoryName:  record.CategoryName,
		Slug:          record.Slug,
		Name:          record.Name,
		IconURL:       record.IconURL,
		IconText:      record.IconText,
		WebsiteURL:    record.WebsiteURL,
		Summary:       record.Summary,
		Description:   record.Description,
		ResourceType:  record.ResourceType,
		Provider:      record.Provider,
		ModelCount:    record.ModelCount,
		FollowerCount: record.FollowerCount,
		Badge:         record.Badge,
		Tags:          record.Tags,
		Metadata:      record.Metadata,
		IsFeatured:    record.IsFeatured,
		SortOrder:     record.SortOrder,
		Status:        record.Status,
	}
}

func toQuickEntryResponses(entries []model.QuickEntry) []dto.QuickEntryResponse {
	resp := make([]dto.QuickEntryResponse, 0, len(entries))
	for _, entry := range entries {
		resp = append(resp, toQuickEntryResponse(entry))
	}
	return resp
}

func toQuickEntryResponse(entry model.QuickEntry) dto.QuickEntryResponse {
	return dto.QuickEntryResponse{
		ID:               entry.ID,
		Code:             entry.Code,
		Title:            entry.Title,
		IconURL:          derefStr(entry.IconURL),
		IconText:         derefStr(entry.IconText),
		TargetURL:        entry.TargetURL,
		Description:      derefStr(entry.Description),
		VisibleRoleCodes: jsonStringArray(entry.VisibleRoleCodes),
		SortOrder:        entry.SortOrder,
		Status:           derefInt16(entry.Status),
	}
}

func normalizeRoleCodes(roleCodes []string) []string {
	seen := make(map[string]struct{}, len(roleCodes))
	codes := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleCode = strings.TrimSpace(roleCode)
		if roleCode == "" {
			continue
		}
		if _, ok := seen[roleCode]; ok {
			continue
		}
		seen[roleCode] = struct{}{}
		codes = append(codes, roleCode)
	}
	return codes
}

func stringArrayJSONValue(values []string) string {
	payload, _ := json.Marshal(values)
	return string(payload)
}

func isValidStatus(status int16) bool {
	return status == statusDisabled || status == statusEnabled
}

func derefInt16(v *int16) int16 {
	if v == nil {
		return 0
	}
	return *v
}
