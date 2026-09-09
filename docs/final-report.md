# Enterprise Go Scaffold — 终审报告

> Phase 8 的交付物。日期:2026-09-09。
> 本文的每一条数字与结论都来自实际执行,命令输出见第 19 节;未执行的项标注
> `SKIPPED` 并写明原因(§0.5)。
> 当前状态以 [`../STATE.md`](../STATE.md) 为准;本文与它冲突时以 `STATE.md` 为准。

## 0. 一句话结论

**可以拿去开新服务。** 12 221 行生产代码、13 632 行测试代码、32 个包、
10 份 ADR;`make check`、`go test -race`、`make test-integration` 全绿;
`goscaffold` 生成的项目能编译、能通过竞态测试、能起来并完成登录与建用户。

**不要在两件事上误判它**:manifest 从未在真实集群 apply 过;
`notify.LogSender` 只把邮件写进日志。

## 1. Architecture

三层同心,依赖单向朝内:

```text
cmd/            组装(手工 DI,可读的装配顺序)
  ↓
internal/<feature>/<feature>http/    HTTP 适配器 → import chi
internal/<feature>/<feature>pg/      存储适配器 → import pgx
  ↓
internal/<feature>/                  领域 + service + 端口定义 → 不 import 任何 I/O
```

关键选择,每条都有 ADR:

- **每个业务能力三个包**,service 包声明它需要的接口,适配器实现。
  `internal/user` 不知道 HTTP 和 PostgreSQL 的存在
- **依赖方向由 depguard 编译期强制**,不是靠文档
- **错误模型两个维度**:`Kind`(粗分类,映射到 HTTP 状态)+ `Code`(稳定的
  机器可读标识)。`Kind` 零值 = Internal,忘记分类的结果是 500 + 脱敏,
  而不是意外泄漏
- **事务藏在未导出的 context key 背后**,`pgx.Tx` 不出现在任何业务签名里
- **成功响应无 envelope**(状态码就是 envelope),失败统一包装

否决清单(ADR-0002):domain/application/infrastructure 分层、
`BaseService`/`GenericRepository[T]`、platform/pkg/common/utils 包、
DI 容器、init() 隐式初始化。

## 2. Tech Stack

| 层 | 选择 | 直接依赖 |
|---|---|---|
| HTTP | `net/http` + chi v5 | ✓ |
| 数据库 | PostgreSQL 17 / pgx v5 / sqlc | ✓ |
| 缓存与限流 | Redis / go-redis v9 | ✓ |
| 消息 | NATS JetStream(默认关) | ✓ |
| 日志 | `log/slog`(标准库) | — |
| 追踪与指标 | OpenTelemetry + Prometheus | ✓ |
| 认证 | golang-jwt v5 + `golang.org/x/crypto`(Argon2id) | ✓ |
| 迁移 | golang-migrate v4 | ✓ |
| 配置 | 手写 loader + godotenv 兜底 | ✓ |

明确否决:Viper、validator、GORM、zap、DI 框架、Cobra(ADR-0001 / Phase 7)。

## 3. Directory Structure

```text
cmd/           api / worker / adminctl / goscaffold
internal/      apperr config validate httpx postgres redis auth user
               ratelimit messaging telemetry notify buildinfo scaffold
api/           openapi.yaml + bruno 调试集合
migrations/    4 条,up/down 齐全
queries/       sqlc 源查询
deployments/   observability/ + k8s/(base + 4 overlay)
docs/          adr/(10) architecture progress development deployment security
test/blackbox/ 对运行中服务的对抗性 HTTP 测试
prompts/       本仓库据以产出的分阶段契约
```

32 个包。**没有** utils / common / helpers / base / pkg。

## 4. APIs

