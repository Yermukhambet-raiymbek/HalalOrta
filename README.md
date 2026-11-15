# LittleVillage

A web app for families in the US who are expecting or caring for a baby. The
whole family learns baby care together: everyone takes short lessons with a
quiz, sees each other's progress, and shares care tasks. There are no
accounts or passwords; a participant is identified by a session cookie.

**Live app:** https://halalorta.onrender.com (free hosting: the first
request after a quiet period can take up to a minute while the server
wakes up).

## Try it in two minutes

No sign-up, and one browser is enough.

1. Open the app and create a family as a parent (for example Anna, Mom).
2. On the Family page copy the invite link and open it in the same browser.
   Join as a second person (for example Mary, Grandma). The header now has
   a "You are" switcher between the two.
3. Switch to Anna, open **Tasks** and assign "Put baby down for a nap" to
   Mary. Before you assign, the form says Mary has not passed the lesson
   "Safe Infant Sleep".
4. Switch to Mary. Her **Today** page shows the task, a note to take the
   lesson first, and the lesson as her next one.
5. Take the lesson and the quiz. Every answer must be correct to pass; you
   can retake it.
6. Back on Today the task now shows the lesson as passed. Mark it as done.
7. Switch to Anna and reload: the Family page shows Mary's result, and the
   Tasks page shows the finished task.

## What the product does

- **Family space.** One person creates a family and shares an invite link.
  Everyone who helps with the baby joins with just a name.
- **Lessons with quizzes.** Short lessons based on AAP and CDC guidance,
  each with a link to its source. Quizzes are graded on the server; the
  answer key is sent only after a submission.
- **Family progress.** One table with every member's best result per lesson.
- **Care tasks with a lesson check.** A task can name the lesson that
  prepares for it. Whether the assignee has passed that lesson is read from
  their quiz attempts on every request, so it is never stale, and it is
  shown before assigning and on the task itself. Only the assignee can mark
  a task done.
- **Today.** The active member's tasks for their calendar day (overdue ones
  included) and the lesson to take next, preferring a lesson that one of
  their open tasks needs.
- **Pricing.** What is free today and what the planned paid family plan
  adds. No payments are taken. A member can say their family would use the
  paid plan; that is saved and counted per family.

Everything shown comes from PostgreSQL through the API. Nothing is mocked
or hardcoded. The only static content is the lesson text, which lives in
`backend/content/lessons/*.json`, is validated on startup, and is synced to
the database.

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
| `GET /api/me`               | public  | Active member, their family, and all members of this browser; `{"member": null}` for a new visitor |
| `GET /api/family`           | session | The active member's family, members with their lesson progress, and the invite link |
| `POST /api/session/switch`  | session | `{member_id}`: make another member of this browser active; 403 for anyone else |
| `GET /api/lessons`          | session | Lessons with the active member's own status |
| `GET /api/lessons/{slug}`   | session | One lesson with its questions, without the answer key |
| `POST /api/lessons/{slug}/attempts` | session | `{answers: [..]}`: grade on the server, save the attempt, return the score, the key and explanations |
| `GET /api/tasks`            | session | Every task of the family, each with `required_lesson` and `lesson_passed` for its assignee |
| `POST /api/tasks`           | session | `{title, assignee_id, lesson_slug?, due_date}`: assign a task to a member of the same family |
| `PATCH /api/tasks/{id}`     | session | `{status: "done" \| "open"}`: only the assignee; 403 for other family members, 404 outside the family |
| `GET /api/today?date=YYYY-MM-DD` | session | The active member's tasks for their calendar day and their next lesson |
| `GET /api/plan/interest`    | session | Whether the member, and how many of the family, would use the paid plan |
| `POST /api/plan/interest`   | session | Save the active member's interest in the paid plan (no payment) |

"session" endpoints answer 401 without an active member. The family is
always taken from the session, never from the request.

Names are trimmed and must be 1 to 40 characters. `relationship` is one of
Mom, Dad, Grandma, Grandpa, Other. A family has at most 10 members and 200
tasks. Task titles are 1 to 80 characters. Dates are calendar days sent by
the browser, so "today" is the family's own day, not the server's.

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

What the tests cover: a member of one family cannot read or change another
family's data (family, progress, tasks); a browser can switch only to its
own members; quiz grading, and that the answer key never reaches the client
before a submission; task rules (only the assignee changes status, the
assignee must be in the family, `lesson_passed` follows quiz attempts); the
Today selection and next lesson; input validation; session cookie flags;
lesson file validation.

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
  internal/store/    all SQL: families, members, invites, devices, lessons, attempts, tasks
  internal/httpapi/  routes, session middleware, handlers, SPA file server, API tests
  content/           lesson files (embedded), their loader and validation
  internal/testdb/   per-test PostgreSQL schema for the tests
  migrations/        SQL files (embedded)
frontend/
  src/api.ts         typed API client
  src/session.tsx    active member state (GET /api/me)
  src/pages/         start, invite, Today, family, tasks, lessons, lesson with quiz, pricing
  src/components/    header with member switcher, task item, form fields, footer, status messages
Dockerfile           multi-stage build: frontend, Go binary, distroless runtime
docker-compose.yml   app + PostgreSQL for local runs
```

## Deploy

The live app runs on Render: a Docker web service built from the root
`Dockerfile` plus a Render PostgreSQL database, redeployed on every push to
`main`. To deploy elsewhere, build the root `Dockerfile` and set
`DATABASE_URL` and `COOKIE_SECURE=true`.
The image listens on `PORT` (default 8080) and stops gracefully on SIGTERM.
