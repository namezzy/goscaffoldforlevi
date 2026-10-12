# goscaffoldforlevi — frontend

一个针对 `../api/openapi.yaml` 写的管理后台：登录/邀请/找回密码、工作台、
个人资料与安全中心（会话管理）、成员与组织架构、邀请、角色权限、操作审计、
工单、通知、监控台（实时请求、消息队列）、异常中心。React + Vite +
TypeScript + Tailwind CSS v4，没有引入额外的组件库或状态管理库。

界面统一使用暗色科技风主题（青色强调、等宽数字）。页面只通过
`src/components/ui` 里的共享组件（`PageHeader`、`Card`、`Table`、`Badge`、
`Button` 等）与样式常量（`inputClass`、`controlClass` 等）搭建，新增页面请沿用，
不要再写 `dark:` 变体或零散的颜色类。

## 运行

后端必须先跑起来（见仓库根目录 `README.zh-CN.md`），且其
`HTTP_CORS_ALLOWED_ORIGINS` 要包含这个前端的地址——`.env`/`.env.example`
里默认就是 `http://localhost:3000`，与 `vite.config.ts` 里固定的开发端口一致。

```sh
cp .env.example .env   # 默认值就能用，指向 http://localhost:8080
npm install
npm run dev             # http://localhost:3000
```

## 目录

```
src/
  api/        # HTTP 客户端：config/errors/http（无鉴权）/client（带 token），
              # 其余每个文件对应后端一个功能（users/tickets/incidents/…）
  auth/       # 鉴权状态：jwt 解码、外部 store（access token 内存持有，
              # refresh token 存 localStorage，快过期时自动刷新）、useAuth hook
  components/ # Layout、路由守卫、Modal、ConfirmDialog、ErrorBanner
  pages/      # 每个路由一个页面；*Shared.tsx 放同一功能多页共用的标签与徽章
```

## 权限

访问令牌只带 `sub`（用户 id）和 `role`，没有邮箱等个人信息（后端刻意如此，
见 `api/openapi.yaml` 里 `bearerAuth` 的说明）。角色有五种（system_admin、
member_admin、operator、read_only、auditor），`src/auth/permissions.ts` 照抄
后端 `internal/auth/policy.go` 的权限表，决定导航与按钮是否出现——前端这层
判断只是为了不让人看到点了也会 403 的按钮，真正的授权仍然在后端。

## 已知限制

- 没有生成 TypeScript 类型的工具，`src/api/types.ts` 是手抄 `openapi.yaml`
  的 schema，后端契约变了要手动同步。
- 权限表同样是手抄，后端 `policy.go` 改了要同步 `permissions.ts`。
- 测试只覆盖 `src/api/http.ts`（`npm test`），页面没有测试。
