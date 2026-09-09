<h1 align="center">goscaffoldforlevi</h1>

<p align="center">
  一套面向生产的 Go 服务脚手架 —— 每一条约定都由构建强制，而不是靠文档自觉。
</p>

<p align="center">
  <a href="https://github.com/namezzy/goscaffoldforlevi/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/namezzy/goscaffoldforlevi/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27%2B-00ADD8?logo=go&logoColor=white">
  <img alt="Tests" src="https://img.shields.io/badge/tests-21%20packages%20green-3fb950">
  <img alt="Test ratio" src="https://img.shields.io/badge/test%20code-53%25-3fb950">
  <img alt="ADRs" src="https://img.shields.io/badge/ADRs-10-8957e5">
  <img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue">
</p>

<p align="center">
  <a href="README.md">English</a> · <strong>简体中文</strong>
</p>

---

## 这是什么

一个 Go HTTP 服务骨架：用户、认证、RBAC、限流、基于 outbox 的消息链路，以及
完整的 OpenTelemetry 接线。它存在的理由是让一个真实项目**第一天就能开工**，
而不是把头一个月花在反复争论同样的十个决策上。

它刻意**不是**框架。没有容器、没有注册表、除 `sqlc` 外没有代码生成、没有基类。
依赖在 `cmd/api/run.go` 里手工装配 —— 你能一行行读完它。

## 为什么还要再写一个脚手架

多数脚手架只给你一个目录结构，然后祈祷。这里有两点不同。

**每个决策都有落纸的理由。** [`docs/adr/`](docs/adr/) 里的十份 ADR 记录了选了
什么、否决了什么、代价是什么。每份都带一节 `When to Revisit`，且必须写成**可
观测的触发条件** —— 不能写"当需求变化时"，那是一个永远不会触发的复审条件。

**要紧的约定由机制强制。** 只写在文档里的规则会被违反，能让构建失败的规则不会。

| 约定 | 机制 | 违反时 |
|---|---|---|
| 依赖方向（ADR-0002） | `.golangci.yml` 的 depguard 规则 | `make lint` 失败 |
| 每个错误 `Kind` 对应唯一 HTTP 状态码 | `internal/httpx/errmap_test.go` | 新增 Kind 未登记 → 测试失败 |
| `.env.example` 与配置加载器不漂移 | `internal/config/env_example_test.go` | 键名不一致 → 测试失败 |
| 路由与 OpenAPI 规格**双向**一致 | `cmd/api/openapi_test.go` | 未文档化的路由 / 规格里的死条目 → 测试失败 |
| 响应体符合声明的 schema | `internal/openapitest` | 结构漂移 → 测试失败 |
| sqlc 生成物不过期 | `make sqlc-check`（`sqlc diff`） | CI 失败 |
| Grafana 面板引用的指标真实存在 | `deployments/observability/dashboard_test.go` | 指标改名 → 测试失败 |
| Bruno 集合覆盖全部 operation | `api/bruno/collection_test.go` | 缺失或悬空的请求 → 测试失败 |

其中依赖方向那条是承重的：`internal/user/*.go` 与 `internal/auth/*.go` 不得
import `net/http`、`chi`、`pgx`、`go-redis`、`nats.go`、`otel`。理由不是纯洁性
—— 一个 import 了 `net/http` 的 service 无法被 CLI 或消费者复用，一个 import 了
`pgx` 的 service 无法脱离数据库测试。

## 项目状态

按阶段构建，每个阶段有自己的进入条件与门禁。**以 [`STATE.md`](STATE.md) 为准。**

