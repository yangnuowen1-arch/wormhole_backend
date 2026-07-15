package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
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

func TestRegisterCreatesUserWithDefaultRole(t *testing.T) {
	repo := &defaultRoleRepo{}
	svc := NewUserService(repo, &config.Config{})

	resp, err := svc.Register(context.Background(), dto.RegisterRequest{
		Username: "alice",
		Email:    "alice@example.com",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if !repo.createdWithDefaultRole {
		t.Fatal("expected Register to create the user with the default role")
	}
	if repo.created == nil || repo.created.Username != "alice" {
		t.Fatalf("created user = %+v, want username alice", repo.created)
	}
	if resp.Username != "alice" {
		t.Fatalf("response username = %q, want alice", resp.Username)
	}
}

func TestLoginWithKeycloakCreatesUserWithDefaultRole(t *testing.T) {
	repo := &defaultRoleRepo{}
	svc := NewUserService(repo, &config.Config{
		JWTSecret:    "abcdef0123456789abcdef0123456789",
		JWTExpireHrs: 1,
	})

	_, err := svc.LoginWithKeycloak(context.Background(), keycloak.Identity{
		Subject:           "kc-subject",
		PreferredUsername: "alice",
	})
	if err != nil {
		t.Fatalf("LoginWithKeycloak returned error: %v", err)
	}
	if !repo.createdWithDefaultRole {
		t.Fatal("expected new Keycloak user to receive the default role")
	}
}

func TestAssignRolesReplacesTargetUserRoles(t *testing.T) {
	adminRole := model.Role{ID: 1, Code: "admin", Name: "管理员"}
	userRole := model.Role{ID: 2, Code: "user", Name: "普通用户"}
	repo := &assignRolesRepo{
		users: map[int64]*model.User{
			7: {ID: 7, Username: "admin"},
			8: {ID: 8, Username: "alice"},
		},
		userRoles: map[int64][]model.Role{7: {adminRole}},
		roles: map[string]model.Role{
			"admin": adminRole,
			"user":  userRole,
		},
	}
	svc := NewUserService(repo, &config.Config{})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	resp, err := svc.AssignRoles(ctx, 8, dto.AssignUserRolesRequest{RoleCodes: []string{" admin ", "user"}})
	if err != nil {
		t.Fatalf("AssignRoles returned error: %v", err)
	}
	if repo.replacedUserID != 8 {
		t.Fatalf("replaced user ID = %d, want 8", repo.replacedUserID)
	}
	if len(repo.replacedRoleIDs) != 2 || repo.replacedRoleIDs[0] != 1 || repo.replacedRoleIDs[1] != 2 {
		t.Fatalf("replaced role IDs = %v, want [1 2]", repo.replacedRoleIDs)
	}
	if resp.ID != 8 || len(resp.Roles) != 2 || resp.Roles[0].Code != "admin" || resp.Roles[1].Code != "user" {
		t.Fatalf("response = %+v, want alice with admin and user roles", resp)
	}
}

func TestAssignRolesRejectsUnknownRoleWithoutReplacing(t *testing.T) {
	adminRole := model.Role{ID: 1, Code: "admin", Name: "管理员"}
	repo := &assignRolesRepo{
		users:     map[int64]*model.User{7: {ID: 7, Username: "admin"}, 8: {ID: 8, Username: "alice"}},
		userRoles: map[int64][]model.Role{7: {adminRole}},
		roles:     map[string]model.Role{"admin": adminRole},
	}
	svc := NewUserService(repo, &config.Config{})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	_, err := svc.AssignRoles(ctx, 8, dto.AssignUserRolesRequest{RoleCodes: []string{"does-not-exist"}})
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("AssignRoles error = %v, want ErrRoleNotFound", err)
	}
	if repo.replaced {
		t.Fatal("ReplaceRoles should not be called for an unknown role")
	}
}

func TestAssignRolesRequiresAdminRole(t *testing.T) {
	userRole := model.Role{ID: 2, Code: "user", Name: "普通用户"}
	repo := &assignRolesRepo{
		users:     map[int64]*model.User{7: {ID: 7, Username: "alice"}, 8: {ID: 8, Username: "bob"}},
		userRoles: map[int64][]model.Role{7: {userRole}},
		roles:     map[string]model.Role{"user": userRole},
	}
	svc := NewUserService(repo, &config.Config{})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "alice"})

	_, err := svc.AssignRoles(ctx, 8, dto.AssignUserRolesRequest{RoleCodes: []string{"user"}})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AssignRoles error = %v, want ErrForbidden", err)
	}
	if repo.replaced {
		t.Fatal("ReplaceRoles should not be called for a non-admin user")
	}
}

