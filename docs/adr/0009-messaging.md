# ADR-0009: 消息与异步处理

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

Prompt 要求脚手架具备消息队列能力。但项目原则同时要求
"避免为了演示某个模式而增加复杂度"。这两者需要一个诚实的调和。

## Decision

### 先回答那个必须回答的问题

> **这个脚手架的 v1 是否真的需要消息队列？**

**业务上不需要。** User CRUD + Auth 全部可以同步完成，
没有任何一个用例因为缺少 broker 而无法实现。

**但仍然纳入**，理由与"演示"无关：

1. 企业后端在**第一次**需要异步时（发邮件、审计日志、通知下游、数据同步），
   通常已经上线。此时补齐 outbox + 消费者 + 幂等 + DLQ 是一次高风险改造。
2. 脚手架的价值恰恰在于**把这类基础设施决策提前做对**，
   而不是把它留给一个在压力下赶工的人。
3. 真正要避免的是"业务代码耦合 broker"——这一点通过接口抽象解决，
   而不是通过不做。

**因此的折中**：

```text
默认 MESSAGING_ENABLED=false
  → EventPublisher 使用 outbox-only 实现（写库，不投递）
  → 或 noop 实现（连 outbox 都不写）
  → 应用完全正常工作，worker 进程可以不部署

MESSAGING_ENABLED=true
  → outbox relay 启动，投递到 NATS JetStream
  → consumer 启动
```

**默认关闭是关键**：使用这个脚手架创建的新服务，
如果不需要异步，它不会被迫运行一个 NATS。

### Broker：NATS JetStream

选型见 ADR-0001。核心理由是运维成本：单个 ~15MB 二进制、
一个容器起、无 ZooKeeper/KRaft、无分区 rebalance。

### Publisher 抽象

```go
// internal/messaging —— 但接口由消费者(业务包)声明，见下
type Event struct {
    ID          uuid.UUID       // UUIDv7，幂等键
    Type        string          // "user.created.v1"
    OccurredAt  time.Time
    TraceParent string          // W3C traceparent
    Payload     json.RawMessage
}
```

业务包按 ADR-0002 的原则声明自己需要的接口：

```go
// internal/user/service.go
type EventPublisher interface {
    Publish(ctx context.Context, e messaging.Event) error
}
```

**约束**：

- 业务包**不得** import `nats.go`（由 depguard 强制）
- `Event` 中不出现 subject / stream / partition / sequence 等 broker 概念
- 提供三个实现：`nats`（生产）、`outbox`（写库）、`memory`（测试）

> 注：`Event` 类型放在 `internal/messaging` 会让业务包 import 它。
> 这是可接受的——`messaging.Event` 是纯数据结构（无 I/O、无 SDK 依赖），
> 与 `apperr` 同类。broker SDK 只出现在 `messaging/nats.go`。

### 事件契约

`Type` 格式：`{aggregate}.{action}.v{n}`，如 `user.created.v1`。

Schema 演进规则：

- **新增可选字段** → 兼容，不升版本
- **删除字段 / 改类型 / 改语义** → 破坏性，必须发布 `v2`，
  并在过渡期**同时发布 v1 和 v2**，直到所有消费者迁移完毕

Payload **禁止**包含：`password`、`password_hash`、任何 token、
完整 PII（邮箱**允许**，因为下游发邮件需要；但需在 `docs/security.md` 中登记）。

原则：payload 应尽量小，只带 ID 和必要字段，消费者需要更多数据时回查。
这样既减少 PII 扩散，也避免消息中的数据陈旧。

### Outbox Pattern：v1 包含

#### 为什么不能跳过

朴素做法是"事务提交后再 publish"。故障场景：

```text
BEGIN; INSERT user; COMMIT;   ← 成功
  ★ 进程在此崩溃 / broker 不可达
publish(user.created)          ← 从未发生
```

结果：用户已创建，但下游永远收不到事件，且**没有任何痕迹**——
这类不一致只能靠人工对账发现。

反过来"先 publish 再 commit"会产生幽灵事件（事件说创建了，实际回滚了）。

