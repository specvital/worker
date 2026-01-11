---
title: Queue Processing
description: ADR on queue-based async processing for long-running analysis tasks
---

# ADR-04: Queue-Based Asynchronous Processing

> 🇰🇷 [한국어 버전](/ko/adr/04-queue-based-async-processing.md)

| Date       | Author       | Repos       |
| ---------- | ------------ | ----------- |
| 2024-12-17 | @KubrickCode | web, worker |

## Context

### The Nature of Long-Running Tasks

Systems that perform computational analysis face a fundamental challenge: the processing time varies significantly and cannot be predicted in advance. This creates a conflict between user expectations for fast responses and the actual time required for analysis.

Key characteristics of such workloads:

| Characteristic         | Description                                             |
| ---------------------- | ------------------------------------------------------- |
| Unpredictable Duration | Seconds to minutes depending on input size              |
| Resource Intensive     | High CPU, memory, and I/O consumption                   |
| User Expectations      | Fast acknowledgment (<1 second) regardless of task size |
| Failure Modes          | Network issues, memory exhaustion, timeout scenarios    |

### HTTP Protocol Limitations

Standard HTTP interactions impose practical constraints:

- **Browser Timeouts**: Most browsers disconnect after 30-60 seconds
- **Load Balancer Limits**: Infrastructure typically enforces 60-second timeouts
- **Connection Management**: Long-held connections consume resources inefficiently
- **User Experience**: Users cannot navigate away during synchronous requests

### The Core Question

When requests initiate work that may take seconds to minutes, how should the system handle the communication between request acceptance and result delivery?

## Decision

**Adopt queue-based asynchronous processing for long-running tasks.**

**Why River:**

- **Polling Issue**: Asynq requires constant Redis polling, increasing latency and resource usage
- **Transactional Consistency**: River uses PostgreSQL, enabling job enqueue within the same DB transaction
- **Operational Simplicity**: Single PostgreSQL instance for both data and queue (no separate Redis)
- **Durability**: PostgreSQL-backed queue with ACID guarantees

The pattern follows this flow:

```
User → API (accepts request) → Queue → Worker (processes) → Database
                 ↓                                              ↓
           Returns job ID                              Stores result
                 ↓                                              ↓
           User polls status ←──────────────────────────────────┘
```

Core principles:

1. **Immediate Acknowledgment**: API returns a job identifier within milliseconds
2. **Background Processing**: Workers consume tasks from a queue at their own pace
3. **Status Visibility**: Users can check progress without blocking
4. **Retry Capability**: Failed tasks automatically retry with backoff

## Options Considered

### Option A: Queue-Based Asynchronous Processing (Selected)

**How It Works:**

1. API receives request, validates input, creates job record
2. Task is enqueued with metadata (job ID, parameters)
3. API returns HTTP 202 Accepted with job ID
4. Worker pulls task from queue, processes, updates database
5. User polls status endpoint or receives notification

**Pros:**

- Immediate user feedback regardless of processing time
- Independent scaling of API and worker components
- Fault isolation: worker failures don't crash API
- Built-in retry mechanisms with exponential backoff
- Dead Letter Queue (DLQ) for unrecoverable failures
- Backpressure handling: queue buffers traffic spikes

**Cons:**

- Additional infrastructure: message queue system required
- Operational complexity: multiple components to monitor
- Eventual consistency: results not immediately available
- Polling overhead or real-time connection complexity

### Option B: Synchronous Processing

**How It Works:**

```
User → API → Process (blocking) → Response
       └────── 30+ seconds ──────┘
```

**Pros:**

- Simple implementation: single request-response cycle
- No additional infrastructure required
- Immediate result delivery when successful
- Easier debugging: single execution path

**Cons:**

- HTTP timeout failures for long tasks
- Resource contention: processing blocks API threads
- Poor user experience: no feedback during wait
- Cascading failures: memory exhaustion affects entire service
- No retry capability: user must manually retry
- Cannot scale processing independently

### Option C: Webhook Callback

**How It Works:**

1. User submits job with callback URL
2. API returns acceptance, begins processing
3. Upon completion, system POSTs results to callback URL
4. User's server receives notification

**Pros:**

- Real-time notification when complete
- No polling required
- Event-driven architecture alignment
- Reduces API load from status checks

**Cons:**

- User must provide and maintain callback endpoint
- Delivery reliability concerns: retries, DLQ for callbacks
- Security complexity: URL validation, HMAC signatures
- Not suitable for end-user facing applications
- Higher integration barrier for consumers

## Consequences

### Positive

**User Experience**

| Metric                | Synchronous    | Asynchronous |
| --------------------- | -------------- | ------------ |
| Initial Response Time | 30+ seconds    | <500ms       |
| Abandonment Rate      | 40-60%         | 10-20%       |
| Error Rate (timeout)  | Varies by task | Near zero    |
| Progress Visibility   | None           | Full status  |

**System Reliability**

- **Fault Isolation**: Worker memory exhaustion doesn't crash API service
- **Graceful Degradation**: Queue buffers requests during downstream failures
- **Automatic Recovery**: Transient failures retry without user intervention
- **Observability**: Queue depth provides clear health signal

**Scalability**

- Scale workers independently based on queue depth
- Scale API based on request rate
- Handle traffic spikes by queue buffering
- Optimize resources: high-memory for workers, low-latency for API

### Negative

**Operational Overhead**

- Queue system becomes critical infrastructure
- Requires monitoring: queue depth, processing latency, failure rates
- Multiple deployment pipelines to maintain
- Environment configuration synchronization needed

**Complexity**

- Distributed system debugging required
- Eventual consistency model to communicate to users
- Additional failure modes: queue unavailability, message loss
- Status synchronization between components

### Technical Implications

| Aspect          | Implication                                                                      |
| --------------- | -------------------------------------------------------------------------------- |
| Queue Selection | PostgreSQL-backed River for transactional consistency and operational simplicity |
| Retry Strategy  | Exponential backoff with jitter; classify transient vs permanent failures        |
| DLQ Handling    | Manual inspection and replay capability required                                 |
| Monitoring      | Queue depth, processing time, failure rate dashboards                            |
| Idempotency     | Workers must handle duplicate task delivery safely                               |

### Error Classification Strategy

| Error Type     | Retry Behavior           | Example                               |
| -------------- | ------------------------ | ------------------------------------- |
| Transient      | Exponential backoff      | Network timeout, temporary DB failure |
| Non-Transient  | Move to DLQ immediately  | Invalid input, parse error            |
| Resource Limit | Backoff with longer wait | Rate limit, memory pressure           |

### User Communication Pattern

1. **Submission**: Return job ID with estimated time
2. **In Progress**: Show current step and percentage
3. **Completion**: Provide results or error details
4. **Failure**: Clear explanation with retry option
