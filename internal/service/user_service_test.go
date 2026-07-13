package service

import (
	"context"
	"errors"
	"testing"

	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/keycloak"
	"github.com/yang/wormhole_backend/internal/repository"
)

func TestMeReturnsUserRoles(t *testing.T) {
	description := "可以维护资源中心配置"
	repo := &meRepo{
		user: &model.User{
			ID:       7,
			Username: "alice",
		},
		roles: []model.Role{
			{
				ID:          1,
				Code:        "admin",
				Name:        "管理员",
				Description: &description,
			},
		},
	}
	svc := NewUserService(repo, &config.Config{})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "alice"})

	resp, err := svc.Me(ctx)
	if err != nil {
		t.Fatalf("Me returned error: %v", err)
	}
	if resp.ID != 7 || resp.Username != "alice" {
		t.Fatalf("Me user = (%d, %q), want (7, alice)", resp.ID, resp.Username)
	}
	if len(resp.Roles) != 1 {
		t.Fatalf("roles length = %d, want 1", len(resp.Roles))
	}
	if resp.Roles[0].Code != "admin" || resp.Roles[0].Description != description {
		t.Fatalf("role = %+v, want admin with description", resp.Roles[0])
	}
}

func TestLoginWithKeycloakRetriesAfterConcurrentCreate(t *testing.T) {
	existing := &model.User{
		ID:         42,
		KeycloakID: "kc-subject",
		Username:   "alice",
	}
	repo := &concurrentCreateRepo{
		existing:  existing,
		createErr: errors.New("duplicate key value violates unique constraint"),
	}
	svc := NewUserService(repo, &config.Config{
		JWTSecret:    "abcdef0123456789abcdef0123456789",
		JWTExpireHrs: 1,
	})

	resp, err := svc.LoginWithKeycloak(context.Background(), keycloak.Identity{
		Subject:           "kc-subject",
		PreferredUsername: "alice",
		Email:             "alice@example.com",
		Name:              "Alice",
	})
	if err != nil {
		t.Fatalf("LoginWithKeycloak returned error: %v", err)
	}
	if resp.User.ID != existing.ID {
		t.Fatalf("logged-in user ID = %d, want %d", resp.User.ID, existing.ID)
	}
	if repo.findByKeycloakCalls != 2 {
		t.Fatalf("FindByKeycloakID calls = %d, want 2", repo.findByKeycloakCalls)
	}
	if !repo.updated {
		t.Fatal("expected existing user profile to be updated after retry")
	}
}

type meRepo struct {
	user  *model.User
	roles []model.Role
}

func (r *meRepo) Create(ctx context.Context, u *model.User) error {
	return nil
}

func (r *meRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, repository.ErrUserNotFound
	}
	return r.user, nil
}

func (r *meRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *meRepo) FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *meRepo) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	return r.roles, nil
}

func (r *meRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (r *meRepo) UpdateKeycloakProfile(ctx context.Context, u *model.User) error {
	return nil
}

type concurrentCreateRepo struct {
	existing            *model.User
	createErr           error
	findByKeycloakCalls int
	updated             bool
}

func (r *concurrentCreateRepo) Create(ctx context.Context, u *model.User) error {
	return r.createErr
}

func (r *concurrentCreateRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *concurrentCreateRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *concurrentCreateRepo) FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error) {
	r.findByKeycloakCalls++
	if r.findByKeycloakCalls == 1 {
		return nil, repository.ErrUserNotFound
	}
	return r.existing, nil
}

func (r *concurrentCreateRepo) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	return nil, nil
}

func (r *concurrentCreateRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (r *concurrentCreateRepo) UpdateKeycloakProfile(ctx context.Context, u *model.User) error {
	r.updated = true
	r.existing = u
	return nil
}

var _ repository.UserRepository = (*meRepo)(nil)
var _ repository.UserRepository = (*concurrentCreateRepo)(nil)
