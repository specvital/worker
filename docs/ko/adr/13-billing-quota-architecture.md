---
title: 빌링 및 쿼터 아키텍처
description: 이벤트 기반 사용량 추적, 구독 티어, 큐 우선순위화에 대한 ADR
---

# ADR-13: 빌링 및 쿼터 아키텍처

> 🇺🇸 [English Version](/en/adr/13-billing-quota-architecture.md)

| 날짜       | 작성자       | 리포지토리         |
| ---------- | ------------ | ------------------ |
| 2026-01-18 | @KubrickCode | web, worker, infra |

## 배경

### 수익화 요구사항

Specvital 테스트 분석 플랫폼의 수익화를 위한 빌링 및 쿼터 시스템 필요. 4개 구독 티어: free, pro, pro_plus, enterprise.

**핵심 요구사항:**

| 요구사항            | 설명                                    |
| ------------------- | --------------------------------------- |
| 사용량 추적         | SpecView 및 Analysis 작업의 정확한 추적 |
| 캐시 인식 빌링      | 캐시 히트는 쿼터 미소비                 |
| 감사 준수           | 리소스 삭제 시에도 사용 기록 보존       |
| 공정한 쿼터 기간    | 가입일과 관계없이 전체 구독 가치 제공   |
| 티어 차별화         | 유료 사용자에게 더 빠른 처리 제공       |
| 엔터프라이즈 무제한 | 최상위 티어의 효과적인 무제한 사용      |

### 제약사항

| 제약사항          | 영향                                                   |
| ----------------- | ------------------------------------------------------ |
| PostgreSQL 백엔드 | River 큐가 PostgreSQL 사용 (ADR-04); 솔루션 통합 필수  |
| 캐시 우선 모델    | SpecView는 캐시된 결과를 무료로 제공; 미스만 쿼터 소비 |
| 익명 접근         | 플랫폼에서 속도 제한이 있는 익명 탐색 허용             |
| 다중 리포지토리   | 솔루션이 web, worker, infra 리포지토리에 걸쳐 적용     |

## 결정

**이벤트 기반 사용량 추적, 롤링 쿼터 기간, 티어별 큐 분리 우선순위화 채택.**

### 1. 이벤트 기반 사용량 추적

작업 완료 시점에 `usage_events` 테이블에 사용량 이벤트 기록:

```sql
table usage_events {
  id: uuid
  user_id -> users
  event_type: specview | analysis
  analysis_id -> analyses? (ON DELETE SET NULL)
  document_id -> spec_documents? (ON DELETE SET NULL)
  quota_amount: int
  created_at: timestamptz
}
```

**핵심 특성:**

- 성공적 완료 시에만 이벤트 기록
- 캐시 히트는 이벤트 미생성
- `ON DELETE SET NULL`로 감사 추적 보존
- 효율적 쿼터 조회를 위한 월별 집계 인덱스
- `quota_amount`에 SpecView의 테스트 케이스 수 저장

### 2. 구독 플랜 아키텍처

NULL이 무제한을 나타내는 4단계 구조:

```sql
table subscription_plans {
  tier: free | pro | pro_plus | enterprise
  monthly_price: int?
  specview_monthly_limit: int?
  analysis_monthly_limit: int?
  retention_days: int?
}
```

```sql
table user_subscriptions {
  user_id -> users
  plan_id -> subscription_plans (ON DELETE RESTRICT)
  status: active | canceled | expired
  current_period_start: timestamptz
  current_period_end: timestamptz
}
```

**제약조건:**

- 부분 유니크 인덱스로 사용자당 하나의 활성 구독 보장
- 사용자 가입 시 무료 플랜 자동 할당
- 롤링 기간: 활성화 날짜로부터 정확히 1개월

### 3. 티어 기반 큐 우선순위화

서비스당 3개 큐와 티어 기반 라우팅:

| 큐           | 구독자                    | 목적                  |
| ------------ | ------------------------- | --------------------- |
| `_priority`  | Pro, Pro Plus, Enterprise | 유료 사용자 빠른 처리 |
| `_default`   | Free                      | 표준 처리             |
| `_scheduled` | System                    | 예약 재분석           |

