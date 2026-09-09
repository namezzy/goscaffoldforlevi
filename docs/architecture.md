# Architecture

本文档说明**为什么这样设计**。目录清单和决策论证在 [`docs/adr/`](./adr/)，本文不重复。

阅读顺序建议：先读 [ADR-0002 目录结构与依赖方向](./adr/0002-project-layout.md)，再读本文。

| 主题 | 位置 |
|------|------|
| 技术选型 | [ADR-0001](./adr/0001-tech-stack.md) |
| 目录与依赖方向 | [ADR-0002](./adr/0002-project-layout.md) |
| 错误模型 | [ADR-0003](./adr/0003-error-model.md) |
| API 约定 / 分页 | [ADR-0004](./adr/0004-api-conventions.md) |
| 数据访问 / 事务 | [ADR-0005](./adr/0005-data-access.md) |
| ID 策略 | [ADR-0006](./adr/0006-id-strategy.md) |
| 认证授权 | [ADR-0007](./adr/0007-auth.md) |
| 限流 | [ADR-0008](./adr/0008-rate-limiting.md) |
| 消息与异步 | [ADR-0009](./adr/0009-messaging.md) |
| 可观测性接线 | [ADR-0010](./adr/0010-observability.md) |

> 手工调试用 `api/bruno/`（Bruno 集合，登录一次后 token 自动流转）。
> 自动化回归看 `test/blackbox/` 与 `make test-integration`。

> **本文只回答"为什么这样设计"。** 具体怎么做见:
> [`development.md`](development.md)（加模块 / 端点 / 迁移 / 查询 / 依赖 / 测试 / 调试）、
> [`deployment.md`](deployment.md)（镜像 / Secret / 迁移顺序 / 探针 / 回滚）、
> [`security.md`](security.md)（防什么、不防什么、已知暴露面）、
> [`final-report.md`](final-report.md)（终审报告：门禁、证据、未验证项）。
> 当前状态以 [`../STATE.md`](../STATE.md) 为准。

---

## 1. 模块边界：以 User 为例

这是整个架构最容易被做错的地方——**结构复用**。
四个"看起来都是 User"的结构，实际上有四种不同的变更驱动力：

```text
┌─────────────────────────────────────────────────────────────────┐
│ userhttp.CreateUserRequest / UserResponse        DTO             │
│   变更驱动：客户端需求、API 版本                                    │
│   知道：JSON 标签、字段可选性、HTTP 语义                            │
│   不知道：数据库、密码如何哈希                                      │
├─────────────────────────────────────────────────────────────────┤
│ user.User                                        Domain Entity   │
│   变更驱动：业务规则                                               │
│   知道：什么是合法的 Email、Status 如何流转、密码不可读出            │
│   不知道：JSON、SQL、HTTP                                         │
├─────────────────────────────────────────────────────────────────┤
│ db.User (sqlc 生成)                              Database Model  │
│   变更驱动：schema 迁移                                            │
│   知道：列名、可空性、pgtype                                        │
│   不知道：业务规则                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 可以复用的

- `user.ID`、`user.Email`、`user.Status` 这类**值对象**可以贯穿三层。
  它们的变更驱动力单一（业务定义），复用不会产生耦合。
- `messaging.Event`、`apperr.Error` 这类**纯数据结构**可跨包复用。

### 绝不可复用的

**`user.User` 不能直接作为 HTTP 响应返回。**

即使今天字段完全一致，也必须写映射函数。理由：

1. `User` 含 `PasswordHash`。直接序列化 = 一个 `json:"-"` 的遗漏就泄漏凭据。
   `UserResponse` 从结构上不包含该字段——**这是类型系统提供的保证，不是纪律**。
2. 加一个内部字段（如 `LoginAttempts`）会立刻变成 API 破坏性变更。
3. 反向也成立：API 要加 `avatar_url` 这种展示字段，不应污染领域实体。

**`db.User` 不能穿透到 Service。** 它由 sqlc 生成，schema 一改就变，
且携带 `pgtype.Timestamptz` 这类驱动类型。

### 映射的代价与位置

三层意味着两次映射，约 40 行样板代码：

```text
userhttp:  CreateUserRequest → (Service 参数)     ; user.User → UserResponse
userpg:    user.User ↔ db.User
```

映射函数放在**外层**（`userhttp` / `userpg`），不放在 `user` 包——
否则 `user` 就要认识 DTO 和 db 类型，依赖方向反转。

### Repository 接口的粒度

接口由 `user.Service` 声明，只包含**它实际调用的方法**。
不存在 `GenericRepository[T]`，也不存在"以后可能用到"的方法。

判断标准：如果一个方法在 Service 中没有调用点，它就不应该在接口里。

---

## 2. 配置

### 结构

```go
type Config struct {
    App       App
    HTTP      HTTP
    Database  Database
    Redis     Redis
    Auth      Auth
    RateLimit RateLimit
    Messaging Messaging
    OTel      OTel
}
```

嵌套结构而非扁平大结构：让 `NewPool(ctx, cfg.Database)` 这样的构造函数
只拿到它需要的部分，而不是整个 `Config`。
**把整个 Config 传给每个组件是一种隐蔽的全局状态。**

### 来源与优先级

```text
显式环境变量  >  .env 文件  >  代码中的默认值
```

- 生产：只用环境变量 / Kubernetes Secret，**不部署 .env**
- 开发：`.env`（`.gitignore` 排除），`.env.example` 提供模板
- **不支持 YAML**。理由见 ADR-0001：多源配置合并会带来
  "这个值到底从哪来的"这类排查成本，而本项目没有需要 YAML 的层级配置

### Fail Fast

```text
main()
  → config.Load()      读取环境 → 解析类型 → 填默认值
  → config.Validate()  ← 失败即 os.Exit(1)，不启动
  → 后续初始化
