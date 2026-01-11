---
title: 리포지토리 패턴
description: Domain 레이어에 정의된 도메인 중심 인터페이스를 통한 데이터 접근 추상화
---

# ADR-07: 리포지토리 패턴 데이터 접근 추상화

> 🇺🇸 [English Version](/en/adr/worker/07-repository-pattern.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2024-12-18 | @KubrickCode | worker     |

## 컨텍스트

### 문제

UseCase 레이어에 직접 데이터베이스 쿼리가 산재되면 여러 문제 발생:

**강한 결합:**

- UseCase가 PostgreSQL 특정 코드(pgx, pgtype)에 직접 의존
- 데이터베이스 벤더 변경 시 비즈니스 로직 수정 필요
- SQL 쿼리가 비즈니스 워크플로우 오케스트레이션과 혼재

**테스트 어려움:**

- 단위 테스트에 데이터베이스 연결 또는 복잡한 모킹 필요
- 비즈니스 로직을 격리하여 테스트 불가
- 간단한 규칙 검증에도 통합 테스트 필요

**코드 구성:**

- 데이터 접근과 비즈니스 로직 사이 명확한 경계 없음
- 쿼리 최적화 관심사가 UseCase로 누출
- 여러 usecase에 쿼리 패턴 중복

### 목표

1. **추상화**: 데이터베이스 구현 세부사항을 UseCase로부터 숨김
2. **테스트 용이성**: 간단한 mock 구현으로 단위 테스트 가능
3. **유지보수성**: 데이터 접근 로직을 전용 레이어에 중앙화
4. **도메인 정렬**: 데이터 작업을 도메인 용어로 표현

## 결정

**Domain 레이어에 정의된 도메인 중심 인터페이스로 Repository 패턴 채택.**

### 인터페이스 설계

```go
// domain/analysis/repository.go
type Repository interface {
    CreateAnalysisRecord(ctx context.Context, params CreateAnalysisRecordParams) (UUID, error)
    RecordFailure(ctx context.Context, analysisID UUID, errMessage string) error
    SaveAnalysisInventory(ctx context.Context, params SaveAnalysisInventoryParams) error
}
```

### 주요 특성

| 항목            | 결정                                            |
| --------------- | ----------------------------------------------- |
| 인터페이스 위치 | Domain 레이어 (`domain/analysis/repository.go`) |
| 구현체 위치     | Adapter 레이어 (`adapter/repository/postgres/`) |
| 트랜잭션 범위   | 메서드 단위 (각 메서드가 원자적)                |
| 파라미터 스타일 | 검증 기능을 가진 Value Object                   |
| 에러 처리       | 도메인 에러 + 래핑된 인프라 에러                |

## 검토한 대안

### 옵션 A: Repository 패턴 (선택됨)

**설명:**

Domain 레이어에 인터페이스 정의, Adapter 레이어에 구현. 각 메서드는 완전하고 원자적인 작업을 표현.

**장점:**

- 도메인 로직과 영속성 간 명확한 분리
- UseCase는 추상화에만 의존
- 단위 테스트를 위한 모킹 용이
- 비즈니스 로직에 영향 없이 구현 변경 가능

**단점:**

- 추가적인 추상화 레이어
- 메서드가 많아지는 "repository 비대화" 위험
- 세분화된 작업과 큰 단위 작업 사이 균형 필요

### 옵션 B: 쿼리 오브젝트 패턴

**설명:**

특정 쿼리를 캡슐화하는 쿼리 객체를 생성하여 범용 실행기에 전달.

**장점:**

- 매우 유연한 쿼리 구성
- 재사용 가능한 쿼리 조각

**단점:**

- 더 복잡한 API 표면
- 쿼리 객체가 영속성 세부사항을 노출할 수 있음
- 데이터 흐름 이해 어려움

### 옵션 C: Active Record 패턴

**설명:**

도메인 객체가 자체 영속성 메서드를 포함.

**장점:**

- CRUD 작업에 간단하고 직관적
- 작은 도메인에서 적은 코드

**단점:**

- 도메인 객체가 영속성 로직으로 무거워짐
- 단일 책임 원칙 위반
- 도메인과 인프라 간 강한 결합
- 도메인 로직을 격리하여 테스트하기 어려움

## 구현 원칙

### Domain 레이어의 인터페이스 정의

인터페이스는 구현되는 곳이 아닌 **사용되는 곳**에 정의:

```
domain/
  analysis/
    repository.go      ← 인터페이스 정의 (Repository)
    autorefresh.go     ← 확장 인터페이스 (AutoRefreshRepository)

adapter/
  repository/
    postgres/
      analysis.go      ← PostgreSQL 구현체
```

**근거:**

- Domain 레이어가 인프라 의존성 없이 유지
- 의존성 역전: 상위 레벨 모듈이 계약 정의
- 구현 세부사항이 Adapter 레이어에 격리

### Value Object 파라미터

원시 타입 파라미터 대신 검증된 Value Object 사용:

```go
type CreateAnalysisRecordParams struct {
    AnalysisID *UUID    // 선택: 제공된 ID 사용 또는 새로 생성
    Branch     string
    CommitSHA  string
    Owner      string
    Repo       string
}

func (p CreateAnalysisRecordParams) Validate() error {
    if p.Owner == "" {
        return fmt.Errorf("%w: owner is required", ErrInvalidInput)
    }
    // ... 검증 로직
}
```

**장점:**

- 자기 문서화되는 메서드 시그니처
- 검증 로직이 데이터와 함께 위치
- API를 깨지 않고 쉽게 확장
- 필수 필드와 선택 필드의 명확한 구분

### 메서드 단위 트랜잭션 범위

각 Repository 메서드는 완전하고 원자적인 작업:

```go
func (r *AnalysisRepository) CreateAnalysisRecord(ctx context.Context, params ...) (UUID, error) {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return NilUUID, fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)  // 안전: 커밋되면 no-op

    // ... 트랜잭션 내 작업

    if err := tx.Commit(ctx); err != nil {
        return NilUUID, fmt.Errorf("commit transaction: %w", err)
    }
    return result, nil
}
```

**근거:**

- 단순한 멘탈 모델: 각 메서드는 완전히 성공하거나 실패
- 메서드 경계를 넘는 트랜잭션 누출 없음
- UseCase가 트랜잭션 라이프사이클 관리 불필요
- Context 취소 자동 처리

### 에러 메시지 Truncation

긴 에러 메시지는 저장 전 잘라냄:

```go
const maxErrorMessageLength = 1000

func truncateErrorMessage(msg string) string {
    if len(msg) <= maxErrorMessageLength {
        return msg
    }
    return msg[:maxErrorMessageLength-15] + "... (truncated)"
}
```

**근거:**

- 데이터베이스 컬럼에 크기 제한 있음
- 과대 데이터로 인한 쿼리 실패 방지
- 에러 메시지의 유용한 부분 보존
- 잘림 발생 표시

### 외부 Analysis ID 지원

Repository는 선택적으로 외부 제공 ID 지원:

```go
type CreateAnalysisRecordParams struct {
    AnalysisID *UUID  // nil이면 새 UUID 생성; 제공되면 사용
    // ...
}

// 구현
analysisID := analysis.NewUUID()
if params.AnalysisID != nil {
    analysisID = *params.AnalysisID
}
```

**사용 사례:**

- Web 서비스가 알려진 ID로 Analysis 레코드 생성
- Worker가 작업 페이로드에서 이 ID 수신
- Worker가 결과 저장 시 동일 ID 사용
- 시스템 간 상관관계 추적 가능

## 결과

### 긍정적

**테스트 용이성:**

```go
// 단위 테스트를 위한 쉬운 모킹
type MockRepository struct {
    CreateAnalysisRecordFn func(...) (UUID, error)
}

func (m *MockRepository) CreateAnalysisRecord(...) (UUID, error) {
    return m.CreateAnalysisRecordFn(...)
}
```

**유연성:**

- PostgreSQL을 다른 데이터베이스로 교체 가능
- 구현 변경이 UseCase 테스트에 영향 없음
- 캐싱 레이어를 투명하게 추가 가능

**유지보수성:**

- 모든 SQL 쿼리가 한 위치에
- 쿼리 최적화가 Adapter 레이어에 격리
- 명확한 책임 경계

### 부정적

**추상화 오버헤드:**

- 추가적인 인터페이스와 구현 파일
- params와 DB 구조체 사이 일부 중복
- 도메인과 영속성 모델 간 매핑 필요

**메서드 증가:**

- 새로운 데이터 접근 패턴에 새 메서드 필요
- Repository가 "신 객체(god object)"가 될 위험
- 특화된 repository로 분리 필요할 수 있음

**트랜잭션 제한:**

- 메서드 간 트랜잭션은 인터페이스에서 지원 안 함
- 복잡한 워크플로우는 Adapter 레이어에서 조율 필요

## 참조

- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md) - 전체 레이어 구조
- [Repository Pattern by Martin Fowler](https://martinfowler.com/eaaCatalog/repository.html)
- [Domain-Driven Design by Eric Evans](https://www.domainlanguage.com/ddd/) - Repository 패턴 출처
