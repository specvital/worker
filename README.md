# specvital collector

Repository analysis worker service for SpecVital platform.

## Architecture

Clean Architecture with separate entry points for Railway deployment:

```
src/cmd/
├── worker/      # River worker - queue processing (Railway service #1)
├── scheduler/   # Cron scheduler - periodic jobs (Railway service #2)
├── enqueue/     # CLI tool for manual task enqueue
```

## Build

```bash
# Build all binaries
just build

# Build specific target
just build worker
just build scheduler
just build enqueue

# Output: bin/worker, bin/scheduler, bin/enqueue
```

## Development

```bash
# Run worker locally with hot reload
just run local

# Run scheduler locally
just run-scheduler local

# Run tests
just test unit
just test integration
just test all
```

## Environment Variables

- `DATABASE_URL`: PostgreSQL connection string (also used for river job queue)

## Railway Deployment

Deploy as two separate services:

- **Worker**: `bin/worker` - processes analysis tasks from queue (scalable)
- **Scheduler**: `bin/scheduler` - runs periodic cron jobs (single instance)
