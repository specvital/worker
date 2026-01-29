# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SpecVital Worker - Background job processing service for analyzing test files in GitHub repositories

- Queue-based async worker (River on PostgreSQL)
- Dual-binary: Worker (scalable) + Scheduler (singleton)
- External parser: `github.com/specvital/core`

### Workers

| Worker         | Kind                | Description                                    |
| -------------- | ------------------- | ---------------------------------------------- |
| AnalyzeWorker  | `analysis:analyze`  | Parse test files from GitHub repos             |
| SpecViewWorker | `specview:generate` | AI-powered test spec documentation (see below) |

### SpecView Worker

Generates human-readable spec documents from test files using Gemini AI.

- **Phase 1**: Domain/feature classification (gemini-2.5-flash)
- **Phase 2**: Test name → behavior conversion (gemini-2.5-flash-lite, parallel)
- **Cache**: Content hash-based deduplication
- **Reliability**: Circuit breaker, rate limiting, exponential backoff

**Batch Mode (Experimental)**: For large repositories (10,000+ tests), optional Batch API mode:

- Async processing via Gemini Batch API (1-24h turnaround)
- 50% cost reduction vs real-time API
- River JobSnooze-based polling mechanism
- Enable: `SPECVIEW_USE_BATCH_API=true`

Required env vars:

- `GEMINI_API_KEY`: Gemini API key
- `GEMINI_PHASE1_MODEL`: Phase 1 model (default: gemini-2.5-flash)
- `GEMINI_PHASE2_MODEL`: Phase 2 model (default: gemini-2.5-flash-lite)

Batch mode env vars (optional):

- `SPECVIEW_USE_BATCH_API`: Enable Batch API (default: false)
- `SPECVIEW_BATCH_THRESHOLD`: Min test count for Batch mode (default: 10000)
- `SPECVIEW_BATCH_POLL_INTERVAL`: Polling interval (default: 30s)

## Documentation Map

| Context                         | Reference        |
| ------------------------------- | ---------------- |
| Architecture / Data flow        | `docs/en/`       |
| Design decisions (why this way) | `docs/en/adr/`   |
| Coding rules / Test patterns    | `.claude/rules/` |

## Commands

Before running commands, read `justfile` or check available commands via `just --list`

## Project-Specific Rules

### Auto-Generated Files (NEVER modify)

- `src/internal/infra/db/{queries.sql.go,models.go,db.go}`
- Workflow: `just dump-schema` → `just gen-sqlc`

### External Dependency

- Parsing logic lives in `github.com/specvital/core`, NOT here
- For parser changes → open issue in core repo first

### Dual Binary Architecture

- **Worker** (`cmd/worker`): horizontally scalable, queue consumer
- **Scheduler** (`cmd/scheduler`): single instance only (distributed lock)
- Must remain separate for Railway deployment - NEVER merge

### Build Artifacts Cleanup

- `just build` outputs binaries to `bin/` directory
- After build verification, ALWAYS clean up: `rm -rf bin/`
- NEVER commit `bin/` directory (already in .gitignore)

## Common Workflows

### DB Schema Changes

1. Modify schema in specvital-infra repo
2. `just dump-schema` → `just gen-sqlc`
3. Update `adapter/repository/` implementation

### Adding New Worker

1. Define worker in `adapter/queue/`
2. Register in `app/container.go`
3. Write tests
