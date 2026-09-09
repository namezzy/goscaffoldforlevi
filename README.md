<h1 align="center">goscaffoldforlevi</h1>

<p align="center">
  A production-minded Go service scaffold — where every convention is enforced by the build, not by documentation.
</p>

<p align="center">
  <a href="https://github.com/namezzy/goscaffoldforlevi/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/namezzy/goscaffoldforlevi/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27%2B-00ADD8?logo=go&logoColor=white">
  <img alt="Tests" src="https://img.shields.io/badge/tests-21%20packages%20green-3fb950">
  <img alt="Test ratio" src="https://img.shields.io/badge/test%20code-53%25-3fb950">
  <img alt="ADRs" src="https://img.shields.io/badge/ADRs-10-8957e5">
  <img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue">
</p>

<p align="center">
  <strong>English</strong> · <a href="README.zh-CN.md">简体中文</a>
</p>

---

## What this is

A Go HTTP service skeleton with users, authentication, RBAC, rate limiting, an
outbox-backed message pipeline, and full OpenTelemetry wiring — built so that a
real project can start from it on day one instead of spending its first month
re-litigating the same ten decisions.

It is deliberately **not** a framework. There is no container, no registry, no
code generation beyond `sqlc`, and no base classes. Dependencies are wired by
hand in `cmd/api/run.go`, where you can read them.

## Why another scaffold

Most scaffolds hand you a directory layout and hope. Two things here are
different.

**Every decision has a written rationale.** Ten ADRs in [`docs/adr/`](docs/adr/)
record what was chosen, what was rejected, and what it costs. Each one carries a
`When to Revisit` section stated as an *observable trigger* — not "when
requirements change", which is a review condition that can never fire.

**Conventions that matter are mechanically enforced.** A rule that lives only in
a document gets violated; a rule that fails the build does not.

| Convention | Mechanism | What happens when violated |
|---|---|---|
| Dependency direction (ADR-0002) | `depguard` rules in `.golangci.yml` | `make lint` fails |
| Every error `Kind` maps to exactly one HTTP status | `internal/httpx/errmap_test.go` | New `Kind` unregistered → test fails |
| `.env.example` never drifts from the loader | `internal/config/env_example_test.go` | Key mismatch → test fails |
| Handlers and the OpenAPI spec agree, both ways | `cmd/api/openapi_test.go` | Undocumented route or dead spec entry → test fails |
| Response bodies match their declared schema | `internal/openapitest` | Shape drift → test fails |
| Generated SQL code is not stale | `make sqlc-check` (`sqlc diff`) | CI fails |
| Grafana dashboards reference metrics that exist | `deployments/observability/dashboard_test.go` | Renamed metric → test fails |
| The Bruno collection covers every operation | `api/bruno/collection_test.go` | Missing or orphaned request → test fails |

The dependency rule is the load-bearing one: `internal/user/*.go` and
`internal/auth/*.go` may not import `net/http`, `chi`, `pgx`, `go-redis`,
`nats.go`, or `otel`. Not for purity — a service that imports `net/http` cannot
be reused by a CLI or a consumer, and one that imports `pgx` cannot be tested
without a database.

## Project status

Built in phases, each with its own entry conditions and gates. The current state
of record is [`STATE.md`](STATE.md).

| Phase | Scope | Status |
|---|---|---|
| 1 | Architecture — 10 ADRs, `docs/architecture.md`, dependency rule | ✅ Complete |
| 2 | Foundation — config, logging, pool, transactions, HTTP, error model | ✅ Complete |
| 3 | Users, auth, RBAC, rate limiting, OpenAPI | ✅ Complete |
| 4 | Messaging — outbox, NATS JetStream, idempotency, DLQ | ✅ Complete |
| 5 | Observability — traces, metrics, log correlation, Grafana stack | ✅ Complete |
| 6 | Dockerfile, CI, Kubernetes manifests | ✅ Complete |
| 7 | `goscaffold` CLI | ✅ Complete |
| 8 | Documentation and final audit | ◻︎ In progress |

