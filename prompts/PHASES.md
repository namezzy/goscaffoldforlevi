# Enterprise Go Scaffold — 分阶段开发 Prompt (v2)

> 使用方式：**每个 Phase 开一个独立会话**。
> 会话开始时，先粘贴 `Phase 0 全局执行契约`，再粘贴当前 Phase 的正文。

---

# Phase 0 — 全局执行契约

以下规则在**所有 Phase 中始终生效**，优先级高于任何单个 Phase 的局部描述。
如果某个 Phase 的要求与本契约冲突，以本契约为准，并在输出中指出冲突。

## 0.1 目标

构建一套真正可用于企业项目的：

**Production-ready Enterprise Go Scaffold**

技术基线：

```text
Go 1.27+
net/http + chi
PostgreSQL / pgx / sqlc
Redis / go-redis
Message Queue (Kafka or NATS JetStream，Phase 1 决策)
log/slog
OpenTelemetry
Prometheus
JWT
OpenAPI 3
golang-migrate
Docker
GitHub Actions
Kubernetes
```

## 0.2 项目原则

```text
Simple > Clever
Explicit > Magic
Composition > Inheritance
Small Interfaces
Dependency Inversion
Fail Fast
Context Everywhere
Observability by Default
Security by Default
Testability by Design
```

不要把 Go 项目写成 Java Spring Boot。

禁止：

```text
controller/service/dao/entity/vo/dto/common/utils 式机械分层
BaseController / BaseService / BaseRepository
GenericCRUD / GenericRepository[T]
AbstractXXX
巨大 interface
全局变量（global db / logger / config）
init() 隐式初始化
Service Locator
复杂反射
过度代码生成
```

## 0.3 状态持久化（强制）

因为每个 Phase 在独立会话中执行，**上下文不会自动继承**。因此：

1. **Phase 1 必须把所有架构决策写入磁盘**，格式为 ADR：

   ```text
   docs/adr/0001-tech-stack.md
   docs/adr/0002-project-layout.md
   docs/adr/0003-error-model.md
   ...
   ```

   每个 ADR 包含：`Status / Context / Decision / Consequences / Alternatives Considered`。

2. **每个 Phase 结束时必须更新 `STATE.md`**（仓库根目录），内容固定为：

   ```markdown
   # Project State

   ## Current Phase
   Phase N completed at <date>

   ## Locked Decisions
   （指向 docs/adr/ 的清单，一行一条，含结论摘要）

   ## Implemented
   （已完成能力清单）

   ## Not Implemented Yet
   （明确未做的事，防止下一阶段误判）

   ## Known Limitations
   （已知妥协及原因）

   ## Next Phase Entry Conditions
   （下一阶段开始前必须成立的前提）
   ```

3. **每个 Phase 开始时**：先读取 `STATE.md` 和 `docs/adr/`，再读代码库。
   **不得重新设计已锁定的决策。**

4. 如需推翻已有决策：新建一个 ADR，`Status: Accepted`，并在文中写明
   `Supersedes: docs/adr/000X-*.md`，同时说明推翻理由。禁止静默修改旧 ADR。

## 0.4 质量门禁

**Phase 2 起**，每个 Phase 完成前必须执行：

```bash
go fmt ./...
go vet ./...
go build ./...
go test ./...
go test -race ./...
golangci-lint run
govulncheck ./...
```

Phase 1 是纯设计阶段，**不执行**上述命令（没有代码）。

发现错误直接修复。**禁止把无法编译或测试不通过的代码交付给我。**

## 0.5 环境兜底（强制诚实）

Docker / Testcontainers / kubectl / 外部网络在某些环境中不可用。

规则：

```text
1. 命令实际执行过 → 贴出真实输出摘要
2. 命令因环境不可用而无法执行 → 明确写 "SKIPPED: <原因>"，并列入 Known Limitations
3. 严禁编造、推测或复述未真实执行的命令结果
```

如果你没有运行某条命令，就不能说它通过了。

## 0.6 范围纪律

```text
不得输出超出当前 Phase 范围的代码
不得提前实现后续 Phase 的功能
不得创建无用空目录
不得为了"演示某个模式"而引入没有真实需求的复杂度
```

每个 Phase 结束时，在输出中回答一句：
**"本阶段有没有做超范围的事？"**

## 0.7 Git 约定

每个 Phase 结束时提交一次，commit message 格式：

```text
phase(N): <一句话总结>
```

`.gitignore` 必须从 Phase 2 起就正确排除：
`.env`、二进制产物、coverage 文件、IDE 缓存、本地数据卷。

---

# Phase 1 — Architecture Design（纯设计，不写实现代码）

你现在是一名资深 Go 架构师。

**本阶段只产出决策文档，不产出实现代码。**
允许出现示意性代码片段（用于说明接口形态），但不创建 Go 工程。

**本阶段唯一的交付物是 `docs/adr/*.md` + `docs/architecture.md` + `STATE.md`。**

## 1.1 项目目标

面向：

```text
REST API / 微服务 / SaaS Backend / 内部管理系统 / B2B Backend
```

