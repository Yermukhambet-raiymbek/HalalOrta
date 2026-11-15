# LittleVillage

A web app for families in the US who are expecting or caring for a baby. The
whole family learns baby care together: everyone takes short lessons with a
quiz, sees each other's progress, and shares care tasks. There are no
accounts or passwords; a participant is identified by a session cookie.

**Current state:** families, invite links and browser sessions work end to
end: create a family, invite relatives by link, and switch between several
family members in one browser. Lessons, quizzes and care tasks come next.

## How sessions work

There are no accounts. The first time a browser creates or joins a family,
the server issues a random 32-byte token in the `lv_device` cookie
(HttpOnly, SameSite=Lax, 30 days, Secure over HTTPS) and stores only its
SHA-256. That browser is a *device*. Every member created on it is linked to
the device, and one of them is active. The header lets you switch between
them, so one person can try the whole family flow in one browser: create a
family, open the invite link in the same browser, add a second member, and
switch.

Every private endpoint goes through one middleware that resolves the cookie
to the active member. The family is always taken from that session, never
from the request.

## API

All responses are JSON. Errors look like
`{"error": {"code": "invite_not_found", "message": "..."}}`.

| Method & path               | Access  | What it does |
|-----------------------------|---------|--------------|
| `GET /api/health`           | public  | Database ping |
| `POST /api/families`        | public  | `{family_name, member_name, relationship}`: create a family, its owner and invite link; the owner becomes active in this browser |
| `GET /api/invites/{token}`  | public  | Family name and inviter's name; 404 for an unknown link |
| `POST /api/families/join`   | public  | `{token, member_name, relationship}`: join by invite; the new member becomes active in this browser |
| `GET /api/me`               | session | Active member, their family, and all members of this browser; 401 without one |
| `GET /api/family`           | session | The active member's family, members and invite link |
| `POST /api/session/switch`  | session | `{member_id}`: make another member of this browser active; 403 for anyone else |

Names are trimmed and must be 1 to 40 characters. `relationship` is one of
Mom, Dad, Grandma, Grandpa, Other. A family has at most 10 members.

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

Swap `go vet ./...` for `go mod tidy`, `go build ./...` or `gofmt -l .`.
The two named volumes cache modules and builds between runs. Node runs on
the host as usual.

### Tests

The API tests run the real router against the real PostgreSQL from
docker compose. Each test gets its own schema, so they never touch app
data. Start the database, then run the tests in the Go container on the
compose network:

```powershell
# PowerShell
docker compose up -d --wait db
docker run --rm --network littlevillage_default -e "TEST_DATABASE_URL=postgres://littlevillage:littlevillage@db:5432/littlevillage?sslmode=disable" -v "${PWD}/backend:/src" -v littlevillage-gomod:/go/pkg/mod -v littlevillage-gobuild:/root/.cache/go-build -w /src golang:1.27.4-alpine go test -count=1 ./...
```

```sh
# Git Bash (on macOS/Linux drop MSYS_NO_PATHCONV=1 and use "$PWD")
docker compose up -d --wait db
MSYS_NO_PATHCONV=1 docker run --rm --network littlevillage_default -e "TEST_DATABASE_URL=postgres://littlevillage:littlevillage@db:5432/littlevillage?sslmode=disable" -v "$(pwd -W)/backend:/src" -v littlevillage-gomod:/go/pkg/mod -v littlevillage-gobuild:/root/.cache/go-build -w /src golang:1.27.4-alpine go test -count=1 ./...
```

Without `TEST_DATABASE_URL` the database tests are skipped.

## Configuration

The server reads environment variables only (see `.env.example`):

| Variable       | Required | Default  | Meaning                                    |
|----------------|----------|----------|--------------------------------------------|
| `DATABASE_URL` | yes      |          | PostgreSQL connection string               |
| `PORT`         | no       | `8080`   | HTTP port                                  |
| `STATIC_DIR`   | no       | `static` | Folder with the built frontend (`/app/static` in the image) |
| `COOKIE_SECURE`| no       | `false`  | `true` forces the Secure flag on the session cookie; set it in production. Without it the flag is still set for HTTPS requests (`X-Forwarded-Proto: https`) |

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
  internal/auth/     device tokens, session cookie, session context
  internal/store/    all SQL for families, members, invites, devices
  internal/httpapi/  routes, session middleware, handlers, SPA file server, API tests
  internal/testdb/   per-test PostgreSQL schema for the tests
  migrations/        SQL files (embedded)
frontend/
  src/api.ts         typed API client
  src/session.tsx    active member state (GET /api/me)
  src/pages/         start page, invite page, family page
  src/components/    header with member switcher, form fields, status messages
Dockerfile           multi-stage build: frontend, Go binary, distroless runtime
docker-compose.yml   app + PostgreSQL for local runs
```

## Deploy

Build the root `Dockerfile` and set `DATABASE_URL` and `COOKIE_SECURE=true`.
The image listens on `PORT` (default 8080) and stops gracefully on SIGTERM.
