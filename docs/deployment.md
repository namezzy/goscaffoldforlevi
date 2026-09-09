# 部署指南

> 本文回答"怎么把它放上去,以及出事时怎么办"。
> 设计上的**为什么**在 [`architecture.md`](architecture.md) 与
> [`adr/`](adr/);日常开发在 [`development.md`](development.md)。

> [!IMPORTANT]
> 这套 manifest **校验过,没有跑过**。它们能渲染、能通过 Kubernetes 1.33 的
> 严格 schema 校验,但从未在真实集群上 apply。schema 校验抓得到字段和类型
> 错误,抓不到调度、准入控制器、CNI 层面的问题。第一次上线请按预发环境对待。

## 目录

- [镜像](#镜像)
- [环境变量](#环境变量)
- [Secret](#secret)
- [迁移](#迁移)
- [Kubernetes](#kubernetes)
- [健康检查与关停时序](#健康检查与关停时序)
- [可观测性接线](#可观测性接线)
- [回滚](#回滚)
- [四个容易搞错的地方](#四个容易搞错的地方)

## 镜像

一份 Dockerfile,三个 target,共享到最后一次 COPY 为止的全部层:

| Target | 基础镜像 | 大小 | 内容 |
|---|---|---|---|
| `api` | `gcr.io/distroless/static:nonroot` | 40.8 MB | HTTP 服务 |
| `worker` | 同上 | 35.1 MB | outbox relay 与消费者 |
| `migrate` | `migrate/migrate:v4.19.1` | — | golang-migrate CLI + 本仓库的 SQL |

```sh
make docker-build                       # api + worker,版本信息取自 git
docker build --target migrate -t m .    # 迁移镜像
```

选 distroless 的理由是本项目无 CGO 依赖(pgx 与 Argon2 都是纯 Go),
`CGO_ENABLED=0` 能静态编译,alpine 的 musl 问题从一开始就不存在。

**代价:最终镜像没有 shell,`kubectl exec` 进不去。** 第一个上线排查的人会
以为镜像坏了。用临时容器:

```sh
kubectl debug -it <pod> --image=busybox --target=api
```

版本、commit、构建时间在链接期注入 `internal/buildinfo`,出现在启动日志、
每个 `/health` 与 `/ready` 响应、以及 `APP_VERSION` 的默认值。不带 flag 构建
出的二进制报告 `dev` 与 `unknown`,而不是空字符串 —— 空版本号看起来像报告
坏了,而不像一个没打戳的构建。

## 环境变量

优先级:**真实环境变量 → `.env` → 内置默认值**。每个键、默认值和默认值背后的
理由都以注释写在 [`.env.example`](../.env.example),并有测试保证它与加载器不
漂移。

集群里的分工:非敏感值进 `configmap.yaml`,`DATABASE_URL`、`AUTH_JWT_SECRET`、
`REDIS_PASSWORD` 进带外创建的 Secret。应用把两者都当普通环境变量读,不关心
它们从哪来 —— 所以换成密钥管理 operator 可以直接替掉那一步,不需要改代码。

配置在启动时校验一次并**一次性报出全部问题**。`APP_ENV=production` 时另有
三道守卫会直接拒绝启动:仍在用示例 JWT 密钥、CORS 配了通配符、metrics 绑在
公开地址上。

base 的 ConfigMap 刻意写死 `APP_ENV: production`,让漏配 overlay 失败在安全
的一侧。

## Secret

**仓库里只有 `secret.example.yaml`,它列出键名,值全是 `REPLACE_ME`,而且
它不在 base 的 `resources` 里** —— applied 出一个值为 `REPLACE_ME` 的 Secret,
比缺 Secret 更难读:Pod 会起来,然后自己拒绝自己的配置。

```sh
kubectl create secret generic goscaffold-secrets \
  --from-literal=DATABASE_URL='postgres://user:pass@host:5432/db?sslmode=require' \
  --from-literal=AUTH_JWT_SECRET="$(openssl rand -base64 48)" \
  --from-literal=REDIS_PASSWORD='...'
```

轮换 `AUTH_JWT_SECRET` 会让**所有已签发的 access token 失效**(最长 15 分钟
内全体用户被登出);refresh token 不受影响,它们是不透明的、存在服务端。

## 迁移

**迁移是独立的一次性 Job,不是 initContainer。** initContainer 每个 Pod 跑
一次,三副本发布会把同一份 DDL 并发执行三遍,而且此后每次重启都再跑一遍,
永远如此。

发布顺序,刻意没有自动化:

```sh
# 1. Secret,带外创建(见上)

# 2. 迁移,并等它完成。这里失败必须阻断发布。
kubectl apply -k deployments/k8s/overlays/production
kubectl wait --for=condition=complete job/goscaffold-migrate --timeout=5m

# 3. 盯住滚动
kubectl rollout status deploy/goscaffold-api
```

迁移镜像与应用镜像带同一个 commit tag,所以"这个版本期望哪份 schema"有确切
答案,而不是推断出来的。

本地:`make migrate-up` / `make migrate-down` / `make migrate-status`。
**Pod 启动时不会自动迁移**(ADR-0005)。

## Kubernetes

Kustomize,一个 base 加四个 overlay:

```sh
kubectl kustomize deployments/k8s/overlays/production   # 渲染
kubectl apply -k deployments/k8s/overlays/production    # 应用
```

| Overlay | 副本 | 特点 |
|---|---|---|
| `local` | 1 | kind/minikube;无 Ingress、无 HPA;自带开发凭据 |
| `development` | 1 | 无 HPA;PDB 放宽到 0 |
| `staging` | 2 | 与生产同构,尺寸更小;trace 全采样 |
| `production` | 5 | HPA 5–30;PDB minAvailable 4 |

**overlay 只允许引用不可变镜像 tag**(`sha-<commit>` 或确切版本号)。`v1` 这类
可变 tag 会让同一份 YAML 在不同时间部署出不同的东西,而这种漂移在 git 里不留
痕迹。`TestNoManifestReferencesAMovingTag` 强制这一条。

`local` overlay 是唯一带凭据的,它们不是密钥 —— 与 `docker-compose.yml` 同
性质的本机口令,`APP_ENV=production` 下会被启动守卫拒绝。**要抄就抄
`staging`,不要抄 `local`。**

## 健康检查与关停时序

| 探针 | 路径 | 查什么 |
|---|---|---|
| liveness | `/live` | **不查任何依赖** |
| readiness | `/ready` | PostgreSQL、Redis,逐个报告 |
| startup | `/live` | 慢启动不被当成进程死亡 |

**`/live` 绝不能碰数据库。** 碰了的话,一次几秒的主库切换会让所有副本同时
探针失败、同时被杀、同时冷启动打向正在恢复的库 —— 而后再次失败。这就是重启
风暴:数据库恢复了,集群还在自己打自己。依赖健康属于 readiness,那里的后果是
"别往这儿发流量",而不是"销毁这个 Pod"。

关停时序,已实测:

```text
SIGTERM → /ready 转 503 draining → 等 5s 让 LB 摘流
        → 处理完 in-flight → 停 consumer → 关依赖 → 退出码 0
```

`terminationGracePeriodSeconds: 45`,而应用预算是 30 秒。**不取刚好大于 30 的
31**:SIGTERM 从 kubelet 发出到进程收到之间有调度延迟,余量太薄会在负载高时
变成 SIGKILL,表现为发布期间偶发 502 —— 一种只在生产出现、本地永远复现不了
的故障。守卫测试要求余量至少 10 秒。

## 可观测性接线

埋点始终编译在内,`OTEL_ENABLED` 与 `METRICS_ENABLED` 只控制数据是否离开
进程。tracing 关闭时业务行为不变,这一条是实跑验证过的。

**`METRICS_ADDR` 在集群里必须是 `:9464`,不能是应用默认的 `127.0.0.1:9464`。**
Prometheus 从另一个 Pod 抓取,绑 loopback 等于谁也抓不到 —— 而且它的失败形式
是"没有数据"而不是"报错",这是最难被发现的一类故障。ConfigMap 里已经覆盖,
`TestMetricsAddressIsNotLoopback` 守着。

**9464 不在 Service 里暴露。** 指标名会枚举每个路由模板、依赖和构建版本;
抓取方应直连 Pod IP(ServiceMonitor 默认就是这么做的),限制访问面是
NetworkPolicy 的事,不是靠一个把抓取方也一起挡在外面的 loopback 绑定。

`OTEL_EXPORTER_OTLP_INSECURE` 在 base 里是 `false`。开发用的 compose 把它设成
了 `true`,**不要把那份配置抄进生产 manifest**。

## 回滚

```sh
kubectl rollout undo deploy/goscaffold-api            # 或 --to-revision=N
```

**回滚代码不等于回滚数据库。** 每条迁移都有 `.down.sql`,但执行它会丢掉新版本
通过它写入的数据。所以:

- 生产优先**向前修复**,而不是执行 down 迁移
- 迁移要设计成上一个版本仍能在新 schema 上运行(先加列、下个版本再写、
  再下个版本才删)
- down 迁移主要服务于本地开发与 CI

消息侧:worker 停掉不会丢消息 —— 事件在 outbox 与 JetStream 里是持久的,
停机只是延迟投递。回滚 worker 前先看 `make outbox-status` 的积压量。

## 四个容易搞错的地方

1. **`METRICS_ADDR` 绑 loopback** —— 指标永远抓不到,且不报错。见上。
2. **`terminationGracePeriodSeconds` 取 31** —— 看起来对,负载高时变 SIGKILL。
3. **把 compose 的 `OTEL_EXPORTER_OTLP_INSECURE=true` 抄进生产** —— 集群内
   mTLS 白配。
4. **以为服务能发邮件** —— `notify.LogSender` 只把邮件写进日志。整条链路
   (outbox、投递、幂等、重试、DLQ)都是真的且有测试,唯独背后没有接任何邮件
   服务商。
