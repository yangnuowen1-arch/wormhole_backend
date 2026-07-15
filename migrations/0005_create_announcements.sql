-- 0005_create_announcements.sql
-- 系统公告：管理员维护，所有已登录用户读取已发布且处于有效期内的公告。

CREATE TABLE IF NOT EXISTS announcements (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title        VARCHAR(128) NOT NULL,
    content      TEXT         NOT NULL,
    is_pinned    BOOLEAN      NOT NULL DEFAULT FALSE,
    status       SMALLINT     NOT NULL DEFAULT 1, -- 1=Published, 0=Draft/Hidden
    published_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   TIMESTAMP,
    created_by   BIGINT,
    updated_by   BIGINT,
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_announcements_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_announcements_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_announcements_visibility
    ON announcements (status, is_pinned DESC, published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_announcements_expires_at ON announcements (expires_at);