```

`Validate()` 必须检查（不完整列表）：

| 项 | 规则 |
|---|---|
| `APP_ENV` | 必须是 `development` / `staging` / `production` 之一 |
| `HTTP_PORT` | 1-65535 |
| `DATABASE_URL` | 非空，可解析为 URL |
| `AUTH_JWT_SECRET` | **≥ 32 字节**；`production` 下额外禁止使用 `.env.example` 中的示例值 |
| `AUTH_ACCESS_TTL` | > 0 且 ≤ 1h（防止误配成长期 token） |
| 各类 timeout | > 0 |
| `HTTP_TRUSTED_PROXY_COUNT` | ≥ 0 |

最后一条尤其重要：**用示例密钥启动生产环境是真实发生过的事故**。
校验器应当主动阻止，而不是相信部署流程。

### 敏感值处理

`Config` 中的密钥字段用具名类型 `Secret string`，实现
`String() string { return "[REDACTED]" }` 和 `LogValue() slog.Value`。
这样 `slog.Any("config", cfg)` 或 `fmt.Printf("%v")` 都不会泄漏。

**默认安全，而不是要求每个调用点记得脱敏。**

---

## 3. 缓存

### v1 明确不做 User 缓存

Cache-Aside 是一个正确的模式，但 v1 没有它的适用条件：

- User 查询不是热点（登录时查一次，之后靠 JWT，不再查）
- 加缓存立刻引入一致性问题：更新用户后要失效缓存，
  失效失败就是脏数据，而这个 bug 只在并发下偶发

**为了在脚手架里"展示缓存"而给 User 加缓存，是典型的用复杂度换演示效果。**

Redis 在 v1 的真实用途是限流（ADR-0008）和幂等去重的快路径。

### 引入缓存时的原则

将来某个查询确实成为热点时，遵循：

```text
Get(ctx, id)
  → Redis GET
      ├─ hit  → 返回
      └─ miss → PostgreSQL
                  → Redis SET (带 TTL)
                  → 返回
