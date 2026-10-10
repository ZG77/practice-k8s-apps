# practice-k8s-apps

Three tiny Go services for the practice EKS platform, plus their CI/CD.
Deploy config lives in the GitOps repo: [ZG77/practice-k8s-gitops](https://github.com/ZG77/practice-k8s-gitops).

## Services

| Service | Routes | Scaling practice |
|---------|--------|------------------|
| `web` | `/`, `/cpu?ms=`, `/mem?mb=&sec=` | HPA on CPU |
| `api` | `/api/hello`, `/api/cpu?ms=`, `/api/mem?mb=&sec=` | KEDA on requests per second |
| `worker` | background loop, `/cpu`, `/mem` | VPA |

All services also have `/healthz`, `/readyz`, `/metrics`, `/version`, and listen on port `8080`.

Metrics: `http_requests_total{route,code}`, `http_request_duration_seconds`, `app_build_info{service,version}`, `app_memory_held_bytes`, `worker_jobs_total`.
Probes and `/metrics` are not counted in `http_requests_total`.

## Environment variables

| Variable | Service | Default |
|----------|---------|---------|
| `PORT` | all | `8080` |
| `SHUTDOWN_DELAY` | all | `10s` (time between readiness failing and server stop) |
| `API_URL` | web | `http://api.backend.svc.cluster.local` |
| `WORK_INTERVAL`, `WORK_MS`, `BASE_MEMORY_MB` | worker | `1s`, `100`, `64` |

## Layout

```
cmd/<service>/main.go   one binary per service
internal/server         HTTP server, health, metrics, graceful shutdown
internal/load           CPU and memory load
Dockerfile              one file for all: --build-arg SERVICE=web
```

## CI/CD flow

```
PR      -> test -> build (amd64 + arm64, no push)
main    -> test -> build changed services -> push to ECR (tag = git SHA)
             -> commit new tag to practice-k8s-gitops/apps/<service>/values.yaml
             -> Argo CD syncs
```

- AWS auth: GitHub OIDC. No AWS keys in GitHub. Only `main` of this repo can assume the role.
- GitOps write: deploy key on the GitOps repo (`GITOPS_DEPLOY_KEY` secret).
- Rollback: `git revert` the deploy commit in the GitOps repo.

## Local

```bash
go test ./...
PORT=8081 go run ./cmd/api &
PORT=8080 API_URL=http://localhost:8081 go run ./cmd/web
curl localhost:8080/
```
