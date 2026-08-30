# Project State

> 本文件是各 Phase 之间唯一的上下文载体。
> 每个 Phase 结束时必须更新；下一个 Phase 开始时必须先读它。
> 内容必须与实际代码一致，禁止写入未完成的事项。

## Current Phase

**Phase 3 (User / Auth / RBAC / Rate Limiting / OpenAPI) completed at 2026-08-30.**

服务现在有真实业务面：用户 CRUD、登录与令牌轮转、基于角色的授权、
四个 scope 的限流，以及一份与路由双向校验的 OpenAPI 文档。
**尚无**：tracing/metrics 实际接入（Phase 5）、消息与 outbox（Phase 7）、
容器化与 CI/CD（Phase 8）。

## Locked Decisions

| ADR | 主题 | 结论摘要 |
|-----|------|---------|
| [0001](docs/adr/0001-tech-stack.md) | 技术选型 | chi / pgx+sqlc / slog / Argon2id / UUIDv7 / NATS JetStream(默认关) / OTel(默认不导出)。明确拒绝 Viper、validator、GORM、zap、DI 框架。目标直接依赖 ≤ 20 |
| [0002](docs/adr/0002-project-layout.md) | 目录与依赖方向 | 扁平 `internal/`；每个业务功能三包：`user` / `user/userpg` / `user/userhttp`；依赖方向由 **golangci-lint depguard 编译期强制**。否决 domain/application/infrastructure、platform、pkg、common、utils、bootstrap |
| [0003](docs/adr/0003-error-model.md) | 错误模型 | `apperr.Error{Kind, Code, Message, cause}`；Kind 零值 = Internal（默认脱敏）；DB 错误码知识只在 `*pg` 包；Kind→HTTP 映射表只在一个文件；500 响应只回 request_id |
| [0004](docs/adr/0004-api-conventions.md) | API 约定 | **成功响应无 envelope**，错误响应有；**offset 分页**（default 20 / max 100，`ORDER BY created_at DESC, id DESC`）；**统一用 PATCH**；`DisallowUnknownFields`；OpenAPI 手写 + 路由一致性测试防 drift |
| [0005](docs/adr/0005-data-access.md) | 数据访问与事务 | pgxpool(MaxConns=10) + sqlc（生成物入库）；`TxManager.WithTx(ctx, fn)`，tx 藏在未导出的 context key 中，`pgx.Tx` 不出现在任何签名；**不支持嵌套事务**；**Pod 启动不自动迁移** |
| [0006](docs/adr/0006-id-strategy.md) | ID 策略 | **UUIDv7，应用侧生成**，PG 原生 `uuid` 类型；业务包定义具名类型 `user.ID`；时间戳由数据库填充，不用 UUID 内嵌时间 |
| [0007](docs/adr/0007-auth.md) | 认证授权 | JWT HS256 access **15min** + opaque refresh **30d**（存 SHA-256 哈希）；**轮转 + family 盗用检测**；Argon2id(64MB/t=3/p=4) + PHC 编码；登录三种失败返回同一 401；RBAC 集中在 `policy.go`，所有权检查在 Service |
| [0008](docs/adr/0008-rate-limiting.md) | 限流 | Redis Lua token bucket；4 个 scope（ip/user/login_ip/login_id）；**通用限流 fail-open，登录限流 fail-closed**；本地兜底限流器；限流中间件**拆成两个**（认证前按 IP / 认证后按 User）；`TRUSTED_PROXY_COUNT` 默认 0 |
| [0009](docs/adr/0009-messaging.md) | 消息与异步 | NATS JetStream，**默认 `MESSAGING_ENABLED=false`**；**Outbox 纳入 v1**；at-least-once + `processed_events` 幂等（与副作用同事务）；指数退避全抖动 5 次 → **DLQ 用数据库表**（便于 SQL 排查） |

未进入 ADR 的设计（模块边界 / 配置 / 缓存 / 可观测性 / 中间件顺序 / 测试策略 /
Bootstrap / 优雅退出 / 各流程图）在 [`docs/architecture.md`](docs/architecture.md)。

### 几条容易被后续阶段违反的硬约束

1. 业务包 `internal/user`、`internal/auth` 根目录的 `*.go` **不得 import**
   `pgx` / `chi` / `go-redis` / `nats.go` / `otel` / `net/http`。由 depguard 强制。
2. Kind → HTTP 状态码的映射**只允许存在于 `internal/httpx` 的一个文件中**。
3. `pgx.Tx` 不得出现在任何 Repository / Service 的方法签名里。
4. 中间件顺序中，**RateLimit 是两个中间件**，分列 Authentication 前后。
5. `terminationGracePeriodSeconds` 必须 > `SHUTDOWN_TIMEOUT`（Phase 6）。
6. Argon2id 与登录限流**必须在 Phase 3 同时落地**——只做前者会引入内存 DoS 入口。