```

**`Cache Failure != Business Failure`**：
Redis 的任何错误（连接失败、超时、反序列化失败）都必须降级为 miss，
记 WARN + 指标，继续走数据库。**缓存层绝不返回 error 给业务层。**

失效策略：写操作后 `DEL`（而非 `SET` 新值），让下次读重新加载。
`DEL` 失败时依赖 TTL 兜底，因此 **TTL 是必需的，不允许永不过期**。

> **When to Revisit**：某个查询 QPS > 500 且 P95 > 50ms，
> 或数据库 CPU 被单一查询主导时。

---

## 4. 可观测性

设计目标：**第一天就能接入，且业务代码零感知。**

### 分层

```text
业务代码
   │  只用 slog（从 context 取 logger）和自己声明的指标接口
   ▼
internal/telemetry     ← 唯一 import OpenTelemetry SDK 的地方（+ 各 adapter）
   │
   ▼
OTLP Collector → Tempo / Prometheus / Grafana
```

`depguard` 禁止 `internal/user`、`internal/auth` import `go.opentelemetry.io/otel`。

### 三根支柱的关联

```text
                  ┌──────────────┐
   HTTP 请求 ────▶ │  RequestID   │ 生成/复用 request_id
                  └──────┬───────┘
                         ▼
                  ┌──────────────┐
                  │   Tracing    │ 创建 root span → trace_id, span_id
                  └──────┬───────┘
                         ▼
                  ┌──────────────┐
                  │   Logging    │ 把 request_id/trace_id/span_id
                  └──────┬───────┘  注入 context 中的 slog.Logger
                         ▼
                    业务代码 log.InfoContext(ctx, ...)
                         │
                         └─▶ 自动携带三个 ID
```

关键设计：**自定义的 `slog.Handler` 从 context 中提取 trace 上下文并自动附加**。
业务代码写 `logger.InfoContext(ctx, "user created", "user_id", id)` 即可，
**不需要手动传 trace_id**。这是"不污染业务逻辑"的具体含义。

### 可关闭

`OTEL_ENABLED=false` → 使用 no-op TracerProvider。
instrumentation 代码仍然执行，但开销是几次空方法调用。
**关闭时业务功能必须完全正常**——Phase 5 有对应测试。

### 高基数防护

Metrics label 与 span name 一律使用**路由模板**（`/api/v1/users/{id}`），
永不使用实际路径。禁止把 `user_id` / `email` / `request_id` / token 作为 label。

原因：Prometheus 的时间序列数量 = 各 label 基数的乘积。
一个 `user_id` label 就能让序列数从几十涨到几十万，直接打爆存储。

### 实现状态（Phase 5 完成）

| 面 | 落地位置 |
|---|---|
| Trace | HTTP（`internal/httpx/observability.go`）、PostgreSQL（`internal/postgres/tracing.go`）、Redis（`internal/redis/tracing.go`）、NATS 发布与消费（`internal/messaging/natsmq/tracing.go`） |
| Metrics | `internal/telemetry/metrics.go`，独立端口导出（`endpoint.go`），默认 `127.0.0.1:9464`（worker 自动 +1） |
| Log 关联 | `internal/telemetry/logger.go` 的 `contextHandler` 注入 `trace_id` / `span_id` |
| 跨进程串联 | traceparent 经 outbox 列与 NATS header 传递，consumer span 是 publish span 的**子节点** |

**属性白名单**：DB 只记 sqlc 的 query 名，Redis 只记 `cmd.Name()`，span 的
error status 只记"失败了"不记原因——驱动的错误文本会引用参数。
详见 [ADR-0010](./adr/0010-observability.md)。

本地起观测栈：`docker compose --profile observability up -d`
（Grafana http://localhost:3000，已 provision Service Overview 仪表盘）。

---

## 5. 中间件顺序

```text
①  Recovery
②    RequestID
③      Tracing
④        Logging
⑤          Metrics
⑥            SecurityHeaders
⑦              CORS
⑧                BodyLimit
⑨                  Timeout
⑩                    RateLimitByIP
⑪                      Authentication
⑫                        RateLimitByUser
⑬                          Authorization
                              Handler
