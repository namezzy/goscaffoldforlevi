# Multi-stage build producing two images from one file: `--target api` and
# `--target worker`. They share every layer up to the final copy, so building
# both costs one compilation of the dependency graph rather than two.
#
# Base image: gcr.io/distroless/static:nonroot (ADR/Phase 6 decision D1).
# Nothing here needs libc — pgx and the Argon2 implementation are pure Go — so
# CGO_ENABLED=0 produces a static binary and alpine's musl compatibility
# problems never arise. distroless brings CA certificates, /etc/passwd and
# tzdata, which scratch would require reassembling by hand.
#
# The cost, stated here because it will be discovered at the worst possible
# moment otherwise: there is no shell in the final image, so `kubectl exec`
# does not work. Use `kubectl debug -it <pod> --image=busybox --target=api`,
# which attaches an ephemeral container sharing the process namespace.

# ----------------------------------------------------------------- build stage

FROM golang:1.27.1-bookworm AS build

WORKDIR /src

# Dependencies are resolved in their own layer so that editing application code
# does not re-download the module graph.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Injected by the build system; see the LDFLAGS variable in the Makefile. The
# defaults keep a bare `docker build` honest rather than empty: a blank version
# in a health response reads as a broken report, not as an unstamped build.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# -trimpath strips local filesystem paths from the binary, which are both noise
# and a small information leak. The -X paths must match internal/buildinfo; the
# linker ignores an -X flag it cannot resolve, without a warning.
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    LDFLAGS="-s -w \
      -X github.com/namezzy/goscaffoldforlevi/internal/buildinfo.Version=${VERSION} \
      -X github.com/namezzy/goscaffoldforlevi/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/namezzy/goscaffoldforlevi/internal/buildinfo.BuildTime=${BUILD_TIME}" && \
    go build -trimpath -ldflags "${LDFLAGS}" -o /out/api    ./cmd/api && \
    go build -trimpath -ldflags "${LDFLAGS}" -o /out/worker ./cmd/worker

# ------------------------------------------------------------------ api target

FROM gcr.io/distroless/static:nonroot AS api

COPY --from=build /out/api /api

# 8080 is the API. 9464 is the metrics listener, which is documented here but
# deliberately not published by compose and not put behind a Service: it is for
# a scraper reaching the pod directly, not for anyone outside the cluster.
EXPOSE 8080 9464

# nonroot is uid 65532 in the distroless images. Declaring it here means the
# image is safe by default even where a Kubernetes securityContext is missing.
USER nonroot:nonroot

ENTRYPOINT ["/api"]

# --------------------------------------------------------------- worker target

FROM gcr.io/distroless/static:nonroot AS worker

COPY --from=build /out/worker /worker

# The worker serves no HTTP API. It exposes only metrics, on its own port so
# that an api and a worker scheduled onto the same host do not collide.
EXPOSE 9465

USER nonroot:nonroot

ENTRYPOINT ["/worker"]

# -------------------------------------------------------------- migrate target

# The migration Job's image: the upstream golang-migrate CLI with this
# repository's SQL baked in.
#
# The alternative was a ConfigMap generated from migrations/, and it was
# rejected for one reason: the ConfigMap and the directory can disagree.
# Baking the files in makes the image immutable and versioned with the code, so
# `sha-<commit>` names exactly the schema that commit expects. It also keeps
# kustomize inside its default load restrictions, which a configMapGenerator
# reaching up out of the base directory would not.
#
# The CLI version must match the golang-migrate version in go.mod, or the Job
# and the local `make migrate-up` can interpret the same directory differently.
FROM migrate/migrate:v4.19.1 AS migrate

COPY migrations /migrations

USER 65532:65532

ENTRYPOINT ["migrate", "-path", "/migrations"]
