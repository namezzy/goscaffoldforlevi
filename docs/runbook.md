# 部署操作手册

> 本文是**照着做**的步骤;每一步都在一个真实的三节点 Kubernetes 集群上执行过。
> 设计上的**为什么**在 [`deployment.md`](deployment.md) 与 [`adr/`](adr/)。
>
> 验证环境:k3s v1.35.5 / containerd 2.2.3 / 三节点。凡是本文写「已验证」的,
> 都指在那套环境上实跑过并观察到结果。

## 目录

- [0. 前置条件](#0-前置条件)
- [1. 准备镜像](#1-准备镜像)
- [2. 创建 Namespace 与依赖](#2-创建-namespace-与依赖)
- [3. 创建 Secret](#3-创建-secret)
- [4. 执行迁移](#4-执行迁移)
- [5. 部署应用](#5-部署应用)
- [6. 创建第一个管理员](#6-创建第一个管理员)
- [7. 冒烟验证](#7-冒烟验证)
- [日常操作](#日常操作)
- [故障处置](#故障处置)
- [升级与回滚](#升级与回滚)
- [排障速查](#排障速查)

## 0. 前置条件

| 项 | 要求 |
|---|---|
| 集群 | Kubernetes 1.30+;已验证 1.35 |
| kubectl | 内置 Kustomize v5(kubectl 1.27+) |
| PostgreSQL | 17;生产用托管实例,**不要**用 `local-deps` |
| Redis | 7;限流依赖它,不是可选项 |
| NATS | 仅当 `MESSAGING_ENABLED=true`,需开 JetStream |
| 镜像仓库 | 集群能拉到 `ghcr.io/<owner>/<project>*` |

**镜像架构必须与节点匹配。** 在 arm64 机器上构建、推到 amd64 集群会得到
`exec format error`。CI 在 amd64 runner 上构建;本地构建请用
`docker build --platform linux/amd64`。

**本机集群怎么拿到镜像,各发行版不同**:

| 集群 | 做法 |
|---|---|
| OrbStack | 与 Docker 共享镜像库,`docker build` 完即可用,`imagePullPolicy: Never` 直接生效 |
| k3d | 需要额外一步 `k3d image import -c <cluster> <image>...` |
| kind | `kind load docker-image <image>...` |

## 1. 准备镜像

CI 会在推 `master` 与 `v*` tag 时自动发布四个镜像:

| 镜像 | 用途 |
|---|---|
| `<project>` | API |
| `<project>-worker` | outbox relay + 消费者 |
| `<project>-migrate` | 迁移(SQL 已烤进镜像) |
| `<project>-adminctl` | 创建首个管理员,一次性 |

手工构建全部四个:

```sh
make docker-build          # 版本信息取自 git
```

**overlay 只能引用不可变 tag**(`sha-<commit>` 或确切版本号)。把 overlay 里的
占位 SHA 换成本次要发布的那个:

```sh
cd deployments/k8s/overlays/production
kustomize edit set image \
  ghcr.io/<owner>/<project>=ghcr.io/<owner>/<project>:sha-<commit> \
  ghcr.io/<owner>/<project>-worker=ghcr.io/<owner>/<project>-worker:sha-<commit> \
  ghcr.io/<owner>/<project>-migrate=ghcr.io/<owner>/<project>-migrate:sha-<commit>
```

## 2. 创建 Namespace 与依赖

**生产**:Namespace 由平台创建;PostgreSQL / Redis / NATS 是托管服务,地址写进
ConfigMap 与 Secret。`base/` 里没有它们,这是有意的——应用的 manifest 不应该
创建数据库。

**本机集群(kind / k3d / minikube)**:

```sh
kubectl apply -k deployments/k8s/local-deps
kubectl -n goscaffold-local rollout status deploy/postgres deploy/redis deploy/nats
```

> [!WARNING]
> `local-deps` 的 PostgreSQL 用 `emptyDir`,**Pod 一重启数据就没了**。
> 这不是理论风险:验证过程中把它缩容到 0 做故障演练,再拉起来时 initdb 重建了
> 空库,四张表全部消失,worker 开始刷 `relation "outbox_events" does not exist`。
> 重跑一次迁移即可恢复,但不要在上面放任何你在意的数据。

## 3. 创建 Secret

**带外创建,绝不进 git。** 仓库里只有 `secret.example.yaml`,它列出键名、值全是
占位符,且**不在** `base` 的 `resources` 里——applied 出一个值为 `REPLACE_ME`
的 Secret,比缺 Secret 更难读:Pod 会起来,然后自己拒绝自己的配置。

```sh
kubectl -n <ns> create secret generic goscaffold-secrets \
  --from-literal=DATABASE_URL='postgres://user:pass@host:5432/db?sslmode=require' \
  --from-literal=AUTH_JWT_SECRET="$(openssl rand -base64 48)" \
  --from-literal=REDIS_PASSWORD='...'
```

三个键都是必需的。`APP_ENV=production` 时,启动会拒绝已知的示例密钥、通配符
CORS、以及绑在公开地址上的 metrics。

## 4. 执行迁移

**迁移是独立的一次性 Job,必须在应用滚动之前完成,失败必须阻断发布。**

```sh
kubectl apply -k deployments/k8s/overlays/<env>
kubectl -n <ns> wait --for=condition=complete job/goscaffold-migrate --timeout=5m
kubectl -n <ns> logs job/goscaffold-migrate
```

预期输出形如:

```text
1/u baseline (6.6ms)
2/u users (19.5ms)
3/u refresh_sessions (32.5ms)
4/u messaging (52.3ms)
```

Job 带 `ttlSecondsAfterFinished: 3600`,一小时后自动清理;在那之前它的日志是
「这次发布把 schema 推到了哪一版」的唯一记录。

重跑迁移(比如改了迁移文件)需要先删掉旧 Job,`Job` 的 spec 不可变:

```sh
kubectl -n <ns> delete job goscaffold-migrate
kubectl apply -k deployments/k8s/overlays/<env>
```

## 5. 部署应用

上一步的 `kubectl apply -k` 已经把 Deployment 一并创建了。盯住滚动:

```sh
kubectl -n <ns> rollout status deploy/goscaffold-api --timeout=5m
kubectl -n <ns> rollout status deploy/goscaffold-worker --timeout=5m
kubectl -n <ns> get pods -o wide
```

**滚动更新期间不会掉容量**(`maxUnavailable: 0`)。已验证:3 副本全量重发布,
集群内探针 117 次请求零失败。

## 6. 创建第一个管理员

**这是个死结,必须显式解开**:`POST /api/v1/users` 需要 `user:create` 权限,
而在第一个管理员存在之前没人拥有它。`adminctl` 镜像就是为这一步存在的。

用 `-password-stdin` 从你的终端传密码,**这样密码不会进入 Pod 日志、etcd 或
任何 Secret**:

```sh
printf '%s' "$ADMIN_PASSWORD" | kubectl -n <ns> run adminctl-bootstrap \
  --rm -i --restart=Never --quiet \
  --image=ghcr.io/<owner>/<project>-adminctl:sha-<commit> \
  --overrides='{"spec":{"containers":[{
      "name":"adminctl-bootstrap",
      "image":"ghcr.io/<owner>/<project>-adminctl:sha-<commit>",
      "stdin":true,"stdinOnce":true,
      "args":["-email","ops@example.com","-name","Ops","-password-stdin"],
      "envFrom":[{"configMapRef":{"name":"goscaffold-config"}},
                 {"secretRef":{"name":"goscaffold-secrets"}}]}]}}'
```

预期输出只有一行,**没有密码**:

```text
created administrator ops@example.com (01a08575-0cb7-789d-b7b7-0dfffe031355)
```

`stdinOnce: true` 不能省——没有它容器等不到 EOF,会一直挂着直到超时。

命令**只能成功一次**。管理员已存在时它会拒绝:

```text
error: an administrator already exists; create further accounts through POST /api/v1/users
```

不传 `-password-stdin` 时它会生成一个随机密码并打印出来 —— 那样密码会留在
Pod 日志里,**集群内不要这么用**。

## 7. 冒烟验证

从集群内部打 Service(而不是 port-forward,后者只固定转发到一个 Pod):

```sh
kubectl -n <ns> run smoke --rm -i --restart=Never --image=curlimages/curl -- sh -c '
  echo "live:  $(curl -s -o /dev/null -w %{http_code} http://goscaffold-api/live)"
  echo "ready: $(curl -s http://goscaffold-api/ready)"
'
```

`/ready` 应当逐个报告依赖:

```json
{"status":"ok","version":"...","commit":"<真实 SHA>","components":{"postgres":"ok","redis":"ok"}}
```

`commit` 不是 `unknown` —— 那说明构建期版本注入生效了,你能确切知道跑的是哪个
构建。

再验证认证链路:

```sh
curl -X POST http://<host>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops@example.com","password":"'"$ADMIN_PASSWORD"'"}'
```

登录响应必须带 `Cache-Control: no-store`,且**不带** `X-RateLimit-*` 头。

## 外部依赖的延迟

生产里 PostgreSQL 与 Redis 是外部服务,不在集群内。已实测的量级(千兆局域网
一跳,应用在集群内、数据库在另一台机器上):

| 端点 | 依赖在集群内 | 依赖在网络另一端 |
|---|---|---|
| `/live` | ~1ms | ~2ms |
| `/ready` | ~2ms | ~13ms(TLS 下 ~20ms) |
| `GET /users`(两次查询) | ~3ms | ~24ms,冷连接首次 64ms |
| 四条迁移合计 | ~110ms | ~420ms |

**迁移慢约 8 倍**,这是设置 `kubectl wait --timeout` 时要记住的:集群内几十
毫秒完成的迁移,跨网络是几百毫秒,而一套几十条迁移的成熟 schema 会把这个差距
放大到分钟级。本文第 4 步用的 `--timeout=5m` 是按后者留的余量。

`/ready` 的 13ms 对每 5 秒一次的 readinessProbe 毫无压力,但它说明探针确实在
做真实的网络往返 —— 依赖变慢时,`/ready` 会先于业务请求反映出来。

### sslmode

生产连接串用 `sslmode=require`,已实测可用:服务端确认为 TLSv1.3 /
TLS_AES_256_GCM_SHA384 / 256 位。切换只需改 Secret 里的连接串,应用侧无需
任何改动。

## 快速重新部署

依赖(PostgreSQL / Redis / NATS)一旦起好就不必重来,重新部署只需要:

```sh
# 1. 改了代码 → 重建并分发镜像
make docker-build
k3d image import -c <cluster> ghcr.io/<owner>/<project>{,-worker,-migrate,-adminctl}:local   # k3d
# OrbStack 跳过这一步

# 2. 重跑迁移(Job 的 spec 不可变,必须先删)
kubectl -n <ns> delete job goscaffold-migrate
kubectl apply -k deployments/k8s/overlays/local
kubectl -n <ns> wait --for=condition=complete job/goscaffold-migrate --timeout=5m

# 3. 让应用换上新镜像
kubectl -n <ns> rollout restart deploy/goscaffold-api deploy/goscaffold-worker
kubectl -n <ns> rollout status deploy/goscaffold-api
```

> [!NOTE]
> `kubectl apply -k` 会用 kustomize 生成的 Secret **覆盖**你手工改过的连接串。
> 想临时指向外部数据库,顺序必须是:先 apply、再 patch Secret、再删掉并重建
> 迁移 Job。反过来会静默回退到集群内的地址,而现象只是一句
> `dial tcp <ClusterIP>:5432: connect: connection refused`。

## 日常操作

```sh
# 看某次请求
kubectl -n <ns> logs deploy/goscaffold-api | grep <request_id>

# 消息积压
kubectl -n <ns> exec deploy/goscaffold-worker -- true   # 无 shell,改用日志
kubectl -n <ns> logs deploy/goscaffold-worker --tail=50 | grep -i outbox

# 扩容
kubectl -n <ns> scale deploy/goscaffold-api --replicas=8

# 改配置后让它生效(ConfigMap 不带 hash 后缀,不会自动滚动)
kubectl -n <ns> rollout restart deploy/goscaffold-api
```

**镜像里没有 shell**,`kubectl exec` 进不去。排查用临时容器:

```sh
kubectl -n <ns> debug -it <pod> --image=busybox --target=api
```

## 故障处置

### 数据库短暂不可用

**不需要做任何事,也不要重启 Pod。** 已验证的行为:

| 时刻 | 现象 |
|---|---|
| +5s | `/ready` 转 503,响应体点名 `postgres: unavailable` |
| +10s | Pod 标记 NotReady,**被摘出 Service 的 EndpointSlice** |
| 全程 | `restartCount` 保持 **0** —— liveness 不查依赖,不会触发重启 |
| 数据库恢复 +10s | 自动重新 Ready,重新进入 Endpoints |

这正是 `/live` 不碰数据库的价值:否则一次几秒的主库切换会让所有副本同时探针
失败、同时被杀、同时冷启动打向正在恢复的库,数据库再也爬不出来。**看到 Pod
NotReady 时,先看数据库,不要动 Pod。**

### 关停期偶发 502

检查 `terminationGracePeriodSeconds` 是否仍是 45,以及它与
`HTTP_SHUTDOWN_TIMEOUT` 的差值是否 ≥ 10 秒。守卫测试盯着这一条,但有人可能
在集群里手工改过。

已验证的正常时序:

```text
SIGTERM → "draining delay=5s" → /ready 转 503 → +5.001s 干净退出
```

Pod 应当在 6 秒左右消失。**接近 45 秒才消失**意味着它被 SIGKILL 了,那时
in-flight 请求会被丢掉。

### Prometheus 抓不到指标

检查 `METRICS_ADDR`。它必须是 `:9464`,**不能**是应用默认的
`127.0.0.1:9464` —— 在 Pod 里绑 loopback 等于谁也抓不到,而且它的失败形式是
「没有数据」而不是「报错」。9464 刻意不在 Service 里暴露,抓取方应直连 Pod IP。

### 探针失败但服务正常

**如果服务是从旧版本升上来的**,检查探针是否被限流。旧版本里 `/live` 与
`/ready` 在 IP 限流器后面,而 kubelet 从**节点 IP** 探测,同一节点上的 Pod
共享一个配额桶——Pod 密集时会自己把自己限流成 NotReady。

当前版本已把三个探针路径从限流中豁免,并有测试守着。已验证:600 次 `/ready`
全部 200,同期 300 次 API 请求有 98 次 429。

## 升级与回滚

```sh
# 升级:改 tag → 迁移 → 滚动
kustomize edit set image ...=...:sha-<new>
kubectl apply -k deployments/k8s/overlays/<env>
kubectl -n <ns> wait --for=condition=complete job/goscaffold-migrate --timeout=5m
kubectl -n <ns> rollout status deploy/goscaffold-api

# 回滚代码
kubectl -n <ns> rollout undo deploy/goscaffold-api      # 或 --to-revision=N
```

> [!IMPORTANT]
> **回滚代码不等于回滚数据库。** 每条迁移都有 `.down.sql`,但执行它会丢掉新
> 版本通过它写入的数据。生产优先**向前修复**;迁移要设计成上一个版本仍能在新
> schema 上运行(先加列、下个版本再写、再下个版本才删)。

worker 停机不丢消息 —— 事件在 outbox 与 JetStream 里是持久的,停机只是延迟
投递。回滚 worker 前先看积压量。

## 排障速查

| 现象 | 最可能的原因 |
|---|---|
| 迁移 Job `ImagePullBackOff` | migrate 镜像没发布,或 tag 写错。守卫测试 `TestEveryReferencedImageIsPublished` 会在 CI 拦下前者 |
| Pod `CreateContainerConfigError` | Secret 不存在。它是带外创建的,见第 3 步 |
| Pod 起来后立刻退出,日志说配置被拒 | 用了示例密钥 / 通配符 CORS / 公开 metrics 地址,而 `APP_ENV=production` |
| 所有副本同时 NotReady | 先查数据库,不要动 Pod。见「数据库短暂不可用」 |
| worker 一直空转,日志无事件 | ConfigMap 的 `MESSAGING_ENABLED` 是否为 `true`。worker 会在自己的 `env` 里覆盖它,所以「false」只关掉了 API 侧:API 丢弃事件,worker 等一个永远不来的事件,两边都健康。守卫测试 `TestOverlaysDeployingTheWorkerEnableMessaging` 拦这个 |
| worker 刷 `relation ... does not exist` | 迁移没跑,或数据库被重建过(`local-deps` 的 emptyDir) |
| 邮件没发出去 | `notify.LogSender` 只把邮件写进日志,背后没有接任何服务商。日志里会写 `email not sent: no provider is configured` |
| `exec format error` | 镜像架构与节点不匹配。用 `--platform linux/amd64` 重建 |
| `kubectl exec` 报找不到 shell | distroless 镜像没有 shell,用 `kubectl debug` |
