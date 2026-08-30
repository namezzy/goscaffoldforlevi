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
