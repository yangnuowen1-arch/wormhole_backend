package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/keycloak"
	"github.com/yang/wormhole_backend/internal/repository"
)

const (
	maxKeycloakIDLength = 64
	maxUsernameLength   = 64
	maxEmailLength      = 128
	maxNicknameLength   = 64
	maxRoleCodeLength   = 32
	maxRolesPerUser     = 20
)

// UserService 用户业务逻辑接口。
type UserService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (dto.UserResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error)
	LoginWithKeycloak(ctx context.Context, identity keycloak.Identity) (dto.LoginResponse, error)
	Me(ctx context.Context) (dto.UserResponse, error)
	ListUsers(ctx context.Context) ([]dto.UserResponse, error)
	GetUser(ctx context.Context, userID int64) (dto.UserResponse, error)
	CreateUser(ctx context.Context, req dto.CreateAdminUserRequest) (dto.UserResponse, error)
	UpdateUser(ctx context.Context, userID int64, req dto.UpdateAdminUserRequest) (dto.UserResponse, error)
	DeleteUser(ctx context.Context, userID int64) error
	AssignRoles(ctx context.Context, userID int64, req dto.AssignUserRolesRequest) (dto.UserResponse, error)
}

type userService struct {
	repo      repository.UserRepository
	adminRepo repository.AdminUserRepository
	cfg       *config.Config
}

// NewUserService 构造 UserService。
func NewUserService(repo repository.UserRepository, cfg *config.Config) UserService {
	adminRepo, _ := repo.(repository.AdminUserRepository)
	return &userService{repo: repo, adminRepo: adminRepo, cfg: cfg}
}

// Register 注册新用户，用户名唯一。
// 注：只有未启用 Keycloak SSO 的兼容模式会注册此接口。启用 SSO 后用户应在
// Keycloak 中注册或由管理员创建，本项目只在首次成功 SSO 后建立业务资料。
func (s *userService) Register(ctx context.Context, req dto.RegisterRequest) (dto.UserResponse, error) {
	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if exists {
		return dto.UserResponse{}, ErrUsernameTaken
	}

	u := &model.User{
		KeycloakID: uuid.NewString(),
		Username:   req.Username,
		Email:      optionalStr(req.Email),
		Nickname:   optionalStr(req.Nickname),
	}
	if err := s.repo.CreateWithDefaultRole(ctx, u); err != nil {
		return dto.UserResponse{}, err
	}

	return toUserResponse(u), nil
}

// Login 校验用户存在并签发 JWT。
// 注：这是 Keycloak 未启用时的兼容逻辑。启用 Keycloak 时路由不会暴露此接口，
// 密码和 SSO 登录均由 Keycloak 处理。
func (s *userService) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	u, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return dto.LoginResponse{}, ErrInvalidCredentials
		}
		return dto.LoginResponse{}, err
	}

	return s.issueSession(u)
}

// LoginWithKeycloak 将已验证的 OIDC sub 映射到本地业务用户，并签发本应用的会话。
// 绝不按 username 或 email 将一个 SSO 身份自动关联到已有账号，以免发生账号接管。
func (s *userService) LoginWithKeycloak(ctx context.Context, identity keycloak.Identity) (dto.LoginResponse, error) {
	if identity.Subject == "" || utf8.RuneCountInString(identity.Subject) > maxKeycloakIDLength {
		return dto.LoginResponse{}, ErrInvalidKeycloakIdentity
	}

	u, err := s.repo.FindByKeycloakID(ctx, identity.Subject)
	switch {
	case err == nil:
		if !isActiveUser(u) {
			return dto.LoginResponse{}, ErrUserDisabled
		}
		u, err = s.syncKeycloakProfile(ctx, u, identity)
		if err != nil {
			return dto.LoginResponse{}, err
		}

	case errors.Is(err, repository.ErrUserNotFound):
		u, err = s.createKeycloakUser(ctx, identity)
		if err != nil {
			return dto.LoginResponse{}, err
		}

	default:
		return dto.LoginResponse{}, err
	}

	return s.issueSession(u)
}

