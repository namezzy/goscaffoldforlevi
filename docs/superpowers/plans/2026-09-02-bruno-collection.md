# Bruno 调试集合 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `api/bruno/` 建一份 git 跟踪的 Bruno 集合,登录一次后 token 自动流转给后续请求,并用一个 Go 测试防止它悄悄指向已删除的接口。

**Architecture:** 四个顶层文件夹按 `folder.bru` 的 `seq` 顺序执行(health → auth → users → errors)。集合级 `collection.bru` 声明 Bearer auth 用 `{{accessToken}}`,单个请求写 `auth: inherit` 即可。`script:post-response` 把 token 与 id 写进 runtime 变量。`api/bruno/collection_test.go` 解析每个 `.bru` 的 method+url,断言它存在于 `api/openapi.yaml`。

**Tech Stack:** Bruno 4.1(GUI 已装)、`@usebruno/cli`(经 npx 调用,不进仓库)、Go + `gopkg.in/yaml.v3`(已是直接依赖)。

**前置条件:** 服务跑在 `:8080`,PostgreSQL/Redis 容器在跑,`BRUNO_ADMIN_PASSWORD` 已设为管理员密码。

---

## 已经实测确认的事实

写这份计划前逐条跑过真实服务,下面的值不是推测:

| 请求 | 状态码 | 响应要点 |
|---|---|---|
| `GET /health` `/live` | 200 | `{"status":"ok","version":"dev"}` |
| `GET /ready` | 200 | 多一个 `components: {postgres, redis}` |
| `POST /api/v1/auth/login` | 200 | `access_token` / `refresh_token` / `token_type: Bearer` / `expires_in: 900` |
| `POST /api/v1/auth/refresh` | 200 | 同上,refresh token 被轮转 |
| `POST /api/v1/auth/logout` | 204 | 空体 |
| `POST /api/v1/users` | 201 | `id,email,name,status,role,created_at,updated_at` |
| `GET /api/v1/users` | 200 | `{items:[...], page_info:{page,page_size,total,total_pages}}` |
| `PATCH /api/v1/users/{id}` | 200 | 完整 user 体 |
| `PUT /api/v1/users/{id}/password` | 204 | 空体 |
| `DELETE /api/v1/users/{id}` | 204 | 空体 |
| 密码错 / 邮箱不存在 / 账号禁用 | 401 | **三者逐字相同**:`INVALID_CREDENTIALS` / `invalid email or password` |
| 无 token | 401 | `UNAUTHENTICATED` / `authentication required` |
| 篡改 token | 401 | `INVALID_TOKEN` / `invalid or expired token` |
| 成员列用户 | 403 | `FORBIDDEN` / `you do not have permission to perform this action` |
| 不存在的 id | 404 | `USER_NOT_FOUND` / `user not found` |
| body 非 JSON | 400 | `REQUEST_BODY_INVALID` / `request body is not valid JSON` |

`.bru` 语法也是跑通验证过的,包括 `collection.bru` 的 auth 继承、`{{process.env.X}}`、`bru.setVar`、`assert` 块,以及 `folder.bru` 的 `seq` 控制文件夹顺序。

**`bru run` 必须在集合根目录执行**(`cd api/bruno && bru run . -r`),在上层目录跑会报 `You can run only at the root of a collection`。

---

## File Structure

| 文件 | 职责 |
|---|---|
| `api/bruno/bruno.json` | 集合定义。Bruno GUI 靠它识别目录 |
| `api/bruno/collection.bru` | 集合级 Bearer auth,单点定义 `{{accessToken}}` |
| `api/bruno/environments/local.bru` | `baseUrl` 与凭据来源 |
| `api/bruno/.env.example` | 说明需要哪些环境变量 |
| `api/bruno/README.md` | 怎么用、为什么这么排、已知限制 |
| `api/bruno/health/` | 3 个探活请求,不需要认证 |
| `api/bruno/auth/` | 登录 / 轮转 / 登出,负责写入 token 变量 |
| `api/bruno/users/` | 一个用户的完整 CRUD 生命周期 |
| `api/bruno/errors/` | 失败路径,自带成员用户的生命周期 |
| `api/bruno/collection_test.go` | 集合路径 → openapi.yaml 的单向一致性检查 |

共 24 个请求。

---

## Task 1: 一致性测试与集合骨架

先写测试。它此刻必然失败——目录还不存在——这正是我们要看到的。

**Files:**
- Create: `api/bruno/collection_test.go`
- Create: `api/bruno/bruno.json`
- Create: `api/bruno/collection.bru`
- Create: `api/bruno/environments/local.bru`
- Create: `api/bruno/.env.example`
- Create: `api/bruno/health/folder.bru`
- Create: `api/bruno/health/health.bru`
- Create: `api/bruno/health/live.bru`
- Create: `api/bruno/health/ready.bru`

- [ ] **Step 1: 写失败的测试**

创建 `api/bruno/collection_test.go`:

```go
// Package bruno holds a Bruno collection for poking at the API by hand.
//
// The test in this file exists because the collection is the kind of artefact
// that fails silently. A request pointing at an endpoint that was renamed or
// removed returns 404, which looks exactly like the service being down, and
// the first instinct is to go and check the service.
package bruno

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// request is one .bru file reduced to the only thing this test cares about.
type request struct {
	file   string
	method string
	path   string
}

// verbs is the closed set of HTTP methods a .bru block can open with. It is a
// set rather than a regexp because the same list is needed when walking the
// OpenAPI document, where a path item also carries non-verb keys such as
// "parameters".
var verbs = map[string]bool{
	"get": true, "post": true, "put": true,
	"patch": true, "delete": true, "head": true, "options": true,
}

// blockRe matches the opening block of a .bru request:
//
//	post {
//	  url: {{baseUrl}}/api/v1/auth/login
var blockRe = regexp.MustCompile(`(?m)^(get|post|put|patch|delete|head|options)\s*\{\s*\r?\n\s*url:\s*(\S+)`)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var varRe = regexp.MustCompile(`\{\{[^}]*\}\}`)

func TestEveryRequestTargetsADocumentedOperation(t *testing.T) {
	documented := documentedOperations(t)
	requests := collectionRequests(t)

	// Without this the test passes by finding nothing, which is the failure
	// mode that matters: someone moves the collection and the guard silently
	// stops guarding.
	if len(requests) == 0 {
		t.Fatal("no .bru requests found; the test would have passed vacuously")
	}

	for _, r := range requests {
		key := r.method + " " + r.path
		if !documented[key] {
			t.Errorf("%s targets %q, which api/openapi.yaml does not document", r.file, key)
		}
	}
}

// documentedOperations reads api/openapi.yaml and returns the set of
// "METHOD /path" strings it declares.
func documentedOperations(t *testing.T) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}

	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	out := make(map[string]bool)
	for path, item := range doc.Paths {
		for key := range item {
			if verbs[strings.ToLower(key)] {
				out[strings.ToUpper(key)+" "+path] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("openapi.yaml declared no operations; the comparison would be meaningless")
	}
	return out
}

// collectionRequests walks the collection and reduces every .bru request to a
// method and a spec-shaped path.
func collectionRequests(t *testing.T) []request {
	t.Helper()

	var out []request
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".bru" {
			return nil
		}
		// folder.bru and collection.bru carry settings, not requests.
		if base := filepath.Base(path); base == "folder.bru" || base == "collection.bru" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m := blockRe.FindStringSubmatch(string(raw))
		if m == nil {
			t.Errorf("%s has no request block; it cannot be run", path)
			return nil
		}
		out = append(out, request{
			file:   path,
			method: strings.ToUpper(m[1]),
			path:   specPath(m[2]),
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk collection: %v", err)
	}
	return out
}

// specPath turns a Bruno URL into the path shape the OpenAPI document uses.
//
// Both variables and literal UUIDs collapse to {id}. Literals appear in the
// error cases, where the whole point is to send an identifier that resolves to
// nothing, and they must not be mistaken for a separate endpoint.
func specPath(url string) string {
	if i := strings.IndexByte(url, '?'); i >= 0 {
		url = url[:i]
	}
	url = strings.TrimPrefix(url, "{{baseUrl}}")

	segments := strings.Split(url, "/")
	for i, s := range segments {
		switch {
		case varRe.MatchString(s):
			segments[i] = "{id}"
		case uuidRe.MatchString(s):
			segments[i] = "{id}"
		}
	}
	return strings.Join(segments, "/")
}
```

- [ ] **Step 2: 运行,确认它失败**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && go test ./api/bruno/ -run TestEveryRequestTargetsADocumentedOperation -v
```

预期:`no .bru requests found; the test would have passed vacuously`。

如果报的是 `build constraints exclude all Go files` 或 `no Go files`,说明文件没落在 `api/bruno/`,先修路径再继续。

- [ ] **Step 3: 建集合骨架**

`api/bruno/bruno.json`:

```json
{
  "version": "1",
  "name": "go-enterprise",
  "type": "collection",
  "ignore": [
    "node_modules",
    ".git"
  ]
}
```

`api/bruno/collection.bru` —— 集合级认证只在这里定义一次:

```
auth {
  mode: bearer
}

auth:bearer {
  token: {{accessToken}}
}
```

`api/bruno/environments/local.bru`:

```
vars {
  baseUrl: http://localhost:8080
  adminEmail: admin@example.com
}

vars:secret [
  adminPassword
]
```

> 注意:`adminPassword` 的值不写在这里。它由 `.env` 提供,请求体里用
> `{{process.env.BRUNO_ADMIN_PASSWORD}}` 引用。

`api/bruno/.env.example`:

```
# Copy to api/bruno/.env and fill in. The root .gitignore has a bare `.env`
# rule, which git applies at any depth, so the real file stays out of history.
#
# The administrator password is printed once by `make bootstrap-admin` and
# never shown again. If you have lost it, create another administrator.
BRUNO_ADMIN_PASSWORD=
```

- [ ] **Step 4: 建 health 文件夹**

`api/bruno/health/folder.bru`:

```
meta {
  name: health
  seq: 1
}
```

`api/bruno/health/health.bru`:

```
meta {
  name: health
  type: http
  seq: 1
}

