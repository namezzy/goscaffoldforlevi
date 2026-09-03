# Bruno 调试集合

用 [Bruno](https://usebruno.com) 手工调这套 API。`.bru` 是纯文本,进 git。

## 用法

1. 复制凭据文件并填入管理员密码:

   ```bash
   cp api/bruno/.env.example api/bruno/.env
   # 编辑 .env,填 BRUNO_ADMIN_PASSWORD
   ```

   密码由 `make bootstrap-admin` 打印,只显示一次。丢了就再建一个管理员。

2. 起服务(见根目录 Makefile),确认 `curl localhost:8080/live` 返回 200。

3. Bruno GUI:`Open Collection` 选 `api/bruno`,右上角环境选 `local`。

4. **先点 `auth/login`。** 它把 access token 存进变量,之后所有标了
   `auth: inherit` 的请求自动带上,不需要再手工复制。

## 整体跑一遍

```bash
cd api/bruno
npx --yes @usebruno/cli@latest run . --env local -r
```

必须在集合根目录执行,在上层跑会报 `You can run only at the root of a collection`。

24 个请求,覆盖 `api/openapi.yaml` 全部 12 个 operation。

## 顺序

文件夹按 `folder.bru` 的 `seq` 执行:health → auth → users → errors。
顺序不是审美问题——`users/get` 依赖 `users/create` 写入的 `userId`,
`errors/login-disabled-account` 依赖前面两个 setup 请求。

单独点某个请求时,先确认它依赖的变量已经有值。

## 这不是测试套件

回归门禁是 `test/blackbox` 的 110 条断言和 `make test-integration`。

这里的断言只回答一个问题:**是环境坏了,还是你正在调的那个东西坏了。**
不要往这里加回归用例——它需要一个跑着的服务和一份真实凭据,不适合当门禁。

## 最值得看的三个请求

`errors/` 里的 `login-wrong-password`、`login-unknown-email`、
`login-disabled-account` 必须返回**逐字相同**的响应:401、
`INVALID_CREDENTIALS`、`invalid email or password`。

第三个尤其值得盯:那组凭据是对的,几个请求之前还返回 200,唯一的变化是
账号被停用。三者只要有一处不同,这个接口就变成了账号枚举器。

`errors/` 为此自带一个成员账号的完整生命周期,不复用 `users/` 那个主体——
后者会被 `users/delete` 先删掉,`login-disabled-account` 于是走"邮箱不存在"
分支。两条分支的响应逐字相同,断言照样绿,测的却是另一回事。

## 已知限制

- **一次完整运行发起 6 次登录**,配额是 10 次 /15 分钟(按 IP)与
  5 次 /15 分钟(按邮箱地址)。**连着跑第二遍会从 `login-as-member`
  开始 429。** 畸形 body 那次也算数——限流器在发现 body 解析不了之前
  就已经扣过账了。清掉限流键即可继续:

  ```bash
  docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' \
    | xargs -r docker exec -i goscaffold-redis redis-cli del
  ```

- **CLI 与 GUI 的变量作用域不同。** `bru run` 每次都是全新进程,运行时
  变量从空开始;GUI 在一次会话里跨点击保留。所以 GUI 里一次失败的登录
  可能留下上一轮的 token,让后面的请求拿着它继续绿。集合里凡是写变量的
  脚本都带状态判断,`errors/login-as-member` 还会先清空
  `memberAccessToken`。

- **查询串只写在 `url:` 里。** CLI 不读 `params:query`——它按 `url:`
  原样构造请求,params 块里只认 path 类型的条目。GUI 行为不同,它的
  params 表会改写地址栏。

- **一致性检查只覆盖路径。** `collection_test.go` 断言每个请求的
  `(method, path)` 存在于 `api/openapi.yaml`,但不检查请求体字段和响应
  schema。后者是 `internal/openapitest` 的职责,它比对的是实现与文档。

- **不进 CI。** 需要跑着的服务和真实凭据。
