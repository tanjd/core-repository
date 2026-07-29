# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/food-maps-backend/Dockerfile .
#
# Uses Debian (glibc) rather than Alpine (musl): mattn/go-sqlite3's cgo
# build references off64_t/pread64/pwrite64, which musl doesn't expose
# the way glibc does.

FROM golang:1.26-bookworm AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends gcc libc6-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY apps/food-maps-backend/go.mod apps/food-maps-backend/go.sum ./
RUN go mod download

COPY apps/food-maps-backend/ ./
RUN CGO_ENABLED=1 go build -o /out/food-maps-backend ./cmd

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    useradd --no-create-home --uid 10001 app

WORKDIR /app
COPY --from=builder /out/food-maps-backend ./food-maps-backend
RUN mkdir -p /app/data && chown -R app:app /app

USER app
EXPOSE 8080
ENTRYPOINT ["./food-maps-backend"]
