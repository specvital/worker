---
title: AI-Based Spec Document Generation Pipeline
description: ADR for two-phase AI pipeline using Gemini for test-to-spec conversion
---

# ADR-14: AI-Based Spec Document Generation Pipeline

> [Korean Version](/ko/adr/14-ai-spec-generation-pipeline.md)

| Date       | Author       | Repos              |
| ---------- | ------------ | ------------------ |
| 2026-01-18 | @KubrickCode | worker, web, infra |

## Context

### Problem Statement

The Specvital platform requires automated conversion of test file collections into human-readable specification documents. This involves two distinct cognitive tasks:

1. **Classification**: Grouping tests by domain and feature (semantic understanding)
2. **Conversion**: Transforming test names into behavior descriptions (language generation)

### Requirements

| Requirement | Description                                                          |
| ----------- | -------------------------------------------------------------------- |
| Scale       | Handle repositories with thousands of tests (up to 500+ per chunk)   |
| Cost        | High-volume processing requires cost-efficient model selection       |
| Consistency | Cross-chunk domain assignment must remain coherent                   |
| Reliability | Production system requires fault tolerance and graceful degradation  |
| Latency     | Gemini server-side timeout of 5 minutes constrains per-request scope |

### Constraints

| Constraint           | Impact                                                      |
| -------------------- | ----------------------------------------------------------- |
| PostgreSQL Backend   | Caching integrated with existing River queue infrastructure |
| Content-Hash Caching | Documents indexed by (content_hash, language, model_id)     |
| Worker Architecture  | Must integrate with existing worker service patterns        |
| Quota System         | Cache hits must not consume user quota (ADR-13)             |

## Decision

**Adopt a two-phase AI pipeline using Google Gemini models with sequential chunking and anchor propagation for large repository handling.**

### Pipeline Architecture

```
Phase 1: Classification (gemini-2.5-flash)
├── Input: Test files + domain hints
├── Chunking: Max 500 tests, 50K tokens per chunk
├── Output: Domain grouping, feature assignment, confidence scores
└── Anchor Propagation: Carry forward domain decisions across chunks

Phase 2: Conversion (gemini-2.5-flash-lite)
├── Input: Per-feature batch of tests
├── Concurrency: Max 5 parallel API calls
├── Output: Test name → behavior description
└── Fallback: Original test name with 0.0 confidence on failure
```

### Configuration

| Parameter               | Value     | Rationale                                 |
| ----------------------- | --------- | ----------------------------------------- |
| Phase 1 Timeout         | 270s      | Below Gemini's 5-minute server limit      |
| Phase 2 Total Timeout   | 7 minutes | Allows processing of large feature sets   |
| Phase 2 Feature Timeout | 90s       | Per-feature isolation for partial success |
| Phase 2 Concurrency     | 5         | Balance API rate limits vs throughput     |
| Inter-chunk Delay       | 5 seconds | Respect rate limits between chunks        |
| Max Tests Per Chunk     | 500       | Reduced from 1000 to avoid 504 errors     |
| Max Tokens Per Chunk    | 50,000    | Keep response under 60s                   |
| Thinking Mode           | Disabled  | Cost optimization for structured tasks    |

### Reliability Patterns

| Mechanism                 | Phase 1       | Phase 2       |
| ------------------------- | ------------- | ------------- |
| Circuit Breaker Threshold | 5 failures    | 3 failures    |
| Retry Attempts            | 3             | 2             |
| Backoff Strategy          | Exponential   | Exponential   |
| Rate Limiter              | Global shared | Global shared |

### Caching Strategy

- **Cache Key**: `(content_hash, language, model_id)`
- Cache hits return existing document without consuming quota
- **Hierarchical Schema**: `spec_documents → spec_domains → spec_features → spec_behaviors`

## Options Considered

### A. LLM Provider Selection

