---
title: 워커 서비스
description: 백그라운드 분석 태스크 처리 워커 서비스
---

# Worker Service

> 🇺🇸 [English Version](/en/prd/04-worker-service.md)

> 백그라운드 분석 워커 (analyzer, spec-generator)

## 핵심 역할

- 메시지 큐에서 분석 태스크 소비
- Git clone → Core 파싱 → DB 저장

## 워크플로우

```
1. Backend → Queue: 분석 요청
2. Worker (analyzer) ← Queue: 태스크 수신
3. Worker → GitHub: git clone
4. Worker → Core: 파싱
5. Worker → DB: 결과 저장
```

## 에러 처리

| 유형        | 정책        |
| ----------- | ----------- |
| 일시적 오류 | 자동 재시도 |
| 영구적 오류 | 실패 처리   |

## 재시도 정책

- 지수 백오프
- Dead Letter Queue

> 설정값은 worker 리포지토리 참조
