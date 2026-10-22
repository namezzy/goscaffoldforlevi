# Project State

> 本文件是各 Phase 之间唯一的上下文载体。
> 每个 Phase 结束时必须更新；下一个 Phase 开始时必须先读它。
> 内容必须与实际代码一致，禁止写入未完成的事项。

## Current Phase

**Phase 0–8 全部完成；Phase 8 之后的功能扩展与前端已提交（2026-10-12）。**

> 以下"Implemented / Not Implemented"等章节按各 Phase 完成时的快照书写，
> 其中"尚无容器化与 CI/CD"之类的说法已被后续 Phase 取代；最新状态以文末
> "Phase 8 之后的功能扩展与收尾"一节为准。

当前能力：脚手架核心（认证/RBAC/限流/Outbox 消息/OTel）、容器化与 CI/CD、
K8s 部署（跨发行版与生产拓扑已验证）、`goscaffold` CLI、组织/邀请/工单/
通知/安全中心/异常中心后端，以及 React 管理后台（统一暗色主题，最近提交
`2b435a1`）。没有进行中的阶段。
**未覆盖**：ticket/org/invite 的 HTTP 层测试、异常中心以外的前端页面测试。

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

**`collection_test.go`** 断言每个请求的 `(method, path)` 与 `openapi.yaml`
的 operation **双向**一一对应,并检查同级 `seq` 不重复。理由与
`deployments/observability/dashboard_test.go` 相同:指向已删除接口的请求
返回 404,和服务挂掉长得一模一样,人会先去查服务;`seq` 撞车则让执行顺序
未定义,表现为某个无关文件夹里的间歇 401/404。上面那句"覆盖全部 12 个
operation"也归它守着——否则加一个接口,这句话会静默失真。经 5 个变异验证
(指向不存在的接口 / 关掉 UUID 归一化 / 让集合为空 / 重复 seq / 删掉某
operation 的唯一请求)。

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
   token 或 id 会让断言绿得毫无意义,CLI 不会。凡是跨请求传递的变量
   (`userId`/`memberId`/`memberAccessToken`)都在 `script:pre-request` 里
   先清空再设置。

设计文档:`docs/superpowers/specs/2026-09-02-bruno-collection-design.md`
实现计划:`docs/superpowers/plans/2026-09-02-bruno-collection.md`

---

# Phase 6 — Infrastructure / Docker / CI / Kubernetes（2026-09-09）

设计文档:`docs/superpowers/specs/2026-09-04-phase6-infrastructure-design.md`。
六个锁定决策(D1 distroless / D2 Kustomize / D3 CI 写完整 / D4 compose full
profile / D5 版本走 /health / D6 禁止可变 tag 进 manifest)全部按原样落地,
另有三处与设计文档不符,见「与设计文档的偏差」。

## Infrastructure Files

```text
Dockerfile                              三个 target:api / worker / migrate
.dockerignore
internal/buildinfo/buildinfo.go         ldflags 注入点
.github/workflows/ci.yml                PR 门禁,三个 job
.github/workflows/docker.yml            master / tag 构建推送 ghcr.io
deployments/k8s/
  base/{kustomization,configmap,secret.example,api-deployment,api-service,
        worker-deployment,migrate-job,ingress,hpa,pdb}.yaml
  overlays/{local,development,staging,production}/kustomization.yaml
  manifests_test.go                     守卫测试,5 个变异验证过
```

修改:`docker-compose.yml`(api/worker + full profile)、`Makefile`
(LDFLAGS、docker-* 三个目标、build-worker、actionlint 进 tools)、
`internal/config/load.go`(APP_VERSION 默认值改为 `buildinfo.Version`)、
`internal/httpx/health.go`(`Build` 结构体,`/health` 与 `/ready` 增加
`commit` 字段)、`cmd/api/run.go` 与 `cmd/worker/run.go`(启动日志打 commit
与 build_time)、`api/openapi.yaml`(两个 schema 增加 `commit`)、README 双语。

## Docker Architecture

单文件多 target,三者共享到最后一次 COPY 为止的全部层。

| Target | 基础镜像 | 大小 | 说明 |
|---|---|---|---|
| `api` | `gcr.io/distroless/static:nonroot` | 40.8 MB | |
| `worker` | 同上 | 35.1 MB | |
| `migrate` | `migrate/migrate:v4.19.1` | — | 本仓库的 SQL 烤进镜像 |

**distroless 的代价已写进 README**:没有 shell,`kubectl exec` 进不去,
必须用 `kubectl debug` 的临时容器。这条不写下来,会在最糟的时刻被发现。

构建期注入 `internal/buildinfo` 的三个变量。`-X` 路径写错时链接器**静默
忽略**,所以 Makefile 与 Dockerfile 里的路径必须一致——已实跑确认注入生效
(容器 `/health` 返回真实 SHA)。

## CI Pipeline

`ci.yml` 三个 job,按**耗时**而不是按主题拆分,目的是让第一个红叉在一分钟
左右出现:

- `quality` — gofmt / vet / golangci-lint / `sqlc diff` / actionlint / OpenAPI lint
- `test` — build / `go test -race` / govulncheck
- `integration` — `go test -tags=integration`(testcontainers)

**集成测试沿用 testcontainers 而不是 Service Container**(§6.6 要求做选择):
仓库本地已用 testcontainers,两套机制会让同一份依赖在一处过、另一处挂,而
差异永远不在人第一眼看的地方。

`docker.yml` 推 `master` 与 `v*` tag 时发布到 `ghcr.io/${{ github.repository }}`,
产出 `sha-<commit>` 与发版的 `v1.2.3 / v1.2 / v1`。

## Kubernetes Strategy

Kustomize:一个 base、四个 overlay(local / development / staging / production)。

- **迁移用独立 Job**,不用 initContainer。initContainer 每 Pod 跑一次,
  三副本发布会并发执行同一份 DDL,且此后每次重启都再跑一遍
- **liveness 只打 `/live`,不碰任何依赖**;readiness 打 `/ready` 查
  PostgreSQL 与 Redis。理由是重启风暴:liveness 若查库,一次几秒的主库切换
  会让所有副本同时失败、同时被杀、同时冷启动打向正在恢复的库
- **`terminationGracePeriodSeconds: 45`**,不是刚好大于 30 的 31
- **`METRICS_ADDR: ":9464"`**,不是应用默认的 loopback
- **HPA 只给 API**,基于 CPU 70%。worker 没有 HPA,因为 CPU 对它几乎永远
  是错的指标:积压时它在等 I/O,CPU 很低,而那正是最该扩容的时刻。正确信号
  是 NATS consumer lag,需要 custom metrics adapter。这条写在 manifest 注释里
  而不只是 README——改 HPA 的人不会去读 README

## Security Decisions

- 三个工作负载(api / worker / migrate)统一:`runAsNonRoot` + `runAsUser:
  65532` + `readOnlyRootFilesystem` + `allowPrivilegeEscalation: false` +
  `capabilities.drop: [ALL]` + `seccompProfile: RuntimeDefault`
- `readOnlyRootFilesystem: true` 成立的前提是进程不写盘:日志走 stdout、
  无本地缓存与临时文件。这个前提一旦变化,应挂 emptyDir 而不是放宽设置
- Service **不暴露 9464**。指标名枚举了每个路由模板、依赖和构建版本;抓取
  方应直连 Pod IP,限制访问面是 NetworkPolicy 的事
- 仓库只放 `secret.example.yaml`,且它**不在** base 的 `resources` 里——
  applied 出一个值为 `REPLACE_ME` 的 Secret,比缺 Secret 更难读
- base ConfigMap 写死 `APP_ENV: production`,让漏配 overlay 失败在安全一侧