func (s *userService) createKeycloakUser(ctx context.Context, identity keycloak.Identity) (*model.User, error) {
	username, err := s.newKeycloakUsername(ctx, identity)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &model.User{
		KeycloakID:  identity.Subject,
		Username:    username,
		Email:       optionalStr(trimRunes(identity.Email, maxEmailLength)),
		Nickname:    optionalStr(trimRunes(displayName(identity), maxNicknameLength)),
		LastLoginAt: &now,
	}
	if err := s.repo.CreateWithDefaultRole(ctx, u); err != nil {
		existing, findErr := s.repo.FindByKeycloakID(ctx, identity.Subject)
		if findErr == nil {
			return s.syncKeycloakProfile(ctx, existing, identity)
		}
		if errors.Is(findErr, repository.ErrUserNotFound) {
			return nil, err
		}
		return nil, errors.Join(err, findErr)
	}
	return u, nil
}

func (s *userService) syncKeycloakProfile(ctx context.Context, u *model.User, identity keycloak.Identity) (*model.User, error) {
	now := time.Now().UTC()
	u.Email = optionalStr(trimRunes(identity.Email, maxEmailLength))
	u.Nickname = optionalStr(trimRunes(displayName(identity), maxNicknameLength))
	u.LastLoginAt = &now
	if err := s.repo.UpdateKeycloakProfile(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *userService) issueSession(u *model.User) (dto.LoginResponse, error) {
	if !isActiveUser(u) {
		return dto.LoginResponse{}, ErrUserDisabled
	}
	token, err := auth.GenerateToken(s.cfg.JWTSecret, s.cfg.JWTExpireHrs, u.ID, u.Username)
	if err != nil {
		return dto.LoginResponse{}, err
	}
	return dto.LoginResponse{
		Token: token,
		User:  toUserResponse(u),
	}, nil
}

// newKeycloakUsername 使用 Keycloak 用户名初始化业务用户名。若旧的本地账号恰好
// 占用同名用户名，则增加由 sub 派生的后缀，而不是错误地把两个账号绑定在一起。
func (s *userService) newKeycloakUsername(ctx context.Context, identity keycloak.Identity) (string, error) {
	base := trimRunes(strings.TrimSpace(identity.PreferredUsername), maxUsernameLength)
	if base == "" && identity.Email != "" {
		localPart, _, _ := strings.Cut(identity.Email, "@")
		base = trimRunes(strings.TrimSpace(localPart), maxUsernameLength)
	}
	if base == "" {
		base = "kc-" + subjectSuffix(identity.Subject)
	}

	exists, err := s.repo.ExistsByUsername(ctx, base)
	if err != nil {
		return "", err
	}
	if !exists {
		return base, nil
	}

	suffix := "-" + subjectSuffix(identity.Subject)
	candidate := trimRunes(base, maxUsernameLength-utf8.RuneCountInString(suffix)) + suffix
	exists, err = s.repo.ExistsByUsername(ctx, candidate)
	if err != nil {
		return "", err
	}
	if exists {
		return "", ErrKeycloakUsernameConflict
	}
	return candidate, nil
}

func displayName(identity keycloak.Identity) string {
	if name := strings.TrimSpace(identity.Name); name != "" {
		return name
	}
	return strings.TrimSpace(identity.PreferredUsername)
}

func subjectSuffix(subject string) string {
	sum := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(sum[:])[:10]
}

// Me 返回当前登录用户信息。
func (s *userService) Me(ctx context.Context) (dto.UserResponse, error) {
	uid, err := currentUserID(ctx)
	if err != nil {
		return dto.UserResponse{}, err
	}
	u, err := s.repo.FindByID(ctx, uid)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if !isActiveUser(u) {
		return dto.UserResponse{}, ErrUserDisabled
	}
	roles, err := s.repo.FindRolesByUserID(ctx, uid)
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponseWithRoles(u, roles), nil
}

// ListUsers 返回所有本地用户及其角色，仅管理员可访问。
func (s *userService) ListUsers(ctx context.Context) ([]dto.UserResponse, error) {
	if _, err := requireAdminRole(ctx, s.repo); err != nil {
		return nil, err
	}
	adminRepo, err := s.requireAdminUserRepository()
	if err != nil {
		return nil, err
	}

	users, err := adminRepo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	rolesByUserID, err := adminRepo.FindRolesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.UserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, toUserResponseWithRoles(&user, rolesByUserID[user.ID]))
	}
	return responses, nil
}

