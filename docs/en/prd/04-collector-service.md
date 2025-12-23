---
title: Collector Service
description: Background analysis worker service specification
---

# Collector Service

> 🇰🇷 [한국어 버전](/ko/prd/04-collector-service.md)

> Background analysis worker

## Core Responsibilities

- Consume analysis tasks from message queue
- Git clone → Core parsing → DB storage

## Workflow

```
1. Backend → Queue: Analysis request
2. Collector ← Queue: Receive task
3. Collector → GitHub: git clone
4. Collector → Core: Parsing
5. Collector → DB: Store results
```

## Error Handling

| Type             | Policy         |
| ---------------- | -------------- |
| Transient errors | Auto retry     |
| Permanent errors | Mark as failed |

## Retry Policy

- Exponential backoff
- Dead Letter Queue

> See collector repository for configuration details