```

逐条理由：

| # | 为什么在这个位置 |
|---|-----------------|
| ① Recovery | **必须最外层**。它要能捕获其他所有中间件的 panic。放在里面，外层中间件的 panic 会直接杀死进程 |
| ② RequestID | 在 Tracing/Logging 之前，因为两者都要引用它。也让 panic 日志能带上 request_id |
| ③ Tracing | 在 Logging 之前，这样日志才能携带 trace_id。反过来则 trace 上下文尚未建立 |
| ④ Logging | 在 Metrics 之前无实质影响，但排在前面能记录更完整的耗时 |
| ⑤ Metrics | 应统计**所有**进入的请求，包括被限流和被拒绝认证的。放在限流之后会漏掉 429，而 429 的量恰恰是最需要监控的 |
| ⑥ SecurityHeaders | 在 CORS 之前，这样即使 CORS 短路返回 preflight，响应也带着安全头。放在后面，preflight 响应会缺头 |
| ⑦ CORS | **必须在限流之前**。浏览器 preflight（OPTIONS）不带凭据、量大，让它消耗业务配额会误伤正常用户。CORS 中间件对 preflight 直接短路返回，不进入后续 |
| ⑧ BodyLimit | 在 Timeout 之前，先限制大小再限制时间。避免超大 body 在超时窗口内持续占用内存 |
| ⑨ Timeout | 在所有业务处理之前设定 deadline，让下游的 DB/Redis 调用都继承取消信号 |
| ⑩ RateLimitByIP | 在 Authentication **之前**：未认证的攻击流量不应先消耗 JWT 验证的 CPU |
| ⑪ Authentication | 产出 `Principal` 放入 context |
| ⑫ RateLimitByUser | 在 Authentication **之后**：此时才知道 UserID。这是 prompt 原始顺序中缺失的一环（见 ADR-0008） |
| ⑬ Authorization | 最内层。它需要 Principal，且只有确定要执行 handler 时才需要判权 |

登录路由额外挂载 `RateLimitLogin`（在 handler 内、解析出 email 之后触发），
因为它的限流维度依赖请求体。

**注意 ⑤ 与 ⑩ 的关系**：Metrics 在限流之前，所以 429 会被计入
`http_requests_total{status="429"}`。这是有意的——
限流拒绝量是核心监控指标。

---

## 6. 测试策略

### 分层与比例

```text
        ╱╲          E2E (docker compose 起全栈)
       ╱  ╲         少量，只覆盖"能跑起来"
      ╱────╲
     ╱      ╲       HTTP Handler (httptest)
    ╱        ╲      每个端点的 status/校验/权限/错误
   ╱──────────╲
  ╱            ╲    Repository (testcontainers + 真实 PostgreSQL)
 ╱              ╲   每个 SQL 查询、约束、事务
╱────────────────╲
      Service      (fake repository，纯内存)
   业务规则、错误分支、边界条件 —— 数量最多、跑得最快
