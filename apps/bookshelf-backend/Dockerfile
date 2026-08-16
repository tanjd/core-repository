# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/bookshelf-backend/Dockerfile .
#
# Uses Debian (glibc) rather than Alpine (musl): mattn/go-sqlite3's cgo
# build references off64_t/pread64/pwrite64, which musl doesn't expose
# the way glibc does (same reasoning as apps/food-maps-backend/Dockerfile).

FROM golang:1.25-bookworm AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends gcc libc6-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY apps/bookshelf-backend/go.mod apps/bookshelf-backend/go.sum ./
RUN go mod download

COPY apps/bookshelf-backend/ ./
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags "-X main.version=${VERSION}" -o /out/bookshelf-backend ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates wget && \
    rm -rf /var/lib/apt/lists/* && \
    useradd --no-create-home --uid 10001 app

WORKDIR /app
COPY --from=builder /out/bookshelf-backend ./bookshelf-backend
RUN mkdir -p /app/data && chown -R app:app /app

USER app
EXPOSE 8000
ENTRYPOINT ["./bookshelf-backend"]