**Read this before deploying anything:** the manifests have been validated, not
run. There is no cluster behind them yet — see
[Verification status](#verification-status) for exactly what was and was not
executed.

## Features

**HTTP** · chi router · no envelope on success, one wrapper on failure ·
`DisallowUnknownFields` on every decode · PATCH-only updates · offset pagination
· `X-Request-ID` on every response · graceful shutdown with a readiness drain
delay so the load balancer stops routing before the listener closes.

**Persistence** · pgx connection pool · `sqlc`-generated queries · migrations via
`golang-migrate`, never applied automatically at pod start ·
`TxManager.WithTx(ctx, fn)` with the transaction hidden behind an unexported
context key, so `pgx.Tx` never appears in a business signature · UUIDv7
application-side IDs.

**Auth** · Argon2id at 64 MiB / t=3 / p=4 · JWT HS256 access tokens (15 min) ·
opaque refresh tokens (30 d) stored only as SHA-256 digests · refresh-token
family theft detection · all three login failure modes return an identical 401
in identical time.

**Authorization** · Two roles, one policy map, five permissions. Handlers ask for
a permission, never for a role.

**Rate limiting** · Redis Lua token bucket, shared across replicas · general
scopes fail *open* with a local fallback limiter, login scopes fail *closed* with
503 · split into two middlewares placed on either side of authentication.

**Messaging** *(off by default)* · Transactional outbox · NATS JetStream ·
at-least-once delivery with a `processed_events` dedupe log · exponential
backoff · database-table DLQ with replay.

**Observability** · OpenTelemetry traces across HTTP, PostgreSQL, Redis and NATS
· Prometheus metrics on a separate listener · `trace_id` injected into every log
line from context · attribute allow-lists on DB and cache spans so no query
argument ever reaches a span.

## Architecture

Three packages per business capability, and the direction of dependency is
one-way:

```
internal/user/            domain types, service, ports        ← no I/O imports
internal/user/userpg/     PostgreSQL adapter                  → imports pgx
internal/user/userhttp/   HTTP adapter                        → imports chi
```

The service package declares the interfaces it needs; the adapters implement
them. Nothing in `internal/user` knows that HTTP or PostgreSQL exist.

Middleware order is fixed, and two positions in it are counter-intuitive on
purpose:

```
Recovery → RequestID → Tracing → Logging → Metrics → SecurityHeaders
  → CORS → BodyLimit → Timeout → RateLimitByIP
  → Authentication → RateLimitByUser → Authorization → Handler
```

- **CORS before rate limiting** — otherwise a browser preflight burns the user's quota.
- **Metrics before rate limiting** — otherwise 429s are never counted, and the 429 rate is exactly the number you need when limits start biting.

Full reasoning, one entry per position, is in
[`docs/architecture.md`](docs/architecture.md).

## Directory structure

```
cmd/
  api/                 HTTP server; main.go decides the exit code, run.go wires
  worker/              outbox relay + event consumer
  adminctl/            creates the first administrator, then refuses to run again
internal/
  apperr/              Kind + Code error model; zero-value Kind = Internal
  config/              env > .env > defaults; Secret type; accumulating Validate()
  validate/            syntactic validation only — no business rules
  httpx/               router, middleware, the single Kind→status table
  postgres/            pool, TxManager, health, tracing
  redis/               client, tracing
  auth/                service · authjwt · authpg · authhttp
  user/                service · userpg · userhttp
  ratelimit/           token bucket · guard · local fallback · HTTP middleware
  messaging/           publisher · outbox relay · consumer · DLQ
  telemetry/           the only package that imports the OTel SDK or Prometheus
  notify/              notification sender (log-backed; see the warning below)
api/
  openapi.yaml         the contract; enforced against the router in both directions
  bruno/               24-request debugging collection (not a test suite)
migrations/            golang-migrate SQL, up and down
queries/               sqlc source queries
cmd/goscaffold/        the CLI: new, module, migration
internal/scaffold/     generators, and the templates they embed
Dockerfile             three targets: api, worker, migrate
.github/workflows/     CI gate, image publishing, generated-project check
deployments/
  observability/       OTel Collector, Prometheus, Tempo, Grafana + dashboard guard
  k8s/                 Kustomize base + four overlays, with a guard test
test/blackbox/         adversarial HTTP tests run against a live service
docs/                  architecture.md · adr/ · progress.md
prompts/               the phased build contract this repository was produced under
```

## Quick start

Requires Go 1.27+ and Docker. Verified end to end on macOS with OrbStack.

```sh
git clone https://github.com/namezzy/goscaffoldforlevi.git
cd goscaffoldforlevi

cp .env.example .env      # every value is a working default; edit nothing to start
make tools                # pins golangci-lint, sqlc, migrate, govulncheck
docker compose up -d      # postgres + redis
make migrate-up
make run
```

Then:

```console
$ curl -s localhost:8080/live
{"status":"ok","version":"dev"}

$ curl -s localhost:8080/ready
{"status":"ok","version":"dev","components":{"postgres":"ok","redis":"ok"}}
```

`/live` answers as long as the process is alive; `/ready` reports each dependency
by name, returns 503 when one is down, and reports `draining` during graceful
shutdown so the load balancer can withdraw the instance before the listener
closes.

Create the first administrator — the API cannot bootstrap itself, because
`POST /users` requires the `user:create` permission:

```sh
make bootstrap-admin email=you@example.com
```

It prints a generated password once and refuses to run again once an
administrator exists. A command that mints privileged accounts outside the API is
a standing bypass of both authorization and the audit trail, so it stops being
available the moment it is no longer needed.

Then log in:

```sh
curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"<the printed password>"}'
```

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "…opaque…",
  "token_type": "Bearer",
  "expires_in": 900
}
```

For interactive exploration, [`api/bruno/`](api/bruno/README.md) is a Bruno
collection whose login script captures the token into a collection variable, so
you stop copying it by hand every fifteen minutes.

## Configuration

Precedence is **real environment variables → `.env` → built-in defaults**. Every
key, its default, and the reasoning behind that default is documented inline in
[`.env.example`](.env.example), which a test keeps in sync with the loader.

| Group | Prefix | Notes |
|---|---|---|
| Application | `APP_*`, `LOG_LEVEL` | `APP_ENV` must be `development` / `staging` / `production` |
| HTTP | `HTTP_*` | `HTTP_WRITE_TIMEOUT` must exceed `HTTP_REQUEST_TIMEOUT`, or clients get a reset instead of a 504 |
| Database | `DATABASE_*` | pool sizes, lifetimes, connect timeout |
| Redis | `REDIS_*` | `REDIS_REQUIRED=false` keeps a Redis outage from removing the instance from the load balancer |
| Auth | `AUTH_*` | startup **fails** if a known example secret is still set while `APP_ENV=production` |
| Rate limiting | `RATELIMIT_*` | quota / period / burst per scope |
| Messaging | `MESSAGING_*` | off by default; `MESSAGING_ACK_WAIT` must exceed the whole retry budget, and startup refuses if it does not |
| Observability | `OTEL_*`, `METRICS_*` | instrumentation is always compiled in; these only control export |

Configuration is validated once at startup and reports **every** problem at once
rather than failing on the first — fixing a misconfiguration one restart at a
time is a bad way to spend an afternoon.

## API

Base path `/api/v1`. Full contract in [`api/openapi.yaml`](api/openapi.yaml).

| Method | Path | Permission |
|---|---|---|
| `POST` | `/auth/login` | public |
| `POST` | `/auth/refresh` | refresh token |
| `POST` | `/auth/logout` | authenticated |
| `POST` | `/users` | `user:create` |
| `GET` | `/users` | `user:list` |
| `GET` | `/users/{id}` | `user:read`, or self |
| `PATCH` | `/users/{id}` | `user:update`, or self |
| `DELETE` | `/users/{id}` | `user:delete` |
| `PATCH` | `/users/{id}/password` | `user:update`, or self |
| `GET` | `/health` `/live` `/ready` | public, unversioned |

Conventions: success responses carry the resource directly — the status code is
the envelope. Failures always use one wrapper:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "the request body is not valid",
    "details": [{ "field": "email", "reason": "must be a valid address" }]
  },
  "request_id": "01930f8e-0000-7000-8000-000000000001"
}
```

