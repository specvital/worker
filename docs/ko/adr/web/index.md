---
title: 웹 ADR
description: 웹 플랫폼(대시보드 및 API) 아키텍처 의사결정 기록
---

# 웹 리포지토리 ADR

> 🇺🇸 [English Version](/en/adr/web/)

[specvital/web](https://github.com/specvital/web) 리포지토리 (웹 플랫폼)의 아키텍처 의사결정 기록.

## ADR 목록

| #   | 제목                                                                    | 날짜       |
| --- | ----------------------------------------------------------------------- | ---------- |
| 01  | [백엔드 언어로 Go 선택](./01-go-backend-language.md)                    | 2024-12-18 |
| 02  | [Next.js 16 + React 19 선택](./02-nextjs-react-selection.md)            | 2025-12-04 |
| 03  | [Chi 라우터 선택](./03-chi-router-selection.md)                         | 2025-01-03 |
| 04  | [TanStack Query 선택](./04-tanstack-query-selection.md)                 | 2025-01-03 |
| 05  | [shadcn/ui + Tailwind CSS 선택](./05-shadcn-tailwind-selection.md)      | 2025-01-03 |
| 06  | [SQLc 선택](./06-sqlc-selection.md)                                     | 2025-01-03 |
| 07  | [Next.js BFF 아키텍처](./07-nextjs-bff-architecture.md)                 | 2025-01-03 |
| 08  | [Clean Architecture 패턴](./08-clean-architecture-pattern.md)           | 2025-01-03 |
| 09  | [DI Container 패턴](./09-di-container-pattern.md)                       | 2025-01-03 |
| 10  | [StrictServerInterface 계약](./10-strict-server-interface-contract.md)  | 2025-01-03 |
| 11  | [Feature 기반 모듈 구조](./11-feature-based-module-organization.md)     | 2025-01-03 |
| 12  | [APIHandlers 합성 패턴](./12-apihandlers-composition-pattern.md)        | 2025-01-03 |
| 13  | [도메인 에러 처리 패턴](./13-domain-error-handling-pattern.md)          | 2025-01-03 |
| 14  | [slog 구조화 로깅](./14-slog-structured-logging.md)                     | 2025-01-03 |
| 15  | [React 19 use() Hook 패턴](./15-react-19-use-hook-pattern.md)           | 2025-01-03 |
| 16  | [nuqs URL 상태 관리](./16-nuqs-url-state-management.md)                 | 2025-01-03 |
| 17  | [next-intl i18n 전략](./17-next-intl-i18n-strategy.md)                  | 2025-01-03 |
| 18  | [next-themes 다크 모드](./18-next-themes-dark-mode.md)                  | 2025-01-03 |
| 19  | [CSS 변수 디자인 토큰 시스템](./19-css-variable-design-token-system.md) | 2025-01-03 |
| 20  | [스켈레톤 로딩 패턴](./20-skeleton-loading-pattern.md)                  | 2025-01-03 |

## 관련 문서

- [전체 ADR](/ko/adr/)
- [Web PRD](/ko/prd/03-web-platform.md)
