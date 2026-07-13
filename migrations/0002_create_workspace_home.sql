-- 0002_create_workspace_home.sql
-- 智能工作台首页业务表：资源中心、常用工具、搜索历史、快捷入口、推荐、轮播与角色化专栏。

CREATE TABLE IF NOT EXISTS resource_categories (
    id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(64)  NOT NULL,
    name        VARCHAR(64)  NOT NULL,
    description VARCHAR(255),
    sort_order  INTEGER      NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL DEFAULT 1, -- 1=Enabled, 0=Disabled
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
    status         SMALLINT     NOT NULL DEFAULT 1, -- 1=Published, 0=Hidden
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
    user_id    BIGINT    NOT NULL,
    resource_id BIGINT   NOT NULL,
    sort_order INTEGER   NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, resource_id),
    CONSTRAINT fk_user_common_tools_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_common_tools_resource FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_common_tools_user_sort ON user_common_tools (user_id, sort_order, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_common_tools_resource ON user_common_tools (resource_id);

CREATE TABLE IF NOT EXISTS search_history (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id          BIGINT       NOT NULL,
    query            VARCHAR(128) NOT NULL,
    search_count     INTEGER      NOT NULL DEFAULT 1,
    last_result_count INTEGER     NOT NULL DEFAULT 0,
    last_searched_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
    status             SMALLINT     NOT NULL DEFAULT 1, -- 1=Enabled, 0=Disabled
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
    status             SMALLINT     NOT NULL DEFAULT 1, -- 1=Published, 0=Hidden
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
    status             SMALLINT     NOT NULL DEFAULT 1, -- 1=Published, 0=Hidden
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
    status             SMALLINT     NOT NULL DEFAULT 1, -- 1=Enabled, 0=Disabled
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
