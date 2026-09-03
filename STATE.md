# Project State

> 本文件是各 Phase 之间唯一的上下文载体。
> 每个 Phase 结束时必须更新；下一个 Phase 开始时必须先读它。
> 内容必须与实际代码一致，禁止写入未完成的事项。

## Current Phase

**Phase 5 (Observability) completed at 2026-09-01.**

服务现在能被观测：结构化日志带 `trace_id`/`span_id`，HTTP / PostgreSQL /
Redis / NATS 全部产生 span 且跨进程连成一条 trace，Prometheus 指标在独立
内网端口导出，Grafana 仪表盘由测试守护不会静默失效。
默认 `OTEL_ENABLED=false`——关掉时业务行为完全不变（有实跑验证）。
**尚无**：容器化与 CI/CD（Phase 6/8）、告警规则、SLO 定义。

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
- 真实的邮件发送：`notify.LogSender` 只把邮件写进日志。整条链路
  （outbox、投递、幂等、重试、DLQ）都是真的并有测试，缺的只是一个
  provider 实现——**不要在未接 provider 的情况下当作"能发信"部署**
- Dockerfile、GitHub Actions、Kubernetes manifests（Phase 6/8）；
  `docker-compose.yml` 已在本阶段落地，理由见"与原始阶段划分的偏差"
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
| ~~没有 seed 的第一个管理员账号~~ **已解决**，见 `cmd/adminctl` | 见下文「引导与 schema 漂移」一节 |
| `Guard` 降级到本地限流器时，配额按副本数放大（`fallbackFactor=2`） | 每副本独立计数无法得到全局配额；放大系数是"别把配额收得比预期紧"的取舍 |
| refresh 家族撤销会把合法客户端一并登出 | 无法区分"被盗用"与"客户端重试"，从安全侧取严 |
| `POST /users` 的 409 会泄漏"该邮箱已注册" | 沿用 ADR-0007 的判断：该端点需要 `user:create`，调用方已是可信管理员 |
| OpenAPI 的**请求体** schema 仍无自动校验（响应体已有，见下文） | 请求体的权威定义在 `validate` 规则里，比对需要另一套映射；响应体是客户端实际消费的一侧，先覆盖它 |
| `make openapi-lint` 需要 npx，未纳入 `make check` | `check` 必须在离线环境可跑 |

工程层面：

- Phase 2 与 Phase 3 所有门禁命令均**实际执行并通过**，无 SKIPPED 项
- 集成测试在真实 PostgreSQL 17 容器上运行（testcontainers + OrbStack）
- `npx @redocly/cli lint api/openapi.yaml`：valid，4 条 warning
  （servers 用 localhost、三个探针没有 4xx 响应）——均为有意为之

## Next Phase Entry Conditions

Phase 4 (Messaging) 开始前已成立：

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

## 手动验证发现并修复的问题（2026-08-31）

把服务真正跑起来（PostgreSQL 17 + Redis 7 + curl 走完整流程）后暴露了两个问题，
都已修复并补了回归测试：

1. **refresh 重放检测形同虚设**。家族撤销写在事务里，紧接着
   `return ErrInvalidToken()` 让整个事务回滚，把撤销一并抹掉——
   被盗的那条链继续有效。修法是把"这个 token 不可用"与"这次操作失败"分开：
   闭包里只置 `rejected` 标志并返回 `nil` 让撤销提交，事务提交之后才生成错误。
   同一条 bug 也影响"账号被禁用"路径。

   **根因是测试假件而不是代码**：`service_test.go` 的 TxManager 只是调用 fn，
   不模拟回滚，所以断言全绿。已改成快照/还原的 `rollbackTx`，
   并用变异验证确认它现在能抓到这个 bug；另加
   `internal/auth/authpg/service_rotation_integration_test.go`
   用真实事务把这条路径钉死。

2. **登录响应带着 `X-RateLimit-*`**。它们来自外层通用 IP 限流器（limit 100），
   不是登录计数器，所以没泄漏剩余尝试次数；但一个 429 旁边写着
   "Remaining: 199" 是自相矛盾的。登录中间件现在主动清掉上游设置的这三个头。

## 对抗性黑盒测试发现并修复的问题（2026-08-31）

把 API 当作陌生人来打，共 110 条断言（`test/blackbox/`），发现 5 个真实缺陷，
全部已修并带回归测试：

1. **姓名里的 U+0000 造成 500**。校验层没有控制字符规则，NUL 一路进到
   PostgreSQL，SQLSTATE 22021 未被翻译成领域错误。新增 `validate.NoControlChars`
   （拒 C0/C1、U+2028/2029、bidi 覆写、U+200B、U+FEFF），接在 Create/Update 上。
   **放行 ZWJ/ZWNJ**：波斯语和天城文的姓名要用它们，一刀切会拒掉合法用户。

2. **防溢出的守卫自己溢出了**。`List` 用 `int64(page)*int64(pageSize) > 1<<30`
   拦越界，`page=9223372036854775807` 时乘积回绕成 -20，守卫放行，负 OFFSET
   进了 PG。改成除法 `page > maxOffset/pageSize`，不产生中间值就不会回绕。
   **同样是假件掩盖了它**：`fakeRepository.List` 默默接受负 offset，
   已改成像 PG 一样报错，现在没有 Docker 也能复现。
   顺带修了越界页返回 `total_pages: 0` 与在范围内不一致的问题。