12 个 operation,基础路径 `/api/v1`,契约在 `api/openapi.yaml`。

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/auth/login` | 公开 |
| POST | `/auth/refresh` | refresh token |
| POST | `/auth/logout` | 公开(必然,access token 可能已过期) |
| POST | `/users` | `user:create` |
| GET | `/users` | `user:list` |
| GET | `/users/{id}` | `user:read` 或本人 |
| PATCH | `/users/{id}` | `user:update` 或本人 |
| DELETE | `/users/{id}` | `user:delete` |
| PATCH | `/users/{id}/password` | `user:update` 或本人 |
| GET | `/health` `/live` `/ready` | 公开,不带版本前缀 |

**路由与规格双向一致由 `cmd/api/openapi_test.go` 强制**:未文档化的路由、
指向不存在路由的规格条目,两个方向都失败。响应体 schema 由
`internal/openapitest` 校验。

## 5. Authentication & Authorization

Argon2id(64 MiB/t=3/p=4)+ JWT HS256 access(15min)+ 不透明 refresh(30d,
只存 SHA-256 摘要,每次使用即轮换,带家族盗用检测,撤销先提交再拒绝)。

登录的三种失败 —— 邮箱不存在、密码错误、账号禁用 —— 返回**完全相同的 401**
且耗时相同。授权是权限而非角色:两个角色、五个权限、一张策略表,未登记的权限
永不授予。

详见 [`security.md`](security.md)。

## 6. Rate Limiting

Redis Lua 令牌桶,四个 scope。**通用 fail-open**(降级到进程内兜底器),
**登录 fail-closed**(503)。登录同时按 IP 与邮箱哈希两个维度限,两个都要过。
登录响应剥掉 `X-RateLimit-*` 头。

限流拆成两个中间件,分列认证前后 —— 按用户限流必须在知道用户是谁之后。

## 7. Database

pgx 连接池 + sqlc 生成查询。schema 的唯一来源是 `migrations/` 目录本身。

审计结果:

- **没有 `SELECT *`**,列名一律显式
- 所有出现在 WHERE 中的列都有索引,一个例外:`users.role`(仅
  `cmd/adminctl` 在引导时查一次,当时表是空的)
- **没有 N+1**:列表查询是单条 SQL,`Count` 与 `List` 各一次往返
- 分页有上界(`MaxPageSize=100`)且 offset 有硬上限(`1<<30`),int32 转换
  因此可证安全
- 事务通过 `TxManager.WithTx` 传递,不支持嵌套;连接释放由 `defer` 保证
- 每条迁移都有 `.down.sql`

## 8. Cache

Redis 的用途主要是**限流存储**而非缓存(ADR-0001 已说明这个命名张力)。
`REDIS_REQUIRED=false` 时,Redis 故障只让服务降级,不把它从负载均衡里摘掉。

## 9. Messaging

默认关闭,而这是设计的一部分:`MESSAGING_ENABLED=false` 时不写 outbox 行、
不需要 broker、不需要 worker 进程。

事务性 outbox → NATS JetStream → 消费者,at-least-once + `processed_events`
去重日志。指数退避重试,DLQ 是**数据库表**(可查询、可重放,而不是一个没人看
的队列)。`MESSAGING_ACK_WAIT` 必须大于整个重试预算,启动时校验。

审计:消息丢失路径已封闭(事件与业务变更同事务);重复消费由去重日志兜住;
consumer 生命周期绑 context,优雅退出会等待 in-flight handler。

## 10. Observability

`internal/telemetry` 是**唯一** import OTel SDK 或 Prometheus 客户端的包
(已用 `go list` 验证);业务包只声明小的 `Observer` 接口。

- 一条 trace 贯穿 HTTP → service → PostgreSQL → Redis → NATS → worker
- 每条日志带 `request_id` 与 `trace_id`,从 context 自动注入
- 指标在**独立监听器**导出;标签用路由模板(chi `RoutePattern`),基数有界
- DB/缓存 span 走属性白名单
- Grafana 面板引用的指标由 `dashboard_test.go` 守着

## 11. Testing

| 层次 | 内容 |
|---|---|
| 单元 | 24 个包,`go test -race` 全绿 |
| 集成 | testcontainers + 真实 PostgreSQL 17,`integration` 构建标签 |
| 黑盒 | `test/blackbox/`,对运行中服务的对抗性 HTTP 测试,曾找出 5 个真实缺陷 |
| 守卫 | errmap / env_example / openapi 双向 / 响应 schema / dashboard / k8s manifests / bruno 集合 / 模板漂移 |

**守卫测试全部做过变异验证**,每条都记录了让它变红的具体变异。这是本仓库
反复强调的一点:没见它红过的测试不算数。

## 12. Docker

一份 Dockerfile 三个 target,共享 build stage:

| Target | 基础镜像 | 大小 |
|---|---|---|
| api | `gcr.io/distroless/static:nonroot` | 40.8 MB |
| worker | 同上 | 35.1 MB |
| migrate | `migrate/migrate:v4.19.1` + 本仓库 SQL | 85.5 MB |

非 root(65532)、无 shell、CA 证书齐备。版本信息经 ldflags 注入
`internal/buildinfo`。

## 13. Kubernetes

Kustomize,base + 4 overlay(local/development/staging/production),
生产渲染出 9 个对象。四项承重设置有守卫测试:关停预算余量 ≥10s、
探针路径、securityContext 四项、镜像 tag 不可变。

## 14. CI/CD

三个 workflow:`ci.yml`(三 job,按耗时拆分)、`scaffold.yml`(生成项目并
编译测试)、`docker.yml`(发布 ghcr.io)。全部在 GitHub Runner 上实际跑绿。

## 15. Scaffold CLI

`goscaffold new / module / migration / version`,标准库 `flag`。

**模板漂移的解法是:代码只有一份。** `templates/project/` 的 220 个文件是本
仓库的派生副本,由 `make templates` 生成、由测试守住,与 `make sqlc-check`
同一个机制,经三个变异验证。模块模板无法派生,由 `scaffold.yml` 编译验证。

生成器**绝不覆盖已有文件**(两阶段 + `O_EXCL`),生成的端点在权限注册前
一律 403。

## 16. Security

见 [`security.md`](security.md)。审计要点:

- 全仓库 grep 不到把 password / token / secret 写进日志的路径
- `internal/httpx/leak_test.go` 端到端扫描:一个请求携带每一类密钥,过完整条
  中间件栈,在日志与 span 里逐个搜索
- 仓库里没有任何真实凭据(见第 21 节的例外记录)
- 容器加固六项齐全
- `govulncheck` 零漏洞

## 17. Performance

只查明显问题(§8.15 明确要求不要过早优化):

| 检查项 | 结果 |
|---|---|
| unbounded allocation | 无。请求体 1 MiB 上限,分页有上界 |
| large body | `HTTP_MAX_BODY_BYTES` 强制 |
| unbounded pagination | `MaxPageSize=100`,offset ≤ `1<<30` |
| bad DB query | 无 `SELECT *`,无 N+1,WHERE 列有索引 |
| connection pool | 池大小/生命周期/健康检查周期均可配 |
| excessive JSON conversion | DTO 与实体各一次转换,没有中间层 |
| high-cardinality metrics | 标签用路由模板,有守卫测试 |

已知的性能取舍:offset 分页在十万行后深翻页会劣化(ADR-0004 记录);
Argon2id 每次登录分配 64 MiB(有意为之,靠登录限流约束)。

## 18. Dependency Review

18 个直接依赖(门槛 ≤20)。逐个的存在理由:

| 依赖 | 为什么存在 | 标准库能替代吗 |
|---|---|---|
| `go-chi/chi/v5` | 路由与 URL 参数;`net/http` 的 ServeMux 不支持中间件链与路由模板标签 | 部分能,代价是自己写路由树 |
| `golang-jwt/jwt/v5` | JWT 签名与校验 | 不能,自己实现 JWT 是典型的高风险自造 |
| `golang-migrate/migrate/v4` | 迁移执行与版本管理 | 不能 |
| `google/uuid` | UUIDv7 生成 | 不能(标准库无 UUID) |
| `jackc/pgx/v5` | PostgreSQL 驱动与连接池 | `database/sql` 能,但会失去 pgx 的类型映射与 COPY |
| `joho/godotenv` | `.env` 兜底加载 | 能,约 50 行;保留是因为它只在开发路径 |
| `nats-io/nats.go` | JetStream 客户端 | 不能 |
| `prometheus/client_golang` | 指标注册与导出 | 不能 |
| `prometheus/client_model` | **仅测试用**:断言指标形状 | 不能 |
| `redis/go-redis/v9` | Redis 客户端与 Lua 执行 | 不能 |
| `testcontainers-go/modules/postgres` | **仅测试用**:真实 PG 集成测试 | 不能 |
| `otel` ×5 | 追踪 SDK 与 OTLP 导出 | 不能 |
| `golang.org/x/crypto` | Argon2id | 不能 |
| `gopkg.in/yaml.v3` | 解析 openapi.yaml 与 k8s manifest(守卫测试) | 不能 |

**没有一个依赖是"因为流行"引入的。** 两个是测试专用,不进生产二进制。

## 19. Commands Executed

```text
$ make check
0 issues.
No vulnerabilities found.
EXIT=0                      # fmt vet lint test-race vuln sqlc-check

