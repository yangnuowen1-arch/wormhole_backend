-- 0003_seed_workspace_home.sql
-- 首页基础种子数据。全部语句幂等，可重复执行。

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
    updated_at = CURRENT_TIMESTAMP;