## Implemented

### 文档

- `docs/adr/0001` ~ `docs/adr/0009`（9 个 ADR）
- `docs/architecture.md`
- `STATE.md`（本文件）

### 代码

| 包 | 内容 |
|----|------|
| `internal/apperr` | `Kind`/`Code`/`Message`/`cause` 错误模型；`Kind` 零值 = Internal；`Kind.String()` 越界返回 `"unknown"`（供穷尽性测试探测） |
| `internal/validate` | `Violations` 累加器 + `Required`/`Email`/`RuneLen`/`OneOf`/`IntRange`/`NormalizeEmail`。不含任何业务规则 |
| `internal/config` | `Secret` 类型（`String`/`GoString`/`LogValue`/`MarshalText` 全部返回 `[REDACTED]`）；`Load()` 环境变量优先 + godotenv 兜底；`Validate()` 一次性累积全部问题 |
| `internal/telemetry` | slog logger（dev 文本 / 其余 JSON），`contextHandler` 自动注入 `request_id`，Phase 5 在同一处加 `trace_id` |
| `internal/postgres` | `NewPool`（失败即关池）、`TxManager.WithTx`、`Querier(ctx, pool)`、`InTx`、`HealthCheck`（执行真实 SQL）、`db/`（sqlc 生成物）、`pgtest/`（testcontainers 夹具） |
| `internal/redis` | `NewClient`；连接失败一律返回 error，required/optional 的取舍留给调用方 |
| `internal/httpx` | `errmap.go`（唯一的 Kind→status 表）、`response.go`（`WriteJSON`/`WriteError`/`DecodeJSON`）、`middleware.go`、`server.go`（五个超时全部显式）、`health.go`、`router.go` |
| `cmd/api` | `main.go`（只决定退出码）+ `run.go`（手工 DI + 优雅退出）+ `main_test.go`（真实启停） |

### 工程配置

`go.mod`（7 个直接依赖，ADR-0001 上限 20）、`.gitignore`、`.golangci.yml`（v2 格式，含 depguard）、
`.env.example`、`Makefile`、`sqlc.yaml`、`migrations/000001_baseline`。

### 已生效的强制机制

| 机制 | 实现位置 | 违反时的后果 |
|------|---------|-------------|
| 依赖方向（ADR-0002） | `.golangci.yml` depguard 三组规则 | `make lint` 失败 |
| Kind→status 映射唯一且穷尽 | `internal/httpx/errmap_test.go` | 新增 Kind 未登记 → 测试失败 |
| `.env.example` 与 loader 不漂移 | `internal/config/env_example_test.go` | 键名不一致 → 测试失败 |
| 示例密钥不得进 production | `Validate()` + `env_example_test.go` 交叉校验 | 启动失败 / 测试失败 |
| sqlc 生成物不过期 | `make sqlc-check`（`sqlc diff`） | CI 失败 |
| 迁移与代码 schema 一致 | `pgtest` 对真实库跑真实迁移 | 集成测试失败 |

### Phase 3 代码

| 包 | 内容 |
|----|------|
| `internal/user` | `ID`（具名 UUIDv7）、`User`、`Status`、`Patch`（全指针 + `ChangesPrivileges`）、`Page`；`Repository`/`PasswordHasher` 窄接口；`Service`（Create/Get/List/Update/ChangePassword/Delete，授权全部走 `auth.Authorize*`）；`AccountStore` 适配器 |
| `internal/user/userpg` | pgx 实现。**全项目唯一翻译 SQL 错误码的地方**：23505→Conflict、23514→InvalidArgument、23503→Conflict、`pgx.ErrNoRows`→NotFound |
| `internal/user/userhttp` | `userResponse`（结构上没有 PasswordHash 的落点）、请求 DTO、6 条路由、`intQuery` 严格解析 |
| `internal/auth` | `policy.go`（Role/Permission/`Can`/`Authorize`/`AuthorizeSelfOrPermission`）、`principal.go`、`password.go`（Argon2id + PHC + `VerifyDummy`）、`token.go`、`account.go`（窄口 `Account`/`AccountStore`）、`service.go`（Login/Refresh/Logout） |
| `internal/auth/authjwt` | HS256 签发与校验；`WithValidMethods`/`WithIssuer`/`WithAudience`/`WithExpirationRequired`；密钥下限 32 字节；所有失败归一到 `ErrInvalidToken` |
| `internal/auth/authpg` | refresh session 仓储；`GetByTokenHash` 带 `FOR UPDATE` |
| `internal/auth/authhttp` | `Authenticate` / `RequirePermission` 中间件、login/refresh/logout handler（`Cache-Control: no-store`） |
| `internal/ratelimit` | `Scope`/`Rule`/`Decision`/`Limiter`；Redis Lua token bucket（时间由应用传入）、`LocalLimiter`（带条目上限）、`Guard`（fail-open/closed 分流 + 降级 + 日志抽样） |
| `internal/ratelimit/ratelimithttp` | `ByIP` / `ByUser` / `Login` 三个中间件；登录路径不暴露 `X-RateLimit-*` |
| `internal/httpx` | 新增 `clientip.go`：XFF 从右往左数 `TRUSTED_PROXY_COUNT` 跳，默认 0 完全忽略该头 |
| `cmd/api` | 新增 `features.go`（组合根 + `registerRoutes`）与 `openapi_test.go`（路由/文档双向一致性） |
| `api/openapi.yaml` | 12 个 operation（9 个 API + 3 个探针），Redocly 校验通过 |

