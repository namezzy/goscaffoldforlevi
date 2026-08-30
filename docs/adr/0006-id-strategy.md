# ADR-0006: ID 策略

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

主键类型影响索引性能、API 安全、分布式友好度和运维复杂度，且几乎不可能事后更换。
需要在 `BIGSERIAL / UUIDv4 / UUIDv7 / ULID / Snowflake` 中选一个默认值。

## Decision

**UUIDv7，应用侧生成，PostgreSQL 中存为原生 `uuid` 类型，API 中以标准文本形式暴露。**

```sql
id uuid PRIMARY KEY   -- 无 DEFAULT，由应用提供
```

```go
// internal/... 统一的 ID 生成入口，不散落在 handler
func NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }
```

### 对比

| 维度 | BIGSERIAL | UUIDv4 | **UUIDv7** | ULID | Snowflake |
|------|-----------|--------|-----------|------|-----------|
| 全局唯一 | 否（需协调） | 是 | **是** | 是 | 是（需 worker id 分配） |
| 时间有序 | 是 | **否** | **是** | 是 | 是 |
| B-tree 插入局部性 | 最好 | **最差** | **好** | 好 | 好 |
| 存储（PG） | 8 B | 16 B | **16 B** | 16 B（但无原生类型） | 8 B |
| PG 原生类型 | `bigint` | `uuid` | **`uuid`** | 无（需 bytea/text） | `bigint` |
| API 暴露安全性 | **差**（可枚举、泄漏业务量） | 好 | **好** | 好 | 中（可推算时间和节点） |
| 写入前可知 ID | 否（需 RETURNING） | 是 | **是** | 是 | 是 |
| 运维复杂度 | 最低 | 低 | **低** | 低 | **高**（节点 ID 分配、时钟回拨） |

### 决定性理由

**1. 排除 BIGSERIAL：API 暴露不安全。**
自增 ID 暴露在 URL 中，攻击者可以枚举 `/users/1`, `/users/2`...，
并从 ID 值推算出注册用户总量和增长速率。加"ID 混淆层"是额外复杂度，不如一开始就用 UUID。

**2. 排除 UUIDv4：索引局部性差。**
完全随机的主键让每次插入落在 B-tree 的随机页上，
导致页分裂频繁、缓冲区命中率下降、WAL 放大。在千万行量级会明显劣化写入吞吐。
UUIDv7 的前 48 位是毫秒时间戳，插入基本追加到 B-tree 右端，行为接近自增。

**3. 排除 Snowflake：运维成本不匹配。**
需要给每个实例分配唯一 worker ID（配置或协调服务），
还要处理时钟回拨。这是为"每秒百万级 ID"设计的方案，
本脚手架的目标场景用不上，却要承担全部运维负担。

**4. UUIDv7 优于 ULID：PostgreSQL 原生支持。**
两者算法上高度相似（时间戳前缀 + 随机后缀）。差别在于：
UUIDv7 是 **RFC 9562 标准**，可以直接存进 PostgreSQL 的 `uuid` 类型
（16 字节、有比较运算符、有 GiST/BRIN 支持、psql 显示正常）。
ULID 没有原生类型，只能存 `bytea`（不可读）或 `text`（26 字节 + 排序规则问题）。
ULID 的 Base32 表示更短更易读，但这不足以抵消类型系统的损失。

**5. 应用侧生成而非 `gen_random_uuid()`。**
这一点很关键：应用侧生成让**实体在写入数据库之前就拥有 ID**，
从而可以：

- 在同一事务中把 ID 写进 outbox 事件（ADR-0009）
- 在日志和 trace 中提前关联
- 避免 `INSERT ... RETURNING id` 的往返依赖
- 让业务逻辑的单元测试不需要数据库

代价是数据库层失去了"忘记给 ID 也能插入"的保护——
通过 `id uuid PRIMARY KEY`（NOT NULL 隐含）在 schema 层兜住。

### 类型封装

业务包中不直接使用 `uuid.UUID`，而是定义具名类型：

```go
// internal/user
type ID uuid.UUID
```

理由：防止 `user.ID` 与 `auth.SessionID` 互相误传（编译期拦截）。

代价：需要写 `String()` / `MarshalJSON()` / `Scan()` / `Value()`，约 30 行样板。

> 注：这引入了一个 `google/uuid` 依赖出现在业务包中的问题。
> `google/uuid` 是纯数据类型库（无 I/O、无框架），
> 与 ADR-0002 禁止的"基础设施依赖"不同类，因此允许。
> `depguard` 规则不拦截它。

### 时间字段

`created_at` / `updated_at` 使用 `timestamptz`，由**数据库**填充
（`DEFAULT now()` 和触发器或显式 `now()`）。

与 ID 相反，时间戳交给数据库的理由：多实例的时钟可能漂移，
而排序和审计需要单一时间源。UUIDv7 内嵌的时间戳仅用于索引局部性，
**不作为业务时间使用**（毫秒精度，且不同实例生成，不可靠）。

## Consequences

### 正面

- 索引插入局部性接近自增，写入性能可预期
- API 中不泄漏业务量、不可枚举
- 分布式友好，将来分库/多写入点不需要改 ID 方案
- 应用侧生成使 outbox、日志关联、单元测试都变简单
- 使用 PostgreSQL 原生 `uuid`，运维和调试体验正常

### 负面 / 代价

- 主键 16 字节 vs BIGSERIAL 的 8 字节；所有外键索引也翻倍。
  在千万行 + 多外键的表上，索引体积增长可感知
- UUID 文本形式 36 字符，日志和 URL 比自增 ID 长
- 人工排查时"ID 是多少"不再是可口述的短数字
- UUIDv7 暴露了创建时间（毫秒级）。对本项目可接受；
  若某类资源的创建时间属于敏感信息，该表应单独评估

### 后续影响

- Phase 2 需提供统一的 `NewID()` 入口
- Phase 3 的 users 表使用 `id uuid PRIMARY KEY`，migration 中体现
- Phase 3 必须保证 ID 生成不出现在 handler 中（由 Service 或实体构造函数负责）
- Phase 4 的 outbox event_id 同样使用 UUIDv7
- Phase 8 审计 `SELECT` 时确认没有依赖 ID 的隐式排序假设之外的用法

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| BIGSERIAL + 对外暴露 hashid | 存储最小、索引最好 | 需要维护映射层，双向转换到处调用 | 复杂度转移而非消除 |
| BIGSERIAL 内部 + UUID 外部列 | 两全 | 每行两个唯一键，两套索引，join 时要选对 | 收益不抵成本 |
| UUIDv4 | 最简单、库支持最广 | 索引局部性差 | 写入性能不可接受 |
| ULID | 更短的文本形式 | 无 PG 原生类型 | 类型系统损失大于可读性收益 |
| Snowflake | 8 字节 + 有序 | 节点 ID 分配、时钟回拨 | 运维成本与场景不匹配 |

## When to Revisit

- 单表超过 5 亿行且索引体积成为主要成本 → 评估该表改用 BIGSERIAL + 外部 ID 列
- 出现"创建时间必须保密"的资源类型 → 该表改用 UUIDv4
- 引入需要跨表全局单调序号的场景（如账本流水号）→ 单独设计序列，不改主键
