# ADR-0004: API 约定（响应格式、分页、HTTP 语义）

- **Status**: Accepted
- **Date**: 2026-08-29
- **Phase**: Phase 1
- **Supersedes**: None
- **Superseded by**: None

## Context

需要确定对外 HTTP 契约的三件事：成功响应是否用 envelope、分页策略、更新用 PUT 还是 PATCH。
这些一旦定下就很难改，且会被 OpenAPI、前端、SDK 全部固化。

## Decision

### 1. 成功响应不使用 envelope

**否决 `{"code":0,"message":"success","data":{}}`。**

单资源直接返回资源本身：

```http
GET /api/v1/users/{id}  →  200
{
  "id": "01930f8e-...",
  "email": "a@example.com",
  "name": "Alice",
  "status": "active",
  "created_at": "2026-08-29T12:00:00Z",
  "updated_at": "2026-08-29T12:00:00Z"
}
```

理由：

1. **HTTP 状态码已经是 envelope。** `code: 0` 与 `200 OK` 表达同一件事，重复且可能矛盾。
2. **envelope 直接催生"所有 API 都返回 200"的反模式**——一旦客户端习惯读 `body.code`，
   状态码就退化成装饰，缓存/网关/重试/监控全部失效。这是本项目明确禁止的。
3. **破坏工具链。** OpenAPI codegen、`curl | jq`、浏览器 devtools 都要多剥一层。
4. 需要携带元数据的场景只有列表，那里用专门的结构表达即可，不需要全局 envelope。

**错误响应则统一使用 envelope**（见 ADR-0003）。非对称是有意的：
成功时数据形状由资源决定，失败时形状必须可预测。

### 2. 列表响应

```http
GET /api/v1/users?page=1&page_size=20  →  200
{
  "items": [ { ... }, { ... } ],
  "page_info": {
    "page": 1,
    "page_size": 20,
    "total": 137,
    "total_pages": 7
  }
}
```

`items` 永远是数组，**空结果返回 `[]` 而非 `null`**。

### 3. 分页：v1 用 offset

| 维度 | offset | cursor |
|------|--------|--------|
| 实现复杂度 | 低 | 中（编解码游标、多列排序、正反向） |
| 前端体验 | 可显示总数、跳页 | 只能上一页/下一页 |
| 并发插入下的稳定性 | 会重复/跳过 | 稳定 |
| 深翻页性能 | O(offset)，10 万行后劣化 | O(1) |

**选择 offset。** 决定性因素是使用场景：v1 的列表是**管理后台的用户列表**，
产品需求就是"第几页 / 共多少条 / 跳到第 N 页"，cursor 无法满足。
在数据量到达十万级之前，offset 的缺点不会显现。

两项必须做的缓解措施：

- **确定性排序**：`ORDER BY created_at DESC, id DESC`。
  仅按 `created_at` 排序在同一毫秒插入多行时顺序不稳定，翻页会重复。
  加 `id`（UUIDv7，本身有序）构成全序。
- **上限**：`page_size` 默认 20，**最大 100**；`page` ≥ 1。
  超出返回 400 而非静默截断——静默截断会让客户端以为拿到了全部数据。

`page_info` 的结构预留了演进空间：将来增加 `next_cursor` 字段属于向后兼容的新增。

> **When to Revisit**：users 表 > 100 万行，或 P95 深翻页延迟 > 200ms，
> 或出现面向 C 端的信息流场景 → 增加 cursor 模式（新增查询参数，保留 offset 兼容）。

### 4. 更新用 PATCH

```http
PATCH /api/v1/users/{id}
{ "name": "New Name" }
```

请求 DTO 所有字段可选（Go 中用指针或 `Optional[T]` 区分"未提供"与"显式置零"）。

理由：客户端通常只想改一两个字段。用 PUT 要求提交完整资源，
意味着客户端必须先 GET、合并、再提交——这不仅啰嗦，还引入
"GET 与 PUT 之间被他人修改"的丢失更新窗口。

**全项目统一使用 PATCH，不同时提供 PUT。** 提供两个语义相近的动词
只会让客户端困惑并使服务端逻辑翻倍。