**서비스별 큐 이름:**

```
Analysis Service:
  analysis_priority   → Pro/Enterprise 사용자
  analysis_default    → Free 티어 사용자
  analysis_scheduled  → 스케줄러/크론 작업

SpecView Service:
  specview_priority   → Pro/Enterprise 사용자
  specview_default    → Free 티어 사용자
  specview_scheduled  → 스케줄러/크론 작업
```

**큐 선택 로직:**

```go
// common/queue/selector.go (web repository)
func SelectQueue(baseQueue string, tier PlanTier, isScheduled bool) string {
    if isScheduled {
        return baseQueue + SuffixScheduled  // "_scheduled"
    }
    switch tier {
    case PlanTierPro, PlanTierProPlus, PlanTierEnterprise:
        return baseQueue + SuffixPriority   // "_priority"
    default:
        return baseQueue + SuffixDefault    // "_default"
    }
}
```

**요청 흐름:**

```
Handler → TierLookup → UseCase → QueueService → SelectQueue
   │           │           │           │              │
   │     GetUserTier()     │     Enqueue()      큐 이름 계산
   │           │           │           │              │
   └───────────┴───────────┴───────────┴──────────────┘
```

**Graceful Degradation:**

- 빈 userID 또는 nil tierLookup → `_default`로 라우팅
- 티어 조회 시 데이터베이스 오류 → 경고 로그, `_default`로 라우팅
- 구독 레코드 없음 → 빈 티어로 처리, `_default`로 라우팅

**워커 할당 (환경 변수로 설정 가능):**

```
Analyzer:
  ANALYZER_QUEUE_PRIORITY_WORKERS=5   (기본값)
  ANALYZER_QUEUE_DEFAULT_WORKERS=3    (기본값)
  ANALYZER_QUEUE_SCHEDULED_WORKERS=2  (기본값)

SpecGen:
  SPECGEN_QUEUE_PRIORITY_WORKERS=3    (기본값)
  SPECGEN_QUEUE_DEFAULT_WORKERS=2     (기본값)
  SPECGEN_QUEUE_SCHEDULED_WORKERS=1   (기본값)
```

### 4. 익명 사용자 속도 제한

고정 윈도우 방식의 IP 기반 인메모리 속도 제한:

- IP당 분당 10개 요청
- analyze API의 익명 사용자에만 적용
- 인메모리 저장 (외부 의존성 없음)

## 검토한 옵션

### A. 사용량 추적 전략

| 옵션                           | 결론                              |
| ------------------------------ | --------------------------------- |
| 완료 시 이벤트 기반 **(선택)** | 감사 친화적, 캐시 정렬, 실패 안전 |
| 실시간 차감                    | 경쟁 조건, 복잡한 환불 로직       |
| 외부 미터링 (Stripe/Orb)       | 외부 의존성, 지연, 대규모 비용    |

**선택 근거:**

이벤트 기반 추적은 기존 PostgreSQL 중심 아키텍처와 정렬되며 캐시 미스만 쿼터를 소비하는 캐시 우선 모델에 자연스럽게 적합.

### B. 쿼터 기간 전략

| 옵션                           | 결론                                   |
| ------------------------------ | -------------------------------------- |
| 가입 시점 롤링 기간 **(선택)** | 모든 사용자에게 공정, 예측 가능한 갱신 |
| 캘린더 월                      | 월말 가입자에게 불공정                 |
| 커스텀 빌링 사이클             | 최대 복잡성, 지원 부담                 |

**선택 근거:**

롤링 기간은 사용자 공정성 우선. 28일에 가입하는 사용자도 캘린더 리셋까지 3일이 아닌 전체 1개월 쿼터 수령.

### C. 큐 우선순위 전략

| 옵션                      | 결론                                   |
| ------------------------- | -------------------------------------- |
| 티어별 큐 분리 **(선택)** | 명확한 격리, SLA 친화적, 모니터링 용이 |
| 단일 큐 + 우선순위 필드   | 우선순위 역전 위험, 복잡한 쿼리        |
| 전용 워커 풀              | 최고 인프라 비용, 용량 공유 불가       |

**선택 근거:**

