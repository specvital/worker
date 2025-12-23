---
title: 아키텍처
description: Specvital 시스템 아키텍처 및 서비스 구성
---

# 시스템 아키텍처

> 🇺🇸 [English Version](/en/prd/01-architecture.md)

## 서비스 구성

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Frontend  │────▶│   Backend   │────▶│  Collector  │
│             │     │             │     │   (Worker)  │
└─────────────┘     └──────┬──────┘     └──────┬──────┘
                           │                   │
                    ┌──────▼──────┐     ┌──────▼──────┐
                    │  PostgreSQL │     │    Core     │
                    │ (River 큐)  │     │  (Parser)   │
                    └─────────────┘     └─────────────┘
```

## 서비스별 역할

| 서비스        | 역할                   |
| ------------- | ---------------------- |
| **Frontend**  | 웹 대시보드            |
| **Backend**   | REST API, OAuth        |
| **Collector** | 비동기 분석 워커       |
| **Core**      | 테스트 파서 라이브러리 |

## 데이터 흐름

```
사용자 → GitHub URL 입력
     → Backend: 분석 요청
     → PostgreSQL (River): 태스크 큐
     → Collector: git clone + 파싱
     → PostgreSQL: 결과 저장
     → Frontend: 결과 조회
```

## 통신 패턴

| 구간                | 방식            |
| ------------------- | --------------- |
| Frontend ↔ Backend | REST/HTTP       |
| Backend → Collector | 메시지 큐       |
| Collector → Core    | 라이브러리 호출 |

## 확장 전략

- Collector 수평 확장
- 분석 결과 캐싱