// GetUser 返回一个用户及其角色，仅管理员可访问。
func (s *userService) GetUser(ctx context.Context, userID int64) (dto.UserResponse, error) {
	if _, err := requireAdminRole(ctx, s.repo); err != nil {
		return dto.UserResponse{}, err
	}
	if userID <= 0 {
		return dto.UserResponse{}, ErrUserNotFound
	}
	user, err := s.repo.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return dto.UserResponse{}, ErrUserNotFound
	}
	if err != nil {
		return dto.UserResponse{}, err
	}
	roles, err := s.repo.FindRolesByUserID(ctx, userID)
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponseWithRoles(user, roles), nil
}

// CreateUser 创建本地业务用户和角色绑定。SSO 模式下调用方必须传入已有的
// Keycloak subject，避免创建无法映射到真实身份的孤立账号。
func (s *userService) CreateUser(ctx context.Context, req dto.CreateAdminUserRequest) (dto.UserResponse, error) {
	if _, err := requireAdminRole(ctx, s.repo); err != nil {
		return dto.UserResponse{}, err
	}
	adminRepo, err := s.requireAdminUserRepository()
	if err != nil {
		return dto.UserResponse{}, err
	}

	username, err := normalizeUsername(req.Username)
	if err != nil {
		return dto.UserResponse{}, err
	}
	email, nickname, avatar, status, err := normalizeAdminUserProfile(req.Email, req.Nickname, req.Avatar, req.Status)
	if err != nil {
		return dto.UserResponse{}, err
	}
	keycloakID := strings.TrimSpace(req.KeycloakID)
	if keycloakID == "" {
		if s.cfg != nil && s.cfg.KeycloakEnabled {
			return dto.UserResponse{}, ErrKeycloakIDRequired
		}
		keycloakID = uuid.NewString()
	}
	if utf8.RuneCountInString(keycloakID) > maxKeycloakIDLength {
		return dto.UserResponse{}, ErrInvalidUser
	}

	usernameExists, err := s.repo.ExistsByUsername(ctx, username)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if usernameExists {
		return dto.UserResponse{}, ErrUsernameTaken
	}
	if _, err := s.repo.FindByKeycloakID(ctx, keycloakID); err == nil {
		return dto.UserResponse{}, ErrKeycloakIDTaken
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return dto.UserResponse{}, err
	}

	roleCodes := req.RoleCodes
	if len(roleCodes) == 0 {
		roleCodes = []string{"user"}
	}
	roles, roleIDs, err := s.resolveRoles(ctx, roleCodes)
	if err != nil {
		return dto.UserResponse{}, err
	}

	user := &model.User{
		KeycloakID: keycloakID,
		Username:   username,
		Email:      optionalStr(email),
		Nickname:   optionalStr(nickname),
		Avatar:     optionalStr(avatar),
		Status:     &status,
	}
	if err := adminRepo.CreateWithRoles(ctx, user, roleIDs); err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponseWithRoles(user, roles), nil
}

