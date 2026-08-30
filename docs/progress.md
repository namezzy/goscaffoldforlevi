# 构建日志

> 本文件记录**做了什么、为什么这么做**，按时间顺序。
>
> 它不是使用说明（那属于 Phase 8 的 `README.md`），也不是设计文档
> （那是 [`docs/architecture.md`](architecture.md) 和 [`docs/adr/`](adr/)）。
> 它回答的是："这个仓库是怎么变成现在这样的？"
>
> 各 Phase 之间的**当前状态**以 [`STATE.md`](../STATE.md) 为准；
> 本文件与它冲突时，以 `STATE.md` 为准。

## 目录

- [阶段零：改造开发 prompt](#阶段零改造开发-prompt)
- [Phase 1：架构设计](#phase-1架构设计)
- [Phase 2：基础工程](#phase-2基础工程)
- [Phase 3：用户、认证、RBAC、限流、OpenAPI](#phase-3用户认证rbac限流openapi)
- [当前仓库结构](#当前仓库结构)
- [在本机跑起来](#在本机跑起来)
- [下一步](#下一步)

---

## 阶段零：改造开发 prompt

起点是一份 36.6 KB、7 个阶段的开发 prompt。评估后判断它的**内容质量高**
（负向约束清单、质量门禁、"拒绝盲从"条款、Template Drift 的洞察都是稀有的），
但**作为多轮执行的工程流程不成立**。

### 发现的 8 个问题与处理

| # | 问题 | 严重性 | 处理 |
|---|------|--------|------|
| 1 | **阶段间无状态持久化**。每个 Phase 独立会话，但没有任何机制把上一阶段的决策传给下一阶段 | 致命 | 新增 `STATE.md` + `docs/adr/` 落盘机制，写进 §0.3 |
| 2 | 全局质量门禁要求"每阶段跑 `go test`"，但 Phase 1 明令"不写代码" | 矛盾 | 门禁改为 **Phase 2 起**适用 |
| 3 | **工具链顺序倒置**：Phase 3 需要 migration 和 sqlc，但它们排在 Phase 5 | 致命 | 前移到 Phase 2，写进 Phase 2 的 Entry Conditions |
| 4 | 要求 LLM 自评分（1-10），并规定"低于 8 分要重做" | 无效 | 删除自评分，改为**客观验收门槛**（命令退出码 / 测试通过） |
| 5 | Rate Limit 和 OpenAPI 出现在需求里，但没有任何阶段负责实现 | 悬空 | Rate Limit 归入 Phase 6，OpenAPI 归入 Phase 8 |
| 6 | 没有环境不可用时的兜底，模型会**编造执行结果** | 致命 | §0.5：环境不可用必须写 `SKIPPED: <原因>`，严禁编造 |
| 7 | 单阶段体量过大，超出单次会话的可靠输出长度 | 高 | 拆分 + 新增 Messaging 阶段，7 阶段 → 8 阶段 |
| 8 | `Authorization: ******` 是脱敏残留，会被当作字面量 | 低 | 修正 |

### 产出

```
prompts/
├── README.md          使用说明、验收清单、失控信号表、阶段依赖图
├── PHASES.md          完整合订本（1740 行）
├── 00-contract.md     全局执行契约，优先级高于任何单个 Phase
├── 01-architecture.md ~ 08-docs-audit.md
└── templates/
    ├── STATE.md
    └── adr.md
```

`00-contract.md` 是这次改造的核心，六条规则：

| 条款 | 内容 |
|------|------|
| §0.3 | 每个 Phase 结束必须更新 `STATE.md`；推翻既有 ADR 必须新建 ADR 并写 `Supersedes:` |
| §0.4 | 质量门禁（Phase 2 起）：`fmt` / `vet` / `build` / `test` / `test -race` / `golangci-lint` / `govulncheck` |
| §0.5 | 环境不可用写 `SKIPPED: <原因>`，**严禁编造执行结果** |
| §0.6 | 不得超范围；每阶段结束自答"本阶段有没有做超范围的事？" |
| §0.7 | commit message 格式 `phase(N): <总结>` |
| §1.20 | 文档归属规则——每条设计决定必须能指到一个 ADR 或 `architecture.md` 的小节 |

ADR 模板里加了一条硬要求：**`When to Revisit` 必须是可观测的触发条件**，
不能写"当需求变化时"。一个无法被观测到的复审条件等于没有复审条件。

---

## Phase 1：架构设计

产出全部为文档，仓库中没有一行 Go 代码。

### 9 个 ADR

| ADR | 主题 | 结论 |
|-----|------|------|
| [0001](adr/0001-tech-stack.md) | 技术选型 | chi / pgx+sqlc / slog / Argon2id / UUIDv7 / NATS JetStream(默认关) / OTel(默认不导出)。**明确拒绝** Viper、validator、GORM、zap、DI 框架 |
| [0002](adr/0002-project-layout.md) | 目录与依赖方向 | 扁平 `internal/`；每业务功能三包 `user` / `user/userpg` / `user/userhttp`；依赖方向由 **depguard 编译期强制**。否决 domain/application/infrastructure、platform、pkg、common、utils、bootstrap |
| [0003](adr/0003-error-model.md) | 错误模型 | `Kind` + `Code` 两个维度；Kind 零值 = Internal；Kind→HTTP 映射表**只在一个文件** |
| [0004](adr/0004-api-conventions.md) | API 约定 | 成功响应**无 envelope**，错误响应有；offset 分页；**统一 PATCH 不提供 PUT**；`DisallowUnknownFields` |
| [0005](adr/0005-data-access.md) | 数据访问与事务 | `TxManager.WithTx(ctx, fn)`，tx 藏在未导出的 context key；**`pgx.Tx` 不出现在任何签名**；不支持嵌套事务；Pod 启动不自动迁移 |
| [0006](adr/0006-id-strategy.md) | ID 策略 | UUIDv7 应用侧生成，PG 原生 `uuid` 类型；业务包定义具名类型 |
| [0007](adr/0007-auth.md) | 认证授权 | JWT HS256 access 15min + opaque refresh 30d；**family 盗用检测**；Argon2id(64MB/t=3/p=4)；登录三种失败返回同一 401 |
| [0008](adr/0008-rate-limiting.md) | 限流 | Redis Lua token bucket；**通用 fail-open，登录 fail-closed**；限流拆成两个中间件分列认证前后 |
| [0009](adr/0009-messaging.md) | 消息与异步 | NATS JetStream，默认关闭；Outbox 纳入 v1；at-least-once + `processed_events` 幂等；DLQ 用数据库表 |

### `docs/architecture.md`

632 行，承载没有进入 ADR 的设计：模块边界、配置、缓存、可观测性、
**中间件顺序（逐条给理由）**、测试策略、Bootstrap 顺序、优雅退出的时间预算、
以及请求 / 错误 / 配置 / 认证 / 消息五条流程图。

中间件顺序是这份文档里最容易被后续阶段搞错的部分：

```
Recovery → RequestID → Tracing → Logging → Metrics → CORS
        → BodyLimit → Timeout → RateLimitByIP
        → Authentication → RateLimitByUser → Authorization
```

其中两条位置是**反直觉但必须的**：

- **CORS 在限流之前** —— 否则浏览器的 preflight 会消耗用户配额
- **Metrics 在限流之前** —— 否则 429 不会被统计，而 429 正是最需要看的指标

---

## Phase 2：基础工程

一个可运行、但没有任何业务功能的骨架服务。

### 包结构

| 包 | 职责 | 关键设计 |
|----|------|---------|
| `internal/apperr` | 错误模型 | `Kind` 零值 = Internal（忘记分类 → 500 + 脱敏，而不是意外泄漏）；`Kind.String()` 越界返回 `"unknown"`，供穷尽性测试探测 |
| `internal/validate` | 语法校验 | `Violations` 累加器，一次返回全部字段错误。**不含任何业务规则**（"密码至少 12 位"属于 auth 包） |
| `internal/config` | 配置 | `Secret` 类型；`Load()` 环境变量优先 + godotenv 兜底；`Validate()` 累积全部问题后一次性返回 |
| `internal/telemetry` | 日志 | `contextHandler` 从 context 自动注入 `request_id`；Phase 5 在同一处加 `trace_id` |
| `internal/postgres` | 连接池与事务 | `TxManager.WithTx` / `Querier(ctx, pool)` / `HealthCheck`；`pgtest` 是 testcontainers 夹具 |
| `internal/redis` | 缓存客户端 | 连接失败一律返回 error，required/optional 的取舍留给调用方 |
| `internal/httpx` | HTTP 传输 | `errmap.go` 是唯一的 Kind→status 表 |
| `cmd/api` | 组装 | `main.go` 只决定退出码；`run.go` 手工 DI + 优雅退出 |

### 把约定变成机制

Phase 2 最重要的产出不是代码，是这张表。写在文档里的规则会被违反，
能让构建失败的规则不会。

| 机制 | 实现位置 | 违反时 |
|------|---------|--------|
| 依赖方向（ADR-0002） | `.golangci.yml` depguard 三组规则 | `make lint` 失败 |
| Kind→status 映射唯一且穷尽 | `internal/httpx/errmap_test.go` | 新增 Kind 未登记 → 测试失败 |
| `.env.example` 与 loader 不漂移 | `internal/config/env_example_test.go` | 键名不一致 → 测试失败 |
| 示例密钥不得进 production | `Validate()` + 测试交叉校验两侧 | 启动失败 / 测试失败 |
| sqlc 生成物不过期 | `make sqlc-check`（`sqlc diff`） | CI 失败 |
| 迁移与代码 schema 一致 | `pgtest` 对真实库跑真实迁移 | 集成测试失败 |

depguard 规则的实质内容：`internal/user/*.go` 和 `internal/auth/*.go`
的根目录**不得 import** `net/http` / `chi` / `pgx` / `go-redis` / `nats.go` / `otel`。
理由不是纯洁性——是一个 import 了 `net/http` 的 service 无法被消费者或 CLI 复用，
一个 import 了 `pgx` 的 service 无法脱离数据库测试。

### 相对 Phase 1 设计的补充决定

Phase 1 没有覆盖到、在 Phase 2 实现中才必须拍板的七件事：

1. **`run(ctx)` 放在 `cmd/api/run.go`（package main）** —— 既满足"shutdown 可测试"，
   又不违反 ADR-0002 对 `bootstrap/` 包的否决。
2. **`main()` 只做 `os.Exit(start())`** —— `os.Exit` 会跳过 defer，
   把二者写在同一个函数里会静默丢掉清理逻辑。
3. **503 / 504 / 405 不进 `Kind` 集合** —— 它们描述传输层状态
   （依赖不可用 / 超时 / 路由表），不是业务错误的分类。
4. **自己实现 `Timeout` 而不用 `http.TimeoutHandler`** —— 后者只掐响应写入，
   不取消 handler 的 context；超时之后数据库查询仍在跑，而那正是要释放的资源。
5. **`RequestID` 校验入站头必须是 UUID** —— 该头由客户端控制，
   未经校验就写进每一行日志，等于开放日志注入。
6. **新增 `LOG_LEVEL` 配置** —— 事故中提高日志级别不应该需要重新构建。
7. **`corsPreflightMaxAge` 是常量不是配置** —— 允许的方法与头在编译期固定，
   运维没有可调的东西。

### 优雅退出的顺序

这是 Phase 2 里唯一一处"顺序错了也能跑，但会在滚动发布时丢请求"的代码：

```
1. /ready 返回 503        ← 负载均衡器需要时间发现
2. 等待 ReadinessDrainDelay（默认 5s）
3. 停止接受新连接，排空在途请求
4. 关闭依赖（pool、redis）
```

第 1、2 步是最常被跳过的，也正是"graceful shutdown 明明做了、
发布时还是掉请求"的原因。`cmd/api/main_test.go` 显式验证了
"仍在服务但 `/ready` 已经 503"这个窗口真实存在。

### 质量门禁执行结果

```
gofmt -l .                                      无输出
go vet ./...  /  go vet -tags=integration ./... OK
go build ./...                                  OK
go test ./...                                   全部 ok
go test -race ./...                             全部 ok
golangci-lint run                               0 issues
golangci-lint run --build-tags=integration      0 issues
govulncheck ./...                               No vulnerabilities found
go test -race -tags=integration -count=1 ./...  全部 ok
sqlc generate / sqlc diff                       干净
```

govulncheck 首次扫描命中一个**真实漏洞**：`golang.org/x/text` 的
`norm.Form.Transform`，由 `pgxpool.NewWithConfig` 的调用链触发。
升级到 v0.41.0 后复扫为零。这条记在这里是因为它说明门禁不是形式——
它在第一次运行时就拦下了一个真实问题。

### 测试覆盖

**单元**：config 校验（含示例密钥拦截、`Secret` 在 fmt / slog / json
四条路径上均不泄漏）、`apperr` 零值与 Unwrap 链、validate、
Kind→status 穷尽性、RequestID / Recovery / Timeout / CORS / BodyLimit、
错误响应脱敏、health / live / ready 三种语义、draining。

**集成**（真实 PostgreSQL 17，testcontainers）：`WithTx` 的提交 / 回滚 /
panic 回滚 / 嵌套复用四种语义、`Querier` 回退到池、`HealthCheck` 执行真实 SQL、
迁移生效、`run()` 的真实启停与 drain 窗口。

集成测试用 `integration` build tag 隔离，`go test ./...` 不需要 Docker。


---

## Phase 3：用户、认证、RBAC、限流、OpenAPI

一次交付九个端点、两张表、四个限流 scope 和一份 OpenAPI 文档。
之所以捆在一起，是因为拆开会留下真实的安全缺口：Argon2id 每次校验分配
64 MB，没有登录限流就是一个开放的内存 DoS 入口；OpenAPI 拖到最后写，
写出来的就是事后补的文档而不是接口契约。

### 依赖方向：user → auth，且 auth 永不回看

`auth` 需要知道"某个邮箱对应的密码哈希和角色"，最省事的写法是 import `user`。
那会立刻形成环：`user` 的授权判断又需要 `auth` 的权限表。

实际做法是 `auth` 自己声明它需要的最小形状：

```go
// internal/auth/account.go
type Account struct {
    UserID       string
    PasswordHash string
    Role         Role
    Active       bool
}

type AccountStore interface {
    FindByEmail(ctx context.Context, email string) (Account, error)
    FindByID(ctx context.Context, userID string) (Account, error)
    UpdatePasswordHash(ctx context.Context, userID, hash string) error
}
```

适配器 `user.AccountStore` 放在 `user` 一侧。`Role` 归 `auth` 所有，
`user.User.Role` 的类型直接就是 `auth.Role`——两边各定义一份，
就会得到两张需要手工同步的权限表，而它们迟早不同步。

### 登录：三种失败必须无法区分

邮箱不存在、密码错误、账号被禁用，返回同一个 401、同一个 code、同一段文案。
只做到这里还不够——耗时也必须一致。邮箱不存在时如果直接返回，
响应会比"跑完一次 Argon2id"快两个数量级，那就是一个计时侧信道。

所以 `Argon2Hasher` 在构造时用随机 32 字节算了一个假哈希，
账号不存在的路径去校验这个假哈希：

```go
// 账号不存在时仍然付出一次真实的哈希代价
h.VerifyDummy()
```

`runtime.KeepAlive` 防止编译器把这次计算优化掉。

### Refresh：轮转 + 家族撤销

每次 refresh 都签发新的一对，并作废旧的。旧 token 再次出现，
只有一种解释：它被复制过。这时撤销的不是那一个 session，而是整个 family——
合法客户端也会被登出。这是有意从严：无法区分"被盗"和"客户端重试"，
而放过的代价是攻击者手里那条链继续有效。

整个流程在一个事务里，且 `GetByTokenHash` 带 `FOR UPDATE`。
没有行锁，两个并发 refresh 会双双成功，重放检测就是摆设。

刷新令牌本身不入库，只存 SHA-256 摘要。集成测试直接查库确认：

```sql
SELECT count(*) FROM refresh_sessions WHERE encode(token_hash, 'escape') = $1
```

### 授权分两半

能不能做这类事，是角色问题，在中间件里判；能不能动这一条记录，
要先知道这条记录属于谁，只能在 Service 里判。

```go
// 列表、创建、删除没有自助路径，在路由上就拦掉
r.With(requirePermission(auth.PermUserCreate)).Post("/", h.Create)
// 读取和更新有自助路径，交给 Service 的 AuthorizeSelfOrPermission
r.Get("/{id}", h.Get)
r.Patch("/{id}", h.Update)
```

其中 PATCH 有个必须防的洞：本人可以改自己的 `name`，但不能改自己的
`role` 和 `status`——否则每个账号距离管理员只有一个请求。
`user.Patch.ChangesPrivileges()` 把这两个字段单独拎出来要求 `user:update`。

### 限流：四个 scope，两种失败策略

| scope | 何时可判定 | 后端故障时 |
|-------|-----------|-----------|
| `ip` | 立刻 | fail-open |
| `user` | 认证之后 | fail-open |
| `login_ip` | 读完 body | **fail-closed** |
| `login_id` | 读完 body | **fail-closed** |

通用流量 fail-open，因为限流是保护措施不是功能，Redis 抖一下就让整站
503，是拿一次真实故障换一次假想故障。登录 fail-closed，因为放开无限次尝试
恰恰发生在没人能观察到的时候，而且那 64 MB 一次的开销会失去上限。

登录同时按 IP 和按账号限。只按 IP，僵尸网络把对一个账号的尝试摊到几千个地址上
就绕过了；只按账号，攻击者故意打错某人的密码就能定向拒绝服务。两者互相补洞。

账号那一维的 key 是邮箱的 SHA-256：Redis 的 key 会出现在 MONITOR 输出、
慢日志和 RDB 快照里，邮箱是个人数据。

`TRUSTED_PROXY_COUNT` 默认 0，即完全忽略 `X-Forwarded-For`。
这个头是请求头，客户端想写什么写什么；不配置代理层数就信它，
等于把限流配额和审计地址一起交给攻击者。配置为 N 时，从右往左数第 N+1 个才是可信值。

### OpenAPI：让文档被测试强制

手写的 spec 会在几周内漂移。所以 `registerRoutes` 被拆成一个不需要数据库、
不需要 Redis、不需要签名密钥就能构造的独立函数，然后测试遍历真实路由树，
与 `api/openapi.yaml` **双向**比对：

```
route %s is served but absent from api/openapi.yaml
api/openapi.yaml documents %s, which no route serves
```

只查一个方向没有意义：漏写会被发现，删掉端点却忘了删文档不会。
这个断言本身也做了变异验证——往 spec 里塞一个不存在的路径，测试确实失败。

`npx @redocly/cli lint` 结果为 valid，余下 4 条 warning
（servers 用 localhost、三个探针没有 4xx）是有意保留的。
该命令没有进 `make check`：`check` 必须能在离线环境里跑完。

### 质量门禁执行结果

```
go fmt ./...                       通过
go vet ./...                       通过
golangci-lint run                  0 issues
go test -race ./...                全部通过（15 个包）
govulncheck ./...                  0 vulnerabilities
sqlc diff                          无差异
go test -tags integration ./...    全部通过（PostgreSQL 17 容器）
npx @redocly/cli lint              valid，4 warnings
```

途中被 lint 挡下来的真实问题：gosec G115 报了四处整数溢出转换，
都在把 `int` 交给 `int32` 参数的地方——修法是加显式上界
（`user.List` 的 `maxOffset = 1<<30`、`decodePHC` 的 `maxComponentLen = 1024`），
而不是加 `//nolint`。

### 测试覆盖

**单元**：Argon2 的 PHC 编解码与 7 种畸形哈希、rehash 检测、权限表穷尽、
`Permissions()` 不返回内部切片的别名、登录三路径不可区分、refresh 重放撤族、
JWT 的 `alg=none` 攻击与他方密钥、限流的 fail-open/fail-closed/降级配额、
XFF 伪造防护（含 IPv4-mapped 归一化，否则同一客户端能拿到双份配额）。

**HTTP**：经真实 chi 路由跑 401/403/404/409/400/204/201，
断言 `Location` 头、空列表序列化成 `[]` 而非 `null`、
响应体在字节层面不含 `password`、403 不点名缺少哪个权限、
登录响应不带 `X-RateLimit-Remaining`（那是给攻击者的剩余次数计数器）。

**集成**（真实 PostgreSQL 17）：唯一约束与 CHECK 约束的错误码翻译、
NotFound 的四条路径、同毫秒插入下的分页稳定性、PATCH 不动未提及的列、
family 撤销不波及其他 family、用户删除级联清理 session，
以及两个不同仓储在同一个 context 事务里写入后一起回滚。

最后一条是 ADR-0005 那个"tx 藏在 context 里"决定的验收：
只要有任何一个仓储偷偷去拿连接池，回滚后就会留下一行。

---

## 当前仓库结构

```
.
├── prompts/            8 阶段开发 prompt（阶段零产出）
├── docs/
│   ├── adr/            0001 ~ 0009
│   ├── architecture.md 未进 ADR 的设计
│   └── progress.md     本文件
├── STATE.md            阶段间的唯一上下文载体
├── cmd/api/            main.go  run.go  features.go  main_test.go  openapi_test.go
├── internal/
│   ├── apperr/  validate/  config/  telemetry/
│   ├── postgres/       pool tx health + db/(sqlc) + pgtest/
│   ├── redis/
│   ├── httpx/          errmap response middleware server health router clientip
│   ├── user/           user service + userpg/ + userhttp/
│   ├── auth/           policy principal password token account service
│   │                   + authjwt/ + authpg/ + authhttp/
│   └── ratelimit/      core redis local guard + ratelimithttp/
├── api/openapi.yaml    12 个 operation，与路由树双向校验
├── migrations/         000001_baseline  000002_users  000003_refresh_sessions
├── queries/            system.sql  user.sql  auth.sql
├── .golangci.yml       含 depguard 依赖方向规则
├── .env.example        与 loader 由测试保证不漂移
├── sqlc.yaml
└── Makefile            工具版本全部固定
```

---

## 在本机跑起来

```bash
# 1. 安装固定版本的工具链
make tools

# 2. 准备配置。AUTH_JWT_SECRET 没有默认值：
#    一个内置密钥的服务就是一个密钥公开的服务
cp .env.example .env

# 3. 起 PostgreSQL，然后应用迁移
make migrate-up

# 4. 跑起来
make run

# 5. 全部门禁（CI 跑的就是这一条）
make check

# 6. 需要 Docker 的集成测试
make test-integration
```

`make help` 列出全部目标。

---

## 下一步

**Phase 4（测试与质量）**。重点不是把覆盖率数字推高，而是补上还没有被断言的行为：

1. `Guard` 降级路径的并发测试——目前只验证了单线程下的配额。
2. 端到端流程测试：注册→登录→带 token 访问→refresh→logout→旧 token 失效。
3. 把 `make openapi-lint` 与 `make test-integration` 接进 CI（Phase 8 落地）。

有一条现在已经是自动强制的：**新增或修改端点必须同步 `api/openapi.yaml`**，
否则 `cmd/api/openapi_test.go` 会从两个方向同时失败。

后续阶段：Phase 5 可观测性 → Phase 6 基础设施（Docker / K8s / CI）
→ Phase 7 消息与 outbox → Phase 8 文档与审计。
