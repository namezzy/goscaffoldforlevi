# ADR-0008: 限流

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

限流在本项目中**不是可选增强**，它是两个设计的必要前提：

1. Argon2id 每次登录消耗 64 MB 内存（ADR-0007）——
   无限流的登录接口是内存耗尽型 DoS 的直接入口
2. 密码登录接口必须抗暴力破解

需要确定：算法、存储、维度、中间件位置、以及**存储故障时的行为**。

## Decision

### 算法：Token Bucket（Redis + Lua）

```text
key:      rl:{scope}:{identity}
value:    { tokens: float, last_refill: unix_ms }
refill:   tokens = min(burst, tokens + elapsed_ms * rate / 1000)
consume:  if tokens >= 1 { tokens -= 1; allow } else { deny }
TTL:      ceil(burst / rate) 秒后自动过期（避免 key 泄漏）
```

整个读-算-写在**一个 Lua 脚本**中完成，保证原子性（Redis 单线程执行脚本）。

选 token bucket 的理由：

| 算法 | 突发容忍 | 精度 | Redis 成本 | 能否算出 Retry-After |
|------|---------|------|-----------|---------------------|
| 固定窗口 | 差（窗口边界可 2 倍突发） | 低 | 1 key | 可 |
| 滑动窗口日志 | 好 | 精确 | O(N) 内存 | 可 |
| 滑动窗口计数 | 中 | 近似 | 2 key | 近似 |
| **Token Bucket** | **好（burst 可控）** | **好** | **1 key** | **精确** |

决定性因素是**能精确计算 `Retry-After`**：客户端拿到 429 后应该知道等多久，
而不是盲目重试。token bucket 可以直接算出"补满 1 个 token 需要 (1-tokens)/rate 秒"。

### 存储：Redis

必须是 Redis 而非内存：多副本部署时，内存限流器的实际配额 = 配置值 × 副本数，
且 Pod 重启即清零。这两点让内存实现在生产中形同虚设。

这也是 Redis 成为 v1 硬依赖的**唯一真实理由**（见 ADR-0001）。

### 限流维度与默认配额

| Scope | 标识 | rate | burst | 说明 |
|-------|------|------|-------|------|
| `ip` | 客户端 IP | 100 / min | 200 | 全局兜底 |
| `user` | Principal.UserID | 600 / min | 1000 | 已认证用户，配额更高 |
| `login_ip` | 客户端 IP | 10 / 15min | 10 | 抗分布式扫号 |
| `login_id` | `sha256(email)` | **5 / 15min** | 5 | 抗针对单账号的暴力破解 |

`login_id` 用邮箱的哈希而非明文作为 key 的一部分：
Redis 的 key 会出现在 `MONITOR`、慢日志、备份中，明文邮箱是 PII 泄漏面。

登录必须**同时**通过 `login_ip` 和 `login_id` 两个限流器。
只做 IP 限流挡不住分布式撞库；只做账号限流会被用来恶意锁定他人账号
（所以 `login_id` 用限流而非"锁定账号"——限流会自动恢复，锁定需要人工解锁，
后者可被武器化为拒绝服务）。

全部配额可通过环境变量调整，可整体 `RATELIMIT_ENABLED=false` 关闭。

### 客户端 IP 的获取

**这是一个安全敏感点。** `X-Forwarded-For` 可被客户端伪造。

规则：

- 配置项 `HTTP_TRUSTED_PROXY_COUNT`（默认 **0**）
- 为 0 时，只使用 `RemoteAddr`，**完全忽略** `X-Forwarded-For`
- 为 N 时，从 XFF 列表**右起第 N+1 个**取值（右侧是最近的代理，可信）

默认 0 意味着"没有配置就不信任任何头"，这是安全的默认值。
部署在 Ingress 后面时必须显式设置，`docs/deployment.md` 中说明。

### 中间件位置：需要拆成两个

prompt 给出的单一顺序 `... Timeout → Rate Limit → Authentication ...` 是不完整的——
按 UserID 限流必须发生在认证**之后**（那时才知道 UserID）。

因此拆分为两个中间件：

```text
Recovery
  → RequestID
    → Tracing
      → Logging
        → Metrics
          → CORS              ← 必须在限流前：preflight 不应消耗配额
            → BodyLimit
              → Timeout
                → RateLimitByIP        ← 未认证流量的兜底
                  → Authentication
                    → RateLimitByUser  ← 已认证流量的配额
                      → Authorization
                        → Handler
```

- **CORS 在限流之前**：浏览器的 OPTIONS 预检不携带凭据、数量可能很大，
  让它消耗业务配额会导致正常用户被误伤
- **RateLimitByIP 在 Authentication 之前**：否则未认证的攻击流量
  会先进入 JWT 验证（有 CPU 成本）
- **RateLimitByUser 在 Authentication 之后**：需要 Principal
- 登录路由额外挂载 `RateLimitLogin`（同时检查 `login_ip` + `login_id`），
  在 handler 之前、解析请求体之后（需要 email）

