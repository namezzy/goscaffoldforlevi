# Phase 0 — 全局执行契约

以下规则在**所有 Phase 中始终生效**，优先级高于任何单个 Phase 的局部描述。
如果某个 Phase 的要求与本契约冲突，以本契约为准，并在输出中指出冲突。

## 0.1 目标

构建一套真正可用于企业项目的：

**Production-ready Enterprise Go Scaffold**

技术基线：

```text
Go 1.27+
net/http + chi
PostgreSQL / pgx / sqlc
Redis / go-redis
Message Queue (Kafka or NATS JetStream，Phase 1 决策)
log/slog
OpenTelemetry
Prometheus
JWT
OpenAPI 3
golang-migrate
Docker
GitHub Actions
Kubernetes
```

## 0.2 项目原则

```text
Simple > Clever
Explicit > Magic
Composition > Inheritance
Small Interfaces
Dependency Inversion
Fail Fast
Context Everywhere
Observability by Default
Security by Default
Testability by Design
```

不要把 Go 项目写成 Java Spring Boot。

禁止：

```text
controller/service/dao/entity/vo/dto/common/utils 式机械分层
BaseController / BaseService / BaseRepository
GenericCRUD / GenericRepository[T]
AbstractXXX
巨大 interface
全局变量（global db / logger / config）
init() 隐式初始化
Service Locator
复杂反射
过度代码生成
```

## 0.3 状态持久化（强制）

因为每个 Phase 在独立会话中执行，**上下文不会自动继承**。因此：

1. **Phase 1 必须把所有架构决策写入磁盘**，格式为 ADR：

   ```text
   docs/adr/0001-tech-stack.md
   docs/adr/0002-project-layout.md
   docs/adr/0003-error-model.md
   ...
   ```

   每个 ADR 包含：`Status / Context / Decision / Consequences / Alternatives Considered`。

2. **每个 Phase 结束时必须更新 `STATE.md`**（仓库根目录），内容固定为：

   ```markdown
   # Project State

   ## Current Phase
   Phase N completed at <date>

   ## Locked Decisions
   （指向 docs/adr/ 的清单，一行一条，含结论摘要）

   ## Implemented
   （已完成能力清单）

   ## Not Implemented Yet
   （明确未做的事，防止下一阶段误判）

   ## Known Limitations
   （已知妥协及原因）

   ## Next Phase Entry Conditions
   （下一阶段开始前必须成立的前提）
   ```

3. **每个 Phase 开始时**：先读取 `STATE.md` 和 `docs/adr/`，再读代码库。
   **不得重新设计已锁定的决策。**

4. 如需推翻已有决策：新建一个 ADR，`Status: Accepted`，并在文中写明
   `Supersedes: docs/adr/000X-*.md`，同时说明推翻理由。禁止静默修改旧 ADR。

## 0.4 质量门禁

**Phase 2 起**，每个 Phase 完成前必须执行：

```bash
go fmt ./...
go vet ./...
go build ./...
go test ./...
go test -race ./...
golangci-lint run
govulncheck ./...
```

Phase 1 是纯设计阶段，**不执行**上述命令（没有代码）。

发现错误直接修复。**禁止把无法编译或测试不通过的代码交付给我。**

## 0.5 环境兜底（强制诚实）

Docker / Testcontainers / kubectl / 外部网络在某些环境中不可用。

规则：

```text
1. 命令实际执行过 → 贴出真实输出摘要
2. 命令因环境不可用而无法执行 → 明确写 "SKIPPED: <原因>"，并列入 Known Limitations
3. 严禁编造、推测或复述未真实执行的命令结果
```

如果你没有运行某条命令，就不能说它通过了。

## 0.6 范围纪律

```text
不得输出超出当前 Phase 范围的代码
不得提前实现后续 Phase 的功能
不得创建无用空目录
不得为了"演示某个模式"而引入没有真实需求的复杂度
```

每个 Phase 结束时，在输出中回答一句：
**"本阶段有没有做超范围的事？"**

## 0.7 Git 约定

每个 Phase 结束时提交一次，commit message 格式：

```text
phase(N): <一句话总结>
```

`.gitignore` 必须从 Phase 2 起就正确排除：
`.env`、二进制产物、coverage 文件、IDE 缓存、本地数据卷。
