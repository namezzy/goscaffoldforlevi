# 前端视觉统一（科技感暗色主题）设计

日期:2026-10-12 · 范围:`frontend/` 全部页面,只改表现层,不改行为与接口。

## 问题

- 两套视觉语言并存:监控台是暗色"控制室"(cyan + 等宽字体),其余页面是
  中性色、跟随系统明暗切换。
- 没有组件层:按钮、卡片、表格、输入框、徽章、加载/空/错误状态在 20 个页面里
  各自手写 Tailwind 类,同一个"次要按钮"有十几份拷贝,细节各不相同。

## 取舍

| 方案 | 结论 |
|---|---|
| A. 设计令牌 + 共享 UI 原语,逐页迁移 | **采用**。一次性消除重复,之后新页面天然一致 |
| B. 只在 CSS 里重映射颜色 | 不采用:`dark:` 变体与硬编码颜色互相打架,结构(间距、层级)仍不统一 |
| C. 引入组件库(MUI / shadcn) | 不采用:新增大依赖,与现有 Tailwind 写法冲突,收益不抵成本 |

## 视觉语言

- **单一暗色主题**(`color-scheme: dark`),去掉所有 `dark:` 变体。科技感以暗色为
  基底;两套主题会让"统一"翻倍成本。
- 背景 `#05070d`,叠一层极淡的网格与顶部 cyan 光晕(纯 CSS,`body` 上)。
- 面板:`bg-slate-900/50` + `border-white/[0.06]` + 轻微 `backdrop-blur`,圆角 `xl`。
- 强调色 cyan(`cyan-400`),主按钮为 cyan 渐变带辉光;危险红、成功绿、警告琥珀、
  信息天蓝 —— 只通过 `Badge` / `Button` 的 tone 使用。
- 文字:主 `slate-100`,次 `slate-400`,弱 `slate-500`。
- 等宽字体(`font-mono`)用于:数字指标、ID、时间戳、小号大写标签(eyebrow)。
- 焦点:cyan 描边 + 外发光,键盘可见。

## 组件(`src/components/ui/`)

| 组件 | 用途 |
|---|---|
| `Button` / `ButtonLink` | variant: primary / secondary / ghost / danger / success;size: sm / md |
| `Card` | 面板容器,可选 `title`、`actions`、`eyebrow` |
| `PageHeader` | 页标题 + 描述 + 右侧操作区,每个页面顶部唯一入口 |
| `Badge` | tone: neutral / cyan / green / amber / red / blue / violet |
| `Field` / `FieldGroup` / `inputClass` | 表单字段;`FieldGroup` 用于含多个控件、不能用 `<label>` 包裹的场景 |
| `Table` / `Th` / `Td` / `TableMessage` | 表格外壳、表头、单元格、加载/空状态行 |
| `StatCard` | 指标卡(可链接) |
| `Loading` / `EmptyState` / `ErrorState` | 统一的加载、空、错误+重试 |
| `Pagination` | 上一页/下一页 |
| `SegmentedControl` | 页内 tab / 范围切换 |

`MembersPage` 不再导出 `Field`/`inputClass`,所有页面从 `components/ui` 引入。
各业务的状态徽章(`ticketShared`、`incidentShared`)改为映射到 `Badge` 的 tone。

## 布局

- 侧边栏分组:**工作**(工作台、工单)、**管理**(成员、组织、邀请、角色、审计)、
  **运维**(异常中心、监控台)、**个人**(我的资料、安全中心);按权限过滤,
  空组隐藏。当前项左侧 cyan 指示条 + 背景高亮。
- 顶栏:页面标题、账号邮箱 + 角色徽章、通知铃、退出。
- 内容区统一 `max-w-6xl`,监控台不再特殊处理(全站已是同一主题)。
- 登录、忘记密码、重置密码、接受邀请共用 `AuthShell`。

## 约束

- 不改任何文案、角色(role)、可访问名称 —— 现有 23 个测试按文本与角色查元素,
  必须原样通过。
- 不改 API 调用、状态管理、路由。
- `npm run build`、`npm test`、`npm run lint`(无新增错误)通过。

## 验证

构建、测试、lint;本机 Vite + 真实后端,用无头浏览器逐页截图检查渲染与控制台无错误。
