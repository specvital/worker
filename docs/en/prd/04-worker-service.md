---
title: Worker Service
description: Background analysis worker service specification
---

# Worker Service

> 🇰🇷 [한국어 버전](/ko/prd/04-worker-service.md)

> Background analysis workers (analyzer, spec-generator)

## Core Responsibilities

- Consume analysis tasks from message queue
- Git clone → Core parsing → DB storage

## Workflow

```
1. Backend → Queue: Analysis request
2. Worker (analyzer) ← Queue: Receive task
3. Worker → GitHub: git clone
4. Worker → Core: Parsing
5. Worker → DB: Store results
```

## Error Handling

| Type             | Policy         |
| ---------------- | -------------- |
| Transient errors | Auto retry     |
| Permanent errors | Mark as failed |

## Retry Policy

- Exponential backoff
- Dead Letter Queue

> See worker repository for configuration details
