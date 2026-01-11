---
title: APIHandlers 합성 패턴
description: 여러 도메인 핸들러를 단일 StrictServerInterface 구현으로 합성하는 ADR
---

# ADR-12: APIHandlers 합성 패턴

> [English Version](/en/adr/web/12-apihandlers-composition-pattern.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-01-03 | @KubrickCode | web        |

## 배경

### 단일 인터페이스 제약

oapi-codegen의 strict-server 모드는 하나의 구조체가 구현해야 하는 단일 `StrictServerInterface` 생성. 이는 각 도메인 모듈(analyzer, auth, user, github 등)이 Clean Architecture 계층 내에서 자체 핸들러 구현을 유지하는 Feature-Based Module Organization([ADR-11](/ko/adr/web/11-feature-based-module-organization.md))과 긴장 발생.

**문제:**

- `StrictServerInterface`가 하나의 인터페이스에 모든 API 엔드포인트 정의 (~20개 이상 메서드)
- Feature 기반 모듈은 별도의 핸들러 패키지 보유 (`modules/analyzer/handler/`, `modules/auth/handler/` 등)
- 각 모듈의 핸들러는 자신의 도메인 로직만 인지
- 서버는 모든 인터페이스 메서드를 구현하는 단일 구조체 필요

**합성 없이는:**

```go
// 불가능: 각 핸들러는 메서드 일부만 구현
var _ StrictServerInterface = (*AnalyzerHandler)(nil)  // auth 메서드 누락
var _ StrictServerInterface = (*AuthHandler)(nil)      // analyzer 메서드 누락
```

초기 구현은 모든 메서드를 가진 단일 핸들러 구조체 사용. 코드베이스가 성장하고 Feature-Based Module Organization을 채택하면서, 단일 인터페이스 요구사항을 충족하면서 도메인 분리를 유지하기 위해 합성 패턴 필요.

## 결정

**다중 도메인 핸들러를 단일 StrictServerInterface 구현으로 합성하기 위해 APIHandlers 합성 패턴 채택.**

### 패턴 구조

```go
// 도메인별 핸들러 인터페이스 (handlers.go)
type AnalyzerHandlers interface {
    AnalyzeRepository(ctx context.Context, request AnalyzeRepositoryRequestObject) (AnalyzeRepositoryResponseObject, error)
    GetAnalysisStatus(ctx context.Context, request GetAnalysisStatusRequestObject) (GetAnalysisStatusResponseObject, error)
}

type AuthHandlers interface {
    AuthCallback(ctx context.Context, request AuthCallbackRequestObject) (AuthCallbackResponseObject, error)
    AuthLogin(ctx context.Context, request AuthLoginRequestObject) (AuthLoginResponseObject, error)
    // ...
}

// 전체 인터페이스를 구현하는 합성 구조체
type APIHandlers struct {
    analyzer AnalyzerHandlers
    auth     AuthHandlers
    bookmark BookmarkHandlers
    // ...
}

var _ StrictServerInterface = (*APIHandlers)(nil)  // 컴파일타임 검사

// 도메인 핸들러로 위임
func (h *APIHandlers) AnalyzeRepository(ctx context.Context, request AnalyzeRepositoryRequestObject) (AnalyzeRepositoryResponseObject, error) {
    return h.analyzer.AnalyzeRepository(ctx, request)
}
```

### 핵심 원칙

1. **도메인별 인터페이스**: 각 핸들러 인터페이스는 해당 도메인과 관련된 메서드만 포함
2. **단일 합성 구조체**: `APIHandlers`가 모든 도메인 핸들러를 집계
3. **위임 패턴**: 각 메서드가 적절한 도메인 핸들러로 위임
4. **컴파일타임 검증**: `var _ StrictServerInterface = (*APIHandlers)(nil)`가 완전성 보장

## 고려한 옵션

### 옵션 A: APIHandlers 합성 패턴 (선택됨)

**작동 방식:**

- StrictServerInterface 메서드 부분집합과 일치하는 도메인별 핸들러 인터페이스 정의
- 모든 도메인 핸들러를 보유하는 합성 `APIHandlers` 구조체 생성
- 적절한 핸들러로 위임하여 StrictServerInterface 구현
- 의존성 주입을 사용하여 `app.go`에서 모든 것을 연결

**장점:**

- **도메인 격리**: 각 핸들러는 자신의 도메인만 인지
- **독립적 테스트**: 다른 의존성 없이 도메인 핸들러 테스트 가능
- **명확한 소유권**: 각 모듈이 자체 핸들러 구현 소유
- **컴파일타임 안전성**: 누락된 구현이 빌드 시 포착
- **확장성**: 새 도메인 추가 시 새 인터페이스 + 핸들러만 필요

**단점:**

- 위임 메서드에 대한 추가 보일러플레이트
- 하나의 간접 참조 계층 추가
- 인터페이스 정의가 OpenAPI 스펙과 동기화 유지 필요

### 옵션 B: 단일 모놀리식 핸들러

**작동 방식:**

- 모든 StrictServerInterface 메서드를 구현하는 하나의 큰 핸들러 구조체
- 모든 UseCase 의존성이 단일 구조체에 주입
- 위임 없이 직접 메서드 구현

**장점:**

- 위임 계층 없는 더 단순한 구조
- 유지보수할 파일 수 감소
- 직접적인 메서드 구현

**단점:**

- **단일 책임 원칙 위반**: 하나의 구조체가 모든 도메인 처리
- **테스트 복잡성**: 모든 테스트에 모든 의존성 모킹 필요
- **확장성 문제**: API 확장에 따라 파일이 무제한 성장
- **낮은 응집도**: 관련 없는 비즈니스 로직이 하나의 파일에 혼합
- **Feature-Based 조직과 충돌**: 모듈 경계 훼손

### 옵션 C: 런타임 라우터 디스패치

**작동 방식:**

- 경로 접두사로 핸들러를 동적 등록
- 라우터가 런타임에 적절한 핸들러로 디스패치
- 각 핸들러가 부분 인터페이스 구현

**장점:**

- 핸들러 등록을 위한 최대 유연성
- 인터페이스 동기화 불필요

**단점:**

- **컴파일타임 안전성 없음**: 누락된 핸들러가 런타임에서만 발견
- **복잡한 등록 로직**: 에러 발생하기 쉬운 핸들러 와이어링
- **StrictServerInterface 목적 무력화**: 타입 안전성 이점 상실
- **디버깅 어려움**: 디스패치 에러 추적 어려움

### 옵션 D: 합성을 위한 코드 생성

**작동 방식:**

- 핸들러 인터페이스에서 합성 계층 생성
- 인터페이스 정의 기반으로 위임 메서드 자동 생성

**평가:**

- 추가 도구 복잡성
- 커스텀 코드 생성 유지보수 부담
- 패턴이 충분히 단순해서 수동 구현 허용됨
- **거부**: 현재 규모에서 오버헤드가 정당화되지 않음

## 구현

### 핸들러 인터페이스 정의

도메인 핸들러 인터페이스는 생성된 `server.gen.go`에 인접한 `internal/api/handlers.go`에 정의:

```
internal/api/
├── handlers.go      # 도메인 핸들러 인터페이스 + APIHandlers 합성
└── server.gen.go    # 생성된 StrictServerInterface
```

### 애플리케이션에서 와이어링

`common/server/app.go`에서 핸들러 생성 및 합성:

```go
func initHandlers(container *infra.Container) (*Handlers, error) {
    // 도메인 핸들러 생성
    analyzerHandler := analyzerhandler.NewHandler(...)
    authHandler := authhandler.NewHandler(...)
    userHandler := userhandler.NewHandler(...)
    githubHandler := githubhandler.NewHandler(...)

    // 단일 인터페이스로 합성
    apiHandlers := api.NewAPIHandlers(
        analyzerHandler,
        userHandler,      // AnalysisHistoryHandlers
        authHandler,
        userHandler,      // BookmarkHandlers
        githubHandler,
        githubAppHandler,
        analyzerHandler,  // RepositoryHandlers
        webhookHandler,
    )

    return &Handlers{API: apiHandlers}, nil
}
```

### 특수 케이스

**선택적 핸들러:**

일부 핸들러는 조건부로 사용 가능 (예: GitHub App이 구성된 경우에만):

```go
func (h *APIHandlers) GetGitHubAppInstallURL(ctx context.Context, request GetGitHubAppInstallURLRequestObject) (GetGitHubAppInstallURLResponseObject, error) {
    if h.githubApp == nil {
        return GetGitHubAppInstallURL500ApplicationProblemPlusJSONResponse{
            InternalErrorApplicationProblemPlusJSONResponse: NewInternalError("GitHub App not configured"),
        }, nil
    }
    return h.githubApp.GetGitHubAppInstallURL(ctx, request)
}
```

**Raw HTTP 핸들러:**

raw HTTP 접근이 필요한 엔드포인트(webhooks)는 별도 인터페이스 사용:

```go
type WebhookHandlers interface {
    HandleGitHubAppWebhookRaw(w http.ResponseWriter, r *http.Request)
}

// 특수 접근자 메서드를 통해 접근
func (h *APIHandlers) WebhookHandler() WebhookHandlers {
    return h.webhook
}
```

## 결과

### 긍정적

**도메인 분리:**

- 각 도메인 핸들러가 해당 모듈의 `handler/` 패키지에 존재
- 도메인별 로직이 다른 도메인과 격리
- 한 도메인의 변경이 다른 도메인에 영향 주지 않음

**테스트 용이성:**

- 모킹된 의존성으로 도메인 핸들러를 독립적으로 테스트
- 단위 테스트에 전체 API 계층 인스턴스화 불필요
- 통합 테스트에서 실제 합성 또는 부분 모킹 사용 가능

**확장성:**

- 새 도메인 추가: 인터페이스 정의, 핸들러 구현, 합성에 추가
- 새 엔드포인트 추가: 적절한 도메인 핸들러에 구현
- 합성이 새 메서드를 위임하지 않으면 컴파일타임 에러

**Clean Architecture 정합:**

- [ADR-08](/ko/adr/web/08-clean-architecture-pattern.md)에 따라 핸들러 계층이 명확히 분리
- [ADR-11](/ko/adr/web/11-feature-based-module-organization.md)에 따라 Feature-Based 모듈 유지
- [ADR-10](/ko/adr/web/10-strict-server-interface-contract.md)에 따라 StrictServerInterface 계약 보존

### 부정적

**보일러플레이트:**

- 각 StrictServerInterface 메서드에 위임 메서드 필요
- `handlers.go`에 ~20개 이상의 한 줄 메서드
- **완화**: 메서드가 단순함; IDE가 쉽게 생성; 거의 변경 없음

**인터페이스 동기화:**

- 도메인 인터페이스가 StrictServerInterface 시그니처 부분집합과 일치해야 함
- API 엔드포인트 추가 시 도메인 인터페이스 업데이트 필요
- **완화**: 컴파일타임 에러가 불일치 즉시 포착

**멘탈 모델:**

- 개발자가 합성 계층 존재를 이해해야 함
- 디버깅 시 위임 추적 필요
- **완화**: 패턴이 단순함; CLAUDE.md에 문서화

## 참고자료

- [Composite 패턴 - 디자인 패턴](https://refactoring.guru/design-patterns/composite)
- [인터페이스 분리 원칙 - SOLID](https://en.wikipedia.org/wiki/Interface_segregation_principle)
- [oapi-codegen Strict Server](https://github.com/oapi-codegen/oapi-codegen)
- [ADR-08: Clean Architecture 패턴](/ko/adr/web/08-clean-architecture-pattern.md)
- [ADR-10: StrictServerInterface 계약](/ko/adr/web/10-strict-server-interface-contract.md)
- [ADR-11: Feature-Based 모듈 조직](/ko/adr/web/11-feature-based-module-organization.md)
