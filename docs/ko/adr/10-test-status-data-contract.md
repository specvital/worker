---
title: TestStatus 데이터 계약
description: 크로스 서비스 TestStatus 열거형 정렬을 통한 데이터 무결성 ADR
---

# ADR-10: TestStatus 데이터 계약

> 🇺🇸 [English Version](/en/adr/10-test-status-data-contract)

| 날짜       | 작성자       | 리포지토리        |
| ---------- | ------------ | ----------------- |
| 2024-12-29 | @KubrickCode | core, worker, web |

## 컨텍스트

### 데이터 흐름 문제

Specvital은 멀티 서비스 파이프라인을 통해 테스트 메타데이터 처리:

```
Core Parser → Worker → Database → Web API → Frontend
```

각 서비스가 자체 `TestStatus` 타입을 정의하여 발생 가능한 문제:

- **데이터 손실**: 변환 중 상태 값 누락
- **의미론적 드리프트**: 동일 enum 값이 다른 의미
- **조용한 실패**: 매핑 오류가 감지되지 않음

### 발생한 문제

개발 중 심각한 데이터 무결성 이슈 발견:

```
Core 정의:      active, focused, skipped, todo, xfail (5개 상태)
Worker 정의: active, skipped, todo (3개 상태)
```

**영향**:

- `focused` 테스트가 `active`로 잘못 매핑
- `xfail` 테스트가 `todo`로 잘못 매핑
- 사용자에게 부정확한 테스트 카운트 및 누락된 상태 표시

### 중요성

| 상태    | 의미론적 의미                   | 손실 시 사용자 영향  |
| ------- | ------------------------------- | -------------------- |
| active  | 정상 테스트, 실행됨             | 기준선, 영향 없음    |
| focused | 디버그 전용 테스트 (.only, fit) | CI 경고 미발생       |
| skipped | 의도적으로 제외                 | 잘못된 skip 카운트   |
| todo    | 플레이스홀더, 미구현            | TODO 추적 누락       |
| xfail   | 실패 예상 (pytest xfail)        | 부정확한 실패 예상치 |

## 결정

**모든 서비스에서 손실 없는 1:1 TestStatus 매핑 강제**

### 정규 상태 정의

모든 서비스는 정확히 이 5개 상태를 지원해야 함:

```go
// 정규 TestStatus enum (출처: core)
type TestStatus string

const (
    TestStatusActive  TestStatus = "active"   // 정상 테스트
    TestStatusFocused TestStatus = "focused"  // .only, fit - 디버그 모드
    TestStatusSkipped TestStatus = "skipped"  // .skip, xit - 제외됨
    TestStatusTodo    TestStatus = "todo"     // 플레이스홀더 테스트
    TestStatusXfail   TestStatus = "xfail"    // 실패 예상
)
```

### 서비스 정렬

| 서비스   | 위치                                    | 상태              |
| -------- | --------------------------------------- | ----------------- |
| Core     | `pkg/domain/status.go`                  | 소스 오브 트루스  |
| Worker   | `internal/domain/analysis/inventory.go` | Core에서 1:1 매핑 |
| Database | schema.sql의 `test_status` ENUM         | 1:1 매핑          |
| Web API  | OpenAPI `TestStatus` 스키마             | 1:1 매핑          |

## 고려된 대안

### 옵션 A: 문자열 패스스루 (기각)

- enum 검증 없이 원시 문자열로 상태 전달
- **기각 이유**: 컴파일 타임 안전성 없음, 오타로 인한 조용한 실패

### 옵션 B: 서브셋 매핑 (이전 상태)

- Worker가 단순화된 3-상태 모델 사용
- `focused → active`, `xfail → todo` 매핑
- **기각 이유**: 데이터 손실, 의미론적 손상

### 옵션 C: 엄격한 1:1 매핑 (선택됨)

- 모든 서비스가 동일한 enum 값 정의
- 모든 케이스를 포함하는 명시적 switch 문
- 알 수 없는 값은 panic/error (fail-fast)

## 구현

### Core (소스 오브 트루스)