3. **`Content-Type: text/plain` 的请求被当作 JSON 接受**。`DecodeJSON`
   从不检查 Content-Type。新增 `requireJSONContentType`（接受 `application/json`
   与 `+json` 后缀）。ADR-0003 锁死了 7 个 Kind，所以沿用 503 的先例：
   `KindInvalidArgument` + 独立 code `UNSUPPORTED_MEDIA_TYPE`。

4. **405 不带 `Allow` 头**（RFC 9110 要求）。根因不在我们的代码：chi 在路由时
   算出允许的方法集，存在非导出字段里，只传给它自己的 responder——为了返回
   JSON 体而替换 responder，就把这个头一起丢了。三种从外部重建列表的办法都更糟
   （子路由挂载点对任何方法都 match、`RoutePattern` 给的是挂载点而非叶子、
   遍历路由树等于重新实现 chi 的模式匹配）。最终**不替换** responder，
   改用一个中间件补上 chi 留空的响应体，头仍由 chi 自己写。

5. **`X-RateLimit-Remaining` 大于 `X-RateLimit-Limit`**（999 > 600）。
   两个数在数不同的东西：`Limit` 报的是持续配额，`Remaining` 报的是令牌桶深度，
   而通用规则是 quota 600/min、burst 1000。客户端按 `limit - remaining`
   算用量会得到负数。改 `Limit` 为桶深——"此刻还能发多少"才是 `Remaining`
   递减的那个量；burst == quota 的登录规则不受影响。

**方法论教训（比上面任何一条都重要）**：第一轮 18 个 JWT 测试全部"通过"却毫无价值——
我用的变量名是 `JWT_SECRET`，真名是 `AUTH_JWT_SECRET`，所有 401 都只是签名不对。
**必须先证明基线能被接受，"改了某个 claim 后被拒绝"才归功于那个 claim。**
同理，客户端没能发出去的请求不等于服务端拒绝了它：未编码的空格和 2MB 体的
提前响应都伪装成了服务端缺陷。

**观察到但判断不改的**：HEAD 打到 GET 路由返回 405（chi 不自动支持，属路由策略）；
重复 JSON 键取最后一个（`encoding/json` 的标准行为，也是 JSON 规范允许的）；
跨域无 CORS 头（未配置允许的 origin，属预期）。


## 与原始阶段划分的偏差

原 Prompt 把限流排在 Phase 6、OpenAPI 排在 Phase 8。重构后的 Prompt
（`prompts/03-user-auth.md`）把两者都并入 Phase 3，理由已写在 ADR-0007 / 0008：
Argon2id 单独上线会引入内存 DoS 入口，而 OpenAPI 越晚写越接近于事后补文档。
本文件的 Phase 2 章节仍保留当时的措辞，作为决策变更的记录。

## 引导与 schema 漂移（2026-08-31）

Known Limitations 里的条目分两类。**ADR 论证过的取舍不动**——family 撤销误伤
合法客户端、`POST /users` 的 409 泄漏、Guard 降级配额放大、通用限流 fail-open、
`make openapi-lint` 不进 `make check`——改掉它们等于推翻已记录的判断，
而不是修复缺陷。剩下两条是真缺口，本次补上：

### 1. 首个管理员的引导路径（`cmd/adminctl`）

在此之前，让系统可用需要人工算一个 Argon2id 哈希再写裸 SQL。这不只是麻烦：
哈希敲错了得到一个谁也登不进去的账号，哈希是从别处粘来的则得到一个
密码在别处已知的账号。

命令做三件事，每一件都是有意的：

- **拒绝在已有管理员时运行**。一个能在 API 之外铸造特权账号的命令，
  就是一条常设的旁路，同时绕开授权和审计。第一个之后必须走
  `POST /api/v1/users`。
- **默认生成密码而不是接受参数**。命令行上敲的密码会进 shell 历史和进程列表；
  生成的密码只显示一次。需要指定时用 `-password-stdin`。
- **走 `user.Service.Create` 而不是 INSERT**。校验、邮箱归一化、哈希参数
  全部复用生产路径。自己写行的引导路径就是会和真实路径漂移的引导路径——
  测试里 `Admin@Example.com ` 被规范成 `admin@example.com` 正是在钉这一点。

授权那一关用合成 principal 过（`bootstrapContext`）。这不是把提权伪装成管道：
能跑这个命令的人已经握有数据库凭据，那严格强于 `user:create`。选择走正常
授权路径而不是给 Service 加一个「跳过检查」开关，是为了让这个例外留在
命令里可见，而不是嵌进 Service 里对所有调用方生效。

用法：`make bootstrap-admin email=you@example.com [name="Your Name"]`

### 2. 响应体 schema 漂移（`internal/openapitest`）

`cmd/api/openapi_test.go` 只保证路径与方法两边一致，对响应体一无所知——
而这正是文档悄悄腐烂的地方：DTO 里改个字段名，所有 Go 测试照样通过
（它们断言的是结构体），文档却继续向每个生成的客户端承诺旧名字。

`internal/openapitest` 拿**处理器真实写出的字节**和文档比对，双向：

- 响应里出现的字段必须有文档 → 抓「加了字段忘了写文档」
- 文档标 required 的字段必须出现在响应里 → 抓「字段改名/删了文档还留着」

文档里的可选字段可以缺席，所以一个不带 `details` 的错误响应不算失败。
`$ref` 会递归解析，因此 `UserPage` 的检查连带覆盖嵌套的 `User`——
列表客户端真正读到的那一份。

已做变异验证确认两个方向都真的会失败：给 `User.required` 加一个
`last_login_at`，和把 DTO 的 `created_at` 改成 `createdAt`，都被抓到。

