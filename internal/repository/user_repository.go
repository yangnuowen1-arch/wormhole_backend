package repository

import (
	"context"
	"errors"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("user not found")

// UserRepository 用户数据访问接口。
type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error)
	FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	UpdateKeycloakProfile(ctx context.Context, u *model.User) error
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造 UserRepository。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *userRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByKeycloakID 通过 OIDC sub 查找本地业务用户。sub 是身份关联的唯一依据，
// 不能用可被修改或复用的 username/email 做自动账号绑定。
func (r *userRepository) FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("keycloak_id = ?", keycloakID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Table(model.TableNameRole).
		Select("roles.*").
		Joins("JOIN user_role ON user_role.role_id = roles.id").
		Where("user_role.user_id = ?", userID).
		Order("roles.id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// UpdateKeycloakProfile 同步 Keycloak 提供的展示资料，并更新最近登录时间。
func (r *userRepository) UpdateKeycloakProfile(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"email":         u.Email,
		"nickname":      u.Nickname,
		"last_login_at": u.LastLoginAt,
	}).Error
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