| 阶段 | 范围 | 状态 |
|---|---|---|
| 1 | 架构设计 —— 10 份 ADR、`docs/architecture.md`、依赖规则 | ✅ 完成 |
| 2 | 基础工程 —— 配置、日志、连接池、事务、HTTP、错误模型 | ✅ 完成 |
| 3 | 用户、认证、RBAC、限流、OpenAPI | ✅ 完成 |
| 4 | 消息 —— outbox、NATS JetStream、幂等、DLQ | ✅ 完成 |
| 5 | 可观测性 —— trace、metrics、日志关联、Grafana 栈 | ✅ 完成 |
| 6 | Dockerfile、CI、Kubernetes manifests | ✅ 完成 |
| 7 | `goscaffold` CLI | ✅ 完成 |
| 8 | 文档与终审 | ◻︎ 进行中 |

**部署前必读**：manifests 是**校验过的，不是跑过的**。它们背后还没有任何集群
——哪些验证真做了、哪些没做，见[验证状态](#验证状态)。

## 能力

**HTTP** · chi 路由 · 成功响应无 envelope，失败响应统一包装 · 每次解码都开
`DisallowUnknownFields` · 只提供 PATCH · offset 分页 · 每个响应带 `X-Request-ID`
· 优雅退出带就绪排空延迟，让负载均衡器先摘流量、监听器后关闭。

**持久化** · pgx 连接池 · `sqlc` 生成查询 · `golang-migrate` 迁移，**不在 Pod
启动时自动执行** · `TxManager.WithTx(ctx, fn)`，事务藏在未导出的 context key
背后，`pgx.Tx` 不出现在任何业务签名里 · UUIDv7 应用侧生成。

**认证** · Argon2id，64 MiB / t=3 / p=4 · JWT HS256 access token（15 分钟）·
不透明 refresh token（30 天），库里只存 SHA-256 摘要 · refresh token **家族
盗用检测** · 登录的三种失败以相同的 401、相同的耗时返回。

**授权** · 两个角色、一张策略表、五个权限。Handler 申请的是权限，不是角色。

**限流** · Redis Lua 令牌桶，配额跨副本共享 · 通用 scope **fail-open**（降级到
本地兜底限流器），登录 scope **fail-closed**（503）· 拆成两个中间件，分列认证
前后。

**消息**（默认关闭）· 事务性 outbox · NATS JetStream · at-least-once 投递 +
`processed_events` 去重日志 · 指数退避 · 数据库表实现的 DLQ，支持重放。

**可观测性** · OpenTelemetry 贯穿 HTTP、PostgreSQL、Redis、NATS · Prometheus
指标在独立监听器上导出 · 从 context 自动把 `trace_id` 注入每条日志 · DB 与缓存
span 走属性白名单，任何查询参数都不会进入 span。

## 架构

每个业务能力三个包，依赖方向单向：

```
internal/user/            领域类型、service、端口定义        ← 不 import 任何 I/O
internal/user/userpg/     PostgreSQL 适配器                  → import pgx
internal/user/userhttp/   HTTP 适配器                        → import chi
```

service 包声明它需要的接口，适配器去实现。`internal/user` 里的任何东西都不知道
HTTP 和 PostgreSQL 的存在。

中间件顺序是固定的，其中两个位置是**刻意反直觉**的：

```
Recovery → RequestID → Tracing → Logging → Metrics → SecurityHeaders
  → CORS → BodyLimit → Timeout → RateLimitByIP
  → Authentication → RateLimitByUser → Authorization → Handler
```

- **CORS 在限流之前** —— 否则浏览器的 preflight 会消耗用户配额。
- **Metrics 在限流之前** —— 否则 429 永远不会被统计，而限流开始咬人时，429 曲线正是你唯一想看的东西。

逐条理由（一个位置一条）在 [`docs/architecture.md`](docs/architecture.md)。

## 目录结构

```
cmd/
  api/                 HTTP 服务；main.go 只决定退出码，run.go 负责装配
  worker/              outbox relay + 事件消费者
  adminctl/            创建第一个管理员，此后拒绝再次运行
internal/
  apperr/              Kind + Code 错误模型；Kind 零值 = Internal
  config/              env > .env > 默认值；Secret 类型；累积式 Validate()
  validate/            只做语法校验 —— 不含任何业务规则
  httpx/               路由、中间件、唯一的 Kind→status 映射表
  postgres/            连接池、TxManager、健康检查、tracing
  redis/               客户端、tracing
  auth/                service · authjwt · authpg · authhttp
  user/                service · userpg · userhttp
  ratelimit/           令牌桶 · guard · 本地兜底 · HTTP 中间件
  messaging/           publisher · outbox relay · consumer · DLQ
  telemetry/           唯一允许 import OTel SDK 与 Prometheus 的包
  notify/              通知发送（当前落到日志，见下方警告）
api/
  openapi.yaml         契约；与路由双向强制一致
  bruno/               24 个请求的调试集合（不是测试套件）
migrations/            golang-migrate 的 up / down SQL
queries/               sqlc 的源查询
cmd/goscaffold/        CLI：new、module、migration
internal/scaffold/     生成器，以及它内嵌的模板
Dockerfile             三个 target：api、worker、migrate
.github/workflows/     CI 门禁、镜像发布、生成项目校验
deployments/
  observability/       OTel Collector、Prometheus、Tempo、Grafana 及面板守卫测试
  k8s/                 Kustomize base + 四个 overlay，带守卫测试
test/blackbox/         对运行中的服务发起的对抗性 HTTP 测试
docs/                  architecture.md · adr/ · progress.md
prompts/               本仓库据以产出的分阶段执行契约
```

## 快速开始

需要 Go 1.27+ 与 Docker。以下流程已在 macOS + OrbStack 上实跑验证。

```sh
git clone https://github.com/namezzy/goscaffoldforlevi.git
cd goscaffoldforlevi

cp .env.example .env      # 所有默认值开箱可用，什么都不改就能起
make tools                # 安装固定版本的 golangci-lint / sqlc / migrate / govulncheck
docker compose up -d      # postgres + redis
make migrate-up
make run
```

然后：

```console
$ curl -s localhost:8080/live
{"status":"ok","version":"dev"}

$ curl -s localhost:8080/ready
{"status":"ok","version":"dev","components":{"postgres":"ok","redis":"ok"}}
```

`/live` 只要进程活着就返回 200；`/ready` 按名字逐个报告依赖状态，任一不可用返回
503，优雅退出期间返回 `draining` —— 让负载均衡器在监听器关闭前先把实例摘掉。

创建第一个管理员。API 无法自举，因为 `POST /users` 需要 `user:create` 权限：

```sh
make bootstrap-admin email=you@example.com
```

它会生成并打印一次密码，且在管理员已存在后拒绝再次运行。一个能在 API 之外铸造
特权账号的命令，是对授权与审计轨迹的长期旁路 —— 所以它在不再被需要的那一刻就
停止可用。

登录：

```sh
curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"<刚才打印的密码>"}'
```

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "…不透明字符串…",
  "token_type": "Bearer",
  "expires_in": 900
}
```

想交互式地摸接口，用 [`api/bruno/`](api/bruno/README.md)：集合里登录请求的脚本
会把 token 写进变量，你不必每十五分钟手工复制一次。

## 配置

优先级为**真实环境变量 → `.env` → 内置默认值**。每个键、它的默认值、以及这个
默认值背后的理由，都以注释写在 [`.env.example`](.env.example) 里，并有测试保证
它与加载器不漂移。

| 分组 | 前缀 | 说明 |
|---|---|---|
| 应用 | `APP_*`、`LOG_LEVEL` | `APP_ENV` 只接受 `development` / `staging` / `production` |
| HTTP | `HTTP_*` | `HTTP_WRITE_TIMEOUT` 必须大于 `HTTP_REQUEST_TIMEOUT`，否则客户端拿到的是连接重置而不是 504 |
| 数据库 | `DATABASE_*` | 连接池大小、生命周期、连接超时 |
| Redis | `REDIS_*` | `REDIS_REQUIRED=false` 让 Redis 故障只降级、不把实例从负载均衡里摘掉 |
| 认证 | `AUTH_*` | `APP_ENV=production` 时仍使用示例密钥，**启动直接失败** |
| 限流 | `RATELIMIT_*` | 每个 scope 的配额 / 周期 / 突发 |
| 消息 | `MESSAGING_*` | 默认关闭；`MESSAGING_ACK_WAIT` 必须大于整个重试预算，否则启动拒绝 |
| 可观测性 | `OTEL_*`、`METRICS_*` | 埋点始终编译在内，这些开关只控制数据是否离开进程 |

配置在启动时校验一次，并**一次性报出全部问题**，而不是遇到第一个就退出 ——
一次重启修一个配置错误是很糟糕的下午过法。

## API

基础路径 `/api/v1`。完整契约见 [`api/openapi.yaml`](api/openapi.yaml)。

| 方法 | 路径 | 所需权限 |
|---|---|---|
| `POST` | `/auth/login` | 公开 |
| `POST` | `/auth/refresh` | refresh token |
| `POST` | `/auth/logout` | 已认证 |
| `POST` | `/users` | `user:create` |
| `GET` | `/users` | `user:list` |
| `GET` | `/users/{id}` | `user:read`，或本人 |
| `PATCH` | `/users/{id}` | `user:update`，或本人 |
| `DELETE` | `/users/{id}` | `user:delete` |
| `PATCH` | `/users/{id}/password` | `user:update`，或本人 |
| `GET` | `/health` `/live` `/ready` | 公开，不带版本前缀 |

约定：成功响应直接返回资源本身 —— 状态码就是 envelope。失败一律走同一个包装：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "the request body is not valid",
    "details": [{ "field": "email", "reason": "must be a valid address" }]
  },
  "request_id": "01930f8e-0000-7000-8000-000000000001"
}
```

