# Phase 7 — Scaffold CLI

先读取 `STATE.md`。

目标：开发 `goscaffold` CLI，减少重复机械工作。

**它不是 Low-Code 平台。保持简单。**

## 7.1 Commands

至少：

```bash
goscaffold new my-service --module github.com/acme/my-service
goscaffold module user
goscaffold migration create_users
goscaffold version
```

## 7.2 Templates

模板通过 `embed.FS` 内嵌，**CLI 运行不得依赖外部模板目录**。

## 7.3 new Command

参数：`name / --module / --output`

可选开关：`--with-redis / --with-messaging / --with-observability`

**不要第一版就做十几个开关。**

职责：创建目录 / 复制模板 / 替换 module name / 替换 service name / 初始化基础配置。

## 7.4 Module Generator

```bash
goscaffold module order
```

生成：domain / service / repository contract / handler / DTO / test skeleton /（可选）SQL query 文件。

**模板必须符合项目当前真实架构。禁止重新引入 Java 式分层。**

## 7.5 Safety

**绝对禁止覆盖已有文件。**

文件存在时默认 **fail**（或 skip 并报告 conflict）。
预留 `--force` 扩展点，第一版可不实现。

## 7.6 Migration Generator

```text
20260829123456_create_users.up.sql
20260829123456_create_users.down.sql
```

时间戳必须避免同秒冲突，必要时提高精度。

## 7.7 Naming

正确处理 `PascalCase / camelCase / snake_case / 复数`。

**不要引入大型 inflection library，除非确实必要。**

## 7.8 CLI Framework

分析 `标准库 flag / Cobra / urfave-cli`。

如果只有几个命令，**不要为了流行而引入过重依赖**。

## 7.9 Dry Run

```bash
goscaffold module order --dry-run
```

显示将创建哪些文件，不写磁盘。

## 7.10 Config

评估是否支持 `.goscaffold.yaml`。**如果第一版没有必要，就不要实现。**

## 7.11 Plugin

**只设计扩展点，不实现 Plugin System。**

当前保持：embedded official template。

## 7.12 Template Drift（关键设计题）

CLI Template 与 Starter Project 极易 drift。

**必须设计解决方案，优先 single source of truth。**

禁止长期维护两套独立代码。给出你的方案并说明为什么可行。

## 7.13 CLI Tests

```text
new project / module generation / migration generation
existing file conflict / dry-run / name conversion
```

**测试必须在临时目录运行，禁止污染项目目录。**

## 7.14 Generated Code Verification（关键质量门槛）

CI 增加一个 job：

```bash
goscaffold new demo --module github.com/example/demo
cd demo && go mod tidy && go vet ./... && go build ./... && go test ./...
```

**确保脚手架生成的项目真的能编译和运行。**

## 7.15 Phase 7 输出

执行 §0.4 全部门禁，并在临时目录验证生成项目。输出：

```text
CLI Commands
Template Architecture
Template Drift Solution
Safety Mechanisms
Generation Examples
Tests / Generated Project Verification
本阶段有没有做超范围的事？
Known Limitations
```

更新 `STATE.md` 并提交。