覆盖 `User` / `UserPage` / `TokenPair` / `Error`（含 `details` 分支）。
`TokenPair` 的检查放在 `internal/auth/authpg` 的集成测试里而不是 `authhttp`：
处理器接的是具体的 `*auth.Service`，由别的东西造出来的 token pair，
其形状是测试作者选的，不是程序的。

### 运行验证

PostgreSQL 17 + Redis 7 + 3 条迁移 + `make bootstrap-admin`，`/ready` 报告
两个组件均 ok。实测：登录 200、创建用户 201 且邮箱被归一化、
member 列用户 403、refresh 轮转后旧 token 401 **且新 token 一并失效**
（上面那个 bug 在真实系统里保持修复状态）、登录失败第 5 次起 429。

---

# Phase 4 — Messaging / 异步处理（2026-09-01）

## Messaging Architecture

链路只有一条，每一段都可以单独替换：

```
HTTP 请求
  └─ user.Service.Create  ── 一个事务 ──┬─ INSERT users
                                        └─ INSERT outbox_events
                                            （事务提交 = 事件已持久）
                        ↓
  cmd/worker: Relay      ── 每 1s / 被 Notify 唤醒
     ClaimBatch (FOR UPDATE SKIP LOCKED)
       → natsmq.Publisher.Publish
       → MarkPublished          （publish 与 mark 在同一个事务内）
                        ↓
              NATS JetStream (stream EVENTS)
                        ↓
  cmd/worker: Subscription → Consumer.Process
     一个事务：ClaimEvent(processed_events) → handler → commit
     失败 → 指数退避重试 → 次数耗尽 → dead_letter_events
```

分层与依赖方向：

| 包 | 职责 | 允许依赖 |
|---|---|---|
| `internal/messaging` | `Event` 信封、`Publisher`/`Handler` 接口、Relay、Consumer | 无基础设施依赖 |
| `internal/messaging/messagingpg` | outbox 存储 + 去重存储（PostgreSQL） | `internal/postgres` |
| `internal/messaging/natsmq` | JetStream 传输 | `nats.go`（全仓唯一允许处） |
| `internal/notify` | 第一个消费者：欢迎邮件 | `messaging`、`user` |
| `cmd/worker` | 组合根：relay + subscription 的生命周期 | 全部 |

`.golangci.yml` 从 Phase 1 起就 deny 业务包 import `nats.go`。
业务代码只见 `messaging.Publisher`，换 Kafka 是改 `natsmq` + main，
不动任何 service。

## Event Contracts

事件类型格式 **`{aggregate}.{action}.v{n}`**，`messaging.ValidateType` 强制，
不合规直接返回错误。版本写在类型里而不是单独字段：消费者订阅的是契约，
不是"名字 + 一个需要自己记得检查的数字"。

已定义的事件：

| 类型 | 触发 | payload |
|---|---|---|
| `user.created.v1` | `POST /api/v1/users` 成功 | `user_id`、`email`、`name`、`role` |

`user.CreatedPayload` **不含密码、不含哈希、不含令牌**。事件会被复制进
broker、进每个消费者的存储、进任何记录它的日志——放进去的秘密在有人 review
之前就已经泄漏了好几遍。`TestTheCreatedEventCarriesNoCredentials` 对**编码后的
字节**断言而不是对解码后的结构体断言，因为解码会静默丢掉多出来的字段，
而多出来的字段正是这个测试要抓的东西。

信封字段：`ID`（UUIDv7，幂等键）、`Type`、`OccurredAt`（事实发生时间，
不是投递时间）、`TraceParent`、`Payload`。

## Delivery Guarantees

**at-least-once。不是 exactly-once。**

消费者必须自己幂等，`processed_events` 提供了一个通用实现，但**副作用发生在
外部系统时（发信、扣款）去重表救不了你**——它只能保证同一个事务里的数据库
写入不重复。

三个地方各自可能产生一次重复，全部是有意的取舍：

1. **relay 发布成功但 mark 失败** → 事务回滚，下一轮重发。选这个而不是
   "先释放锁再发布"，因为后者会让两个 relay 并发发同一条。
2. **JetStream ack 丢失** → 重投。
3. **worker 在 handler 提交后、ack 前被杀** → 重投。

去重表吸收这三种情况。代价是每条消息最多一次重复，收益是不会丢。

**顺序**：不保证。`ClaimOutboxEvents` 按 `created_at` 排序只是"大致有序"——
两个事务的提交顺序可以和插入顺序不同。任何依赖顺序的消费者都会出错。

## Idempotency Strategy

`processed_events` 主键是 **`(event_id, consumer)`**，不是 `event_id` 单列。
单列会让"给已有事件类型新增一个消费者"静默什么都不做：没有报错、没有日志，
只是一个永远不会运行的功能。`TestADifferentConsumerStillProcessesTheEvent`
锁住这一点。

**去重记录与 handler 的副作用必须在同一个事务里。**
`Consumer.attempt` 在 `store.WithTx` 里先 `ClaimEvent` 再跑 handler。
`messagingpg.Store.ClaimEvent` 会检查 `postgres.InTx(ctx)`，不在事务里**直接
报错**——脱离事务时它保证会静默失效，而静默失效的幂等等于没有幂等。

## Retry & DLQ Design

消费者侧（`internal/messaging/consumer.go`）：

| 参数 | 默认 | 说明 |
|---|---|---|
| `MaxAttempts` | 5 | 含首次 |
| `BaseBackoff` / `MaxBackoff` | 1s / 30s | 指数 + **full jitter** |
| `HandlerTimeout` | 30s | 每次尝试独立超时 |

