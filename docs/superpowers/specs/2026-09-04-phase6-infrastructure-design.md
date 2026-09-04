# Phase 6 — Infrastructure 设计

> 对应 `prompts/06-infrastructure.md`。目标:开发可启动 / CI 可验证 /
> 镜像可构建 / 生产可部署 / Kubernetes 可运行。

## 出发点

Phase 5 结束时仓库里已经有的:`docker-compose.yml`(postgres / redis /
nats / 可观测性四件套,已按 profile 划分)、完整的 Makefile 质量门禁、
`deployments/observability/` 及其守卫测试。**没有的**:Dockerfile、
`.github/`、任何 Kubernetes manifest、构建期版本注入。

## 已锁定的决策

### D1 — 基础镜像:`gcr.io/distroless/static:nonroot`

项目没有 CGO 依赖(pgx 与 argon2 都是纯 Go),因此 `CGO_ENABLED=0` 可以
静态编译。这让 alpine 的 musl 兼容问题从一开始就不存在,也就没有理由为
它付出代价。

| 候选 | 否决理由 |
|---|---|
| alpine | 带 shell 与 apk,攻击面大;musl 的好处本项目用不上 |
| scratch | CA 证书、`/etc/passwd`、tzdata 全要手工塞回去,等于把 distroless 已经做好的事重做一遍 |
| debian-slim | 带完整包管理器,与「minimal packages」直接冲突 |

**代价必须写进文档**:distroless 没有 shell,`kubectl exec` 进不去。第一次
线上排查时人会以为镜像坏了。缓解是 `kubectl debug`(ephemeral container),
必须出现在 README 的部署章节里,否则这个代价会在最糟的时刻被发现。

### D2 — 多环境:Kustomize

`kubectl v1.33` 内置 Kustomize v5.6,零新依赖,且**是本机唯一能真实
`--dry-run=client` 验证的方案**(helm 未安装,选它会凭空多一条 SKIPPED)。

Helm 的价值在于把 chart 分发给第三方;这是自用脚手架,那层 Go template
间接性买不到东西,正是 §0.2「Simple > Clever」要挡的。单套 manifest +
环境变量则撑不住 replicas / resources / HPA 这类结构性差异。

### D3 — CI:写完整 workflow,不建远端仓库

仓库当前没有 git remote,workflow 不会被触发。按 §0.5 强制诚实:

- ✅ workflow 引用的每条命令,本地实跑通过(与 `make check` 同一批)
- ✅ `actionlint` 静态校验通过
- ⛔ **SKIPPED:workflow 未在 GitHub Runner 上执行(无远端仓库)**

镜像推送目标写 `ghcr.io/${{ github.repository }}`,不写死任何用户名。

### D4 — compose 中的 API:定义但默认不启动

`api` 与 `worker` 定义为 compose 服务并归入 `full` profile。

不进 compose 会让 §6.3 的 `full → 全部` 名不副实;默认启动则把日常循环从
「改代码 → `go run` 重启 1 秒、断点可用」变成「改代码 → 重建镜像 → 重启
容器」,慢一个数量级且丢掉调试器。归入 profile 两者兼得,代价是 compose
里的服务定义要与 Dockerfile 保持同步。

### D5 — 版本信息走 `/health`,不新增 `/version`

§6.2 允许「`/version` 或 health metadata」。新增 operation 会连带修改
`api/openapi.yaml`,而 `api/bruno/collection_test.go` 的**双向**一致性断言
会立刻要求补一个 `.bru` 请求。为一条元数据付三处改动不划算。

`/health` 当前已返回 `{"status":"ok","version":"dev"}`,只需扩展字段。

**与现有 `APP_VERSION` 的关系必须明确,否则两个来源会打架**:
`internal/config/load.go:52` 现在写死 `p.str("APP_VERSION", "dev")`。改为
把 `buildinfo.Version` 当作该项的**默认值**——环境变量仍可覆盖(compose 里
用得上),但不设时拿到的是构建期注入的真值而不是 `"dev"`。`commit` 与
`buildTime` 没有对应的环境变量,只来自 ldflags。

### D6 — 可变 tag 照做,但禁止进 manifest

§6.7 邀请反驳,这是反驳:`v1.2` / `v1` 会让**同一份 YAML 在不同时间部署出
不同的东西**,而这种漂移没有任何痕迹。

- CI 产出 `v1.2.3` / `v1.2` / `v1` / `sha-<commit>`
- **Kustomize overlay 只允许引用不可变引用**(完整版本号或 SHA tag)
- 可变 tag 仅供人手动 `docker run` 尝鲜

## 文件结构

```text
Dockerfile                              多阶段,--target api / worker
.dockerignore
internal/buildinfo/buildinfo.go         ldflags 注入点
.github/workflows/ci.yml                PR 门禁,三 job
.github/workflows/docker.yml            main / tag 构建推送
deployments/k8s/
  base/
    kustomization.yaml
    configmap.yaml
    secret.example.yaml
    api-deployment.yaml
    api-service.yaml
    worker-deployment.yaml
    migrate-job.yaml
    ingress.yaml
    hpa.yaml
    pdb.yaml
  overlays/{local,development,staging,production}/kustomization.yaml
  manifests_test.go                     守卫测试
```

修改:`docker-compose.yml`(加 api/worker + full profile)、`Makefile`
(docker-* 目标、build 加 ldflags)、`cmd/api/run.go` 与
`internal/httpx`(health 暴露 commit)、`README.md`(Deploy 章节)、
`STATE.md`。

