---
title: SpecView Worker Binary Separation
description: ADR for separating SpecViewWorker from AnalyzerWorker into independent binaries
---

# ADR-08: SpecView Worker Binary Separation

> 🇰🇷 [한국어 버전](/ko/adr/worker/08-specview-worker-separation)

| Date       | Author     | Repos  |
| ---------- | ---------- | ------ |
| 2026-01-13 | @specvital | worker |

## Context

### Design Intent Violation

The Specvital worker architecture follows a binary separation pattern ([ADR-05](./05-worker-scheduler-separation.md)) where each workload type runs as an independent process with dedicated configuration and dependencies.

Initial SpecView implementation violated this pattern by integrating SpecViewWorker into AnalyzerContainer, creating several architectural problems.

### Problems with Integrated Approach

| Problem               | Impact                                                       |
| --------------------- | ------------------------------------------------------------ |
| Secret Contamination  | Analyzer required GEMINI_API_KEY despite not using Gemini    |
| Queue Routing Failure | "Unhandled job kind" errors when jobs routed to wrong worker |
| Scaling Mismatch      | CPU-bound parsing coupled with I/O-bound API workloads       |
| Cost Unpredictability | Pay-per-token AI costs mixed with predictable parsing        |

### Workload Characteristic Asymmetry

| Concern         | Analyzer              | Spec-Generator            |
| --------------- | --------------------- | ------------------------- |
| External API    | None                  | Gemini API                |
| Secrets         | ENCRYPTION_KEY        | GEMINI_API_KEY            |
| Scaling Profile | CPU-bound (parsing)   | I/O-bound (API calls)     |
| Cost Profile    | Predictable (compute) | Variable (pay-per-token)  |
| Timeout         | Short (~30s)          | Long (~10min)             |
| Failure Mode    | Memory exhaustion     | Rate limiting, API errors |

The fundamental mismatch: test file parsing is a deterministic, local computation while spec generation is a non-deterministic, network-dependent AI task.

## Decision

**Separate AnalyzeWorker and SpecViewWorker into independent binaries with dedicated queues and configuration requirements.**

### Architecture

```
src/cmd/
├── analyzer/main.go       # Test file parsing (Tree-sitter, ENCRYPTION_KEY)
├── spec-generator/main.go # AI document generation (Gemini API, GEMINI_API_KEY)
├── scheduler/main.go      # Cron-based job scheduling
└── enqueue/main.go        # Manual enqueuing utility

River Queues:
├── analyze_repository     # Consumed by analyzer binary only
└── generate_spec_document # Consumed by spec-generator binary only
```

### Binary Responsibilities

**analyzer/main.go:**

- Consumes `analyze_repository` jobs from River queue
- Clones repository, runs Tree-sitter parsing, extracts test metadata
- Requires: `DATABASE_URL`, `ENCRYPTION_KEY` (for OAuth token decryption)
- Does NOT require: `GEMINI_API_KEY`

**spec-generator/main.go:**

- Consumes `generate_spec_document` jobs from River queue
- Calls Gemini API for classification and conversion ([ADR-14](/en/adr/14-ai-spec-generation-pipeline))
- Requires: `DATABASE_URL`, `GEMINI_API_KEY`
- Does NOT require: `ENCRYPTION_KEY`

### Queue Isolation

Each binary registers only its supported job kinds:

```go
// analyzer/main.go
river.AddWorker(client, &AnalyzeRepositoryWorker{})
// Only handles: analyze_repository

// spec-generator/main.go
river.AddWorker(client, &GenerateSpecDocumentWorker{})
// Only handles: generate_spec_document
```

## Options Considered

### Option A: Binary Separation (Selected)

Separate binaries (`cmd/analyzer`, `cmd/spec-generator`) with dedicated queues and configuration validation.

**Pros:**

- Secret isolation - each binary only loads required secrets
- Independent scaling - scale AI workloads separately from parsing
- Cost attribution - clear separation of compute vs API costs
- Failure isolation - Gemini rate limits don't affect test parsing
- Queue clarity - each queue maps to exactly one consumer binary

**Cons:**

- Two binaries to build, deploy, and monitor
- Shared code must be extracted to internal packages
- Configuration duplication for common settings

### Option B: Single Binary with Runtime Mode

Single binary with `--mode=analyzer` or `--mode=spec-generator` flag.

**Pros:**

- Single build artifact
- Simpler CI/CD pipeline

**Cons:**

- Binary includes all dependencies (Gemini SDK loaded even in analyzer mode)
- Runtime misconfiguration risk
- Secrets must be validated at runtime, not startup
- Binary size bloat

### Option C: Combined Process with Goroutines

Single process runs both workers as separate goroutines.

**Pros:**

- Simplest deployment
- Shared connection pools

**Cons:**

- Secret exposure - every instance has both keys
- Cannot scale independently
- Resource contention between CPU-bound and I/O-bound tasks
- Failure coupling
- Violates [ADR-05](./05-worker-scheduler-separation.md) pattern

## Consequences

### Positive

| Area              | Benefit                                                                            |
| ----------------- | ---------------------------------------------------------------------------------- |
| Security          | Analyzer never touches GEMINI_API_KEY; spec-generator never touches ENCRYPTION_KEY |
| Scaling           | Scale spec-generator independently based on AI queue depth                         |
| Cost Visibility   | Gemini API costs isolated to spec-generator service metrics                        |
| Reliability       | Gemini outages don't affect test parsing pipeline                                  |
| Timeout           | Analyzer: 30s (fast fail), Spec-generator: 10min (AI tolerance)                    |
| PaaS Optimization | Different instance sizes per workload profile                                      |

### Negative

| Area                   | Trade-off                                           |
| ---------------------- | --------------------------------------------------- |
| Operational Complexity | Two services to monitor with separate health checks |
| Build Pipeline         | Two Docker images to build and push                 |
| Shared Code            | Must extract common utilities to internal packages  |
| Debugging              | Cross-service tracing for related jobs              |

## References

- [ADR-05: Worker-Scheduler Process Separation](./05-worker-scheduler-separation.md)
- [ADR-14: AI-Based Spec Document Generation Pipeline](/en/adr/14-ai-spec-generation-pipeline.md)
- [ADR-04: Queue-Based Asynchronous Processing](/en/adr/04-queue-based-async-processing.md)
- Commit `f3fae45`: refactor(worker): separate AnalyzeWorker and SpecViewWorker into independent binaries
- Commit `3cfee6f`: fix(queue): isolate dedicated queues per worker to resolve Unhandled job kind error
