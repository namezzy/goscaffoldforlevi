# Phase 4 — Messaging / 异步处理

先读取 `STATE.md` 和 `docs/adr/0009-messaging.md`。

**前置判断**：如果 Phase 1 的 ADR-0009 结论是"第一版不引入消息队列"，
则本阶段只实现**接口抽象 + 内存实现 + 测试**，并跳过 4.3 之后的 broker 相关内容，
在 `STATE.md` 中记录跳过理由。否则完整执行。

目标：让项目具备可靠的异步能力，同时**业务层完全不感知 broker**。

## 4.1 Publisher 抽象

在 application 层定义消费者视角的小接口，例如：

```go
type EventPublisher interface {
    Publish(ctx context.Context, event Event) error
}
```

要求：

```text
业务代码不得 import kafka / nats SDK
Event 结构不得直接暴露 broker 概念（partition/offset/subject）
提供内存实现用于单元测试
```

## 4.2 消息契约

设计 Event 信封：

```text
event_id (唯一, 用于幂等)
event_type (含版本，例如 user.created.v1)
occurred_at
trace_context (W3C traceparent)
payload
```

说明 schema 演进策略：新增字段向后兼容、破坏性变更必须升版本号。

**payload 中禁止出现**：password / password_hash / token / 完整 PII。

## 4.3 Broker 接入

按 ADR-0009 选定的 broker 实现 Producer / Consumer。

配置项：`brokers / topic 或 stream / consumer group / 超时 / 重试参数 / enabled`。

**`enabled=false` 时，整个应用必须仍能正常启动并提供 HTTP 服务。**

## 4.4 Outbox Pattern

如果业务需要"数据库写入与消息发布"的原子性：

```text
业务事务内写 outbox 表
独立 relay 进程/goroutine 读取并投递
投递成功后标记
失败重试，带退避
```

**如果 ADR-0009 判断第一版不需要 Outbox，请明确说明放弃它的后果**
（消息可能丢失 / 双写不一致），并写入 Known Limitations。

## 4.5 Consumer

实现至少一个真实消费者，例如 `user.created.v1` → 发送欢迎邮件（可用 no-op 邮件实现）。

要求：

```text
每条消息一个独立 context，带超时
handler 返回 error 时进入重试
panic 必须被捕获，不能杀死 consumer
consumer 生命周期由 bootstrap 管理，不用 init()
```

## 4.6 幂等消费

实现去重：基于 `event_id`，存储在 Redis 或数据库。

明确说明：**至少一次投递（at-least-once）是默认语义，消费者必须自己保证幂等。**

## 4.7 重试与 DLQ

```text
指数退避 + 抖动
最大重试次数
超过上限 → Dead Letter Queue / dead letter 表
DLQ 消息必须记录失败原因和原始 payload
```

提供查看/重放 DLQ 的最小手段（CLI 命令或 SQL 说明即可）。

## 4.8 优雅退出

收到 SIGTERM 后：

```text
停止拉取新消息 → 等待 in-flight 消息处理完成（带超时）→ 提交 offset/ack → 关闭连接
```

必须有测试覆盖"退出时不丢消息"。

## 4.9 Trace 传播

Publish 时注入 W3C traceparent，Consume 时提取并作为 span 的 parent。

**目标：一次 HTTP 请求触发的异步处理，在 trace 中是同一条链路。**
（具体 instrumentation 在 Phase 5 完成，本阶段先保证 context 字段被正确传递和读取。）

## 4.10 Tests

```text
Publisher 单元测试（内存实现）
Consumer handler 单元测试
幂等去重测试
重试与 DLQ 测试
优雅退出测试
Broker 集成测试（testcontainers；不可用则 SKIPPED）
```

## 4.11 Docker Compose

将 broker 加入 compose，但放在**独立 profile** 中，不默认启动：

```bash
docker compose --profile messaging up -d
```

## 4.12 Phase 4 输出

执行 §0.4 全部门禁。输出：

```text
Messaging Architecture
Event Contracts
Delivery Guarantees（明确说明是 at-least-once / at-most-once）
Idempotency Strategy
Retry & DLQ Design
Tests
Commands Run
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。
