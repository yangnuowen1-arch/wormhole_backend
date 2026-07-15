package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

func TestAnnouncementListVisibleUsesVisibleFilter(t *testing.T) {
	publishedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
	repo := &announcementRepoStub{
		list: []model.AnnouncementRecord{{
			ID:          1,
			Title:       "平台维护通知",
			Content:     "本周六维护。",
			IsPinned:    true,
			Status:      statusEnabled,
			PublishedAt: publishedAt,
			CreatedAt:   publishedAt,
			UpdatedAt:   publishedAt,
		}},
	}
	svc := NewAnnouncementService(repo, &announcementRoleFinder{})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 8, Username: "alice"})

	announcements, err := svc.ListVisible(ctx)
	if err != nil {
		t.Fatalf("ListVisible returned error: %v", err)
	}
	if !repo.listFilter.VisibleOnly {
		t.Fatalf("list filter = %+v, want VisibleOnly=true", repo.listFilter)
	}
	if len(announcements) != 1 || announcements[0].Title != "平台维护通知" || !announcements[0].IsPinned {
		t.Fatalf("announcements = %+v, want visible pinned announcement", announcements)
	}
}

func TestAnnouncementListVisibleRequiresLogin(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, &announcementRoleFinder{})

	_, err := svc.ListVisible(context.Background())
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("ListVisible error = %v, want ErrUnauthenticated", err)
	}
	if repo.listCalled {
		t.Fatal("repository List should not be called without a logged-in user")
	}
}

func TestAnnouncementCreateByAdminSetsAuditAndScheduleFields(t *testing.T) {
	repo := &announcementRepoStub{}
	roleFinder := &announcementRoleFinder{roles: []model.Role{{Code: "admin"}}}
	svc := NewAnnouncementService(repo, roleFinder)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	announcement, err := svc.Create(ctx, dto.CreateAnnouncementRequest{
		Title:       " 平台维护通知 ",
		Content:     " 本周六 02:00-03:00 进行维护。 ",
		IsPinned:    true,
		PublishedAt: "2026-07-13T10:00:00Z",
		ExpiresAt:   "2026-07-20T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected repository Create to be called")
	}
	if repo.created.Title != "平台维护通知" || repo.created.Content != "本周六 02:00-03:00 进行维护。" {
		t.Fatalf("created announcement = %+v, want trimmed title and content", repo.created)
	}
	if repo.created.Status != statusEnabled || !repo.created.IsPinned {
		t.Fatalf("created status/pin = (%d, %t), want (%d, true)", repo.created.Status, repo.created.IsPinned, statusEnabled)
	}
	if repo.created.CreatedBy == nil || repo.created.UpdatedBy == nil || *repo.created.CreatedBy != 7 || *repo.created.UpdatedBy != 7 {
		t.Fatalf("audit users = (%v, %v), want both 7", repo.created.CreatedBy, repo.created.UpdatedBy)
	}
	if repo.created.ExpiresAt == nil || !repo.created.ExpiresAt.After(repo.created.PublishedAt) {
		t.Fatalf("schedule = (%s, %v), want expires after published", repo.created.PublishedAt, repo.created.ExpiresAt)
	}
	if announcement.ID != 1 || announcement.PublishedAt != "2026-07-13T10:00:00Z" {
		t.Fatalf("response = %+v, want persisted announcement", announcement)
	}
}

func TestAnnouncementCreateRejectsNonAdmin(t *testing.T) {
	repo := &announcementRepoStub{}
	roleFinder := &announcementRoleFinder{roles: []model.Role{{Code: "user"}}}
	svc := NewAnnouncementService(repo, roleFinder)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 8, Username: "alice"})

	_, err := svc.Create(ctx, dto.CreateAnnouncementRequest{Title: "通知", Content: "内容"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create error = %v, want ErrForbidden", err)
	}
	if repo.created != nil {
		t.Fatal("repository Create should not be called for a non-admin user")
	}
}