要求：

```text
长期维护 / 多人协作 / 可测试 / 可扩展 / 可观测 / 安全 / 易部署 / 开发体验良好
```

同时必须保持 Go 的简单性。

## 1.2 技术选型

逐项分析下列每一项：

```text
为什么选择 / 解决什么问题 / 是否第一版必须 / 替代方案 / 什么时候应该换
```

重点辩论：

```text
chi vs Gin vs Echo
pgx + sqlc vs GORM
slog vs zap
JWT vs Session
UUIDv7 vs ULID vs UUIDv4 vs BIGSERIAL
Redis 是否应该默认存在
OpenTelemetry 是否应该默认开启
Kafka vs NATS JetStream vs Redis Streams vs 暂不引入
```

不要因为我给了默认技术栈就机械接受。
**如果你认为某项属于过度设计，明确指出并给出删除建议。**

→ 输出到 `docs/adr/0001-tech-stack.md`

## 1.3 目录结构

不要机械照搬 `domain/application/repository/transport/infrastructure`。

结合 Go Idiomatic Design 给出最终目录，并对每个目录说明：

```text
负责什么 / 不能放什么 / 谁可以依赖谁
```

明确判断是否真的需要：`domain / service / repository / platform / bootstrap / infrastructure / application`。

避免目录过深。

**验收标准：新人进入项目后 10 分钟内可以理解代码结构。**

→ 输出到 `docs/adr/0002-project-layout.md`

## 1.4 Dependency Rule

业务核心不得依赖：HTTP framework / PostgreSQL / Redis / 消息队列 / 云厂商 SDK。

给出 ASCII 依赖图。如果你认为可以进一步简化分层，给出你的方案并说明理由。

重点说明 **interface 应该定义在哪一侧**，遵循：

> Interface belongs to the consumer.

禁止出现 `repository/interface.go` 里堆几十个全局 Repository Interface。

→ 输出到 `docs/adr/0002-project-layout.md`

## 1.5 User 示例模块边界

模型：`ID / Email / PasswordHash / Name / Status / CreatedAt / UpdatedAt`

设计以下结构之间的边界，明确**哪些可以复用、哪些必须分离**：

```text
Domain Entity / Business Rules / Repository Contract
Application Service / HTTP Handler / DTO / Database Model
```

## 1.6 Error Model

设计错误体系：

```text
ErrNotFound / ErrInvalidArgument / ErrUnauthorized
ErrForbidden / ErrConflict / ErrRateLimited / ErrInternal
```

要求业务层完全不知道 HTTP 状态码的存在。

设计链路：

```text
domain/application error → transport mapping → HTTP status → API error response
```

必须考虑 `errors.Is` / `errors.As` / `%w`。

→ 输出到 `docs/adr/0003-error-model.md`

## 1.7 API Response

分析成功响应是否需要 envelope（`{code, message, data}`）。

给出结论并说明理由。设计以下四者如何配合：

```text
business error code / HTTP status / request_id / error details
```

**禁止：所有 API 都返回 HTTP 200。**

→ 输出到 `docs/adr/0004-api-conventions.md`

## 1.8 Database

设计：`connection pool / transaction / migration / repository / query / schema`。

分析 Transaction Manager 是否必要。如果必要，API 必须简单：

```go
txManager.WithTransaction(ctx, func(ctx context.Context) error {
    return nil
})
```

避免在 Repository / Service 之间到处传递 `pgx.Tx`。

→ 输出到 `docs/adr/0005-data-access.md`

## 1.9 Pagination

分析 offset vs cursor，第一版选一个。考虑：数据规模 / 稳定排序 / 实现复杂度 / 前端体验。

→ 写入 `docs/adr/0004-api-conventions.md`

## 1.10 ID Strategy

分析 `BIGSERIAL / UUIDv4 / UUIDv7 / ULID / Snowflake`，结合：
PostgreSQL index / 分布式 / API 暴露 / 排序 / 运维简单度。

**选定一个默认 ID 类型。**

→ 输出到 `docs/adr/0006-id-strategy.md`

## 1.11 Configuration

设计 `Config / AppConfig / HTTPConfig / DatabaseConfig / RedisConfig / MessagingConfig / AuthConfig / OTelConfig`。

支持：环境变量 / `.env` / 默认值 /（YAML 可选）。

明确：生产优先环境变量；敏感配置禁止进 Git；启动阶段必须 `Load → Validate → Fail Fast`。

## 1.12 Authentication & Authorization

设计 JWT Access Token + Refresh Token + RBAC（至少 `admin` / `user`）。

分析 Refresh Token 应该是 JWT 还是 opaque token + 存储，考虑：
`revocation / rotation / expiry / logout / 安全性`。

同时设计 RBAC 的**集中式**表达方式，避免 `if role == "admin"` 散落各处。

→ 输出到 `docs/adr/0007-auth.md`

## 1.13 Rate Limiting

**这是必须实现的能力，不是可选项。**

设计：