**已知代价**：PATCH 不是幂等的天然保证（虽然本项目的实现是幂等的），
且"未提供"与"设为 null"的区分需要在 DTO 里显式处理，比 PUT 麻烦。

### 5. 路由与状态码约定

| 操作 | 方法与路径 | 成功状态 | 响应体 |
|------|-----------|---------|--------|
| 创建 | `POST /api/v1/users` | **201** + `Location` 头 | 资源 |
| 列表 | `GET /api/v1/users` | 200 | `{items, page_info}` |
| 详情 | `GET /api/v1/users/{id}` | 200 | 资源 |
| 更新 | `PATCH /api/v1/users/{id}` | 200 | 更新后的资源 |
| 删除 | `DELETE /api/v1/users/{id}` | **204** | 空 |
| 登录 | `POST /api/v1/auth/login` | 200 | token 对 |
| 刷新 | `POST /api/v1/auth/refresh` | 200 | token 对 |
| 登出 | `POST /api/v1/auth/logout` | 204 | 空 |

删除返回 204 且**对不存在的 ID 返回 404**（不做幂等 204）——
让客户端能区分"删掉了"和"本来就没有"。

### 6. 通用约定

- 请求/响应字段一律 `snake_case`（与 JSON 生态和 OpenAPI 惯例一致）
- 时间一律 RFC 3339 UTC，字段名以 `_at` 结尾
- ID 在 API 中是字符串（UUIDv7 的标准文本形式）
- 版本在路径中：`/api/v1`。破坏性变更开 `/api/v2`，不做 header 版本协商
- 所有响应携带 `X-Request-ID`
- 请求体大小上限 1 MB（`http.MaxBytesReader`），超出返回 413
- 未知的 JSON 字段**拒绝**（`DisallowUnknownFields`），返回 400。
  静默忽略拼错的字段名是难以排查的 bug 来源

### 7. OpenAPI 是单一真相

`api/openapi.yaml` 手写维护（不从代码生成，也不生成代码）。

一致性通过测试保证：Phase 3 起提供一个测试，
遍历 chi 路由树，断言每个 `(method, pattern)` 都在 spec 中存在，反之亦然。
**这条测试是防止 drift 的机制，文档纪律不是。**

## Consequences

### 正面

- 响应体干净，工具链友好
- 状态码保持语义，网关/CDN/监控可以正确工作
- 分页有明确上限，不存在 `page_size=999999` 的打库路径
- 路由与 spec 的一致性由 CI 保证

### 负面 / 代价

- 没有 envelope 意味着某些前端框架的统一拦截器要按状态码写，而不是按 `body.code`
- offset 分页在数据量增长后需要迁移工作
- PATCH 的可选字段处理在 Go 里比 PUT 繁琐（需要指针或 Optional 包装）
- 手写 OpenAPI 需要纪律 + 一致性测试兜底

### 后续影响

- Phase 3 的所有 handler 必须遵守本约定，并同步更新 `api/openapi.yaml`
- Phase 3 必须实现路由↔spec 一致性测试
- Phase 8 的验收门槛包含"所有已注册路由存在于 openapi.yaml"

## Alternatives Considered

| 方案 | 优点 | 缺点 | 未选择的原因 |
|------|------|------|--------------|
| 统一 envelope `{code,message,data}` | 前端拦截器写一次 | 与 HTTP 语义重复，诱发全 200 反模式 | 明确禁止 |
| JSON:API / HAL | 标准化、自描述 | 极其啰嗦，客户端心智负担大 | 对内部/B2B API 是过度设计 |
| cursor 分页 | 稳定、深翻页快 | 无法跳页和显示总数 | v1 场景是管理后台，需求不匹配 |
| PUT 全量更新 | 幂等语义清晰 | 客户端需先 GET 再合并，有丢失更新窗口 | 实际使用体验更差 |
| 代码生成 OpenAPI（注解） | 不会 drift | 注解污染 handler，且生成的 spec 质量差 | 用一致性测试达到同样效果 |

## When to Revisit

- 出现面向 C 端的大数据量列表 → 引入 cursor
- 出现需要部分成功语义的批量接口 → 该接口可能需要专门的响应结构（207 或自定义）
- 客户端 SDK 需要自动生成 → 重新评估是否改为 spec-first codegen
