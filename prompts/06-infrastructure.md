# Phase 6 — Infrastructure / Docker / CI / Kubernetes

先读取 `STATE.md`。

目标：**开发可启动 / CI 可验证 / 镜像可构建 / 生产可部署 / Kubernetes 可运行。**

## 6.1 Dockerfile

Multi-stage build。要求：small image / non-root / minimal packages / CA certificates。

分析 `distroless / scratch / alpine / debian-slim` 并选择一种，说明理由。
避免无意义的 alpine glibc 兼容问题。

## 6.2 Build Metadata

构建时注入 `version / commit /（可选）build time`，
通过 `/version` 或 health metadata 暴露**非敏感**版本信息。

## 6.3 Docker Compose

开发环境包含 `postgres / redis /（broker）`。

分析 API 本身是否应该默认在 compose 中启动（本地跑通常更好），给出理由。

profile 划分：

```text
default        → postgres, redis
messaging      → broker
observability  → collector, prometheus, grafana, tempo/jaeger
full           → 全部
```

## 6.4 Makefile 完善

```bash
make run build test test-race lint fmt vet vuln
make generate sqlc
make migrate-up migrate-down migrate-create
make docker-up docker-down docker-build
```

## 6.5 GitHub Actions

Pull Request 流程：

```text
checkout → setup go → cache → go mod download
→ fmt check → vet → lint
→ unit test → race test → integration test
→ govulncheck → build
→ openapi lint
```

根据时间成本合理拆分 Job（快速反馈 job 与慢速 job 分离）。

## 6.6 Integration Test in CI

分析 GitHub Actions Service Container vs testcontainers-go。

**如果 Repository Test 已使用 Testcontainers，优先保持一致**，避免两套机制。

## 6.7 Docker Build CI

main 分支构建镜像，tag 为 commit SHA。
Release tag `v1.2.3` 同时产出 `v1.2.3 / v1.2 / v1`。

如果你认为 mutable tags 不合适，说明理由并调整。

## 6.8 Security in CI

加入 `govulncheck`，可选 Trivy（镜像 + 依赖）。

**不要加入过多重型安全工具。**

## 6.9 Kubernetes Manifests

```text
Deployment / Service / ConfigMap / Secret example
Ingress / HPA / PodDisruptionBudget
Migration Job（一次性，与 Deployment 分离）
Consumer Deployment（如果 Phase 4 引入了消费者，独立于 API 部署）
```

## 6.10 Deployment 配置

正确配置 `replicas / rolling update / resources.requests / resources.limits / terminationGracePeriodSeconds`。

`terminationGracePeriodSeconds` 必须大于应用的 shutdown timeout。说明这个关系。

## 6.11 Probes

```text
livenessProbe  → /live
readinessProbe → /ready
```

**PostgreSQL 临时抖动不得触发 liveness failure**，否则会产生重启风暴。说明原因。

## 6.12 Security Context

```text
runAsNonRoot: true
readOnlyRootFilesystem: true（where possible）
allowPrivilegeEscalation: false
capabilities: drop ALL
```

## 6.13 Config & Secret

普通配置用 ConfigMap，敏感配置用 Secret。

Git 中**只能**提供 `secret.example.yaml`。**禁止真实 Secret 入库。**

## 6.14 HPA

基于 CPU 提供基础 HPA。说明未来何时应该升级为 custom metrics（RPS / consumer lag）。

## 6.15 Graceful Shutdown 端到端验证

确认链路匹配：

```text
SIGTERM → readiness false → 停止接收新请求
→ 处理完 in-flight 请求 → 停止 consumer → 关闭依赖 → 退出
```

## 6.16 多环境

建议 `local / development / staging / production`。

**不要复制四套完全独立 YAML。** 在 Kustomize / Helm / 基础 manifests 中选最简单的一种。

## 6.17 README Deploy 章节

说明 `Docker build / Docker Compose / Kubernetes deployment / Environment variables / Migration / Rollback`。

## 6.18 Phase 6 输出

执行 §0.4 全部门禁，另加（可用时）：

```bash
docker build .
docker compose config
kubectl apply --dry-run=client -f deployments/
```

不可用则 SKIPPED。输出：

```text
Infrastructure Files
Docker Architecture
CI Pipeline
Kubernetes Strategy
Security Decisions
Verification
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。