```text
限流维度：IP / user / API key / 全局
算法：token bucket vs sliding window
存储：内存 vs Redis（多实例一致性）
失败策略：Redis 不可用时 fail-open 还是 fail-closed
响应：429 + Retry-After + X-RateLimit-* headers
```

给出第一版选择。

→ 输出到 `docs/adr/0008-rate-limiting.md`

## 1.14 Cache

Redis 使用 Cache-Aside。原则：**Cache Failure != Business Failure**。

分析第一版是否真的有必要给 User 模块加缓存。
**避免为了演示缓存而增加复杂度。**

## 1.15 Messaging / 异步

设计异步消息能力：

```text
选型：Kafka vs NATS JetStream vs Redis Streams
Producer 接口形态（业务层不得依赖具体 SDK）
Consumer 生命周期与优雅退出
消息契约与 schema 演进
Outbox Pattern 是否第一版必须
幂等消费 / 去重
重试策略与退避
Dead Letter Queue
顺序性保证需求
```

**必须回答一个问题：这个脚手架的第一版是否真的需要消息队列？**
如果你认为不需要，请说明，并给出"何时应该引入"的触发条件。
但无论结论如何，都要给出接口抽象设计，使后续接入不需要改动业务层。

→ 输出到 `docs/adr/0009-messaging.md`

## 1.16 Observability

设计 `Structured Logging / OTel Trace / Metrics / Request ID / Trace ID`。

要求第一天就可接入，但**业务代码不得依赖 OpenTelemetry SDK**。

## 1.17 Middleware 顺序

给出顺序并逐条解释原因，例如：

```text
Recovery → Request ID → Tracing → Logging → Metrics
→ CORS → Timeout → Rate Limit → Authentication → Authorization → Handler
```

判断上述顺序是否正确，若有问题请纠正。

## 1.18 Testing Strategy

设计 `Unit / Integration / Repository / HTTP / End-to-End` 分层。

优先 `testing` / `httptest` / `testcontainers-go`。

明确：**哪些值得 Mock，哪些绝对不应该 Mock。**

## 1.19 Bootstrap & Graceful Shutdown

启动顺序：

```text
main → config → logger → observability → database → redis → messaging
→ repositories → services → handlers → router → http server
```

优先 Manual Dependency Injection。

关闭顺序（SIGINT / SIGTERM）：

```text
readiness=false → http server drain → consumers → messaging
→ redis → database → telemetry
```

说明为什么是这个顺序。

## 1.20 文档归属规则

上文标注了 `→ 输出到 docs/adr/000N-*.md` 的小节，必须各自成为独立 ADR。

未标注 ADR 归属的小节（1.5 / 1.11 / 1.14 / 1.16 / 1.17 / 1.18 / 1.19）
统一写入 `docs/architecture.md` 的对应章节。

ADR 使用 `prompts/templates/adr.md` 的格式，必须包含
`Status / Date / Phase / Supersedes / Context / Decision / Consequences（含负面代价）/
Alternatives Considered / When to Revisit`。

**`When to Revisit` 不得写"视情况而定"，必须是可观测的触发条件**，
例如"当写 QPS 超过 5000"或"当需要跨服务共享该逻辑时"。

## 1.21 Phase 1 输出

按顺序输出：

```text
A. Architecture Decision Summary
B. Final Tech Stack
C. Project Structure（完整目录树）
D. Dependency Diagram（ASCII）
E. Request Lifecycle
F. Error Flow
G. Configuration Flow
H. Authentication Flow
I. Messaging Flow
J. Test Strategy
K. Overengineering Review
   —— 明确告诉我：哪些要求你删除了、哪些保留了、为什么
L. ADR 文件清单（实际写入磁盘的路径）
M. Phase 2 Implementation Plan
```

**写入 `STATE.md`。不要进入 Phase 2 编码。**

---

# Phase 2 — Foundation

先读取 `STATE.md` 和 `docs/adr/`，再读代码库。

**不得重新设计架构。** 如发现 Phase 1 存在严重问题，新建 ADR 说明并 supersede。

目标：建立一个**可启动 / 可配置 / 可连接依赖 / 有日志 / 有 Router / 有 Middleware / 有错误处理 / 有健康检查 / 可优雅退出**的最小生产级基础工程。

## 2.1 工程初始化

```text
module: github.com/namezzy/goscaffoldforlevi
```

按 ADR-0002 建立目录。禁止创建无用空目录。

## 2.2 Config

类型安全 Config，支持 `.env` / 环境变量 / 默认值，环境变量优先级最高。

必须 Validate：`port / database URL / JWT key / timeout / environment`。
关键配置缺失 → **启动失败**。

## 2.3 Logger

使用 `log/slog`。

```text
development → 可读文本
production  → JSON
```

基础字段：`service / environment / version`。

**禁止业务代码自行创建 logger。**

## 2.4 Database

`pgxpool`，实现 `NewPool / Ping / Close`。

配置并给出合理默认值：
`MaxConns / MinConns / MaxConnLifetime / MaxConnIdleTime / HealthCheckPeriod`。

所有初始化必须接受 `context.Context` 并设置连接超时。