- `messaging.Permanent(err)` 标记的错误**不重试**，直接进 DLQ。
  解不开的 JSON 不会在第五次变得能解开。
- handler panic 会被 recover 并当作可重试失败——一个 goroutine 的 panic
  不该带走整个 worker。
- 重试预算（5 次 × 30s + 1+2+4+8s ≈ 2m45s）必须小于 `MESSAGING_ACK_WAIT`
  （5m），否则 JetStream 会在消费者还在重试时并发重投同一条。
  **`internal/config/load.go` 把这条不变量做成了启动校验**，不是注释。
- 同理 `PROCESSED_RETENTION`（30d）必须大于 `STREAM_MAX_AGE`（7d），
  否则去重记录先过期，一次迟到的重投会被当成新消息处理两次。同样有启动校验。

DLQ 运维（ADR-0009 说 Phase 4 只给 SQL + make 目标，CLI 留给 Phase 7）：

```sh
make outbox-status   # 积压 / 已发布 / 正在失败 / 最差重试次数
make dlq-list        # 待处理的死信，含失败原因
make dlq-replay id=<dead_letter_id>   # 放回 outbox 重新投递
```

replay **保留原 event ID**，这样任何已经处理过它的消费者会认出并跳过。
换新 ID 会让全系统的去重记录一起失效。

## 本阶段做的超范围的事

一件，明确记录：**`docker-compose.yml`**。
`prompts/04-messaging.md` §4.11 要求把 broker 放进 compose 的独立 profile，
但仓库里根本没有 compose 文件，STATE.md 又把 compose 归给 Phase 6。
处理方式是建一个最小的（pg + redis + nats），**nats 在 `--profile messaging`
之下**——只做 messaging 需要的部分，Dockerfile / CI / K8s 仍然留给 Phase 6/8。

## Known Limitations（Phase 4 新增）

| 妥协 | 说明 |
|---|---|
| `notify.LogSender` 只写日志 | 占位实现，不是"假装能用"。链路是真的，缺 provider。已登记进 Not Implemented Yet |
| outbox `payload` 是 `jsonb` | PostgreSQL 会规范化空白与键序，**原始字节不被保留**。消费者解析 JSON 所以无影响，但任何对原始 payload 签名/哈希的做法都会失效 |
| 事件顺序无保证 | 见 Delivery Guarantees |
| 单 stream / 单 consumer | 所有事件一个 stream，worker 一个 durable consumer。分流留给有第二个消费者时 |
| relay 在持锁期间发布 | 换来"最多重复一次"，代价是一条慢消息会占着它那批的锁直到 `PublishTimeout` |
| `traceparent` 目前只是转发请求头 | `httpx.TraceContext` 转发调用方的头，非法值丢弃。Phase 5 接入 OTel 后会改为从活跃 span 取 |

## 测试与变异验证

新增测试全部做了变异验证（改坏实现，确认测试会失败）。**捕获率 15/15**，
其中两次暴露了"测试通过的原因是错的"：

1. **`r.Use(TraceContext)` 从路由里删掉，所有中间件测试照样通过。**
   中间件本身测得很好，但没有任何东西验证它被挂上了。一个写对了却没挂载的
   中间件，和没写完全一样。补 `TestTheRouterPutsTheRequestTraceOnTheContext`
   后变异被抓。
2. **把 `Durable:` 改成 `Name:`（消费者不再持久），"重新订阅保持位置"的测试
   照样通过。** 因为同一进程内重新订阅会复用已存在的 consumer，根本没模拟
   重启。改为直接向 broker 查询 `info.Config.Durable` 后变异被抓——
   顺带发现非持久 consumer 的失活阈值只有 5s。

覆盖的六类（§4.10）：

| 类别 | 位置 |
|---|---|
| 信封 / 类型校验 | `internal/messaging/messaging_test.go` |
| Relay（批量、毒消息隔离、突发排空、优雅退出、保留期） | `internal/messaging/relay_test.go` |
| Consumer（幂等、重试、panic、DLQ、取消） | `internal/messaging/consumer_test.go` |
| trace 传播 | `internal/messaging/trace_test.go`、`internal/httpx/*_test.go` |
| 事件契约（含"payload 不含凭据"） | `internal/user/events_test.go` |
| 消费者实现 | `internal/notify/notify_test.go` |
| PostgreSQL 真实语义（SKIP LOCKED、事务内去重、DLQ replay） | `internal/messaging/messagingpg/*_integration_test.go` |
| JetStream 真实语义（往返、重投、毒消息、持久性） | `internal/messaging/natsmq/*_integration_test.go` |

natsmq 集成测试在没有 broker 时 **SKIP 而不是 FAIL**——只跑默认 compose
profile 的人不该看到红色，否则大家会学会忽略它。

## 运行验证（2026-09-01）

真实跑通，不是"应该能跑"：

- `POST /api/v1/users` 带 `traceparent` 头 → outbox 行的 `traceparent`
  列就是那个值 → NATS → `notify` 处理，全程约 420ms
- 停 worker，建 25 个用户 → 25 条积压，**API 不受影响**（201 照常）→
  重启 worker → 0 积压、52 published、52 handled、52 processed、0 DLQ
- 手工插一条死信 → `make dlq-list` 看到 → `make dlq-replay` → worker 处理掉
- `MESSAGING_ENABLED=false`：API 健康、建用户 201、**outbox 不增长**；
  worker 拒绝启动并说明原因（不是空转）