get {
  url: {{baseUrl}}/health
  body: none
  auth: none
}

assert {
  res.status: eq 200
  res.body.status: eq ok
}

docs {
  Aggregate health. Unauthenticated on purpose: a probe that needs a token
  cannot tell you the service is up when authentication is what broke.
}
```

`api/bruno/health/live.bru`:

```
meta {
  name: live
  type: http
  seq: 2
}

get {
  url: {{baseUrl}}/live
  body: none
  auth: none
}

assert {
  res.status: eq 200
  res.body.status: eq ok
}

docs {
  Liveness. This must not check dependencies: a liveness probe that fails
  when the database is down makes the orchestrator restart a healthy process
  during an outage, turning a database problem into an outage of everything.
}
```

`api/bruno/health/ready.bru`:

```
meta {
  name: ready
  type: http
  seq: 3
}

get {
  url: {{baseUrl}}/ready
  body: none
  auth: none
}

assert {
  res.status: eq 200
  res.body.status: eq ok
  res.body.components.postgres: eq ok
}

docs {
  Readiness. Unlike /live this does check dependencies, and it reports 503
  during shutdown drain so a rolling deploy loses no requests.
}
```

- [ ] **Step 5: 运行测试,确认通过**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && go test ./api/bruno/ -count=1 -v
```

预期:`PASS`,3 个请求全部匹配到 `openapi.yaml`。

- [ ] **Step 6: 用 Bruno CLI 实跑 health**

```bash
cd /Users/withlevi/Desktop/Project/Go_/api/bruno && npx --yes @usebruno/cli@latest run . --env local -r
```

预期:3 requests / 3 passed,7 个断言全绿。

- [ ] **Step 7: 提交**

```bash
cd /Users/withlevi/Desktop/Project/Go_
git add api/bruno
git commit -m "test(bruno): guard collection paths against the OpenAPI spec

The collection is added one folder at a time behind a test that checks
every request path against api/openapi.yaml. A request aimed at a removed
endpoint answers 404, which reads as the service being down.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 2: auth 文件夹 —— token 流转

这是整份集合存在的理由:登录一次,后面不再手工复制 token。

**Files:**
- Create: `api/bruno/auth/folder.bru`
- Create: `api/bruno/auth/login.bru`
- Create: `api/bruno/auth/refresh.bru`
- Create: `api/bruno/auth/logout.bru`

- [ ] **Step 1: 建文件夹并排序**

`api/bruno/auth/folder.bru`:

```
meta {
  name: auth
  seq: 2
}
```

- [ ] **Step 2: login —— 写入两个 token**

`api/bruno/auth/login.bru`:

```
meta {
  name: login
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/api/v1/auth/login
  body: json
  auth: none
}

body:json {
  {
    "email": "{{adminEmail}}",
    "password": "{{process.env.BRUNO_ADMIN_PASSWORD}}"
  }
}

script:post-response {
  bru.setVar("accessToken", res.body.access_token);
  bru.setVar("refreshToken", res.body.refresh_token);
}

assert {
  res.status: eq 200
  res.body.token_type: eq Bearer
  res.body.expires_in: eq 900
}

docs {
  Run this first. It stores accessToken and refreshToken, and every request
  with `auth: inherit` picks accessToken up from collection.bru.

  Login answers without X-RateLimit-* headers, unlike every other endpoint.
  A remaining count next to a refused login is a progress meter for whoever
  is guessing. Only Retry-After survives.
}
```

- [ ] **Step 3: refresh —— 覆盖两个 token**

`api/bruno/auth/refresh.bru`:

```
meta {
  name: refresh
  type: http
  seq: 2
}

post {
  url: {{baseUrl}}/api/v1/auth/refresh
  body: json
  auth: none
}

body:json {
  {
    "refresh_token": "{{refreshToken}}"
  }
}

script:post-response {
  bru.setVar("accessToken", res.body.access_token);
  bru.setVar("refreshToken", res.body.refresh_token);
}

assert {
  res.status: eq 200
  res.body.token_type: eq Bearer
}

docs {
  Refresh rotates: the token sent here is dead afterwards, so both variables
  are overwritten. Replaying an already-used refresh token is treated as
  theft and revokes the whole family (ADR-0007). The black-box suite covers
  that; it is deliberately not a request here, because triggering it would
  invalidate your session mid-run.
}
```

- [ ] **Step 4: logout**

`api/bruno/auth/logout.bru`:

```
meta {
  name: logout
  type: http
  seq: 3
}

post {
  url: {{baseUrl}}/api/v1/auth/logout
  body: json
  auth: none
}

body:json {
  {
    "refresh_token": "{{refreshToken}}"
  }
}

assert {
  res.status: eq 204
}

docs {
  Revokes the refresh family. The access token keeps working until it
  expires, which is not a bug: access tokens are stateless by design
  (ADR-0007), and that is why they are short-lived. This is why the users/
  folder still works after this request runs.
}
```

- [ ] **Step 5: 一致性测试**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && go test ./api/bruno/ -count=1
```

预期:`ok`。