## 2.5 Migration 工具链（前移自原 Phase 5）

**本阶段必须建立完整 migration 能力，因为 Phase 3 会立刻用到。**

```bash
make migrate-up
make migrate-down
make migrate-create NAME=create_users
```

使用 golang-migrate。目录 `migrations/`。

同时在 `docs/` 中说明：
**为什么生产环境不能让每个 API Pod 启动时自动执行 migration**，
以及推荐的执行方式（CI/CD Job / Kubernetes Job / 手动受控步骤）。

## 2.6 sqlc 工具链（前移自原 Phase 5）

建立 `sqlc.yaml`，配置 `queries/` → 生成代码目录。

```bash
make sqlc
make generate
```

本阶段不需要写业务查询，但工具链必须可运行（可用一个最小 query 验证）。

## 2.7 Redis

go-redis，实现 `NewClient / Ping / Close`。

Redis 不可用时的策略由配置决定：`required` / `optional`。项目必须有明确默认值。

## 2.8 HTTP Server

`net/http` + chi。正确配置：

```text
ReadHeaderTimeout / ReadTimeout / WriteTimeout / IdleTimeout / MaxHeaderBytes
```

**禁止 `http.ListenAndServe(...)` 裸启动。**

## 2.9 Router 与健康检查

```text
/api/v1
GET /health
GET /live    → 只判断进程存活
GET /ready   → 检查必要依赖（PostgreSQL）
```

Redis 若为 optional，其临时故障不得导致 readiness failure。

## 2.10 Error Handling

统一 App Error：
`InvalidArgument / NotFound / Unauthorized / Forbidden / Conflict / RateLimited / Internal`

HTTP 映射示例：

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "invalid request",
  "request_id": "..."
}
```

Internal Error **不得**向客户端暴露：database error / stack / secret / SQL / internal path。

## 2.11 Middleware

本阶段实现：`Recovery / Request ID / Logging / Timeout / CORS`。

Tracing / Metrics / Rate Limit 预留挂载点，但不实现。

Request ID：客户端传入合法 ID 可复用，否则生成新的；
响应头返回 `X-Request-ID`。

## 2.12 Recovery

捕获 panic，记录 `panic / stack trace / request_id`，返回 500。
单个请求 panic 不得导致进程退出。

## 2.13 Request Logging

记录：`method / path / status / duration / request_id`（`remote_ip` / `user_agent` 可选）。

**禁止记录**：`Authorization / Cookie / Password / Token / Request Body`。

## 2.14 Graceful Shutdown

监听 SIGINT / SIGTERM，调用 `http.Server.Shutdown()`，配置 shutdown timeout，
然后按 ADR 顺序关闭 database / redis / telemetry。

**shutdown 流程必须可测试。**

## 2.15 Bootstrap

手动 DI。禁止 global database / logger / config。`main.go` 保持简单。

## 2.16 Environment Files

提供 `.env.example`（不含真实 secret）+ development 默认值。

## 2.17 Makefile

```bash
make run build test test-race fmt vet lint vuln
make generate sqlc
make migrate-up migrate-down migrate-create
```

工具未安装时，README 说明安装方式。

## 2.18 Quality Tools

接入 `golangci-lint` + `govulncheck`。不要使用已废弃的 linter。

## 2.19 最低测试要求

```text
config validation
health / live / ready endpoints
error mapping
request ID middleware
recovery middleware
graceful shutdown
```

## 2.20 Phase 2 输出

执行 §0.4 全部门禁命令，失败立即修复。输出：

```text
Files Created
Architecture Notes
Commands Run（含真实输出摘要 / SKIPPED 说明）
Test Results
本阶段有没有做超范围的事？
Known Limitations
```

**更新 `STATE.md` 并提交。不要进入 User 业务模块。**

---

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

---

# Phase 4 — Messaging / 异步处理

先读取 `STATE.md` 和 `docs/adr/0009-messaging.md`。

**前置判断**：如果 Phase 1 的 ADR-0009 结论是"第一版不引入消息队列"，
则本阶段只实现**接口抽象 + 内存实现 + 测试**，并跳过 4.3 之后的 broker 相关内容，
在 `STATE.md` 中记录跳过理由。否则完整执行。

目标：让项目具备可靠的异步能力，同时**业务层完全不感知 broker**。

## 4.1 Publisher 抽象

在 application 层定义消费者视角的小接口，例如：

```go
type EventPublisher interface {
    Publish(ctx context.Context, event Event) error
}
```

要求：

```text
业务代码不得 import kafka / nats SDK
Event 结构不得直接暴露 broker 概念（partition/offset/subject）
提供内存实现用于单元测试
```

## 4.2 消息契约

设计 Event 信封：

```text
event_id (唯一, 用于幂等)
event_type (含版本，例如 user.created.v1)
occurred_at
trace_context (W3C traceparent)
payload
```

说明 schema 演进策略：新增字段向后兼容、破坏性变更必须升版本号。

**payload 中禁止出现**：password / password_hash / token / 完整 PII。

## 4.3 Broker 接入

按 ADR-0009 选定的 broker 实现 Producer / Consumer。

配置项：`brokers / topic 或 stream / consumer group / 超时 / 重试参数 / enabled`。

**`enabled=false` 时，整个应用必须仍能正常启动并提供 HTTP 服务。**

## 4.4 Outbox Pattern

如果业务需要"数据库写入与消息发布"的原子性：

```text
业务事务内写 outbox 表
独立 relay 进程/goroutine 读取并投递
投递成功后标记
失败重试，带退避
```

**如果 ADR-0009 判断第一版不需要 Outbox，请明确说明放弃它的后果**
（消息可能丢失 / 双写不一致），并写入 Known Limitations。

## 4.5 Consumer

实现至少一个真实消费者，例如 `user.created.v1` → 发送欢迎邮件（可用 no-op 邮件实现）。

要求：

```text
每条消息一个独立 context，带超时
handler 返回 error 时进入重试
panic 必须被捕获，不能杀死 consumer
consumer 生命周期由 bootstrap 管理，不用 init()
```

## 4.6 幂等消费

实现去重：基于 `event_id`，存储在 Redis 或数据库。

明确说明：**至少一次投递（at-least-once）是默认语义，消费者必须自己保证幂等。**

## 4.7 重试与 DLQ

```text
指数退避 + 抖动
最大重试次数
超过上限 → Dead Letter Queue / dead letter 表
DLQ 消息必须记录失败原因和原始 payload
```

提供查看/重放 DLQ 的最小手段（CLI 命令或 SQL 说明即可）。

## 4.8 优雅退出

收到 SIGTERM 后：

```text
停止拉取新消息 → 等待 in-flight 消息处理完成（带超时）→ 提交 offset/ack → 关闭连接
```

必须有测试覆盖"退出时不丢消息"。

## 4.9 Trace 传播

Publish 时注入 W3C traceparent，Consume 时提取并作为 span 的 parent。

**目标：一次 HTTP 请求触发的异步处理，在 trace 中是同一条链路。**
（具体 instrumentation 在 Phase 5 完成，本阶段先保证 context 字段被正确传递和读取。）

## 4.10 Tests

```text
Publisher 单元测试（内存实现）
Consumer handler 单元测试
幂等去重测试
重试与 DLQ 测试
优雅退出测试
Broker 集成测试（testcontainers；不可用则 SKIPPED）
```

## 4.11 Docker Compose

将 broker 加入 compose，但放在**独立 profile** 中，不默认启动：

```bash
docker compose --profile messaging up -d
```

## 4.12 Phase 4 输出

执行 §0.4 全部门禁。输出：

```text
Messaging Architecture
Event Contracts
Delivery Guarantees（明确说明是 at-least-once / at-most-once）
Idempotency Strategy
Retry & DLQ Design
Tests
Commands Run
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。

