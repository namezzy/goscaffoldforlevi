# 安全

> 本文说明这套脚手架在安全上做了什么、**没有**做什么,以及每条选择的理由。
> 认证与限流的完整论证在 [ADR-0007](adr/0007-auth.md) 与
> [ADR-0008](adr/0008-rate-limiting.md)。

## 目录

- [威胁模型的边界](#威胁模型的边界)
- [密码](#密码)
- [Access token（JWT）](#access-tokenjwt)
- [Refresh token](#refresh-token)
- [授权](#授权)
- [限流](#限流)
- [密钥管理](#密钥管理)
- [日志与遥测](#日志与遥测)
- [HTTP 层](#http-层)
- [依赖与容器](#依赖与容器)
- [消息](#消息)
- [已知的暴露面](#已知的暴露面)

## 威胁模型的边界

这套脚手架防的是:**账号枚举、凭据爆破、token 盗用与重放、越权访问、
密钥泄漏进日志与遥测、容器逃逸面**。

它**不**防:应用层 DDoS(限流只到进程与 Redis 这一层)、内部人员滥用管理员
权限(没有二次授权与审批流)、数据库被直接拖库(没有列级加密)、
供应链攻击(依赖有 `govulncheck`,没有 SBOM 与签名)。

## 密码

Argon2id,64 MiB / t=3 / p=4(OWASP 推荐值)。硬件允许时**优先加内存** ——
那是 GPU 攻击者无法用钱绕开的参数。

代价必须知道:**每次登录都要分配 64 MiB**。这正是登录限流不可省的原因,
而不是可选的加固。

存储格式是标准 PHC 字符串,参数写在哈希里,所以调高成本参数不会让旧密码失效:
`VerifyResult.NeedsRehash` 会在登录成功时报告需要升级 —— 登录是唯一能拿到
明文的时刻,也就是唯一能升级的时刻。

**校验失败时会走 `VerifyDummy()`**,让"邮箱不存在"和"密码错误"消耗相同的时间。

## Access token（JWT）

HS256,15 分钟。短是设计的一部分:泄漏的 access token 只在这段时间里有用。

**代价:撤销和改角色最长有 15 分钟延迟**(ADR-0007)。这是拿"不查库的无状态
校验"换来的,不是疏漏。需要立即撤销的场景,应当在中间件里加一次黑名单查询,
并接受它带来的每请求一次 Redis 往返。

`jti` 是强制的:没有唯一标识就无法撤销单个 token,也无法把一次审计事件归到
某一个 token 上。

校验失败**一律返回同一个错误**,调用方无法区分"过期"和"伪造"。

## Refresh token

不透明的 256 位随机串,不是 JWT。因为它是均匀随机而非用户选择的,所以**不需要
密钥拉伸**,猜中它在计算上不可行。

库里**只存 SHA-256 摘要** —— 数据库泄漏不会直接产出可用的 token。

**每次使用即轮换,并带家族盗用检测**:出示一个已经被轮换过的 token,意味着它
要么被盗用、要么被重放,服务会撤销**整个家族**,且**撤销先提交、再返回拒绝**
—— 顺序反过来的话,攻击者可以靠让请求失败来阻止撤销落库。

代价:无法区分"被盗用"与"客户端并发重试",所以合法客户端也会被一起登出。
从安全侧取严。

## 授权

两个角色、五个权限、一张策略表(`internal/auth/policy.go`)。
**Handler 申请的是权限,不是角色** —— 角色判断散落在 handler 里,是权限模型
腐烂的起点。

`rolePermissions` 是包内私有,只在初始化时写入;`Role.Permissions()` 返回
副本,调用方无法通过它改变策略。

**没登记的权限永远不授予。** 这让新增端点默认是不可达的(403),而不是默认
可达 —— `goscaffold module` 生成的端点正是靠这一条做到 fail-closed。

需要"本人或有权限"的操作(读/改自己的记录)在 **service 层**判断,因为那需要
目标对象的身份,而中间件拿不到。

## 限流

Redis Lua 令牌桶,配额跨副本共享。四个 scope:

| Scope | 配额 | 失败模式 |
|---|---|---|
| 按 IP | 100 / 1min | **fail-open**,降级到本地兜底限流器 |
| 按用户 | 600 / 1min | **fail-open** |
| 登录按 IP | 10 / 15min | **fail-closed**,503 |
| 登录按邮箱哈希 | 5 / 15min | **fail-closed**,503 |

通用 scope fail-open 的理由:Redis 故障时,把整个服务变成 503 的伤害大于短暂
失去限流。**登录 fail-closed 的理由相反**:限流是那里唯一挡住密码爆破的东西,
失去它比拒绝服务更糟。

登录同时按 IP 与邮箱哈希两个维度限,**两个都要过**:前者挡住单一来源枚举大量
账号,后者挡住大量来源打同一个账号。

**登录响应会剥掉 `X-RateLimit-*` 头。** 在一次被拒的登录旁边显示剩余次数,
既是给攻击者的进度条,也自相矛盾。登录只带 `Retry-After`。

已知空窗:Redis 故障期间,通用 scope 降级到进程内限流器,配额按副本数放大
(`fallbackFactor=2`),这是"别把配额收得比预期紧"的取舍。

## 密钥管理

- 仓库里**没有任何真实凭据**。`.env` 与 `api/bruno/.env` 被 gitignore;
  `.env.example` 与 `secret.example.yaml` 只有占位符
- `config.Secret` 类型的 `String()` 与 `LogValue()` 都返回掩码,所以把整个
  配置结构体打进日志也不会泄漏
- `APP_ENV=production` 时,启动会拒绝已知的示例密钥
- 生产 Secret 带外创建,见 [`deployment.md`](deployment.md#secret)

> [!WARNING]
> Phase 7 期间发生过一次真实泄漏:模板生成按目录纳入 `api/bruno`,而复制时
> 加的 `.tmpl` 后缀让 `.gitignore` 的 `.env` 规则失效,于是一个被忽略的凭据
> 文件进了提交。根因已修复(模板只收 `git ls-files` 跟踪的文件),事故记录在
> `STATE.md`。**教训是通用的:任何给文件改名或加后缀的复制过程,都会丢掉
> "被忽略"这个属性。**

## 日志与遥测

- 日志是结构化的 `slog`,`request_id` 与 `trace_id` 从 context 自动注入
- **DB 与缓存 span 走属性白名单**,任何查询参数或缓存值都不可能进入 span
- 内部错误对客户端只返回 `request_id`,不返回任何细节 —— 错误文本可能包含
  DSN
- `internal/httpx/leak_test.go` 是一次端到端扫描:一个请求携带本应用处理的
  每一类密钥,过完整条中间件栈,然后在产生的日志与 span 里逐个搜索它们

最后这条的写法值得说明:它**刻意不是每个中间件一个测试**。脱敏不是任何单个
组件的属性 —— 一个值只有在没有任何一层写出它时才是安全的,而当新加一层开始
记日志时,每个"逐层测试"仍然全绿。

## HTTP 层

| 措施 | 值 |
|---|---|
| 请求体上限 | 1 MiB(`HTTP_MAX_BODY_BYTES`) |
| 请求超时 | 15s;写超时必须大于它,否则客户端拿到的是连接重置而不是 504 |
| Panic | Recovery 在最外层,能捕获其他所有中间件的 panic |
| CORS | 逗号分隔白名单;`APP_ENV=production` 时拒绝 `*` |
| 安全头 | `X-Content-Type-Options: nosniff` 等 |
| JSON 解码 | `DisallowUnknownFields`,拒绝多余字段 |
| Token 响应 | `Cache-Control: no-store` + `Pragma: no-cache` |

**`HTTP_TRUSTED_PROXY_COUNT` 默认 0,即完全忽略 `X-Forwarded-For`。**
那是客户端可控的头。会话记录客户端 IP,记录一个可伪造的值会让审计轨迹**比
没有更糟** —— 它会自信地指向错误的地方。配真实跳数,绝不能配大。

## 依赖与容器

18 个直接依赖,每一个在最终报告的依赖审计里都有存在理由。`make vuln`
(`govulncheck`)在 `make check` 与 CI 中运行。

容器加固,三个工作负载(api / worker / migrate)一致:

```yaml
runAsNonRoot: true
runAsUser: 65532
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities: { drop: [ALL] }
seccompProfile: { type: RuntimeDefault }
```

`readOnlyRootFilesystem: true` 成立的前提是进程不写盘:日志走 stdout,无本地
缓存与临时文件。**这个前提一旦变化,应该挂 emptyDir 而不是放宽设置。**

distroless 基础镜像没有 shell 和包管理器,攻击面小;代价是排查要用
`kubectl debug`。

**没有做镜像扫描**(Trivy 之类):`govulncheck` 已覆盖 Go 依赖,而在几乎没有
OS 包的 distroless 镜像上,再加一个扫描器的增量价值很低。

## 消息

- 事件与业务变更**在同一个事务里**写入 outbox,两者不可能不一致
- 投递是 at-least-once,消费端靠 `processed_events` 去重日志做幂等
- 去重日志的保留期**必须长于**消息流本身 —— 剪得比流早,一条被重投的消息会
  被处理两次
- `MESSAGING_ACK_WAIT` 必须大于整个重试预算,否则 broker 会在消费者还在处理时
  重投;启动时会拒绝不满足这一条的配置
- DLQ 是数据库表,可查询、可重放,不是一个没人看的死信队列
- **NATS 凭据走 Secret**,不进 ConfigMap

外部副作用(发邮件)无法 exactly-once,只能依赖外部系统的幂等 —— 这是 ADR-0009
明确接受的限制。

## 已知的暴露面

逐条列出,每条都在对应 ADR 里论证过:

| 暴露 | 影响 | 出处 |
|---|---|---|
| access token 撤销/改角色最长 15 分钟延迟 | 被撤销的用户在窗口内仍可访问 | ADR-0007 |
| 通用限流 fail-open | Redis 故障期间防护真空(有本地兜底缓解) | ADR-0008 |
| `POST /users` 的 409 泄漏"该邮箱已注册" | 需要 `user:create` 权限,调用方已是可信管理员 | ADR-0007 |
| UUIDv7 暴露资源创建时间(毫秒级) | 可推断创建顺序与时间 | ADR-0006 |
| refresh 家族撤销会把合法客户端一并登出 | 可用性代价,换取盗用检测 | ADR-0007 |
| 没有自助注册,建用户需要权限 | 范围决定,不是遗漏 | —— |
| 请求体尚无 OpenAPI schema 自动校验 | 权威定义在 `validate` 规则里 | —— |