### 相对 Phase 1 设计的补充决定

1. **`run(ctx)` 放在 `cmd/api` 的 `run.go`（package main）**，由 `main_test.go` 驱动。
   既满足"shutdown 可测试"，又不违反 ADR-0002 对 `bootstrap/` 包的否决。
2. **`main()` 只做 `os.Exit(start())`**。`os.Exit` 会跳过 defer，把二者写在同一函数里
   会静默丢掉清理逻辑。
3. **503 / 504 / 405 不进 `Kind` 集合**。它们描述传输层状态（依赖不可用 / 超时 /
   路由表），不是业务错误分类，由 health handler、Timeout 中间件、
   `MethodNotAllowed` 直接写出。
4. **`Timeout` 自己实现而非用 `http.TimeoutHandler`**。后者只掐响应写入，
   不取消 handler 的 context，超时后数据库查询仍在跑——那正是要释放的资源。
5. **`RequestID` 校验入站头必须是 UUID**。该头由客户端控制，未校验就写进每行日志
   等于开放日志注入。
6. **新增 `LOG_LEVEL` 配置**。事故中提高日志级别不应该需要重新构建。
7. **`corsPreflightMaxAge` 是常量不是配置**。允许的方法与头在编译期固定，
   运维没有可调的东西。

### Phase 3 的补充决定

1. **`Role` 归 `auth` 所有，`user.User.Role` 的类型就是 `auth.Role`**。
   依赖方向是 `user → auth`，`auth` 永不 import `user`；`auth` 自己定义
   `Account`/`AccountStore` 窄接口，适配器放在 `user` 一侧。
   两处各定义一份 Role 会立刻产生两张需要手工同步的权限表。
2. **503 不进 `apperr.Kind`**（延续 Phase 2 的第 3 条）。限流后端不可用用
   sentinel `ratelimit.ErrBackendUnavailable`，中间件直接写 503。
3. **`users` 表带 `role` 列并有 CHECK 约束**。角色是授权判据，
   让它只存在于应用内存里意味着数据库可以存下应用拒绝的值。
4. **登录限流必须读 body**，因此 `Login` 中间件以 4 KiB 上限读取后再把 body 还原。
   不设这个上限，限流器本身就会变成让服务端缓冲大 body 的入口。
5. **refresh 轮转全程在一个事务内**，且 `GetByTokenHash` 取行锁。
   没有锁，两个并发 refresh 会双双成功，重放检测形同虚设。
6. **`registerRoutes` 拆成不依赖数据库的独立函数**，
   使路由表能在单元测试里被遍历并与 OpenAPI 对照。
   只能通过完整启动才能拿到的路由表，等于没有被任何东西校验。

## Not Implemented Yet

- OpenTelemetry 实际接入与指标（Phase 5，中间件栈中已留注释挂载点）
- 消息队列、outbox 与 `processed_events`（Phase 7）
- Dockerfile、docker-compose、GitHub Actions、Kubernetes manifests（Phase 6/8）
- `goscaffold` CLI（Phase 8）
- 用户注册（自助 sign-up）：当前建用户需要 `user:create` 权限，
  没有公开注册端点；这是范围问题，不是遗漏

## Known Limitations

设计层面已知的妥协（均已在对应 ADR 中论证）：

| 妥协 | 出处 |
|------|------|
| access token 撤销/改角色有最长 15 分钟延迟 | ADR-0007 |
| 通用限流 fail-open：Redis 故障期间防护真空（有本地兜底器缓解） | ADR-0008 |
| tx 通过 context 隐式传递，与"Explicit > Magic"有张力 | ADR-0005 |
| offset 分页在并发插入下可能重复/跳过；深翻页在十万行后劣化 | ADR-0004 |
| 外部副作用（发邮件）无法 exactly-once，只能依赖外部系统幂等 | ADR-0009 |
| 每个业务功能三个包，包数量多于单包方案 | ADR-0002 |
| UUIDv7 暴露资源创建时间（毫秒级） | ADR-0006 |
| 注册接口的 409 会泄漏"该邮箱已注册" | ADR-0007 |
| Redis 成为 v1 硬依赖（因限流），单机部署也需运行 | ADR-0001 / 0008 |
| outbox 引入至少一个轮询周期（默认 1s）的消息延迟 | ADR-0009 |