Clients branch on `code`, which is stable; `message` is prose and may change.
`request_id` is the only link between what a user saw and what an operator can
look up — internal errors return nothing else.

Updates are `PATCH` only. There is no `PUT`, because full replacement makes every
omitted field a silent deletion.

## Security

- **Passwords** — Argon2id, 64 MiB / t=3 / p=4 (OWASP). Raise memory first when hardware allows; it is the parameter a GPU attacker cannot buy their way around. Every login allocates that much, which is why the login rate limit is not optional.
- **Login** — unknown address, wrong password, and disabled account return the same 401, with the same code, in the same time. Any difference makes the endpoint an account enumeration oracle.
- **Refresh tokens** — opaque, high-entropy, stored only as SHA-256 digests, rotated on every use. Presenting a token that has already been rotated revokes the entire family; the revocation is committed *before* the rejection is returned.
- **Rate limits** — login limits are enforced per client address *and* per email hash. Both must pass: one stops a single source enumerating many accounts, the other stops many sources targeting one account. `X-RateLimit-*` headers are stripped from login responses; a remaining-attempts counter next to a refused login is a progress meter for an attacker.
- **Trusted proxies** — `HTTP_TRUSTED_PROXY_COUNT` defaults to `0`, meaning `X-Forwarded-For` is ignored entirely. It is a client-controlled header; set the real hop count, never higher.
- **Secrets in telemetry** — DB and cache spans use attribute allow-lists, so no query argument or cache value can reach a span. An automated audit test asserts it.
- **Production guards** — startup refuses a known example JWT secret, a wildcard CORS origin, or a metrics listener bound to a public address when `APP_ENV=production`.

