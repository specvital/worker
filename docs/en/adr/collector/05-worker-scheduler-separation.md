---
title: Worker-Scheduler Separation
description: ADR on separating Worker and Scheduler into independent processes
---

# ADR-05: Worker-Scheduler Process Separation

> 🇰🇷 [한국어 버전](/ko/adr/collector/05-worker-scheduler-separation.md)

| Date       | Author       | Repos     |
| ---------- | ------------ | --------- |
| 2024-12-18 | @KubrickCode | collector |

## Context

### Asymmetric Scaling Requirements

Background job processing systems typically consist of two distinct components with fundamentally different scaling characteristics:

**Scheduler:**

- Triggers periodic jobs (cron-based task enqueuing)
- Must run as single instance to prevent duplicate executions
- Lightweight resource footprint (CPU <5%, memory <256 MB)
- Rarely changes (cron expression updates, new job types)

**Worker:**

- Processes queued tasks (analysis, file operations, API calls)
- Scales horizontally based on queue depth
- Heavy resource consumption (CPU-intensive, memory for large payloads)
- Frequent updates (business logic changes, algorithm improvements)

### Problems with Combined Process

Running scheduler and worker in a single process creates several issues:

| Issue                     | Impact                                               |
| ------------------------- | ---------------------------------------------------- |
| Resource Waste            | Scheduler code runs in all worker instances (unused) |
| Scaling Inefficiency      | Can't scale workers without duplicating schedulers   |
| Failure Coupling          | Worker OOM crashes the scheduler                     |
| Deployment Lock           | Scheduler changes require full redeployment          |
| Distributed Lock Overhead | All instances acquire locks, only one succeeds       |

### Different Dependency Requirements

Each process type has distinct dependency needs:

- **Worker**: Requires encryption keys (OAuth token decryption for private repositories)
- **Scheduler**: Requires distributed lock (single-instance guarantee), no encryption needed

Combining these creates unnecessary security exposure and configuration complexity.

## Decision

**Separate Worker and Scheduler into independent processes with dedicated entry points and DI containers.**

### Architecture

```
┌──────────────┐      ┌───────────┐      ┌──────────────┐
│  Scheduler   │─────>│PostgreSQL │<─────│   Workers    │
│ (1 instance) │      │River Queue│      │ (0-N scaled) │
└──────────────┘      └───────────┘      └──────────────┘
       │                    │                    │
       └────────────────────┴────────────────────┘
                            │
                     ┌──────────────┐
                     │  PostgreSQL  │
                     │ (Data Store) │
                     └──────────────┘
```

### Entry Point Separation

```
cmd/
├── worker/main.go     # Queue task processing
└── scheduler/main.go  # Periodic job scheduling
```

Each entry point:

- Validates only its required configuration
- Initializes only its required dependencies
- Has dedicated graceful shutdown handling

### DI Container Separation

```
WorkerContainer:
├── Encryption adapter (OAuth token decryption)
├── Analysis handler (queue task processor)
├── Queue client (task consumption)
└── Shared: Database pool, PostgreSQL connection

SchedulerContainer:
├── Distributed lock (single-instance guarantee)
├── Scheduler handler (periodic job executor)
├── Queue client (task enqueuing)
└── Shared: Database pool, PostgreSQL connection
```

**Key Principle**: Worker container never initializes lock, scheduler container never initializes encryption.

## Options Considered

### Option A: Process Separation (Selected)

**Description:**

Separate binaries with dedicated entry points and DI containers.

**Pros:**

