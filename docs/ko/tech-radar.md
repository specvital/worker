---
title: 기술 레이더
description: Specvital 플랫폼 기술 채택 현황 및 평가
---

# 기술 레이더

> 🇺🇸 [English Version](/en/tech-radar.md)

Specvital 플랫폼의 기술 채택 현황.

## 링 정의

| 링         | 설명                            |
| ---------- | ------------------------------- |
| **Adopt**  | 프로덕션 검증, 신규 작업에 권장 |
| **Trial**  | 제한된 범위에서 테스트 중       |
| **Assess** | 탐색 가치 있음, 미사용          |
| **Hold**   | 신규 프로젝트에 비권장          |

## 사분면

### 언어 및 런타임

| 기술       | 링    | 버전   | 비고        |
| ---------- | ----- | ------ | ----------- |
| Go         | Adopt | 1.24   | 주 백엔드   |
| TypeScript | Adopt | 5.8    | 프론트엔드  |
| Node.js    | Adopt | 22 LTS | 도구 런타임 |

### 백엔드

| 기술           | 링    | 버전 | 비고                      |
| -------------- | ----- | ---- | ------------------------- |
| Chi            | Adopt | 5.2  | HTTP 라우터               |
| Tree-sitter    | Adopt | -    | 다중 언어 파서            |
| River          | Adopt | -    | PostgreSQL 기반 태스크 큐 |
| pgx            | Adopt | -    | PostgreSQL 드라이버       |
| sqlc           | Adopt | -    | 타입 안전 SQL             |
| oapi-codegen   | Adopt | -    | OpenAPI 코드 생성         |
| testcontainers | Adopt | -    | 통합 테스트               |

### 프론트엔드

| 기술             | 링    | 버전 | 비고              |
| ---------------- | ----- | ---- | ----------------- |
| Next.js          | Adopt | 16.0 | React 프레임워크  |
| React            | Adopt | 19.1 | UI 라이브러리     |
| TanStack Query   | Adopt | -    | 서버 상태 관리    |
| TanStack Virtual | Adopt | -    | 가상 스크롤       |
| Tailwind CSS     | Adopt | 4.1  | 유틸리티 CSS      |
| Radix UI         | Adopt | -    | 헤드리스 컴포넌트 |
| Zod              | Adopt | -    | 스키마 검증       |
| Vitest           | Adopt | -    | 단위 테스트       |

### 플랫폼 및 인프라

| 기술           | 링    | 비고                   |
| -------------- | ----- | ---------------------- |
| Railway        | Adopt | 애플리케이션 호스팅    |
| NeonDB         | Adopt | 서버리스 PostgreSQL 16 |
| Docker         | Adopt | 컨테이너화             |
| GitHub Actions | Adopt | CI/CD                  |
| Dev Containers | Adopt | 개발 환경              |

### 개발 도구

| 기술             | 링    | 비고                |
| ---------------- | ----- | ------------------- |
| pnpm             | Adopt | 패키지 매니저       |
| Just             | Adopt | 태스크 러너         |
| Air              | Adopt | Go 핫 리로드        |
| Atlas            | Adopt | 스키마 마이그레이션 |
| Prettier         | Adopt | 코드 포매터         |
| Husky            | Adopt | Git 훅              |
| semantic-release | Adopt | 자동 버저닝         |

### AI 도구

| 기술        | 링    | 비고                   |
| ----------- | ----- | ---------------------- |
| Claude Code | Adopt | AI 코딩 어시스턴트     |
| MCP         | Adopt | Model Context Protocol |
| Gemini      | Adopt | AI 코딩 어시스턴트     |

## 원칙

1. **타입 안전성**: 컴파일 타임 검증 (Go, TypeScript, sqlc)
2. **단순성**: 잘 유지되고 집중된 라이브러리
3. **락인 방지**: 표준 프로토콜, 이식 가능한 솔루션
4. **DX 우선**: 핫 리로드, 타입 생성

## 관련 문서

- [기술 스택 (PRD)](./prd/06-tech-stack.md)
- [ADR 인덱스](./adr/)