func TestAdminUserCRUD(t *testing.T) {
	adminRole := model.Role{ID: 1, Code: "admin", Name: "管理员"}
	userRole := model.Role{ID: 2, Code: "user", Name: "普通用户"}
	active := int16(1)
	repo := &adminUserRepo{assignRolesRepo: &assignRolesRepo{
		users: map[int64]*model.User{
			7: {ID: 7, KeycloakID: "admin-sub", Username: "admin", Status: &active},
			8: {ID: 8, KeycloakID: "alice-sub", Username: "alice", Status: &active},
		},
		userRoles: map[int64][]model.Role{
			7: {adminRole},
			8: {userRole},
		},
		roles: map[string]model.Role{
			"admin": adminRole,
			"user":  userRole,
		},
	}}
	svc := NewUserService(repo, &config.Config{KeycloakEnabled: true})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if len(users) != 2 || users[1].Username != "alice" || len(users[1].Roles) != 1 || users[1].Roles[0].Code != "user" {
		t.Fatalf("users = %+v, want admin and alice with roles", users)
	}

	created, err := svc.CreateUser(ctx, dto.CreateAdminUserRequest{
		KeycloakID: "bob-sub",
		Username:   "bob",
		Email:      "bob@example.com",
		RoleCodes:  []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if created.Username != "bob" || created.KeycloakID != "bob-sub" || len(created.Roles) != 1 || created.Roles[0].Code != "user" {
		t.Fatalf("created user = %+v, want bob with user role", created)
	}

	email := ""
	nickname := "Bobby"
	updated, err := svc.UpdateUser(ctx, created.ID, dto.UpdateAdminUserRequest{Email: &email, Nickname: &nickname})
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if updated.Email != "" || updated.Nickname != "Bobby" {
		t.Fatalf("updated user = %+v, want cleared email and nickname Bobby", updated)
	}

	if err := svc.DeleteUser(ctx, created.ID); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if repo.users[created.ID].Status == nil || *repo.users[created.ID].Status != 0 {
		t.Fatalf("deleted user status = %v, want disabled", repo.users[created.ID].Status)
	}
	if err := svc.DeleteUser(ctx, 7); !errors.Is(err, ErrSelfUserModification) {
		t.Fatalf("DeleteUser own account error = %v, want ErrSelfUserModification", err)
	}
}

func TestCreateUserRequiresKeycloakIDWhenSSOIsEnabled(t *testing.T) {
	adminRole := model.Role{ID: 1, Code: "admin", Name: "管理员"}
	repo := &adminUserRepo{assignRolesRepo: &assignRolesRepo{
		users:     map[int64]*model.User{7: {ID: 7, Username: "admin"}},
		userRoles: map[int64][]model.Role{7: {adminRole}},
		roles:     map[string]model.Role{"admin": adminRole},
	}}
	svc := NewUserService(repo, &config.Config{KeycloakEnabled: true})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	_, err := svc.CreateUser(ctx, dto.CreateAdminUserRequest{Username: "alice"})
	if !errors.Is(err, ErrKeycloakIDRequired) {
		t.Fatalf("CreateUser error = %v, want ErrKeycloakIDRequired", err)
	}
}

type meRepo struct {
	user  *model.User
	roles []model.Role
}

func (r *meRepo) CreateWithDefaultRole(ctx context.Context, u *model.User) error {
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

func (r *meRepo) FindRolesByCodes(ctx context.Context, codes []string) ([]model.Role, error) {
	return nil, nil
}

func (r *meRepo) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int32) error {
	return nil
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

func (r *concurrentCreateRepo) CreateWithDefaultRole(ctx context.Context, u *model.User) error {
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

func (r *concurrentCreateRepo) FindRolesByCodes(ctx context.Context, codes []string) ([]model.Role, error) {
	return nil, nil
}

func (r *concurrentCreateRepo) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int32) error {
	return nil
}

func (r *concurrentCreateRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (r *concurrentCreateRepo) UpdateKeycloakProfile(ctx context.Context, u *model.User) error {
	r.updated = true
	r.existing = u
	return nil
}

type defaultRoleRepo struct {
	created                *model.User
	createdWithDefaultRole bool
}

func (r *defaultRoleRepo) CreateWithDefaultRole(ctx context.Context, u *model.User) error {
	u.ID = 1
	r.created = u
	r.createdWithDefaultRole = true
	return nil
}

func (r *defaultRoleRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	if r.created == nil || r.created.ID != id {
		return nil, repository.ErrUserNotFound
	}
	return r.created, nil
}

func (r *defaultRoleRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *defaultRoleRepo) FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *defaultRoleRepo) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	return nil, nil
}

func (r *defaultRoleRepo) FindRolesByCodes(ctx context.Context, codes []string) ([]model.Role, error) {
	return nil, nil
}

func (r *defaultRoleRepo) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int32) error {
	return nil
}

func (r *defaultRoleRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (r *defaultRoleRepo) UpdateKeycloakProfile(ctx context.Context, u *model.User) error {
	return nil
}

type assignRolesRepo struct {
	users           map[int64]*model.User
	userRoles       map[int64][]model.Role
	roles           map[string]model.Role
	replaced        bool
	replacedUserID  int64
	replacedRoleIDs []int32
}

func (r *assignRolesRepo) CreateWithDefaultRole(ctx context.Context, u *model.User) error {
	return nil
}

func (r *assignRolesRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *assignRolesRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *assignRolesRepo) FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *assignRolesRepo) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	return r.userRoles[userID], nil
}