Outbox 用一张表把两者变成一次原子写入：

```text
BEGIN;
  INSERT INTO users ...;
  INSERT INTO outbox_events (id, type, payload, traceparent, created_at);
COMMIT;                        ← 原子

relay goroutine（独立于请求）：
  SELECT ... FROM outbox_events
   WHERE published_at IS NULL
   ORDER BY created_at
   FOR UPDATE SKIP LOCKED
   LIMIT 100;
  → publish → UPDATE published_at = now()
```

`FOR UPDATE SKIP LOCKED` 让多个 relay 实例可以并行拉取而不冲突。

投递语义：**at-least-once**。
relay 可能在 publish 成功但 UPDATE 前崩溃，导致重复投递——
这由消费者侧的幂等处理（见下）兜住。

#### 代价

- 一张表 + 一个后台 goroutine + 定期清理已发布记录（保留 7 天用于排查）
- 消息延迟 = relay 轮询间隔（默认 1s）。
  需要更低延迟时用 `LISTEN/NOTIFY` 唤醒，v1 不做
- outbox 表是写热点，需要 `(published_at, created_at)` 上的部分索引：
  `CREATE INDEX ... ON outbox_events (created_at) WHERE published_at IS NULL`

### 消费者

v1 实现一个真实消费者：`user.created.v1` → 发送欢迎邮件。

邮件发送器在 v1 是 `LogSender`（记日志，不真发）——
**它是一个接口的占位实现，不是假装的功能**，
在 `STATE.md` 的 Not Implemented Yet 中明确登记。

消费者约束：

- 每条消息一个独立 context，带超时（默认 30s）
- handler panic 必须被 recover，**不能杀死 consumer**
- 生命周期由 `cmd/worker/main.go` 管理，**不使用 `init()`**
- 消费并发度可配置，默认 1（顺序处理，最保守）

### 幂等消费

at-least-once 意味着**消费者必须自己保证幂等**。这不是可选项。

实现：

```sql
CREATE TABLE processed_events (
    event_id     uuid PRIMARY KEY,
    consumer     text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now()
);
```

消费流程：

```text
BEGIN;
  INSERT INTO processed_events (event_id, consumer) VALUES (...)
    ON CONFLICT DO NOTHING;
  if 影响行数 == 0 → 已处理过，直接 COMMIT 并 ack；
  执行副作用（必须是同一数据库内的写入）;
COMMIT;
ack;
```

关键点：**去重记录与副作用在同一事务**。
如果副作用是外部调用（发邮件），则无法做到真正的 exactly-once——
此时的保证降级为"至多一次重复"，需要外部系统自身幂等
（如邮件服务支持 idempotency key）。**这个限制必须写进文档，不能含糊。**

Redis 是否参与去重？**不参与。**
Redis 的持久化保证不足以承担"这条消息是否处理过"的权威判断，
且它与副作用不在同一事务中。权威记录只在 PostgreSQL。

`processed_events` 按 `processed_at` 定期清理（保留 30 天，
必须长于消息的最大重投递窗口）。

### 重试与 DLQ

```text
尝试 1 → 失败 → 等 1s   ± 抖动
尝试 2 → 失败 → 等 2s   ± 抖动
尝试 3 → 失败 → 等 4s   ± 抖动
尝试 4 → 失败 → 等 8s   ± 抖动
尝试 5 → 失败 → 进入 DLQ
```

指数退避 + **全抖动**（`sleep = random(0, base * 2^n)`），
避免大量消费者在同一时刻重试造成雪崩。

DLQ 用数据库表而非 NATS stream：

```text
dead_letter_events(
  id, event_id, event_type, payload, traceparent,
  consumer, attempts, last_error, failed_at
)
```

理由：DLQ 的核心用途是**人工排查和重放**，需要 SQL 查询能力
（按类型统计、按时间范围筛选、看错误分布）。
放在 broker 里只能顺序消费，排查体验差。

最小运维手段（v1）：