| Option                          | Verdict                                                                 |
| ------------------------------- | ----------------------------------------------------------------------- |
| Gemini 2.5 Flash **(Selected)** | 1M token context, $0.30/1M input cost-effective, Flash-Lite for Phase 2 |
| Claude 3.5/Opus 4.5             | Superior reasoning but $5/1M (17x more expensive), 200K context limit   |
| GPT-4o/GPT-5.x                  | Broad ecosystem but $2-3/1M, 128K context limit                         |

**Selection Rationale:**

- **Cost efficiency**: Flash at $0.30/1M, Flash-Lite at $0.10/1M tokens
- **Context window**: 1M token capacity handles large test files without complex chunking
- **Task fit**: Classification and conversion are structured tasks that do not require advanced reasoning
- **Two-tier availability**: Flash for classification quality, Flash-Lite for high-volume conversion

### B. Pipeline Architecture

| Option                                                 | Verdict                                                                      |
| ------------------------------------------------------ | ---------------------------------------------------------------------------- |
| Two-phase (Classification → Conversion) **(Selected)** | Separation of concerns, different cost profiles, independent failure domains |
| Single-pass Pipeline                                   | Simpler but harder to debug, all-or-nothing failure                          |
| Multi-pass with Semantic Analysis                      | Highest quality but 3+ API calls, complex orchestration                      |

**Selection Rationale:**

- **Task specialization**: Classification and conversion have different optimal prompting strategies
- **Cost optimization**: Phase 2 uses cheaper Flash-Lite model
- **Failure isolation**: Phase 2 failures can fallback without losing Phase 1 classification
- **Independent tuning**: Each phase can be optimized separately

### C. Large Repository Handling

| Option                                                     | Verdict                                                      |
| ---------------------------------------------------------- | ------------------------------------------------------------ |
| Sequential Chunking with Anchor Propagation **(Selected)** | Predictable memory, consistent domain assignment via anchors |
| Parallel Chunking                                          | Maximum throughput but domain inconsistency across chunks    |
| Stream Processing                                          | Minimal memory but no cross-document context                 |

**Selection Rationale:**

- **Consistency**: Anchor domains propagate across chunks, preventing "feature drift"
- **Rate limit compliance**: Inter-chunk delay (5s) naturally respects API limits
- **Debuggability**: Clear chunk boundaries enable reproduction of issues
- **Memory predictability**: Fixed chunk size prevents OOM

## Implementation Details

### Model Configuration

**Deterministic Output Settings:**

| Parameter        | Value            | Rationale                                    |
| ---------------- | ---------------- | -------------------------------------------- |
| Temperature      | 0.0              | Eliminate randomness for reproducible output |
| Seed             | 42               | Fixed seed for consistent classification     |
| MaxOutputTokens  | 65,536           | Gemini maximum to prevent truncation         |
| ResponseMIMEType | application/json | Structured output enforcement                |
| ThinkingBudget   | 0                | Disable dynamic thinking overhead            |

### Token Usage Tracking

Real-time token tracking per analysis for cost monitoring:

```go
type TokenUsage struct {
    CandidatesTokens int32   // Output tokens
    Model            string  // Model identifier
    PromptTokens     int32   // Input tokens
    TotalTokens      int32   // Sum of input + output
}
```

- Extract from `GenerateContentResponse.UsageMetadata`
- Aggregate across Phase 1/2 calls per `analysis_id`
- Structured log output: `specview_token_usage`

**Rationale**: Google AI Studio usage statistics are delayed ~1 day, making real-time per-repository cost tracking impossible without custom extraction.

### Chunk Size Evolution

Progressive reduction to resolve 504 DEADLINE_EXCEEDED errors:

| Iteration | Tests/Chunk | Result                          |
| --------- | ----------- | ------------------------------- |
| Initial   | 10,000      | JSON truncation                 |
| v2        | 3,000       | Still 504 errors on large repos |
| v3        | 1,000       | Improved but occasional timeout |
| Final     | 500         | Stable 15-25s processing time   |

### Prompt Engineering

**Phase 1 (Classification):**

- Classification constraints with confidence scoring
- Language-specific instructions for output format
- Domain hint integration from analysis results