- [ ] **Step 6: 实跑,确认 token 真的流转了**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' | xargs -r docker exec -i goscaffold-redis redis-cli del
cd api/bruno && npx --yes @usebruno/cli@latest run . --env local -r
```

预期:6 requests / 6 passed。顺序必须是 health 三个在前、auth 三个在后。
若 auth 跑在 health 前面,检查两个 `folder.bru` 的 `seq`。

- [ ] **Step 7: 提交**

```bash
cd /Users/withlevi/Desktop/Project/Go_
git add api/bruno/auth
git commit -m "feat(bruno): add the auth folder and the token flow

login stores accessToken once and collection.bru hands it to every later
request. That single step is the reason this collection exists; the token
lives fifteen minutes and copying it out of a response by hand is the part
that wastes the day.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 3: users 文件夹 —— 一个用户的完整生命周期

**Files:**
- Create: `api/bruno/users/folder.bru`
- Create: `api/bruno/users/create.bru`
- Create: `api/bruno/users/get.bru`
- Create: `api/bruno/users/list.bru`
- Create: `api/bruno/users/update.bru`
- Create: `api/bruno/users/change-password.bru`
- Create: `api/bruno/users/delete.bru`

- [ ] **Step 1: 文件夹排序**

`api/bruno/users/folder.bru`:

```
meta {
  name: users
  seq: 3
}
```

- [ ] **Step 2: create —— 生成唯一邮箱并写入 userId**

`api/bruno/users/create.bru`:

```
meta {
  name: create
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/api/v1/users
  body: json
  auth: inherit
}

script:pre-request {
  // A fixed address makes the second run fail with 409 and look like a bug
  // in create rather than leftover state from the first run.
  bru.setVar("userEmail", "bruno-user-" + Date.now() + "@example.com");
  bru.setVar("userPassword", "correct horse battery staple");
}

body:json {
  {
    "email": "{{userEmail}}",
    "name": "Bruno User",
    "password": "{{userPassword}}",
    "role": "user"
  }
}

script:post-response {
  bru.setVar("userId", res.body.id);
}

assert {
  res.status: eq 201
  res.body.status: eq active
  res.body.role: eq user
}

docs {
  Stores userId for the rest of this folder.

  role is optional and defaults to `user`. It is spelled out here only to
  show the field; leaving it out cannot create an administrator by accident,
  which is the reason the default is the least privileged value.
}
```

- [ ] **Step 3: get**

`api/bruno/users/get.bru`:

```
meta {
  name: get
  type: http
  seq: 2
}

get {
  url: {{baseUrl}}/api/v1/users/{{userId}}
  body: none
  auth: inherit
}

assert {
  res.status: eq 200
  res.body.id: eq {{userId}}
  res.body.email: eq {{userEmail}}
}
```

- [ ] **Step 4: list**

`api/bruno/users/list.bru`:

```
meta {
  name: list
  type: http
  seq: 3
}

get {
  url: {{baseUrl}}/api/v1/users?page=1&page_size=20
  body: none
  auth: inherit
}

params:query {
  page: 1
  page_size: 20
}

assert {
  res.status: eq 200
  res.body.page_info.page: eq 1
  res.body.page_info.page_size: eq 20
}

docs {
  Listing requires the user:list permission, so a member gets 403 here.
  errors/users-forbidden.bru is that same request with a member token.

  A page past the end returns an empty items array with the real total, not
  a 404: asking for page 99999999 is a valid question with a boring answer.
}
```

- [ ] **Step 5: update**

`api/bruno/users/update.bru`:

```
meta {
  name: update
  type: http
  seq: 4
}

patch {
  url: {{baseUrl}}/api/v1/users/{{userId}}
  body: json
  auth: inherit
}

body:json {
  {
    "name": "Bruno User Renamed"
  }
}

assert {
  res.status: eq 200
  res.body.name: eq Bruno User Renamed
  res.body.email: eq {{userEmail}}
}

docs {
  PATCH, not PUT. There is no full replacement for a user, because with PUT
  every omitted field becomes a deletion and a client that forgets one wipes
  it silently (ADR-0004).

  An empty body is rejected with 400 rather than answered 200: a patch that
  changes nothing is a client bug, and a success would hide it.
}
```

- [ ] **Step 6: change-password**

`api/bruno/users/change-password.bru`:

```
meta {
  name: change-password
  type: http
  seq: 5
}

put {
  url: {{baseUrl}}/api/v1/users/{{userId}}/password
  body: json
  auth: inherit
}

body:json {
  {
    "password": "a different correct horse"
  }
}

assert {
  res.status: eq 204
}

docs {
  PUT is right here even though updates use PATCH: a password has no partial
  state to merge. The whole value is replaced or nothing happens.

  Length is the only rule, 12 to 128. Character-class requirements push
  people toward predictable substitutions that add little entropy; the upper
  bound exists because the hash function processes the entire input.
}
```

- [ ] **Step 7: delete**

`api/bruno/users/delete.bru`:

```
meta {
  name: delete
  type: http
  seq: 6
}

delete {
  url: {{baseUrl}}/api/v1/users/{{userId}}
  body: none
  auth: inherit
}

assert {
  res.status: eq 204
}

docs {
  Runs last so the folder leaves nothing behind.

  Disabling is usually preferable to deleting: a disabled account keeps the
  audit trail intact and can be reversed. errors/ demonstrates that path.
}
```

