---
title: 컬렉터 서비스
description: 백그라운드 분석 태스크 처리 워커 서비스
---

# Collector Service

> 🇺🇸 [English Version](/en/prd/04-collector-service.md)

> 백그라운드 분석 워커

## 핵심 역할

- 메시지 큐에서 분석 태스크 소비
- Git clone → Core 파싱 → DB 저장

## 워크플로우

```
1. Backend → Queue: 분석 요청
2. Collector ← Queue: 태스크 수신
3. Collector → GitHub: git clone
4. Collector → Core: 파싱
5. Collector → DB: 결과 저장
```

## 에러 처리

| 유형        | 정책        |
| ----------- | ----------- |
| 일시적 오류 | 자동 재시도 |
| 영구적 오류 | 실패 처리   |

## 재시도 정책

- 지수 백오프
- Dead Letter Queue

> 설정값은 collector 리포지토리 참조
