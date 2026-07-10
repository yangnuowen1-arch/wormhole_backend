-- 0001_create_users.sql
-- 认证采用 Keycloak：本地库不存密码，仅保存 keycloak_id 与业务资料。
CREATE TABLE IF NOT EXISTS users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    keycloak_id   VARCHAR(64)  NOT NULL,
    username      VARCHAR(64)  NOT NULL,
    email         VARCHAR(128),
    nickname      VARCHAR(64),
    avatar        VARCHAR(255),
    status        SMALLINT     NOT NULL DEFAULT 1, -- 1=Active, 0=Disabled
    last_login_at TIMESTAMP,
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_keycloak_id UNIQUE (keycloak_id),
    CONSTRAINT uk_username UNIQUE (username)
);
CREATE INDEX IF NOT EXISTS idx_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_user_status ON users (status);
CREATE INDEX IF NOT EXISTS idx_user_created_at ON users (created_at);

CREATE TABLE IF NOT EXISTS roles (
    id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(32)  NOT NULL,
    name        VARCHAR(64)  NOT NULL,
    description VARCHAR(255),
    CONSTRAINT uk_code UNIQUE (code)
);

CREATE TABLE IF NOT EXISTS user_role (
    user_id BIGINT  NOT NULL,
    role_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_role ON user_role (role_id);
