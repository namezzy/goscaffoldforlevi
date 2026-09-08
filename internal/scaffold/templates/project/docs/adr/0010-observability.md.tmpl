# ADR-0010: 可观测性的接线方式

- **Status**: Accepted
- **Date**: 2026-09-01
- **Phase**: Phase 5
- **Supersedes**: None
- **Superseded by**: None

## Context

Phase 5 要给 HTTP、PostgreSQL、Redis、NATS 和若干业务事件加上 trace 与
metrics。可观测性代码有一个众所周知的失败模式：**它渗进每一个业务函数**。
一旦 `internal/user` 直接 import Prometheus，它的单元测试就需要一个
registry；registry 重复注册会 panic；测试于是开始互相干扰，最后有人加了个
全局变量和一个 `sync.Once`，问题被埋起来而不是解决。

同时 ADR-0001 已经规定业务包不感知具体基础设施。Phase 5 是这条规定第一次
被真正施压——因为埋点天然想待在业务逻辑旁边。

## Decision

### 1. `internal/telemetry` 是唯一知道 OTel SDK 与 Prometheus 的包

其他任何包（包括 `internal/postgres`、`internal/redis`、`internal/httpx`）
都只依赖 `go.opentelemetry.io/otel/trace` 这类**API** 包，或者只依赖自己
声明的窄接口。SDK 与 exporter 的构造全部在 `internal/telemetry`。

用 `go list` 可以机械验证这一条，Phase 8 会把它固化为静态审计。
例外：`natsmq` 的**测试**用 `sdk/trace/tracetest` 做录制器——测试需要一个
真的 span 记录器，用 fake 会让测试无法证明 span 真的被创建。

### 2. 业务包声明接口，组合根注入实现

```go
// internal/user/observer.go
type Observer interface {
    UserRegistered()
}
```

`*telemetry.Metrics` 天然满足这些接口，组合根直接传入，**不写适配器**。

接口方法名因此带上了领域前缀（`MessagingPublished` 而不是 `Published`）。
这看着啰嗦，但适配器是标签被悄悄丢掉的地方：一个 `func(t string)` 转成
`func(t, c string)` 的适配层，很容易在某次重构里把 consumer 标签填成常量。

### 3. 不使用现成的 pgx / go-redis instrumentation

它们默认把语句记进 `db.statement`。在本项目里那意味着 span 上会出现
`rl:login:victim@example.com` 和 refresh token 本身。

改为**白名单**：
- PostgreSQL 只记 sqlc `-- name: X :one` 注释里的 query 名
- Redis 只记 `cmd.Name()`（**不是** `cmd.String()`，一个方法之差就是泄漏）

span 的 error status 同理**只记"失败了"不记"为什么"**：驱动的错误文本会
引用参数，唯一约束冲突的报文里就带着那个邮箱地址。

### 4. HTTP 中间件手写，不用 otelhttp

chi 的 route 模板要等 `next.ServeHTTP` 返回之后才能从 `RoutePattern()`
读到。otelhttp 在进入时就定名，只能用原始路径——而原始路径带 id，是高基数
时间序列的直接来源。手写版本开 span 时用临时名，出栈时 `span.SetName()`，
未匹配的路由统一归并为 `unmatched`。

### 5. `/metrics` 走独立监听端口

三个方案：

| 方案 | 问题 |
|---|---|
| 挂在 public HTTP 端口 | 只差一条 ingress 规则就暴露到公网；而且会被自己的限流吃掉，正好在最需要指标的时候 |
| 挂在带鉴权的 admin 端口 | Prometheus 要配 scrape 凭据，凭据要轮转，轮转失败表现为指标静默消失 |
| **独立内网端口** | 靠网络策略控制——而那正是 Prometheus 部署本来就需要的控制 |

选第三个。listener 在**构造期**打开而不是在 goroutine 里：端口被占必须让
进程启动失败（编排器看得见），否则它只是一条日志，进程自称健康却什么都不导出。

### 6. metrics registry 是实例而不是全局

`prometheus.NewRegistry()` 存在 `*telemetry.Metrics` 里。全局 registry 的
重复注册会 panic，这会让"同一个进程里跑两次组合根"的集成测试无法写。

## Consequences

**好的**：

- 业务包的测试不需要任何遥测基础设施，用 fake observer 即可
- 换掉 Prometheus 或 OTel 的影响半径是一个包
- 高基数与密钥泄漏各有明确的收口点，可以针对性地写审计测试
- tracing 默认关闭时，全部埋点退化为 noop，业务行为不变（有集成测试验证）

**代价**：

- 每加一个业务指标要动两个文件（业务包的接口 + telemetry 的实现）
- 接口方法名带前缀，读起来啰嗦
- 手写 HTTP 中间件意味着 otelhttp 的后续改进不会自动获得

## Alternatives Considered

**业务包直接 import Prometheus。** 少写一层接口，但违反 ADR-0001，且让
业务单元测试依赖全局 registry。拒绝。

**用 OTel metrics API 而不是 Prometheus client。** 更统一，且能顺带拿到
exemplars。但需要额外跑一个 Prometheus exporter 或让 Collector 承担
scrape，增加一个必须活着的组件才能看到最基础的指标。本阶段选择直接用
Prometheus client，把 exemplars 记为 Known Limitation。

**给所有依赖用现成 instrumentation，再加脱敏处理器。** 脱敏是黑名单，
黑名单会漏。白名单在这里成本极低（query 名与命令名本来就是有界集合）。

## When to Revisit

- 需要 exemplars（从延迟尖峰直接跳到那条慢 trace）时，重新评估改用
  OTel metrics API
- 出现第二个非 chi 的 HTTP 入口时，重新评估手写中间件的复用方式
- 指标基数超出单 Prometheus 实例承载时，重新评估标签集合
