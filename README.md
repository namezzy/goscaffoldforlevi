<h1 align="center">goscaffoldforlevi</h1>

<p align="center">
  A production-minded Go service scaffold — where every convention is enforced by the build, not by documentation.
</p>

<p align="center">
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
| 6 | Dockerfile, CI, Kubernetes manifests | ◻︎ Designed, not implemented |
| 7 | `goscaffold` CLI | ◻︎ Not started |
| 8 | Documentation and final audit | ◻︎ In progress |

**Read this before deploying anything:** the Phase 6 artefacts do not exist yet.
There is no Dockerfile, no CI workflow, and no Kubernetes manifest in this
repository. `docker-compose.yml` provisions local dependencies only and is not a
deployment artefact. See [Not implemented yet](#not-implemented-yet).

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
deployments/
  observability/       OTel Collector, Prometheus, Tempo, Grafana + dashboard guard
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

## Not implemented yet

Stated plainly, because a scaffold that overstates its readiness is worse than
one that under-delivers.

- **No Dockerfile, no CI workflow, no Kubernetes manifests.** Phase 6 is designed and locked (`docs/superpowers/specs/2026-09-04-phase6-infrastructure-design.md` — distroless, Kustomize, build-time version injection) but not built.
- **No `goscaffold` CLI.** Phase 7.
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
| [`api/openapi.yaml`](api/openapi.yaml) | The API contract |
| [`prompts/`](prompts/README.md) | The phased build contract, including the honesty and scope rules |

Where `STATE.md` and any other document disagree, `STATE.md` wins.

## License

Apache-2.0, as declared in `api/openapi.yaml`.
