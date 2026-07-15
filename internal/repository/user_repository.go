package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/yang/wormhole_backend/internal/dal/model"
	"gorm.io/gorm"
)

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("user not found")

const defaultUserRoleCode = "user"

// UserRepository 用户数据访问接口。
type UserRepository interface {
	CreateWithDefaultRole(ctx context.Context, u *model.User) error
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByKeycloakID(ctx context.Context, keycloakID string) (*model.User, error)
	FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error)
	FindRolesByCodes(ctx context.Context, codes []string) ([]model.Role, error)
	ReplaceRoles(ctx context.Context, userID int64, roleIDs []int32) error
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	UpdateKeycloakProfile(ctx context.Context, u *model.User) error
}

// AdminUserRepository 是管理端用户维护所需的数据访问能力。它与 UserRepository
// 分开声明，以免普通认证流程的测试替身被不必要的管理能力耦合。
type AdminUserRepository interface {
	ListUsers(ctx context.Context) ([]model.User, error)
	FindRolesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.Role, error)
	CreateWithRoles(ctx context.Context, u *model.User, roleIDs []int32) error
	UpdateUser(ctx context.Context, userID int64, updates map[string]any) (*model.User, error)
	DisableUser(ctx context.Context, userID int64) error
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造 UserRepository。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// CreateWithDefaultRole 创建用户并在同一事务中授予 user 角色，避免出现已创建但没有角色的账号。
func (r *userRepository) CreateWithDefaultRole(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var defaultRole model.Role
		if err := tx.Where("code = ?", defaultUserRoleCode).First(&defaultRole).Error; err != nil {
			return fmt.Errorf("find default user role: %w", err)
		}
		return createUserWithRoleIDs(tx, u, []int32{defaultRole.ID})
	})
}

// CreateWithRoles 创建用户并在同一事务中写入完整角色集合。
func (r *userRepository) CreateWithRoles(ctx context.Context, u *model.User, roleIDs []int32) error {
	if len(roleIDs) == 0 {
		return errors.New("at least one role is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createUserWithRoleIDs(tx, u, roleIDs)
	})
}

func createUserWithRoleIDs(tx *gorm.DB, u *model.User, roleIDs []int32) error {
	if err := tx.Create(u).Error; err != nil {
		return err
	}

	assignments := make([]model.UserRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		assignments = append(assignments, model.UserRole{UserID: u.ID, RoleID: roleID})
	}
	if err := tx.Create(&assignments).Error; err != nil {
		return fmt.Errorf("assign user roles: %w", err)
	}
	return nil
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

// FindRolesByUserIDs 批量查询多个用户的角色，供管理员列表避免 N+1 查询。
func (r *userRepository) FindRolesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.Role, error) {
	rolesByUserID := make(map[int64][]model.Role, len(userIDs))
	if len(userIDs) == 0 {
		return rolesByUserID, nil
	}
	for _, userID := range userIDs {
		rolesByUserID[userID] = []model.Role{}
	}

	type userRoleRow struct {
		UserID      int64   `gorm:"column:user_id"`
		ID          int32   `gorm:"column:id"`
		Code        string  `gorm:"column:code"`
		Name        string  `gorm:"column:name"`
		Description *string `gorm:"column:description"`
	}
	var rows []userRoleRow
	err := r.db.WithContext(ctx).
		Table(model.TableNameUserRole).
		Select("user_role.user_id, roles.id, roles.code, roles.name, roles.description").
		Joins("JOIN roles ON roles.id = user_role.role_id").
		Where("user_role.user_id IN ?", userIDs).
		Order("user_role.user_id ASC, roles.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		rolesByUserID[row.UserID] = append(rolesByUserID[row.UserID], model.Role{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			Description: row.Description,
		})
	}
	return rolesByUserID, nil
}

// FindRolesByCodes 按角色编码查询角色，用于管理员分配角色前的完整性校验。
func (r *userRepository) FindRolesByCodes(ctx context.Context, codes []string) ([]model.Role, error) {
	if len(codes) == 0 {
		return []model.Role{}, nil
	}

	var roles []model.Role
	err := r.db.WithContext(ctx).
		Where("code IN ?", codes).
		Order("id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// ReplaceRoles 在单个事务中替换用户的全部角色，避免角色集合只更新了一部分。
func (r *userRepository) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int32) error {
	if len(roleIDs) == 0 {
		return errors.New("at least one role is required")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}

		assignments := make([]model.UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			assignments = append(assignments, model.UserRole{UserID: userID, RoleID: roleID})
		}
		return tx.Create(&assignments).Error
	})
}

// ListUsers 返回所有本地业务用户，供管理员成员管理页面使用。
func (r *userRepository) ListUsers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).Order("id ASC").Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUser 部分更新管理员可维护的用户资料，并返回最新记录。
func (r *userRepository) UpdateUser(ctx context.Context, userID int64, updates map[string]any) (*model.User, error) {
	if len(updates) == 0 {
		return r.FindByID(ctx, userID)
	}

	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrUserNotFound
	}
	return r.FindByID(ctx, userID)
}

// DisableUser 逻辑删除用户。保留其 Keycloak subject，才能阻止该身份在下一次 SSO
// 登录时被自动当作新用户重新创建。
func (r *userRepository) DisableUser(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"status":     int16(0),
		"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
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