---

# Phase 5 — Observability

先读取 `STATE.md`。

目标：让项目默认具备 `Logs / Metrics / Tracing / Request Correlation / Dependency Observability`，
但**不污染业务逻辑**。

## 5.1 OpenTelemetry

配置：`service.name / service.version / deployment.environment / OTLP endpoint / sampling / enabled`。

**Observability 必须可以关闭；关闭时业务功能完全正常。**

## 5.2 Trace Provider

统一 `NewTracerProvider / Shutdown`。在 OTLP gRPC / HTTP 中选一个默认。

## 5.3 HTTP Tracing

记录 `method / route / status / duration`。

Span name 必须使用路由模板：

```text
GET /api/v1/users/{id}     ✅
GET /api/v1/users/8c91...  ❌ 高基数
```

## 5.4 Trace Context

支持 W3C Trace Context，跨 HTTP / service / database / redis / **消息队列** / external API 传播。

## 5.5 Database Tracing

为 PostgreSQL 添加 tracing。

**不得记录**：password / 完整 SQL 参数 / PII。
如果 instrumentation 自动记录语句，必须确认不会泄漏敏感数据。

## 5.6 Redis Tracing

记录 `command name / latency / error`。
**不得记录** token / session content / user PII。

## 5.7 Messaging Tracing

Producer span 与 Consumer span 通过 traceparent 关联。

验证：一次 `POST /api/v1/users` 能在 trace 中看到
`HTTP Span → Service Span → PostgreSQL Span → Publish Span → Consumer Span`。

## 5.8 Logging Correlation

日志自动包含 `request_id / trace_id / span_id`。

**不得让每个 Service 手动提取 trace_id。**

## 5.9 Metrics

技术指标：

```text
http_requests_total / http_request_duration_seconds / http_errors_total
db_pool_* / redis_operation_*
messaging_published_total / messaging_consumed_total
messaging_consume_duration / messaging_retry_total / messaging_dlq_total
rate_limit_rejected_total
```

业务指标：

```text
user_registration_total / login_total / login_failure_total
```

## 5.10 Prometheus

提供 `/metrics`。分析应该 public expose / internal only / 独立 admin port，并给出生产建议。

## 5.11 Label Cardinality

