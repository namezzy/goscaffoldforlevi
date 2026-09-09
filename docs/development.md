# 开发指南

> 本文回答"我要做 X,该改哪些文件"。
> 设计上的**为什么**在 [`architecture.md`](architecture.md) 与 [`adr/`](adr/);
> 部署在 [`deployment.md`](deployment.md);安全在 [`security.md`](security.md)。

## 目录

- [第一次上手](#第一次上手)
- [新增一个业务模块](#新增一个业务模块)
- [新增一个 API 端点](#新增一个-api-端点)
- [新增一条迁移](#新增一条迁移)
- [新增一条查询](#新增一条查询)
- [新增一个依赖](#新增一个依赖)
- [写测试](#写测试)
- [调试](#调试)
- [门禁与它们各自守着什么](#门禁与它们各自守着什么)

## 第一次上手

```sh
cp .env.example .env      # 所有默认值开箱可用
make tools                # 固定版本的 golangci-lint / sqlc / migrate / govulncheck
docker compose up -d      # postgres + redis
make migrate-up
make run
```

`make help` 列出全部 target。

有一件事值得先知道:**这个仓库里很多约定是由测试强制的,不是靠自觉**。改错了
不会得到一句温和的提醒,而是一个失败的构建,并且失败信息会说明这条规则为什么
存在。清单见[最后一节](#门禁与它们各自守着什么)。

## 新增一个业务模块

```sh
goscaffold module order      # 需要先 make scaffold
```

生成 7 个文件,三包结构(ADR-0002):

```text
internal/order/order.go               实体、ID、CreateInput、Patch、Page
internal/order/service.go             Repository 端口 + Service
internal/order/service_test.go        手写 fake
internal/order/orderhttp/dto.go       请求/响应类型
internal/order/orderhttp/handler.go   端点 + 权限常量
internal/order/orderhttp/handler_test.go
internal/order/orderpg/repository.go  编译得过的桩
```

生成器**不修改任何已有文件**,所以下面这些是你的事,它会在结束时把清单打出来:

1. **注册权限** —— `internal/auth/policy.go` 的 `rolePermissions`。
   在你注册之前,新端点对所有人返回 403。这是刻意的:策略表是权限的唯一来源,
   没登记的永远不授予,所以一个生成出来的端点在有人决定"谁可以访问"之前是
   不可达的。
2. **加进 depguard** —— `.golangci.yml` 的 `business-layer` 规则按包名逐个
   列举。新包默认**不在**保护范围内,也就是说没有任何东西会阻止
   `internal/order/service.go` 去 import `net/http` 或 `pgx`。
3. **装配** —— `cmd/api/features.go` 构造 service 与 handler,
   `cmd/api/routes.go` 挂载路由。
4. **写迁移与查询**,然后把 `orderpg/repository.go` 里返回
   `NOT_IMPLEMENTED` 的方法体换成真实实现。
5. **补 `api/openapi.yaml`** —— 否则 `cmd/api/openapi_test.go` 双向失败。

三包的依赖方向是单向的:`orderhttp` 与 `orderpg` 都依赖 `order`,
`order` 谁也不依赖。它不知道 HTTP 和 SQL 的存在。

## 新增一个 API 端点

**四处要改,其中三处有测试强制:**

| 改哪里 | 不改会怎样 |
|---|---|
| handler 与路由表 | —— |
| `api/openapi.yaml` | `cmd/api/openapi_test.go` **双向**失败:未文档化的路由、或指向不存在路由的规格条目 |
| `api/bruno/` 加一个请求 | `api/bruno/collection_test.go` 失败 |
| 返回新结构时补响应 schema | `internal/openapitest` 的断言失败 |

约定(ADR-0004):成功响应直接返回资源,状态码就是 envelope;失败统一包装;
更新只用 `PATCH`,不提供 `PUT`;解码一律 `DisallowUnknownFields`。

新增错误 `Kind` 时,`internal/httpx/errmap.go` 是唯一的 Kind→status 映射表,
`errmap_test.go` 的穷尽性检查会要求你登记它。

## 新增一条迁移

```sh
make migrate-create name=add_orders_status   # 等价于 goscaffold migration
make migrate-up
make migrate-status
```

版本号是固定 17 位的毫秒时间戳。**不要改回 `-seq`**:顺序编号在两个分支各加
一条迁移时必然撞车,而撞车只在合并时才被发现;更要紧的是,一旦目录里混入了
不同宽度的版本号,整数比较的顺序就会错乱。

写迁移时守两条:

- **先加后用**:先加列、下个版本再写它、再下个版本才删旧的。回滚代码不等于
  回滚数据库,上一个版本必须还能在新 schema 上跑。
- **给线上表建索引**要用 `CREATE INDEX CONCURRENTLY`,并且**单独一条迁移**
  —— 它不能在事务里执行。

每条迁移都必须有对应的 `.down.sql`。事后凭记忆补写的 down 迁移,是没人敢跑的
那种。

## 新增一条查询

```sh
vim queries/order.sql     # 带 -- name: XxxYyy :one|:many|:exec 注释
make sqlc                 # 重新生成 internal/postgres/db
make sqlc-check           # CI 用它确认生成物没过期
```

schema 的来源是 `migrations/` 目录本身,不是单独的 dump —— 两个 schema 事实
来源是生成代码与真实数据库开始漂移的地方。

**列名一律显式列出,不要 `SELECT *`。** 生成的 struct 会随着表结构悄悄变化,
而调用方在编译期看不出差别。

数据库错误到 `apperr` 的翻译**只能出现在 `*pg` 包**。如果 service 层看起来
需要判断驱动的错误码,说明下一层漏了一次翻译:改仓储,不要改 service。

## 新增一个依赖

先问三个问题(§8.14 的依赖审计对每个现有依赖都回答过):

1. 它解决的问题标准库解决不了吗?
2. 它会出现在多少个包里?一个只在一处使用的依赖,通常可以内联成几十行。
3. 它自己拖进来多少间接依赖?

ADR-0001 已经**明确否决**过:Viper、validator、GORM、zap、DI 框架。
这些不是品味问题,理由写在 ADR 里,要推翻需要新开一份 ADR 并写明 `Supersedes:`。

加完跑 `make vuln`,并确认 `go.mod` 的直接依赖仍在 20 个以内。

## 写测试

```sh
make test               # 单元测试,不需要 Docker
make test-race          # 竞态检测器
make test-integration   # testcontainers,需要 Docker
make cover              # 覆盖率报告
```

几条本仓库反复验证过的经验:

- **覆盖率不是目标,没有被断言的行为才是。** Phase 5 期间有若干变异在测试
  全绿的情况下存活。加测试时先问"这条断言失败时,是什么坏了"。
- **假实现要真实。** Phase 8 里有一条 handler 测试红了,原因是我的假仓储对
  "会话不存在"返回了普通 error,而真实仓储返回 `apperr.NotFound` —— 服务对
  这两者的处理完全不同。假得不像的双测出来的是另一件事。
- **集成测试用 testcontainers**(`internal/postgres/pgtest`),对真实
  PostgreSQL 跑真实迁移。它们带 `integration` 构建标签,默认不跑。
- **守卫测试要做变异验证。** 仓库里所有守卫测试(dashboard、manifests、bruno
  集合、模板漂移)都记录了它们被验证过会变红的具体变异。没见它红过的测试不算数。
- `test/blackbox/` 是对**运行中的服务**发起的对抗性 HTTP 测试。它们存在的
  理由是 Go 测试与服务共享同一套假设。

## 调试

**接口调试**用 `api/bruno/`:登录请求的脚本会把 token 写进集合变量,不必每
15 分钟手工复制一次。注意它一次完整运行要花掉 6 次登录配额(IP 限 10 次/15
分钟),连跑第二遍会 429 —— 那不是服务故障。

**看链路**:

```sh
docker compose --profile observability up -d
OTEL_ENABLED=true make run
```

Grafana 在 <http://localhost:3000>。一条 trace 贯穿 HTTP → service →
PostgreSQL → Redis → NATS → worker,链路内每条日志都带同一个 `trace_id`。
拿到用户报的 `request_id` 后,日志里 grep 它就能定位到那一条链路。

**看消息积压**:

```sh
make outbox-status        # 待发 / 已发 / 失败中
make dlq-list             # 死信
make dlq-replay id=<id>   # 放回 outbox
```

**容器里进不去**:镜像是 distroless,没有 shell,`kubectl exec` 不可用。
用 `kubectl debug -it <pod> --image=busybox --target=api`。

## 门禁与它们各自守着什么

`make check` = `fmt` + `vet` + `lint` + `test -race` + `vuln` + `sqlc-check`。
它和 CI 跑的是同一批命令 —— 只存在于 CI 配置里的检查,是开发者无法在推送前
复现的检查。

| 机制 | 守着什么 | 违反时 |
|---|---|---|
| `.golangci.yml` 的 depguard | 依赖方向(ADR-0002) | `make lint` 失败 |
| `internal/httpx/errmap_test.go` | Kind→status 映射唯一且穷尽 | 新 Kind 未登记 → 测试失败 |
| `internal/config/env_example_test.go` | `.env.example` 与加载器不漂移 | 键名不一致 → 测试失败 |
| `cmd/api/openapi_test.go` | 路由与规格**双向**一致 | 任一方向缺失 → 测试失败 |
| `internal/openapitest` | 响应体符合声明的 schema | 结构漂移 → 测试失败 |
| `make sqlc-check` | 生成的查询代码没过期 | CI 失败 |
| `deployments/observability/dashboard_test.go` | 面板引用的指标真实存在 | 指标改名 → 测试失败 |
| `deployments/k8s/manifests_test.go` | 关停预算、探针路径、securityContext、镜像 tag | 任一项被改坏 → 测试失败 |
| `api/bruno/collection_test.go` | 集合覆盖全部 operation | 缺失或悬空请求 → 测试失败 |
| `internal/scaffold` 的漂移测试 | 模板与本仓库一致 | 改了源文件没重新生成 → 测试失败 |

`make actionlint` 在本机缺 shellcheck 时会明说"本次检查比 CI 弱"。这一条是
踩出来的:CI 的 Runner 装了 shellcheck,本机没有,于是本地绿、CI 红。