$ go test -race ./...
EXIT=0                      # 24 个包 ok

$ make test-integration
EXIT=0                      # 27 个包 ok(真实 PostgreSQL 17)

$ go test -cover  (核心包)
internal/apperr           78.4%
internal/validate        100.0%
internal/httpx            93.5%
internal/auth             82.2%
internal/auth/authhttp    92.4%    ← 本阶段从 42.4% 提升
internal/auth/authjwt     94.3%
internal/user             73.3%
internal/user/userhttp    77.3%
internal/ratelimit        77.7%
internal/messaging        88.5%

$ docker build --target {api,worker,migrate}
api 40.8MB / worker 35.1MB / migrate 85.5MB      全部成功

$ docker compose --profile full config
OK (9 services)

$ kubectl kustomize overlays/{local,development,staging,production} | kubeconform -strict
Valid: 8 / 8 / 9 / 9,Invalid: 0

$ actionlint -shellcheck ...
OK

$ goscaffold new final-test → go mod tidy / build / vet / test / test -race
22 个包 ok(含 -race);服务启动后 /live /ready 200,
login 200(Cache-Control: no-store)、GET /users 200、无 token 401、POST /users 201

$ README Quick Start
docker compose up -d && make migrate-up && make run
/live  → {"status":"ok",...} [200]
/ready → {"status":"ok",...,"components":{"postgres":"ok","redis":"ok"}} [200]
```

**SKIPPED**:`kubectl apply` 到真实集群 —— 无可用集群。
`kubectl apply --dry-run=client` 也用不了(它要从集群下载 OpenAPI schema),
改用 `kubeconform -strict` 离线校验官方 1.33 schema。

## 20. Final Verification Gate

| # | 条件 | 结果 | 证据 |
|---|---|---|---|
| 1 | golangci-lint 零告警 | ✅ | `0 issues.`;14 处 `nolint` 全部带行内理由,见下 |
| 2 | govulncheck 零 HIGH/CRITICAL | ✅ | `No vulnerabilities found.` |
| 3 | `go test -race` 全过 | ✅ | EXIT=0,24 包 |
| 4 | 核心包覆盖率 ≥70% | ✅ | 最低 73.3%(`internal/user`),见第 19 节 |
| 5 | 直接依赖 ≤20 且各有理由 | ✅ | 18 个,见第 18 节 |
| 6 | 无 global mutable state | ✅ | 仅 9 个包级 var:全部未导出、仅初始化时写入;策略表的访问器返回副本 |
| 7 | 无 password/token 进日志 | ✅ | grep 无命中 + `leak_test.go` 端到端扫描 |
| 8 | 路由全部在 openapi.yaml | ✅ | `openapi_test.go` 双向断言通过 |
| 9 | goscaffold 生成项目可 build+test | ✅ | 22 包,含 `-race` |
| 10 | README Quick Start 实际走通 | ✅ | 见第 19 节 |
| 11 | STATE.md 与代码一致 | ✅ | 本阶段逐节核对并更新 |

**14 处 `nolint` 的分类**(§8.18 要求逐条说明):

- 6 处 `gosec` G304/G301/G204:生成器读写**调用方指定的**路径,这正是它的
  职责;路径来源在同一函数内可见
- 4 处 `gosec` G115(整数转换):每处上方都有把值限定在范围内的检查
  (`MaxPageSize`、`maxComponentLen`、重试上限)
- 1 处 `gosec` G404:退避抖动用 `math/rand/v2`,不是密钥
- 2 处测试中的 G304:测试读自己包内的固定路径
- 1 处 `gosec` 文件权限:模板不是密钥

没有一处 `nolint` 是用来掩盖真实问题的;没有使用 `//nolint:all` 或裸 `nolint`。

