# syntax=docker/dockerfile:1

# Build target: analyzer, spec-generator, enqueue
ARG SERVICE=analyzer

FROM golang:1.24-alpine AS builder

ARG SERVICE

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY src/go.mod src/go.sum ./

RUN go mod download

COPY src/ ./

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /service ./cmd/${SERVICE}

FROM alpine:3.21

RUN apk add --no-cache ca-certificates git

RUN adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /service .

USER appuser

ENTRYPOINT ["./service"]
