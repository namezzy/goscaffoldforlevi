# ADR-0005: 数据访问与事务

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

需要确定连接池、查询生成、事务管理、迁移的组织方式，并回答一个关键问题：
**Transaction Manager 是否必要？** 如果为了"展示事务"而引入抽象，就是过度设计。

## Decision

### 连接池

`pgxpool`，参数与默认值：

| 参数 | 默认 | 依据 |
|------|------|------|
| `MaxConns` | 10 | PostgreSQL 默认 `max_connections=100`。假设 4 个 API Pod + 2 个 worker，10×6=60，为迁移 Job 和人工连接留余量 |
| `MinConns` | 2 | 保持少量热连接，避免冷启动时建连风暴 |
| `MaxConnLifetime` | 1h | 让连接定期轮转，配合滚动升级和 DNS 变更 |
| `MaxConnIdleTime` | 30m | 回收闲置连接 |
| `HealthCheckPeriod` | 1m | 及时剔除坏连接 |
| `ConnectTimeout` | 5s | 启动时快速失败 |

全部可通过环境变量覆盖。**`MaxConns` 必须在部署文档中与副本数一起计算**——
这是生产事故的常见来源（Pod 扩容后打爆数据库连接数）。

初始化签名强制带 context：

```go
func NewPool(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error)
```

启动时用带 5s 超时的 context 执行一次 `Ping`，失败则**启动失败**（Fail Fast）。

### 查询：sqlc

- SQL 源文件在 `queries/*.sql`
- 生成物到 `internal/postgres/db/`（package `db`）
- 生成物**提交进 Git**（CI 校验 `make sqlc` 后 `git diff --exit-code` 为空）
- 所有查询显式列出列名，**禁止 `SELECT *`**——
  `SELECT *` 会让加列变成破坏性变更，且掩盖不必要的数据传输

#### 动态查询的处理方式

sqlc 不擅长可选过滤条件。**不引入 query builder**，用 SQL 本身表达：

```sql
-- name: ListUsers :many
SELECT id, email, name, status, created_at, updated_at
FROM users
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;
```

**代价**：`(param IS NULL OR col = param)` 可能导致 PostgreSQL 选错执行计划。
应对：过滤维度超过 3 个、或 EXPLAIN 显示走了全表扫描时，
改为在 `userpg` 包中手写 pgx 查询（sqlc 与手写 pgx 可以共存，
因为 sqlc 生成物接受 `DBTX` 接口）。

> **When to Revisit**：超过 5 个查询因动态条件而必须手写 → 评估引入 builder。

### 事务管理

#### 是否必要：是

不是为了演示。v1 存在**两个真实的原子性需求**：

1. **Refresh token 轮转**（ADR-0007）：插入新 session + 标记旧 session 已用，
   必须原子。中间崩溃会导致用户被登出或旧 token 仍可用（安全问题）。
2. **Outbox 写入**（ADR-0009）：业务写入 + outbox 记录必须在同一事务，
   否则消息可能丢失或产生幽灵事件。

两者都不是可选的，因此 TxManager 是必需品。

#### 接口

```go
// internal/postgres
type TxManager interface {
    WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

- `fn` 返回 error → 回滚；返回 nil → 提交
- `fn` panic → 回滚后重新 panic（不吞掉）
- **不支持嵌套事务**。嵌套调用直接复用外层事务（检测 context 中已有 tx 则不新建），
  不使用 SAVEPOINT。理由：嵌套事务的语义容易被误解，
  且 v1 没有"部分回滚"的需求

#### 实现：context 携带 tx

```go
type txKey struct{}   // 未导出，仅 internal/postgres 可见

func (m *manager) WithTx(ctx context.Context, fn func(context.Context) error) error {
    if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return fn(ctx) // 复用外层事务
    }
    tx, err := m.pool.Begin(ctx)
    ...
}