```

### 应该 Mock 的

| 对象 | 理由 |
|------|------|
| Repository（在 Service 测试中） | 让业务规则测试不依赖数据库，毫秒级 |
| 时钟 | 测试过期逻辑必须能控制时间 |
| ID 生成器 | 断言需要确定性 |
| 外部网络调用（邮件、第三方 API） | 不可控、慢、有副作用 |
| EventPublisher | 用内存实现断言"发了什么事件" |

### 绝不应该 Mock 的

| 对象 | 理由 |
|------|------|
| **PostgreSQL** | Mock 数据库只能验证"调用了这个方法"，无法验证 SQL 是否正确、约束是否生效、事务是否原子。**用 mock 测 SQL 等于没测**。用 testcontainers |
| **Redis（限流测试）** | 限流的正确性在于 Lua 脚本的原子性，mock 掉就把被测对象删了 |
| **HTTP 层** | `httptest.NewServer` 已经足够轻量，mock `http.ResponseWriter` 只会漏掉真实的编码/头/状态码问题 |
| **被测包内部的结构** | mock 自己的内部实现是在测试实现细节而非行为 |

### 一条原则

> Mock 进程边界之外的东西。不要 mock 你自己的代码。

### Docker 不可用时

Repository / 限流的集成测试用构建标签或 `testing.Short()` 跳过，
并在测试输出中明确打印 SKIP 原因。**绝不改成 mock 版本蒙混通过**
（那会让"测试通过"失去意义）。CI 中 Docker 始终可用，因此覆盖率由 CI 保证。

---

## 7. Bootstrap

### 顺序与理由

```text
main()
 │
 ├─ 1. config.Load + Validate        ─── 失败即退出，不产生任何副作用
 ├─ 2. telemetry.NewLogger           ─── 尽早，让后续步骤的失败有结构化日志
 ├─ 3. telemetry.NewTracerProvider   ─── 在 DB/Redis 之前，让它们的初始化可被追踪
 ├─ 4. postgres.NewPool + Ping       ─── 必需依赖，失败即退出
 ├─ 5. redis.NewClient + Ping        ─── 必需（限流依赖），失败即退出
 ├─ 6. messaging.New (若 enabled)    ─── 可选依赖
 ├─ 7. repositories                  ─── 纯构造，不会失败
 ├─ 8. services                      ─── 纯构造
 ├─ 9. handlers                      ─── 纯构造
 ├─ 10. httpx.NewRouter              ─── 装配中间件与路由
 ├─ 11. httpx.NewServer              ─── 配置超时参数
 └─ 12. Run + 等待信号
```

设计要点：

- **有副作用的初始化（1-6）全部在前，纯构造（7-9）在后。**
  这样任何环境问题都在启动的前几毫秒暴露，而不是在装配一半时
- 每一步失败都返回 error 给一个 `run() error`，由 `main` 统一
  `log.Error` + `os.Exit(1)`。**`main` 中不出现业务逻辑，不出现 `panic`**
- 依赖显式传参构造，**没有全局变量**。
  `main.go` 会有 60-100 行装配代码——这是可读的显式代价，
  比 DI 框架的运行时反射好

### 为什么不用 DI 框架

wire / fx / dig 解决的是"依赖太多，手写麻烦"。
但本项目的依赖图是**一条链**（pool → repo → service → handler），
手写不麻烦。而 DI 框架会把"忘记提供某个依赖"从编译错误
变成运行时 panic，是纯粹的损失。

---

## 8. 优雅退出

### 顺序与理由

```text
SIGINT / SIGTERM
 │
 ├─ 1. readiness = false
 │      └─ Kubernetes 把 Pod 从 Service Endpoints 移除
 │         ★ 必须先于停止接收请求，否则会有一段时间流量仍被路由过来但被拒绝
 │         ★ 需要等待一小段时间（默认 5s）让 kube-proxy/Ingress 完成传播
 │
 ├─ 2. http.Server.Shutdown(ctx)
 │      └─ 停止接受新连接，等待 in-flight 请求完成
 │
 ├─ 3. 停止 messaging consumer + outbox relay
 │      └─ 停止拉取新消息，等待 in-flight 消息处理完
 │         未完成的不 ack，靠重投递 + 幂等兜住
 │
 ├─ 4. 关闭 NATS 连接
 ├─ 5. 关闭 Redis 客户端
 ├─ 6. 关闭 pgxpool
 │      ★ 数据库必须在 HTTP 和 consumer 之后关闭，
 │        否则正在处理的请求会拿到"连接池已关闭"错误
 │
 └─ 7. TracerProvider.Shutdown(ctx)
        ★ 最后关闭，确保前面各步骤产生的 span 能被导出
        ★ 这一步用独立的、未被取消的 context
```

### 时间预算

```text
SHUTDOWN_TIMEOUT (默认 30s)
  ├─ readiness 传播等待     5s
  ├─ HTTP drain            15s
  ├─ consumer drain         5s
  └─ 依赖关闭 + telemetry    5s

