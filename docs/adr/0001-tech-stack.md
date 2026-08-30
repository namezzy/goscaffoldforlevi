# ADR-0001: 技术选型

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

需要为一个企业级 Go 后端脚手架确定技术栈。目标场景是 REST API / 微服务 / SaaS Backend /
内部管理系统，要求长期维护、多人协作、可测试、可观测、安全、易部署。

约束：

- 团队规模假设 3-10 人，不是专职平台团队，运维预算有限
- 必须保持 Go 的简单性，不能演变成 Java Spring Boot
- 每一个依赖都要能回答"为什么不能用标准库"

基线中的 `Go 1.27+` 已确认可用（本机 `go1.27.0 darwin/arm64`）。

## Decision

| 领域 | 选择 | 是否 v1 必须 |
|------|------|-------------|
| HTTP 路由 | `net/http` + `chi/v5` | 是 |
| 数据库驱动 | `pgx/v5` + `pgxpool` | 是 |
| 查询生成 | `sqlc` | 是 |
| 数据库 | PostgreSQL 17 | 是 |
| 迁移 | `golang-migrate` | 是 |
| 缓存/限流存储 | Redis 8 + `go-redis/v9` | 是（但用途不是缓存，见 §Consequences） |
| 日志 | `log/slog`（标准库） | 是 |
| 追踪/指标 | OpenTelemetry SDK + OTLP | 是（默认关闭导出） |
| 指标暴露 | Prometheus client | 是 |
| Token | JWT (HS256) + opaque refresh token | 是 |
| ID | UUIDv7（见 ADR-0006） | 是 |
| 消息队列 | NATS JetStream | **否**（默认 `enabled=false`，见 ADR-0009） |
| API 文档 | OpenAPI 3.1，手写 spec + 路由一致性测试 | 是 |
| 校验 | 手写校验函数，**不引入** validator 库 | 是 |
| 配置 | `env` 解析，**不引入** Viper | 是 |
| 容器 | Docker multi-stage → distroless | 是 |
| CI | GitHub Actions | 是 |
| 编排 | Kubernetes + Kustomize | 是 |

目标：**直接依赖数量 ≤ 20**。

### 逐项论证

#### chi vs Gin vs Echo vs 标准库 ServeMux

**选择 chi。**

chi 的关键性质是它**没有自己的 Context 类型**——handler 就是 `http.HandlerFunc`，
中间件就是 `func(http.Handler) http.Handler`。这意味着：

- 所有 `net/http` 生态的中间件（含 `otelhttp`）直接可用，不需要适配层
- 业务 handler 可以在不引入 chi 的情况下被测试
- 将来换路由器，handler 一行都不用改

Gin 和 Echo 都引入了 `*gin.Context` / `echo.Context`。一旦 handler 签名依赖它，
框架就从"路由库"升级成了"框架锁定"，且它们的 Context 同时承担了
请求解析、响应写入、参数绑定、错误处理多重职责，违反 Small Interfaces。

标准库 `ServeMux` 自 Go 1.22 起已支持 `GET /users/{id}` 形式的方法+路径匹配，
是一个真实的候选。它不足的地方是：**没有路由组和组级中间件**。
本项目需要 `/api/v1` 下再分 `公开路由 / 需认证路由 / 需 admin 权限路由` 三个子树，
用 ServeMux 表达需要手工包装每个 handler，容易漏。chi 的 `r.Group` / `r.Route`
恰好解决这一点，代价是一个零传递依赖、纯 `http.Handler` 的小库。

> **When to Revisit**：如果最终路由树退化为单层、不再需要组级中间件，
> 应删除 chi 改用标准库 ServeMux。

#### pgx + sqlc vs GORM

**选择 pgx + sqlc。**

sqlc 从真实 SQL 生成类型安全的 Go 代码：SQL 在代码评审中可见，
执行计划可预测，没有运行时反射，没有隐式 JOIN，没有 N+1 惊喜。

GORM 的问题不在性能，而在**不可见性**：`Preload` / 关联 / 钩子 / 软删除
会生成开发者没写过的 SQL。在企业项目里，"我不知道这行代码会打出什么查询"
是长期维护的主要成本来源。

pgx 相对 `database/sql` 的价值：原生 PostgreSQL 协议、
更好的类型映射（`uuid` / `jsonb` / 数组）、`CopyFrom` 批量写、
连接池可观测指标——这些在 `database/sql` 上都要打折。

**已知代价**：sqlc 对动态查询（可选过滤条件）支持较弱。
应对方式写在 ADR-0005，不通过引入 query builder 解决。

> **When to Revisit**：如果出现超过 5 个查询无法用 sqlc 表达而必须手写 pgx，
> 说明领域已进入动态查询密集型，此时应评估 `squirrel` 之类的 builder。

#### slog vs zap

**选择 `log/slog`。**

标准库、零依赖、结构化、`slog.Handler` 是可替换接口。
zap 的性能优势（分配次数）在每秒数万条日志时才有意义；
在此之前，它带来的是一个额外依赖和一套非标准 API。

如果将来真的需要 zap 的性能，实现一个 `slog.Handler` 转发到 zap 即可，
**业务代码一行不用改**——这正是选择标准库接口的价值。

#### JWT vs Session

**选择 JWT access token + opaque refresh token 的混合模式。** 详见 ADR-0007。