// Repository 通过这个 helper 取执行器
func Querier(ctx context.Context, pool *pgxpool.Pool) DBTX {
    if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return tx
    }
    return pool
}
```

Repository 方法签名保持干净：

```go
func (r *Repository) Create(ctx context.Context, u user.User) error {
    q := db.New(postgres.Querier(ctx, r.pool))
    ...
}
```

**`pgx.Tx` 不出现在任何 Repository / Service 的方法签名中。**

#### 对"Explicit > Magic"的诚实交代

把 tx 藏在 context 里是一种隐式传递，与项目原则有张力。

接受它的理由：另一种做法是每个 Repository 方法多一个 `tx` 参数，
或者提供 `Repo` / `TxRepo` 两套实现——前者污染全部签名（约 12 个方法），
后者代码翻倍。相比之下，隐式的**作用范围被严格限制**：
context key 未导出，只有 `internal/postgres` 能读写，
Service 层看到的仍然是普通的 `ctx`。

风险缓解：`Querier` 是唯一入口，Phase 8 审计时 grep 确认没有绕过它的路径。

#### 事务使用纪律

- 事务内**禁止**发起 HTTP 调用、发布消息到 broker、执行长计算
- 事务应尽可能短；跨越用户输入的事务绝对禁止
- 隔离级别用默认 `READ COMMITTED`。需要更强隔离时在该查询处显式说明理由

### 迁移

- `golang-migrate`，文件在 `migrations/`，命名 `{timestamp}_{name}.{up|down}.sql`
- **每个 up 必须有可用的 down**，Phase 8 会验证回滚
- 迁移与应用部署解耦：**API Pod 启动时不执行迁移**

不自动迁移的理由（写入 `docs/deployment.md`）：

1. 多副本同时启动会并发执行迁移，golang-migrate 的锁能防止损坏但会导致启动串行化和超时
2. 迁移失败会变成 CrashLoopBackOff，故障表现被掩盖成"应用起不来"
3. 回滚应用版本不会回滚 schema，两者生命周期不同
4. 破坏性迁移需要人工确认时机（如加索引锁表）

执行方式：CI/CD 中的独立步骤，或 Kubernetes `Job`（Phase 6 提供）。

### Repository 的职责边界

`userpg` / `authpg` 只做三件事：

1. 调用 sqlc 生成的查询
2. 在 sqlc 的 row 类型与业务实体之间转换
3. **翻译数据库错误**为 `apperr`（ADR-0003）

不做：业务判断、跨表编排、缓存。

## Consequences

### 正面

- SQL 完全可见可评审，执行计划可预测
- 事务 API 只有一个方法，调用点一目了然
- `pgx.Tx` 不泄漏到业务层，Service 的单元测试用 fake repo 即可
- 迁移与部署解耦，故障边界清晰

### 负面 / 代价

- tx 藏在 context 中是隐式行为，新人需要读一次文档才理解
- sqlc 是构建期依赖，本地和 CI 都要安装并保持版本一致
- 生成物入库会让 diff 变大
- 动态查询需要 `IS NULL OR` 技巧，有执行计划风险
- 不支持嵌套事务，将来若需要要改实现

### 后续影响

- Phase 2 落地 pool、TxManager、sqlc/migrate 工具链
- Phase 3 的 refresh token 轮转是第一个真实事务用例
- Phase 4 的 outbox 依赖 TxManager
- Phase 6 提供 migration Job
- Phase 8 审计 N+1、连接泄漏、`SELECT *`、down 迁移可用性

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| 显式传 `pgx.Tx` 参数 | 完全显式 | 污染全部方法签名，Service 层被迫认识 pgx | 违反依赖方向 |
| `Repo` + `TxRepo` 双实现 | 显式且不污染签名 | 代码量翻倍，两套实现易不一致 | 维护成本过高 |
| Unit of Work 模式 | 抽象完整 | 对 Go 而言过重，且需要仓储注册表 | 过度设计 |
| GORM 事务 | 开箱即用 | 见 ADR-0001 | 已否决 GORM |
| 应用启动自动迁移 | 部署简单 | 见上述 4 条理由 | 生产不可接受 |

## When to Revisit

- 需要 SAVEPOINT / 部分回滚 → 重新设计 TxManager
- 出现跨库/跨服务事务需求 → 引入 Saga 或 outbox 编排，不要扩展本 TxManager
- `MaxConns × 副本数` 逼近数据库 `max_connections` 的 70% → 引入 PgBouncer