## 关键设计点

### 部署陷阱:METRICS_ADDR 绑了 loopback

`METRICS_ADDR` 默认 `127.0.0.1:9464`。本地正确——指标不该对外。但在
K8s 里 Prometheus 从**另一个 Pod** 抓取,绑 loopback 会永远抓不到,且
表现为「指标莫名其妙没有」而不是任何错误。

ConfigMap 必须覆盖成 `:9464`,同时**不在 Service 里暴露 9464**,靠
NetworkPolicy 或 ServiceMonitor 直连 Pod IP 限制访问面。

### 关停时序

```text
SIGTERM → readiness 转 false → 等 READINESS_DRAIN_DELAY(5s)让 LB 摘流
        → 处理完 in-flight → 停 consumer → 关依赖 → 退出
        总预算 HTTP_SHUTDOWN_TIMEOUT = 30s
```

`terminationGracePeriodSeconds: 45`。**不取刚好大于 30 的 31**:SIGTERM 从
kubelet 发出到进程收到之间还有调度延迟,余量太薄会在负载高时变成 SIGKILL,
表现为关停期偶发 502——一种只在生产出现、本地永远复现不了的故障。

### 探针

- `livenessProbe → /live`:**不查任何依赖**
- `readinessProbe → /ready`:查 PostgreSQL / Redis

PostgreSQL 抖动必须只影响 readiness。若 liveness 也查数据库,一次几秒的
主库切换会让**所有副本同时**探针失败 → 同时被杀 → 同时重启 → 冷启动加剧
数据库压力 → 再次失败。这就是重启风暴:数据库恢复了,集群却还在自己打自己。

### 安全上下文

```yaml
runAsNonRoot: true
runAsUser: 65532          # distroless nonroot
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities: { drop: [ALL] }
seccompProfile: { type: RuntimeDefault }
```

`readOnlyRootFilesystem: true` 可行的前提是进程不写盘。本项目日志走
stdout、无本地缓存,成立。

### Migration Job 与 Deployment 分离

迁移是一次性的、需要串行、失败必须阻断发布;Deployment 是持续的、多副本、
滚动。把迁移塞进 initContainer 会让**每个副本都跑一次迁移**,并发执行同一
份 DDL。独立 Job + `ttlSecondsAfterFinished` 自动清理。

### HPA

基于 CPU(`averageUtilization: 70`)。什么时候该换成 custom metrics:

- **API**:CPU 与请求量脱钩时(例如大量时间花在等数据库),应改用 RPS 或
  P95 延迟
- **worker**:CPU 几乎永远是错的指标,正确的是 **NATS consumer lag**。
  积压时 CPU 可能很低(在等 I/O),而这恰恰是最需要扩容的时刻

这两条写进 manifest 注释,而不只是 README——改 HPA 的人不会去读 README。

## 守卫测试 `deployments/k8s/manifests_test.go`

沿用 `deployments/observability/dashboard_test.go` 与
`api/bruno/collection_test.go` 的同一条理由:**这些错误不会报错,只会
表现成"服务本身有问题"。**

| 断言 | 弄错了会怎样 |
|---|---|
| `terminationGracePeriodSeconds > ConfigMap 的 HTTP_SHUTDOWN_TIMEOUT` | 关停期偶发 502,只在生产出现 |
| liveness 指向 `/live`、readiness 指向 `/ready`,且两者存在于 `openapi.yaml` | 探针指向已删除路径 → 404 → 重启风暴 |
| securityContext 四项齐全 | 静默失去加固,无任何症状 |
| `METRICS_ADDR` 不是 loopback | Prometheus 永远抓不到,像是指标没接 |

测试读取 `kustomize build` 的输出还是 base YAML:**读 base YAML**。理由是
测试不应依赖外部二进制(CI 上 `kubectl` 未必存在),而这四条约束都在 base
层定义,overlay 不会覆盖它们。若将来 overlay 开始覆盖,应扩展测试而不是
改成调用 kustomize。

## 验证计划

| 命令 | 预期 |
|---|---|
| `make check` | 全绿 |
| `docker build --target api -t x .` | 成功;镜像大小如实记录,不预设阈值 |
| `docker compose config` | 成功 |
| `docker compose --profile full up -d` + `curl /live` | 200 |
| `kubectl apply --dry-run=client -k deployments/k8s/overlays/production` | 成功 |
| `actionlint .github/workflows/*.yml` | 无问题 |
| `go test ./deployments/k8s/ -count=1` | 绿,且经变异验证会红 |

**守卫测试必须做变异验证**:把 `terminationGracePeriodSeconds` 改成 20、
把 liveness 改成 `/healthz`、删掉一项 securityContext——三者都必须让测试
变红并点名具体文件。本项目反复出现「测试绿了但原因不对」,没见它红过的
测试不算数。

## 明确不做

- **Trivy**:§6.8 说「不要加入过多重型安全工具」。govulncheck 已覆盖 Go
  依赖;镜像扫描的增量价值在 distroless(几乎没有 OS 包)上很低
- **NetworkPolicy**:§6.9 未列出,且需要 CNI 支持才有意义
- **ServiceMonitor / PrometheusRule**:属于告警与 SLO,STATE.md 已把它们
  列为 Phase 6 之外
- **真实 Secret**:仓库只放 `secret.example.yaml`
- **在 K8s 里实际部署**:无可用集群,§6.18 的 `kubectl apply` 只做
  client dry-run,如实标注