客户端应当基于 `code` 分支，它是稳定的；`message` 是给人看的散文，随时可能变。
`request_id` 是用户看到的现象与运维能查到的记录之间**唯一**的连接 —— 内部错误
除它之外什么都不返回。

更新只用 `PATCH`。不提供 `PUT`，因为全量替换会让每一个被省略的字段变成一次静默
删除。

## 安全

- **密码** —— Argon2id，64 MiB / t=3 / p=4（OWASP 推荐）。硬件允许时优先加内存：那是 GPU 攻击者无法用钱绕开的参数。每次登录都要分配这么多内存，这正是登录限流不可省的原因。
- **登录** —— 邮箱不存在、密码错误、账号被禁用，返回相同的 401、相同的 code、相同的耗时。任何差异都会把这个端点变成账号枚举预言机。
- **Refresh token** —— 不透明、高熵，库里只存 SHA-256 摘要，每次使用即轮换。出示一个已被轮换过的 token 会撤销整个家族，且**撤销先提交、再返回拒绝**。
- **限流** —— 登录同时按客户端地址与邮箱哈希两个维度限，两个都要过：前者挡住单一来源枚举大量账号，后者挡住大量来源打同一个账号。登录响应会剥掉 `X-RateLimit-*` 头 —— 在一次被拒的登录旁边显示剩余次数，等于给攻击者一个进度条。
- **可信代理** —— `HTTP_TRUSTED_PROXY_COUNT` 默认 `0`，即完全忽略 `X-Forwarded-For`。那是客户端可控的头，只能配真实跳数，绝不能配大。
- **遥测中的敏感数据** —— DB 与缓存 span 使用属性白名单，任何查询参数或缓存值都不可能进入 span，并有自动化审计测试断言这一点。
- **生产守卫** —— `APP_ENV=production` 时，仍使用示例 JWT 密钥、CORS 配了通配符、或 metrics 绑在公开地址上，启动都会被拒绝。