Phase 2 引入的新妥协：

| 妥协 | 理由 |
|------|------|
| `Timeout` 中间件在 handler 已开始写响应后无法改写状态码，只能截断 | 无法回收已发出的字节；日志里能看到真实原因 |
| `queries/system.sql` 只有一条 `ReadinessProbe`，仅为验证 sqlc 工具链 | Phase 3 起才有真实业务查询 |
| `migrations/000001_baseline` 只建了 `set_updated_at()` 触发器函数 | Phase 2 无业务表；该函数是后续每张表都要用的基础设施 |
| 集成测试需要 Docker，默认 `go test ./...` 不跑 | 用 `-tags=integration` 显式开启，见 `make test-integration` |
| `internal/postgres/db` 的生成代码带 `emit_interface`，当前无人实现该接口 | Phase 3 的 repository 会用它做测试替身 |

Phase 3 引入的新妥协：

| 妥协 | 理由 |
|------|------|
| 没有 seed 的第一个管理员账号；首个 admin 需人工插库或后续 Phase 提供 CLI | 引导路径未在本阶段定义，硬编码一个默认 admin 是常见的后门来源 |
| `Guard` 降级到本地限流器时，配额按副本数放大（`fallbackFactor=2`） | 每副本独立计数无法得到全局配额；放大系数是"别把配额收得比预期紧"的取舍 |
| refresh 家族撤销会把合法客户端一并登出 | 无法区分"被盗用"与"客户端重试"，从安全侧取严 |
| `POST /users` 的 409 会泄漏"该邮箱已注册" | 沿用 ADR-0007 的判断：该端点需要 `user:create`，调用方已是可信管理员 |
| OpenAPI 由手写维护，只有路径/方法/路径参数被自动校验，schema 与实现之间仍可能漂移 | 全量 schema 校验需要引入运行时校验中间件，成本与收益不匹配 |
| `make openapi-lint` 需要 npx，未纳入 `make check` | `check` 必须在离线环境可跑 |

工程层面：

- Phase 2 与 Phase 3 所有门禁命令均**实际执行并通过**，无 SKIPPED 项
- 集成测试在真实 PostgreSQL 17 容器上运行（testcontainers + OrbStack）
- `npx @redocly/cli lint api/openapi.yaml`：valid，4 条 warning
  （servers 用 localhost、三个探针没有 4xx 响应）——均为有意为之

## Next Phase Entry Conditions

Phase 4 (Testing & Quality) 开始前已成立：

- [x] `make check` 全绿（fmt / vet / lint / test -race / vuln / sqlc-check），
      `golangci-lint run` 0 issues
- [x] `make test-integration` 全绿（真实 PostgreSQL 17）
- [x] 9 个业务端点全部实现，且与 `api/openapi.yaml` 双向一致（有测试强制）
- [x] 登录三种失败路径不可区分；refresh 重放会撤销整个 family（有测试）
- [x] 四个限流 scope 落地，通用 fail-open / 登录 fail-closed（有测试）
- [x] Argon2id 与登录限流同阶段落地（ADR-0007 / 0008 的硬约束）

Phase 4 必须遵守且最容易被违反的点：

1. **新增或修改端点必须同步 `api/openapi.yaml`**，否则
   `cmd/api/openapi_test.go` 会双向失败。这条现在是自动强制的，不靠自觉。
2. 数据库错误码的翻译**只能出现在 `*pg` 包**。Phase 4 补测试时若发现
   Service 层需要判断具体错误，说明翻译漏了一层，改仓储而不是改 Service。
3. 覆盖率不是目标，**没有被断言的行为才是**。Phase 4 的重点是补
   `internal/auth/authpg` 与 `internal/user/userpg` 之外仍未被覆盖的分支，
   以及为 `Guard` 的降级路径补并发测试。
4. 任何"为了测试"而放宽的生产代码（导出内部符号、加 test-only 分支）
   都必须先在 ADR 或本文件里说明理由。

## 与原始阶段划分的偏差

原 Prompt 把限流排在 Phase 6、OpenAPI 排在 Phase 8。重构后的 Prompt
（`prompts/03-user-auth.md`）把两者都并入 Phase 3，理由已写在 ADR-0007 / 0008：
Argon2id 单独上线会引入内存 DoS 入口，而 OpenAPI 越晚写越接近于事后补文档。
本文件的 Phase 2 章节仍保留当时的措辞，作为决策变更的记录。
