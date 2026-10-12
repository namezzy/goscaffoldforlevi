# 异常中心前端 + 收尾（2026-10-12）

## 背景

Phase 8 之后的扩展（组织、邀请、工单、通知、审计、安全中心、异常中心）后端已
完整：`internal/incident` 的 10 个接口已挂在 `/api/v1/incidents`，OpenAPI 与
Bruno 都有。未完成的部分：

1. 前端没有任何异常中心入口：无 API 客户端、无页面、`permissions.ts` 缺
   `incident:read` / `incident:manage`，通知页对 `target_type = "incident"`
   的通知无法跳转。
2. `internal/scaffold` 的模板漂移测试失败：`internal/incident` 的 3 个测试
   文件没有嵌入模板。
3. 文档未记录本轮扩展（`frontend/README.md` 仍写"12 个 operation"）。

## 设计

沿用工单页面的结构，不引入新依赖。

- `src/api/types.ts`：`Incident`、`IncidentPage`、`IncidentSummary` 等，手抄
  `openapi.yaml`。
- `src/api/incidents.ts`：10 个接口各一个函数。
- `src/auth/permissions.ts`：补两个权限，角色映射照抄 `internal/auth/policy.go`
  （system_admin、operator 读写；auditor 只读）。
- `src/pages/incidentShared.tsx`：严重级别 / 状态 / 重放结果的中文标签与徽章。
- `src/pages/IncidentsPage.tsx`（`/incidents`）：顶部按严重级别统计的未解决数量
  （点击即筛选），状态 / 严重级别 / 搜索筛选，条件存于 URL，分页。
- `src/pages/IncidentDetailPage.tsx`（`/incidents/:id`）：详情、最后错误、处理
  记录；操作按后端状态机显示——仅 `open` 可确认，非 `resolved` 可解决，
  `replay_status = pending` 时不可再次重放（"已提交" 不等于 "已成功"）。
  负责人：可"指派给我"/"清除"；持有 `user:list` 时额外提供成员下拉。
- 路由用 `RequirePermission permission="incident:read"`，导航新增"异常中心"。
- `NotificationsPage.targetHref` 支持 `incident`。

`incidenthttp` 不加 handler 测试：与 ticket/org/invite 的 HTTP 层一致，服务层
已有单测，路由与契约由 `cmd/api/openapi_test.go` 覆盖。

## 验证

`go test ./...`（含模板漂移）、`npm run build`、`npm run lint`、`npm test`。
