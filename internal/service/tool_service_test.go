package service

import (
	"context"
	"errors"
	"testing"

	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

func TestQuickEntryCreateRequiresAdmin(t *testing.T) {
	svc := NewQuickEntryService(&quickEntryRepoStub{}, &meRepo{
		roles: []model.Role{{Code: "user", Name: "普通用户"}},
	})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "alice"})

	_, err := svc.Create(ctx, dto.CreateQuickEntryRequest{
		Code:      "github",
		Title:     "GitHub",
		TargetURL: "https://github.com",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create error = %v, want ErrForbidden", err)
	}
}

func TestQuickEntryCreateByAdminNormalizesDefaults(t *testing.T) {
	repo := &quickEntryRepoStub{}
	svc := NewQuickEntryService(repo, &meRepo{
		roles: []model.Role{{Code: "admin", Name: "管理员"}},
	})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	resp, err := svc.Create(ctx, dto.CreateQuickEntryRequest{
		Code:             " github ",
		Title:            " GitHub ",
		TargetURL:        " https://github.com ",
		VisibleRoleCodes: []string{"admin", "admin", "designer", ""},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.Code != "github" || resp.Title != "GitHub" || resp.TargetURL != "https://github.com" {
		t.Fatalf("quick entry response = %+v, want trimmed fields", resp)
	}
	if resp.Status != 1 {
		t.Fatalf("status = %d, want 1", resp.Status)
	}
	if len(resp.VisibleRoleCodes) != 2 || resp.VisibleRoleCodes[0] != "admin" || resp.VisibleRoleCodes[1] != "designer" {
		t.Fatalf("visible roles = %#v, want [admin designer]", resp.VisibleRoleCodes)
	}
	if repo.created == nil || repo.created.CreatedBy == nil || *repo.created.CreatedBy != 7 {
		t.Fatalf("created entry audit user = %+v, want 7", repo.created)
	}
}

type quickEntryRepoStub struct {
	created *model.QuickEntry
}

func (r *quickEntryRepoStub) List(ctx context.Context, filter repository.QuickEntryListFilter) ([]model.QuickEntry, error) {
	return nil, nil
}

func (r *quickEntryRepoStub) FindByID(ctx context.Context, id int32) (*model.QuickEntry, error) {
	return nil, repository.ErrQuickEntryNotFound
}

func (r *quickEntryRepoStub) Create(ctx context.Context, entry *model.QuickEntry) error {
	entry.ID = 1
	r.created = entry
	return nil
}

func (r *quickEntryRepoStub) Update(ctx context.Context, id int32, updates map[string]any) (*model.QuickEntry, error) {
	return nil, repository.ErrQuickEntryNotFound
}

func (r *quickEntryRepoStub) UpdateSortOrders(ctx context.Context, items []repository.QuickEntrySortItem, updatedBy int64) error {
	return nil
}

var _ repository.QuickEntryRepository = (*quickEntryRepoStub)(nil)
