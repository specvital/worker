---
title: StrictServerInterface 계약
description: 컴파일타임 API 계약 강제를 위한 oapi-codegen strict-server 모드 채택 ADR
---

# ADR-10: StrictServerInterface 계약

> [English Version](/en/adr/web/10-strict-server-interface-contract.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-01-03 | @KubrickCode | web        |

## 배경

### API 계약 문제

API 개발에는 명세와 구현 사이의 근본적인 긴장 존재. OpenAPI 명세와 핸들러 구현이 독립적으로 진화할 때 여러 문제 발생:

**수동 동기화:**

- 핸들러 함수 시그니처를 API 명세와 수동으로 맞춰야 함
- 누락되거나 잘못된 파라미터가 런타임에서만 발견됨
- 응답 타입 불일치가 잘못된 JSON을 조용히 생성

**런타임 vs 컴파일타임 에러:**

- 전통적인 핸들러는 raw `http.ResponseWriter`와 `*http.Request` 사용
- 타입 에러가 API 테스트나 프로덕션에서만 드러남
- 파라미터 추출이나 응답 포매팅에 컴파일러 지원 없음

**핸들러-명세 드리프트:**

- API 파라미터 추가 시 OpenAPI 스펙과 핸들러 코드 모두 업데이트 필요
- 오퍼레이션 이름 변경이 조용히 연결을 끊음
- 응답 상태 코드가 타입 시스템으로 강제되지 않음

### OpenAPI-First 타입 생성

프로젝트는 oapi-codegen을 통한 OpenAPI-First 개발 채택:

| 변경 사항                              | 동기                        |
| -------------------------------------- | --------------------------- |
| OpenAPI 기반 타입 생성 파이프라인 설정 | API 타입의 단일 진실 공급원 |
| strict-server 모드 활성화              | 컴파일타임 API 계약 검증    |
| APIHandlers 합성 패턴 도입             | 다중 도메인 핸들러 관리     |

초기 타입 생성은 타입을 생성했지만 여전히 수동 핸들러 와이어링 필요. strict-server 개선은 컴파일타임 강제 도입.

## 결정

**컴파일타임 API 계약 강제를 위해 oapi-codegen strict-server 모드 채택.**

`api/oapi-codegen.yaml` 설정:

```yaml
package: api
output: internal/api/server.gen.go
generate:
  models: true
  chi-server: true
  strict-server: true
```

strict-server 옵션은 타입이 지정된 요청/응답 객체를 가진 `StrictServerInterface` 생성, 모든 API 엔드포인트가 올바르게 구현되었는지 컴파일타임에 검증.

### 핸들러 구현 패턴

모든 HTTP 핸들러는 생성된 `StrictServerInterface` 구현:

```go
// 생성된 인터페이스 (server.gen.go)
type StrictServerInterface interface {
    GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error)
    // ... 모든 엔드포인트
}

// 구현 검증 (handlers.go)
var _ StrictServerInterface = (*APIHandlers)(nil)
```

이 컴파일타임 어설션은 구현이 OpenAPI 명세와 정확히 일치함을 보장.

## 고려한 옵션

### 옵션 A: StrictServerInterface (선택됨)

**작동 방식:**

- oapi-codegen이 타입이 지정된 시그니처를 가진 `StrictServerInterface` 생성
- 각 엔드포인트는 강력히 타입 지정된 `RequestObject`를 받고 `ResponseObject`를 반환
- 핸들러 래퍼가 HTTP와 타입 지정된 인터페이스 사이를 변환
- 컴파일러가 인터페이스 구현 완전성을 강제

**함수 시그니처 비교:**

```diff
// ServerInterface (non-strict)
-GetAnalysis(w http.ResponseWriter, r *http.Request, owner string, repo string)

// StrictServerInterface (strict)
+GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error)
```

**장점:**

- **컴파일타임 강제**: 누락된 엔드포인트가 빌드 실패 유발
- **타입 안전성**: 요청 파라미터와 응답 본문이 타입 지정됨
- **명시적 에러 처리**: 에러 반환이 명시적 처리 요구
- **Context 전파**: 취소/타임아웃을 위한 `context.Context` 명시적 전달
- **HTTP 추상화**: 비즈니스 로직에서 직접적인 `http.ResponseWriter` 조작 없음

**단점:**

- 생성 코드 의존성 (OpenAPI 변경 후 `just gen-api` 실행 필요)
- HTTP와 핸들러 사이에 추가 추상화 계층
- 생성된 요청/응답 타입에 대한 학습 곡선

### 옵션 B: 표준 ServerInterface

**작동 방식:**

- oapi-codegen이 raw HTTP 핸들러를 가진 `ServerInterface` 생성
- 생성된 코드가 `*http.Request`에서 파라미터 추출
- `http.ResponseWriter`를 통한 수동 응답 작성 처리

**함수 시그니처:**

```go
GetAnalysis(w http.ResponseWriter, r *http.Request, owner string, repo string)
```

**장점:**

- 고급 사용 사례를 위한 직접적인 HTTP 제어
- 익숙한 Go HTTP 핸들러 패턴
- 약간 적은 생성 코드

**단점:**

- 컴파일러가 응답 타입을 강제하지 않음
- 에러가 발생하기 쉬운 상태 코드로 수동 JSON 직렬화
- 비즈니스 로직에 HTTP 관심사 혼합
- 응답 타입 정확성에 대한 컴파일타임 검사 없음

### 옵션 C: 수동 핸들러 구현

**작동 방식:**

- 코드 생성 없이 핸들러 작성
- 파라미터를 수동으로 추출하고 요청 검증
- 응답을 수동으로 구성하고 작성

**장점:**

- 모든 측면에 대한 완전한 제어
- 코드 생성 의존성 없음
- 생성된 타입에 대한 학습 곡선 없음

**단점:**

- 컴파일타임 계약 강제 없음
- 명세-구현 드리프트 불가피
- OpenAPI와 Go 간 중복 타입 정의
- 수동 파라미터 추출이 에러 발생하기 쉬움

## 구현

### Request Object 패턴

각 엔드포인트의 요청은 생성된 구조체에 캡슐화:

```go
// 생성된 요청 객체
type GetAnalysisRequestObject struct {
    Owner string
    Repo  string
}

// 핸들러는 강력히 타입 지정된 요청 수신
func (h *Handler) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
    result, err := h.usecase.Execute(ctx, usecase.GetAnalysisInput{
        Owner: request.Owner,
        Repo:  request.Repo,
    })
    // ...
}
```

### Response Object 패턴

응답은 특정 응답 타입을 가진 유니온 타입 패턴 사용:

```go
// 생성된 응답 인터페이스
type GetAnalysisResponseObject interface {
    VisitGetAnalysisResponse(w http.ResponseWriter) error
}

// 구체적인 응답 타입
type GetAnalysis200JSONResponse AnalysisResult
type GetAnalysis404ApplicationProblemPlusJSONResponse ProblemDetail

// 핸들러가 특정 응답 타입 반환
func (h *Handler) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
    result, err := h.usecase.Execute(ctx, input)

    switch {
    case errors.Is(err, domain.ErrNotFound):
        return GetAnalysis404ApplicationProblemPlusJSONResponse{
            Status: 404,
            Title:  "Not Found",
            Detail: "Analysis not found",
        }, nil
    case err != nil:
        return nil, err
    }

    return GetAnalysis200JSONResponse(*result), nil
}
```

### APIHandlers 합성

여러 도메인 핸들러가 단일 `StrictServerInterface` 구현으로 합성:

```go
type APIHandlers struct {
    analyzer        AnalyzerHandlers
    auth            AuthHandlers
    bookmark        BookmarkHandlers
    // ...
}

var _ StrictServerInterface = (*APIHandlers)(nil)

func (h *APIHandlers) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
    return h.analyzer.GetAnalysis(ctx, request)
}
```

이 패턴은 HTTP 서버를 위한 단일 인터페이스를 유지하면서 도메인별 핸들러 허용.

## 결과

### 긍정적

**컴파일타임 API 계약:**

- 구현 없이 새 엔드포인트 추가 시 컴파일 실패
- 요청 파라미터 변경 시 핸들러 시그니처 업데이트 강제
- 빌드 타임에 응답 타입 불일치 포착
- 핸들러-명세 드리프트로 인한 런타임 서프라이즈 없음

**타입 안전성:**

- 생성된 코드가 요청 파라미터를 추출하고 타입 지정
- 응답 본문이 OpenAPI 스키마 정의와 일치
- 에러 응답이 일관된 ProblemDetail 구조 사용
- 핸들러에서 수동 JSON 마샬링 없음

**Clean Architecture 정합:**

- 핸들러가 HTTP 세부사항이 아닌 요청/응답 매핑에 집중
- UseCase 계층의 비즈니스 로직이 타입 지정된 입력 수신
- 도메인 에러가 타입 지정된 HTTP 응답으로 매핑
- API 계약과 구현 사이의 명확한 분리

**개발자 경험:**

- 요청/응답 타입에 대한 IDE 자동완성
- 컴파일러 에러가 API 구현을 안내
- 모든 엔드포인트에 걸쳐 일관된 핸들러 패턴

### 부정적

**코드 생성 의존성:**

- OpenAPI 변경 후 `just gen-api` 실행 필수
- CI에서 생성된 코드가 최신인지 검증 필요
- **완화**: pre-commit 훅 또는 생성된 파일 최신성 CI 검사

**생성 코드 볼륨:**

- `server.gen.go`가 3000줄 이상의 생성 코드
- Request/Response 타입이 바이너리 크기를 약간 증가
- **완화**: 타입 안전성의 비용으로 수용; 코드 리뷰에서 제외

**학습 곡선:**

- 팀이 생성된 타입 패턴을 이해해야 함
- 응답 유니온 타입 학습 필요
- **완화**: CLAUDE.md에 패턴 문서화; 예제 제공

**Webhook 예외:**

- 일부 엔드포인트(GitHub webhooks)는 raw HTTP 접근 필요
- 모든 사용 사례가 strict 인터페이스 패턴에 맞지 않음
- **완화**: 별도의 raw 핸들러를 가진 `WebhookHandlers` 인터페이스

## 참고자료

- [oapi-codegen GitHub 저장소](https://github.com/oapi-codegen/oapi-codegen)
- [oapi-codegen Strict Server 문서](https://github.com/oapi-codegen/oapi-codegen/blob/main/README.md#strict-server)
- [ADR-08: Clean Architecture 패턴](/ko/adr/web/08-clean-architecture-pattern.md)
