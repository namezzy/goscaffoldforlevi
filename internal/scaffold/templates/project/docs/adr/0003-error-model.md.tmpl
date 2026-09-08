# ADR-0003: 错误模型

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

需要一套错误体系，满足：

- 业务层完全不知道 HTTP 状态码的存在
- 支持 `errors.Is` / `errors.As` / `%w`
- 内部错误细节（SQL、栈、路径、凭据）绝不泄漏给客户端
- 客户端能拿到稳定的机器可读错误码用于分支处理
- 校验错误能定位到字段

## Decision

### 两个维度：Kind + Code

```go
// internal/apperr
type Kind uint8

const (
    KindInternal Kind = iota // 零值 = 最保守的分类
    KindInvalidArgument
    KindNotFound
    KindUnauthorized
    KindForbidden
    KindConflict
    KindRateLimited
)

type Error struct {
    Kind    Kind
    Code    string   // 稳定的机器码，如 "USER_EMAIL_TAKEN"
    Message string   // 面向客户端的安全文案
    Details []Detail // 可选，字段级信息
    cause   error    // 内部原因，只进日志，绝不出网
}

type Detail struct {
    Field  string `json:"field"`
    Reason string `json:"reason"`
}

func (e *Error) Error() string { ... }
func (e *Error) Unwrap() error { return e.cause }
```

- **`Kind`** 决定 HTTP 状态码，由 transport 层消费。业务层只赋值，不关心映射。
- **`Code`** 是对外契约的一部分，写进 OpenAPI，前端可以据此分支。
- **`Message`** 必须假定会被用户看到，不得包含内部信息。
- **`cause`** 通过 `Unwrap` 暴露给 `errors.Is/As` 和日志，但序列化时被丢弃。

`KindInternal` 是零值，保证**未显式分类的错误默认按最保守方式处理**（500 + 不泄漏）。

### 构造与判定

```go
func New(kind Kind, code, msg string) *Error
func Wrap(err error, kind Kind, code, msg string) *Error

// 便捷构造
func NotFound(code, msg string) *Error
func Invalid(code, msg string, details ...Detail) *Error
func Conflict(code, msg string) *Error
func Unauthorized(code, msg string) *Error
func Forbidden(code, msg string) *Error
func Internal(err error) *Error   // Message 固定为 "internal error"

// 判定：走完整 Unwrap 链
func KindOf(err error) Kind
func Is(err error, k Kind) bool
```

**不使用 `ErrNotFound` 这类哨兵变量作为主要机制。** 理由：哨兵值无法携带
Code / Details，且 `errors.Is(err, ErrNotFound)` 与 `KindOf(err) == KindNotFound`
表达力相同而后者能同时拿到码。哨兵仅在需要精确等值比较时补充定义。

### 错误流

```text
┌──────────────────────────────────────────────────────────────┐
│ userpg.Repository                                            │
│   pgx.ErrNoRows            → apperr.NotFound("USER_NOT_FOUND")│
│   23505 unique_violation   → apperr.Conflict("USER_EMAIL_TAKEN")│
│   其他 pgconn.PgError      → apperr.Internal(err)  (Wrap 原因) │
│   ★ 数据库错误码的知识只存在于这一层                            │
└───────────────────────────┬──────────────────────────────────┘
                            ▼
┌──────────────────────────────────────────────────────────────┐
│ user.Service                                                 │
│   业务规则违反 → apperr.Invalid / Forbidden / Conflict         │
│   Repository 返回的 apperr 直接向上传递（可用 %w 补充上下文）    │
│   ★ 不判断 pgx / PostgreSQL 错误码                             │
└───────────────────────────┬──────────────────────────────────┘
                            ▼
┌──────────────────────────────────────────────────────────────┐
│ httpx.WriteError(w, r, err)                                  │
│   kind := apperr.KindOf(err)                                 │
│   status := statusOf(kind)                                   │
│   if kind == KindInternal:                                   │
│       log.Error(cause, stack, request_id)   ← 完整信息进日志   │
│       body.Message = "internal error"       ← 出网信息脱敏     │
│   else:                                                      │
│       log.Warn/Info(...)                                     │
│   写出 JSON + request_id                                      │
└──────────────────────────────────────────────────────────────┘
```

