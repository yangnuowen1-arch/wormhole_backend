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

// ensureSchema 幂等地创建所需表和基础种子数据，供本地开发免手动建表。
// 结构与 migrations/ 下的 SQL 保持一致。
func ensureSchema(db *gorm.DB) {
	schemaSQL := `
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
CREATE INDEX IF NOT EXISTS idx_role ON user_role (role_id);

CREATE TABLE IF NOT EXISTS resource_categories (
	id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	code        VARCHAR(64)  NOT NULL,
	name        VARCHAR(64)  NOT NULL,
	description VARCHAR(255),
	sort_order  INTEGER      NOT NULL DEFAULT 0,
	status      SMALLINT     NOT NULL DEFAULT 1,
	created_by  BIGINT,
	updated_by  BIGINT,
	created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_resource_categories_code UNIQUE (code),
	CONSTRAINT fk_resource_categories_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_resource_categories_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_resource_categories_status_sort ON resource_categories (status, sort_order, id);

CREATE TABLE IF NOT EXISTS resources (
	id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	category_id    INTEGER,
	slug           VARCHAR(128) NOT NULL,
	name           VARCHAR(128) NOT NULL,
	icon_url       VARCHAR(255),
	icon_text      VARCHAR(16),
	website_url    VARCHAR(512),
	summary        VARCHAR(255),
	description    TEXT,
	resource_type  VARCHAR(32)  NOT NULL DEFAULT 'tool',
	provider       VARCHAR(128),
	model_count    INTEGER      NOT NULL DEFAULT 0,
	follower_count INTEGER      NOT NULL DEFAULT 0,
	badge          VARCHAR(32),
	tags           JSONB        NOT NULL DEFAULT '[]'::jsonb,
	metadata       JSONB        NOT NULL DEFAULT '{}'::jsonb,
	is_featured    BOOLEAN      NOT NULL DEFAULT FALSE,
	sort_order     INTEGER      NOT NULL DEFAULT 0,
	status         SMALLINT     NOT NULL DEFAULT 1,
	created_by     BIGINT,
	updated_by     BIGINT,
	created_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_resources_slug UNIQUE (slug),
	CONSTRAINT fk_resources_category FOREIGN KEY (category_id) REFERENCES resource_categories(id) ON DELETE SET NULL,
	CONSTRAINT fk_resources_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_resources_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_resources_category_status_sort ON resources (category_id, status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_resources_status_sort ON resources (status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_resources_featured_sort ON resources (is_featured, status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_resources_search ON resources USING GIN (
	to_tsvector('simple', coalesce(name, '') || ' ' || coalesce(summary, '') || ' ' || coalesce(provider, '') || ' ' || coalesce(description, ''))
);
CREATE INDEX IF NOT EXISTS idx_resources_tags ON resources USING GIN (tags);

CREATE TABLE IF NOT EXISTS user_common_tools (
	user_id     BIGINT    NOT NULL,
	resource_id BIGINT    NOT NULL,
	sort_order  INTEGER   NOT NULL DEFAULT 0,
	created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (user_id, resource_id),
	CONSTRAINT fk_user_common_tools_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
	CONSTRAINT fk_user_common_tools_resource FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_common_tools_user_sort ON user_common_tools (user_id, sort_order, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_common_tools_resource ON user_common_tools (resource_id);

CREATE TABLE IF NOT EXISTS search_history (
	id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id           BIGINT       NOT NULL,
	query             VARCHAR(128) NOT NULL,
	search_count      INTEGER      NOT NULL DEFAULT 1,
	last_result_count INTEGER      NOT NULL DEFAULT 0,
	last_searched_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at        TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at        TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_search_history_user_query UNIQUE (user_id, query),
	CONSTRAINT fk_search_history_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_search_history_user_recent ON search_history (user_id, last_searched_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS quick_entries (
	id                 INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	code               VARCHAR(64)  NOT NULL,
	title              VARCHAR(64)  NOT NULL,
	icon_url           VARCHAR(255),
	icon_text          VARCHAR(16),
	target_url         VARCHAR(512) NOT NULL,
	description        VARCHAR(255),
	visible_role_codes JSONB        NOT NULL DEFAULT '[]'::jsonb,
	sort_order         INTEGER      NOT NULL DEFAULT 0,
	status             SMALLINT     NOT NULL DEFAULT 1,
	created_by         BIGINT,
	updated_by         BIGINT,
	created_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_quick_entries_code UNIQUE (code),
	CONSTRAINT fk_quick_entries_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_quick_entries_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_quick_entries_status_sort ON quick_entries (status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_quick_entries_visible_role_codes ON quick_entries USING GIN (visible_role_codes);

CREATE TABLE IF NOT EXISTS recommendation_items (
	id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	resource_id        BIGINT,
	title              VARCHAR(128) NOT NULL,
	subtitle           VARCHAR(128),
	source_name        VARCHAR(64),
	source_url         VARCHAR(512),
	icon_url           VARCHAR(255),
	icon_text          VARCHAR(16),
	target_url         VARCHAR(512),
	published_at       TIMESTAMP,
	visible_role_codes JSONB        NOT NULL DEFAULT '[]'::jsonb,
	sort_order         INTEGER      NOT NULL DEFAULT 0,
	status             SMALLINT     NOT NULL DEFAULT 1,
	created_by         BIGINT,
	updated_by         BIGINT,
	created_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_recommendation_items_resource FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE SET NULL,
	CONSTRAINT fk_recommendation_items_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_recommendation_items_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_status_sort ON recommendation_items (status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_published_at ON recommendation_items (published_at DESC);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_resource ON recommendation_items (resource_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_visible_role_codes ON recommendation_items USING GIN (visible_role_codes);

CREATE TABLE IF NOT EXISTS carousel_slides (
	id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	code               VARCHAR(64)  NOT NULL,
	title              VARCHAR(128) NOT NULL,
	subtitle           VARCHAR(128),
	description        VARCHAR(512),
	image_url          VARCHAR(512),
	background         VARCHAR(64),
	button_text        VARCHAR(32),
	target_url         VARCHAR(512),
	autoplay_seconds   INTEGER      NOT NULL DEFAULT 5,
	starts_at          TIMESTAMP,
	ends_at            TIMESTAMP,
	visible_role_codes JSONB        NOT NULL DEFAULT '[]'::jsonb,
	sort_order         INTEGER      NOT NULL DEFAULT 0,
	status             SMALLINT     NOT NULL DEFAULT 1,
	created_by         BIGINT,
	updated_by         BIGINT,
	created_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_carousel_slides_code UNIQUE (code),
	CONSTRAINT fk_carousel_slides_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_carousel_slides_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_carousel_slides_status_sort ON carousel_slides (status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_carousel_slides_window ON carousel_slides (starts_at, ends_at);
CREATE INDEX IF NOT EXISTS idx_carousel_slides_visible_role_codes ON carousel_slides USING GIN (visible_role_codes);

CREATE TABLE IF NOT EXISTS workspace_channels (
	id                 INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	code               VARCHAR(64)  NOT NULL,
	name               VARCHAR(64)  NOT NULL,
	description        VARCHAR(255),
	icon_url           VARCHAR(255),
	icon_text          VARCHAR(16),
	target_url         VARCHAR(512),
	visible_role_codes JSONB        NOT NULL DEFAULT '[]'::jsonb,
	sort_order         INTEGER      NOT NULL DEFAULT 0,
	status             SMALLINT     NOT NULL DEFAULT 1,
	created_by         BIGINT,
	updated_by         BIGINT,
	created_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_workspace_channels_code UNIQUE (code),
	CONSTRAINT fk_workspace_channels_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_workspace_channels_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_workspace_channels_status_sort ON workspace_channels (status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_workspace_channels_visible_role_codes ON workspace_channels USING GIN (visible_role_codes);

CREATE TABLE IF NOT EXISTS announcements (
	id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	title        VARCHAR(128) NOT NULL,
	content      TEXT         NOT NULL,
	is_pinned    BOOLEAN      NOT NULL DEFAULT FALSE,
	status       SMALLINT     NOT NULL DEFAULT 1,
	published_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at   TIMESTAMP,
	created_by   BIGINT,
	updated_by   BIGINT,
	created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_announcements_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
	CONSTRAINT fk_announcements_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_announcements_visibility ON announcements (status, is_pinned DESC, published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_announcements_expires_at ON announcements (expires_at);`
	if err := db.Exec(schemaSQL).Error; err != nil {
		log.Fatalf("初始化数据库表失败: %v", err)
	}
	seedDefaults(db)
}

