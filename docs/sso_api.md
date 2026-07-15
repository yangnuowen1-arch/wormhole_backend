# SSO 与前端鉴权接口文档

本文档说明前端接入 Keycloak SSO 后需要使用的后端接口。

## 基础地址

当前联调环境：

```text
Backend API: http://192.168.31.28:8081/api/v1
Frontend:    http://192.168.31.28:5173
```

Swagger UI：

```text
http://192.168.31.28:8081/swagger/index.html
```

## 登录态说明

SSO 登录成功后，后端会写入 HttpOnly Cookie：

```text
wormhole_session
```

前端 JavaScript 不能也不应该读取这个 Cookie。浏览器会在跨域请求时自动携带它，但前端必须显式配置：

```ts
fetch(url, { credentials: "include" })
```

或 axios：

```ts
axios.create({ withCredentials: true })
```

前端判断是否已登录时，以 `GET /users/me` 的响应为准：

```text
200 = 已登录
401 = 未登录或会话过期
```

---

## 1. 发起 SSO 登录

```http
GET /auth/sso/login?return_to=/navigation
```

完整地址示例：

```text
http://192.168.31.28:8081/api/v1/auth/sso/login?return_to=/navigation
```

### 用途

前端 SSO 登录按钮跳转到该接口。后端会：

1. 生成 OIDC `state`、`nonce`、PKCE `verifier`。
2. 写入临时 HttpOnly Cookie。
3. 302 重定向到 Keycloak 登录页。

### 前端调用方式

不要用 `fetch`，直接改浏览器地址：

```ts
window.location.href =
  "http://192.168.31.28:8081/api/v1/auth/sso/login?return_to=/navigation";
```

### Query 参数

| 参数 | 必填 | 说明 |
|---|---:|---|
| `return_to` | 否 | 登录成功后跳回的前端路径，例如 `/navigation`、`/users`。建议传相对路径。 |

### 成功响应

```http
302 Found
Location: http://192.168.31.200:8090/realms/test/protocol/openid-connect/auth?...
```

### 失败响应示例

```json
{
  "code": 50201,
  "message": "获取 Keycloak 授权地址失败",
  "data": null,
  "error": "...",
  "requestId": "5547e998-1127-4c9d-ae7e-f3508c42b96c",
  "timestamp": "2026-07-10T16:30:01+08:00"
}
```

---

## 2. SSO 回调

```http
GET /auth/sso/callback?code=xxx&state=xxx
```

完整地址：

```text
http://192.168.31.28:8081/api/v1/auth/sso/callback
```

### 用途

该接口由 Keycloak 登录成功后重定向调用，前端不需要主动请求。

后端会：

1. 校验临时 Cookie 和 query 中的 `state`。
2. 用 `code` 向 Keycloak token endpoint 换取 token。
3. 验证 ID Token 签名、issuer、audience、nonce 和过期时间。
4. 按 Keycloak `sub` 创建或更新本地用户。
5. 写入 `wormhole_session` HttpOnly Cookie。
6. 302 跳回前端 `return_to` 页面。

### 成功响应

```http
302 Found
Set-Cookie: wormhole_session=...; HttpOnly; Path=/; SameSite=Lax
Location: http://192.168.31.28:5173/navigation
```

### 常见失败

| HTTP | code | message | 常见原因 |
|---:|---:|---|---|
| 400 | 40002 | SSO 状态已过期，请重新登录 | 登录流程耗时太久、临时 Cookie 丢失 |
| 400 | 40002 | SSO state 校验失败 | 重复刷新 callback、state 不匹配 |
| 502 | 50202 | Keycloak token 校验失败 | Keycloak token 过期、机器时间不一致、issuer/audience 不匹配 |
| 500 | 50002 | 建立本地会话失败 | 数据库写入用户失败 |

---

## 3. 获取当前登录用户

```http
GET /users/me
```

完整地址：

```text
http://192.168.31.28:8081/api/v1/users/me
```

### 用途

前端路由权限判断接口。

### 前端调用

```ts
const res = await fetch("http://192.168.31.28:8081/api/v1/users/me", {
  credentials: "include",
});

if (res.status === 401) {
  // 未登录，跳转登录页
  window.location.href = "/login";
}

const body = await res.json();
const user = body.data;
```

axios：

```ts
const api = axios.create({
  baseURL: "http://192.168.31.28:8081/api/v1",
  withCredentials: true,
});

const res = await api.get("/users/me");
```

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "nickname": "Alice"
  },
  "error": null,
  "requestId": "5547e998-1127-4c9d-ae7e-f3508c42b96c",
  "timestamp": "2026-07-10T16:30:01+08:00"
}
```

### 未登录响应

```http
401 Unauthorized
```

```json
{
  "code": 40101,
  "message": "未登录，请先登录",
  "data": null,
  "error": null,
  "requestId": "5547e998-1127-4c9d-ae7e-f3508c42b96c",
  "timestamp": "2026-07-10T16:30:01+08:00"
}
```

---

## 4. 退出登录

```http
POST /auth/logout
```

完整地址：

```text
http://192.168.31.28:8081/api/v1/auth/logout
```

### 用途

清除本应用的 `wormhole_session` Cookie。

当前接口只退出 Wormhole 后端应用会话，不保证退出 Keycloak 全局会话。如果后续需要完整单点登出，可以再接 Keycloak `end_session_endpoint`。

### 前端调用

```ts
await fetch("http://192.168.31.28:8081/api/v1/auth/logout", {
  method: "POST",
  credentials: "include",
});

