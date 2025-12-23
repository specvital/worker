---
title: 기술 스택
description: Specvital 플랫폼 기술 스택 선택 및 기술 원칙
---

# 기술 스택

> 🇺🇸 [English Version](/en/prd/06-tech-stack.md)

## 요약

| 영역     | 선택                    | 이유               |
| -------- | ----------------------- | ------------------ |
| Parser   | Go + Tree-sitter        | 고성능, 다중 언어  |
| Backend  | Go                      | 성능, 배포 단순    |
| Frontend | React (Next.js)         | 생태계, SSR        |
| Queue    | River (PostgreSQL 기반) | DB 통합 큐, 내구성 |
| DB       | PostgreSQL              | 범용성, 안정성     |
| Deploy   | PaaS                    | DX 우선            |

## 기술 원칙

1. **타입 안전**: 컴파일 타임 검증
2. **서버리스 우선**: 초기 비용 최소화
3. **락인 회피**: 표준 기술 선택

## 리스크 관리

- 서비스별 마이그레이션 계획 수립
- 벤더 의존성 모니터링

> 버전 및 상세는 각 리포지토리의 go.mod, package.json 참조
