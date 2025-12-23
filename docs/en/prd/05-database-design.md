---
title: Database Design
description: Database schema design for codebases, analyses, and tests
---

# Database Design

> 🇰🇷 [한국어 버전](/ko/prd/05-database-design.md)

## ERD Overview

```
codebases (1) ──▶ (N) analyses
                      │
                      ▼
                 (N) test_suites ◀── parent (self)
                      │
                      ▼
                 (N) test_cases

users (1) ──▶ (N) oauth_accounts
```

## Domain Areas

### Core Domain

| Table       | Role                |
| ----------- | ------------------- |
| codebases   | GitHub repositories |
| analyses    | Analysis jobs       |
| test_suites | describe blocks     |
| test_cases  | it/test blocks      |

### Auth Domain

| Table          | Role         |
| -------------- | ------------ |
| users          | Users        |
| oauth_accounts | OAuth tokens |

## Key Enums

- analysis_status: pending, running, completed, failed
- test_status: active, skipped, todo, focused, xfail

> See infra repository for schema details