- SIGTERM：`nats subscription drained` → `outbox relay stopped` →
  `worker stopped cleanly`
- 黑盒回归 **110/110 通过**，确认 Phase 3 行为未被破坏
  （`POST /users` 现在多了一层事务）

## 本阶段修掉的既有缺陷

1. **`.golangci.yml` 的 depguard 规则自相矛盾**：deny `math/rand` 会前缀匹配
   到 `math/rand/v2`——正是它自己的提示信息让你用的那个包。加了定向 exclusion。
2. **黑盒套件不可移植**：`sys.path` 硬编码 `/tmp/apitest`，`t_http.py` 硬编码
   了管理员密码（我轮转密码后它就以 401 失败，看起来像授权 bug 而不是过期
   fixture）。改为从环境变量读，并补上 README 里早就写了、代码里却不存在的
   bootstrap 入口（`python3 lib.py`）。`tok.json` 加进 `.gitignore`——
   那是一个运行中服务的真实凭据。
3. **`user.UserCreatedPayload` 命名 stutter**（revive），改为 `user.CreatedPayload`。

## Next Phase Entry Conditions

Phase 5 (Observability) 开始前已成立：

- [x] `make check` 全绿，`golangci-lint run` 0 issues
- [x] `make test-integration` 全绿（真实 PostgreSQL 17 + 真实 NATS JetStream）
- [x] outbox 与调用方事务同生共死（有集成测试）
- [x] `FOR UPDATE SKIP LOCKED` 让多 relay 不重发（有集成测试）
- [x] 去重键是 `(event_id, consumer)`，去重与副作用同事务（有集成测试）
- [x] 重试 / DLQ / replay 闭环，且预算不变量有启动校验
- [x] 默认 `MESSAGING_ENABLED=false`，关掉时 API 完整可用（已实跑验证）
- [x] 新增测试 15/15 通过变异验证

Phase 5 必须注意：

1. **`httpx.TraceContext` 要改为从活跃 span 取 traceparent**，
   目前只是转发调用方的头。`messaging.ContextWithTraceParent` 的校验要保留——
   伪造的 traceparent 比没有更糟。
2. **`Event.TraceParent` 已经端到端贯通**（HTTP 头 → outbox 列 → NATS header
   → 消费者），Phase 5 只需要在消费侧用它 **link** 到原始 trace，
   不要新建一条无关的 trace。
3. 中间件栈里 `// Phase 5: Metrics` 的挂载点在限流**之前**，
   这样 429 会被计数——挪到之后就看不见被拒绝的流量了。

---

# Phase 5 — Observability（2026-09-01）

## Telemetry Architecture

```
业务包（user / auth / ratelimit / messaging / postgres / redis / httpx）
        │  只依赖自己声明的窄接口（Observer / PoolStats / TracerProvider）
        ▼
internal/telemetry        ← 唯一 import OTel SDK 与 Prometheus 的包
        │
        ├── tracing.go     TracerProvider / Resource / OTLP exporter / Noop
        ├── propagation.go traceparent ⇄ context（W3C，单头 carrier）
        ├── metrics.go     实例化 registry + 15 组指标
        ├── endpoint.go    /metrics 独立监听
        └── logger.go      contextHandler 注入 trace_id / span_id
        ▲
        │  组合根（cmd/api、cmd/worker）是唯一把两者接起来的地方
```

**分层的理由**：ADR-0001 要求业务包不感知具体基础设施。业务包 import
Prometheus 的那一刻，单元测试就需要一个 registry，而 registry 重复注册会
panic，测试于是开始互相干扰。改为业务包声明 `Observer` 接口、由组合根传入
`*telemetry.Metrics`，业务测试用 fake 即可。

`Observer` 的方法名带 `Messaging` 前缀是**故意**的：这样 `*telemetry.Metrics`
天然满足接口，不需要适配器。适配器是标签被悄悄丢掉的地方。

## Metrics Added

独立端口导出（API `127.0.0.1:9464`，worker `9465`），`METRICS_PATH` 默认 `/metrics`。

| 指标 | 类型 | 标签 | 为什么是这个形状 |
|---|---|---|---|
| `http_requests_total` | counter | method, route, status | route 是 chi 模板不是路径，否则每个 id 一条时间序列 |
| `http_request_duration_seconds` | histogram | method, route | |
| `http_errors_total` | counter | method, route | **只计 5xx**；4xx 是 API 正常工作 |
| `rate_limit_rejected_total` | counter | scope | 在日志采样器**之外**计数 |
| `redis_operation_total` / `_duration_seconds` | counter/histogram | command, outcome | 只记命令名，不记 key |
| `messaging_published_total` | counter | event_type | 投递成功后、`MarkPublished` 之前 |
| `messaging_consumed_total` | counter | consumer, event_type, outcome | |
| `messaging_consume_duration_seconds` | histogram | consumer, event_type | |
| `messaging_retry_total` / `_dlq_total` / `_duplicate_total` | counter | consumer, event_type | duplicate **不折进** consumed |
| `outbox_pending_events` | gauge | — | 在 `drain()` 结束时 defer 上报 |
| `user_registration_total` | counter | — | **事务提交后**计数 |
| `login_total` / `login_failure_total` | counter | — | 两者都计，失败率的分母必须是尝试数 |
| `db_pool_{total,idle,acquired}_conns` | gauge | — | pgx 池状态 |

几条不显然的取舍：

