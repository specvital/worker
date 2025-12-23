---
title: Architecture
description: Specvital system architecture and service composition
---

# System Architecture

> 🇰🇷 [한국어 버전](/ko/prd/01-architecture.md)

## Service Composition

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Frontend  │────▶│   Backend   │────▶│  Collector  │
│             │     │             │     │   (Worker)  │
└─────────────┘     └──────┬──────┘     └──────┬──────┘
                           │                   │
                    ┌──────▼──────┐     ┌──────▼──────┐
                    │  PostgreSQL │     │    Core     │
                    │ (River Queue)     │  (Parser)   │
                    └─────────────┘     └─────────────┘
```

## Service Roles

| Service       | Role                  |
| ------------- | --------------------- |
| **Frontend**  | Web dashboard         |
| **Backend**   | REST API, OAuth       |
| **Collector** | Async analysis worker |
| **Core**      | Test parser library   |

## Data Flow

```
User → Enter GitHub URL
    → Backend: Analysis request
    → PostgreSQL (River): Task queue
    → Collector: git clone + parsing
    → PostgreSQL: Store results
    → Frontend: View results
```

## Communication Patterns

| Path                | Method        |
| ------------------- | ------------- |
| Frontend ↔ Backend | REST/HTTP     |
| Backend → Collector | Message queue |
| Collector → Core    | Library call  |

## Scaling Strategy

- Horizontal scaling of Collectors
- Analysis result caching