func (r *assignRolesRepo) FindRolesByCodes(ctx context.Context, codes []string) ([]model.Role, error) {
	roles := make([]model.Role, 0, len(codes))
	for _, code := range codes {
		if role, ok := r.roles[code]; ok {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (r *assignRolesRepo) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int32) error {
	r.replaced = true
	r.replacedUserID = userID
	r.replacedRoleIDs = append([]int32(nil), roleIDs...)
	return nil
}

func (r *assignRolesRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (r *assignRolesRepo) UpdateKeycloakProfile(ctx context.Context, u *model.User) error {
	return nil
}

type adminUserRepo struct {
	*assignRolesRepo
	nextID int64
}

func (r *adminUserRepo) ListUsers(ctx context.Context) ([]model.User, error) {
	ids := make([]int64, 0, len(r.users))
	for id := range r.users {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	users := make([]model.User, 0, len(ids))
	for _, id := range ids {
		users = append(users, *r.users[id])
	}
	return users, nil
}

func (r *adminUserRepo) FindRolesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.Role, error) {
	rolesByUserID := make(map[int64][]model.Role, len(userIDs))
	for _, userID := range userIDs {
		rolesByUserID[userID] = append([]model.Role(nil), r.userRoles[userID]...)
	}
	return rolesByUserID, nil
}

func (r *adminUserRepo) CreateWithRoles(ctx context.Context, u *model.User, roleIDs []int32) error {
	if r.nextID == 0 {
		r.nextID = 9
	}
	u.ID = r.nextID
	r.nextID++
	r.users[u.ID] = u
	roles := make([]model.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		for _, role := range r.roles {
			if role.ID == roleID {
				roles = append(roles, role)
				break
			}
		}
	}
	r.userRoles[u.ID] = roles
	return nil
}

func (r *adminUserRepo) UpdateUser(ctx context.Context, userID int64, updates map[string]any) (*model.User, error) {
	user, ok := r.users[userID]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	if username, ok := updates["username"].(string); ok {
		user.Username = username
	}
	if email, ok := updates["email"].(*string); ok {
		user.Email = email
	} else if _, exists := updates["email"]; exists {
		user.Email = nil
	}
	if nickname, ok := updates["nickname"].(*string); ok {
		user.Nickname = nickname
	} else if _, exists := updates["nickname"]; exists {
		user.Nickname = nil
	}
	if avatar, ok := updates["avatar"].(*string); ok {
		user.Avatar = avatar
	} else if _, exists := updates["avatar"]; exists {
		user.Avatar = nil
	}
	if status, ok := updates["status"].(int16); ok {
		user.Status = &status
	}
	return user, nil
}

func (r *adminUserRepo) DisableUser(ctx context.Context, userID int64) error {
	user, ok := r.users[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	disabled := int16(0)
	user.Status = &disabled
	return nil
}

var _ repository.UserRepository = (*meRepo)(nil)
var _ repository.UserRepository = (*concurrentCreateRepo)(nil)
var _ repository.UserRepository = (*defaultRoleRepo)(nil)
var _ repository.UserRepository = (*assignRolesRepo)(nil)