// UpdateUser 部分更新用户的资料或启用状态。角色请使用 AssignRoles 管理，避免
// 角色替换和资料更新混在一个请求中。
func (s *userService) UpdateUser(ctx context.Context, userID int64, req dto.UpdateAdminUserRequest) (dto.UserResponse, error) {
	adminID, err := requireAdminRole(ctx, s.repo)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if userID <= 0 {
		return dto.UserResponse{}, ErrUserNotFound
	}
	adminRepo, err := s.requireAdminUserRepository()
	if err != nil {
		return dto.UserResponse{}, err
	}

	updates := make(map[string]any, 5)
	if req.Username != nil {
		username, err := normalizeUsername(*req.Username)
		if err != nil {
			return dto.UserResponse{}, err
		}
		user, err := s.repo.FindByID(ctx, userID)
		if errors.Is(err, repository.ErrUserNotFound) {
			return dto.UserResponse{}, ErrUserNotFound
		}
		if err != nil {
			return dto.UserResponse{}, err
		}
		if user.Username != username {
			exists, err := s.repo.ExistsByUsername(ctx, username)
			if err != nil {
				return dto.UserResponse{}, err
			}
			if exists {
				return dto.UserResponse{}, ErrUsernameTaken
			}
		}
		updates["username"] = username
	}
	if req.Email != nil {
		email, err := normalizeOptionalEmail(*req.Email)
		if err != nil {
			return dto.UserResponse{}, err
		}
		updates["email"] = optionalStr(email)
	}
	if req.Nickname != nil {
		nickname := strings.TrimSpace(*req.Nickname)
		if utf8.RuneCountInString(nickname) > maxNicknameLength {
			return dto.UserResponse{}, ErrInvalidUser
		}
		updates["nickname"] = optionalStr(nickname)
	}
	if req.Avatar != nil {
		avatar := strings.TrimSpace(*req.Avatar)
		if utf8.RuneCountInString(avatar) > 255 {
			return dto.UserResponse{}, ErrInvalidUser
		}
		updates["avatar"] = optionalStr(avatar)
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return dto.UserResponse{}, ErrInvalidUser
		}
		if userID == adminID && *req.Status == statusDisabled {
			return dto.UserResponse{}, ErrSelfUserModification
		}
		updates["status"] = *req.Status
	}
	if len(updates) == 0 {
		return dto.UserResponse{}, ErrInvalidUser
	}
	updates["updated_at"] = time.Now().UTC()

	user, err := adminRepo.UpdateUser(ctx, userID, updates)
	if errors.Is(err, repository.ErrUserNotFound) {
		return dto.UserResponse{}, ErrUserNotFound
	}
	if err != nil {
		return dto.UserResponse{}, err
	}
	roles, err := s.repo.FindRolesByUserID(ctx, userID)
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponseWithRoles(user, roles), nil
}

// DeleteUser 逻辑删除用户；保留 Keycloak subject 并停用账号，确保该身份无法在
// 后续 SSO 登录时自动创建一个新的本地账号。
func (s *userService) DeleteUser(ctx context.Context, userID int64) error {
	adminID, err := requireAdminRole(ctx, s.repo)
	if err != nil {
		return err
	}
	if userID <= 0 {
		return ErrUserNotFound
	}
	if userID == adminID {
		return ErrSelfUserModification
	}
	adminRepo, err := s.requireAdminUserRepository()
	if err != nil {
		return err
	}
	if err := adminRepo.DisableUser(ctx, userID); errors.Is(err, repository.ErrUserNotFound) {
		return ErrUserNotFound
	} else {
		return err
	}
}

// AssignRoles 由管理员完整替换指定用户的角色集合。
func (s *userService) AssignRoles(ctx context.Context, userID int64, req dto.AssignUserRolesRequest) (dto.UserResponse, error) {
	adminID, err := requireAdminRole(ctx, s.repo)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if userID <= 0 {
		return dto.UserResponse{}, ErrUserNotFound
	}

	roleCodes, err := normalizeAssignedRoleCodes(req.RoleCodes)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if userID == adminID && !containsRoleCode(roleCodes, "admin") {
		return dto.UserResponse{}, ErrSelfUserModification
	}

	user, err := s.repo.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return dto.UserResponse{}, ErrUserNotFound
	}
	if err != nil {
		return dto.UserResponse{}, err
	}

	roles, roleIDs, err := s.resolveRoles(ctx, roleCodes)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if err := s.repo.ReplaceRoles(ctx, userID, roleIDs); err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponseWithRoles(user, roles), nil
}