## 与设计文档的偏差

三处,均按 §0.3 明示而非静默修改。

1. **D2 的论据错了,结论仍成立。** 设计称 Kustomize 是「本机唯一能真
   `--dry-run=client` 验证的方案」。实测 `kubectl apply --dry-run=client`
   要从集群下载 OpenAPI schema,本机无集群时直接报
   `failed to download openapi`——四个 overlay 全部失败。改用 `kubeconform
   -strict` 对官方 1.33 schema 离线校验,strict 会拒绝未知字段,正是想用
   dry run 抓的那类笔误。选 Kustomize 的其余理由(零新依赖、比 Helm 简单)
   不受影响。

2. **D3 的前提已失效。** 设计写「仓库当前没有 git remote,workflow 不会被
   触发」,据此把 CI 标为永久 SKIPPED。远端仓库已于 2026-09-08 建立并推送,
   workflow **会**在下次 push 时真实触发。因此 SKIPPED 的理由从「不可能
   跑」降级为「尚未观察到结果」。

3. **迁移 Job 的承载方式设计文档没定。** 备选是 configMapGenerator 从
   `migrations/` 生成 ConfigMap,否决理由有两条:ConfigMap 与目录会漂移;
   且 kustomize 默认的 load restrictor 不允许 base 目录向上读文件,启用
   `LoadRestrictionsNone` 会让 `kubectl apply -k` 需要额外 flag。改为在
   Dockerfile 增加第三个 target,把 SQL 烤进 `migrate/migrate` 镜像——镜像
   不可变且与代码同 commit,「这个版本期望哪份 schema」有确切答案。

## 测试与变异验证

`deployments/k8s/manifests_test.go` 读 **base YAML** 而不是 `kustomize build`
的输出:测试不该依赖 CI 上未必存在的二进制,而这些约束都在 base 层定义。
若将来 overlay 开始覆盖它们,应扩展测试而不是改成调用 kustomize。

五个变异全部让测试变红,且每条都点名了具体文件与原因:

| 变异 | 被哪个测试抓到 |
|---|---|
| `terminationGracePeriodSeconds` 45 → 20 | `TestGracePeriodOutlastsTheShutdownBudget` |
| livenessProbe `/live` → `/healthz` | `TestProbesPointAtPathsTheServiceServes` |
| 删掉 `readOnlyRootFilesystem` | `TestWorkloadsAreHardened` |
| `METRICS_ADDR` → `127.0.0.1:9464` | `TestMetricsAddressIsNotLoopback` |
| production overlay 改用 `:latest` | `TestNoManifestReferencesAMovingTag` |

余量断言值得单独说明:测试不只要求 grace period **大于** shutdown timeout,
还要求余量 ≥ 10 秒。「刚好大于」是同一个 bug 的隐蔽版本。

## 运行验证（2026-09-09）

| 命令 | 结果 |
|---|---|
| `make check` | 全绿 |
| `make test-integration` | 全绿(PostgreSQL 17,testcontainers) |
| `docker build --target api / worker / migrate` | 三个均成功 |
| 镜像连真实 Postgres/Redis 运行 | `/ready` 200,`commit` 返回真实 SHA |
| `docker compose --profile full up -d` | 9 个服务起来,`/live` 200 |
| SIGTERM 关停时序 | `/ready` 转 `draining` 503 约 5 秒 → 监听器关闭 → 退出码 0 |
| `kubectl kustomize` 四个 overlay | 均渲染成功(local/dev 8 对象,staging/prod 9 对象) |
| `kubeconform -strict -kubernetes-version 1.33.0` | 四个 overlay 全部 Valid,0 Invalid |
| `actionlint .github/workflows/*.yml` | 无问题 |
| `go test ./deployments/k8s/` | 绿,并经 5 个变异验证会红 |
| `make actionlint`(带 shellcheck) | 无问题 |
| **GitHub Actions 在 Runner 上执行** | **CI 与 Docker 均 success**(第三次尝试,见下节) |
| **在真实集群上 apply** | **SKIPPED:无可用集群** |

### D3 的 SKIPPED 已解除,代价是两个只在 Runner 上才出现的缺陷

设计文档把 CI 标为永久 SKIPPED,理由是没有远端仓库。远端已建立,于是这条
从"不可能跑"变成"跑了并且看到了结果"——**头两次都是红的**,而两个原因在本机
都无法察觉:

1. **`golangci-lint-action@v6` 不认 v2 配置。** 该 action 的 v6 线只支持
   golangci-lint v1,本仓库 `.golangci.yml` 声明 `version: "2"`。v8.0.0 的
   发布说明明确写着 "Requires golangci-lint version >= v2.1.0"。失败发生在
   解析配置阶段,读起来像配置写坏了,而不像 action 版本选错了。

2. **`actionlint` 在本机和在 CI 上是两个强度的检查。** actionlint 会对每个
   `run:` 块调用 shellcheck,而 shellcheck 不是 Go 工具、不在 `make tools`
   里。本机没装 → 静默跳过一半检查 → 本地绿、CI 红。真实告警是 SC2086:
   `go install ...@${SQLC_VERSION}` 未加引号。

第 2 条同时暴露了一个更值得记的问题:仓库一直宣称"CI 跑的每条命令都有对应的
Makefile target",但 actionlint 此前没有。已补 `make actionlint`,且它在缺
shellcheck 时**明确提示本次检查比 CI 弱**,而不是假装通过。复现只需官方静态
二进制:`actionlint -shellcheck <path>`,不必装进系统。

这两次失败正是 §0.5 存在的理由。设计阶段写下的 "SKIPPED:未在 Runner 上跑过"
如果一直留着,这两个缺陷会留在仓库里,直到第一个 clone 它的人撞上。

## 本阶段做的超范围的事

三件,均记录理由:

1. **`internal/httpx.Build` 结构体**替代原来的 `version string` 参数。
   §6.2 只要求暴露版本信息;改签名是为了不引入第二个位置参数(两个相邻
   string 参数极易调换且编译器不报错),代价是 8 处测试调用点机械更新。
2. **`cmd/worker/run.go` 的启动日志**也打 commit 与 build_time。§6.2 没提
   worker;不加会出现「api 能说自己是哪个构建、worker 不能」的不对称。
3. **`kubeconform` 与 `actionlint` 两个新工具**。前者是 D2 论据失效后的
   替代验证手段,后者是设计文档验证计划里就列出的。actionlint 已加入
   `make tools`;kubeconform 没有加,它只在有 manifest 变更时才需要。

**没有做**:Trivy、NetworkPolicy、ServiceMonitor、PrometheusRule、真实
Secret、Helm——设计文档的「明确不做」清单原样执行。

## Known Limitations（Phase 6 新增）

| 妥协 | 理由 |
|------|------|
| manifests 从未在真实集群 apply 过 | 无可用集群。schema 校验能抓字段与类型错误,抓不到调度、准入控制器、CNI 层面的问题 |
| `secret.example.yaml` 不在 base 的 resources 里,首次部署必然缺 Secret | 这是刻意的:Pod 因缺 Secret 起不来,比起来一个值为 `REPLACE_ME` 的 Secret 让服务启动后自我拒绝要好读得多 |
| `local` overlay 里带了开发用凭据 | 与 `docker-compose.yml` 同性质的本机口令,`APP_ENV=development` 下有效、production 下被启动守卫拒绝。文件里已写明「要抄就抄 staging,不要抄 local」 |
| overlay 的镜像 tag 是占位 SHA(全 0) | 仓库没有已发布镜像。真实部署时由 CI 或人替换;`TestNoManifestReferencesAMovingTag` 只禁可变 tag,不校验 SHA 是否存在 |
| HPA 用 CPU,worker 无 HPA | custom metrics adapter 不在本阶段范围。触发条件已写进 manifest 注释 |
| `migrate` 镜像的 CLI 版本与 go.mod 手工对齐 | 两者不一致时,Job 与 `make migrate-up` 可能以不同方式解释同一个目录。目前靠注释约束,没有测试守卫 |