- [ ] **Step 8: 一致性测试**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && go test ./api/bruno/ -count=1
```

预期:`ok`。`/api/v1/users/{{userId}}` 与 `/api/v1/users/{{userId}}/password` 应分别归一为 `/api/v1/users/{id}` 与 `/api/v1/users/{id}/password`。

- [ ] **Step 9: 实跑**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' | xargs -r docker exec -i goscaffold-redis redis-cli del
cd api/bruno && npx --yes @usebruno/cli@latest run . --env local -r
```

预期:12 requests / 12 passed。

- [ ] **Step 10: 提交**

```bash
cd /Users/withlevi/Desktop/Project/Go_
git add api/bruno/users
git commit -m "feat(bruno): add the user lifecycle folder

create through delete on one user, with the address generated per run. A
fixed address makes the second run fail with 409 and look like a bug in
create rather than leftover state.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 4: errors 文件夹 —— 失败路径

这个文件夹自带一个成员用户的完整生命周期,因为 403 与"禁用账号登录"两条断言都需要一个**活着的**非管理员账号。复用 `users/` 那个主体行不通:`users/delete` 会先把它删掉,`login-disabled-account` 就会走"邮箱不存在"分支,断言照样绿,测的却是另一回事。

**Files:**
- Create: `api/bruno/errors/folder.bru`
- Create: `api/bruno/errors/login-wrong-password.bru`
- Create: `api/bruno/errors/login-unknown-email.bru`
- Create: `api/bruno/errors/login-malformed-body.bru`
- Create: `api/bruno/errors/setup-member.bru`
- Create: `api/bruno/errors/login-as-member.bru`
- Create: `api/bruno/errors/users-forbidden.bru`
- Create: `api/bruno/errors/setup-disable-member.bru`
- Create: `api/bruno/errors/login-disabled-account.bru`
- Create: `api/bruno/errors/users-no-token.bru`
- Create: `api/bruno/errors/users-tampered-token.bru`
- Create: `api/bruno/errors/users-not-found.bru`
- Create: `api/bruno/errors/cleanup-member.bru`

- [ ] **Step 1: 文件夹排序**

`api/bruno/errors/folder.bru`:

```
meta {
  name: errors
  seq: 4
}
```

- [ ] **Step 2: 登录的三种失败 —— 前两种**

`api/bruno/errors/login-wrong-password.bru`:

```
meta {
  name: login wrong password
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/api/v1/auth/login
  body: json
  auth: none
}

body:json {
  {
    "email": "{{adminEmail}}",
    "password": "definitely not the password"
  }
}

assert {
  res.status: eq 401
  res.body.error.code: eq INVALID_CREDENTIALS
  res.body.error.message: eq invalid email or password
}

docs {
  One of three requests that must produce byte-identical responses. Compare
  this with login-unknown-email and login-disabled-account: same status,
  same code, same message.

  Any difference between them turns the endpoint into an account enumeration
  oracle, and "this account is disabled" is precisely the sentence an
  attacker wants to read.
}
```

`api/bruno/errors/login-unknown-email.bru`:

```
meta {
  name: login unknown email
  type: http
  seq: 2
}

post {
  url: {{baseUrl}}/api/v1/auth/login
  body: json
  auth: none
}

body:json {
  {
    "email": "nobody-here@example.com",
    "password": "correct horse battery staple"
  }
}

assert {
  res.status: eq 401
  res.body.error.code: eq INVALID_CREDENTIALS
  res.body.error.message: eq invalid email or password
}

docs {
  Second of the three identical failures.

  The service performs a full Argon2id verification against a dummy hash
  here, so the response time does not reveal that the address is unknown.
  Watch the duration: it should be indistinguishable from the wrong-password
  case above.
}
```

- [ ] **Step 3: 畸形 body**

`api/bruno/errors/login-malformed-body.bru`:

```
meta {
  name: login malformed body
  type: http
  seq: 3
}

post {
  url: {{baseUrl}}/api/v1/auth/login
  body: text
  auth: none
}

