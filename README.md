# LittleVillage

A web app for families in the US who are expecting or caring for a baby. The
whole family learns baby care together: everyone takes short lessons with a
quiz, sees each other's progress, and shares care tasks. There are no
accounts or passwords; a participant is identified by a session cookie.

**Current state:** project skeleton. The server connects to PostgreSQL,
applies migrations, and serves a health check and the frontend page that
displays it. Product features come next.

## Stack

- Backend: Go (chi, pgx), PostgreSQL. SQL migrations are embedded in the
  binary and applied on startup.
- Frontend: React + TypeScript + Vite.
- One service: the Go server serves the API under `/api` and the built
  frontend for every other path (unknown paths get `index.html`).

## Run locally

Requirements: Docker with Compose. Node 24 only for frontend development.

```sh
docker compose up --build
```

Open http://localhost:8080. The health check is at
http://localhost:8080/api/health:

```json
{"status":"ok","database":"ok"}
```

It returns HTTP 503 with `"database":"unreachable"` when PostgreSQL cannot
be reached.

### Frontend development

Keep `docker compose up` running (it serves the API on port 8080), then:

```sh
cd frontend
npm ci
npm run dev
```

Open http://localhost:5173. Vite proxies `/api` to `http://localhost:8080`.
`npm run build` type-checks and writes the production build to
`frontend/dist`.

### Go commands run in Docker

Go is never run on the host; on the team's Windows machine Smart App Control
blocks `go.exe`. Every Go command runs in the `golang:1.27.4-alpine`
container with `backend/` mounted. This version must match the `go` line in
`backend/go.mod` and the build stage in `Dockerfile`. From the repo root:

```powershell
# PowerShell
docker run --rm -v "${PWD}/backend:/src" -v littlevillage-gomod:/go/pkg/mod -v littlevillage-gobuild:/root/.cache/go-build -w /src golang:1.27.4-alpine go vet ./...
```

```sh
# Git Bash (on macOS/Linux drop MSYS_NO_PATHCONV=1 and use "$PWD")
MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd -W)/backend:/src" -v littlevillage-gomod:/go/pkg/mod -v littlevillage-gobuild:/root/.cache/go-build -w /src golang:1.27.4-alpine go vet ./...
```

Swap `go vet ./...` for `go mod tidy`, `go build ./...`, `go test ./...` or
`gofmt -l .`. The two named volumes cache modules and builds between runs.
Node runs on the host as usual.

## Configuration

The server reads environment variables only (see `.env.example`):

| Variable       | Required | Default  | Meaning                                    |
|----------------|----------|----------|--------------------------------------------|
| `DATABASE_URL` | yes      |          | PostgreSQL connection string               |
| `PORT`         | no       | `8080`   | HTTP port                                  |
| `STATIC_DIR`   | no       | `static` | Folder with the built frontend (`/app/static` in the image) |

## Migrations

Migrations live in `backend/migrations` as `NNNN_description.sql` and are
embedded into the binary. On startup the server takes a PostgreSQL advisory
lock, applies the files that are not yet recorded in `schema_migrations` in
name order, and records each one in the same transaction. To change the
schema, add a new file; never edit an applied one.

## Project layout

```
backend/
  cmd/server/        entry point: config, DB pool, migrations, HTTP server, graceful shutdown
  internal/config/   environment variables
  internal/db/       pgx connection pool
  internal/migrate/  embedded migration runner
  internal/httpapi/  routes: /api and the SPA file server
  migrations/        SQL files (embedded)
frontend/            React + TypeScript + Vite app
Dockerfile           multi-stage build: frontend, Go binary, distroless runtime
docker-compose.yml   app + PostgreSQL for local runs
```

## Deploy

Build the root `Dockerfile` and set `DATABASE_URL`. The image listens on
`PORT` (default 8080) and stops gracefully on SIGTERM.
