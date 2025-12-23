---
title: Go Backend Language
description: ADR on choosing Go for backend to share infrastructure with existing services
---

# ADR-01: Go as Backend Language

> 🇰🇷 [한국어 버전](/ko/adr/web/01-go-backend-language.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-18 | @KubrickCode | web   |

## Context

### The Language Selection Question

The web platform requires a backend language choice. Two primary candidates emerged:

1. **Go**: Aligns with existing collector and core services
2. **NestJS (TypeScript)**: Aligns with Next.js frontend

### Existing Architecture

The system already uses Go for:

- **Core Library**: Parser engine, crypto utilities, domain models
- **Collector Service**: Background worker processing analysis jobs via River
- **Shared Infrastructure**: PostgreSQL-based task queue (River)

### Key Consideration

The web backend needs to:

- Enqueue analysis tasks for collector processing
- Share cryptographic operations with collector (OAuth token encryption)
- Access core library functionality when needed

## Decision

**Adopt Go as the web backend language to maximize technology stack integration with existing services.**

Core principles:

1. **Single Queue System**: Share River with collector (no separate BullMQ)
2. **Direct Library Access**: Import core library without RPC overhead
3. **Unified Tooling**: Single language CI/CD, monitoring, deployment
4. **Shared Cryptography**: Same encryption/decryption for OAuth tokens

## Options Considered

### Option A: Go Backend (Selected)

**How It Works:**

- Go HTTP server (Chi/Gin/Echo) serves REST API
- Direct import of `github.com/specvital/core` packages
- Shared River client for task enqueueing
- Same PostgreSQL instance as collector

**Pros:**

- **Zero Integration Overhead**: Direct core library import
- **Shared Queue Infrastructure**: Single PostgreSQL instance, unified River protocol
- **Consistent Cryptography**: Identical encryption across services
- **Operational Simplicity**: One language runtime to manage
- **Resource Efficiency**: Lower memory footprint than Node.js

**Cons:**

- Frontend developers must learn Go basics
- Smaller ecosystem compared to npm
- No shared types with TypeScript frontend

### Option B: NestJS + TypeScript

**How It Works:**

- NestJS framework with TypeScript
- BullMQ for task queue (separate from River)
- Core library access via gRPC wrapper or TypeScript rewrite

**Pros:**

- Shared language with Next.js frontend
- Larger package ecosystem (npm)
- Type sharing between frontend and backend

**Cons:**

- **Core Library Incompatibility**: Cannot directly use Go core
- **Dual Queue Systems**: BullMQ (web) + River (collector) = complex bridging
- **Cryptography Reimplementation**: Must rewrite NaCl encryption in TypeScript
- **Operational Complexity**: Two language runtimes, separate CI/CD

### Option C: NestJS BFF + Go Core API

**How It Works:**

- NestJS as Backend-for-Frontend layer
- Go service wrapping core library
- gRPC communication between layers

**Evaluation:**

- Three-tier latency overhead
- Complex deployment topology
- Premature optimization for current scale
- **Rejected**: Overkill for requirements

## Implementation Considerations

### Core Library Integration

```
Web Service (Go)                    Collector (Go)
      │                                   │
      └─── import core/pkg/crypto ────────┘
      │                                   │
      └─── import core/pkg/domain ────────┘
```

Key packages shared:

- `core/pkg/crypto`: NaCl SecretBox encryption for OAuth tokens
- `core/pkg/domain`: Type-safe domain models

### Queue Architecture

```
Web (Producer)       PostgreSQL (NeonDB)      Collector (Consumer)
      │                       │                        │
      ├─ river.Insert() ────→ task_queue ────→ river.Work()
      │                       │                        │
      └──────────── shared River protocol ─────────────┘
```

Benefits:

- Single PostgreSQL instance (cost reduction)
- Type-safe task payloads
- Built-in retry, scheduling, dead-letter queue

### Cryptography Sharing

OAuth flow requires:

1. Web encrypts GitHub token before DB storage
2. Collector decrypts token when accessing GitHub API

With Go:

- Same `crypto.Encryptor` interface
- Same encryption key (environment variable)
- Guaranteed compatibility

With NestJS:

- Must reimplement NaCl SecretBox in TypeScript
- Risk of subtle cryptographic incompatibilities
- Additional testing burden

## Consequences

### Positive

**Infrastructure Efficiency:**

- Single PostgreSQL instance serves both web and collector
- Unified monitoring and alerting
- Shared deployment patterns

**Development Velocity:**

- No serialization layer between web and core
- Compile-time type safety across services
- Consistent error handling patterns

**Operational Simplicity:**

- One language for backend services
- Single CI/CD pipeline pattern
- Unified dependency management (go.mod)

### Negative

**Learning Curve:**

- Frontend developers need Go familiarity
- **Mitigation**: Focused training, pair programming

**Type Sharing:**

- No automatic type generation for frontend
- **Mitigation**: OpenAPI codegen for TypeScript clients

**Ecosystem:**

- Fewer ready-made packages than npm
- **Mitigation**: Evaluate alternatives before starting features

## References

- [ADR-04: Queue-Based Asynchronous Processing](/en/adr/04-queue-based-async-processing.md)
- [ADR-07: Shared Infrastructure Strategy](/en/adr/07-shared-infrastructure.md)
- [Core ADR-01: Core Library Separation](/en/adr/core/01-core-library-separation.md)
