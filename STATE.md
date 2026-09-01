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