Full treatment: [ADR-0007](docs/adr/0007-auth.md) and
[ADR-0008](docs/adr/0008-rate-limiting.md).

## Messaging

Disabled by default, and that default is the point: with `MESSAGING_ENABLED=false`
the service writes no outbox rows, needs no broker, and needs no worker. A project
that never becomes asynchronous pays nothing for this being here.

When enabled, an event is written to `outbox_events` **inside the same
transaction** as the business change, so the two cannot disagree.
`cmd/worker` relays pending rows to NATS JetStream and consumes them:

```sh
make broker-up            # NATS JetStream, behind the `messaging` compose profile
make worker               # relay + consumer
make outbox-status        # pending / published / failing counts
make dlq-list             # dead letters awaiting triage
make dlq-replay id=<id>   # put one back in the outbox
```

Delivery is at-least-once. Consumers are made idempotent by a `processed_events`
dedupe log, which is retained far longer than the stream itself — prune it sooner
and a redelivered message would be processed twice.

> [!WARNING]
> `notify.LogSender` writes "sent" mail to the log. The entire pipeline — outbox,
> delivery, idempotency, retry, DLQ — is real and tested, but there is no mail
> provider behind it. Do not deploy this believing the service can send email.

## Observability

Instrumentation is always compiled in; `OTEL_ENABLED` and `METRICS_ENABLED` only
control whether data leaves the process. With tracing off, behaviour is
unchanged — verified, not assumed.

```sh
docker compose --profile observability up -d
OTEL_ENABLED=true make run
```

Grafana on <http://localhost:3000> (anonymous, local-only), Prometheus on `:9090`,
Tempo on `:3200`. A single trace spans HTTP → service → PostgreSQL → Redis →
NATS → worker, and every log line inside it carries the same `trace_id`.

Metrics are served on a **separate listener** (`127.0.0.1:9464` by default), not
on the API port: metric names enumerate every route template, dependency and
build version, and the API port is the one meant to be reachable from outside.