## Next Phase Entry Conditions

Phase 7 (`goscaffold` CLI) 开始前已成立:

- [x] `make check` 与 `make test-integration` 全绿
- [x] 三个镜像可构建,且以非 root 运行
- [x] 构建期版本信息端到端连通(ldflags → buildinfo → config → /health)
- [x] compose 的四个 profile 划分与 §6.3 一致,`full` 名副其实
- [x] 四个 overlay 渲染并通过严格 schema 校验
- [x] 关停时序实测与设计一致
- [x] 承重的部署约束有守卫测试,且经变异验证

Phase 7 必须注意:

1. **`goscaffold new` 生成的项目要不要带 Phase 6 的产物**,是第一个要定的
   问题。带上则模板里出现 `ghcr.io/namezzy/...` 这类本仓库特有的字面量,
   必须一并参数化;不带则生成出的项目缺一整层。
2. **模板里的 Dockerfile 与本仓库的会漂移。** 本仓库改了 Dockerfile 而
   模板没改,生成出的项目就带着一份过时的构建定义。需要一个守卫测试,或者
   明确接受漂移并写进文档。
3. §7.5 的「绝对禁止覆盖已有文件」是硬约束。第一版可以不做 `--force`,
   但不能默认覆盖。

---

# Phase 7 — Scaffold CLI（2026-09-09）

## CLI Commands

`cmd/goscaffold`,四个命令,标准库 `flag`。

```text
goscaffold new <name> --module <path> [--output <dir>] [--dry-run]
goscaffold module <name>              [--dir <project>] [--dry-run]
goscaffold migration <name>           [--dir <project>] [--dry-run]
goscaffold version
```

**§7.8 的框架选择:标准库 `flag`。** Cobra 与 urfave/cli 买到的是补全、嵌套
子命令和生成的帮助文本。本程序四个命令、没有嵌套、帮助文本一屏放得下,所以
唯一的收益是生成帮助,代价是一棵比这个二进制本身还大的依赖树。与 ADR-0001
否决 Viper 同一条理由。复审触发条件:某个命令长出自己的子命令,或补全成为
真实需求而不是好主意。

## Template Architecture

```text
internal/scaffold/
  names.go        标识符的五种形态,单一来源
  plan.go         两阶段写入:先建计划,再落盘
  project.go      清单 + new 的生成
  module.go       module 的生成
  migration.go    migration 的生成
  templates.go    embed + Sync
  templates/
    project/      221 个文件,本仓库的派生副本,勿手改
    module/       7 个手写模板
    newproject/   生成项目自己的 README
  gentemplates/   make templates 调用的写入端
```

模板文件一律加 `.tmpl` 后缀。**这个后缀是承重的**:没有它,内嵌的每个 `.go`
都会被当作本模块的一部分编译,每个 `_test.go` 都会作为整个项目的第二份冲突
副本运行。有 `TestEveryTemplateHasTheSuffix` 守着。

`//go:embed all:templates` 的 `all:` 前缀同样承重:没有它,embed 会跳过所有
以点开头的路径,于是 `.github`、`.golangci.yml`、`.env.example`、`.gitignore`
会从每个生成的项目里静默消失。

## Template Drift Solution（§7.12）

**代码只有一份。** `templates/project/` 不是手工维护的第二套代码,而是本仓库
的派生副本,由 `make templates` 生成,由 `TestProjectTemplatesMatchTheRepository`
守住。改了源文件却没重新生成 → `make check` 失败并点名文件。

这与 `make sqlc-check` 是同一个机制,只是对准了另一种生成物。**经三个变异
验证**:

| 变异 | 测试输出 |
|---|---|
| 改 `internal/apperr/apperr.go` 不重新生成 | 点名 `internal/apperr/apperr.go.tmpl`,并提示 `make templates` |
| 删掉 `Makefile.tmpl` | "not embedded at all: Makefile.tmpl" |
| 留下一个源文件已不存在的模板 | "embedded but no longer in the repository: GHOST.tmpl" |

**模块模板是例外,它无法派生**——它描述的是一个还不存在的模块。守住它的是
`.github/workflows/scaffold.yml`:生成项目 → 在其中生成模块 → 编译 + vet +
test。`apperr` 或 `httpx` 的改动若破坏了模板假设的形状,这个 job 会红。

`scaffold.yml` 单独成文件而不是 `ci.yml` 的一个 job,因为 **`ci.yml` 会被
复制进每个生成的项目**,而生成的项目没有 `cmd/goscaffold`。它已加入
`projectExcludes`。

## Safety Mechanisms（§7.5）

两阶段:先构建完整的 `Plan`,核对全部路径,再落盘,且用 `O_EXCL` 让文件系统
做最终裁决(先检查后写入之间有竞态,只有 `O_EXCL` 不可被绕过)。任一文件挡路
则**一个字节都不写**,并列出全部冲突而不是遇到第一个就停。

边走边写的生成器一旦中途停下,留下的目录既不是旧状态也不是新状态——那是唯一
无法靠重跑恢复的结果。

**没有实现 `--force`**(§7.5 允许第一版不做)。扩展点就是 `Plan.Write` 里那
一处冲突检查,它下面的代码不读取任何已有状态。

`--dry-run` 与真实运行**走同一条构建路径**,只在最后分叉,所以它不可能与真实
行为脱节。

## Generation Examples

`goscaffold module order` 产出 7 个文件,三包结构:

```text
internal/order/order.go              ID / 实体 / CreateInput / Patch / Page
internal/order/service.go            Repository 端口 + Service
internal/order/service_test.go       手写 fake,6 个测试
internal/order/orderhttp/dto.go      请求/响应类型,与实体分离
internal/order/orderhttp/handler.go  5 个端点 + 权限常量
internal/order/orderhttp/handler_test.go  6 个测试
internal/order/orderpg/repository.go 编译得过的桩,方法返回 NOT_IMPLEMENTED
```

**生成的端点在权限注册前一律 403。** 模板用 `auth.Permission("order:create")`
这样的常量,而策略表是权限的唯一来源、没登记的永远不授予——于是新端点在有人
决定"谁可以访问"之前是不可达的。这是刻意的 fail-closed,不是遗漏。

**pg 仓储是桩,也是刻意的。** 为一张还不存在的表生成 SQL,产出的是没人读过
的生成 SQL——而那正是压测时才发现写错了的那种代码。桩返回 `apperr.Internal`
而不是零值:静默返回零值会让 service、handler 及其测试全绿,而功能什么也不做。

## Tests / Generated Project Verification

- `internal/scaffold`:名称转换(含 `city→cities`、`box→boxes`、`holiday→holidays`、`address→addresses`)、拒绝歧义输入、迁移版本、冲突拒绝、dry-run 不落盘、三包结构、依赖方向
- `cmd/goscaffold`:参数解析两种顺序、dry-run、冲突退出码、next steps 内容、非法名字
- **全部在 `t.TempDir()` 中运行**(§7.13)

**§7.14 生成项目验证**(本机实跑,并已作为 CI job):

```text
goscaffold new demo --module github.com/example/demo
goscaffold module order
goscaffold migration create_orders
→ gofmt -l . 输出 0 行 / go vet 干净 / go build 通过 / go test 24 个包全绿
```

生成的 Go 在写盘前过一遍 `go/format`。原因是替换会改变标识符宽度:按
`__TYPE__` 对齐的模板产出围绕 `Order` 就错位,生成的项目第一次跑自己的
`make check` 就会挂。顺带的好处是,不是合法 Go 的模板在**生成时**就报错。