### 存储故障时的行为：分场景

这是本 ADR 最重要的决策。**不存在统一正确答案，必须按场景区分。**

| 场景 | Redis 不可用时 | 理由 |
|------|--------------|------|
| `ip` / `user` 通用限流 | **fail-open**（放行） | 限流是保护措施，不是业务功能。因为 Redis 抖动就让整站 503，是用可用性事故换取一个防护措施，得不偿失 |
| **登录限流** | **fail-closed**（拒绝，503） | 登录是攻击面。此时放行等于关闭暴力破解防护，且 Argon2id 的内存 DoS 敞开。宁可短暂无法登录 |

两种情况都必须：

- 记 WARN 日志（含 scope，不含 identity）
- 递增 `ratelimit_backend_errors_total{scope}` 指标 → **配置告警**
- 不向客户端泄漏"限流后端故障"（登录场景返回通用 503）

fail-open 的风险已知：Redis 故障期间无限流保护。
缓解手段是**进程内的兜底限流器**——一个不依赖 Redis 的本地令牌桶，
配额设为 Redis 配额的 2 倍（宽松，仅防雪崩）。

> v1 是否实现本地兜底？**是**，因为它只有约 40 行且没有外部依赖。
> 它同时也是 Redis 未启用时的降级路径。

### 429 响应

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 37
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1756483200
Content-Type: application/json

{
  "error": { "code": "RATE_LIMITED", "message": "too many requests" },
  "request_id": "..."
}
```

`X-RateLimit-*` 头在**所有**响应上返回（不只是 429），让客户端可以主动退避。

例外：登录限流被触发时**不返回** `X-RateLimit-*` 头，
只返回 `Retry-After`。原因：暴露剩余尝试次数等于给攻击者一个精确的探测计数器。

对应 `apperr.KindRateLimited` → 429（ADR-0003）。

日志级别 WARN（可能是攻击信号），但**必须防日志洪水**——
触发限流的请求本身也会产生日志，攻击时日志量 = 攻击量。
缓解：限流拒绝的日志按 identity 采样（每个 identity 每 10 秒最多记 1 条），
指标则全量。

## Consequences

### 正面

- 登录接口具备暴力破解防护，Argon2id 的内存开销可控
- 多副本下配额准确
- `Retry-After` 精确，客户端可以正确退避
- 故障策略按场景区分，可用性与安全性各自最优
- 本地兜底限流器让 Redis 不是绝对单点

### 负面 / 代价

- Redis 成为 v1 硬依赖，单机部署也要跑一个
- 每个请求增加 1 次 Redis 往返（约 0.2-1 ms）——
  这是所有接口的公共成本
- 两个限流中间件 + 一个登录专用限流器 = 三处配置，运维需要理解
- fail-open 期间存在防护真空
- Lua 脚本需要随版本管理（用 `EVALSHA` + 回退 `EVAL`）
- 共享 IP（NAT、企业出口、移动网络）下多个用户共用配额，可能误伤

### 后续影响

- Phase 2 只预留中间件挂载点，**不实现**（ratelimit 属于 Phase 3）
- Phase 3 必须与 Argon2id 同时落地，测试覆盖：放行 / 拒绝 / 窗口恢复 /
  Redis 故障下的 fail-open 与 fail-closed
- Phase 5 需要 `ratelimit_rejected_total{scope}` 与 `ratelimit_backend_errors_total{scope}`
- Phase 6 的 Ingress 配置必须与 `HTTP_TRUSTED_PROXY_COUNT` 对应
- Phase 8 审计：确认 Redis key 中不含明文 PII

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| 只在 Ingress/网关限流 | 不占用应用资源 | 拿不到 UserID，无法按账号限流；也保护不了 Argon2id | 粒度不够 |
| 内存限流器 | 无依赖、零延迟 | 多副本失真、重启清零 | 生产中无效 |
| 固定窗口计数 | 实现最简单 | 窗口边界可 2 倍突发 | 精度不足以保护登录 |
| 滑动窗口日志 | 最精确 | 内存 O(N) | 收益不抵成本 |
| 账号锁定（N 次失败锁定） | 直观 | 可被用来恶意锁定他人账号 | 会变成 DoS 武器 |
| 全局 fail-closed | 安全 | Redis 抖动 = 全站不可用 | 可用性代价过高 |
| 全局 fail-open | 可用 | 故障期登录防护归零 | 安全代价过高 |

## When to Revisit

- 出现 API Key / 第三方集成 → 增加 `apikey` scope 与独立配额
- 共享 IP 误伤投诉增多 → 引入更细的标识（设备指纹）或白名单
- Redis 往返成为 P99 延迟瓶颈 → 评估本地令牌桶 + 周期性与 Redis 对账的混合方案
- 需要按套餐分级配额（SaaS 计费）→ 配额从配置改为按 tenant 查询