## 21. Known Limitations

**设计上的取舍**(每条在对应 ADR 中论证):

| 取舍 | 出处 |
|---|---|
| access token 撤销/改角色最长 15 分钟延迟 | ADR-0007 |
| 通用限流 fail-open:Redis 故障期防护真空 | ADR-0008 |
| tx 经 context 隐式传递,与"Explicit > Magic"有张力 | ADR-0005 |
| offset 分页并发插入下可能重复/跳过;深翻页劣化 | ADR-0004 |
| 外部副作用无法 exactly-once | ADR-0009 |
| UUIDv7 暴露创建时间(毫秒级) | ADR-0006 |
| Redis 是 v1 硬依赖(因限流) | ADR-0001/0008 |
| outbox 引入至少一个轮询周期的延迟 | ADR-0009 |

**尚未实现 / 未验证**:

- **manifest 从未在真实集群 apply 过**。schema 校验抓不到调度、准入控制器、
  CNI 层面的问题
- **`notify.LogSender` 只把邮件写进日志**。整条链路是真的且有测试,唯独背后
  没有接任何邮件服务商 —— **不要在认为它能发信的前提下部署**
- 没有自助注册(范围决定)
- **请求体**尚无 OpenAPI schema 自动校验(响应体已有)
- 没有 NetworkPolicy / ServiceMonitor / PrometheusRule / 镜像扫描 / SBOM
- 复数规则只覆盖三条英语规则(`person→persons` 是错的)
- `make run` 用 `go run`,不注入 ldflags,所以开发运行报告 `dev`/`unknown`
  —— 这正是 buildinfo 默认值想表达的意思,不是缺陷

