# 使用说明

这是 **Enterprise Go Scaffold** 的分阶段开发 prompt 集。

## 文件

```text
prompts/
├── README.md            ← 你正在读的文件
├── PHASES.md            ← 完整合订本（备查，日常不用）
├── 00-contract.md       ← 全局执行契约（每个会话都要粘贴）
├── 01-architecture.md   ← Phase 1 架构设计（纯文档）
├── 02-foundation.md     ← Phase 2 基础工程
├── 03-user-auth.md      ← Phase 3 User + Auth + RBAC + RateLimit + OpenAPI
├── 04-messaging.md      ← Phase 4 消息队列 / 异步
├── 05-observability.md  ← Phase 5 可观测性
├── 06-infrastructure.md ← Phase 6 Docker / CI / Kubernetes
├── 07-scaffold-cli.md   ← Phase 7 goscaffold CLI
├── 08-docs-audit.md     ← Phase 8 文档 + 最终审计
└── templates/
    ├── STATE.md         ← 状态文件模板
    └── adr.md           ← ADR 模板
```

## 核心工作方式

**一个 Phase = 一个全新会话。**

不要试图在一个会话里跑完 8 个阶段——上下文会溢出，模型会开始遗忘早期决策、
重新发明架构、或者谎报测试结果。

每个会话的输入固定为两块：

```text
[00-contract.md 的全部内容]

---

[0N-xxx.md 的全部内容]
```

上下文的传递**不靠会话记忆，靠磁盘文件**：

```text
Phase 1  →  写出 docs/adr/*.md + docs/architecture.md + STATE.md
Phase 2+ →  开场先读 STATE.md 和 docs/adr/，然后才动代码
```

这就是为什么 `00-contract.md` 必须每次都粘贴：它定义了这套落盘协议。

## 逐阶段操作

### 首次

```bash
cd /path/to/your/project
git init
mkdir -p docs/adr
cp prompts/templates/STATE.md ./STATE.md
```

### 每个 Phase

1. 开新会话
2. 粘贴 `00-contract.md` + 当前阶段文件
3. 等待模型完成，检查输出中的 **Commands Run** 段落
4. **人工验证**（见下方检查清单）
5. 确认 `STATE.md` 已更新且与实际代码一致
6. `git commit -m "phase(N): ..."`
7. 关闭会话，进入下一阶段

### 每个 Phase 结束后你要亲自做的检查

```bash
# 别信输出里的"测试通过"，自己跑一遍
go build ./... && go vet ./... && go test ./... && go test -race ./...

# 看看有没有偷偷加依赖
git diff HEAD~1 -- go.mod

# 看看 STATE.md 是不是真的更新了
cat STATE.md
```

重点盯三件事：

| 检查项 | 为什么 |
|---|---|
| `Commands Run` 里有没有 `SKIPPED` | 模型最容易在这里含糊过去 |
| 是否做了超范围的事 | 提前实现后续阶段会破坏阶段隔离 |
| `STATE.md` 的 `Not Implemented Yet` 是否诚实 | 下一阶段全靠这个判断前提 |

## 阶段依赖关系

```text
Phase 1 (ADR)
   │
   ├─ 锁定：技术栈 / 目录 / 错误模型 / ID / Auth / RateLimit / Messaging
   ▼
Phase 2 (Foundation)  ← 含 migrate + sqlc 工具链
   ▼
Phase 3 (User/Auth/RBAC/RateLimit/OpenAPI)
   ▼
Phase 4 (Messaging)   ← 若 ADR-0009 判定不需要 MQ，只做接口抽象
   ▼
Phase 5 (Observability)  ← 依赖 3 和 4 才有东西可观测
   ▼
Phase 6 (Docker/CI/K8s)
   ▼
Phase 7 (goscaffold CLI) ← 依赖 2-6 全部稳定，否则模板立刻过期
   ▼
Phase 8 (Docs + Final Audit)
```

**不要跳阶段。** 尤其不要在 Phase 6 之前做 Phase 7——
CLI 模板必须复制一个已经定型的项目，否则 template drift 会立刻发生。

## 可以裁剪的部分

如果你的场景用不上，可以整段删掉：

| 阶段 | 何时可以跳过 |
|---|---|
| Phase 4 Messaging | 纯同步 CRUD 服务。但建议保留 4.1 的 Publisher 接口抽象 |
| Phase 7 CLI | 只做单个服务、不需要复制脚手架时 |
| Phase 6 的 K8s 部分 | 只用 Docker Compose 部署时 |

裁剪后记得同步删掉 Phase 8 审计清单里对应的检查项。

## 常见失控信号

看到这些就打断，让它重来：

```text
"我先把后面几个阶段一起做了"        → 违反 §0.6
"测试应该可以通过"                  → 违反 §0.5，让它真跑
"我重新设计了一下目录结构"          → 违反 §0.3，除非新建了 ADR
输出里出现 BaseService/GenericRepo   → 违反 §0.2
架构评分 9/10 但没跑 lint            → 违反 §8.18，要客观证据
STATE.md 没更新就说完成了            → 违反 §0.3
```

## 关于 Go 版本

`00-contract.md` 写的是 `Go 1.27+`。开工前确认：

```bash
go version
```

版本对不上就直接改 `00-contract.md` 里的基线，不要让模型自己猜。
