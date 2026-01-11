---
title: 도메인 에러 처리 패턴
description: 도메인 에러와 HTTP 상태 코드 분리를 위한 센티널 에러 패턴 ADR
---

# ADR-13: 도메인 에러 처리 패턴

> [English Version](/en/adr/web/13-domain-error-handling-pattern.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-01-03 | @KubrickCode | web        |

## 배경

### 초기 아키텍처 문제

초기 서비스 계층은 비즈니스 결과와 함께 HTTP 상태 코드를 직접 반환:

```go
// Before: HTTP를 인식하는 서비스 계층
func (s *Service) GetAnalysis(...) (api.Response, int, error) {
    if analysis == nil {
        return nil, http.StatusNotFound, errors.New("not found")
    }
    return response, http.StatusOK, nil
}
```

이는 Clean Architecture의 의존성 규칙 위반—내부 계층(도메인/서비스)은 외부 계층(HTTP 전송)을 알면 안 됨.

### HTTP 결합의 문제점

**관심사 분리 위반:**

- 서비스 계층이 비즈니스 로직과 전송 관심사를 혼합
- REST에서 gRPC로 전환 시 서비스 계층 재작성 필요
- 비즈니스 규칙이 상태 코드 결정과 얽힘

**테스트 복잡성:**

- 테스트가 비즈니스 로직과 HTTP 상태 코드 모두 검증 필요
- 목 설정에 HTTP 시맨틱 이해 필요
- 상태 코드 어설션이 비즈니스 로직 테스트를 어지럽힘

**다중 핸들러 비일관성:**

- 서로 다른 핸들러가 같은 에러를 다른 상태 코드로 매핑할 수 있음
- 도메인 에러 시맨틱에 대한 단일 진실의 원천 없음

## 결정

**핸들러 계층 HTTP 매핑과 함께 센티널 에러 패턴 채택.**

### 핵심 원칙

1. **도메인의 센티널 에러**: 각 모듈이 `domain/errors.go`에 `var ErrXxx = errors.New(...)`를 정의
2. **`errors.Is()`로 에러 검사**: 핸들러가 분류를 위해 `errors.Is(err, domain.ErrXxx)` 사용
3. **핸들러에서만 HTTP 매핑**: 오직 핸들러 계층만 HTTP 상태 코드를 알고 있음
4. **컨텍스트 래핑**: `fmt.Errorf("context: %w", err)`로 에러 타입을 보존하면서 컨텍스트 추가

### 센티널 에러 카테고리

| 카테고리       | 예시                                                     | HTTP 매핑 |
| -------------- | -------------------------------------------------------- | --------- |
| **Not Found**  | `ErrNotFound`, `ErrUserNotFound`, `ErrCodebaseNotFound`  | 404       |
| **Validation** | `ErrInvalidCursor`, `ErrInvalidState`                    | 400       |
| **Auth**       | `ErrUnauthorized`, `ErrTokenExpired`, `ErrNoGitHubToken` | 401       |
| **Permission** | `ErrAccessDenied`, `ErrInsufficientScope`                | 401/403   |
| **Rate Limit** | `RateLimitError` (커스텀 타입)                           | 429       |
| **Conflict**   | `ErrAlreadyQueued`                                       | 409       |

### 커스텀 에러 타입

추가 데이터가 필요한 에러에는 커스텀 에러 타입:

```go
type RateLimitError struct {
    Limit     int
    Remaining int
    ResetAt   time.Time
}

func (e *RateLimitError) Error() string { ... }

// errors.As()로 검사
func IsRateLimitError(err error) bool {
    var rateLimitErr *RateLimitError
    return errors.As(err, &rateLimitErr)
}
```

## 고려한 옵션

### 옵션 A: 핸들러 매핑과 함께 센티널 에러 (선택됨)

**작동 방식:**

- 도메인 계층이 `var ErrXxx = errors.New("...")`를 정의
- UseCase가 도메인 에러 또는 래핑된 도메인 에러 반환
- 핸들러가 `errors.Is()`로 분류하고 HTTP 상태로 매핑

**장점:**

- 도메인 계층에 전송 의존성 없음
- `errors.Is()`가 타입 안전한 에러 검사 제공
- 에러 래핑이 컨텍스트 보존 (`fmt.Errorf("%w", err)`)
- 모든 모듈에서 일관된 패턴
- Go 1.13+ 에러 처리 관용구와 정합

**단점:**

- 각 핸들러에서 명시적 매핑 필요
- 관리되지 않으면 에러 증식 위험
- 많은 에러 케이스로 핸들러 코드가 장황해짐

### 옵션 B: 도메인/서비스에서 HTTP 상태 코드

**작동 방식:**

- 서비스가 `(result, httpStatus, error)` 튜플 반환
- 핸들러가 반환된 상태 코드를 직접 사용

**장점:**

- 더 단순한 핸들러 코드
- 비즈니스 로직에서 직접 상태 코드

**단점:**

- Clean Architecture 의존성 규칙 위반
- 도메인 계층이 HTTP 전송에 묶임
- HTTP가 아닌 전송(gRPC, CLI)에 도메인 로직 재사용 불가
- 서비스 테스트에 HTTP 지식 필요

### 옵션 C: 에러 코드 열거형

**작동 방식:**

- 도메인에 숫자/문자열 에러 코드 정의
- 핸들러에서 에러 코드를 HTTP 상태로 매핑

**장점:**

- 명시적인 에러 카탈로그
- 문서화 용이

**단점:**

- 센티널 에러보다 Go 관용적이지 않음
- 추가 매핑 레이어 필요
- 코드 값에서 에러 타입 정보 손실

### 옵션 D: panic/recover 예외 스타일

**작동 방식:**

- 도메인에서 타입화된 에러로 panic
- 미들웨어에서 recover하고 매핑

**장점:**

- 더 깔끔한 해피 패스 코드
- 자동 전파

**단점:**

- Go 관용적이지 않음
- 스택 해제 오버헤드
- 복구 범위 제어 어려움
- 호출자에게 예상치 못한 동작

## 구현

### 모듈 에러 정의

각 모듈이 자체 `domain/errors.go` 유지:

```
modules/
├── analyzer/domain/errors.go    # ErrNotFound, ErrInvalidCursor
├── auth/domain/errors.go        # ErrUserNotFound, ErrTokenExpired, ...
├── github/domain/errors.go      # ErrUnauthorized, RateLimitError
├── github-app/domain/errors.go  # ErrInstallationNotFound, ...
└── user/domain/errors.go        # ErrCodebaseNotFound, ErrInvalidCursor
```

### UseCase 에러 전파

UseCase는 도메인 에러를 반환하거나 컨텍스트와 함께 래핑:

```go
func (uc *GetAnalysisUseCase) Execute(ctx context.Context, input Input) (*Result, error) {
    analysis, err := uc.repo.GetByOwnerRepo(ctx, input.Owner, input.Repo)
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            return nil, err  // 그대로 전파
        }
        return nil, fmt.Errorf("get analysis: %w", err)  // 예상치 못한 에러 래핑
    }
    return &Result{Analysis: analysis}, nil
}
```

### 핸들러 에러 매핑

핸들러가 도메인 에러를 HTTP 응답으로 매핑:

```go
func (h *Handler) GetAnalysis(ctx context.Context, req Request) (Response, error) {
    result, err := h.getAnalysis.Execute(ctx, input)
    if err != nil {
        switch {
        case errors.Is(err, domain.ErrNotFound):
            return api.GetAnalysis404JSONResponse{...}, nil
        case errors.Is(err, domain.ErrInvalidCursor):
            return api.GetAnalysis400JSONResponse{...}, nil
        default:
            h.logger.Error(ctx, "unexpected error", "error", err)
            return api.GetAnalysis500JSONResponse{...}, nil
        }
    }
    return api.GetAnalysis200JSONResponse{...}, nil
}
```

### 모듈 간 에러 처리

UseCase가 다른 모듈의 port에 의존할 때 모듈 간 에러 처리:

```go
// analyzer/usecase/helper.go
token, err := uc.tokenProvider.GetGitHubToken(ctx, userID)
if err != nil {
    // analyzer 컨텍스트에서 auth 모듈 에러 처리
    if errors.Is(err, authdomain.ErrUserNotFound) ||
       errors.Is(err, authdomain.ErrNoGitHubToken) {
        return nil, domain.ErrNoGitHubToken  // 로컬 도메인 에러로 번역
    }
    return nil, fmt.Errorf("get github token: %w", err)
}
```

## 결과

### 긍정적

**도메인 독립성:**

- 도메인 계층에 전송 의존성 없음
- gRPC, CLI 또는 기타 전송에 도메인 로직 재사용 가능
- 비즈니스 규칙과 전달 메커니즘의 깔끔한 분리

**타입 안전한 에러 처리:**

- `errors.Is()`와 `errors.As()`가 컴파일 타임 안전성 제공
- 에러 분류에 문자열 비교 없음
- 에러 래핑이 전체 컨텍스트 체인 보존

**모듈 간 일관성:**

- 모든 모듈이 같은 패턴 따름: `domain/errors.go` + 핸들러 매핑
- 예측 가능한 에러 처리 코드 구조
- 확립된 패턴을 따라 새 에러 타입 추가 용이

**테스트 용이성:**

- UseCase 테스트가 HTTP 코드가 아닌 도메인 에러 반환 검증
- 핸들러 테스트가 에러-상태 매핑에 집중
- 비즈니스 로직과 전송 테스트 간 명확한 경계

### 부정적

**핸들러 장황함:**

- 각 핸들러가 에러 분류 switch 구현 필요
- 다수의 에러 케이스가 반복적인 코드 유발
- **완화**: 공통 에러 매핑을 헬퍼 함수로 추출

**에러 증식 위험:**

- 거버넌스 없이 새 센티널 에러 추가 용이
- 너무 많은 에러는 시맨틱 명확성 감소
- **완화**: 모듈당 5-7개 에러로 제한; 새 에러 신중히 검토

**모듈 간 복잡성:**

- 핸들러가 여러 모듈의 에러 검사 필요할 수 있음
- 모듈 간 에러 번역이 코드 추가
- **완화**: 문서화된 에러 계약과 함께 명확한 port 인터페이스 정의

## 참고자료

- [Go Blog: Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
- [ADR-08: Clean Architecture 패턴](/ko/adr/web/08-clean-architecture-pattern.md)
- [Worker ADR-02: Clean Architecture 계층](/ko/adr/worker/02-clean-architecture-layers.md)