**一次真实的凭据泄漏**(Phase 7,已修复):模板生成按目录纳入 `api/bruno`,
复制时加的 `.tmpl` 后缀让 `.gitignore` 失效,一个被忽略的凭据文件进了提交并
推送到公开仓库。根因已修(模板只收 `git ls-files` 跟踪的文件),历史已改写,
**但该凭据必须视为已泄露并轮换**。完整记录在 `STATE.md`。

## 22. Recommended Future Work

按价值排序:

1. **在真实集群上跑一次**。这是当前最大的未知,其他所有验证都到此为止。
2. **接一个邮件服务商**,把 `notify` 从日志改成真实投递。链路已经完整。
3. **请求体 schema 校验**。响应体已有,请求体是目前唯一没有自动校验的一侧;
   它需要一套 `validate` 规则到 JSON Schema 的映射。
4. **HPA 换成自定义指标**。API 用 RPS 或 P95 延迟,worker 用 NATS consumer
   lag —— CPU 对 worker 几乎永远是错的指标(积压时它在等 I/O)。
5. **NetworkPolicy 与 ServiceMonitor**。前者让"9464 不进 Service"这个决定
   真正闭环,后者是抓 Pod IP 的标准做法。
6. **把 depguard 的业务包规则从枚举改成可推导的形式**。现在新增模块必须手工
   加进 `.golangci.yml`,漏了就静默失去保护;`goscaffold` 会提醒,但那是
   提醒不是强制。
7. **`docs/` 的英文化**。README 是双语的,而 10 份 ADR 与 architecture.md 是
   中文 —— 对英文读者,深层设计文档目前是不可读的。
