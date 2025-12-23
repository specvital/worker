---
title: 웹 플랫폼
description: 테스트 대시보드 웹 애플리케이션 및 REST API
---

# Web Platform

> 🇺🇸 [English Version](/en/prd/03-web-platform.md)

> 웹 대시보드 및 REST API

## 구성

| 컴포넌트 | 역할            |
| -------- | --------------- |
| Backend  | REST API, OAuth |
| Frontend | 대시보드 SPA    |

## Backend API 영역

- **인증**: GitHub OAuth
- **코드베이스**: CRUD
- **분석**: 요청/결과 조회

## Frontend 주요 화면

| 화면     | 기능            |
| -------- | --------------- |
| 홈       | GitHub URL 입력 |
| 대시보드 | 코드베이스 목록 |
| 상세     | 테스트 트리뷰   |

## 인증 흐름

```
GitHub OAuth → JWT 발급 → 쿠키 저장
```

> API 상세는 OpenAPI 스펙 또는 web 리포지토리 참조
