---
title: 멀티큐 우선순위 라우팅
description: 티어 기반 큐 라우팅 아키텍처 및 워커 할당 설정 ADR
---

# ADR-16: 멀티큐 우선순위 라우팅 아키텍처

> 🇬🇧 [English Version](/en/adr/16-multi-queue-priority-routing.md)

| 날짜       | 작성자       | 관련 레포          |
| ---------- | ------------ | ------------------ |
| 2026-01-19 | @KubrickCode | web, worker, infra |

## 컨텍스트

### 비즈니스 문제

Specvital 플랫폼은 [ADR-13](/ko/adr/13-billing-quota-architecture.md)에 따라 테스트 분석 서비스 수익화를 위한 티어 요금제 (Free, Pro, Pro Plus, Enterprise) 도입. 그러나 기존 큐 인프라는 모든 요청을 동등하게 처리하여 세 가지 문제 발생:

| 문제             | 비즈니스 영향                                         |
| ---------------- | ----------------------------------------------------- |
| 우선순위 미구분  | 피크 시간대 유료 고객이 무료 티어 뒤에서 대기         |
| 스케줄러 경합    | 백그라운드 재분석 작업과 사용자 요청이 동일 큐 공유   |
| 워커 라우팅 오류 | Analyzer와 SpecGenerator 워커가 잘못된 작업 타입 수신 |

### 기술적 발전 과정

큐 아키텍처의 세 단계 발전:

**Phase 1: 단일 공유 큐**

```
모든 요청 → 단일 FIFO 큐 → 모든 워커
```

결과: 워커가 호환되지 않는 작업 타입 수신 시 "Job kind not registered" 오류 발생.

**Phase 2: 서비스별 전용 큐**

```
Analysis 요청 → analysis 큐 → Analyzer 워커
SpecView 요청 → specview 큐 → SpecGen 워커
```

결과: 작업 라우팅 오류 해결, 그러나 유료 사용자 우선순위 제어 불가.

**Phase 3: 티어 기반 멀티큐 (현재)**

```
Pro 사용자 분석 → analysis_priority → Analyzer (priority 워커)
Free 사용자 분석 → analysis_default → Analyzer (default 워커)
스케줄러 작업   → analysis_scheduled → Analyzer (scheduled 워커)
```

### 제약 조건

| 제약               | 출처                                                              | 영향                                  |
| ------------------ | ----------------------------------------------------------------- | ------------------------------------- |
| River 큐 네이밍    | 라이브러리 검증                                                   | 큐 이름에 콜론(`:`) 사용 불가         |
| PostgreSQL 기반 큐 | [ADR-04](/ko/adr/04-queue-based-async-processing.md)              | 기존 River 설정과 통합 필수           |
| 워커 프로세스 분리 | [ADR-05 Worker](/ko/adr/worker/05-worker-scheduler-separation.md) | Worker와 Scheduler 별도 배포          |
| 빌링 티어 통합     | [ADR-13](/ko/adr/13-billing-quota-architecture.md)                | 큐 선택 시 구독 정보의 티어 활용 필수 |

### 결정 배경

ADR-13에서 구독 티어 체계 수립 완료. 그러나 운영상 차별화 부재로 유료 고객이 체감하는 처리 속도 이점 없음. 유료 요금제 가치 제안 약화.

## 결정

**서비스당 3-티어 큐 아키텍처 및 환경 변수 기반 워커 할당 채택.**

### 큐 구조

각 워커 서비스는 티어 기반 라우팅으로 세 개의 큐 유지:

```
{service}_priority   ← Pro / Pro Plus / Enterprise 사용자
{service}_default    ← Free 티어 사용자
{service}_scheduled  ← 백그라운드 스케줄러 작업
```

구체적 큐 이름:

| 서비스        | Priority 큐         | Default 큐         | Scheduled 큐         |
| ------------- | ------------------- | ------------------ | -------------------- |
| Analyzer      | `analysis_priority` | `analysis_default` | `analysis_scheduled` |
| SpecGenerator | `specview_priority` | `specview_default` | `specview_scheduled` |