完整论证见 [ADR-0007](docs/adr/0007-auth.md) 与
[ADR-0008](docs/adr/0008-rate-limiting.md)。

## 消息

默认关闭，而这个默认值本身就是设计的一部分：`MESSAGING_ENABLED=false` 时服务
不写 outbox 行、不需要 broker、不需要 worker 进程。一个永远不会走向异步的项目，
不会为这套东西的存在付任何代价。

打开后，事件与业务变更**在同一个事务里**写入 `outbox_events`，两者不可能不一致。
`cmd/worker` 负责把待发行投递到 NATS JetStream 并消费：

```sh
make broker-up            # NATS JetStream，位于 compose 的 messaging profile
make worker               # relay + consumer
make outbox-status        # 待发 / 已发 / 失败中的数量
make dlq-list             # 待处置的死信
make dlq-replay id=<id>   # 把一条死信放回 outbox
```

投递语义是 at-least-once，消费端靠 `processed_events` 去重日志做幂等。该日志的
保留期**远长于**消息流本身 —— 剪得比流早，一条被重投的消息就会被处理两次。

> [!WARNING]
> `notify.LogSender` 只是把"已发送"的邮件写进日志。整条链路 —— outbox、投递、
> 幂等、重试、DLQ —— 都是真的且有测试，唯独背后没有接任何邮件服务商。**不要在
> 认为这个服务能发信的前提下部署它。**