**Phase 2 (Conversion):**

- "Specification Notation" style: completion states, not actions
- Example: "User successfully authenticated" not "User should be able to authenticate"
- Confidence scoring for downstream filtering

### Error Handling Flow

```
Phase 1 Failure
├── Retry (up to 3 attempts with exponential backoff)
├── Circuit breaker trips after 5 consecutive failures
└── Job marked as failed, no partial results

Phase 2 Failure (per feature)
├── Retry (up to 2 attempts)
├── Fallback: Original test name with 0.0 confidence
└── Continue processing other features
```

**Retryable Error Patterns:**

```go
retryablePatterns := []string{
    "rate limit", "quota exceeded", "too many requests",
    "service unavailable", "internal server error", "timeout",
    "connection reset", "connection refused", "temporary failure",
}
```

**JSON Parse Error Handling**: Gemini occasionally returns truncated JSON responses. JSON parse errors are wrapped as `RetryableError` to trigger automatic retry with exponential backoff.

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                    SpecView Generation Pipeline                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐     ┌─────────────────────┐     ┌──────────────┐  │
│  │ Analysis    │────▶│  Phase 1: Classify  │────▶│ Phase 2:     │  │
│  │ Results     │     │  (gemini-2.5-flash) │     │ Convert      │  │
│  │ + Hints     │     │                     │     │ (flash-lite) │  │
│  └─────────────┘     └─────────────────────┘     └──────┬───────┘  │
│                                                          │          │
│                                                          ▼          │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                     PostgreSQL                               │   │
│  │  spec_documents                                              │   │
│  │    └── spec_domains                                          │   │
│  │          └── spec_features                                   │   │
│  │                └── spec_behaviors                            │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive

| Area              | Benefit                                                                      |
| ----------------- | ---------------------------------------------------------------------------- |
| Cost Efficiency   | Flash-Lite at $0.10/1M tokens reduces Phase 2 cost by 60%+                   |
| Reliability       | Circuit breaker and fallback ensure degraded operation over complete failure |
| Scalability       | Chunking handles repositories of any size with predictable resources         |
| Cache Integration | Content-hash caching prevents redundant processing and quota consumption     |
| Observability     | Two-phase separation provides clear metrics boundaries                       |
| Consistency       | Anchor propagation maintains domain naming coherence                         |

### Negative

| Area              | Trade-off                                                                   |
| ----------------- | --------------------------------------------------------------------------- |
| Latency           | Sequential chunking adds processing time proportional to repo size          |
| Vendor Lock-in    | Deep Gemini integration requires migration effort if provider change needed |
| Deprecation Risk  | Gemini 2.5 Flash scheduled for deprecation June 2026                        |
| Quality Ceiling   | Gemini classification accuracy below Claude for complex test hierarchies    |
| Fallback Quality  | Phase 2 fallback (original test name) provides no semantic improvement      |
| Thinking Disabled | Cost optimization trades potential quality improvement                      |

### Technical Implications

| Aspect            | Implication                                                                  |
| ----------------- | ---------------------------------------------------------------------------- |
| Schema            | Hierarchical: spec_documents → spec_domains → spec_features → spec_behaviors |
| Cache Key         | (content_hash, language, model_id) enables model-version-aware caching       |
| Timeout Design    | 270s Phase 1 stays below Gemini's 5-minute server limit                      |
| Concurrency Model | Global shared rate limiter coordinates across phases                         |
| Error Recovery    | Confidence scoring enables downstream filtering of low-quality conversions   |

## References

- [ADR-04: Queue-Based Async Processing](/en/adr/04-queue-based-async-processing.md)
- [ADR-12: Worker-Centric Analysis Lifecycle](/en/adr/12-worker-centric-analysis-lifecycle.md)
- [ADR-13: Billing and Quota Architecture](/en/adr/13-billing-quota-architecture.md)
- [Related commits](https://github.com/specvital/worker/commits/main) - SpecView worker implementation