## 与 Phase 7 要求的偏差

1. **§7.6 迁移版本改用固定 17 位毫秒时间戳**,而不是"秒级 + 冲突时提高精度"。
   后者会静默破坏顺序:17 位的版本号大于所有 14 位的,所以今天为避开冲突而
   加精度的文件,会排在下周才写的迁移之后。宽度必须恒定,版本号才能永远当整数
   比较。已有的 `000001`–`000004` 保持不变(它们更小,顺序正确);
   `make migrate-create` 已改为调用 goscaffold,`-seq` 不再可达。

2. **`--with-redis` / `--with-messaging` / `--with-observability` 三个开关
   都没做**(§7.3 称其为"可选开关",且明说"不要第一版就做十几个开关")。
   理由逐条:Redis 是限流的硬依赖(ADR-0008);telemetry 穿在每个构造函数上
   (ADR-0010),去掉会编译不过;而 messaging **ADR-0009 已经在运行期做成默认
   关闭**——开关只能删掉一些本来就不花代价的文件。

3. **§7.10 配置文件、§7.11 插件系统均未实现**,与 prompt 的"如果第一版没有
   必要就不要实现"/"只设计扩展点"一致。换模板来源的扩展点是
   `readTemplateTree`,一个函数。

## 本阶段做的超范围的事

1. **`Plan.Add` 对 `.go` 内容调用 `go/format`。** §7 未要求。不做的话生成的
   项目自带两个未格式化文件,第一次 `make check` 就红。
2. **`make migrate-create` 改为调用 goscaffold。** §7.6 只要求 CLI 有这个
   命令。不改的话仓库里有两条产出不同命名的路径,而两者的差异只会在合并时
   才显现。
3. **`make scaffold` / `make templates` 两个目标。** 后者是漂移机制的写入端,
   没有它,测试只会说"过期了"却不给出修复方式。

## Known Limitations（Phase 7 新增）

| 妥协 | 理由 |
|------|------|
| `new` 本质上是"带替换的整仓库复制" | §7.3 定义的职责就是"创建目录/复制模板/替换 module 与服务名"。真正的差异化在于替换的正确性有测试守着,以及副本不会漂移 |
| 生成的项目仍带 `docs/adr/` 与 `docs/architecture.md` | 设计理由是这套脚手架的主要资产。代价是新项目继承了一份写着别人项目名的历史文档 |
| 复数规则只覆盖三条英语规则 | `person→persons` 是错的。失败可见且改个名就好;引入 inflection 库要付一个依赖和一张词表,而它的失败模式是没人预料到的复数 |
| 文件权限一律 0644,不保留可执行位 | 当前清单里没有需要可执行位的文件。若将来加入 `scripts/*.sh`,需要扩展 `File.Mode` 的填充 |
| 模块模板的 pg 仓储是桩 | 见上文。使用者必须补 migration、queries、`make sqlc` 才有真实实现 |
| 模板只收 git 跟踪的文件,因此 `make templates` 与漂移测试都需要 git 可用 | 取舍是明确的:失去在无 git 环境下运行的能力,换来「被忽略的文件不可能进模板」。见上文的凭据泄漏 |
| 模板替换是纯文本 | 若某个项目名恰好是 Go 关键字或与依赖包同名,替换后可能编译不过。`ProjectOptions.Validate` 只检查了字符集,没检查这一点 |

## 本阶段泄漏了一个真实凭据（2026-09-09）

必须记下来,因为它是这套模板机制第一次实际造成的伤害,而修复它的方式决定了
它不会再发生。

**发生了什么**:模板清单按**目录**纳入 `api/bruno`。`api/bruno/.env` 被
`.gitignore` 忽略,里面是 Bruno 集合用的真实管理员密码。复制成模板时每个文件
都加了 `.tmpl` 后缀,于是 `.gitignore` 的 `.env` 规则不再匹配
`api/bruno/.env.tmpl`——它被提交并推送到公开仓库,存在约十分钟。

**怎么发现的**:不是靠审阅,是 CI 变红。Runner 上没有这个未跟踪文件,漂移
测试因此报「embedded but no longer in the repository」。**同一个机制既造成了
泄漏,又暴露了它**——如果模板不是派生并被守住的,这个文件会静静躺在仓库里。

**根因修复**:`CollectFromRepo` 现在只收 `git ls-files` 报告的跟踪文件。
清单无法穷举将来可能出现在某个纳入目录下的每一个被忽略文件,而 git 已经知道。
这让这类泄漏在构造上不可能发生,而不是靠再加一条黑名单。

**处置**:提交经 amend 后 force-push,该 blob 不再被任何引用可达。但
**GitHub 仍按 SHA 提供那个孤儿提交**(实测 HTTP 200),未被引用的对象在 GC
前依然可取。所以清理历史是次要的,**该凭据必须视为已泄露并轮换**。

三条可推广的教训:

1. **按目录纳入意味着纳入你没在看的文件。** 清单应当基于 git 的已知事实,
   而不是人对目录内容的记忆。
2. **重命名会让 `.gitignore` 失效。** 任何给文件改名或加后缀的复制过程,都
   把「被忽略」这个属性丢掉了。
3. 一个未跟踪文件在本机存在、在 Runner 上不存在——**本机与 CI 的文件集合
   不同,是一类独立的 bug 来源**,而漂移测试恰好对它敏感。

## Next Phase Entry Conditions

Phase 8 (文档与终审) 开始前已成立:

- [x] `make check` 与 `make test-integration` 全绿
- [x] `goscaffold new/module/migration` 产出的项目 build / vet / test / gofmt 全过
- [x] 模板漂移由测试强制,并经三个变异验证
- [x] 生成器绝不覆盖已有文件,且有测试
- [x] 生成项目验证已进 CI(`scaffold.yml`)

Phase 8 必须注意:

1. **README 已经写了三轮**(Phase 6 的部署章节、Phase 7 的 CLI 章节)。
   §8.1 要求的章节清单需要与现有内容对照,补缺而不是重写。
2. **§8.4 要求做一次 drift 检查**。路由与 spec 的双向一致已经是自动强制的
   (`cmd/api/openapi_test.go`),该条应聚焦在**请求体** schema——那是目前
   唯一没有自动校验的一侧,已列在 Known Limitations 里。
3. **`docs/development.md` 与 `docs/deployment.md` 尚不存在**,而 README 里
   已有相当一部分对应内容。要决定的是拆分还是链接,不要两处各写一份。

---

# Phase 8 — Documentation & Final Audit（2026-09-09）

终审报告是本阶段的主交付物:[`docs/final-report.md`](docs/final-report.md),
22 节,含逐项门禁与证据。本节只记录**做了什么改动**与**审计发现了什么**。

## 文档结构的决定

进入本阶段前留下的问题是「拆分还是链接,不要两处各写一份」。答案:

- **README 是导览**,负责让人在一屏内判断这是什么、特别在哪、怎么起步
- **`docs/` 是深度**,负责操作细节与设计理由

据此把 README 的部署章节从 ~140 行压到 ~45 行(英文版 677→599,中文版
617→545),细节移交 `docs/deployment.md`。概念上的重叠(比如"四处承重设置"
那张表)保留在 README,因为那是评估者最该看到的东西——落地页与手册有概念重叠
是正常的,两份同样的操作步骤才是重复。

新增三份文档,均为中文,与 `docs/` 既有的 10 份 ADR 和 architecture.md 一致:

| 文档 | 回答什么 |
|---|---|
| `docs/development.md` | 我要做 X,该改哪些文件 |
| `docs/deployment.md` | 怎么放上去,出事怎么办 |
| `docs/security.md` | 防什么、不防什么、已知暴露面 |

