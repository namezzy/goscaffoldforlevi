# Phase 2 — Foundation

先读取 `STATE.md` 和 `docs/adr/`，再读代码库。

**不得重新设计架构。** 如发现 Phase 1 存在严重问题，新建 ADR 说明并 supersede。

目标：建立一个**可启动 / 可配置 / 可连接依赖 / 有日志 / 有 Router / 有 Middleware / 有错误处理 / 有健康检查 / 可优雅退出**的最小生产级基础工程。

## 2.1 工程初始化

```text
module: github.com/namezzy/goscaffoldforlevi
```

按 ADR-0002 建立目录。禁止创建无用空目录。

## 2.2 Config

类型安全 Config，支持 `.env` / 环境变量 / 默认值，环境变量优先级最高。

必须 Validate：`port / database URL / JWT key / timeout / environment`。
关键配置缺失 → **启动失败**。

## 2.3 Logger

使用 `log/slog`。

```text
development → 可读文本
production  → JSON
```

基础字段：`service / environment / version`。

**禁止业务代码自行创建 logger。**

## 2.4 Database

`pgxpool`，实现 `NewPool / Ping / Close`。

配置并给出合理默认值：
`MaxConns / MinConns / MaxConnLifetime / MaxConnIdleTime / HealthCheckPeriod`。

所有初始化必须接受 `context.Context` 并设置连接超时。

## 2.5 Migration 工具链（前移自原 Phase 5）

**本阶段必须建立完整 migration 能力，因为 Phase 3 会立刻用到。**

```bash
make migrate-up
make migrate-down
make migrate-create NAME=create_users
```

使用 golang-migrate。目录 `migrations/`。

同时在 `docs/` 中说明：
**为什么生产环境不能让每个 API Pod 启动时自动执行 migration**，
以及推荐的执行方式（CI/CD Job / Kubernetes Job / 手动受控步骤）。

## 2.6 sqlc 工具链（前移自原 Phase 5）

建立 `sqlc.yaml`，配置 `queries/` → 生成代码目录。

```bash
make sqlc
make generate
```

本阶段不需要写业务查询，但工具链必须可运行（可用一个最小 query 验证）。

## 2.7 Redis

go-redis，实现 `NewClient / Ping / Close`。

Redis 不可用时的策略由配置决定：`required` / `optional`。项目必须有明确默认值。

## 2.8 HTTP Server

`net/http` + chi。正确配置：

```text
ReadHeaderTimeout / ReadTimeout / WriteTimeout / IdleTimeout / MaxHeaderBytes
```

**禁止 `http.ListenAndServe(...)` 裸启动。**

## 2.9 Router 与健康检查

```text
/api/v1
GET /health
GET /live    → 只判断进程存活
GET /ready   → 检查必要依赖（PostgreSQL）
```

Redis 若为 optional，其临时故障不得导致 readiness failure。

## 2.10 Error Handling

统一 App Error：
`InvalidArgument / NotFound / Unauthorized / Forbidden / Conflict / RateLimited / Internal`

HTTP 映射示例：

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "invalid request",
  "request_id": "..."
}
```

Internal Error **不得**向客户端暴露：database error / stack / secret / SQL / internal path。

## 2.11 Middleware

本阶段实现：`Recovery / Request ID / Logging / Timeout / CORS`。

Tracing / Metrics / Rate Limit 预留挂载点，但不实现。

Request ID：客户端传入合法 ID 可复用，否则生成新的；
响应头返回 `X-Request-ID`。

## 2.12 Recovery

捕获 panic，记录 `panic / stack trace / request_id`，返回 500。
单个请求 panic 不得导致进程退出。

## 2.13 Request Logging

记录：`method / path / status / duration / request_id`（`remote_ip` / `user_agent` 可选）。

**禁止记录**：`Authorization / Cookie / Password / Token / Request Body`。

## 2.14 Graceful Shutdown

监听 SIGINT / SIGTERM，调用 `http.Server.Shutdown()`，配置 shutdown timeout，
然后按 ADR 顺序关闭 database / redis / telemetry。

**shutdown 流程必须可测试。**

## 2.15 Bootstrap

手动 DI。禁止 global database / logger / config。`main.go` 保持简单。

## 2.16 Environment Files

提供 `.env.example`（不含真实 secret）+ development 默认值。

## 2.17 Makefile

```bash
make run build test test-race fmt vet lint vuln
make generate sqlc
make migrate-up migrate-down migrate-create
```

工具未安装时，README 说明安装方式。

## 2.18 Quality Tools

接入 `golangci-lint` + `govulncheck`。不要使用已废弃的 linter。

## 2.19 最低测试要求

```text
config validation
health / live / ready endpoints
error mapping
request ID middleware
recovery middleware
graceful shutdown
```

## 2.20 Phase 2 输出

执行 §0.4 全部门禁命令，失败立即修复。输出：

```text
Files Created
Architecture Notes
Commands Run（含真实输出摘要 / SKIPPED 说明）
Test Results
本阶段有没有做超范围的事？
Known Limitations
```

**更新 `STATE.md` 并提交。不要进入 User 业务模块。**
