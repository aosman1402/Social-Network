# --- Build stage -------------------------------------------------------
# go-sqlite3 uses cgo, so the build needs a real C toolchain; alpine +
# gcc/musl-dev keeps that toolchain out of the final image.
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/server .

# --- Runtime stage -------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs && \
    adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /out/server ./server
COPY --from=builder /app/pkg/db/migrations ./pkg/db/migrations

RUN mkdir -p /app/uploads /app/pkg/db/sqlite && \
    chown -R appuser:appuser /app

# Snapshot of the project's database (demo + seed users) and the images
# its posts/avatars point to. Docker copies these into the named volumes
# the first time they're created (only while the volume is still empty),
# so a fresh `docker compose up` starts with the same data as a local run,
# and later changes made in the container persist in the volume.
COPY --from=builder --chown=appuser:appuser /app/pkg/db/sqlite/social-network.db ./pkg/db/sqlite/social-network.db
COPY --from=builder --chown=appuser:appuser /app/uploads/ ./uploads/

# Explicit paths so the server doesn't depend on whatever WORKDIR it's
# started from; both directories are meant to be mounted as volumes so
# the database and uploaded files survive container recreation.
ENV DB_PATH=/app/pkg/db/sqlite/social-network.db
ENV MIGRATIONS_PATH=/app/pkg/db/migrations/sqlite
ENV PORT=8081

EXPOSE 8081
VOLUME ["/app/pkg/db/sqlite", "/app/uploads"]

USER appuser

CMD ["./server"]