**语言不一致是已知状态,不是疏忽**:README 双语(英文默认)而 `docs/` 为
中文。把 10 份 ADR 与 architecture.md 译成英文超出本阶段范围,而在 `docs/`
里开第二套语言约定比统一用中文更糟。已在 README 明写,并列入终审报告的
「未来工作」第 7 条。

**三份新文档刻意不进模板清单**:`deployment.md` 写着本仓库的验证状态、
`security.md` 记着 Phase 7 那次泄漏事故——复制到新项目里就是假话。与不复制
README 是同一条理由。

## 审计发现并修复的问题

**`internal/auth/authhttp` 覆盖率 42.4%,低于门槛。** 该包有中间件测试,
但**完全没有 handler 测试**——登录、刷新、登出三个最敏感的端点在 HTTP 层
没有任何断言。补 9 个测试后到 **92.4%**。新增断言覆盖:token 响应必须带
`Cache-Control: no-store`(缺了它,共享代理或浏览器后退键会把一个用户的
token 交给下一个请求)、登录失败不可区分、响应体不回显提交的密码、
`X-Forwarded-For` 在 `trustedProxies=0` 时被忽略。

写这批测试时踩到一件值得记的事:**假实现不真实,测出来的是另一件事。**
最初的假仓储对"会话不存在"返回普通 `error`,于是登出返回 500,我一度以为
发现了 bug。真实仓储返回的是 `apperr.NotFound`,而服务对这两者的处理**本就
应该不同**——前者是预期状态(隐藏),后者是数据库故障(必须上报)。修正假
实现后又补了一条测试专门钉住这个区别:登出对不存在的会话返回 204,对真实
故障返回 500。永远返回 204 会告诉客户端"家族已撤销"而实际什么都没发生,
被盗的 token 继续有效。

**漂移守卫在本阶段抓到两次,第二次抓到的是它自己的缺陷。**

第一次是正常工作:改了 `docs/architecture.md`(它在模板清单里)而没重新生成,
`make check` 立刻变红并点名文件。

第二次暴露了 Phase 7 那个机制的真实缺陷:**它的结果依赖 git 暂存状态**。
`CollectFromRepo` 用的是 `git ls-files --cached`,只看已跟踪的文件。于是新写的
`handler_test.go` 在提交前是不可见的——本机 `make check` 全绿,提交之后立刻
变红,而 CI 是第一个看到的人。**一个在提交前放行、提交后才失败的检查,恰好在
没人看的时刻失败。**

改为 `--cached --others --exclude-standard`:已跟踪的,加上未跟踪且未被
gitignore 的。两半都是承重的,且各自来自一次真实缺陷——排除被忽略的文件是
Phase 7 那次凭据泄漏的修复,纳入未跟踪文件是这一次的修复。新增
`TestCollectFollowsGitRatherThanTheFilesystem` 在临时 git 仓库里同时钉住两者,
经两个变异验证(退回 `--cached`、去掉 `--exclude-standard`),各自点名对应
的失败原因。

## 审计通过、未发现问题的项

逐条记录,因为"查过且没问题"与"没查"是两回事:

| 审计 | 结论 |
|---|---|
| §8.7 架构 | 无 utils/common/base 包;无 DI 容器;无 init() 隐式初始化;超过 5 个方法的接口 4 个,其中 `db.Querier`(26)是 sqlc 生成物,其余三个是按实际使用列举的具名操作,不是 `GenericRepository[T]` |
| §8.8 Go 惯用法 | 包名皆为单数名词;错误一律 `%w` 包装;context 贯穿所有 I/O;资源关闭走 defer |
| §8.9 并发 | `go test -race` 全绿;非测试代码 7 处 goroutine,全部绑 context 且有退出路径 |
| §8.10 HTTP | 超时/体积上限/panic 恢复/取消传播/优雅退出/状态码/CORS/限流 均已落地并有测试 |
| §8.11 数据库 | 无 `SELECT *`;无 N+1;WHERE 列均有索引(唯一例外 `users.role`,仅引导时查一次且当时表为空);分页有硬上界 |
| §8.12 消息 | 丢失路径封闭(同事务 outbox);重复由去重日志兜住;DLQ 可查询可重放;ACK_WAIT 与重试预算的关系有启动校验 |
| §8.13 安全 | grep 无密钥入日志路径;`leak_test.go` 端到端扫描通过 |
| §8.14 依赖 | 18 个直接依赖,逐个给出存在理由,其中 2 个仅测试用 |
| §8.15 性能 | 七项明显问题逐项检查,均无 |
| §8.17 仓库清洁 | 无 `.env`、密钥、IDE 缓存、二进制、覆盖率产物入库 |

## 本阶段做的超范围的事

一件:**给 `internal/auth/authhttp` 补了 9 个 handler 测试**。§8 说"不要继续
加功能",测试不是功能,而 §8.18 的覆盖率门槛要求它。不补就无法诚实地勾上那一项。

## Known Limitations（Phase 8 新增）

| 妥协 | 理由 |
|------|------|
| `docs/` 为中文,README 为双语 | 见上。已列入未来工作 |
| 三份新文档不进模板清单,生成的项目只继承 ADR 与 architecture.md | 它们含本仓库的验证状态与事故记录,复制过去即为假话 |
| 覆盖率门槛只对"核心包"成立 | `cmd/api`(5.4%)与 `internal/config`(39.5%)偏低。前者是装配代码,由集成测试与黑盒套件覆盖;后者的分支多在错误路径上。两者都不是 domain/service/handler |
| `make run` 不注入 ldflags | 开发运行报告 `dev`/`unknown`,这正是 buildinfo 默认值想表达的意思 |

## Next Phase Entry Conditions

**八个阶段全部完成。** 后续工作不再是阶段,而是终审报告第 22 节的清单,
按价值排序:真实集群验证 → 邮件 provider → 请求体 schema 校验 → 自定义指标
HPA → NetworkPolicy/ServiceMonitor → depguard 规则可推导化 → 文档英文化。

任何后续改动仍受 §0.3 约束:推翻既有 ADR 必须新建 ADR 并写明 `Supersedes:`;
每次改动后 `STATE.md` 必须与代码保持一致。


---

# 真实集群验证（2026-09-09）

终审报告第 22 节的第 1 条「在真实集群上跑一次」已完成,并**找出三个只有真实
部署才会暴露的缺陷**。

## 环境

局域网服务器(Ubuntu 24.04 / x86_64 / 8 核 / 30G)上的 **k3d 三节点集群**
(1 server + 2 agent),k3s v1.35.5 / containerd 2.2.3,API 暴露到局域网,
从本机 Mac 用 kubeconfig 驱动。

**Mac 没能作为节点**:kubelet 只跑 Linux,OrbStack 的虚拟机是 NAT 网络,
服务器路由不回来。服务器 sudo 需要密码,所以用 k3d 而不是宿主机 k3s。
镜像在服务器上构建——Mac 是 arm64,服务器是 x86_64,交叉跑会
`exec format error`。

## 验证通过的项

| 项 | 观察到的结果 |
|---|---|
| kubelet 真的调用探针 | 日志里有真实 request_id |
| **数据库故障只影响 readiness** | `/ready` 5s 转 503 → 10s 摘出 EndpointSlice → **restartCount 全程 0** |
| 恢复自愈 | 数据库回来后 10s 重新 Ready,零重启 |
| `maxUnavailable: 0` | 3 副本全量重发布,集群内探针 **117 次请求零失败** |
| 关停时序 | SIGTERM → `draining delay=5s` → `/ready` 503 → **+5.001s 干净退出** |
| securityContext | 六项全部通过准入,未被丢弃 |
| PDB 强制 | 收紧后驱逐被拒:`Cannot evict pod as it would violate the pod's disruption budget` |
| outbox → JetStream → 消费者 | 跨节点走通,事件被消费并标记 handled |
| 迁移 Job 与 Deployment 分离 | Job 完成后应用才 Ready |

