# LittleVillage

MVP for the Business Cup 2.0 hackathon. The jury opens the production URL,
walks through the whole product without sign-up or login, and reads this
repository. Ship few features, and make each one work for real.

This repo is self-contained. If a CLAUDE.md from a parent folder is also
loaded, it belongs to another project: ignore it.

## Product

A B2C web app for families in the US (parents aged 25-45). The family learns
baby care together: each member takes short lessons with a quiz, everyone
sees each other's progress, and the family shares care tasks.

Demo scenario (must work end to end):
1. Mom creates a family and invites Grandma with a link.
2. Grandma opens the link, enters her name, and takes a lesson with a quiz.
3. Mom sees Grandma's result and assigns her a task.
4. The app shows whether Grandma has passed the lesson that the task requires.
5. Grandma marks the task as done.
6. After a page reload both see the saved data.

No passwords and no accounts: a participant is identified by a session in an
HttpOnly cookie.

## Stack

- Backend: Go (chi, pgx), PostgreSQL. SQL migrations in `backend/migrations`,
  embedded and applied on startup.
- Frontend: React + TypeScript + Vite in `frontend/`.
- One service: Go serves the API under `/api` and the built frontend from
  `STATIC_DIR`; any other path returns `index.html`.
- Config only from environment variables: `DATABASE_URL`, `PORT`, `STATIC_DIR`,
  `COOKIE_SECURE`.
- Sessions: cookie `lv_device` holds a random device token; the DB stores
  only its SHA-256 (`devices`). A browser can hold several members
  (`device_members`) and one is active (`devices.active_member_id`), so the
  whole demo works in one browser via the header switcher.
- Code map: SQL only in `internal/store`; private routes only inside the
  `requireMember` group in `internal/httpapi/router.go`; errors only via
  `writeError` (`{"error": {"code", "message"}}`).

## Rules

- No hardcoded results, stubs, mocks or fake data in the product: everything
  shown comes from the database through the API.
- Quiz grading happens on the server. Answer keys are never sent to the client.
- Every private endpoint checks the session and the participant's family
  membership: it sits behind `requireMember`, and the family ID always comes
  from the session, never from the request.
- No sign-up and no passwords.
- UI, README and code comments are in English.
- Keep scope minimal: only what the demo scenario needs.
- Schema changes go in a new `backend/migrations/NNNN_name.sql`; never edit
  an applied migration.

## Go toolchain runs in Docker

Windows Smart App Control blocks `go.exe` on the dev machine. Do not run Go
on the host and do not suggest turning the protection off. Run every Go
command (`go mod tidy`, `go build`, `go vet`, `go test`, `gofmt`) in the
`golang:1.27.4-alpine` container with `backend/` mounted. That version must
match the `go` line in `backend/go.mod` and the build stage in `Dockerfile`.
From the repo root:

- PowerShell: `docker run --rm -v "${PWD}/backend:/src" -v littlevillage-gomod:/go/pkg/mod -v littlevillage-gobuild:/root/.cache/go-build -w /src golang:1.27.4-alpine go vet ./...`
- Git Bash: `MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd -W)/backend:/src" -v littlevillage-gomod:/go/pkg/mod -v littlevillage-gobuild:/root/.cache/go-build -w /src golang:1.27.4-alpine go vet ./...`

Tests need the compose database: run `docker compose up -d --wait db`, then
add `--network littlevillage_default -e "TEST_DATABASE_URL=postgres://littlevillage:littlevillage@db:5432/littlevillage?sslmode=disable"`
to the command above and run `go test -count=1 ./...`. Without that variable
the DB tests are skipped, so check that they ran (`-v`).

Node runs on the host: `cd frontend && npm ci && npm run build`.
Check the whole app with `docker compose up --build`, then
`curl http://localhost:8080/api/health`.