- **`login_total` 同时计成功与失败**。用成功数当分母，在"全部登录失败"的
  事故里分母归零，告警永远不触发。
- **malformed 登录请求不计入失败**。否则任何人 POST 一个空 body 就能拉高
  认证失败率，把安全告警变成噪音。
- **`user_registration_total` 计提交而非尝试**。计尝试会让注册曲线变成表单
  提交曲线：校验逻辑回归导致注册全挂时，图上反而是流量创纪录。
- **`messaging_duplicate_total` 与 consumed 分开**。折进成功数后，broker
  全量重投会被读成"吞吐翻倍"而不是"broker 有问题"。
- **`outbox_pending_events` 不用独立 ticker**。独立 ticker 是第二个会停的
  东西；relay 卡死时它仍在持续上报最后读到的值，看上去一切正常。

## Trace Coverage

| 位置 | span | 关键属性 |
|---|---|---|
| HTTP 入站 | `{METHOD} {route}` SERVER | `http.route`（模板；无匹配为 `unmatched`） |
| PostgreSQL | `db.query {QueryName}` CLIENT | 名字取自 sqlc 的 `-- name:` 注释 |
| Redis | `redis {COMMAND}` CLIENT | `cmd.Name()`，**不是** `cmd.String()` |
| NATS 发布 | `publish {event_type}` PRODUCER | 父节点 = outbox 里存的 traceparent |
| NATS 消费 | `consume {event_type}` CONSUMER | 父节点 = live publish span |

实跑验证的完整链路（Tempo 中的真实 trace，12 个 span，跨 API 与 worker 两个进程）：

```
POST /api/v1/users                            [api, SERVER]
├── redis EVALSHA ×2                          [限流]
├── db.query BEGIN / CreateUser / InsertOutboxEvent / COMMIT
└── publish user.created.v1                   [worker, PRODUCER]
    └── consume user.created.v1               [worker, CONSUMER]
        └── db.query BEGIN / ClaimEventForConsumer / COMMIT
```

**高基数与泄漏的三道防线**（每一道都有测试）：

1. `routeTemplate()` 用 chi 的 `RoutePattern()`，未匹配路由统一归并为
   `unmatched`。不这样做，一次扫描器攻击就能把 registry 撑爆。
2. DB/Redis 走**白名单**：只记 query 名 / 命令名。现成的库默认记
   `db.statement`，在本项目里那就是 `rl:login:victim@example.com` 和
   refresh token 本身。
3. span 的 error status **只记"失败了"不记"为什么"**。驱动的错误文本会引用
   参数：唯一约束冲突的报文里就带着那个邮箱地址。

**为什么手写 HTTP 中间件而不用 otelhttp**：chi 的 route 模板只有在
`next.ServeHTTP` **之后**才能读到。开 span 时先用临时名，出栈时 `SetName()`。
otelhttp 做不到这一点，只能拿到原始路径——那正是高基数的来源。

**traceparent 的优先级**（两处，都是刻意的）：

- `traceParentForEvents`（HTTP → outbox）：**活跃 server span 优先于入站头**。
  tracing 关闭时才回退到转发调用方的头，保证默认部署下 outbox 的
  traceparent 列不为空。
- `traceParentForMessage`（relay → NATS）：**live publish span 优先于库里存的头**。
  这让 consumer 成为 publish 的**子节点**而非兄弟。兄弟只说明"两件事发生过"，
  父子才说明"一件导致了另一件"。tracing 关闭时回退到 Phase 4 已有的头。

## Logging Changes

`internal/telemetry/logger.go` 的 `contextHandler.Handle` 在有效 span 下追加
`trace_id` / `span_id`。用 `sc.IsValid()` 而不是 `IsRecording()`：未采样的
span 依然有合法的 trace id，日志仍应带上它，否则采样一开日志就断链。

日志本身的字段、脱敏规则、采样策略沿用 Phase 2，本阶段未改。

## Docker Observability Stack

`docker compose --profile observability up -d` 拉起：

| 服务 | 镜像 | 端口 | 作用 |
|---|---|---|---|
| otel-collector | 0.115.1 | 4317/4318 | 接 OTLP，转发 Tempo |
| tempo | 2.6.1 | 3200 | trace 存储与查询 |
| prometheus | v3.0.1 | 9090 | 抓 API:9464 与 worker:9465 |
| grafana | 11.4.0 | 3000 | datasource 与 dashboard 均已 provision |

配置在 `deployments/observability/`。Grafana 打开即有
**Service Overview** 仪表盘（9 个面板：请求率 / 错误率 / P50-P95-P99 延迟 /
限流拒绝 / DB 池 / Redis / 消息发布与消费 / outbox 深度 / 登录失败率）。

**profile 是默认关闭的**：这套栈占约 1.5G 内存，不该是 `docker compose up`
的默认代价。

## 本阶段做的超范围的事

1. **`cmd/worker` 的 metrics 端口自动 +1**（`workerMetricsConfig()`）。
   prompt 没要求，但 API 与 worker 共用同一份 `.env`，不这样做两个进程会抢
   9464，且失败发生在启动时——正是最容易被当成"端口冲突、重启一下"忽略的地方。
2. **`deployments/observability/dashboard_test.go`**。prompt 只要求提供仪表盘。
   但仪表盘是本阶段唯一会**静默失效**的产物：查询一个不存在的指标名，渲染
   出来是一张空图，与"指标真的是 0"完全无法区分。这个测试解析仪表盘里的
   PromQL，断言每个指标名在真实 registry 中存在——**首次运行就抓到 2 个错名**。
