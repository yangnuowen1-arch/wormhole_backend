package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
)

// UserService 用户业务逻辑接口。
type UserService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (dto.UserResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error)
	LoginWithKeycloak(ctx context.Context, identity keycloak.Identity) (dto.LoginResponse, error)
	Me(ctx context.Context) (dto.UserResponse, error)
}

type userService struct {
	repo repository.UserRepository
	cfg  *config.Config
}

// NewUserService 构造 UserService。
func NewUserService(repo repository.UserRepository, cfg *config.Config) UserService {
	return &userService{repo: repo, cfg: cfg}
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
	if err := s.repo.Create(ctx, u); err != nil {
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
		now := time.Now().UTC()
		u.Email = optionalStr(trimRunes(identity.Email, maxEmailLength))
		u.Nickname = optionalStr(trimRunes(displayName(identity), maxNicknameLength))
		u.LastLoginAt = &now
		if err := s.repo.UpdateKeycloakProfile(ctx, u); err != nil {
			return dto.LoginResponse{}, err
		}

	case errors.Is(err, repository.ErrUserNotFound):
		username, usernameErr := s.newKeycloakUsername(ctx, identity)
		if usernameErr != nil {
			return dto.LoginResponse{}, usernameErr
		}
		now := time.Now().UTC()
		u = &model.User{
			KeycloakID:  identity.Subject,
			Username:    username,
			Email:       optionalStr(trimRunes(identity.Email, maxEmailLength)),
			Nickname:    optionalStr(trimRunes(displayName(identity), maxNicknameLength)),
			LastLoginAt: &now,
		}
		if err := s.repo.Create(ctx, u); err != nil {
			return dto.LoginResponse{}, err
		}

	default:
		return dto.LoginResponse{}, err
	}

	return s.issueSession(u)
}

func (s *userService) issueSession(u *model.User) (dto.LoginResponse, error) {
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
	return toUserResponse(u), nil
}

func toUserResponse(u *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:       u.ID,
		Username: u.Username,
		Email:    derefStr(u.Email),
		Nickname: derefStr(u.Nickname),
	}
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
