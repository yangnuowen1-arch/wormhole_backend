-- 0004_sync_initial_user_roles.sql
-- 现有账号角色初始化：仅 id=1 且 username=test 的账号为管理员，其余账号均为普通用户。
-- 本脚本幂等；执行后 user_role 中每个现有 users 账号都只保留一条符合该规则的角色映射。

BEGIN;

-- 兼容尚未运行基础种子数据的数据库。
INSERT INTO roles (code, name, description)
VALUES
    ('admin', '管理员', '可以维护资源中心、首页推荐、幻灯片和快捷入口配置'),
    ('user', '普通用户', '可以访问工作台、资源中心和个人常用工具')
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description;

-- 先补齐每个已有用户应有的角色。
WITH desired_roles AS (
    SELECT
        u.id AS user_id,
        r.id AS role_id
    FROM users AS u
    JOIN roles AS r
        ON r.code = CASE
            WHEN u.id = 1 AND u.username = 'test' THEN 'admin'
            ELSE 'user'
        END
)
INSERT INTO user_role (user_id, role_id)
SELECT user_id, role_id
FROM desired_roles
ON CONFLICT (user_id, role_id) DO NOTHING;

-- 清理与上述规则不匹配的旧角色记录，确保当前每个用户只有指定角色。
WITH desired_roles AS (
    SELECT
        u.id AS user_id,
        r.id AS role_id
    FROM users AS u
    JOIN roles AS r
        ON r.code = CASE
            WHEN u.id = 1 AND u.username = 'test' THEN 'admin'
            ELSE 'user'
        END
)
DELETE FROM user_role AS ur
WHERE NOT EXISTS (
    SELECT 1
    FROM desired_roles AS desired
    WHERE desired.user_id = ur.user_id
      AND desired.role_id = ur.role_id
);

COMMIT;
