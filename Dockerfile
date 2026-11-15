# 1) Build the frontend.
FROM node:24-alpine AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY frontend/ ./
RUN npm run build

# 2) Build the Go server.
# Keep this Go version equal to the "go" line in backend/go.mod.
FROM golang:1.27.4-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# 3) Small runtime image: one static binary plus the built frontend, non-root.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/server /app/server
COPY --from=frontend /frontend/dist /app/static
ENV PORT=8080 \
    STATIC_DIR=/app/static
EXPOSE 8080
ENTRYPOINT ["/app/server"]
