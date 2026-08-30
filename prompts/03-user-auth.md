# Phase 3 — User + Auth + RBAC + Rate Limit + OpenAPI

先读取 `STATE.md` 和 `docs/adr/`。必须保持已锁定架构。

目标：通过 User 模块验证整个架构可以支撑真实业务。

## 3.1 User Domain

`ID / Email / PasswordHash / Name / Status(active|disabled) / CreatedAt / UpdatedAt`

保持简单。**不要构建复杂 Aggregate Root。**

## 3.2 ID

按 ADR-0006 实现。ID 生成逻辑必须集中，**不得散落在 Handler**。

## 3.3 Migration

创建 users 表：`primary key / email unique index / status / timestamps`。

**email 必须在数据库层 unique，不能只在 Service 检查。**

## 3.4 sqlc Queries

`queries/` 下实现：
`CreateUser / GetUserByID / GetUserByEmail / ListUsers / UpdateUser / DeleteUser`

必须明确列名，**禁止 `SELECT *`**。

## 3.5 Repository

Application 层定义自己真正需要的**小接口**：

```go
type UserRepository interface {
    Create(ctx context.Context, user User) error
    GetByID(ctx context.Context, id ID) (User, error)
    GetByEmail(ctx context.Context, email string) (User, error)
}
```

**禁止 `GenericRepository[T]`。**

## 3.6 Error Translation

Repository 负责把 `pgx.ErrNoRows` / unique violation / 其他 DB error
翻译成 `ErrNotFound` / `ErrConflict` / wrapped internal error。

**Service 不得判断 PostgreSQL error code。**

## 3.7 Create User

```text
Validate Input → Normalize Email → DB Unique Constraint
→ Hash Password → Create User → Return User
```

密码及其 hash **绝不返回**。

## 3.8 Password Hashing

在 Argon2id / bcrypt 中选择，说明理由。必须有 salt 和安全参数。

**禁止自行设计加密算法。**

## 3.9 DTO

`CreateUserRequest / UpdateUserRequest / UserResponse` 与 Domain Model 分离。

**API 输出中禁止出现 `PasswordHash`。**

## 3.10 User API

```text
POST   /api/v1/users
GET    /api/v1/users
GET    /api/v1/users/{id}
PUT|PATCH /api/v1/users/{id}
DELETE /api/v1/users/{id}
```

根据 REST 语义在 PUT / PATCH 中选一种并全项目保持一致。

## 3.11 Pagination

按 ADR-0004 实现。

```text
default page_size = 20
max page_size = 100
```

**禁止无限 page_size。**

## 3.12 Login

```text
POST /api/v1/auth/login
输入: email, password
输出: access_token, refresh_token, expires_in, token_type
```

错误信息统一为 `invalid credentials`，
**不得区分"邮箱不存在"和"密码错误"**（防用户枚举）。

## 3.13 Access Token

JWT claims 至少：`sub / role / iat / exp / iss`（`jti` 可选）。

签发与验证独立封装，**业务 Service 不得直接依赖具体 JWT library**。

## 3.14 Refresh Token

按 ADR-0007 实现，优先：opaque token + hashed storage + rotation + revocation。

```text
POST /api/v1/auth/refresh
```

必须支持 rotation。若第一版为简化使用 JWT refresh token，必须明确写出安全权衡。

## 3.15 Logout

```text
POST /api/v1/auth/logout
```

撤销 refresh token。Access Token 保持短生命周期。

## 3.16 Authentication Middleware

读取 `Authorization: Bearer <token>`，验证后将 `UserID` 与 `Role` 写入 request context。

**不要把完整 User Object 放进 Context。**

## 3.17 Authorization（RBAC）

实现最小 RBAC：

```text
admin: create user / delete user / list all users
user:  get own profile / update own profile
```

必须有**集中式权限定义**，
**禁止 `if role == "admin"` 散落在所有 Handler。**

## 3.18 Rate Limiting（本阶段必须落地）

按 ADR-0008 实现：

```text
中间件形式挂载，位置在 Auth 之前
支持按 IP 与按 UserID 两种维度
登录接口必须有独立、更严格的限流策略（防爆破）
Redis 存储 + 明确的 fail-open / fail-closed 策略
响应 429 + Retry-After + X-RateLimit-Limit/Remaining/Reset
限流配置可通过 Config 调整，可整体关闭
```

测试必须覆盖：正常放行 / 触发限流 / 窗口重置 / Redis 故障时的降级行为。

## 3.19 Validation

实现 `email / password policy / name length / status / pagination` 校验。

明确区分 **syntax validation** 与 **business validation**。

## 3.20 OpenAPI（本阶段必须同步产出）

**每新增一个 API，必须同步更新 `api/openapi.yaml`。**

覆盖：`auth` / `users` / error schemas / pagination schema / 认证方式定义。

在 CI 中加入 spec 校验（例如 `redocly lint` 或等价工具）。
若工具不可用，至少提供一个测试断言：所有已注册路由都存在于 spec 中。

**目标：从第一天起就不存在实现与文档的 drift。**

## 3.21 Transactions

选择一个**真实**业务流程展示事务，例如 `Create User + Create Refresh Session`。

**如果没有真实需求，不要为了展示事务而强行使用。**

## 3.22 Repository Integration Tests

使用 `testcontainers-go` + PostgreSQL，测试：

```text
Create / GetByID / GetByEmail / Unique Email / Update / Delete / Pagination
```

**禁止 Mock PostgreSQL 来测试 SQL。**

若 Docker 不可用：按 §0.5 标记 SKIPPED，不得伪造结果。

## 3.23 Service Tests

使用 Fake Repository，测试：
`duplicate email / not found / invalid password / disabled account / login success / authorization`。

## 3.24 Handler Tests

使用 `httptest`，测试：
`request validation / status code / error response / auth middleware / RBAC / rate limit`。

## 3.25 Phase 3 输出

执行 §0.4 全部门禁。输出：

```text
Implemented APIs
Database Changes
Auth Model
Rate Limiting Design
OpenAPI Coverage
Security Decisions
Tests Added
Commands Run
Test Results
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。