Kubernetes terminationGracePeriodSeconds 必须 > SHUTDOWN_TIMEOUT
（默认设 45s，留 15s 余量）
```

**这个不等式是 Phase 6 的硬性要求**：如果 grace period 小于 shutdown timeout，
Kubernetes 会在应用完成清理前发 SIGKILL，导致请求被截断、span 丢失。

### 可测试性

`Run(ctx)` 接受 context，收到 cancel 即执行上述流程并返回。
测试中用 `context.WithCancel` 触发，不需要真的发信号。
**这是"shutdown 流程可测试"的实现方式。**

---

## 9. 请求生命周期

```text
Client
  │ POST /api/v1/users  { email, password, name }
  ▼
┌────────────────────────────────────────────────────────────┐
│ httpx: Recovery → RequestID → Tracing → Logging → Metrics  │
│        → CORS → BodyLimit(1MB) → Timeout(30s)              │
│        → RateLimitByIP → Authentication → RateLimitByUser  │
│        → Authorization(PermUserCreate)                     │
└──────────────────────────┬─────────────────────────────────┘
                           ▼
┌────────────────────────────────────────────────────────────┐
│ userhttp.Handler.Create                                    │
│   decode JSON (DisallowUnknownFields)                      │
│   req.Validate()            → apperr.Invalid + Details     │
│   映射为 Service 参数                                        │
└──────────────────────────┬─────────────────────────────────┘
                           ▼
┌────────────────────────────────────────────────────────────┐
│ user.Service.Create                                        │
│   normalize email (lower + trim)                           │
│   业务校验（密码策略等）                                       │
│   hash password (Argon2id)                                 │
│   id := NewID()  (UUIDv7)                                  │
│   txManager.WithTx:                                        │
│     ├─ repo.Create(ctx, u)                                 │
│     └─ publisher.Publish(ctx, user.created.v1)  → outbox   │
└──────────────────────────┬─────────────────────────────────┘
                           ▼
┌────────────────────────────────────────────────────────────┐
│ userpg.Repository.Create                                   │
│   q := db.New(postgres.Querier(ctx, pool))   ← 拿到 tx      │
│   q.CreateUser(...)                                        │
│   23505 → apperr.Conflict("USER_EMAIL_TAKEN")              │
└──────────────────────────┬─────────────────────────────────┘
                           ▼
                     PostgreSQL
                           │
                           ▼  (返回路径)
┌────────────────────────────────────────────────────────────┐
│ userhttp: user.User → UserResponse  (无 PasswordHash)      │
│ 201 Created + Location + X-Request-ID                      │
└────────────────────────────────────────────────────────────┘

                     ...稍后，异步...
┌────────────────────────────────────────────────────────────┐
│ cmd/worker: outbox relay → NATS → consumer → 发欢迎邮件      │
│             traceparent 让它与上面的请求在同一条 trace 中     │
└────────────────────────────────────────────────────────────┘
```

---

## 10. 认证流程

```text
① 登录
   POST /auth/login {email, password}
     → RateLimitLogin(login_ip + login_id)   ← 失败即 429
     → 查用户（不存在时仍执行一次假哈希，保持时序一致）
     → Argon2id 验证
     → 任何失败都返回同一个 401 INVALID_CREDENTIALS
     → 成功：
        【事务】插入 refresh_session(family_id=new, token_hash=sha256(t))
        签发 JWT access (15m)
     → { access_token, refresh_token, expires_in, token_type: "Bearer" }

② 携带访问
   Authorization: Bearer <jwt>
     → 本地验签 + exp/iss/aud 校验（无远程调用）
     → Principal{UserID, Role} 入 context

③ 刷新（轮转）
   POST /auth/refresh {refresh_token}
     → 查 sha256(token)
        ├─ 不存在/过期        → 401
        ├─ revoked（重用！）  → 撤销整个 family + 安全告警 → 401
        └─ 有效 → 【事务】revoke 旧 + insert 新（同 family）
                 → 新的 access + refresh

④ 登出
   POST /auth/logout {refresh_token}
     → 撤销该 family 全部 session → 204
     → access token 最多 15 分钟后自然失效