3. **`internal/httpx/leak_test.go`**（§5.13 要求审计，未要求自动化）。
   一次请求同时在 body / header / query 里携带密码、access token、refresh
   token、cookie、api key，走完整中间件栈，然后在日志与 span 里同时搜。
   将来新增中间件会自动被它覆盖。
4. **升级 `google.golang.org/grpc` v1.81.1 → v1.82.1**。引入 OTLP gRPC
   exporter 后 `govulncheck` 报出 GO-2026-6061，属于本阶段引入的依赖。
5. **删除 `httpx.TraceContext` 中间件及其 3 个单测**。Phase 4 留下的占位实现
   只转发调用方的头，被 `Tracing` 中间件完全取代；留着它会产生两个来源不同
   的 traceparent。

## Known Limitations（Phase 5 新增）

1. **没有告警规则，也没有 SLO**。仪表盘能让人看见问题，但需要有人在看。
   告警需要先有 SLO，SLO 需要先有稳定的基线流量——这两样都还不存在。
2. **Collector 是单点且无持久队列**。Collector 挂掉期间的 span 直接丢失。
   生产上应加 `sending_queue` 的磁盘持久化，或直接双写。
3. **Tempo 用本地磁盘存储，无保留策略**。开发用足够，生产需要对象存储。
4. **指标端口无鉴权**，靠绑定 `127.0.0.1` 与网络策略保护。`endpoint.go` 的
   注释里写了三种方案的取舍：public 端口只差一条 ingress 规则就暴露到公网
   且会被限流吃掉；admin 端口要耦合鉴权；独立内网端口靠网络策略控制，是
   Prometheus 部署本来就需要的控制。
5. **`db_pool_*` 是 pgx 的采样值**，在 `/metrics` 被抓取时读取。抓取间隔内的
   瞬时耗尽看不见。
6. **exemplars 未接**。指标与 trace 之间只能靠时间窗口手工关联，不能从一个
   延迟尖峰直接跳到那条慢 trace。
7. **worker 的 `consume` span 不覆盖 handler 内部的业务分支**，只到
   `ClaimEventForConsumer` 与 handler 调用为止。

## 测试与变异验证

新增测试文件 11 个，全部经过变异验证（故意注入缺陷、确认测试变红）：

| 模块 | 测试数 | 变异 |
|---|---|---|
| config | +4 | 4/4 |
| telemetry/tracing | 10 | 5/5 |
| telemetry/propagation | 4 | 3/3（1 个存活，见下） |
| telemetry/metrics | 11 | 6/6 |
| telemetry/endpoint | 5 | 3/3 |
| telemetry/logger | +3 | 3/3 |
| httpx/observability | 11 | 8/8 |
| postgres/tracing | 8 | 6/6 |
| redis/tracing | 6 | 4/4 |
| messaging/observer + consumer + relay | 9 | 7/7 |
| messaging/natsmq/tracing | 6 | 3/3 |
| ratelimit/rejection | 3 | 2/2 |
| user/observer、auth/observer | 8 | 5/5 |
| deployments/observability/dashboard | 1 | 2/2 |
| cmd/api 集成（/metrics） | +1 | 2/2 |

### 变异抓到的真问题（写下来是因为它们都"测试全绿"过）

- **postgres M5**：`span.SetStatus(codes.Error, data.Err.Error())` 通过了全部
  测试，因为测试用的错误文本是无害的。真实驱动错误长这样：
  `DETAIL: Key (email)=(victim@example.com) already exists.`——最常见的那种
  失败，错误消息里就带着失败的那个邮箱。**只用无害文本测脱敏等于没测。**
- **redis M4**：同类问题。补了一个用 MOVED/CROSSSLOT 风格（错误文本里含 key）
  的测试之后才被抓到。
- **messaging Consumer M2**（重试耗尽却报成功）**存活**：唯一的死信测试走的是
  `ErrPermanent` 分支。而"耗尽"才是生产上更常见的路径（依赖宕机）。
- **user M3**（提交前计数）**存活**：唯一被拒的注册是校验失败，发生在事务之前。
  补了"事务回滚"的测试才抓到。
- **telemetry M2**（无 span 时的 `TraceParentFromContext`）**存活**：所有测试
  都持有真实 span，漏掉了"tracing 关闭"这个**默认部署形态**。全零的
  traceparent 语法合法，会让全系统共用同一个 trace id。

### 一个存活但合理的变异

`ContextWithTraceParent` 里的 `sc.IsValid()` 守卫被删掉后测试全绿——因为 W3C
propagator 本身已经拒绝了非法输入，这个守卫是冗余的。选择**保留**并把注释改
诚实：它是第二道防线，防的是将来换成 B3 / Jaeger propagator 的情况。

## 运行验证（2026-09-01）

- `make check` 全绿（`golangci-lint run` 0 issues，`govulncheck` 0 vulnerabilities）
- `make test-integration` 全绿（真实 PostgreSQL 17 + 真实 NATS JetStream）
- 黑盒回归 **110/110** 通过（t_http 14、t_authz 20、t_input 37、t_jwt 18、
  t_fixed 9、t_race 12）——Phase 3/4 无回归
- 实跑完整栈（Collector / Tempo / Prometheus / Grafana），注册一个用户：
  - Tempo 中得到跨进程的 12-span 完整 trace（见上）
  - Prometheus 两个 target 均 `up`，`http_requests_total`、`redis_operation_total`、
    `user_registration_total`、`login_total`、`messaging_published_total`、
    `messaging_consumed_total`、`outbox_pending_events`、`db_pool_total_conns` 均有值
  - 错误前缀 `/v1/users` 的 404 正确归并为 `POST unmatched`，未产生高基数序列
