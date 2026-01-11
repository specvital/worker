---
title: Graceful Shutdown
description: ADR on context-based lifecycle management for graceful shutdown in PaaS environments
---

# ADR-03: Graceful Shutdown and Context-Based Lifecycle Management

> 🇰🇷 [한국어 버전](/ko/adr/worker/03-graceful-shutdown.md)

| Date       | Author       | Repos  |
| ---------- | ------------ | ------ |
| 2024-12-18 | @KubrickCode | worker |

## Context

### The Shutdown Problem in Queue-Based Systems

Queue-based asynchronous processing (ADR-05) introduces lifecycle management challenges:

**Long-Running Task Handling:**

- Analysis tasks may run for extended periods (repository clone, parsing, metric calculation)
- Naive shutdown (immediate termination) causes data loss and inconsistent state
- Waiting indefinitely for completion blocks deployments

**PaaS Environment Constraints:**

- Platforms send SIGTERM with a grace period before SIGKILL
- Services must complete cleanup within this window
- Unresponsive processes are forcefully terminated

**Post-Cancellation Cleanup:**

- Some operations must complete even after cancellation (error logging, state updates)
- Parent context cancellation propagates to child operations
- Cleanup code fails when using cancelled context

### Failure Scenarios Without Proper Lifecycle Management

| Scenario                    | Without Management           | With Management                |
| --------------------------- | ---------------------------- | ------------------------------ |
| Deploy during long task     | Task killed mid-execution    | Task completes or times out    |
| SIGTERM received            | Abrupt termination           | Graceful drain and cleanup     |
| Task exceeds expected time  | Blocks shutdown indefinitely | Timeout forces completion      |
| Error during cancelled task | Cleanup fails silently       | Cleanup succeeds independently |

## Decision

**Adopt a context-based lifecycle management pattern with four key principles.**

### 1. Server Lifecycle Separation

Separate server start from shutdown control:

**Pattern:**

```
Start() → begins processing
Shutdown() → signals graceful stop, waits for in-flight tasks
```

**Rationale:**

- `Run()` pattern (common in libraries) blocks until internal termination
- `Start()` + `Shutdown()` allows external control of lifecycle
- Enables coordinated shutdown across multiple components

### 2. Task-Level Timeout

Apply configurable timeout to individual task execution:

**Pattern:**

```
taskCtx, cancel := context.WithTimeout(parentCtx, taskTimeout)
defer cancel()
executeTask(taskCtx)
```

**Rationale:**

- Prevents single task from blocking entire system
- Provides predictable maximum execution time
- Enables resource planning and SLA compliance

### 3. Cleanup Context Independence

Use independent context for post-cancellation cleanup:

**Pattern:**

```
if err := executeTask(taskCtx); err != nil {
    cleanupCtx := context.Background()
    recordFailure(cleanupCtx, err)  // Succeeds even if taskCtx cancelled
}
```

**Rationale:**

- Parent cancellation should not prevent error recording
- Database writes for failure tracking must complete
- Audit trail integrity requires independent cleanup

### 4. Scheduler Context Propagation

Propagate parent context for coordinated scheduler shutdown:

**Pattern:**

```
RunWithContext(ctx) → respects ctx.Done() for termination
```

**Rationale:**

- Scheduler loops must respond to shutdown signals
- Enables clean exit from periodic job loops
- Coordinates with server shutdown sequence

## Options Considered

### Option A: Context-Based Lifecycle Management (Selected)

**Description:**

Use Go's context package for propagating cancellation, timeouts, and deadlines throughout the call stack. Combine with explicit Start/Shutdown separation.

**Pros:**

- Native Go pattern, well-understood by developers
- Composable: timeouts, cancellation, and values in single abstraction
- Propagates automatically through call chain
- Enables fine-grained control per operation

**Cons:**

- Requires discipline in context propagation
- Cleanup context pattern may seem counterintuitive
- Testing requires context-aware mocking

### Option B: Fixed Wait Duration

**Description:**

Wait a fixed duration after shutdown signal, then force terminate.

```
SIGTERM → wait(30s) → force exit
```

**Pros:**

- Simple implementation
- Predictable shutdown time

**Cons:**

- Short wait: tasks terminated prematurely
- Long wait: delayed deployments, wasted resources
- No per-task granularity
- Cannot adapt to actual task requirements

### Option C: Unlimited Wait (No Timeout)

**Description:**

Wait for all in-flight tasks to complete naturally.

**Pros:**

- No task ever terminated mid-execution
- Simple mental model

**Cons:**

- Stuck tasks block shutdown indefinitely
- PaaS will SIGKILL after grace period anyway
- No protection against infinite loops or deadlocks
- Deployment velocity suffers

## Implementation Principles

### Context Hierarchy

```
applicationCtx (cancels on SIGTERM)
  └── serverCtx (cancels on Shutdown())
        └── taskCtx (cancels on timeout or parent cancellation)
              └── operationCtx (inherits from task)
```

### Shutdown Sequence

1. Receive shutdown signal (SIGTERM, API call, etc.)
2. Stop accepting new work
3. Cancel application context
4. Wait for in-flight tasks (with timeout)
5. Execute cleanup handlers
6. Exit

### Timeout Strategy

| Component       | Timeout Consideration                       |
| --------------- | ------------------------------------------- |
| Individual Task | Based on expected maximum duration + buffer |
| Server Shutdown | Sum of task timeout + cleanup time          |
| Platform Grace  | Must exceed server shutdown timeout         |

## Consequences

### Positive

**Deployment Reliability:**

- Blue-green deployments work correctly
- No orphaned processes or stuck tasks
- Predictable rollout timing

**Resource Management:**

- Bounded execution time prevents resource exhaustion
- Failed tasks don't consume resources indefinitely
- Clean process termination releases all resources

**Observability:**

- Failure records always persisted (cleanup context)
- Timeout events logged for analysis
- Shutdown sequence auditable

**PaaS Compatibility:**

- Respects SIGTERM/SIGKILL contract
- Completes within platform grace period
- Enables auto-scaling and instance replacement

### Negative

**Complexity:**

- Context propagation adds boilerplate
- Cleanup context pattern requires explanation
- Multiple timeout values to configure and tune

**Tuning Required:**

- Timeout values must match workload characteristics
- Too short: premature termination
- Too long: slow deployments

**Testing Overhead:**

- Tests must handle context cancellation scenarios
- Mock implementations need context awareness
- Timeout tests may be slow or flaky

## References

- [ADR-04: Queue-Based Asynchronous Processing](/en/adr/04-queue-based-async-processing.md)
- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md)
- [Go Context Package](https://pkg.go.dev/context)