func seedDefaults(db *gorm.DB) {
	seedSQL := `
INSERT INTO roles (code, name, description)
VALUES
	('admin', '管理员', '可以维护资源中心、首页推荐、幻灯片和快捷入口配置'),
	('user', '普通用户', '可以访问工作台、资源中心和个人常用工具')
ON CONFLICT (code) DO UPDATE SET
	name = EXCLUDED.name,
	description = EXCLUDED.description;

INSERT INTO resource_categories (code, name, description, sort_order, status)
VALUES
	('dev-tools', '开发工具', '研发、部署、代码托管与工程效率工具', 10, 1),
	('design', '设计', '设计协作、原型与视觉生产工具', 20, 1),
	('ai-ml', 'AI 与机器学习', 'AI 助手、模型平台与机器学习资源', 30, 1)
ON CONFLICT (code) DO UPDATE SET
	name = EXCLUDED.name,
	description = EXCLUDED.description,
	sort_order = EXCLUDED.sort_order,
	status = EXCLUDED.status,
	updated_at = CURRENT_TIMESTAMP;

INSERT INTO resources (
	category_id, slug, name, icon_text, website_url, summary, description, resource_type,
	provider, model_count, follower_count, badge, tags, metadata, is_featured, sort_order, status
)
VALUES
	((SELECT id FROM resource_categories WHERE code = 'ai-ml'), 'claude', 'Claude', 'Cl', 'https://claude.ai', 'AI 助手', 'Anthropic 旗下 AI 助手与模型入口。', 'tool', 'Anthropic', 5, 968, 'Enterprise', '["ai","assistant"]'::jsonb, '{}'::jsonb, true, 10, 1),
	((SELECT id FROM resource_categories WHERE code = 'dev-tools'), 'vercel', 'Vercel', 'Ve', 'https://vercel.com', '部署平台', '前端应用与边缘函数部署平台。', 'tool', 'Vercel', 0, 1300, 'Enterprise', '["deployment","frontend"]'::jsonb, '{}'::jsonb, true, 20, 1),
	((SELECT id FROM resource_categories WHERE code = 'design'), 'figma', 'Figma', 'Fi', 'https://figma.com', '设计工具', '协作式 UI 设计与原型工具。', 'tool', 'Figma', 0, 2100, 'Team', '["design","prototype"]'::jsonb, '{}'::jsonb, true, 30, 1),
	((SELECT id FROM resource_categories WHERE code = 'dev-tools'), 'github', 'GitHub', 'GH', 'https://github.com', '代码托管平台', '代码托管、协作和自动化工作流平台。', 'tool', 'GitHub', 0, 3200, 'Team', '["code","ci"]'::jsonb, '{}'::jsonb, false, 40, 1)
ON CONFLICT (slug) DO UPDATE SET
	category_id = EXCLUDED.category_id,
	name = EXCLUDED.name,
	icon_text = EXCLUDED.icon_text,
	website_url = EXCLUDED.website_url,
	summary = EXCLUDED.summary,
	description = EXCLUDED.description,
	resource_type = EXCLUDED.resource_type,
	provider = EXCLUDED.provider,
	model_count = EXCLUDED.model_count,
	follower_count = EXCLUDED.follower_count,
	badge = EXCLUDED.badge,
	tags = EXCLUDED.tags,
	metadata = EXCLUDED.metadata,
	is_featured = EXCLUDED.is_featured,
	sort_order = EXCLUDED.sort_order,
	status = EXCLUDED.status,
	updated_at = CURRENT_TIMESTAMP;

INSERT INTO quick_entries (code, title, icon_text, target_url, description, visible_role_codes, sort_order, status)
VALUES
	('github', 'GitHub', 'GH', 'https://github.com', '代码托管平台', '[]'::jsonb, 10, 1),
	('vercel', 'Vercel', 'Ve', 'https://vercel.com', '部署平台', '[]'::jsonb, 20, 1),
	('figma', 'Figma', 'Fi', 'https://figma.com', '设计工具', '[]'::jsonb, 30, 1),
	('claude', 'Claude', 'Cl', 'https://claude.ai', 'AI 助手', '[]'::jsonb, 40, 1)
ON CONFLICT (code) DO UPDATE SET
	title = EXCLUDED.title,
	icon_text = EXCLUDED.icon_text,
	target_url = EXCLUDED.target_url,
	description = EXCLUDED.description,
	visible_role_codes = EXCLUDED.visible_role_codes,
	sort_order = EXCLUDED.sort_order,
	status = EXCLUDED.status,
	updated_at = CURRENT_TIMESTAMP;

INSERT INTO recommendation_items (
	resource_id, title, subtitle, source_name, source_url, icon_text, target_url,
	published_at, visible_role_codes, sort_order, status
)
SELECT
	r.id, seed.title, seed.subtitle, seed.source_name, seed.source_url, seed.icon_text, seed.target_url,
	CURRENT_TIMESTAMP, seed.visible_role_codes::jsonb, seed.sort_order, 1
FROM (
	VALUES
		('vercel', '用 Vercel Edge Config 更快发布', 'Vercel 博客', 'Vercel', 'https://vercel.com/blog', 'Ve', 'https://vercel.com/blog', '[]', 10),
		('claude', 'Claude 工程实践要点', 'Anthropic', 'Anthropic', 'https://www.anthropic.com/news', 'Cl', 'https://www.anthropic.com/news', '[]', 20),
		('figma', '用 Figma Variables 打造可扩展设计系统', 'Figma', 'Figma', 'https://www.figma.com/blog', 'Fi', 'https://www.figma.com/blog', '[]', 30)
) AS seed(slug, title, subtitle, source_name, source_url, icon_text, target_url, visible_role_codes, sort_order)
JOIN resources r ON r.slug = seed.slug
WHERE NOT EXISTS (
	SELECT 1 FROM recommendation_items existing WHERE existing.title = seed.title
);

INSERT INTO carousel_slides (
	code, title, subtitle, description, background, button_text, target_url,
	autoplay_seconds, visible_role_codes, sort_order, status
)
VALUES
	('ai-console', 'AI 助手控制台', '看板', '重塑工具、对比性能、生成报告，一站式为你提供沉浸式生产力体验。', '#6D28D9', '立即查看', 'https://claude.ai', 5, '[]'::jsonb, 10, 1),
	('resource-center', '资源中心精选', '工具库', '快速发现团队常用的开发、设计与 AI 工具。', '#0F766E', '浏览资源', '/resources', 5, '[]'::jsonb, 20, 1)
ON CONFLICT (code) DO UPDATE SET
	title = EXCLUDED.title,
	subtitle = EXCLUDED.subtitle,
	description = EXCLUDED.description,
	background = EXCLUDED.background,
	button_text = EXCLUDED.button_text,
	target_url = EXCLUDED.target_url,
	autoplay_seconds = EXCLUDED.autoplay_seconds,
	visible_role_codes = EXCLUDED.visible_role_codes,
	sort_order = EXCLUDED.sort_order,
	status = EXCLUDED.status,
	updated_at = CURRENT_TIMESTAMP;

INSERT INTO workspace_channels (code, name, description, icon_text, target_url, visible_role_codes, sort_order, status)
VALUES
	('workspace', '工作台', '普通工作台入口', 'W', '/workspace', '[]'::jsonb, 10, 1),
	('admin-console', '管理控制台', '管理员配置入口', 'A', '/admin', '["admin"]'::jsonb, 20, 1)
ON CONFLICT (code) DO UPDATE SET
	name = EXCLUDED.name,
	description = EXCLUDED.description,
	icon_text = EXCLUDED.icon_text,
	target_url = EXCLUDED.target_url,
	visible_role_codes = EXCLUDED.visible_role_codes,
	sort_order = EXCLUDED.sort_order,
	status = EXCLUDED.status,
	updated_at = CURRENT_TIMESTAMP;`
	if err := db.Exec(seedSQL).Error; err != nil {
		log.Fatalf("初始化种子数据失败: %v", err)
	}
}