## 找出并修复的三个缺陷

**1. `adminctl` 不在任何镜像里,集群内无法引导第一个管理员。**
Dockerfile 只有 api/worker/migrate 三个 target。这是个死结:`POST /users` 需要
`user:create` 权限,而第一个管理员存在之前没人拥有它。新增 `adminctl` target,
并把用法写进 runbook——用 `-password-stdin` 从操作者终端传密码,**密码因此不
进入 Pod 日志、etcd 或任何 Secret**。已实测:管理员已存在时拒绝运行;不存在时
创建成功且输出不含密码;用该密码登录返回 200。

`stdinOnce: true` 不能省,没有它容器等不到 EOF 会一直挂着——实测踩到过。

**2. 探针端点在 IP 限流器后面。** `r.Use(deps.RateLimitByIP)` 在
`Health.routes(r)` 之前,没有路径豁免,限流键就是客户端 IP。

危害是间接的,因此极难诊断:限流配额经 Redis **跨副本共享**,而 kubelet 探针的
源 IP 是**节点 IP**——同一节点上的 Pod 越多越共享同一个桶,每 Pod 每分钟约 18
次探针,六个 Pod 就超过 100/分钟的配额。然后 `/ready` 返回 429,kubelet 判定
探针失败,Pod 被标记 NotReady 并摘出 Service。**限流器自己把服务搞挂了,而
没有任何日志说「限流了 kubelet」**,表现出来只是集群在抖。

修法是把三个探针路径从限流中豁免,且**只豁免这三条**——用路由分组会把 404
泛洪也一并放行,那是真实的保护回退。路径集合由 `Health.paths()` 导出,与挂载
表同源,不会漂移。已实测:600 次 `/ready` 全部 200、零 429,同期 300 次 API
请求有 98 次 429。

**3. CI 不发布 `migrate` 镜像,而 manifests 引用它。** `docker.yml` 的矩阵只有
api 与 worker,`base/migrate-job.yaml` 却指向 `-migrate`。真实部署会在迁移 Job
上 `ImagePullBackOff`——发布停在「新 Pod 已经开始滚动、schema 却没动」的位置。
我们最初没撞上,只因为镜像是手工构建导入的。矩阵补齐四个镜像。

## 新增的守卫测试（均经变异验证）

| 测试 | 抓什么 |
|---|---|
| `TestProbesAreNotRateLimited` | 探针被限流(429 即探针失败),以及豁免被放宽到全部路径 |
| `TestEveryReferencedImageIsPublished` | manifest 引用了 CI 不构建的镜像 |
| `TestOverlaysDeployingTheWorkerEnableMessaging` | overlay 部署了 worker 却把 messaging 关掉 |

第三条对应的是本次发现的一个**矛盾而非故障**的配置:`local` overlay 的
ConfigMap 写 `MESSAGING_ENABLED=false`,而 worker 的 Deployment 在 `env` 里
硬写 `true`——`env` 优先级高于 `envFrom`。结果是 API 丢弃事件、worker 等一个
永远不来的事件,**两个进程都健康,没有任何错误**,唯一的症状是功能什么都不做。
已把 `local` overlay 改为 `true`(local-deps 提供了 broker)。

## 新增 `deployments/k8s/local-deps/`

`base/` 里没有 PostgreSQL / Redis / NATS,这对生产是对的(它们是托管服务),
但让本机集群验证无从下手。新增这一层,明确标注**不是部署制品**:
无资源限制、无持久卷、数据在 `emptyDir` 里。

验证过程中我自己撞上了这个限制:把 postgres 缩容到 0 做故障演练,再拉起来时
initdb 重建了空库,四张表全没,worker 开始刷
`relation "outbox_events" does not exist`。README 里写了,我还是撞上了——所以
runbook 里把它升级成了 WARNING。

## 新增 `docs/runbook.md`

照着做的部署步骤与故障处置,每一步都在上述集群上执行过。它与
`deployment.md` 的分工:后者回答「为什么这样部署」,前者回答「照着敲什么」。

## 仍未覆盖

托管集群特有的部分:真实 Ingress 控制器、HPA(需 metrics-server)、
NetworkPolicy(需支持它的 CNI)、LoadBalancer、真实存储、以及公司集群常见的
准入控制器(OPA / Kyverno 经常会拒掉一些字段)。


---

# 跨发行版与生产拓扑验证（2026-09-10）

在第一次真实集群验证之后追加两轮,各自关掉了一块此前无法验证的区域。

## 一、跨发行版:同一套 manifests 在两个 Kubernetes 上

第二个集群是 Mac 上 OrbStack 内置的 Kubernetes(v1.35.6+orb1,单节点),
与服务器上的 k3d(k3s v1.35.5,三节点)是**两个不同的发行版**。

**零改动部署成功**——同一份 overlay、同一套 manifests。逐项对照:

| 验证项 | k3s(3 节点) | OrbStack(单节点) |
|---|---|---|
| securityContext 六项通过准入 | ✅ | ✅ 完全相同 |
| 探针不被限流 | 600×200 / 0×429 | 600×200 / 0×429 |
| 限流仍对 API 生效 | 300 中 98 次 429 | 300 中 99 次 429 |
| 数据库故障 → NotReady | 10s | 10s |
| 故障期间 restartCount | 0 | 0 |
| 恢复自愈 | 10s | 10s |
| 关停时序 | SIGTERM +5.001s | +5.003s |
| adminctl 引导(stdin) | ✅ | ✅ |
| outbox → JetStream → 消费者 | ✅ | ✅ |

时序数字逐位吻合,因为它们由 manifest 的 `periodSeconds` / `failureThreshold`
与应用自己的 `HTTP_READINESS_DRAIN_DELAY` 决定,**不由发行版决定**。这正是想
确认的事。

**唯一的发行版差异是镜像分发**:OrbStack 的 k8s 与 Docker 共享镜像库,
`docker build` 完 `imagePullPolicy: Never` 直接生效;k3d 需要额外一步
`k3d image import`。差异对本项目无害,已写进 runbook。

## 二、生产拓扑:依赖在集群之外,跨真实网络

此前所有验证里 PostgreSQL 与 Redis 都在同一个集群内一跳可达,而**生产里它们
是外部托管服务**。这一轮把应用留在服务器的集群里,把 PostgreSQL 与 Redis 放到
另一台机器(Mac,192.168.1.102),走真实局域网。

**结论:不需要改任何代码或 manifest,只改 Secret 里的连接串。** manifests 用
`postgres:5432` 这样的 Service 名只是默认值,不是假设。

服务端 `pg_stat_activity` 确认连接确实来自集群一侧,不是本地回环。

| 指标 | 值 |
|---|---|
| `/live`(不碰依赖) | ~2ms |
| `/ready`(PG + Redis 各一次往返) | ~13ms |
| `GET /users`(两次查询) | ~24ms,冷连接首次 64ms |
| 迁移耗时 | 172 / 252 / 327 / 420ms |
| 同一批迁移在集群内 | 6 / 19 / 32 / 52ms |

**迁移慢约 8 倍**,这是真实网络延迟的直接体现。值得记住的是它对
`kubectl wait --timeout` 的含义:集群内 50ms 完成的迁移,跨网络要几百毫秒,
而一套几十条迁移的成熟 schema 会把这个差距放大到分钟级。

### `sslmode=require` 首次验证