`internal/telemetry` is the only package permitted to import the OTel SDK or the
Prometheus client. Business packages declare small `Observer` interfaces and stay
ignorant of both — enforced by `depguard`, and verified with `go list`.

## Testing

```sh
make test               # unit tests — no Docker required
make test-race          # with the race detector
make test-integration   # testcontainers; needs Docker
make check              # fmt · vet · lint · test -race · vuln · sqlc-check
```

`make check` is the gate, and it is the same set of commands CI runs. A command
that exists only in a CI configuration is a command developers cannot reproduce
locally, which is how "works on my machine" starts.

Beyond the Go tests, [`test/blackbox/`](test/blackbox/README.md) holds 110
adversarial checks that run against a **live** service over HTTP. They exist
because the Go tests share the service's own assumptions: a fake repository that
happily accepts a negative offset cannot reveal that the offset overflowed. That
suite found five real defects, all recorded in `STATE.md`.

Coverage is not a target. Behaviour that nothing asserts is the target — several
mutations survived a fully green suite during Phase 5 and are documented as such.

## Development workflow

```sh
make help                          # every target, with a description
make run | build | clean
make fmt vet lint vuln
make sqlc | sqlc-check             # regenerate / verify generated query code
make migrate-create name=add_orders
make migrate-up | migrate-down | migrate-status
make openapi-lint                  # requires npx; deliberately outside `make check`
```

Adding an endpoint touches four places, and three of them are enforced: the
handler, `api/openapi.yaml` (`cmd/api/openapi_test.go` fails otherwise), a Bruno
request (`collection_test.go` fails otherwise), and — if it returns a new shape —
a response schema check.

Database errors are translated to `apperr` **only in `*pg` packages**. If a
service layer seems to need a driver error code, a translation is missing one
layer down; fix the repository, not the service.

## The goscaffold CLI

```sh
make scaffold      # installs it into GOPATH/bin

goscaffold new billing --module github.com/acme/billing
cd billing
goscaffold module invoice
goscaffold migration create_invoices
```

| Command | What it writes |
|---|---|
| `new <name> --module <path>` | The whole scaffold, with the module path and service name substituted throughout |
| `module <name>` | Domain, service and ports, HTTP adapter, PostgreSQL stub, tests — the three-package shape from ADR-0002 |
| `migration <name>` | An up/down pair with a collision-free version |
| `version` | The build stamp |

`--dry-run` lists what would be written and writes nothing. `--output` and
`--dir` choose where.

### Nothing is ever overwritten

Every command builds the complete set of files first, checks all of them
against the disk, and only then writes — with `O_EXCL`, so the file system, not
a prior check, has the final say. If anything is in the way, it names every
conflict and writes nothing at all. A generator that writes as it goes and
stops halfway leaves a directory that is neither the old state nor the new one,
which is the one outcome you cannot recover from by re-running it.

There is no `--force`. The seam for adding one is a single conditional, but the
command is usually pointed at a directory someone has already put work into.

### How template drift is prevented

This is the hard problem in any scaffold CLI, and the answer here is that
**there is only one copy of the code**.

`internal/scaffold/templates/project/` is not maintained by hand. It is a
derived copy of this repository, produced by `make templates` and verified by
`TestProjectTemplatesMatchTheRepository`. Edit `internal/httpx/router.go`
without regenerating, and `make check` fails, naming the file. It is the same
mechanism as `make sqlc-check`, pointed at a different generated artefact —
mutation-verified three ways: an edited source file, a missing template, and a
template whose source was deleted.

The module templates are the exception, and they cannot be derived from
anything: they describe a module that does not exist yet. What keeps them
honest is the `Scaffold` workflow, which generates a project, generates a
module inside it, and compiles and tests the result. If a change to `apperr` or
`httpx` breaks the shape the template assumes, that job goes red.

Generated Go is run through `go/format` before it is written, so a template
laid out around `__TYPE__` does not produce misaligned code around `Order` —
and a template that is not valid Go fails at generation time rather than at
`go build`.