```bash
goscaffold dlq list --type user.created.v1   # Phase 7 提供
goscaffold dlq replay <id>
```

Phase 4 先提供 SQL 说明 + 一个 `make dlq-list` 目标，CLI 命令在 Phase 7 补。

### 优雅退出

```text
SIGTERM
  → 停止从 JetStream 拉取新消息
  → 等待 in-flight 消息处理完成（超时 = shutdown timeout - 5s）
  → 超时未完成的消息：不 ack，让它重投递（依赖幂等）
  → 停止 outbox relay（当前批次完成后退出）
  → 关闭 NATS 连接
  → 关闭数据库连接池
```

**不丢消息的依据**：未 ack 的消息会被 JetStream 重投递；
未发布的 outbox 记录仍在表中，下次启动继续。
两条路径都不依赖"退出时处理完"。

必须有测试覆盖：退出过程中 in-flight 消息不会丢失。

### Trace 传播

Publish 时把当前 span 的 W3C `traceparent` 写入 `Event.TraceParent`
（对 outbox 而言是写入表的列）。
Consume 时提取并作为新 span 的 parent（用 span link 还是 parent？
选 **parent**，因为消费是该请求逻辑的延续；
若一条消息被多个消费者处理，各自形成独立分支）。

效果：一次 `POST /api/v1/users` 在 trace 中呈现为
`HTTP → Service → PG → Outbox Insert ... (异步) Relay Publish → Consumer → Send Email`。

Phase 4 只保证字段被正确写入和读出；实际 instrumentation 在 Phase 5。

## Consequences

### 正面

- 业务代码零 broker 耦合，换 Kafka 只改一个包
- outbox 消除了"数据写了但事件丢了"这类静默不一致
- 幂等 + DLQ 让 at-least-once 可以安全使用
- 默认关闭，不需要异步的服务不承担任何成本

### 负面 / 代价

- 三张额外的表（outbox / processed / DLQ）+ 定期清理任务
- 消息延迟至少一个 relay 轮询周期（默认 1s）
- 外部副作用（发邮件）无法真正 exactly-once，只能降级到"外部系统幂等"
- 多一个部署单元（`cmd/worker`）
- outbox 表是写热点，高 TPS 下需要分区或归档策略
- 开发者必须理解 at-least-once 语义，写消费者时不能假设只收到一次

### 后续影响

- Phase 2 **不涉及** messaging
- Phase 3 的 CreateUser 在同一事务中写 outbox（这是 TxManager 的第二个用例）
- Phase 4 完整实现本 ADR
- Phase 5 增加 `messaging_*` 指标与 producer/consumer span
- Phase 6 需要独立的 worker Deployment 和 NATS 的 compose profile
- Phase 8 审计消息丢失路径、消费者泄漏、DLQ 可运维性

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| v1 完全不做消息 | 最简单 | 第一次需要异步时是高风险改造 | 用户明确要求纳入；且默认关闭已消除成本 |
| Kafka | 吞吐、生态、保留 | 运维负担大 | 与目标团队规模不匹配 |
| Redis Streams | 复用已有 Redis | 持久性弱；与限流争抢同一实例 | 可靠性不足 |
| 不做 outbox，commit 后 publish | 少一张表 | 崩溃即静默丢消息 | 不满足"production-ready" |
| DLQ 放 broker stream | 与消息系统一致 | 无法 SQL 排查 | 运维体验差 |
| Redis 做幂等去重 | 快 | 与副作用不同事务，持久性弱 | 正确性不足 |
| 事务性 outbox + CDC(Debezium) | 无轮询延迟 | 引入 Kafka Connect 全套 | 远超本项目复杂度预算 |

## When to Revisit

- outbox 轮询延迟成为业务问题 → 加 `LISTEN/NOTIFY` 唤醒
- outbox 表写入成为瓶颈（> 1000 events/s）→ 按时间分区 + 定期 DETACH
- 需要消息重放 / 长期保留 / 流处理 → 换 Kafka
- 出现多个服务共享同一套事件 → 需要 schema registry 和契约测试
