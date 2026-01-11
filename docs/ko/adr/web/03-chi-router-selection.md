---
title: Chi 라우터 선택
description: 표준 라이브러리 호환성을 중시한 Go 백엔드 HTTP 라우터로 Chi 선택에 관한 ADR
---

# ADR-03: Chi 라우터 선택

> [English Version](/en/adr/web/03-chi-router-selection.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2024-12-03 | @KubrickCode | web    |

## 배경

### 라우터 선택 문제

Go가 백엔드 언어로 선택되면서([ADR-01](/ko/adr/web/01-go-backend-language.md) 참조), REST API를 위한 HTTP 라우터 필요. 핵심 요구사항:

1. **OpenAPI 호환성**: 타입 안전 API 생성을 위해 `oapi-codegen`과 통합 필수
2. **표준 라이브러리 정렬**: `net/http` 호환 솔루션 선호
3. **미들웨어 조합성**: 인증, CORS, 로깅, 레이트 리미팅을 위한 모듈식 미들웨어 체인
4. **낮은 학습 곡선**: JavaScript 프레임워크에 익숙한 풀스택 개발자의 마찰 최소화
5. **최소 의존성**: 의존성 트리를 간결하게 유지

### 기존 아키텍처 제약사항

- **OpenAPI-First**: `openapi.yaml` → `oapi-codegen` → Go 서버 핸들러
- **Clean Architecture**: 핸들러, 유스케이스, 어댑터, 도메인 분리
- **BFF 패턴**: Next.js 프론트엔드가 Go 백엔드 호출; Go가 모든 비즈니스 로직 처리
- **배포**: Railway (Go 서버) + Vercel (Next.js) + Neon (PostgreSQL)

### 평가 후보

1. **Chi**: `net/http` 기반 경량 라우터, 모듈식 미들웨어
2. **Gin**: 커스텀 컨텍스트를 가진 고성능 프레임워크
3. **Echo**: 자체 미들웨어 스택을 가진 풀 기능 프레임워크
4. **Fiber**: `fasthttp` 기반 Express 스타일 프레임워크

## 결정

**Go 표준 라이브러리 인터페이스에 철저히 준수하는 Chi(go-chi/chi v5)를 HTTP 라우터로 채택.**

핵심 원칙:

1. **net/http 네이티브**: 모든 핸들러가 `http.Handler`와 `http.HandlerFunc` 사용
2. **제로 프레임워크 락인**: 모든 `net/http` 미들웨어가 어댑터 없이 작동
3. **oapi-codegen 통합**: `HandlerFromMux()`를 통한 네이티브 지원
4. **설정보다 조합**: 필요한 것만 정확히 구축

## 검토된 옵션

### 옵션 A: Chi (선택됨)

**작동 방식:**

- `net/http`의 얇은 래퍼
- 미들웨어가 표준 `func(http.Handler) http.Handler` 시그니처 사용
- 라우터가 `http.Handler` 인터페이스 구현
- `context.WithValue()`를 통한 컨텍스트 값 (Go 표준)

**장점:**

- **표준 라이브러리 호환**: 모든 기존 `net/http` 미들웨어가 직접 작동
- **oapi-codegen 네이티브**: `chi-server` 생성 타겟을 통한 일급 통합
- **최소 API 표면**: 라우터 + 미들웨어 패턴만; 그 이상 없음
- **테스트 단순성**: 표준 `httptest` 패키지가 어댑터 없이 작동
- **낮은 의존성 수**: `net/http`와 `context`에만 의존

**단점:**

- 풀 프레임워크보다 "배터리 포함" 적음
- 내장 요청 바인딩/검증 없음 (oapi-codegen이 처리)
- 수동 응답 헬퍼 (생성된 코드가 처리)

### 옵션 B: Gin

**작동 방식:**

- 커스텀 `*gin.Context`가 요청/응답 래핑
- 미들웨어가 `gin.HandlerFunc` 시그니처 사용
- 고성능 래딕스 트리 라우터
- 내장 JSON 바인딩, 검증, 응답 헬퍼

**평가:**

- **컨텍스트 결합**: `*gin.Context`가 모든 핸들러에서 프레임워크 의존성 생성
- **미들웨어 비호환**: `net/http` 미들웨어가 `WrapH()` 어댑터 필요
- **oapi-codegen 지원**: 작동하지만 핸들러가 표준 인터페이스가 아닌 `*gin.Context` 수신
- **오버헤드**: 불필요한 기능(바인딩, 검증)이 oapi-codegen에 의해 이미 제공됨
- **기각**: 프레임워크 락인이 편의 기능보다 큼

### 옵션 C: Echo

**작동 방식:**

- 커스텀 `echo.Context` 인터페이스
- 클로저 기반 미들웨어 `echo.MiddlewareFunc`
- 내장 라우팅, 바인딩, 검증
- 자체 컨텍스트 풀링으로 좋은 성능

**평가:**

- **컨텍스트 결합**: Gin과 유사, 전체에 커스텀 컨텍스트
- **미들웨어 어댑터**: `net/http` 미들웨어가 `echo.WrapHandler()` 필요
- **기능 중복**: 바인딩/검증이 oapi-codegen과 중복
- **기각**: Gin과 동일한 락인 문제

### 옵션 D: Fiber

**작동 방식:**

- `net/http` 대신 `fasthttp` 기반 구축
- Express.js 스타일 API
- 원시 성능 벤치마크 최고
- 모든 작업에 커스텀 `*fiber.Ctx`

**평가:**

- **API 비호환**: `fasthttp` 시그니처가 `net/http`와 완전히 다름
- **에코시스템 고립**: 어떤 `net/http` 미들웨어도 사용 불가
- **HTTP/2 & HTTP/3**: 미지원 (fasthttp 한계)
- **메모리 관리**: 수동 라이프사이클 제어 필요; 누수 위험
- **oapi-codegen**: 작동하지만 어댑터 레이어 오버헤드 있음
- **초기 고려**: 프로젝트 초기 설정에서 원래 권장됨
- **기각**: 에코시스템 비호환; HTTP/2 격차; 메모리 관리 복잡성

## 구현 세부사항

### 라우터 설정

```go
// cmd/server/main.go
func newRouter(...) *chi.Mux {
    r := chi.NewRouter()

    // 표준 미들웨어 체인
    r.Use(chimiddleware.RequestID)
    r.Use(chimiddleware.RealIP)
    r.Use(middleware.Logger())
    r.Use(chimiddleware.Recoverer)
    r.Use(middleware.SecurityHeaders())
    r.Use(middleware.CORS(origins))
    r.Use(chimiddleware.Timeout(apiTimeout))
    r.Use(middleware.Compress())
    r.Use(authMiddleware.OptionalAuth)

    // 인증 엔드포인트용 레이트 리미팅
    authLimiter := middleware.NewIPRateLimiter(authRateLimit)
    r.Route("/api/auth", func(authRouter chi.Router) {
        authRouter.Use(middleware.RateLimit(authLimiter))
    })

    // oapi-codegen 생성 핸들러
    strictHandler := api.NewStrictHandler(apiHandler, nil)
    api.HandlerFromMux(strictHandler, r)

    return r
}
```

### 미들웨어 아키텍처

| 미들웨어        | 소스           | 목적                         |
| --------------- | -------------- | ---------------------------- |
| RequestID       | chi/middleware | 로그 간 요청 상관관계        |
| RealIP          | chi/middleware | 프록시 뒤 클라이언트 IP 추출 |
| Logger          | custom         | slog로 구조화된 로깅         |
| Recoverer       | chi/middleware | 패닉을 500 응답으로 복구     |
| SecurityHeaders | custom         | HSTS, X-Frame-Options, CSP   |
| CORS            | go-chi/cors    | 교차 출처 리소스 공유        |
| Timeout         | chi/middleware | 요청 타임아웃 적용           |
| Compress        | custom         | gzip 응답 압축               |
| OptionalAuth    | custom         | 존재 시 JWT 검증             |
| RateLimit       | custom         | IP 기반 레이트 리미팅        |

### oapi-codegen 통합

```yaml
# oapi-codegen.yaml
generate:
  chi-server: true
  strict-server: true
  models: true
output: internal/api/server.gen.go
```

생성된 코드 제공:

- `StrictServerInterface`: 타입 안전 핸들러 시그니처
- `HandlerFromMux()`: Chi 라우터에 모든 라우트 등록
- 컴파일 타임 요청/응답 검증

### RouteRegistrar 패턴

```go
// common/server/registrar.go
type RouteRegistrar interface {
    RegisterRoutes(r chi.Router)
}

// 모듈식 라우트 등록 활성화
func (h *HealthHandler) RegisterRoutes(r chi.Router) {
    r.Get("/health", h.Check)
}
```

## 결과

### 긍정적

**표준 라이브러리 정렬:**

- 핸들러가 모든 `net/http` 호환 라우터로 이식 가능
- 모든 Go HTTP 테스트 패턴이 변경 없이 작동
- 전체 Go 에코시스템의 미들웨어 호환

**OpenAPI 통합:**

- oapi-codegen Chi 지원이 성숙하고 잘 문서화됨
- StrictServerInterface가 컴파일 타임 API 계약 검증 보장
- 라우트 등록에 제로 보일러플레이트

**개발자 경험:**

- 최소 API 표면이 학습 곡선 감소
- 표준 라이브러리 지식이 직접 적용 가능
- 라우팅과 비즈니스 로직의 명확한 분리

**유지보수성:**

- 작은 의존성 풋프린트 (chi v5.2.0, go-chi/cors v1.2.1)
- 시맨틱 버저닝으로 활발한 유지보수
- 이해해야 할 커스텀 추상화 없음

### 부정적

**수동 응답 헬퍼:**

- 내장 응답 포매팅(JSON, XML) 없음
- **완화**: oapi-codegen이 모든 응답 타입 생성

**"배터리 포함" 적음:**

- 내장 요청 바인딩, 검증, 템플릿 없음
- **완화**: oapi-codegen이 바인딩/검증 처리; API에 템플릿 불필요

**미들웨어 발견:**

- 호환되는 `net/http` 미들웨어 패키지 찾아야 함
- **완화**: go-chi 에코시스템이 일반 미들웨어 제공 (cors, httplog)

### 진화 경로

| 시나리오                | 접근법                                           |
| ----------------------- | ------------------------------------------------ |
| GraphQL 필요            | graphql-go를 Chi 마운트로 추가; 라우터 변경 없음 |
| gRPC 필요               | grpc-gateway 추가; Chi가 REST 계속 서빙          |
| 성능 이슈               | 먼저 프로파일링; Chi 오버헤드는 무시할 수준      |
| 프레임워크 마이그레이션 | 핸들러 이식 가능; 라우터 설정만 변경             |

## 참고자료

### 내부

- [ADR-01: 백엔드 언어로 Go 선택](/ko/adr/web/01-go-backend-language.md)
- [ADR-02: Next.js 16 + React 19 선택](/ko/adr/web/02-nextjs-react-selection.md)
- [Tech Radar](/ko/tech-radar.md)
- [PRD: 기술 스택](/ko/prd/06-tech-stack.md)

### 외부

- [go-chi/chi GitHub](https://github.com/go-chi/chi)
- [oapi-codegen Chi 통합](https://github.com/oapi-codegen/oapi-codegen)
- [Chi vs Gin vs Echo vs Fiber 비교](https://www.linkedin.com/pulse/comparing-go-frameworks-chi-vs-gin-fiber-httprouter-echo-parasuraman-uj0bc)
- [Fiber fasthttp 한계](https://wnjoon.github.io/2025/11/11/comparison-go-http-lib-en/)
- [Gin, Echo, Chi의 라우팅 이해](https://leapcell.io/blog/understanding-routing-and-middleware-in-gin-echo-and-chi)