这一条关掉了文档里一个明确的空白:`deployment.md` 一直让人用
`sslmode=require`,而此前所有验证都跑在 `sslmode=disable` 上。

在 Mac 上起了一个开启 TLS 的 PostgreSQL(自签证书),把连接串改为
`sslmode=require`。迁移与全部业务链路正常,服务端 `pg_stat_ssl` 确认:

```text
ssl     = t
version = TLSv1.3
cipher  = TLS_AES_256_GCM_SHA384
bits    = 256
```

TLS 下 `/ready` 约 20ms,明文约 13ms。

## 过程中的一个错误

在服务器上执行 `docker compose up -d` **之前没有先看那台机器上跑着什么**。
它实际运行着 26 个容器,包括 Vaultwarden 密码管理器与多个 `buzz-prod-*`
生产服务,部分已运行数周。那条命令试图占用 5432 与 6379,两个端口都被真实
服务占用——**绑定冲突让它安全失败,但那是运气,不是谨慎**。

发现后改变了方向:把数据库放到 Mac 一侧,**服务器不新增任何监听端口**。
失败留下的两个 Created 容器、一个网络、一个卷均已清理,原有服务全程未受影响。

教训可推广:**在别人的机器上发布端口之前,先列出已经在跑什么。** 端口冲突
是少数会安全失败的错误之一,不能指望下一次还这么走运。

## 仍未覆盖

托管集群特有的部分,没有真实托管集群就做不了:真实 Ingress 控制器、
HPA(需 metrics-server)、NetworkPolicy(需支持它的 CNI)、LoadBalancer、
真实存储(StatefulSet + PVC),以及公司集群常见的准入控制器
(OPA / Kyverno 经常会拒掉一些字段)。

---

# Phase 8 之后的功能扩展与收尾（2026-10-12）

Phase 8 之后在工作区里累积了一批未提交的扩展:用户名登录、组织与部门、
邀请、五角色权限与审计、工单、站内通知、安全中心(会话管理、自助密码
重置)、实时请求监控、消息队列管理端点,以及异常中心(`internal/incident`,
把死信归并成可分派的异常)。后端与 React 前端(`frontend/`)大部分已完成,
本轮补齐其中没收尾的部分。

## 本轮发现并补上的缺口

| 缺口 | 处理 |
|---|---|
| 模板漂移测试失败:`internal/incident` 的 3 个测试文件未嵌入 | `make templates` |
| `internal/incident/watcher_test.go` 未经 gofmt(`make check` 会拦) | `gofmt -w` 后重新同步模板 |
| 前端没有异常中心:无 API 客户端、无页面、`permissions.ts` 缺 `incident:*` | 新增 `api/incidents.ts`、`IncidentsPage`、`IncidentDetailPage`、`incidentShared.tsx`,路由与导航按 `incident:read` 守卫 |
| 新异常的站内通知点进去落回工作台 | `NotificationsPage.targetHref` 支持 `incident` |
| 4 处代码注释引用 `docs/security.md` 的"异常中心"小节,但该小节不存在 | 补写该小节,并补"策略表之外的两处手抄"说明 |
| `docs/security.md` 仍写"两个角色、五个权限" | 改为五个角色、二十二个权限 |
| `frontend/README.md` 仍写"12 个 operation"、两种角色 | 按现状重写 |

详情页的操作按后端状态机显示:只有 `open` 可确认;`resolved` 之后不可再
操作;`replay_status = pending` 时重放按钮禁用,文案是"已提交,等待结果",
从不显示成成功。负责人可"指派给我"/"清除";持有 `user:list` 时额外有成员
下拉(operator 没有 `user:list`,所以不给它一个必然 403 的列表)。

设计文档:`docs/superpowers/specs/2026-10-12-exception-center-frontend-design.md`

## 验证

- `go build ./...`、`go vet ./...`、`go test ./...` 全部通过(含模板漂移)
- 前端 `npm run build`、`npm test` 通过;`npm run lint` 无错误,新增文件只有
  与既有页面同类的两种警告(`set-state-in-effect`、`only-export-components`)

## 端到端验证（Docker,独立环境）

独立起一套 PostgreSQL / Redis / NATS 容器,API 与 worker 用不同的
`AUTH_JWT_SECRET` 启动,让邀请邮件 handler 解不开 token、事件进死信,
再走完整流程。发现三个真实缺陷,单元测试(全是 fake)都看不到:

| 缺陷 | 根因 | 修复 | 回归测试 |
|---|---|---|---|
| **异常中心从不产生任何异常**,watcher 每次轮询都报错 | `incident_poll_state.last_failed_at` 种子值是 `'-infinity'`,pgx 无法扫进 `time.Time` | 迁移 `20261012030000000_incident_poll_state_epoch`:默认值与现有行改为 `'epoch'` | `incidentpg/poll_state_integration_test.go` |
| **死信重放在保留期内必然 500**(Phase 4 起就存在,`make dlq-replay` 同样受影响) | `Replay` 用原事件 id 新插一行 outbox,而已投递的原行在 `PublishedRetention` 内仍在,主键冲突 | 新查询 `RequeueOutboxEvent`:`ON CONFLICT (id)` 把原行重置为未发布 | `messagingpg` 的 `TestReplayWhileTheOriginalOutboxRowIsRetained` |
| **2 分钟内的第二次重放被 broker 静默丢弃**,异常永远停在"重放中" | JetStream 去重键是事件 id,重放保留 id,落在 `Duplicates` 窗口内即被当作重复 | 迁移 `20261012030100000_outbox_replay_generation` 加 `replay_generation`,每次重放 +1;发布端去重键为 `id/replay-N`(首投仍是 id,relay 崩溃重发仍去重);消费者去重不变 | `natsmq` 的 `TestAReplayIsNotSwallowedByBrokerDedupe` |

另:详情页在无 `user:read` 时不再发注定 403 的用户查询(operator 打开详情页
控制台曾有两条 403)。

走通的流程:死信 → watcher 开异常 → 同指纹再失败归并(次数 +1)→ 改级别 /
指派 / 确认(重复确认 409)/ 备注(空备注 400)→ 重放(错误 secret 下
`pending → failed`,并再次归并)→ 换正确 secret 后 30 秒内再次重放
`pending → succeeded` → 解决(重复解决 409)→ 摘要计数归零;处理记录与审计
一致;新异常的站内通知 `target_type = incident`。权限:auditor 可读不可写
(403),operator 可读写,read_only 403,匿名 401。前端用无头 Chrome 以
operator 登录,导航 → 列表 → 详情渲染正常,控制台无错误。

`go test ./...`、`go test -tags integration ./...`(NATS 指向测试 broker)、
`make sqlc-check`、模板漂移、前端 build / test / lint 均通过。

## 用户本机环境验证

在用户自己的 Docker(`make docker-up` / `broker-up` / `migrate-up`,库
`go_enterprise`)上重跑同一流程,全部通过。过程中停掉了两个陈旧进程:一个
无数据库连接的旧 API 构建,一个占着 metrics 端口 9465 并抢消费事件的旧
worker。注意:要让 worker 跑消费者、watcher 与 relay,`.env` 需
`MESSAGING_ENABLED=true`。

## 收尾（提交、收紧已解决状态、补测试）

提交:`cfb4bd8` 后端扩展 · `d196495` 前端 · `36294bb` 端到端三个缺陷 ·
`4115830` 已解决状态收紧 · `7c4037d` HTTP 层测试 · `2aa4a8c` 前端页面测试。