纯 Session（每个请求查一次存储）更简单也更安全，但对于面向
SPA / 移动端 / 第三方集成的 API，无状态 access token 让水平扩容不依赖共享会话存储。

代价是 access token 在过期前无法撤销。用 **15 分钟短 TTL** 把这个窗口压到可接受。

#### Redis 是否应该默认存在

**是，但它的第一职责不是缓存。**

这是一个需要明确纠偏的点：脚手架很容易为了"展示缓存"而给 User 模块加 Cache-Aside，
而 User 查询根本不是热点。那样做只会增加缓存一致性问题，没有任何收益。

Redis 在 v1 的真实职责是：

1. **分布式限流计数**（ADR-0008）——多实例部署时必须共享状态，无法用内存实现
2. **消费幂等去重的快路径**（ADR-0009，权威记录仍在 PostgreSQL）

**User 模块的缓存在 v1 明确不做**，列入 Future Work。

#### OpenTelemetry 是否应该默认开启

**代码默认接入，导出默认关闭。**

`OTEL_ENABLED=false` 时使用 no-op TracerProvider，开销可忽略，
且不要求开发者本地跑一个 Collector。生产通过环境变量打开。

关键约束：**业务代码不得 import OpenTelemetry SDK**。
instrumentation 只出现在 `internal/httpx`、`internal/postgres`、
`internal/redis`、`internal/messaging`、`internal/telemetry` 中。

#### 消息队列：Kafka vs NATS JetStream vs Redis Streams

**选择 NATS JetStream，且 v1 默认关闭。**

诚实的结论是：**本脚手架的 v1 业务（User CRUD + Auth）并不需要消息队列。**
引入它是为了让脚手架具备"生产环境迟早会用到"的能力，而不是因为现在需要。

在此前提下，选型标准是**运维成本**而非吞吐量：

| 方案 | 运维成本 | 持久化 | 结论 |
|------|---------|--------|------|
| Kafka | 高（即使 KRaft 也要管分区/副本/保留策略/rebalance） | 强 | 吞吐和生态最好，但对 3-10 人团队是负担 |
| NATS JetStream | 低（单个 ~15MB 二进制，一个容器起） | 足够（文件存储、副本、durable consumer） | **选中** |
| Redis Streams | 最低（复用已有 Redis） | 弱（AOF 也难保证跨故障不丢） | 与限流共用实例会互相影响，否决 |

`EventPublisher` 抽象（ADR-0009）保证换成 Kafka 时业务代码零改动。

> **When to Revisit**：单 topic 持续写入 > 50 MB/s，
> 或需要 > 7 天消息保留做重放，或需要与已有 Kafka 生态（Connect / ksqlDB）集成时，
> 换 Kafka。

#### 明确拒绝引入的依赖

| 依赖 | 拒绝理由 |
|------|---------|
| Viper | 配置只需要"读环境变量 + 校验"。Viper 带来多源合并、热重载、大量传递依赖，全是本项目不用的能力 |
| validator (struct tag) | tag 里写规则不可测试、错误信息难本地化、复杂规则要写自定义函数反而更绕。手写 `Validate() error` 更直白 |
| wire / fx / dig | 手工 DI 在 `main.go` 里就是几十行显式构造。DI 框架把编译期错误变成运行期错误 |
| GORM | 见上 |
| logrus / zap | 见上 |
| 任意 utils / lo / funk | 泛型工具库会催生 `utils` 文化 |

## Consequences

### 正面

- 依赖树小，`go.sum` 可审计，供应链攻击面小
- 所有关键抽象（logger / router / tracer）都是标准库接口，可替换
- SQL 可见、可评审、可解释执行计划

### 负面 / 代价

- 手写校验代码量比 struct tag 多（约每个 DTO 20-40 行）
- sqlc 是构建期依赖，CI 和本地都要安装，动态查询要绕
- 不用 DI 框架意味着 `main.go` 会有 60-100 行显式装配代码
- Redis 成为 v1 硬依赖（因为限流），单机部署也要跑一个 Redis

### 后续影响

- Phase 2 必须先建立 sqlc + golang-migrate 工具链，否则 Phase 3 无法开工
- Phase 5 的 instrumentation 必须限制在技术包内，Phase 8 会审计业务包的 import
- Phase 8 的验收门槛包含"直接依赖 ≤ 20"，本 ADR 的拒绝清单是该门槛的依据

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| Gin + GORM + Viper + zap | 生态成熟、上手快、示例多 | 框架锁定、SQL 不可见、依赖树大 | 与"Simple > Clever / Explicit > Magic"直接冲突 |
| 纯标准库（ServeMux + database/sql） | 零依赖 | 缺路由组、类型映射弱、连接池指标缺失 | 省下的依赖不足以抵消手写成本 |
| Ent / Bun | 类型安全、schema 即代码 | 又一套 DSL，生成物庞大，逃逸到原生 SQL 时割裂 | sqlc 用真实 SQL 达到同样的类型安全 |

## When to Revisit

- 直接依赖数逼近 20 → 每新增一个必须先删一个
- 出现第二个服务需要验证同一批 JWT → 见 ADR-0007（换非对称签名）
- 消息吞吐或保留需求触及上表阈值 → 换 Kafka
- 路由树退化为单层 → 删除 chi