## 可观测性

埋点始终编译在内，`OTEL_ENABLED` 与 `METRICS_ENABLED` 只控制数据是否离开进程。
tracing 关闭时业务行为不变 —— 这是实跑验证过的，不是假设。

```sh
docker compose --profile observability up -d
OTEL_ENABLED=true make run
```

Grafana 在 <http://localhost:3000>（匿名访问，仅绑本机），Prometheus 在 `:9090`，
Tempo 在 `:3200`。一条 trace 贯穿 HTTP → service → PostgreSQL → Redis → NATS →
worker，链路内的每条日志都带同一个 `trace_id`。

指标在**独立监听器**上导出（默认 `127.0.0.1:9464`），不走 API 端口：指标名会
枚举出每一个路由模板、每一个依赖和构建版本，而 API 端口是本来就要对外可达的那个。

`internal/telemetry` 是唯一允许 import OTel SDK 或 Prometheus 客户端的包。业务包
只声明小的 `Observer` 接口，对两者一无所知 —— 由 depguard 强制，并用 `go list`
验证过。

## 测试

```sh
make test               # 单元测试，不需要 Docker
make test-race          # 打开竞态检测器
make test-integration   # testcontainers，需要 Docker
make check              # fmt · vet · lint · test -race · vuln · sqlc-check
```

`make check` 是门禁，且与 CI 跑的是同一批命令。只存在于 CI 配置里的命令，是
开发者无法在本地复现的命令 —— "在我机器上是好的"就是这么开始的。

Go 测试之外，[`test/blackbox/`](test/blackbox/README.md) 有 110 条对抗性检查，
针对**运行中的**服务走 HTTP。它们存在的理由是：Go 测试与服务共享同一套假设 ——
一个默默接受负数 offset 的假仓储，无法暴露 offset 溢出了。这套用例找出过 5 个
真实缺陷，全部记录在 `STATE.md`。

覆盖率不是目标，**没有被断言的行为**才是。Phase 5 期间有若干变异在测试全绿的
情况下存活下来，这件事也如实写进了文档。

## 开发工作流

```sh
make help                          # 列出全部 target 及说明
make run | build | clean
make fmt vet lint vuln
make sqlc | sqlc-check             # 重新生成 / 校验生成的查询代码
make migrate-create name=add_orders
make migrate-up | migrate-down | migrate-status
make openapi-lint                  # 需要 npx；刻意不纳入 make check
```

