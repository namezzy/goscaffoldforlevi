# ADR-0002: 目录结构与依赖方向

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

需要确定项目布局，并让依赖方向可被**编译期**强制，而不是只写在文档里。

约束：

- 新人 10 分钟内能理解结构
- 业务核心不得依赖 HTTP framework / PostgreSQL / Redis / 消息队列 / 云厂商 SDK
- 避免目录过深
- 禁止 `controller/service/dao/entity/vo/dto` 式机械分层
- 禁止 `common` / `utils` 垃圾包

## Decision

### 最终目录

```text
.
├── cmd/
│   ├── api/main.go              HTTP 服务入口
│   └── worker/main.go           消息消费者 + outbox relay 入口
├── internal/
│   ├── apperr/                  错误类型与 Kind（被所有层依赖，自身零依赖）
│   ├── config/                  配置结构、加载、校验
│   ├── validate/                校验原语（email/长度/枚举），零业务语义
│   ├── user/                    ★ 业务包
│   │   ├── user.go              实体 + 业务规则
│   │   ├── service.go           用例 + 它需要的 Repository 接口
│   │   ├── userpg/              PostgreSQL 实现
│   │   └── userhttp/            HTTP handler + DTO
│   ├── auth/                    ★ 业务包
│   │   ├── auth.go              Token/Session 实体、RBAC 策略表
│   │   ├── service.go           登录/刷新/登出用例 + 接口
│   │   ├── authpg/              refresh session 存储实现
│   │   └── authhttp/            handler + 认证/授权中间件
│   ├── httpx/                   服务器、路由装配、通用中间件、错误映射、响应写出
│   ├── postgres/                pgxpool、TxManager、sqlc 生成物
│   ├── redis/                   客户端封装
│   ├── ratelimit/               限流器（Redis token bucket）
│   ├── messaging/               EventPublisher 抽象 + NATS 实现 + outbox relay
│   └── telemetry/               slog handler、TracerProvider、MeterProvider
├── migrations/                  golang-migrate 的 .up.sql / .down.sql
├── queries/                     sqlc 输入（纯 SQL）
├── api/openapi.yaml             API 契约（单一真相）
├── deployments/
│   ├── docker/
│   └── k8s/
├── scripts/
├── docs/
│   ├── adr/
│   ├── architecture.md
│   ├── development.md
│   ├── deployment.md
│   └── security.md
├── prompts/                     开发用 prompt（不属于产物，可在交付时删除）
├── STATE.md
├── Makefile
├── sqlc.yaml
├── .golangci.yml
└── .env.example
```

### 每个目录的职责边界

| 包 | 负责 | 禁止放入 | 可以 import |
|----|------|---------|------------|
| `cmd/*` | 组装依赖、启动、优雅退出 | 任何业务逻辑 | 所有 internal 包 |
| `apperr` | 错误 Kind、构造、判定 | HTTP 状态码、任何业务错误码常量之外的逻辑 | 仅标准库 |
| `config` | 配置结构、`Load()`、`Validate()` | 使用配置的逻辑 | 标准库 + godotenv |
| `validate` | `Email()` `Len()` `OneOf()` 等原语 | 业务规则（如"密码至少 12 位"） | 标准库 |
| `user` / `auth` | 实体、业务规则、用例、**它自己需要的接口** | SQL、HTTP、pgx、chi、redis、nats | `apperr` `validate` + 标准库 |
| `user/userpg` `auth/authpg` | Repository 实现、DB 错误翻译 | 业务判断 | 父包 + `postgres` + `apperr` + pgx |
| `user/userhttp` `auth/authhttp` | 请求解析、DTO 映射、响应 | 业务判断、SQL | 父包 + `httpx` + `apperr` + chi |
| `httpx` | Server、Router、Recovery/RequestID/Logging/Timeout/CORS、错误→状态码映射 | 任何具体业务路由的 handler 实现 | `apperr` `telemetry` `config` |
| `postgres` | 连接池、TxManager、sqlc 生成物 | 业务语义 | pgx、`config`、`telemetry` |
| `redis` | 客户端生命周期 | 业务语义、缓存策略 | go-redis、`config` |
| `ratelimit` | 限流算法与 Redis 脚本 | HTTP 细节（中间件在 `httpx`） | `redis` `apperr` |
| `messaging` | `EventPublisher` 接口、NATS 实现、内存实现、outbox relay | 业务事件的处理逻辑 | `apperr` `postgres` nats |
| `telemetry` | logger/tracer/meter 的构造与关闭 | 业务指标定义（由各业务包注册） | OTel SDK、slog、`config` |

### 依赖方向

```text
                    ┌──────────────┐
                    │   cmd/api    │  ← 唯一知道"全部"的地方
                    │  cmd/worker  │
                    └──────┬───────┘
                           │ 组装
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
 ┌─────────────┐   ┌──────────────┐   ┌──────────────┐
 │  userhttp   │   │    userpg    │   │  messaging   │
 │  authhttp   │   │    authpg    │   │  ratelimit   │
 │  (transport)│   │  (adapters)  │   │  (adapters)  │
 └──────┬──────┘   └──────┬───────┘   └──────┬───────┘
        │  实现/调用       │  实现            │  实现
        └────────┬─────────┴──────────────────┘
                 ▼
        ┌────────────────────┐
        │   user  /  auth    │  ← 业务核心
        │  实体 + 规则 + 用例  │
        │  + 自己声明的接口    │
        └─────────┬──────────┘
                  ▼
        ┌────────────────────┐
        │ apperr / validate  │  ← 仅标准库
        └────────────────────┘

技术设施（httpx / postgres / redis / telemetry）被 adapters 使用，
业务核心永远不引用它们。箭头方向 = import 方向，无环。
```

