# Phase 8 — Documentation & Final Audit

先读取 `STATE.md` 和全部 ADR。

**本阶段不要继续加功能。**

目标：把项目变成**别人真正可以使用、新人真正可以上手、团队真正可以长期维护**的企业级 Starter。

## 8.1 README

包含：

```text
Project Overview / Why This Scaffold / Features / Architecture
Directory Structure / Quick Start / Configuration
Database / Redis / Messaging / Authentication / Rate Limiting / API
Testing / Observability / Docker / Kubernetes / CI-CD
Scaffold CLI / Security / Development Workflow / Troubleshooting
```

Quick Start 必须真实可执行：

```bash
git clone ... && cd ...
cp .env.example .env
docker compose up -d
make migrate-up
make run
```

然后 `GET /live`、`GET /ready` 可用。

**你必须实际按这个流程走一遍并报告结果**（环境不允许则 SKIPPED）。

## 8.2 docs/architecture.md

说明 dependency direction / layer responsibilities / interface placement /
repository design / transaction design / error design / configuration /
messaging / observability / authentication。

**重点解释"为什么这样设计"，而不是只描述目录。**

## 8.3 docs/development.md

`How to add module / API / migration / query / dependency / test / debug`。

## 8.4 API 文档一致性

确认 `api/openapi.yaml` 完整覆盖 auth / users / error schemas / pagination。

**执行一次 drift 检查**：所有已注册路由与 spec 逐一比对，列出差异并修复。

## 8.5 docs/deployment.md

`Docker / Kubernetes / Environment variables / Secrets / Migration / Health checks / Observability / Rollback`。

## 8.6 docs/security.md

`Password / JWT / Refresh Token / Secrets / Logging / Rate Limit / CORS /
Dependency vulnerabilities / Container security / Messaging security`。

## 8.7 Architecture Audit

从零审计整个项目，查找：

```text
过度抽象 / 重复 abstraction / 巨大 interface / 无价值 wrapper
global state / hidden dependency / utils-common 垃圾包
循环依赖风险 / 过深目录 / 过多 DTO mapping
```

**发现即重构。**

## 8.8 Go Idiom Audit

```text
package naming / interface size / error wrapping / context propagation
defer usage / goroutine lifecycle / resource closing
pointer vs value receiver / constructor usage / zero value
```

## 8.9 Concurrency Audit

```text
goroutine leak / race condition / channel misuse
context cancellation / background worker lifecycle / shared state
```

执行 `go test -race ./...`。

## 8.10 HTTP Audit

```text
timeouts / body size limit / panic / request cancellation
graceful shutdown / status code / content type / CORS / rate limit
```

## 8.11 Database Audit

```text
N+1 / missing index / transaction 正确性 / connection leak
context / timeout / SELECT * / SQL injection / migration rollback 可用性
```

## 8.12 Messaging Audit

```text
消息丢失路径 / 重复消费 / 幂等性 / consumer 泄漏
DLQ 可运维性 / 优雅退出 / 契约版本兼容
```

## 8.13 Security Audit

检查是否泄漏：

```text
password / password hash / JWT / refresh token / Authorization
database credentials / Redis credentials / broker credentials / internal errors
```

## 8.14 Dependency Audit

对每个依赖回答：**为什么存在 / 能否用标准库替代 / 是否仍然需要。**

删除不必要依赖，保持依赖树尽量小。

## 8.15 Performance Audit

**不要过早优化。** 只检查明显问题：

```text
unbounded allocation / large body / unbounded pagination
bad DB query / connection pool misuse / excessive JSON conversion
high-cardinality metrics
```

## 8.16 Generated Scaffold Audit

```bash
goscaffold new final-test
cd final-test && go mod tidy && go build ./... && go test ./... && go test -race ./...
```

启动后验证 `/live` `/ready` `login` `users`。

## 8.17 Clean Repository

检查 Git 仓库，禁止包含：

```text
.env / database password / JWT secret / IDE cache
binary / temporary files / coverage artifacts / local database volume
```

完善 `.gitignore`。

## 8.18 Final Verification Gate（客观门槛）

执行：

```bash
go fmt ./... && go vet ./... && go build ./...
go test ./... && go test -race ./...
golangci-lint run
govulncheck ./...
```

环境允许时另加：

```bash
docker build .
docker compose config
kubectl apply --dry-run=client -f deployments/
```

**必须全部满足以下客观条件，否则继续整改：**

```text
[ ] golangci-lint 零告警（不得靠 nolint 掩盖，例外必须逐条说明理由）
[ ] govulncheck 零 HIGH/CRITICAL
[ ] go test -race 全部通过
[ ] 核心包（domain/service/handler）测试覆盖率 >= 70%，并贴出 go test -cover 真实输出
[ ] 直接依赖数量 <= 20，且每一个都在 Dependency Audit 中有存在理由
[ ] 全仓库 grep 不到 global mutable state
[ ] 全仓库 grep 不到 password/token 被写入日志的路径
[ ] 所有已注册 HTTP 路由都存在于 api/openapi.yaml
[ ] goscaffold 生成的项目能通过 go build + go test
[ ] README Quick Start 实际走通（或明确 SKIPPED 及原因）
[ ] STATE.md 与实际代码一致
```

**不要用主观打分（如"架构 9/10"）代替上述客观检查。**
主观评分可以给，但只作为补充说明，不作为通过标准。

## 8.19 最终交付报告

输出正式工程审计报告：

```markdown
# Enterprise Go Scaffold Final Report

## 1. Architecture
## 2. Tech Stack
## 3. Directory Structure
## 4. APIs
## 5. Authentication & Authorization
## 6. Rate Limiting
## 7. Database
## 8. Cache
## 9. Messaging
## 10. Observability
## 11. Testing
## 12. Docker
## 13. Kubernetes
## 14. CI/CD
## 15. Scaffold CLI
## 16. Security
## 17. Performance
## 18. Dependency Review
## 19. Commands Executed（真实输出 / SKIPPED 说明）
## 20. Final Verification Gate 检查表（逐项 pass/fail + 证据）
## 21. Known Limitations
## 22. Recommended Future Work
```

**最终目标：**

> 一套团队可以真正拿去创建新企业 Go 服务的生产级脚手架，
> 而不是展示性质的 Demo。
