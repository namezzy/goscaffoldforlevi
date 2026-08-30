# Phase 5 — Observability

先读取 `STATE.md`。

目标：让项目默认具备 `Logs / Metrics / Tracing / Request Correlation / Dependency Observability`，
但**不污染业务逻辑**。

## 5.1 OpenTelemetry

配置：`service.name / service.version / deployment.environment / OTLP endpoint / sampling / enabled`。

**Observability 必须可以关闭；关闭时业务功能完全正常。**

## 5.2 Trace Provider

统一 `NewTracerProvider / Shutdown`。在 OTLP gRPC / HTTP 中选一个默认。

## 5.3 HTTP Tracing

记录 `method / route / status / duration`。

Span name 必须使用路由模板：

```text
GET /api/v1/users/{id}     ✅
GET /api/v1/users/8c91...  ❌ 高基数
```

## 5.4 Trace Context

支持 W3C Trace Context，跨 HTTP / service / database / redis / **消息队列** / external API 传播。

## 5.5 Database Tracing

为 PostgreSQL 添加 tracing。

**不得记录**：password / 完整 SQL 参数 / PII。
如果 instrumentation 自动记录语句，必须确认不会泄漏敏感数据。

## 5.6 Redis Tracing

记录 `command name / latency / error`。
**不得记录** token / session content / user PII。

## 5.7 Messaging Tracing

Producer span 与 Consumer span 通过 traceparent 关联。

验证：一次 `POST /api/v1/users` 能在 trace 中看到
`HTTP Span → Service Span → PostgreSQL Span → Publish Span → Consumer Span`。

## 5.8 Logging Correlation

日志自动包含 `request_id / trace_id / span_id`。

**不得让每个 Service 手动提取 trace_id。**

## 5.9 Metrics

技术指标：

```text
http_requests_total / http_request_duration_seconds / http_errors_total
db_pool_* / redis_operation_*
messaging_published_total / messaging_consumed_total
messaging_consume_duration / messaging_retry_total / messaging_dlq_total
rate_limit_rejected_total
```

业务指标：

```text
user_registration_total / login_total / login_failure_total
```

## 5.10 Prometheus

提供 `/metrics`。分析应该 public expose / internal only / 独立 admin port，并给出生产建议。

## 5.11 Label Cardinality

**禁止**将 `user_id / email / request_id / full URL / token` 作为 Metrics Label。

Route 一律使用模板形式。

## 5.12 Structured Logging 约定

日志等级约定：

```text
4xx 不一定是 ERROR
5xx 是 ERROR
expected not found 不是 ERROR
限流拒绝是 WARN 或 INFO，不是 ERROR
```

## 5.13 Sensitive Data 复查

再次全量检查日志与 trace，禁止出现：

```text
password / password_hash / JWT / refresh token / Authorization / Cookie
database password / Redis password / broker credentials
```

## 5.14 Observability Docker Stack

Docker Compose 增加可选 profile：

```bash
docker compose --profile observability up -d
```

包含：OpenTelemetry Collector / Prometheus / Grafana / Tempo 或 Jaeger。

**不要默认开发时全部启动。**

## 5.15 Dashboard

提供至少一个基础 Grafana Dashboard 配置或文档，关注：

```text
RPS / Latency P50 P95 P99 / HTTP 5xx / DB Connections
Login failures / User registrations / Consumer lag / DLQ size
```

## 5.16 Tests

```text
request ID propagation
trace context propagation（含 HTTP → messaging）
metrics endpoint
observability disabled 时应用正常工作
sensitive data not logged
```

## 5.17 Phase 5 输出

执行 §0.4 全部门禁。若 Docker 可用，验证 Collector / Prometheus / Grafana / Tempo 能启动；
不可用则 SKIPPED。输出：

```text
Telemetry Architecture
Metrics Added
Trace Coverage
Logging Changes
Docker Observability Stack
Tests / Verification Results
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。
