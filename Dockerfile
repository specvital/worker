# syntax=docker/dockerfile:1

# Build target: worker, scheduler, enqueue, collector
ARG SERVICE=worker

FROM golang:1.24-alpine AS builder

ARG SERVICE

WORKDIR /app

RUN apk add --no-cache git

COPY src/go.mod src/go.sum ./

RUN go mod download

COPY src/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service ./cmd/${SERVICE}

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

RUN adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /service .

USER appuser

ENTRYPOINT ["./service"]