func TestAnnouncementUpdateRejectsInvalidTimeRange(t *testing.T) {
	publishedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
	repo := &announcementRepoStub{find: &model.AnnouncementRecord{
		ID:          1,
		Title:       "通知",
		Content:     "内容",
		Status:      statusEnabled,
		PublishedAt: publishedAt,
	}}
	roleFinder := &announcementRoleFinder{roles: []model.Role{{Code: "admin"}}}
	svc := NewAnnouncementService(repo, roleFinder)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})
	expiresAt := "2026-07-13T09:00:00Z"

	_, err := svc.Update(ctx, 1, dto.UpdateAnnouncementRequest{ExpiresAt: &expiresAt})
	if !errors.Is(err, ErrInvalidAnnouncement) {
		t.Fatalf("Update error = %v, want ErrInvalidAnnouncement", err)
	}
	if repo.updateCalled {
		t.Fatal("repository Update should not be called for an invalid time range")
	}
}

func TestAnnouncementUpdateWritesEditableFields(t *testing.T) {
	publishedAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
	repo := &announcementRepoStub{find: &model.AnnouncementRecord{
		ID:          1,
		Title:       "旧通知",
		Content:     "旧内容",
		Status:      statusEnabled,
		PublishedAt: publishedAt,
		CreatedAt:   publishedAt,
		UpdatedAt:   publishedAt,
	}}
	roleFinder := &announcementRoleFinder{roles: []model.Role{{Code: "admin"}}}
	svc := NewAnnouncementService(repo, roleFinder)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})
	title := " 新通知 "
	pinned := true

	_, err := svc.Update(ctx, 1, dto.UpdateAnnouncementRequest{Title: &title, IsPinned: &pinned})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if !repo.updateCalled {
		t.Fatal("expected repository Update to be called")
	}
	if got, _ := repo.updates["title"].(string); got != "新通知" {
		t.Fatalf("title update = %q, want 新通知", got)
	}
	if got, _ := repo.updates["is_pinned"].(bool); !got {
		t.Fatalf("is_pinned update = %v, want true", repo.updates["is_pinned"])
	}
	if got, _ := repo.updates["updated_by"].(int64); got != 7 {
		t.Fatalf("updated_by = %d, want 7", got)
	}
}

type announcementRoleFinder struct {
	roles []model.Role
	err   error
}

func (s *announcementRoleFinder) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	return s.roles, s.err
}

type announcementRepoStub struct {
	listCalled   bool
	list         []model.AnnouncementRecord
	listFilter   repository.AnnouncementListFilter
	listErr      error
	find         *model.AnnouncementRecord
	findErr      error
	created      *model.AnnouncementRecord
	updateCalled bool
	updates      map[string]any
}

func (r *announcementRepoStub) List(ctx context.Context, filter repository.AnnouncementListFilter) ([]model.AnnouncementRecord, error) {
	r.listCalled = true
	r.listFilter = filter
	return r.list, r.listErr
}

func (r *announcementRepoStub) FindByID(ctx context.Context, id int64) (*model.AnnouncementRecord, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.find == nil {
		return nil, repository.ErrAnnouncementNotFound
	}
	return r.find, nil
}

func (r *announcementRepoStub) Create(ctx context.Context, announcement *model.AnnouncementRecord) error {
	announcement.ID = 1
	r.created = announcement
	return nil
}

func (r *announcementRepoStub) Update(ctx context.Context, id int64, updates map[string]any) (*model.AnnouncementRecord, error) {
	r.updateCalled = true
	r.updates = updates
	if r.find == nil {
		return nil, repository.ErrAnnouncementNotFound
	}
	updated := *r.find
	if title, ok := updates["title"].(string); ok {
		updated.Title = title
	}
	if content, ok := updates["content"].(string); ok {
		updated.Content = content
	}
	if pinned, ok := updates["is_pinned"].(bool); ok {
		updated.IsPinned = pinned
	}
	if status, ok := updates["status"].(int16); ok {
		updated.Status = status
	}
	return &updated, nil
}

var _ UserRoleFinder = (*announcementRoleFinder)(nil)
var _ repository.AnnouncementRepository = (*announcementRepoStub)(nil)
