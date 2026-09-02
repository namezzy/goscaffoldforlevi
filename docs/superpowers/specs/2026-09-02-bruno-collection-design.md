# Bruno 调试集合设计

- **Date**: 2026-09-02
- **Status**: Approved
- **相关**: `api/openapi.yaml`、`test/blackbox/`、ADR-0004(API 约定)

## 目的

给这套 API 一份可以**手工点、可版本管理**的调试集合,用 Bruno。

要解决的具体痛点是:调试认证后面的接口时,每次都得先登录、把 access token
从响应里复制出来、粘进下一个请求的 Authorization 头。token 15 分钟过期,
于是这个动作一天要做几十次。

## 不是什么

**不替代 `test/blackbox` 的 110 条断言。** 黑盒套件是门禁,在终端跑,失败
即阻断。Bruno 集合是给人点的:你想看某个 400 响应的完整 body 长什么样、想
改一个字段再发一次、想确认限流头到底带了哪几个值。

两者会有覆盖上的重叠,这是有意的。区别在于**当集合里的断言失败时,它回答
的是"环境坏了还是我要调的东西坏了"**,而不是"要不要阻止合并"。

## 位置

`api/bruno/`,紧挨着 `api/openapi.yaml`。

理由:它们是同一份接口契约的两种表现,并且会一起过期。放在一起,是为了让
改动其中一个的人看得见另一个。

## 结构

```
api/bruno/
├── bruno.json                  集合定义(name / version / type)
├── collection.bru              集合级 Bearer auth + 通用 header
├── environments/
│   └── local.bru               baseUrl;凭据走 process.env
├── .env.example                （.env 被 .gitignore 覆盖）
├── health/
│   ├── health.bru  live.bru  ready.bru
├── auth/
│   ├── login.bru  refresh.bru  logout.bru
│   └── errors/
│       ├── wrong-password.bru  unknown-email.bru
│       ├── disabled-account.bru  malformed-body.bru
└── users/
    ├── create.bru  list.bru  get.bru  update.bru
    ├── disable.bru  change-password.bru  delete.bru
    └── errors/
        ├── no-token.bru  forbidden.bru  not-found.bru  tampered-token.bru
```

共 21 个请求:12 个覆盖 `openapi.yaml` 的全部 12 个 operation,1 个是 PATCH 的
第二种用法(停用账号),8 个是失败路径。

## 变量流转

集合级 auth 统一使用 `{{accessToken}}`,因此单个请求不需要写 Authorization 头。

| 请求 | post-response 写入 |
|---|---|
| `auth/login` | `accessToken`、`refreshToken` |
| `auth/refresh` | `accessToken`、`refreshToken`(**覆盖**,因为 refresh 会轮转) |
| `users/create` | `userId`(给 get / update / change-password / delete 用) |

点一次 login,后面全部可用。这是这份集合存在的主要理由。

`auth/logout` 之后 `accessToken` 仍然有效直到过期——这是 ADR-0007 的设计
(access token 是无状态的,logout 撤销的是 refresh family),集合的注释里要
写明,否则它看起来像 bug。

## 凭据处理

管理员密码**不进仓库**。`environments/local.bru` 里写成:

```
vars {
  adminPassword: {{process.env.BRUNO_ADMIN_PASSWORD}}
}
```

配一份 `api/bruno/.env.example` 说明需要哪些变量。根 `.gitignore` 的 `.env`
规则不带斜杠,因此对 `api/bruno/.env` 同样生效,无需新增规则。

这一条来自教训:黑盒套件曾硬编码管理员密码,轮转之后整套以 401 失败,
看起来像授权 bug,实际是过期 fixture。

## 断言

每个请求都带断言,**包括失败路径的**——期望 401 就断言 401。这样整个集合
可以 `bru run` 一把跑完当冒烟检查。

失败路径的断言还要覆盖错误体的形状(`error.code`),因为 ADR-0003 规定了
统一的错误包装,而"状态码对了但 body 变了"是客户端最难排查的一类回归。

**登录的三种失败必须断言同一个 401 与同一个 `INVALID_CREDENTIALS`**:
密码错、邮箱不存在、账号被禁用。区分它们等于给攻击者一个账号枚举接口。
这是集合里最值得存在的三个请求——三个响应必须逐字相同,肉眼一比就能看出来。

第三种需要先构造:`users/create` → `users/disable`(PATCH `status=disabled`)
→ 用该账号登录。因此 `users/disable.bru` 存在的目的不是演示 PATCH,而是让
这条断言可点。

## 一致性测试

`api/bruno/collection_test.go`(package `bruno`):

1. 递归读取全部 `.bru` 文件,解析出 `method` 与 `url`
2. 剥掉 `{{baseUrl}}` 前缀,把 `{{userId}}` 一类的变量还原为 `{id}`
3. 断言得到的 `(method, path)` 存在于 `api/openapi.yaml`

理由和 Phase 5 的 `deployments/observability/dashboard_test.go` 相同:
**这是又一个会静默失效的产物。** 一个指向已删除接口的请求点下去返回 404,
和"服务挂了"长得一模一样;人会先去查服务,而不是怀疑请求本身。

测试只做单向检查(集合 → spec),不要求 spec 的每个 operation 都有对应请求
——反向要求会让新增接口时集合变成阻塞项,而那正是让人删掉测试的原因。

## 验证方式

`npx @usebruno/cli run api/bruno --env local`,不往仓库添加 npm 依赖。

需要服务在跑(`:8080`)且 `BRUNO_ADMIN_PASSWORD` 已设置。

## 已知限制

1. **集合与 spec 的漂移只在路径层面被检测。** 请求体字段、响应 schema 的
   漂移不在检查范围内——那是 `internal/openapitest` 的职责,它检查的是实现
   与 spec,而不是集合与 spec。
2. **`bru run` 的顺序依赖变量流转。** 单独运行 `users/get` 而不先运行
   `login` 与 `create` 会失败。Bruno 的 seq 保证集合整体运行时的顺序,
   但手工点选时需要自己注意。
3. **没有 CI 集成。** 这份集合不进 `make check`;它需要一个跑着的服务和一份
   凭据,而 CI 里已经有黑盒套件承担这件事。