- Independent scaling (workers: 0-N, scheduler: exactly 1)
- Failure isolation (worker crash doesn't affect scheduling)
- Resource optimization (scheduler: minimal, workers: heavy)
- Clear security boundaries (encryption key only in workers)
- Build-time optimization (smaller scheduler binary)

**Cons:**

- Two deployment pipelines to maintain
- Configuration synchronization required
- More complex monitoring setup

### Option B: Single Binary with Runtime Mode

**Description:**

Single binary that switches behavior based on environment variable or flag.

```bash
./collector --mode=worker
./collector --mode=scheduler
```

**Pros:**

- Single build artifact
- Shared codebase (less duplication)
- Simpler CI/CD pipeline

**Cons:**

- Binary includes unused code (scheduler loads worker deps in memory)
- Runtime misconfiguration risk (wrong mode deployed)
- Unclear from code inspection what service does
- Still requires separate deployment configurations

### Option C: Combined Process

**Description:**

Single process runs both scheduler and worker in different goroutines.

**Pros:**

- Simplest deployment (one service)
- No inter-process communication overhead
- Single configuration file

**Cons:**

- Cannot scale components independently
- Resource waste (scheduler in every worker instance)
- Failure coupling (worker panic kills scheduler)
- Must provision for max(scheduler, worker) resources

## Implementation Principles

### Configuration Validation

Each process validates only its requirements:

```
Worker startup:
├── Check DATABASE_URL (required)
├── Check ENCRYPTION_KEY (required) ← Unique to worker
└── Fail fast if missing

Scheduler startup:
├── Check DATABASE_URL (required)
├── Initialize distributed lock ← Unique to scheduler
└── Fail fast if connection fails
```

### Distributed Lock Strategy

Scheduler uses PostgreSQL-based distributed lock to ensure single-instance execution:

```
Instance A: Acquires lock → Executes scheduled jobs
Instance B: Lock acquisition fails → Remains standby
Instance C: Lock acquisition fails → Remains standby
```

**Benefits:**

- High availability during blue-green deployments
- Automatic failover if active instance crashes
- Lock heartbeat extends TTL for long-running jobs

### Queue-Based Communication

Scheduler and workers communicate exclusively through the message queue:

```
Scheduler ──[Enqueue Task]──> River Queue (PostgreSQL) ──[Dequeue Task]──> Worker
```

**Decoupling Benefits:**

- Scheduler doesn't wait for worker completion
- Workers process at their own pace
- Natural backpressure through queue depth
- No direct scheduler-worker network communication

### Graceful Shutdown Handling

Each process has tailored shutdown behavior:

**Worker Shutdown:**

1. Stop accepting new tasks from queue
2. Wait for in-flight tasks (with configurable timeout)
3. Close database/PostgreSQL connections
4. Exit

**Scheduler Shutdown:**

1. Stop cron scheduler (prevent new job triggers)
2. Wait for current job completion (with timeout)
3. Release distributed lock
4. Close database/PostgreSQL connections
5. Exit

## Consequences

### Positive

**Independent Scaling:**

- Scale workers based on queue depth without touching scheduler
- Scheduler stays minimal (single instance, low resources)
- PaaS auto-scaling applies only to workers

**Cost Optimization:**

- Scheduler: Fixed minimal resources (~0.25 vCPU, 256 MB)
- Workers: Scale 0-N based on demand
- Significant savings vs. running N combined instances

**Failure Isolation:**

- Worker memory exhaustion doesn't affect scheduling
- Scheduler issues don't prevent worker task processing
- Partial degradation instead of total outage

**Deployment Independence:**

- Update worker logic without scheduler downtime
- Change cron schedules without worker redeployment
- Different release cadences per component

**Security Boundaries:**

- Encryption keys confined to worker processes
- Scheduler operates with reduced privileges
- Clear audit trail per process type

### Negative

**Operational Complexity:**

- Two services to monitor, deploy, and maintain
- Multiple CI/CD pipelines
- Log aggregation across services

**Configuration Management:**

- Shared task schemas require coordination
- Environment variable synchronization
- Queue naming consistency

**Debugging Overhead:**

- Cross-service request tracing
- Distributed log correlation
- Multiple dashboards to monitor

### Technical Implications

| Aspect                 | Implication                                           |
| ---------------------- | ----------------------------------------------------- |
| Infrastructure         | Separate PaaS services, shared PostgreSQL             |
| Deployment             | Independent release cycles, coordinated for contracts |
| Scaling                | Workers: auto-scale, Scheduler: fixed single instance |
| Monitoring             | Per-service metrics, unified queue depth monitoring   |
| Blue-Green Deployments | Workers: overlapping, Scheduler: lock-based handoff   |

## References

- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md) (Container separation)
- [ADR-03: Graceful Shutdown](./03-graceful-shutdown.md) (Lifecycle management)
- [ADR-03: API and Worker Service Separation](/en/adr/03-api-worker-service-separation.md) (Cross-cutting)
- [ADR-04: Queue-Based Asynchronous Processing](/en/adr/04-queue-based-async-processing.md)