func toUserResponse(u *model.User) dto.UserResponse {
	status := statusEnabled
	if u.Status != nil {
		status = *u.Status
	}
	return dto.UserResponse{
		ID:          u.ID,
		KeycloakID:  u.KeycloakID,
		Username:    u.Username,
		Email:       derefStr(u.Email),
		Nickname:    derefStr(u.Nickname),
		Avatar:      derefStr(u.Avatar),
		Status:      status,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		Roles:       []dto.RoleResponse{},
	}
}

func toUserResponseWithRoles(u *model.User, roles []model.Role) dto.UserResponse {
	resp := toUserResponse(u)
	resp.Roles = toRoleResponses(roles)
	return resp
}

func toRoleResponses(roles []model.Role) []dto.RoleResponse {
	resp := make([]dto.RoleResponse, 0, len(roles))
	for _, role := range roles {
		resp = append(resp, dto.RoleResponse{
			ID:          role.ID,
			Code:        role.Code,
			Name:        role.Name,
			Description: derefStr(role.Description),
		})
	}
	return resp
}

// optionalStr 把空字符串视为 NULL，非空返回其指针。
func optionalStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// derefStr 安全解引用 *string，nil 返回空串。
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func trimRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	return string([]rune(value)[:max])
}

func normalizeAssignedRoleCodes(roleCodes []string) ([]string, error) {
	if len(roleCodes) == 0 || len(roleCodes) > maxRolesPerUser {
		return nil, ErrInvalidRoleAssignment
	}

	seen := make(map[string]struct{}, len(roleCodes))
	codes := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleCode = strings.ToLower(strings.TrimSpace(roleCode))
		if roleCode == "" || utf8.RuneCountInString(roleCode) > maxRoleCodeLength {
			return nil, ErrInvalidRoleAssignment
		}
		if _, exists := seen[roleCode]; exists {
			return nil, ErrInvalidRoleAssignment
		}
		seen[roleCode] = struct{}{}
		codes = append(codes, roleCode)
	}
	return codes, nil
}

func (s *userService) resolveRoles(ctx context.Context, roleCodes []string) ([]model.Role, []int32, error) {
	normalized, err := normalizeAssignedRoleCodes(roleCodes)
	if err != nil {
		return nil, nil, err
	}
	roles, err := s.repo.FindRolesByCodes(ctx, normalized)
	if err != nil {
		return nil, nil, err
	}
	if len(roles) != len(normalized) {
		return nil, nil, ErrRoleNotFound
	}
	roleIDs := make([]int32, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}
	return roles, roleIDs, nil
}

func (s *userService) requireAdminUserRepository() (repository.AdminUserRepository, error) {
	if s.adminRepo == nil {
		return nil, ErrAdminUserStoreUnavailable
	}
	return s.adminRepo, nil
}

func normalizeUsername(value string) (string, error) {
	value = strings.TrimSpace(value)
	if count := utf8.RuneCountInString(value); count < 3 || count > maxUsernameLength {
		return "", ErrInvalidUser
	}
	return value, nil
}

func normalizeAdminUserProfile(email, nickname, avatar string, status *int16) (string, string, string, int16, error) {
	normalizedEmail, err := normalizeOptionalEmail(email)
	if err != nil {
		return "", "", "", 0, err
	}
	nickname = strings.TrimSpace(nickname)
	avatar = strings.TrimSpace(avatar)
	if utf8.RuneCountInString(nickname) > maxNicknameLength || utf8.RuneCountInString(avatar) > 255 {
		return "", "", "", 0, ErrInvalidUser
	}
	resolvedStatus := statusEnabled
	if status != nil {
		resolvedStatus = *status
	}
	if !isValidStatus(resolvedStatus) {
		return "", "", "", 0, ErrInvalidUser
	}
	return normalizedEmail, nickname, avatar, resolvedStatus, nil
}

func normalizeOptionalEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxEmailLength {
		return "", ErrInvalidUser
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", ErrInvalidUser
	}
	return value, nil
}

func containsRoleCode(codes []string, target string) bool {
	for _, code := range codes {
		if strings.EqualFold(code, target) {
			return true
		}
	}
	return false
}

func isActiveUser(user *model.User) bool {
	return user != nil && (user.Status == nil || *user.Status == statusEnabled)
}