- `OTEL_ENABLED=false`（默认）下全部集成测试通过，业务行为不变

## 本阶段查清的一个"假故障"

端到端验证时 publish / consume span 始终不出现在 trace 里，而 worker 的日志
明确显示事件已处理。按 systematic-debugging 四阶段排查后确认：**代码没有问题**
——`/tmp/goscaffold-worker` 是 Phase 4 遗留的旧二进制（9-01 15:47 启动），没有
Phase 5 埋点，却在和新 worker 抢同一份 outbox 与同一个 JetStream consumer。
杀掉它之后链路立刻完整。

教训：验证可观测性时，**先确认在跑的就是刚构建的那个二进制**。这类"代码正确
但观测不到"的现象，最省时间的第一步是 `ps` 而不是读代码。

## Next Phase Entry Conditions

Phase 6 (容器化 / 部署) 开始前已成立：

- [x] `make check` 与 `make test-integration` 全绿
- [x] tracing 默认关闭，且关闭时业务行为不变（有实跑验证）
- [x] 业务包的**非测试代码**不 import OTel SDK 或 Prometheus（已用 `go list` 验证；
      `natsmq` 的测试用 `sdk/trace/tracetest` 做录制器，属于测试专用，Phase 8 静态审计须放行）
- [x] HTTP / DB / Redis / NATS 全链路 span 跨进程连通（有实跑验证）
- [x] 指标在独立端口导出，Prometheus 抓取成功
- [x] 仪表盘的指标名由测试守护
- [x] 无高基数标签，无密钥进入日志或 span（有自动化审计测试）

Phase 6 必须注意：

1. **镜像里要暴露 metrics 端口，但不要 publish 到宿主机**。`METRICS_ADDR`
   在容器里应绑 `0.0.0.0`(让 Prometheus 抓得到)，同时靠 NetworkPolicy /
   安全组限制来源——绑 `127.0.0.1` 在容器里等于谁也抓不到。
2. **`OTEL_EXPORTER_OTLP_INSECURE` 默认 false**。集群内走 mTLS 时不要为了
   省事打开它；开发环境的 compose 里已经显式设成 true，不要把那份配置复制到
   生产的 manifest。
3. **worker 与 api 是两个 Deployment**，各自的 metrics 端口不再冲突，
   `workerMetricsConfig()` 的 +1 逻辑在容器里是多余但无害的——不要因此删掉它，
   本地开发仍然依赖它。

---

# 调试工具:Bruno 集合(2026-09-02)

`api/bruno/` 是一份 git 跟踪的 Bruno 集合,24 个请求,覆盖
`api/openapi.yaml` 的全部 12 个 operation,外加失败路径。

**它解决的是一个具体的浪费**:调认证后面的接口时,每次都要先登录、把
access token 从响应里复制出来、粘进下一个请求。token 15 分钟过期,这个动作
一天要重复几十次。集合里 `auth/login` 的 post-response 脚本把 token 写进
变量,`collection.bru` 声明集合级 bearer 认证,请求写 `auth: inherit`。

**它不是测试套件。** 门禁仍然是 `test/blackbox`(110 条)与
`make test-integration`。集合里的断言只用来区分"环境坏了"和"你要调的东西
坏了"。不要往里加回归用例。

**`collection_test.go`** 断言每个请求的 `(method, path)` 存在于
`openapi.yaml`,单向。理由与 `deployments/observability/dashboard_test.go`
相同:指向已删除接口的请求返回 404,和服务挂掉长得一模一样,人会先去查
服务。经 3 个变异验证(指向不存在的接口 / 关掉 UUID 归一化 / 让集合为空)。

**凭据不进仓库**:管理员密码走 `{{process.env.BRUNO_ADMIN_PASSWORD}}`,
根 `.gitignore` 的 `.env` 规则(不带斜杠,任意层级生效)覆盖
`api/bruno/.env`。

**顺序由 `folder.bru` 的 `seq` 控制**:health → auth → users → errors。
实测发现 `bru run -r` 会先跑子文件夹再跑同级请求,所以 `errors/` 是顶层
文件夹而不是嵌在 `auth/` 与 `users/` 下面。

**`errors/` 自带一个成员用户的生命周期**(setup-member → login-as-member →
403 → setup-disable-member → 禁用登录 401 → cleanup-member),不复用
`users/` 那个主体。复用会被 `users/delete` 先删掉,`login-disabled-account`
于是走"邮箱不存在"分支——断言全绿,测的却是另一件事。

维护这份集合时有三个坑是实测踩出来的,改之前先读:

1. **`script:post-response` 对每个响应都会跑**,不只是 2xx。无保护的
   `bru.setVar` 会在登录失败时把好 token 覆盖成 `undefined`,之后每个请求
   都 401,而病因在两个请求之外。集合里所有写变量的脚本都带状态判断。
2. **一次完整运行花掉 6 次登录配额**(IP 10 次 /15 分钟)。连跑第二遍会从
   `login-as-member` 开始 429。这不是服务故障。
3. **CLI 每次运行的变量从空开始,GUI 在会话内保留。** 所以 GUI 里陈旧
   token 会让断言绿得毫无意义,CLI 不会。

设计文档:`docs/superpowers/specs/2026-09-02-bruno-collection-design.md`
实现计划:`docs/superpowers/plans/2026-09-02-bruno-collection.md`
