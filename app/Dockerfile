# -- Build Stage --
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Cache dependency downloads separately from source code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile a fully static binary with no CGO dependencies.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o task-processor .

# -- Runtime Stage --
FROM alpine:3.20

# Install CA certificates for any future HTTPS calls.
RUN apk add --no-cache ca-certificates

# Run as non-root for security.
RUN adduser -D -u 1001 appuser
USER appuser

WORKDIR /app
COPY --from=builder /build/task-processor .

EXPOSE 8080

ENTRYPOINT ["./task-processor"]
