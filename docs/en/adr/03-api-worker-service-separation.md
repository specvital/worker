---
title: API Worker
description: ADR on separating API and worker services for independent scaling and fault isolation
---

# ADR-03: API and Worker Service Separation

> [한국어 버전](/ko/adr/03-api-worker-service-separation.md)

| Date       | Author       | Repos       |
| ---------- | ------------ | ----------- |
| 2024-12-17 | @KubrickCode | web, worker |

## Context

### Problem Statement

Systems that perform long-running analysis tasks face a fundamental architectural challenge: how to handle operations with unpredictable execution times while maintaining responsive user interactions.

Key characteristics of analysis workloads:

1. **Unpredictable Duration**: Processing time varies significantly based on input size (seconds to minutes)
2. **Resource Intensive**: High CPU, memory, and I/O consumption during analysis
3. **Conflicting Requirements**: Users expect fast API responses, but analysis requires extended processing time

### The Monolith Dilemma

Running API and analysis in a single service creates several problems:

| Issue               | Impact                                                   |
| ------------------- | -------------------------------------------------------- |
| Resource Contention | Analysis tasks starve API handlers of CPU/memory         |
| Cascading Failures  | Worker OOM crashes take down the entire service          |
| Inefficient Scaling | Must scale entire service even if only analysis needs it |
| Deployment Coupling | API changes require redeploying analysis code            |
| Cold Start Overhead | Larger binary increases startup time                     |

### Core Question

How should we architect a system that requires both responsive API interactions and resource-intensive background processing?

## Decision

**Separate the API service and worker service into independent deployable units.**

Architecture:

- **API Service**: Handles HTTP requests, validates input, enqueues tasks, returns results
- **Worker Service**: Processes queued tasks, performs analysis, persists results
- **Communication**: Message queue for task distribution
- **Data**: Shared database for results

## Options Considered

### Option A: Service Separation (Selected)

```
User → API Service → Queue → Worker Service → Database
                ↓                      ↓
           (enqueue)            (process & store)
```

**Pros:**

- Independent scaling of API and worker based on their respective loads
- Fault isolation - worker crashes don't affect API availability
- Resource optimization - allocate high-memory instances only to workers
- Deployment independence - update API without redeploying workers
- Clear separation of concerns and codebase

**Cons:**

- Operational complexity - two services to monitor and deploy
- Communication overhead - queue system dependency
- Debugging complexity - distributed system tracing required
- Infrastructure cost - queue system (PostgreSQL) as additional dependency

### Option B: Monolithic Service

```
User → Single Service (API + Worker in-process)
```

**Pros:**

- Simple deployment and operations
- No network overhead for internal communication
- Easier debugging with single process
- Lower infrastructure costs (single service)

**Cons:**

- Scaling inefficiency - must scale entire service
- Failure propagation - analysis OOM kills API
- Resource contention - analysis delays API responses
- Deployment coupling - any change requires full redeploy
- Larger container images increase cold start time

### Option C: Serverless Functions

```
User → API Service → Serverless Function (analysis)
```

**Pros:**

- Auto-scaling built-in
- Pay-per-use cost model
- No server management

**Cons:**

- Cold start latency problematic for analysis (native dependencies)
- Execution time limits may be insufficient
- Complex dependency management for native libraries
- Higher per-invocation cost for long-running tasks

## Consequences

### Positive

1. **Independent Scaling**
   - Scale workers horizontally during high analysis demand
   - Scale API independently based on user traffic
   - Cost optimization by allocating resources where needed

2. **Fault Isolation**
   - Worker memory exhaustion doesn't crash API
   - Analysis timeouts don't block API responses
   - Partial degradation instead of total failure

3. **Technical Optimization**
   - Workers: Optimize for throughput (batch processing, high memory)
   - API: Optimize for latency (caching, connection pooling)

4. **Deployment Flexibility**
   - Hotfix API bugs without touching worker code
   - Update analysis algorithms without API downtime
   - Independent release cycles

### Negative

1. **Operational Overhead**
   - Two services require separate monitoring, logging, alerting
   - Multiple deployment pipelines to maintain
   - Environment configuration synchronization needed

2. **Communication Dependency**
   - Queue system (PostgreSQL) becomes critical infrastructure
   - Network latency added to task processing
   - Message delivery guarantees must be configured

3. **Consistency Challenges**
   - Database access from multiple services requires coordination
   - Schema changes affect both services
   - Shared data models need versioning strategy

### Technical Implications

| Aspect          | Implication                                       |
| --------------- | ------------------------------------------------- |
| Queue System    | PostgreSQL-based queue required (River)           |
| Database Access | Both services need DB connection management       |
| Monitoring      | Distributed tracing for cross-service debugging   |
| Deployment      | Separate CI/CD pipelines or coordinated releases  |
| Configuration   | Shared environment variables must be synchronized |