**禁止**将 `user_id / email / request_id / full URL / token` 作为 Metrics Label。

Route 一律使用模板形式。

## 5.12 Structured Logging 约定

日志等级约定：

```text
4xx 不一定是 ERROR
5xx 是 ERROR
expected not found 不是 ERROR
限流拒绝是 WARN 或 INFO，不是 ERROR
```

## 5.13 Sensitive Data 复查

再次全量检查日志与 trace，禁止出现：

```text
password / password_hash / JWT / refresh token / Authorization / Cookie
database password / Redis password / broker credentials
```

## 5.14 Observability Docker Stack

Docker Compose 增加可选 profile：

```bash
docker compose --profile observability up -d
```

包含：OpenTelemetry Collector / Prometheus / Grafana / Tempo 或 Jaeger。

**不要默认开发时全部启动。**

## 5.15 Dashboard

提供至少一个基础 Grafana Dashboard 配置或文档，关注：

```text
RPS / Latency P50 P95 P99 / HTTP 5xx / DB Connections
Login failures / User registrations / Consumer lag / DLQ size
```

## 5.16 Tests

```text
request ID propagation
trace context propagation（含 HTTP → messaging）
metrics endpoint
observability disabled 时应用正常工作
sensitive data not logged
```

## 5.17 Phase 5 输出

执行 §0.4 全部门禁。若 Docker 可用，验证 Collector / Prometheus / Grafana / Tempo 能启动；
不可用则 SKIPPED。输出：

```text
Telemetry Architecture
Metrics Added
Trace Coverage
Logging Changes
Docker Observability Stack
Tests / Verification Results
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。

---

# Phase 6 — Infrastructure / Docker / CI / Kubernetes

先读取 `STATE.md`。

目标：**开发可启动 / CI 可验证 / 镜像可构建 / 生产可部署 / Kubernetes 可运行。**

## 6.1 Dockerfile

Multi-stage build。要求：small image / non-root / minimal packages / CA certificates。

分析 `distroless / scratch / alpine / debian-slim` 并选择一种，说明理由。
避免无意义的 alpine glibc 兼容问题。

## 6.2 Build Metadata

构建时注入 `version / commit /（可选）build time`，
通过 `/version` 或 health metadata 暴露**非敏感**版本信息。

## 6.3 Docker Compose

开发环境包含 `postgres / redis /（broker）`。

分析 API 本身是否应该默认在 compose 中启动（本地跑通常更好），给出理由。

profile 划分：

```text
default        → postgres, redis
messaging      → broker
observability  → collector, prometheus, grafana, tempo/jaeger
full           → 全部
```

## 6.4 Makefile 完善

```bash
make run build test test-race lint fmt vet vuln
make generate sqlc
make migrate-up migrate-down migrate-create
make docker-up docker-down docker-build
```

## 6.5 GitHub Actions

Pull Request 流程：

```text
checkout → setup go → cache → go mod download
→ fmt check → vet → lint
→ unit test → race test → integration test
→ govulncheck → build
→ openapi lint
```

根据时间成本合理拆分 Job（快速反馈 job 与慢速 job 分离）。

## 6.6 Integration Test in CI

分析 GitHub Actions Service Container vs testcontainers-go。

**如果 Repository Test 已使用 Testcontainers，优先保持一致**，避免两套机制。

## 6.7 Docker Build CI

main 分支构建镜像，tag 为 commit SHA。
Release tag `v1.2.3` 同时产出 `v1.2.3 / v1.2 / v1`。

如果你认为 mutable tags 不合适，说明理由并调整。

## 6.8 Security in CI

加入 `govulncheck`，可选 Trivy（镜像 + 依赖）。

**不要加入过多重型安全工具。**

## 6.9 Kubernetes Manifests

```text
Deployment / Service / ConfigMap / Secret example
Ingress / HPA / PodDisruptionBudget
Migration Job（一次性，与 Deployment 分离）
Consumer Deployment（如果 Phase 4 引入了消费者，独立于 API 部署）
```

## 6.10 Deployment 配置

正确配置 `replicas / rolling update / resources.requests / resources.limits / terminationGracePeriodSeconds`。

`terminationGracePeriodSeconds` 必须大于应用的 shutdown timeout。说明这个关系。

## 6.11 Probes

```text
livenessProbe  → /live
readinessProbe → /ready
```

**PostgreSQL 临时抖动不得触发 liveness failure**，否则会产生重启风暴。说明原因。

## 6.12 Security Context

```text
runAsNonRoot: true
readOnlyRootFilesystem: true（where possible）
allowPrivilegeEscalation: false
capabilities: drop ALL
```

## 6.13 Config & Secret

普通配置用 ConfigMap，敏感配置用 Secret。

Git 中**只能**提供 `secret.example.yaml`。**禁止真实 Secret 入库。**

## 6.14 HPA

基于 CPU 提供基础 HPA。说明未来何时应该升级为 custom metrics（RPS / consumer lag）。

## 6.15 Graceful Shutdown 端到端验证

确认链路匹配：

```text
SIGTERM → readiness false → 停止接收新请求
→ 处理完 in-flight 请求 → 停止 consumer → 关闭依赖 → 退出
```

## 6.16 多环境

建议 `local / development / staging / production`。

**不要复制四套完全独立 YAML。** 在 Kustomize / Helm / 基础 manifests 中选最简单的一种。

## 6.17 README Deploy 章节

说明 `Docker build / Docker Compose / Kubernetes deployment / Environment variables / Migration / Rollback`。

## 6.18 Phase 6 输出

执行 §0.4 全部门禁，另加（可用时）：

```bash
docker build .
docker compose config
kubectl apply --dry-run=client -f deployments/
```

不可用则 SKIPPED。输出：

```text
Infrastructure Files
Docker Architecture
CI Pipeline
Kubernetes Strategy
Security Decisions
Verification
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。