### What it deliberately does not do

- **No `--with-redis` / `--with-observability` flags.** Redis is a hard dependency of rate limiting (ADR-0008), and `internal/telemetry` is threaded through every constructor (ADR-0010); a flag that removed either would emit a project that does not compile.
- **No `--with-messaging` flag.** ADR-0009 already made messaging default-off at runtime: with `MESSAGING_ENABLED=false` there is no outbox row, no broker and no worker. A flag would only delete files that already cost nothing.
- **No config file, no plugin system.** Four commands and three flags do not need `.goscaffold.yaml`, and a config file that changed what `new` produces would reintroduce the drift the templates were designed to remove. The extension point for a different template source is `readTemplateTree`; it is one function.
- **It never edits existing files**, so wiring a new module into `cmd/api/features.go`, registering its permissions in `internal/auth/policy.go`, adding it to the depguard rule and documenting it in `api/openapi.yaml` remain yours. The command prints the list when it finishes.

Generated endpoints answer **403 until you register their permissions**. The
policy map is the only source of permissions and an unregistered one is never
granted — so a generated endpoint is unreachable before anyone has decided who
may reach it.

## Deployment

Full procedure, rollback and the pitfalls: **[docs/deployment.md](docs/deployment.md)**.

One Dockerfile, three targets — `api` (40.8 MB) and `worker` (35.1 MB) on
`gcr.io/distroless/static:nonroot`, plus `migrate`, which is the golang-migrate
CLI with this repository's SQL baked in so an image tag names exactly the schema
that commit expects.

```sh
make docker-build
kubectl apply -k deployments/k8s/overlays/production
kubectl wait --for=condition=complete job/goscaffold-migrate --timeout=5m
kubectl rollout status deploy/goscaffold-api
```

Kustomize, one base and four overlays. Migration runs as a Job, never as an
initContainer: an initContainer executes once per pod, so a three-replica
rollout would run the same DDL three times concurrently, and again on every
restart forever.

Four settings in those manifests are load-bearing, and each has a guard test in
`deployments/k8s/manifests_test.go` — because none of them fail in a way that
points at itself:

| Setting | Why it matters |
|---|---|
| `terminationGracePeriodSeconds: 45` | The app's budget is 30 s. "Just over 30" fails under load: SIGTERM delivery is not instant, and a thin margin becomes SIGKILL — intermittent 502s during deploys, only in production. |
| liveness → `/live` | It never touches the database. If it did, a few seconds of failover would fail every replica at once, kill them together, and have them cold-start into the recovering database. |
| `METRICS_ADDR: ":9464"` | The application default binds loopback, which inside a pod is reachable by nothing, including Prometheus. It fails by producing no data rather than an error. |
| `runAsNonRoot`, `readOnlyRootFilesystem`, `drop: [ALL]`, `seccompProfile` | Losing one costs nothing observable. The workload keeps running with more privilege than it needs. |

> [!IMPORTANT]
> There is no shell in the final image, so `kubectl exec` does not work. Use
> `kubectl debug -it <pod> --image=busybox --target=api`.

### CI/CD

`ci.yml` runs on every pull request in three jobs, split by duration so the
first red X arrives in about a minute: `quality` (gofmt, vet, golangci-lint,
`sqlc diff`, actionlint, OpenAPI lint), `test` (build, `go test -race`,
govulncheck), and `integration` (testcontainers). `scaffold.yml` generates a
project, generates a module inside it, and compiles and tests the result.
`docker.yml` publishes to `ghcr.io`.

Every command also exists as a Makefile target — a check that only runs in CI is
a check nobody can reproduce before pushing.

### Verification status

Recorded honestly, because "it deploys" is a claim worth being precise about.