新增一个端点要改四处，其中三处有强制：handler、`api/openapi.yaml`（否则
`cmd/api/openapi_test.go` 失败）、一个 Bruno 请求（否则 `collection_test.go`
失败），以及 —— 如果返回了新结构 —— 一处响应 schema 校验。

数据库错误到 `apperr` 的翻译**只能出现在 `*pg` 包**。如果 service 层看起来需要
判断驱动的错误码，说明下一层漏了一次翻译：改仓储，不要改 service。

## goscaffold CLI

```sh
make scaffold      # 安装到 GOPATH/bin

goscaffold new billing --module github.com/acme/billing
cd billing
goscaffold module invoice
goscaffold migration create_invoices
```

| 命令 | 写出什么 |
|---|---|
| `new <name> --module <path>` | 整套脚手架，module 路径与服务名全文替换 |
| `module <name>` | 领域、service 与端口、HTTP 适配器、PostgreSQL 桩、测试——ADR-0002 的三包结构 |
| `migration <name>` | 一对 up/down，版本号不会撞车 |
| `version` | 构建信息 |

`--dry-run` 只列出将写入哪些文件，不落盘。`--output` 与 `--dir` 指定位置。

### 绝不覆盖已有文件

每个命令先构建完整的文件集合，逐个与磁盘核对，然后才写——而且用 `O_EXCL`，
让文件系统而不是先前那次检查来做最终裁决。只要有一个文件挡路，它会列出全部
冲突并且**一个字节都不写**。边走边写的生成器一旦中途停下，留下的目录既不是
旧状态也不是新状态，那是唯一无法靠重跑恢复的结果。

**没有 `--force`。** 加它只需要一个条件判断，但这个命令通常指向一个别人已经
投入过工作的目录。

### 模板漂移是怎么解决的

这是任何脚手架 CLI 的真问题，这里的答案是：**代码只有一份**。

`internal/scaffold/templates/project/` 不是手工维护的，它是本仓库的**派生
副本**，由 `make templates` 生成、由 `TestProjectTemplatesMatchTheRepository`
守着。改了 `internal/httpx/router.go` 却没重新生成，`make check` 就会失败并
点名那个文件。这与 `make sqlc-check` 是同一个机制，只是对准了另一种生成物
——并经三种变异验证：改动源文件、模板缺失、源文件已删而模板残留。

模块模板是例外，它无法从任何东西派生：它描述的是一个还不存在的模块。守住它的
是 `Scaffold` workflow——生成项目、在里面生成模块、编译并测试结果。如果对
`apperr` 或 `httpx` 的改动破坏了模板所假设的形状，那个 job 就会变红。

生成的 Go 代码在写盘前都过一遍 `go/format`，所以按 `__TYPE__` 宽度对齐的模板
不会产出围绕 `Order` 错位的代码；而不是合法 Go 的模板会在**生成时**就失败，
而不是拖到 `go build`。

### 刻意不做的事

- **没有 `--with-redis` / `--with-observability` 开关。** Redis 是限流的硬依赖（ADR-0008），`internal/telemetry` 穿在每个构造函数上（ADR-0010）；去掉任何一个的开关都会产出编译不过的项目。
- **没有 `--with-messaging` 开关。** ADR-0009 已经在运行期把消息设为默认关闭：`MESSAGING_ENABLED=false` 时没有 outbox 行、不需要 broker、不需要 worker。这个开关只能删掉一些本来就不花代价的文件。
- **没有配置文件，没有插件系统。** 四个命令三个 flag 不需要 `.goscaffold.yaml`；而一个能改变 `new` 产物的配置文件，会把模板机制刚消灭掉的漂移重新引进来。换模板来源的扩展点是 `readTemplateTree`，就一个函数。
- **它绝不修改已有文件**，所以把新模块接进 `cmd/api/features.go`、在 `internal/auth/policy.go` 注册权限、加进 depguard 规则、在 `api/openapi.yaml` 里补文档，都还是你的事。命令结束时会把清单打出来。

