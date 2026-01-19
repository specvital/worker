---
title: 익명 사용자 Rate Limiting
description: Analyzer API에 대한 익명 사용자용 IP 기반 인메모리 Rate Limiting ADR
---

# ADR-21: 익명 사용자 Rate Limiting

> [English Version](/en/adr/web/21-anonymous-rate-limiting.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2026-01-15 | @KubrickCode | web        |

## Context

### 문제 상황

Analyzer API는 인증 없이 공개 접근 가능하여 익명 사용자의 플랫폼 탐색 허용. 그러나 다음과 같은 남용 위험 존재:

| 위험              | 영향                  | 가능성 |
| ----------------- | --------------------- | ------ |
| API 남용/스크래핑 | 서비스 성능 저하      | 중간   |
| 리소스 고갈       | 유료 사용자 영향 중단 | 중-저  |
| 비용 증폭         | 컴퓨팅 비용 증가      | 중간   |

### 기존 아키텍처

플랫폼의 2단계 사용자 경험:

- **인증된 사용자**: 빌링 서비스를 통한 쿼터 시스템 관리 ([ADR-13](/ko/adr/13-billing-quota-architecture.md))
- **익명 사용자**: 본 결정 이전에는 보호 메커니즘 부재

잠재 고객 전환을 위한 플랫폼 탐색을 허용하면서 익명 사용자에 대한 스로틀링 필요.

### 제약 조건

| 제약 조건                 | 출처   | 영향                               |
| ------------------------- | ------ | ---------------------------------- |
| 단일 인스턴스 배포        | 인프라 | 분산 상태 불필요                   |
| PostgreSQL 중심           | ADR-04 | Redis 추가 시 새 의존성 도입       |
| PaaS 우선 전략            | ADR-06 | 단순하고 자체 완결적 솔루션 선호   |
| 인증된 사용자는 쿼터 보유 | ADR-13 | Rate Limiting은 익명 사용자만 대상 |

## Decision

**Analyzer API에 대한 익명 사용자용 IP 기반 인메모리 Rate Limiting 구현.**

설정:

| 파라미터 | 값            |
| -------- | ------------- |
| 알고리즘 | Fixed window  |
| 제한     | 10 요청       |
| 윈도우   | 1분           |
| 키       | 클라이언트 IP |
| 범위     | Analyzer API  |
| 대상     | 익명 사용자만 |

구현 패턴:

```go
userID := middleware.GetUserID(ctx)

if userID == "" && h.anonymousRateLimiter != nil {
    clientIP := middleware.GetClientIP(ctx)
    if !h.anonymousRateLimiter.Allow(clientIP) {
        return 429 Response
    }
}
// 인증된 사용자는 Rate Limiting 우회
```

응답 형식은 RFC 7807 Problem Details 준수:

```json
{
  "status": 429,
  "title": "Too Many Requests",
  "detail": "Rate limit exceeded. Please sign in for higher limits or try again later."
}
```

## Options Considered

### Option A: 인메모리 Fixed Window (선택됨)

**동작 방식:**

- IP별 요청 카운터가 매 분 윈도우마다 리셋
- Go map에 저장, 백그라운드 정리 고루틴 동작
- 외부 의존성 없음

**장점:**

- 인프라 의존성 제로
- 단순 구현, 예측 가능한 동작
- 배포 변경 없이 즉시 사용 가능

**단점:**

- 단일 인스턴스 전용 (수평 확장 미지원)
- 재시작 시 상태 손실 (배포 시 제한 리셋)
- Fixed window 경계 버스트 (최악의 경우: 윈도우 경계에서 20 요청)

### Option B: Redis 기반 분산

**동작 방식:**

- Redis에 중앙 집중식 카운터 저장
- TTL과 함께 원자적 증가

**장점:**

- 수평 확장을 위한 다중 인스턴스 지원
- 재시작 간 상태 지속
- 대규모 검증 완료

**단점:**

- 새 인프라 의존성 (ADR-06 위반)
- 요청당 네트워크 지연 오버헤드
- 운영 복잡성 (Redis 모니터링, 페일오버)

**결정:** 현재 단일 인스턴스 배포에서는 기각. 다중 인스턴스 확장 시 재검토.

### Option C: Cloud WAF / API Gateway

**동작 방식:**

- Cloudflare 또는 API Gateway 레벨에서 Rate Limiting 설정
- 애플리케이션 도달 전 엣지에서 적용

**장점:**

- 애플리케이션 코드 변경 없음
- 글로벌 엣지 적용
- DDoS 보호 포함

**단점:**

- 엣지에서 인증/익명 사용자 구분 불가
- 애플리케이션 레벨보다 조잡한 세분화
- 외부 의존성, 요청당 비용

**결정:** 심층 방어 계층으로 유지, 주요 솔루션으로는 부적합.

### Option D: 애플리케이션 레벨 제한 없음

**동작 방식:**

- 인프라 보호(Cloudflare)에만 의존
- 애플리케이션 레벨 스로틀링 없음

**장점:**

- 가장 단순한 접근, 이미 배포됨

**단점:**

- 애플리케이션 시맨틱(사용자 유형) 이해 불가
- 모든 사용자 동등 처리, 2단계 경험과 충돌

**결정:** 사용자 유형별 동작 제공 불가로 기각.

## Implementation

### Rate Limiter 컴포넌트

```
src/backend/
├── common/
│   ├── ratelimit/
│   │   └── limiter.go       # Fixed window IPRateLimiter
│   ├── middleware/
│   │   └── ratelimit.go     # Token bucket 미들웨어 (대안)
│   └── httputil/
│       └── client_ip.go     # IP 추출
└── modules/analyzer/
    └── handler/http.go      # Rate limiter 통합
```

### IP 추출 우선순위

1. `X-Forwarded-For` 헤더 (목록의 첫 번째 IP)
2. `X-Real-IP` 헤더
3. `RemoteAddr` (폴백)

신뢰할 수 있는 리버스 프록시(Railway, Cloudflare) 뒤 배포 가정하에 프록시 헤더 신뢰.

### 초기화

```go
// app.go
anonymousRateLimiter := ratelimit.NewIPRateLimiter(10, time.Minute)
closers = append(closers, anonymousRateLimiter) // Graceful shutdown
```

## Consequences

### Positive

**1. 외부 의존성 제로**

- Redis 또는 외부 서비스 불필요
- ADR-06 PaaS 우선 전략 부합
- 배포 및 운영 단순화

**2. 남용 방지**

- 익명 남용으로부터 플랫폼 리소스 보호
- 인증된 사용자를 위한 공정한 접근 보장
- 비용 증폭 위험 감소

**3. 명확한 사용자 경험**

- 익명 사용자에게 제한 존재 안내
- 에러 메시지로 인증 유도
- 인증된 사용자는 제한 완전 우회

### Negative

**1. 단일 인스턴스 제한**

- 수평 확장 미지원
- **마이그레이션 경로:** 다중 인스턴스 배포 필요 시 Redis 기반 솔루션 구현

**2. 상태 미지속**

- 애플리케이션 재시작 시 제한 리셋
- **영향:** 1분 윈도우에서는 수용 가능; 제한이 빠르게 복구

**3. IP 기반 식별 한계**

- 공유 IP(NAT, 기업 네트워크)에서 오탐
- **영향:** 낮은 제한(10/분)이 정상 탐색에 거의 영향 없음
- **완화:** 사용자는 인증하여 더 높은 제한 획득 가능

### 트레이드오프 요약

| 트레이드오프            | 결정         | 근거                                     |
| ----------------------- | ------------ | ---------------------------------------- |
| 단순성 vs. 확장성       | 단순성 우선  | 현재 단일 인스턴스; 확장 시 재검토       |
| IP 정확도 vs. 구현 비용 | IP 한계 수용 | NAT 오탐이 탐색 계층에서 수용 가능       |
| 메모리 vs. 외부 의존성  | 메모리 우선  | 익명 사용자 수에 대해 인메모리 수용 가능 |

## References

- [ADR-13: 빌링 및 쿼터 아키텍처](/ko/adr/13-billing-quota-architecture.md) - 인증된 사용자를 위한 쿼터 시스템
- [ADR-13 (Web): 도메인 에러 처리 패턴](/ko/adr/web/13-domain-error-handling-pattern.md) - RateLimitError 커스텀 타입
- [GitHub Issue #207](https://github.com/specvital/web/issues/207) - 구현 추적
- [Commit 107f387](https://github.com/specvital/web/commit/107f387) - 구현 커밋