```go
// pkg/domain/status.go
type TestStatus string

const (
    TestStatusActive  TestStatus = "active"
    TestStatusSkipped TestStatus = "skipped"
    TestStatusTodo    TestStatus = "todo"
    TestStatusFocused TestStatus = "focused"
    TestStatusXfail   TestStatus = "xfail"
)
```

### Worker (소비자)

```go
// internal/domain/analysis/inventory.go
type TestStatus string

const (
    TestStatusActive  TestStatus = "active"
    TestStatusFocused TestStatus = "focused"
    TestStatusSkipped TestStatus = "skipped"
    TestStatusTodo    TestStatus = "todo"
    TestStatusXfail   TestStatus = "xfail"
)
```

### 매핑 레이어

```go
// internal/adapter/mapping/core_domain.go
func convertCoreTestStatus(coreStatus domain.TestStatus) analysis.TestStatus {
    switch coreStatus {
    case domain.TestStatusFocused:
        return analysis.TestStatusFocused
    case domain.TestStatusSkipped:
        return analysis.TestStatusSkipped
    case domain.TestStatusTodo:
        return analysis.TestStatusTodo
    case domain.TestStatusXfail:
        return analysis.TestStatusXfail
    default:
        return analysis.TestStatusActive
    }
}
```

### 데이터베이스 스키마

```sql
CREATE TYPE public.test_status AS ENUM (
    'active',
    'skipped',
    'todo',
    'focused',
    'xfail'
);
```

### Web API (OpenAPI)

```yaml
TestStatus:
  type: string
  enum:
    - active
    - focused
    - skipped
    - todo
    - xfail
  description: |
    테스트 상태 표시자:
    - active: 실행될 정상 테스트
    - focused: 단독 실행 표시 테스트 (예: it.only)
    - skipped: 건너뛰기 표시 테스트 (예: it.skip)
    - todo: 구현 예정 플레이스홀더 테스트
    - xfail: 실패 예상 테스트 (pytest xfail)
```

## 결과

### 긍정적

**데이터 무결성**:

- 파이프라인에서 정보 손실 없음
- 모든 상태 유형에 대한 정확한 테스트 카운트
- focused 테스트에 대한 신뢰할 수 있는 CI 경고

**타입 안전성**:

- 타입 enum으로 컴파일 타임 검증
- 명시적 매핑으로 조용한 실패 방지
- 상태 값에 대한 IDE 자동완성

**API 명확성**:

- 프론트엔드가 정확한 상태 정보 수신
- 모든 엔드포인트에서 일관된 동작
- 자체 문서화되는 enum 값

### 부정적

**조정 오버헤드**:

- 새 상태 추가 시 4곳 모두 변경 필요:
  - Core: `pkg/domain/status.go`
  - Worker: `internal/domain/analysis/inventory.go`
  - Database: ENUM 변경 마이그레이션
  - Web: OpenAPI 스키마 업데이트
- 배포 중 버전 불일치 위험

**스키마 진화**:

- PostgreSQL ENUM 변경은 마이그레이션 필요
- 상태 값 쉽게 제거 불가 (deprecate만 가능)
- ENUM 값 순서가 저장에 영향

## 향후 변경 가이드라인

### 새 상태 추가

1. Core `pkg/domain/status.go`에 먼저 추가
2. Worker 도메인 및 매핑 레이어에 추가
3. ENUM 추가를 위한 데이터베이스 마이그레이션 생성
4. OpenAPI 스키마 업데이트
5. 순서대로 배포: Database → Worker → Web → Core

### 상태 폐기

1. 문서에서 deprecated로 표시
2. Worker에서 deprecated 상태를 대체 상태로 매핑
3. deprecated 상태 출력을 중단하도록 파서 업데이트
4. 마이그레이션 기간 후 OpenAPI에서 제거

## 참조

- [PostgreSQL ENUM Types](https://www.postgresql.org/docs/current/datatype-enum.html)
- [OpenAPI Enum Best Practices](https://swagger.io/docs/specification/data-models/enums/)