生成的端点在你注册权限之前一律返回 **403**。策略表是权限的唯一来源，没登记的
权限永远不会被授予——所以一个生成出来的端点，在没人决定"谁可以访问"之前是
不可达的。

## 部署

完整流程、回滚与踩坑清单:**[docs/deployment.md](docs/deployment.md)**。

一份 Dockerfile 三个 target —— `api`(40.8 MB)与 `worker`(35.1 MB)基于
`gcr.io/distroless/static:nonroot`,外加 `migrate`:golang-migrate CLI 把本
仓库的 SQL 烤了进去,所以一个镜像 tag 就确切地指明了那个 commit 期望的 schema。

```sh
make docker-build
kubectl apply -k deployments/k8s/overlays/production
kubectl wait --for=condition=complete job/goscaffold-migrate --timeout=5m
kubectl rollout status deploy/goscaffold-api
```

Kustomize,一个 base 四个 overlay。迁移用 Job 而不是 initContainer:
initContainer 每 Pod 跑一次,三副本发布会把同一份 DDL 并发执行三遍,而且此后
每次重启都再跑一遍。

这些 manifest 里有四处是承重的,每一处都有 `deployments/k8s/manifests_test.go`
守着 —— 因为它们出错时都不指向自己:

| 设置 | 为什么要紧 |
|---|---|
| `terminationGracePeriodSeconds: 45` | 应用预算是 30 秒。"刚好大于 30"在负载高时就错:SIGTERM 送达不是瞬时的,余量太薄变成 SIGKILL——发布期偶发 502,只在生产出现。 |
| liveness → `/live` | 它绝不碰数据库。碰了的话,几秒的主库切换会让所有副本同时探针失败、同时被杀、同时冷启动打向正在恢复的库。 |
| `METRICS_ADDR: ":9464"` | 应用默认绑 loopback,在 Pod 里等于谁也抓不到,包括 Prometheus。失败形式是"没有数据"而不是"报错"。 |
| `runAsNonRoot`、`readOnlyRootFilesystem`、`drop: [ALL]`、`seccompProfile` | 少一项没有任何可观测代价,负载照常运行,只是权限比需要的多。 |

> [!IMPORTANT]
> 最终镜像没有 shell,`kubectl exec` 进不去。用
> `kubectl debug -it <pod> --image=busybox --target=api`。

### CI/CD

`ci.yml` 在每个 PR 上跑三个 job,按耗时拆分,让第一个红叉在一分钟左右出现:
`quality`(gofmt、vet、golangci-lint、`sqlc diff`、actionlint、OpenAPI lint)、
`test`(build、`go test -race`、govulncheck)、`integration`(testcontainers)。
`scaffold.yml` 生成项目、在其中生成模块、编译并测试。`docker.yml` 发布到
`ghcr.io`。

每条命令都同时是一个 Makefile target —— 只存在于 CI 里的检查,是没人能在推送
前复现的检查。

### 验证状态

如实记录，因为「能部署」是一句值得说准确的话。

| 检查 | 结果 |
|---|---|
| `make check` | ✅ 通过 |
| `make test-integration` | ✅ 通过（PostgreSQL 17，testcontainers） |
| `docker build`（三个 target） | ✅ 构建成功 |
| 镜像连真实 Postgres 与 Redis 运行 | ✅ `/ready` 200，commit 报告正确 |
| `docker compose --profile full up -d` | ✅ 9 个服务，`/live` 200 |
| 优雅关停（SIGTERM → 排空 → 退出） | ✅ `/ready` 报 `draining` 约 5 秒，退出码 0 |
| 四个 overlay 的 `kubectl kustomize` | ✅ 均可渲染 |
| manifests 对 Kubernetes 1.33 schema 校验 | ✅ `kubeconform -strict`，0 invalid |
| 生成项目的 build / vet / test / gofmt | ✅ 24 个包通过，0 个未格式化 |
| 两个 workflow 的 `actionlint` | ✅ 无问题（带 shellcheck，与 CI 一致） |
| 守卫测试的变异验证 | ✅ 5 个变异，全部被抓 |
| 两个 workflow 在 GitHub Runner 上 | ✅ 全绿——CI 与 Docker，第三次尝试 |
| **在真实集群上部署** | ⛔ **未做——没有可用集群** |