body:text {
  {
}

headers {
  Content-Type: application/json
}

assert {
  res.status: eq 400
  res.body.error.code: eq REQUEST_BODY_INVALID
}

docs {
  400, not 401. A request that never parsed cannot have failed
  authentication, and counting it as an authentication failure would let
  anyone raise the auth failure rate by posting garbage, turning the
  security alert into noise.
}
```

- [ ] **Step 4: 建成员用户**

`api/bruno/errors/setup-member.bru`:

```
meta {
  name: setup member
  type: http
  seq: 4
}

post {
  url: {{baseUrl}}/api/v1/users
  body: json
  auth: inherit
}

script:pre-request {
  bru.setVar("memberEmail", "bruno-member-" + Date.now() + "@example.com");
  bru.setVar("memberPassword", "correct horse battery staple");
}

body:json {
  {
    "email": "{{memberEmail}}",
    "name": "Bruno Member",
    "password": "{{memberPassword}}",
    "role": "user"
  }
}

script:post-response {
  bru.setVar("memberId", res.body.id);
}

assert {
  res.status: eq 201
  res.body.role: eq user
}

docs {
  Setup, not an example. This folder needs a live non-administrator to show
  both 403 and the disabled-account login, and it cannot borrow the one from
  users/ because users/delete removes it first. Borrowing it would leave
  login-disabled-account green while it actually exercised the unknown-email
  branch.
}
```

- [ ] **Step 5: 以成员身份登录**

`api/bruno/errors/login-as-member.bru`:

```
meta {
  name: login as member
  type: http
  seq: 5
}

post {
  url: {{baseUrl}}/api/v1/auth/login
  body: json
  auth: none
}

body:json {
  {
    "email": "{{memberEmail}}",
    "password": "{{memberPassword}}"
  }
}

script:post-response {
  bru.setVar("memberAccessToken", res.body.access_token);
}

assert {
  res.status: eq 200
}

docs {
  Writes memberAccessToken, deliberately not accessToken: overwriting the
  administrator's token here would make every later request fail with 403
  for reasons that have nothing to do with what they are testing.
}
```

- [ ] **Step 6: 403**

`api/bruno/errors/users-forbidden.bru`:

```
meta {
  name: users forbidden
  type: http
  seq: 6
}

get {
  url: {{baseUrl}}/api/v1/users
  body: none
  auth: bearer
}

auth:bearer {
  token: {{memberAccessToken}}
}

assert {
  res.status: eq 403
  res.body.error.code: eq FORBIDDEN
}

docs {
  403, not 404. The member is authenticated and the collection exists; what
  is missing is permission. Hiding it behind 404 would be lying to a client
  that has every right to be told it lacks access.

  Contrast with users-no-token.bru, which is 401: no identity at all.
}
```

- [ ] **Step 7: 停用成员**

`api/bruno/errors/setup-disable-member.bru`:

```
meta {
  name: setup disable member
  type: http
  seq: 7
}

patch {
  url: {{baseUrl}}/api/v1/users/{{memberId}}
  body: json
  auth: inherit
}

body:json {
  {
    "status": "disabled"
  }
}

assert {
  res.status: eq 200
  res.body.status: eq disabled
}

docs {
  auth: inherit, so this runs as the administrator. login-as-member stored
  its token under a different name precisely so this still works.
}
```

- [ ] **Step 8: 禁用账号登录 —— 三种失败中的第三种**

`api/bruno/errors/login-disabled-account.bru`:

```
meta {
  name: login disabled account
  type: http
  seq: 8
}

post {
  url: {{baseUrl}}/api/v1/auth/login
  body: json
  auth: none
}

body:json {
  {
    "email": "{{memberEmail}}",
    "password": "{{memberPassword}}"
  }
}

assert {
  res.status: eq 401
  res.body.error.code: eq INVALID_CREDENTIALS
  res.body.error.message: eq invalid email or password
}

docs {
  Third of the three identical failures, and the one worth reading closely.

  The credentials here are correct. Five requests ago they returned 200. The
  only change is status=disabled, and the response must still be
  indistinguishable from a wrong password.

  This request only means anything while the account exists. If setup-member
  or setup-disable-member did not run, the 401 arrives via the unknown-email
  branch and the assertion passes while proving nothing.
}
```

- [ ] **Step 9: 401 的两种**

`api/bruno/errors/users-no-token.bru`:

```
meta {
  name: users no token
  type: http
  seq: 9
}

get {
  url: {{baseUrl}}/api/v1/users
  body: none
  auth: none
}

assert {
  res.status: eq 401
  res.body.error.code: eq UNAUTHENTICATED
  res.body.error.message: eq authentication required
}
```

`api/bruno/errors/users-tampered-token.bru`:

```
meta {
  name: users tampered token
  type: http
  seq: 10
}

get {
  url: {{baseUrl}}/api/v1/users
  body: none
  auth: bearer
}

auth:bearer {
  token: {{accessToken}}x
}

assert {
  res.status: eq 401
  res.body.error.code: eq INVALID_TOKEN
  res.body.error.message: eq invalid or expired token
}

docs {
  One character appended to a valid token breaks the signature.

  INVALID_TOKEN and UNAUTHENTICATED are different codes because they call
  for different client behaviour: refresh and retry versus log in. Neither
  says which part of the token was wrong, because that would help someone
  iterate toward a valid one.
}
```

- [ ] **Step 10: 404**

`api/bruno/errors/users-not-found.bru`:

```
meta {
  name: users not found
  type: http
  seq: 11
}

get {
  url: {{baseUrl}}/api/v1/users/01a05c5c-0000-7000-8000-000000000000
  body: none
  auth: inherit
}

assert {
  res.status: eq 404
  res.body.error.code: eq USER_NOT_FOUND
}

docs {
  A well-formed UUID that resolves to nothing gives 404. A malformed one
  gives 400 with INVALID_USER_ID, because the request was never answerable
  and saying "not found" would suggest the client should go looking.
}
```

- [ ] **Step 11: 清理**

`api/bruno/errors/cleanup-member.bru`:

```
meta {
  name: cleanup member
  type: http
  seq: 12
}

delete {
  url: {{baseUrl}}/api/v1/users/{{memberId}}
  body: none
  auth: inherit
}

assert {
  res.status: eq 204
}

docs {
  Without this every full run leaves a disabled account behind, and the user
  list slowly fills with debris that makes the pagination examples harder to
  read.
}
```

- [ ] **Step 12: 一致性测试**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && go test ./api/bruno/ -count=1 -v
```

预期:`PASS`,24 个请求全部匹配。注意 `users-not-found.bru` 里那个字面 UUID 必须被归一为 `{id}`——如果测试报它未被文档记载,说明 `specPath` 的 UUID 分支没生效。

- [ ] **Step 13: 实跑整个集合**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' | xargs -r docker exec -i goscaffold-redis redis-cli del
cd api/bruno && npx --yes @usebruno/cli@latest run . --env local -r
```

预期:24 requests / 24 passed。执行顺序必须是 health → auth → users → errors。

若 `login-disabled-account` 报 200,说明 `setup-disable-member` 没跑或跑晚了,检查 seq。

- [ ] **Step 14: 提交**

```bash
cd /Users/withlevi/Desktop/Project/Go_
git add api/bruno/errors
git commit -m "feat(bruno): add the failure paths, with their own member user

The three login failures must be byte-identical: wrong password, unknown
address, disabled account. Any difference between them is an account
enumeration oracle. Putting the three side by side is most of the value of
this folder.

The disabled case needs a live account, so errors/ owns one from creation
to deletion rather than borrowing the subject from users/, which gets
deleted first. Borrowing it would have left the assertion green while the
401 actually came from the unknown-address branch.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 5: 变异验证一致性测试

测试写完了不代表它有用。这一步故意破坏,确认它真的会红。

**Files:**
- Modify (临时): `api/bruno/health/health.bru`
- Modify (临时): `api/bruno/errors/users-not-found.bru`
- Modify (临时): `api/bruno/collection_test.go`

- [ ] **Step 1: 变异 M1 —— 指向不存在的接口**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && cp api/bruno/health/health.bru /tmp/h.bak
python3 - <<'EOF'
p='api/bruno/health/health.bru'
s=open(p).read()
old='url: {{baseUrl}}/health'
assert old in s
open(p,'w').write(s.replace(old,'url: {{baseUrl}}/healthz'))
EOF
go test ./api/bruno/ -count=1 2>&1 | tail -5
cp /tmp/h.bak api/bruno/health/health.bru
```

预期:`FAIL`,信息里点名 `health/health.bru targets "GET /healthz"`。
若通过,说明测试没在读这个文件——先修再往下走。

- [ ] **Step 2: 变异 M2 —— UUID 归一化失效**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && cp api/bruno/collection_test.go /tmp/ct.bak
python3 - <<'EOF'
p='api/bruno/collection_test.go'
s=open(p).read()
old='\t\tcase uuidRe.MatchString(s):\n\t\t\tsegments[i] = "{id}"\n'
assert old in s
open(p,'w').write(s.replace(old,''))
EOF
go test ./api/bruno/ -count=1 2>&1 | tail -5
cp /tmp/ct.bak api/bruno/collection_test.go
```

预期:`FAIL`,点名 `users-not-found.bru`。
这个变异证明字面 UUID 的那条分支确实被 `errors/users-not-found.bru` 覆盖着——
否则那段代码是死的,将来谁删掉它都不会有人发现。

- [ ] **Step 3: 变异 M3 —— 空集合静默通过**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && cp api/bruno/collection_test.go /tmp/ct.bak
python3 - <<'EOF'
p='api/bruno/collection_test.go'
s=open(p).read()
old='if d.IsDir() || filepath.Ext(path) != ".bru" {'
assert old in s
open(p,'w').write(s.replace(old,'if d.IsDir() || filepath.Ext(path) != ".brunope" {'))
EOF
go test ./api/bruno/ -count=1 2>&1 | tail -5
cp /tmp/ct.bak api/bruno/collection_test.go && rm -f /tmp/h.bak /tmp/ct.bak
```

预期:`FAIL`,信息是 `no .bru requests found; the test would have passed vacuously`。
这条守卫存在的意义就在这里:集合被挪走之后,测试必须报错而不是继续绿。

- [ ] **Step 4: 确认全部还原**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && git diff --stat api/bruno && go test ./api/bruno/ -count=1
```

预期:`git diff` 无输出,测试 `ok`。

---

## Task 6: 文档与收尾

**Files:**
- Create: `api/bruno/README.md`
- Modify: `STATE.md`(在 Phase 5 章节之后追加一节)
- Modify: `docs/architecture.md`(§4 之外,补一条指向集合的说明)

- [ ] **Step 1: 写 README**

创建 `api/bruno/README.md`:

```markdown
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

## 已知限制

- **一次完整运行发起 5 次登录。** 登录的 IP 限流是 10 次 /15 分钟,
  连着跑两遍就打满,第三遍开始 429。开发时可以清掉:

  ```bash
  docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' \
    | xargs -r docker exec -i goscaffold-redis redis-cli del
  ```

- **一致性检查只覆盖路径。** `collection_test.go` 断言每个请求的
  `(method, path)` 存在于 `api/openapi.yaml`,但不检查请求体字段和响应
  schema。后者是 `internal/openapitest` 的职责,它比对的是实现与文档。

- **不进 CI。** 需要跑着的服务和真实凭据。
```

- [ ] **Step 2: 更新 STATE.md**

在 `STATE.md` 末尾追加:

```markdown

---

# 调试工具:Bruno 集合(2026-09-02)

`api/bruno/` 是一份 git 跟踪的 Bruno 集合,24 个请求,覆盖
`api/openapi.yaml` 的全部 12 个 operation 加 11 条失败路径。

**它解决的是一个具体的浪费**:调认证后面的接口时,每次都要先登录、把
access token 从响应里复制出来、粘进下一个请求。token 15 分钟过期,这个动作
一天要重复几十次。集合里 `auth/login` 的 post-response 脚本把 token 写进
变量,`collection.bru` 声明集合级 Bearer auth,其余请求写 `auth: inherit`。

**它不是测试套件。** 门禁仍然是 `test/blackbox`(110 条)与
`make test-integration`。集合里的断言只用来区分"环境坏了"和"你要调的东西
坏了"。不要往里加回归用例。

**`collection_test.go`** 断言每个请求的 `(method, path)` 存在于
`openapi.yaml`,单向。理由与 `deployments/observability/dashboard_test.go`
相同:指向已删除接口的请求返回 404,和服务挂掉长得一模一样,人会先去查
服务。经 3 个变异验证。

**凭据不进仓库**:管理员密码走 `{{process.env.BRUNO_ADMIN_PASSWORD}}`,
根 `.gitignore` 的 `.env` 规则(不带斜杠,任意层级生效)覆盖
`api/bruno/.env`。

**顺序由 `folder.bru` 的 `seq` 控制**:health → auth → users → errors。
实测发现 `bru run -r` 会先跑子文件夹再跑同级请求,所以 `errors/` 是顶层
文件夹而不是嵌在 `auth/` 与 `users/` 下面。

**`errors/` 自带一个成员用户的生命周期**(setup-member → login-as-member →
403 → setup-disable-member → 禁用登录 401 → cleanup-member),不复用
`users/` 那个主体。复用会被 `users/delete` 先删掉,`login-disabled-account`
于是走"邮箱不存在"分支——断言全绿,测的却是另一件事。

设计文档:`docs/superpowers/specs/2026-09-02-bruno-collection-design.md`
```

- [ ] **Step 3: 在 architecture.md 里指路**

在 `docs/architecture.md` 的 ADR 索引表**之后**、第一个 `##` 小节**之前**插入:

```markdown
> 手工调试用 `api/bruno/`(Bruno 集合,登录一次后 token 自动流转)。
> 自动化回归看 `test/blackbox/` 与 `make test-integration`。
```

- [ ] **Step 4: 全量验证**

```bash
cd /Users/withlevi/Desktop/Project/Go_ && export PATH="$PATH:$(go env GOPATH)/bin" && make check 2>&1 | grep -E "FAIL|issues|vulnerabilit"
```

预期:`0 issues`、`No vulnerabilities found`、无 `FAIL`。

若 `golangci-lint` 对 `api/bruno/collection_test.go` 报 revive 的
package-comments,检查包注释是否紧贴 `package bruno` 子句(中间不能有空行)。

- [ ] **Step 5: 提交**

```bash
cd /Users/withlevi/Desktop/Project/Go_
git add -A api/bruno STATE.md docs/architecture.md
git commit -m "docs(bruno): document the collection and how it differs from the suites

The README says plainly what this is not: the gate is still the black-box
suite. Assertions here answer one question, whether the environment broke
or the thing being debugged broke, and treating it as a regression suite
would put a running service and a real credential on the critical path.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Self-Review 结果

对照 spec 逐节检查:

| Spec 要求 | 覆盖它的 Task |
|---|---|
| 位置 `api/bruno/` | Task 1 Step 3 |
| 结构:四个顶层文件夹 + folder.bru seq | Task 1/2/3/4 各自 Step 1 |
| 变量流转 login → accessToken | Task 2 Step 2 |
| refresh 覆盖两个变量 | Task 2 Step 3 |
| create 写入 userId | Task 3 Step 2 |
| 凭据走 process.env,`.env` 不进仓库 | Task 1 Step 3 |
| 每个请求带断言,含失败路径 | Task 1–4 全部 |
| 失败路径断言 `error.code` | Task 4 全部 |
| 三种登录失败逐字相同 | Task 4 Step 2 与 Step 8 |
| errors/ 自带成员用户生命周期 | Task 4 Step 4–7、11 |
| `collection_test.go` 单向一致性 | Task 1 Step 1 |
| 变异验证 | Task 5 |
| 已知限制写进文档 | Task 6 Step 1 |

无 TBD、无"参照 Task N"、无未定义的函数或类型。
`specPath`、`collectionRequests`、`documentedOperations`、`request`、
`verbs`、`blockRe`、`uuidRe`、`varRe` 均在 Task 1 Step 1 一次性定义齐全,
Task 5 的变异脚本引用的字符串与之逐字一致。