| Check | Result |
|---|---|
| `make check` | ✅ passes |
| `make test-integration` | ✅ passes (PostgreSQL 17, testcontainers) |
| `docker build` (all three targets) | ✅ built |
| Image run against real Postgres and Redis | ✅ `/ready` 200, correct commit reported |
| `docker compose --profile full up -d` | ✅ 9 services, `/live` 200 |
| Graceful shutdown (SIGTERM → drain → exit) | ✅ `/ready` reports `draining` for 5 s, exit code 0 |
| `kubectl kustomize` on all four overlays | ✅ renders |
| Manifests against Kubernetes 1.33 schemas | ✅ `kubeconform -strict`, 0 invalid |
| Generated project: build, vet, test, gofmt | ✅ 24 packages pass, 0 unformatted |
| `actionlint` on both workflows | ✅ clean (with shellcheck, as CI runs it) |
| Guard test mutation-verified | ✅ 5 mutations, all caught |
| Both workflows on a GitHub runner | ✅ green — CI and Docker, third attempt |
| **Deployed to a real cluster** | ⛔ **not done — no cluster available** |

`kubectl apply --dry-run=client` could not be used: it downloads the OpenAPI
schema from a cluster, and there is none. `kubeconform` validates the same
manifests offline against the published 1.33 schemas, in strict mode, which
rejects unknown fields — the class of typo a dry run is usually wanted for.

The workflows needed two fixes before they were green, and both were invisible
locally: `golangci-lint-action@v6` only understands golangci-lint v1 while this
repository declares `version: "2"`, and `actionlint` calls shellcheck on every
`run:` block — which a machine without shellcheck installed silently skips.
`make actionlint` now says so rather than passing quietly.

## Not implemented yet

Stated plainly, because a scaffold that overstates its readiness is worse than
one that under-delivers.

- **Nothing has run in a real cluster.** The manifests render and validate; they have never been applied. See [Verification status](#verification-status).
- **No NetworkPolicy, ServiceMonitor or PrometheusRule.** Alerting and SLOs are outside Phase 6, and a NetworkPolicy without a CNI that enforces it is decoration.
- **No image scanning.** `govulncheck` covers the Go dependencies; a scanner adds little on a distroless image with almost no OS packages.
- **No mail provider.** See the warning above.
- **No self-service registration.** Creating a user requires `user:create`. This is a scope decision, not an omission.
- **Request bodies are not schema-checked against OpenAPI.** Responses are. The authoritative definition of a request lives in the `validate` rules, and reconciling the two needs a mapping layer that does not exist yet.
- **`docker-compose.yml` is for development only** — no resource limits, no secrets management, no restart policy.

Known trade-offs, each argued in its ADR, are tabulated in
[`STATE.md`](STATE.md#known-limitations): the ≤15-minute delay on access-token
revocation, fail-open general rate limiting, implicit transaction propagation
through context, offset pagination under concurrent inserts, and more.

## Documentation

| Document | What it answers |
|---|---|
| [`STATE.md`](STATE.md) | What is true right now — implemented, not implemented, limitations |
| [`docs/architecture.md`](docs/architecture.md) | Why the design is shaped this way |
| [`docs/adr/`](docs/adr/) | Ten decisions, with alternatives and revisit triggers |
| [`docs/progress.md`](docs/progress.md) | How the repository became what it is, in order |
| [`docs/development.md`](docs/development.md) | How to add a module, an endpoint, a migration, a query, a dependency |
| [`docs/deployment.md`](docs/deployment.md) | Images, secrets, migration order, probes, rollback |
| [`docs/security.md`](docs/security.md) | What is defended, what is not, and the known exposures |
| [`docs/final-report.md`](docs/final-report.md) | The Phase 8 audit: every gate, its evidence, and what is still unverified |
| [`api/openapi.yaml`](api/openapi.yaml) | The API contract |
| [`prompts/`](prompts/README.md) | The phased build contract, including the honesty and scope rules |

Where `STATE.md` and any other document disagree, `STATE.md` wins.

The documents under `docs/` are written in Chinese, as they have been since the
first architecture phase; this README is the bilingual entry point.

## License

Apache-2.0, as declared in `api/openapi.yaml`.