window.location.href = "/login";
```

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "logged_out": true
  },
  "error": null,
  "requestId": "5547e998-1127-4c9d-ae7e-f3508c42b96c",
  "timestamp": "2026-07-10T16:30:01+08:00"
}
```

---

## 5. 管理员用户管理

以下接口都要求当前登录用户拥有 `admin` 角色；请求同样要携带 `credentials: "include"`（或 axios 的 `withCredentials: true`）。

| 方法 | 路径 | 用途 |
|---|---|---|
| `GET` | `/admin/users` | 获取全部本地用户，包含已停用用户和角色 |
| `GET` | `/admin/users/{id}` | 获取单个用户详情 |
| `POST` | `/admin/users` | 创建本地用户并分配角色 |
| `PATCH` | `/admin/users/{id}` | 更新用户名、邮箱、昵称、头像或状态 |
| `DELETE` | `/admin/users/{id}` | 逻辑删除（停用）用户 |
| `PUT` | `/admin/users/{id}/roles` | 完整替换用户角色 |

用户列表的 `data` 是数组，每一项包含 `id`、`keycloakId`、`username`、`email`、`nickname`、`avatar`、`status`（`1` 为启用，`0` 为停用）和 `roles`。

新增用户示例：

```http
POST /api/v1/admin/users
Content-Type: application/json

{
  "keycloakId": "65b336b8-b4c5-4bce-bb36-7831fc22b641",
  "username": "alice",
  "email": "alice@example.com",
  "nickname": "Alice",
  "roleCodes": ["user"]
}
```

在 Keycloak SSO 模式下，`keycloakId` 必须是该用户在 Keycloak 中已有账号的 `sub`；本服务只管理本地业务资料和角色，不会代为创建或删除 Keycloak 账号。未传 `roleCodes` 时自动授予 `user` 角色。

更新示例（未传字段保持不变；邮箱、昵称、头像传空字符串可清空）：

```http
PATCH /api/v1/admin/users/42
Content-Type: application/json

{
  "nickname": "Alice Chen",
  "status": 1
}
```

删除调用会将本地账号停用，而不是抹除其 Keycloak subject；已签发的应用会话会被后端拒绝，也不会在该用户下次 SSO 登录时重新生成一个本地账号。管理员不能停用、删除自己，也不能通过角色接口移除自己的 `admin` 角色。

---

## 6. 管理员分配用户角色

```http
PUT /admin/users/{id}/roles
```

例如把 ID 为 `42` 的用户设为管理员和普通用户：

```http
PUT /api/v1/admin/users/42/roles
Content-Type: application/json

{
  "roleCodes": ["admin", "user"]
}
```

该接口要求当前登录用户已有 `admin` 角色。`roleCodes` 会完整替换目标用户原有的角色，必须至少传入一个已存在的角色编码；内置角色为 `admin`、`user`。

前端调用示例：

```ts
await api.put("/admin/users/42/roles", {
  roleCodes: ["admin", "user"],
});
```

成功时返回更新后的用户资料和角色列表；目标用户不存在返回 `40401`，角色不存在返回 `40402`，非管理员返回 `40301`。

---

## 7. 系统公告（通知）

所有已登录用户都可以读取公告：

```http
GET /announcements
```

接口只返回已发布、已到发布时间且尚未到期的公告；置顶公告排在前面。用于顶部通知铃铛时可直接读取返回列表。

管理员管理接口：

| 方法 | 路径 | 用途 |
|---|---|---|
| `GET` | `/admin/announcements?status=0\|1` | 查看全部公告，可筛选状态 |
| `POST` | `/admin/announcements` | 新增公告 |
| `PATCH` | `/admin/announcements/{id}` | 编辑内容、置顶、发布时间、到期时间或状态 |
| `PATCH` | `/admin/announcements/{id}/status` | 快速发布或下架 |

新增示例：

```http
POST /api/v1/admin/announcements
Content-Type: application/json

{
  "title": "平台维护通知",
  "content": "本周六 02:00-03:00 进行例行维护。",
  "isPinned": true,
  "status": 1,
  "publishedAt": "2026-07-13T10:00:00Z",
  "expiresAt": "2026-07-20T10:00:00Z"
}
```

`status` 中 `1` 为发布，`0` 为草稿/下架。`publishedAt` 为空时立即生效，`expiresAt` 为空时永不过期；到期时间必须晚于发布时间。所有 `/admin/announcements` 接口都要求当前用户拥有 `admin` 角色。

---

## 前端路由权限判断建议

1. 应用启动时调用 `GET /users/me`。
2. `loading=true` 时显示加载页，不要马上跳转。
3. `/users/me` 返回 200：保存用户信息，允许进入受保护页面。
4. `/users/me` 返回 401：清空用户信息，跳 `/login`。
5. 已登录用户访问 `/login`：跳 `/navigation`。

示例：

```tsx
function ProtectedRoute() {
  const { user, loading } = useAuth();

  if (loading) return <div>Loading...</div>;
  if (!user) return <Navigate to="/login" replace />;

  return <Outlet />;
}
```

## 注意事项

- 前端不要保存 Keycloak `client_secret`。
- 前端不要把 Keycloak token 存进 localStorage。
- 前端不要自己判断 Cookie 是否存在，HttpOnly Cookie 读不到。
- 所有需要登录态的请求都要带 `credentials: "include"` 或 `withCredentials: true`。
- 前端访问地址和 CORS 配置必须一致，例如都使用 `http://192.168.31.28:5173`，不要混用 `localhost`。