### Kind → HTTP 状态码

| Kind | HTTP | 日志级别 |
|------|------|---------|
| `KindInvalidArgument` | 400 | INFO |
| `KindUnauthorized` | 401 | INFO |
| `KindForbidden` | 403 | WARN |
| `KindNotFound` | 404 | INFO |
| `KindConflict` | 409 | INFO |
| `KindRateLimited` | 429 | WARN |
| `KindInternal` | 500 | ERROR |

**映射表只存在于 `internal/httpx` 一个文件中。**

日志级别的取舍：4xx 是客户端问题，不是系统故障，记为 ERROR 会污染告警。
只有 5xx 才是 ERROR。403/429 记 WARN 因为它们可能是攻击信号。

### 线上格式

```json
{
  "error": {
    "code": "USER_EMAIL_TAKEN",
    "message": "email already registered",
    "details": [
      { "field": "email", "reason": "already_registered" }
    ]
  },
  "request_id": "01J8Z9K3M4N5P6Q7R8S9T0V1W2"
}
```

`details` 仅在 `KindInvalidArgument` 与部分 `KindConflict` 出现，`KindInternal` 永不出现。

### 500 响应的信息边界

`KindInternal` 的响应体**固定**为：

```json
{ "error": { "code": "INTERNAL", "message": "internal error" }, "request_id": "..." }
```

客户端拿到 `request_id`，运维用它在日志里定位真实原因。
这是唯一的关联手段——**绝不通过响应体传递内部信息**。

明确禁止出现在响应中的内容：SQL 语句、表名/列名、pgx 错误文本、文件路径、
栈帧、Redis/NATS 地址、配置键名、任何凭据。

### panic

panic 不是错误模型的一部分。Recovery 中间件捕获后：
记录 `panic value + stack + request_id`（ERROR），返回标准 500 响应体。

## Consequences

### 正面

- 业务层与 HTTP 完全解耦，同一套 Service 可被 gRPC / CLI 复用
- 数据库错误码的知识被限制在 `*pg` 包，符合 ADR-0002 的依赖方向
- 默认值安全：忘记分类 → 500 + 脱敏，而不是意外泄漏
- 错误码进 OpenAPI，前端有稳定契约

### 负面 / 代价

- 每个 Repository 方法都要写错误翻译，样板代码增加
- `Code` 字符串常量需要集中维护，否则会出现拼写不一致
  → 约定：每个业务包定义 `const CodeXxx = "..."`，OpenAPI 中列举
- 客户端调试 500 时只能拿到 request_id，需要运维配合

### 后续影响

- Phase 2 落地 `apperr` + `httpx` 映射 + 对应测试
- Phase 3 每个业务错误都要定义 Code 并写入 `api/openapi.yaml`
- Phase 5 日志级别约定依赖本表
- Phase 8 安全审计逐项检查"响应中不含内部信息"

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| 纯哨兵错误 (`var ErrNotFound = errors.New(...)`) | 最简单、标准库风格 | 无法携带 Code/Details，每个场景要新建哨兵 | 表达力不足以支撑 API 契约 |
| 每层独立错误类型 + 逐层转换 | 层次纯粹 | 转换样板爆炸，且信息在转换中丢失 | 收益与成本不成比例 |
| 直接返回 HTTP 状态码 | 少一层映射 | 业务层耦合 HTTP，无法复用到非 HTTP 入口 | 被 prompt 明确禁止 |
| gRPC `codes.Code` 风格 | 已有成熟枚举 | 引入 grpc 依赖，且语义不完全贴合 HTTP | 自定义 7 个 Kind 已够用 |

## When to Revisit

- 需要错误信息国际化时（`Message` 要变成 key + 参数）
- 引入 gRPC 入口时（需要新增 Kind → gRPC code 映射，Kind 枚举可能要扩充）
- `Code` 常量超过 50 个时，需要考虑集中注册表 + 唯一性测试