---

# Phase 7 — Scaffold CLI

先读取 `STATE.md`。

目标：开发 `goscaffold` CLI，减少重复机械工作。

**它不是 Low-Code 平台。保持简单。**

## 7.1 Commands

至少：

```bash
goscaffold new my-service --module github.com/acme/my-service
goscaffold module user
goscaffold migration create_users
goscaffold version
```

## 7.2 Templates

模板通过 `embed.FS` 内嵌，**CLI 运行不得依赖外部模板目录**。

## 7.3 new Command

参数：`name / --module / --output`

可选开关：`--with-redis / --with-messaging / --with-observability`

**不要第一版就做十几个开关。**

职责：创建目录 / 复制模板 / 替换 module name / 替换 service name / 初始化基础配置。

## 7.4 Module Generator

```bash
goscaffold module order
```

生成：domain / service / repository contract / handler / DTO / test skeleton /（可选）SQL query 文件。

**模板必须符合项目当前真实架构。禁止重新引入 Java 式分层。**

## 7.5 Safety

**绝对禁止覆盖已有文件。**

文件存在时默认 **fail**（或 skip 并报告 conflict）。
预留 `--force` 扩展点，第一版可不实现。

## 7.6 Migration Generator

```text
20260829123456_create_users.up.sql
20260829123456_create_users.down.sql
```

时间戳必须避免同秒冲突，必要时提高精度。

## 7.7 Naming

正确处理 `PascalCase / camelCase / snake_case / 复数`。

**不要引入大型 inflection library，除非确实必要。**

## 7.8 CLI Framework

分析 `标准库 flag / Cobra / urfave-cli`。

如果只有几个命令，**不要为了流行而引入过重依赖**。

## 7.9 Dry Run

```bash
goscaffold module order --dry-run
```

显示将创建哪些文件，不写磁盘。

## 7.10 Config

评估是否支持 `.goscaffold.yaml`。**如果第一版没有必要，就不要实现。**

## 7.11 Plugin

**只设计扩展点，不实现 Plugin System。**

当前保持：embedded official template。

## 7.12 Template Drift（关键设计题）

CLI Template 与 Starter Project 极易 drift。

**必须设计解决方案，优先 single source of truth。**

禁止长期维护两套独立代码。给出你的方案并说明为什么可行。

## 7.13 CLI Tests

```text
new project / module generation / migration generation
existing file conflict / dry-run / name conversion
```

**测试必须在临时目录运行，禁止污染项目目录。**

## 7.14 Generated Code Verification（关键质量门槛）

CI 增加一个 job：

```bash
goscaffold new demo --module github.com/example/demo
cd demo && go mod tidy && go vet ./... && go build ./... && go test ./...
```

**确保脚手架生成的项目真的能编译和运行。**

## 7.15 Phase 7 输出

执行 §0.4 全部门禁，并在临时目录验证生成项目。输出：