- **已解决是终态,后端强制**:之前只有确认/解决检查状态,已解决的异常仍可
  改级别、指派、写备注、重放。`Service.lockUnresolved` 在动作事务里
  `GetForUpdate` 锁行后判断,一律 409 `INCIDENT_INVALID_TRANSITION`,不写
  审计、不触发重放;`RequestReplay` 原本在事务外无锁读取,一并移进事务。
  OpenAPI 四个端点补 409。本机实测:对已解决异常的六个动作全部 409,
  审计无新增行。
- **`incidenthttp` handler 测试**:权限(401/403)、状态码(202/409/400/404)、
  响应体对照 OpenAPI;经变异验证(改 202、去掉路由权限中间件均被抓到)。
- **前端页面测试**:引入 `@testing-library/react` + `jsdom`(仅开发依赖,
  页面测试文件单独声明 jsdom 环境)。覆盖按状态/权限显示动作、pending
  不显示为成功、409 展示、无 `user:read` 不查用户、列表过滤与 URL 同步。
- **测试暴露的前端缺陷**:负责人控件包在 `<label>` 里,operator(无
  `user:list`)时 label 的第一个可标注元素是"指派给我",点"负责人"标题
  文字就把异常指派给自己。改为 `role=group` + `aria-labelledby`。

## 仍未覆盖

- ticket/org/invite 的 HTTP 层仍没有 handler 测试;路由与契约一致性由
  `cmd/api/openapi_test.go` 守着。
- 前端除异常中心外的页面没有测试(测试基础设施已就位)。

## 前端视觉统一（2026-10-12）

提交 `2b435a1`：统一科技感暗色主题，抽出共享 UI 组件（`frontend/src/components/ui/`），
全部页面改用之；设计文档 `docs/superpowers/specs/2026-10-12-frontend-visual-unification-design.md`。

## 评审提出的两个 P1 并发窗口（2026-10-21）

都是"在事务外读取状态、用这份快照决定授权、再在事务里写入"的同一类缺陷，
按 TDD 修：先写会失败的测试，再改实现。

**工单授权快照未与写入版本绑定**（`internal/ticket/ticket.go`）
`transition` 按读到的 `before` 授权，却用客户端传来的 `ExpectedVersion` 做
条件更新。旧负责人把 `ExpectedVersion` 预填成下一版本，就能在经理改派提交
之后仍然写入成功——授权看的是改派前的快照。现在要求
`ExpectedVersion == before.Version`，否则 409 `TICKET_VERSION_CONFLICT`；
条件更新一律用 `before.Version`。回归测试
`TestAPrefilledNextVersionCannotCarryAStaleAuthorizationPastAReassign`
（fake 仓储在 `GetByID` 后插入一次并发改派）。

**用户账户守卫读到的角色可能已过期**（`internal/user/service.go`）
`Update`、`Delete`、`ChangePassword` 都在事务外 `GetByID` 读角色，再用它跑
`GuardActionOnAccount` 和最后管理员保护。目标在这个窗口里被提升为
system_admin 时，member_admin 仍能停用／改密，最后管理员保护也不触发。
新增查询 `GetUserByIDForUpdate`（`SELECT ... FOR UPDATE`）与仓储方法
`GetByIDForUpdate`，三处的读取与守卫全部移进各自的写事务、在行锁下进行。
`ChangePassword` 的密码校验因此排到守卫之前：Argon2 哈希不能在持锁期间做。

回归测试两层：
- `internal/user/service_test.go` 的
  `TestAPromotionBetweenTheReadAndTheWriteStillProtectsTheAccount`，
  覆盖 update / delete / change password / 最后管理员四种路径（fake 的
  `GetByIDForUpdate` 模拟"并发写先落地、锁再放行"）。
- `internal/user/userpg/account_guard_lock_integration_test.go` 的
  `TestAGuardWaitsForAConcurrentPromotionToCommit`：真 PostgreSQL，测试自己
  开一个事务持有目标行锁并提升角色，确定性地证明 Update 会阻塞在锁上、拿到
  的是提升后的角色。变异验证：把加锁读改回 `GetByID`，该测试失败。

验证:`go build ./...`、`go vet ./...`、`go test ./...`（含模板漂移）、
`make sqlc-check`、`go test -tags integration ./internal/user/...` 全部通过。

## 异常中心的两个 P2（2026-10-22）

评审剩下的两个缺陷都在 `internal/incident`，都要改表，一起修。同样先写会
失败的测试。

**重放结果归因到了错的事件**（`watcher.go` 的 `resolveOne`）
判定一次重放成功与否，原本比较 `incident.LastSeenAt` 与 `ReplayRequestedAt`:
晚于请求就算又失败了。但 `last_seen_at` 是整个指纹的，同指纹下任何别的事件
失败都会抬高它，于是一次成功的重放被记成失败;合并还会把 `incident.event_id`
挪到最新那个事件上，连"成功"那一侧也在问错事件。

改为请求重放时记下被重放的那个事件:`incidents.replay_event_id`（迁移
`20261022040000000`，把在途的 pending 行回填成 `event_id`）。成功＝
`processed_events` 里该事件有晚于请求的行;失败＝该事件自己的死信行
`failed_at` 晚于请求（新查询 `ExistsDeadLetterSince`）;两者都没有就继续
pending。`last_seen_at` 不再参与判定。回归测试
`TestResolvePendingReplayIgnoresAnUnrelatedFailureUnderTheSameFingerprint`。

**死信水位扫描会永久漏掉晚提交的行**（`pollOnce`）
`incident_poll_state.last_failed_at` 记"已处理到哪个 failed_at"，轮询取
`failed_at > watermark`。提交顺序不等于 `failed_at` 顺序:一个事务盖了时间戳
之后、比它晚的失败已经把水位推过去了才提交，这行就永远看不见了——而且无声
无息，因为没人会回头看。

改成逐行认领:`dead_letter_events.incident_processed_at`，轮询用
`ClaimUntriagedDeadLetter`（`WHERE incident_processed_at IS NULL ORDER BY
failed_at LIMIT 1 FOR UPDATE SKIP LOCKED`）与 `MarkDeadLetterTriaged`，
`incident_poll_state` 整张表删除（迁移 `20261022041000000`）。SKIP LOCKED
让第二个 watcher 副本去拿下一行而不是排队;两个副本同指纹并发 Create 会被
`incidents_fingerprint_active_idx` 挡回、事务回滚、下轮重试。

当初选时间戳水位而不是逐行标记，理由是 `dead_letter_events` 每个
(event_id, consumer) 只有一行、重试复用该行，标记会"留在"一次它没见过的
失败上。这个理由现在在复用的那一处回答掉了:`RecordDeadLetter` 的
ON CONFLICT 同时把 `incident_processed_at` 清空，所以每次新的失败——不论新
行还是复用行——都重新进入待分诊。

回填取舍:迁移把 `failed_at <= 旧水位` 的行标成已处理。当年被这个 bug 跳过的
旧行和正常处理过的行已经无法区分，于是不去追溯;它们要是再失败一次，会照常
重新分诊。

验证（除单测外都是真 PostgreSQL）:
- `internal/incident/watcher_test.go`:`TestPollOnceStillPicksUpADeadLetterThatCommittedLate`。
- `incidentpg/triage_claim_integration_test.go`:晚提交仍被认领、SKIP LOCKED
  不排队、复用行重新进入分诊三个测试。
- `messagingpg_integration_test.go` 的 `TestARepeatFailureReopensTriage`:守住
  上面那条 ON CONFLICT。
- 变异验证:把认领改回水位语义 → 晚提交测试失败;去掉 ON CONFLICT 里的
  `incident_processed_at = NULL` → 重复失败测试失败。
- `make sqlc-check`、`make templates`、`go vet ./...`、`go test ./...`、
  `go test -tags integration ./...` 全部通过。

删除:`incidentpg/poll_state_integration_test.go`（它测的表已不存在）。
