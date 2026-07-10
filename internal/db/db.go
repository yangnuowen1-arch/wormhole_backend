package db

import (
	"fmt"
	"log"

	"github.com/yang/wormhole_backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectDB 建立 PostgreSQL 连接并返回 *gorm.DB，失败直接终止进程。
func ConnectDB(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// 启动时执行幂等迁移，保证表结构就绪。
	ensureSchema(gdb)

	return gdb
}

// ensureSchema 幂等地创建所需表（若不存在），供本地开发免手动建表。
// 结构与 migrations/0001_create_users.sql 保持一致（Keycloak 认证方案）。
func ensureSchema(db *gorm.DB) {
	sql := `
CREATE TABLE IF NOT EXISTS users (
	id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	keycloak_id   VARCHAR(64)  NOT NULL,
	username      VARCHAR(64)  NOT NULL,
	email         VARCHAR(128),
	nickname      VARCHAR(64),
	avatar        VARCHAR(255),
	status        SMALLINT     NOT NULL DEFAULT 1,
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
CREATE INDEX IF NOT EXISTS idx_role ON user_role (role_id);`
	if err := db.Exec(sql).Error; err != nil {
		log.Fatalf("初始化数据库表失败: %v", err)
	}
}