```text
CLI Commands
Template Architecture
Template Drift Solution
Safety Mechanisms
Generation Examples
Tests / Generated Project Verification
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。

---

# Phase 8 — Documentation & Final Audit

先读取 `STATE.md` 和全部 ADR。

**本阶段不要继续加功能。**

目标：把项目变成**别人真正可以使用、新人真正可以上手、团队真正可以长期维护**的企业级 Starter。

## 8.1 README

包含：

```text
Project Overview / Why This Scaffold / Features / Architecture
Directory Structure / Quick Start / Configuration
Database / Redis / Messaging / Authentication / Rate Limiting / API
Testing / Observability / Docker / Kubernetes / CI-CD
Scaffold CLI / Security / Development Workflow / Troubleshooting
```

Quick Start 必须真实可执行：

```bash
git clone ... && cd ...
cp .env.example .env
docker compose up -d
make migrate-up
make run
```

然后 `GET /live`、`GET /ready` 可用。

**你必须实际按这个流程走一遍并报告结果**（环境不允许则 SKIPPED）。

## 8.2 docs/architecture.md

说明 dependency direction / layer responsibilities / interface placement /
repository design / transaction design / error design / configuration /
messaging / observability / authentication。

**重点解释"为什么这样设计"，而不是只描述目录。**

## 8.3 docs/development.md

`How to add module / API / migration / query / dependency / test / debug`。

## 8.4 API 文档一致性

确认 `api/openapi.yaml` 完整覆盖 auth / users / error schemas / pagination。

**执行一次 drift 检查**：所有已注册路由与 spec 逐一比对，列出差异并修复。

## 8.5 docs/deployment.md

`Docker / Kubernetes / Environment variables / Secrets / Migration / Health checks / Observability / Rollback`。

## 8.6 docs/security.md

`Password / JWT / Refresh Token / Secrets / Logging / Rate Limit / CORS /
Dependency vulnerabilities / Container security / Messaging security`。

## 8.7 Architecture Audit

从零审计整个项目，查找：

```text
过度抽象 / 重复 abstraction / 巨大 interface / 无价值 wrapper
global state / hidden dependency / utils-common 垃圾包
循环依赖风险 / 过深目录 / 过多 DTO mapping
```

**发现即重构。**

## 8.8 Go Idiom Audit

```text
package naming / interface size / error wrapping / context propagation
defer usage / goroutine lifecycle / resource closing
pointer vs value receiver / constructor usage / zero value
```

## 8.9 Concurrency Audit

```text
goroutine leak / race condition / channel misuse
context cancellation / background worker lifecycle / shared state
```

执行 `go test -race ./...`。

## 8.10 HTTP Audit

```text
timeouts / body size limit / panic / request cancellation
graceful shutdown / status code / content type / CORS / rate limit
```

## 8.11 Database Audit

```text
N+1 / missing index / transaction 正确性 / connection leak
context / timeout / SELECT * / SQL injection / migration rollback 可用性
```

## 8.12 Messaging Audit

```text
消息丢失路径 / 重复消费 / 幂等性 / consumer 泄漏
DLQ 可运维性 / 优雅退出 / 契约版本兼容
```

## 8.13 Security Audit

检查是否泄漏：

```text
password / password hash / JWT / refresh token / Authorization
database credentials / Redis credentials / broker credentials / internal errors
```

## 8.14 Dependency Audit

对每个依赖回答：**为什么存在 / 能否用标准库替代 / 是否仍然需要。**

删除不必要依赖，保持依赖树尽量小。

## 8.15 Performance Audit

**不要过早优化。** 只检查明显问题：

```text
unbounded allocation / large body / unbounded pagination
bad DB query / connection pool misuse / excessive JSON conversion
high-cardinality metrics
```

## 8.16 Generated Scaffold Audit

```bash
goscaffold new final-test
cd final-test && go mod tidy && go build ./... && go test ./... && go test -race ./...
```

启动后验证 `/live` `/ready` `login` `users`。

## 8.17 Clean Repository

检查 Git 仓库，禁止包含：

```text
.env / database password / JWT secret / IDE cache
binary / temporary files / coverage artifacts / local database volume
```

完善 `.gitignore`。

## 8.18 Final Verification Gate（客观门槛）

执行：

```bash
go fmt ./... && go vet ./... && go build ./...
go test ./... && go test -race ./...
golangci-lint run
govulncheck ./...
```

环境允许时另加：

```bash
docker build .
docker compose config
kubectl apply --dry-run=client -f deployments/
```

**必须全部满足以下客观条件，否则继续整改：**

```text
[ ] golangci-lint 零告警（不得靠 nolint 掩盖，例外必须逐条说明理由）
[ ] govulncheck 零 HIGH/CRITICAL
[ ] go test -race 全部通过
[ ] 核心包（domain/service/handler）测试覆盖率 >= 70%，并贴出 go test -cover 真实输出
[ ] 直接依赖数量 <= 20，且每一个都在 Dependency Audit 中有存在理由
[ ] 全仓库 grep 不到 global mutable state
[ ] 全仓库 grep 不到 password/token 被写入日志的路径
[ ] 所有已注册 HTTP 路由都存在于 api/openapi.yaml
[ ] goscaffold 生成的项目能通过 go build + go test
[ ] README Quick Start 实际走通（或明确 SKIPPED 及原因）
[ ] STATE.md 与实际代码一致
```

**不要用主观打分（如"架构 9/10"）代替上述客观检查。**
主观评分可以给，但只作为补充说明，不作为通过标准。

## 8.19 最终交付报告

输出正式工程审计报告：

```markdown
# Enterprise Go Scaffold Final Report

## 1. Architecture
## 2. Tech Stack
## 3. Directory Structure
## 4. APIs
## 5. Authentication & Authorization
## 6. Rate Limiting
## 7. Database
## 8. Cache
## 9. Messaging
## 10. Observability
## 11. Testing
## 12. Docker
## 13. Kubernetes
## 14. CI/CD
## 15. Scaffold CLI
## 16. Security
## 17. Performance
## 18. Dependency Review
## 19. Commands Executed（真实输出 / SKIPPED 说明）
## 20. Final Verification Gate 检查表（逐项 pass/fail + 证据）
## 21. Known Limitations
## 22. Recommended Future Work
```

**最终目标：**

> 一套团队可以真正拿去创建新企业 Go 服务的生产级脚手架，
> 而不是展示性质的 Demo。