### 워커 할당 전략

환경 변수를 통한 큐별 워커 수 설정 (합리적 기본값 제공):

**Analyzer 서비스:**

```
ANALYZER_QUEUE_PRIORITY_WORKERS=5   # 워커 용량의 50%
ANALYZER_QUEUE_DEFAULT_WORKERS=3    # 워커 용량의 30%
ANALYZER_QUEUE_SCHEDULED_WORKERS=2  # 워커 용량의 20%
```

**SpecGenerator 서비스:**

```
SPECGEN_QUEUE_PRIORITY_WORKERS=3    # 워커 용량의 50%
SPECGEN_QUEUE_DEFAULT_WORKERS=2     # 워커 용량의 33%
SPECGEN_QUEUE_SCHEDULED_WORKERS=1   # 워커 용량의 17%
```

### 큐 선택 로직

```go
func SelectQueue(baseQueue string, tier PlanTier, isScheduled bool) string {
    if isScheduled {
        return baseQueue + "_scheduled"
    }
    switch tier {
    case PlanTierPro, PlanTierProPlus, PlanTierEnterprise:
        return baseQueue + "_priority"
    default:
        return baseQueue + "_default"
    }
}
```

### 네이밍 규칙

River 검증 규칙 준수를 위해 언더스코어를 구분자로 사용:

- 허용: 영문자, 숫자, 언더스코어, 하이픈
- 금지: 콜론, 공백, 특수문자

초기 설계 `analysis:priority`에서 `analysis_priority`로 변경.

## 검토 옵션

### Option A: 티어 기반 멀티큐 + 환경 변수 워커 할당 (선택)

**설명:**
서비스별 티어별 별도 큐, 환경 변수로 워커 수 설정.

```
analysis_priority   → Priority 워커 (5)
analysis_default    → Default 워커 (3)
analysis_scheduled  → Scheduled 워커 (2)
```

**장점:**

| 이점            | 설명                                                  |
| --------------- | ----------------------------------------------------- |
| 명확한 SLA 경계 | Priority 큐 깊이로 유료 사용자 경험 측정 가능         |
| 독립적 스케일링 | 코드 변경 없이 워커 비율 조정                         |
| 스케줄러 격리   | 백그라운드 작업이 사용자 요청 기아 방지               |
| 모니터링 세분화 | 큐별 메트릭으로 티어별 알림 설정 가능                 |
| 우아한 저하     | Priority 큐 비어있으면 워커 유휴 (우선순위 역전 방지) |

**단점:**

| 트레이드오프     | 완화 방안                                |
| ---------------- | ---------------------------------------- |
| 설정 복잡도      | 합리적 기본값 제공; 필요 시에만 조정     |
| 워커 유휴 용량   | SLA 보장을 위한 수용 가능한 트레이드오프 |
| 모니터링 큐 증가 | 큐별 패널이 있는 통합 대시보드           |

### Option B: 단일 큐 + Priority 필드

**설명:**
서비스당 단일 큐, 각 작업에 `priority` 필드. `ORDER BY priority DESC`로 우선 처리.

```sql
SELECT * FROM river_job
WHERE queue = 'analysis' AND state = 'available'
ORDER BY priority DESC, scheduled_at ASC
LIMIT 1 FOR UPDATE SKIP LOCKED;
```

**장점:**

- 간단한 설정 (서비스당 큐 하나)
- 워커 항상 작업 보유 (유휴 용량 없음)
- 인프라 구성 요소 최소화

**단점:**

| 문제               | 심각도                                    |
| ------------------ | ----------------------------------------- |
| 우선순위 역전 위험 | 높음 - 대량의 무료 작업이 Pro 사용자 지연 |
| 쿼리 복잡도        | 중간 - 핫 테이블에 `ORDER BY`             |
| SLA 보장 불가      | 높음 - 우선 처리 시간 보장 불가           |
| 모니터링 어려움    | 중간 - 단일 큐 깊이로 티어 상태 파악 불가 |

