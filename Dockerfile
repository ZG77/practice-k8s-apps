# syntax=docker/dockerfile:1
# One Dockerfile for all services. Pick one with --build-arg SERVICE=web|api|worker.
# Multi-arch without QEMU: the build stage runs on the build machine's CPU and
# cross-compiles for TARGETARCH (amd64 or arm64).

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
ARG SERVICE
ARG VERSION=dev
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY internal/ internal/
COPY cmd/ cmd/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "$SERVICE" || { echo "set --build-arg SERVICE"; exit 1; }; \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/app ./cmd/${SERVICE}

# No shell, no package manager. Runs as user 65532 (nonroot).
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/app /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app"]
