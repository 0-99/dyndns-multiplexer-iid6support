# Build-Stage
FROM golang:1.25.1-alpine AS builder
WORKDIR /app
# Docker BuildKit Caching for Go modules (gomod-cache) and build (gocache), see https://medium.com/@marcin.niemira/optimise-docker-build-for-go-c03d6eb8b4b
ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache
# copy source files
COPY ./src/go/ . 
# build
# Statically linked binary
RUN --mount=type=cache,target=/gomod-cache --mount=type=cache,target=/go-cache \
   go build -ldflags="-s -w" -o app .

# Runtime-Stage
FROM alpine:3.22.1
WORKDIR /app
COPY --from=builder /app/app .

# Non-root user
RUN addgroup -S app && adduser -S app -G app
USER app

EXPOSE 8080

# Healthcheck, 1 time per day, timeout 10s, start after 30s, 1 retry
HEALTHCHECK --interval=86400s --timeout=10s --start-period=30s --retries=1 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./app"]