분리된 큐는 공유 워커 풀을 통한 효율성을 허용하면서 더 깔끔한 SLA 보장 제공.

## 크로스 서비스 아키텍처

```
┌─────────────────────────────────────────────────────────────────────┐
│                      빌링 & 쿼터 흐름                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐     ┌─────────────────────┐     ┌──────────────┐  │
│  │   Web API   │────▶│  Quota Check API    │◀───▶│  PostgreSQL  │  │
│  │  (Go + Chi) │     │  POST /usage/check  │     │              │  │
│  └──────┬──────┘     │  GET /usage/current │     │  테이블:     │  │
│         │            └─────────────────────┘     │  - users     │  │
│         │                                        │  - subscript │  │
│         ▼                                        │    ion_plans │  │
│  ┌──────────────┐    ┌─────────────────────┐    │  - user_sub  │  │
│  │ 큐 선택      │───▶│  River Queue        │    │    scriptions│  │
│  │ (티어 기반)  │    │  - :priority        │    │  - usage_    │  │
│  └──────────────┘    │  - :default         │    │    events    │  │
│                      │  - :scheduled       │    └──────────────┘  │
│                      └──────────┬──────────┘                       │
│                                 │                                  │
│                                 ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                     Worker Service                           │  │
│  │  ┌───────────────┐     ┌───────────────┐                    │  │
│  │  │   Analyzer    │     │  SpecView Gen │                    │  │
│  │  │   Worker      │     │    Worker     │                    │  │
│  │  └───────┬───────┘     └───────┬───────┘                    │  │
│  │          │                     │                             │  │
│  │          ▼                     ▼                             │  │
│  │  usage_event 기록       usage_event 기록                    │  │
│  │  (type: analysis)      (type: specview)                     │  │
│  │                        (캐시 미스 시에만)                   │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## 결과

### 긍정적

| 영역                | 이점                                      |
| ------------------- | ----------------------------------------- |
| 빌링 정확성         | 성공적이고 캐시되지 않은 작업만 쿼터 소비 |
| 감사 준수           | `SET NULL`로 완전한 사용 기록 보존        |
| 사용자 공정성       | 롤링 기간으로 전체 구독 가치 보장         |
| 유료 티어 경험      | 큐 분리로 처리 우선순위 보장              |
| 운영 유연성         | 환경 변수로 워커 할당 조정 가능           |
| 엔터프라이즈 단순성 | NULL 값으로 무제한 깔끔하게 표현          |

### 부정적

| 영역             | 트레이드오프                                              |
| ---------------- | --------------------------------------------------------- |
| 쿼터 가시성      | 사용자가 제출 시가 아닌 작업 완료 후 사용량 업데이트 확인 |
| 리포팅 복잡성    | 롤링 기간이 코호트 분석 복잡화                            |
| 스토리지 증가    | 이벤트 기반 추적으로 시간 경과에 따른 레코드 축적         |
| 큐 모니터링      | 서비스당 3개 큐로 옵저버빌리티 설정 증가                  |
| 비례 배분 복잡성 | 주기 중간 플랜 변경 시 쿼터 조정 필요                     |

### 기술적 시사점

| 측면                | 시사점                                                      |
| ------------------- | ----------------------------------------------------------- |
| 데이터베이스 스키마 | 월별 집계 인덱스가 있는 `usage_events` 테이블               |
| 쿼리 패턴           | 인덱싱된 `created_at`에 대한 집계 쿼리로 월별 사용량 조회   |
| 워커 설정           | 환경 변수로 워커-큐 비율 제어                               |
| 속도 제한           | IP 기반 인메모리 제한으로 익명 사용자 처리 (분당 10개 요청) |
| 플랜 전환           | 부분 유니크 인덱스로 사용자당 단일 활성 구독 보장           |

## 참조

- [ADR-04: 큐 기반 비동기 처리](/ko/adr/04-queue-based-async-processing.md)
- [ADR-12: Worker 중심 분석 라이프사이클](/ko/adr/12-worker-centric-analysis-lifecycle.md)
- [관련 커밋](https://github.com/specvital/infra/commits/main) - 구독 및 사용량 추적 데이터베이스 스키마
