---
title: slog 기반 구조화된 로깅
description: 컨텍스트 인식 request ID 주입을 포함한 Go 표준 라이브러리 slog 선택에 관한 ADR
---

# ADR-14: slog 기반 구조화된 로깅

> [English Version](/en/adr/web/14-slog-structured-logging.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-13 | @KubrickCode | web   |

## Context

### 로깅 라이브러리 선택 문제

Go 백엔드에서 다음을 위한 구조화된 로깅 솔루션이 필요:

1. **관측성(Observability)**: 로그 집계 도구(Datadog, CloudWatch, Splunk)를 위한 기계 판독 가능한 로그
2. **요청 추적**: 단일 요청의 모든 로그 항목에 일관된 `request_id` 포함
3. **구조화된 데이터**: 필터링 및 분석을 위한 키-값 쌍
4. **의존성 관리**: 프로젝트의 최소 의존성 전략과 정렬

### 초기 구현

초기 백엔드 스켈레톤(`d32ebb5`)에서는 `go-chi/httplog/v2` 사용:

```go
logger := httplog.NewLogger(serviceName, httplog.Options{
    LogLevel:       slog.LevelInfo,
    Concise:        true,
    RequestHeaders: true,
})
```

기능적으로는 충분했으나, Go 1.21+에서 네이티브로 제공하는 기능에 대해 외부 의존성을 도입하는 문제 존재.

### Go 1.21 slog 도입

Go 1.21(2023년 8월)에서 `log/slog`가 표준 라이브러리 패키지로 도입되어, 외부 의존성 없이 구조화된 로깅 제공. 이를 통해 기능 동등성을 유지하면서 의존성 풋프린트를 줄일 수 있는 기회가 생김.

## Decision

**httplog/v2에서 컨텍스트 인식 Logger 래퍼를 포함한 Go 표준 라이브러리 slog로 마이그레이션.**

구현 원칙:

1. **표준 라이브러리 우선**: 모든 구조화된 로깅에 `log/slog` 직접 사용
2. **컨텍스트 전파**: Chi 미들웨어 컨텍스트에서 `request_id` 자동 주입
3. **필드 체이닝**: 컨텍스트 필드(owner, repo 등) 추가를 위한 `With()` 메서드 지원
4. **DI 통합**: 테스트 용이성을 위해 의존성 주입으로 Logger 주입

## Options Considered

### Option A: slog (표준 라이브러리) - 선택됨

**작동 방식:**

- `log/slog` 패키지 직접 사용
- 컨텍스트 인식 `request_id` 주입을 위한 커스텀 `Logger` 래퍼
- HTTP 상태/크기 캡처를 위한 커스텀 `responseWriter` 래퍼

**장점:**

- **외부 의존성 제로**: Go 1.21부터 표준 라이브러리에 포함
- **장기 안정성**: Go 팀이 유지보수; 하위 호환성 보장
- **생태계 통합**: 다른 로깅 라이브러리들이 slog를 백엔드로 사용 가능
- **충분한 성능**: 메모리 할당 40 B/op; 웹 애플리케이션에 충분
- **미래 보장**: 향후 모든 Go 도구가 slog와 통합될 예정

**단점:**

- 극한의 고처리량 시나리오에서 zerolog/zap보다 약간 느림
- 특화된 라이브러리보다 기능이 적음

### Option B: zerolog

**작동 방식:**

- 제로 할당 JSON 로거
- 체이닝 API: `log.Info().Str("key", "val").Msg("message")`
- 외부 의존성: `github.com/rs/zerolog`

**평가:**

- Go 생태계에서 가장 빠른 로깅 라이브러리
- 일반 작업에서 제로 할당
- 외부 의존성 필요
- **기각**: 웹 애플리케이션 규모에서 성능 이점 불필요; 의존성 추가

### Option C: zap

**작동 방식:**

- Uber의 고성능 구조화 로거
- `zap.String()`, `zap.Int()` 헬퍼를 사용한 필드 기반 API
- 외부 의존성: `go.uber.org/zap`

**평가:**

- 광범위한 커스터마이징 옵션
- Uber 규모에서 강력한 프로덕션 실적
- 더 높은 메모리 할당(slog 40 B/op 대비 168 B/op)
- **기각**: 요구사항에 비해 과함; 불필요한 외부 의존성

### Option D: logrus

**작동 방식:**

- 훅 시스템을 갖춘 구조화된 로깅
- JSON 및 텍스트 포매터
- 외부 의존성: `github.com/sirupsen/logrus`

**평가:**

- 광범위한 훅 생태계를 갖춘 풍부한 기능
- 더 이상 적극 개발되지 않음(유지보수 모드)
- 현대적 대안보다 높은 오버헤드
- **기각**: 레거시 상태; 새 프로젝트에 권장되지 않음

### Option E: httplog/v2 (현상 유지)

**작동 방식:**

- Chi 특화 로깅 미들웨어 래퍼
- slog 위에 구축됨
- 외부 의존성: `github.com/go-chi/httplog/v2`

**평가:**

- 편리한 Chi 통합
- slog가 네이티브로 제공하는 것에 의존성 레이어 추가
- 컨텍스트 주입에 대한 제한된 커스터마이징
- **기각**: 불필요한 추상화; 직접 slog가 더 많은 제어권 제공

## Implementation Details

### HTTP 요청 로거 미들웨어

커스텀 `responseWriter` 래퍼가 HTTP 상태와 응답 크기를 캡처:

```go
type responseWriter struct {
    http.ResponseWriter
    status int
    size   int
}

func Logger() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(rw, r)

            slog.Info("http request",
                "request_id", middleware.GetReqID(r.Context()),
                "method", r.Method,
                "path", r.URL.Path,
                "status", rw.status,
                "size", rw.size,
                "duration", time.Since(start).String(),
            )
        })
    }
}
```

### 컨텍스트 인식 Logger 래퍼

`Logger` 구조체가 컨텍스트에서 `request_id` 자동 포함:

```go
type Logger struct {
    base  *slog.Logger
    attrs []any
}

func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
    l.logger(ctx).Info(msg, args...)
}

func (l *Logger) logger(ctx context.Context) *slog.Logger {
    allAttrs := make([]any, 0, len(l.attrs)+2)
    allAttrs = append(allAttrs, "request_id", middleware.GetReqID(ctx))
    allAttrs = append(allAttrs, l.attrs...)
    return l.base.With(allAttrs...)
}
```

### 사용 패턴

```go
// handler/service 내에서
logger := logger.New().With("owner", owner, "repo", repo)
logger.Info(ctx, "analysis started")
// 출력: {"level":"INFO","msg":"analysis started","request_id":"abc123","owner":"foo","repo":"bar"}
```

## Consequences

### Positive

**의존성 감소:**

- `go.mod`에서 `github.com/go-chi/httplog/v2` 제거
- 프로젝트의 최소 의존성 철학과 정렬

**장기 안정성:**

- 표준 라이브러리가 하위 호환성 보장
- slog를 사용하는 다른 패키지와 버전 충돌 없음
- Go 런타임 개선의 자동 혜택

**생태계 통합:**

- 필요시 zerolog/zap를 slog 백엔드로 사용 가능
- Go 생태계 전반에 걸친 일관된 로깅 인터페이스
- 로그 집계 도구(Datadog, CloudWatch)가 slog JSON 출력을 네이티브로 파싱

**향상된 테스트 용이성:**

- DI 패턴으로 Logger 주입
- 유닛 테스트를 위한 쉬운 모킹
- Go 관용구와 일치하는 컨텍스트 기반 설계

### Negative

**성능 트레이드오프:**

- 극한 처리량에서 zerolog보다 ~10-15% 느림
- **완화**: 웹 애플리케이션 규모에서 무시할 수 있음; 병목 아님

**커스텀 래퍼 유지보수:**

- `Logger` 래퍼 코드 유지보수 필요
- **완화**: 최소한의 코드(~47줄); 안정적인 요구사항

**제한된 고급 기능:**

- 내장 로그 로테이션이나 훅 시스템 없음
- **완화**: 로컬 파일 대신 외부 로그 집계(CloudWatch, Datadog) 사용

## References

### Internal

- [ADR-01: Go를 백엔드 언어로](/ko/adr/web/01-go-backend-language.md)

### External

- [Structured Logging with slog - Go Blog](https://go.dev/blog/slog)
- [slog 패키지 문서](https://pkg.go.dev/log/slog)
- [Go 로깅 벤치마크](https://github.com/betterstack-community/go-logging-benchmarks)