### Interface 定义在消费者一侧

`user/service.go` 声明它**自己真正需要的**最小接口：

```go
// internal/user/service.go
type Repository interface {
    Create(ctx context.Context, u User) error
    GetByID(ctx context.Context, id ID) (User, error)
    GetByEmail(ctx context.Context, email Email) (User, error)
    List(ctx context.Context, p Page) ([]User, int, error)
    Update(ctx context.Context, u User) error
    Delete(ctx context.Context, id ID) error
}
```

`userpg.Repository` 实现它，但**不 import 该接口做断言**（Go 隐式满足）。
装配在 `cmd/api/main.go` 完成。

**明确禁止**：`internal/repository/interface.go` 里堆放所有实体的 Repository 接口。
那是把接口放在实现方一侧，会导致所有业务包依赖同一个巨型接口文件。

### 编译期强制

用 `golangci-lint` 的 `depguard` 规则约束（Phase 2 配置）：

```yaml
# .golangci.yml 片段（Phase 2 落地）
depguard:
  rules:
    business-core:
      files: ["**/internal/user/*.go", "**/internal/auth/*.go"]
      deny:
        - pkg: "github.com/jackc/pgx/v5"
        - pkg: "github.com/go-chi/chi/v5"
        - pkg: "github.com/redis/go-redis/v9"
        - pkg: "github.com/nats-io/nats.go"
        - pkg: "go.opentelemetry.io/otel"
        - pkg: "net/http"
```

注意 `files` 只匹配业务包根目录的 `*.go`，不递归到 `userpg/` `userhttp/`。
**这条 lint 规则是依赖方向的执行机制；文档只是它的解释。**

### 被否决的目录

| 目录 | 否决理由 |
|------|---------|
| `domain/` `application/` `infrastructure/` `transport/` | 按技术角色分层，导致一个功能改动要跨 4 个目录。Go 社区惯例是按功能分包 |
| `platform/` | 想把 `postgres/redis/messaging` 收进去。但 Go 的目录嵌套不产生任何可见性约束（`internal` 除外），多这一层只增加路径长度，不增加保护 |
| `pkg/` | 本项目不对外提供库。`internal/` 已经表达"不可被外部 import" |
| `common/` `utils/` | 无边界的包必然膨胀。有明确职责就有明确包名（如 `validate`） |
| `bootstrap/` | 装配逻辑属于 `cmd/`。单独一个包只是把 `main` 的内容搬走 |
| `models/` | 实体属于它所在的业务包 |

### 最大嵌套深度

`internal/user/userpg/repository.go` = 4 层，是全项目最深路径。可接受。

## Consequences

### 正面

- 依赖方向由 lint 强制，不靠自觉
- 一个功能的全部代码在一个子树内（`internal/user/**`），改动局部化
- 业务包的单元测试不需要 Docker，因为它编译时就不含 pgx

### 负面 / 代价

- 每个业务功能是 3 个包而不是 1 个，包数量比"一个 feature 一个包"多
- `userpg` / `userhttp` 这类包名有前缀冗余（为避免与 `net/http` 等混淆，不能直接叫 `http`）
- 小功能也要走三包结构，短期看有仪式感

**为什么仍然接受**：prompt 明确要求"业务核心不得依赖 HTTP framework / PostgreSQL"，
而单包方案（entity/service/repo/handler 放在同一个包）无法在编译期保证这一点——
只要同包内有一个文件 import 了 pgx，整个包就依赖 pgx，
`depguard` 也就无法区分。**要么放弃强制，要么接受三包。这里选择强制。**

### 后续影响

- Phase 2 必须落地 `.golangci.yml` 的 depguard 规则，否则约束不存在
- Phase 7 的 `goscaffold module <name>` 必须生成三包结构
- Phase 8 审计项"业务包不含基础设施 import"由 lint 自动完成

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| 单包 feature（`internal/user` 一个包放全部） | 包最少、导航最快 | 无法编译期隔离，业务测试要拖 pgx | 与硬性依赖规则冲突 |
| 经典四层（domain/application/infrastructure/transport） | 层次清晰、文献多 | 改一个功能跨 4 目录，且 Go 里没有对应的语言支撑 | 明确被 prompt 禁止，且实践中导致跳转疲劳 |
| 三包 + `platform/` 分组 | 业务与设施视觉分离 | 多一层路径，无实际约束力 | YAGNI |

## When to Revisit

- 业务包数量 > 15 且顶层 `internal/` 目录列表变得难以扫读 → 引入按限界上下文的一级分组
- 出现某个 feature 明显不需要 HTTP 或不需要持久化 → 该 feature 可以只有 1-2 个包，不必凑齐三个
- `depguard` 规则频繁被 `nolint` 绕过 → 说明分包方式与实际工作方式不符，重新评估