**판정:** 기각. 우선순위 역전이 유료 티어 차별화라는 핵심 비즈니스 목표 훼손.

### Option C: 티어별 워커 인스턴스 분리

**설명:**
티어별 전용 워커 배포:

```
analyzer-priority-worker  (Pro/Enterprise 전용)
analyzer-default-worker   (Free 전용)
analyzer-scheduled-worker (Scheduler 전용)
```

**장점:**

- 완전한 리소스 격리
- 독립적 스케일링 및 배포
- 명확한 보안 경계 설정 가능

**단점:**

| 문제        | 심각도                                   |
| ----------- | ---------------------------------------- |
| 인프라 비용 | 높음 - 서비스당 3배 워커 배포            |
| 용량 낭비   | 높음 - 티어 간 유휴 워커 공유 불가       |
| 운영 복잡도 | 높음 - 6개 이상 서비스 배포/모니터링     |
| 배포 조율   | 중간 - 스키마 변경 시 모든 인스턴스 영향 |

**판정:** 기각. 현재 규모에서 과도한 인프라 오버헤드. 대규모 엔터프라이즈 전용 워커 필요 시 재검토.

## 결과

### 긍정적

**유료 사용자 가치 제안:**

- Pro/Enterprise 사용자가 워커 용량의 50% 전용 할당 경험
- 피크 로드 시 유료 사용자 일관된 처리 시간 유지, 무료 티어는 우아하게 저하
- 요금제 차별화의 명확한 근거

**운영 제어:**

- 배포 없이 환경 변수로 워커 할당 조정 가능
- 티어별 큐 깊이로 사전 스케일링 결정 가능
- 스케줄러 작업이 사용자 대면 지연에 영향 불가

**관측 가능성:**

| 메트릭            | 의미                    |
| ----------------- | ----------------------- |
| Priority 큐 깊이  | 유료 사용자 경험 건강도 |
| Default 큐 깊이   | 무료 티어 대기 시간     |
| Scheduled 큐 깊이 | 백그라운드 작업 백로그  |

### 부정적

**설정 오버헤드:**

- 워커 서비스당 6개 환경 변수 (전체 3개 서비스에 18개)
- 잘못된 비율 설정 시 용량 낭비 또는 유료 경험 저하
- 운영자용 문서화 필요

**워커 유휴 용량:**

- 유료 사용자 트래픽 저조 시 Priority 워커 유휴
- 큐 간 동적 재균형 불가
- SLA 보장을 위한 수용 가능한 트레이드오프

**모니터링 복잡도:**

- 서비스당 3개 큐 (기존 2개 대비 총 6개)
- 대시보드 및 알림 설정 증가
- 디버깅 시 큐 간 상관관계 분석 필요

### 기술적 함의

| 측면        | 함의                                                      |
| ----------- | --------------------------------------------------------- |
| River 설정  | 워커당 3개 항목의 `QueueConfig` 맵                        |
| 큐 네이밍   | 언더스코어 구분자 (`_priority`, `_default`, `_scheduled`) |
| 티어 조회   | API 레이어에서 enqueue 전 구독 정보 조회                  |
| 우아한 저하 | 알 수 없는 티어는 `_default` 큐로 라우팅                  |
| 워커 헬스   | 각 큐별 독립적 워커 풀 상태                               |
| 배포        | 환경 변수로 기본 워커 수 오버라이드                       |

## 참조

- [ADR-04: 큐 기반 비동기 처리](/ko/adr/04-queue-based-async-processing.md) - 큐 아키텍처 기반
- [ADR-05 Worker: Worker-Scheduler 분리](/ko/adr/worker/05-worker-scheduler-separation.md) - 스케줄러 격리 근거
- [ADR-13: 빌링 및 할당량 아키텍처](/ko/adr/13-billing-quota-architecture.md) - 티어 정의 및 빌링 컨텍스트
- [River Queue Documentation](https://riverqueue.com/docs) - 큐 네이밍 제약사항