```

授权判定分两类，见 [ADR-0007](./adr/0007-auth.md)：
角色权限走中间件，资源所有权走 Service 层的统一 helper。

---

## 11. 配置流程

```text
环境变量 / .env
      │
      ▼
config.Load()          解析 → 类型转换 → 填默认值
      │
      ▼
config.Validate()  ──失败──▶ log.Error + os.Exit(1)   ★ 不启动
      │ 成功
      ▼
Config{App, HTTP, Database, Redis, Auth, RateLimit, Messaging, OTel}
      │
      ├─ cfg.Database ──▶ postgres.NewPool
      ├─ cfg.Redis    ──▶ redis.NewClient
      ├─ cfg.Auth     ──▶ auth.NewTokenIssuer      ← Secret 类型，日志中显示 [REDACTED]
      ├─ cfg.RateLimit──▶ ratelimit.New
      ├─ cfg.Messaging──▶ messaging.New
      ├─ cfg.OTel     ──▶ telemetry.NewTracerProvider
      └─ cfg.HTTP     ──▶ httpx.NewServer

★ 每个组件只拿到自己那一段，不接收整个 Config
```

---

## 12. 错误流

见 [ADR-0003](./adr/0003-error-model.md) 的完整图。要点：

```text
pgx.ErrNoRows / 23505     ← 数据库错误码的知识只存在于 *pg 包
        ↓ 翻译
apperr.Error{Kind, Code, Message, cause}
        ↓ 传递（Service 不认识 PostgreSQL）
httpx.WriteError
        ↓ Kind → status（映射表只在一个文件里）
        ├─ 5xx: cause + stack → 日志(ERROR)，客户端只得到 request_id
        └─ 4xx: Code + Message + Details → 客户端，日志 INFO/WARN
```

---

## 13. 消息流

```text
【同步路径】
  Service ──【同一事务】──▶ users 表
                        └─▶ outbox_events 表 (含 traceparent)
                          COMMIT ← 原子，不会出现"写了但没事件"

【异步路径】cmd/worker
  relay: SELECT ... WHERE published_at IS NULL FOR UPDATE SKIP LOCKED
      → NATS JetStream publish
      → UPDATE published_at
        ★ publish 成功但 UPDATE 前崩溃 → 重复投递（at-least-once）

  consumer:
      【事务】INSERT processed_events ON CONFLICT DO NOTHING
             ├─ 0 行 → 已处理过，直接 ack
             └─ 1 行 → 执行副作用 → COMMIT → ack
      失败 → 指数退避+抖动重试 5 次 → dead_letter_events 表
```

投递语义是 **at-least-once**。外部副作用（发邮件）无法真正 exactly-once，
详见 [ADR-0009](./adr/0009-messaging.md)。

---

## 14. 依赖方向总图

```text
                    ┌──────────────┐
                    │   cmd/api    │  唯一知道"全部"的地方
                    │  cmd/worker  │
                    └──────┬───────┘
                           │ 组装（显式传参，无全局变量）
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
 ┌─────────────┐   ┌──────────────┐   ┌──────────────┐
 │  userhttp   │   │    userpg    │   │  messaging   │
 │  authhttp   │   │    authpg    │   │  ratelimit   │
 └──────┬──────┘   └──────┬───────┘   └──────┬───────┘
        │                 │                  │
        │ 使用            │ 使用             │ 使用
        ▼                 ▼                  ▼
 ┌──────────┐      ┌──────────┐       ┌──────────┐
 │  httpx   │      │ postgres │       │  redis   │   技术设施
 └──────────┘      └──────────┘       └──────────┘
        │                 │                  │
        └────────┬────────┴──────────────────┘
                 │  ★ 业务核心永远不指向这里
                 │
        ┌────────┴───────────┐
        │   user  /  auth    │  业务核心
        │  实体+规则+用例      │  由 depguard 强制不含
        │  +自己声明的接口     │  pgx/chi/redis/nats/otel/net-http
        └─────────┬──────────┘
                  ▼
        ┌────────────────────┐
        │ apperr / validate  │  仅标准库
        └────────────────────┘
```
