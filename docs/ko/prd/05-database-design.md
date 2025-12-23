---
title: 데이터베이스 설계
description: Specvital 데이터베이스 스키마 및 도메인 모델 설계
---

# 데이터베이스 설계

> 🇺🇸 [English Version](/en/prd/05-database-design.md)

## ERD 개요

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

## 도메인 영역

### Core Domain

| 테이블      | 역할              |
| ----------- | ----------------- |
| codebases   | GitHub 리포지토리 |
| analyses    | 분석 작업         |
| test_suites | describe 블록     |
| test_cases  | it/test 블록      |

### Auth Domain

| 테이블         | 역할       |
| -------------- | ---------- |
| users          | 사용자     |
| oauth_accounts | OAuth 토큰 |

## 주요 Enum

- analysis_status: pending, running, completed, failed
- test_status: active, skipped, todo, focused, xfail

> 스키마 상세는 infra 리포지토리 참조