`kubectl apply --dry-run=client` 用不了：它要从集群下载 OpenAPI schema，而本机
没有集群。`kubeconform` 用官方发布的 1.33 schema 离线校验同一批 manifest，且开
strict 模式（拒绝未知字段）——正是大家想用 dry run 抓的那类笔误。

两个 workflow 前后修了两次才全绿，而两个问题在本机都看不见：
`golangci-lint-action@v6` 只支持 golangci-lint v1，而本仓库声明的是
`version: "2"`；`actionlint` 会对每个 `run:` 块调用 shellcheck，本机没装就
静默跳过这一半检查。`make actionlint` 现在会明说这件事，而不是悄悄通过。

## 尚未实现

明确写出来，因为一个夸大自身就绪度的脚手架，比一个少给了东西的脚手架更糟。

- **没有任何东西在真实集群上跑过。** manifests 能渲染、能通过 schema 校验，但从未被 apply。见[验证状态](#验证状态)。
- **没有 NetworkPolicy、ServiceMonitor、PrometheusRule。** 告警与 SLO 在 Phase 6 范围之外；而没有 CNI 真正执行的 NetworkPolicy 只是装饰。
- **没有镜像扫描。** `govulncheck` 已覆盖 Go 依赖；在几乎没有 OS 包的 distroless 镜像上，再加一个扫描器的增量价值很低。
- **没有接入邮件服务商。** 见上方警告。
- **没有自助注册。** 建用户需要 `user:create` 权限。这是范围决定，不是遗漏。
- **请求体尚无 OpenAPI schema 自动校验**（响应体已有）。请求体的权威定义在 `validate` 规则里，比对两者需要另一套还不存在的映射。
- **`docker-compose.yml` 仅供开发** —— 没有资源限制、没有密钥管理、没有重启策略。

已知的取舍（每一条都在对应 ADR 里论证过）汇总在
[`STATE.md`](STATE.md#known-limitations)：access token 撤销最长 15 分钟延迟、
通用限流 fail-open、事务经由 context 隐式传递、offset 分页在并发插入下的重复/
跳过，等等。

## 文档

| 文档 | 回答什么 |
|---|---|
| [`STATE.md`](STATE.md) | 现在到底是什么状态 —— 做了什么、没做什么、有什么限制 |
| [`docs/architecture.md`](docs/architecture.md) | 为什么设计成这样 |
| [`docs/adr/`](docs/adr/) | 十个决策，含备选方案与复审触发条件 |
| [`docs/progress.md`](docs/progress.md) | 这个仓库是怎么一步步变成现在这样的 |
| [`docs/development.md`](docs/development.md) | 怎么加模块、端点、迁移、查询、依赖 |
| [`docs/deployment.md`](docs/deployment.md) | 镜像、Secret、迁移顺序、探针、回滚 |
| [`docs/security.md`](docs/security.md) | 防什么、不防什么、已知暴露面 |
| [`docs/final-report.md`](docs/final-report.md) | Phase 8 终审：每项门禁、证据，以及仍未验证的部分 |
| [`api/openapi.yaml`](api/openapi.yaml) | API 契约 |
| [`prompts/`](prompts/README.md) | 分阶段执行契约，含强制诚实与范围纪律条款 |

`STATE.md` 与任何其他文档冲突时，以 `STATE.md` 为准。

## 许可

Apache-2.0，与 `api/openapi.yaml` 中的声明一致。
