---
title: DI Container 패턴
description: 테스트 용이성 향상을 위해 sync.Once 싱글톤에서 Container 기반 의존성 주입으로 마이그레이션한 ADR
---

# ADR-09: DI Container 패턴

> [English Version](/en/adr/web/09-di-container-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### 초기 패턴: sync.Once 싱글톤

초기 코드베이스는 Go의 `sync.Once` 패턴을 사용하여 공유 의존성 초기화:

**식별된 문제점:**

- **테스트 어려움**: 싱글톤 인스턴스를 목(mock)으로 대체 불가
- **숨겨진 의존성**: 모듈들이 명시적 선언 없이 전역 싱글톤에 접근
- **초기화 순서**: 암묵적 의존성 순서로 인해 초기화 실패 디버깅 어려움
- **라이프사이클 관리**: 리소스에 대한 중앙화된 정리 메커니즘 부재

### Clean Architecture와 함께한 발전

코드베이스가 Clean Architecture로 마이그레이션됨에 따라 ([ADR-08](/ko/adr/web/08-clean-architecture-pattern.md)), 명시적 의존성 주입의 필요성 대두:

- **UseCase 계층**: 포트 인터페이스 주입 필요
- **Handler 계층**: UseCase 주입 필요
- **Adapter 계층**: 인프라 클라이언트(DB, Queue) 주입 필요

### Worker 서비스와의 일관성

Worker 서비스는 진입점별(Worker, Scheduler) 별도 컨테이너를 사용하는 Container 패턴 사용. 유사한 패턴을 채택하여 저장소 간 일관성 확보.

## Decision

**sync.Once 싱글톤에서 Container 기반 의존성 주입으로 마이그레이션.**

### 2계층 Container 아키텍처

| Container           | 위치                          | 책임                                               |
| ------------------- | ----------------------------- | -------------------------------------------------- |
| **infra.Container** | `internal/infra/container.go` | 인프라 의존성 (DB, Queue, OAuth, Encryptor)        |
| **server.App**      | `common/server/app.go`        | 애플리케이션 의존성 (Handlers, UseCases, Adapters) |

### 의존성 흐름

```
main.go
    └─→ server.NewApp()
            └─→ infra.NewContainer() → Container{DB, River, OAuth, JWT, ...}
            └─→ initHandlers(container)
                    └─→ Adapter (container.DB 사용)
                    └─→ UseCase (port 사용)
                    └─→ Handler (usecase 사용)
            └─→ App{Handlers, Middleware, infra}
```

## Options Considered

### Option A: Container 기반 DI (선택됨)

**작동 방식:**

- `Config` 구조체로 설정과 컨테이너 생성 분리
- `ConfigFromEnv()`로 환경 변수를 Config로 로드
- `NewContainer()`로 모든 인프라 의존성 생성
- `App`에서 컨테이너를 사용하여 핸들러와 유스케이스 조립
- `Close()` 메서드로 역순 정리 처리

**장점:**

- **테스트 용이성**: 생성자를 통한 의존성 주입으로 쉬운 목킹
- **명시적 의존성**: 모든 의존성이 Container 구조체에서 가시적
- **라이프사이클 제어**: Close() 메서드로 중앙화된 정리
- **일관성**: Worker 서비스 패턴과 일치

**단점:**

- 초기 설정 복잡성
- 의존성 연결을 위한 보일러플레이트 코드
- 모든 의존성이 선행 생성됨 (사용하지 않아도)

### Option B: sync.Once 싱글톤 (이전 방식)

**작동 방식:**

- 각 모듈이 `sync.Once`를 통해 싱글톤 초기화
- 패키지 수준 `Get*()` 함수로 싱글톤 접근
- 중앙 의존성 레지스트리 없음

**평가:**

- 더 단순한 초기 구현
- 명시적 연결 불필요
- 테스트 시 전역 상태 조작 필요
- 의존성 순서 버그가 런타임에 나타남
- **기각**: Clean Architecture 테스트 요구사항에 부적합

### Option C: Wire/Fx DI 프레임워크

**작동 방식:**

- Google Wire: 코드 생성을 통한 컴파일 타임 의존성 주입
- Uber Fx: 라이프사이클 훅을 가진 런타임 DI 컨테이너

**평가:**

- 보일러플레이트 코드 감소
- 외부 의존성 추가
- 매직 코드 생성(Wire) 또는 리플렉션(Fx)
- 현재 규모(~10개 의존성)에는 과함
- **기각**: 비례하지 않는 복잡성 추가

### Option D: 수동 생성자 주입

**작동 방식:**

- 각 모듈이 의존성을 받는 생성자 정의
- 중앙 컨테이너 없음; 호출 체인을 통해 의존성 전달
- main.go에서 전체 의존성 그래프 구성

**평가:**

- 최대한의 명시성
- 긴 생성자 매개변수 목록
- 여러 계층을 통해 의존성을 스레딩해야 함
- **기각**: Container가 더 나은 조직화 제공

## Implementation

### Config 분리

설정을 컨테이너 생성에서 분리하여 다음을 가능하게 함:

- 환경별 설정
- 테스트 설정 주입
- 리소스 할당 전 검증

```go
type Config struct {
    DatabaseURL   string
    EncryptionKey string
    JWTSecret     string
    // ... 기타 필드
}

func ConfigFromEnv() Config {
    return Config{
        DatabaseURL:   os.Getenv("DATABASE_URL"),
        EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
        // ...
    }
}
```

### Container 생성

Container는 모든 인프라 의존성을 생성하고 보유:

```go
type Container struct {
    DB             *pgxpool.Pool
    River          *RiverClient
    Encryptor      crypto.Encryptor
    JWTManager     authport.TokenManager
    GitHubOAuth    authport.OAuthClient
    GitHubAppClient ghappport.GitHubAppClient
    // ... 기타 의존성
}

func NewContainer(ctx context.Context, cfg Config) (*Container, error) {
    // 검증
    if err := validateConfig(cfg); err != nil {
        return nil, err
    }

    // 순서대로 의존성 생성
    pool, err := NewPostgresPool(ctx, PostgresConfig{URL: cfg.DatabaseURL})
    // ... 다른 의존성 생성

    return &Container{DB: pool, ...}, nil
}
```

### 애플리케이션 조립

App은 Container를 사용하여 비즈니스 계층 의존성 생성:

```go
type App struct {
    AuthMiddleware *middleware.AuthMiddleware
    Handlers       *Handlers
    infra          *infra.Container
}

func NewApp(ctx context.Context) (*App, error) {
    cfg := infra.ConfigFromEnv()
    container, err := infra.NewContainer(ctx, cfg)

    handlers, err := initHandlers(container)
    // handlers 생성: adapters → usecases → handlers

    return &App{Handlers: handlers, infra: container}, nil
}
```

### RouteRegistrar 인터페이스

모듈 간 라우트 등록 표준화:

```go
type RouteRegistrar interface {
    RegisterRoutes(r chi.Router)
}
```

### 리소스 정리

생성 역순으로 정리:

```go
func (c *Container) Close() error {
    if c.DB != nil {
        c.DB.Close()
    }
    return nil
}

func (a *App) Close() error {
    return a.infra.Close()
}
```

## Consequences

### Positive

**테스트 용이성:**

- Mock UseCase로 Handler 테스트 가능
- Mock Port로 UseCase 테스트 가능
- 전역 상태 조작 불필요
- 테스트 케이스별로 테스트 설정 격리

**명시적 의존성:**

- 모든 의존성이 Container/App 구조체에서 가시적
- 코드 구조에서 의존성 그래프 명확
- 전역 싱글톤을 통한 숨겨진 결합 없음

**라이프사이클 관리:**

- 정의된 순서로 리소스 생성
- Close() 체인을 통한 정리 보장
- main.go에서 graceful shutdown 지원

**일관성:**

- Worker 서비스 Container 패턴과 일치
- 저장소 간 작업 시 익숙한 구조
- 의존성 관리에 대한 공유 멘탈 모델

### Negative

**초기 복잡성:**

- 싱글톤 접근보다 더 많은 파일
- 컨테이너 흐름 이해에 문서화 필요
- **완화**: CLAUDE.md에 패턴 문서화; 명확한 명명 규칙

**보일러플레이트:**

- initHandlers()에서 명시적 연결 (~150줄)
- 각 새 모듈에 연결 추가 필요
- **완화**: 명시성과 테스트 용이성을 위한 수용 가능한 트레이드오프

**선행 생성:**

- 시작 시 모든 의존성 생성
- 사용하지 않는 의존성도 리소스 소비
- **완화**: 현재 규모에서는 지연 초기화 불필요

## References

- [Dependency Injection in Go - Alex Edwards](https://www.alexedwards.net/blog/organising-database-access)
- [Worker ADR-02: Clean Architecture Layers](/ko/adr/worker/02-clean-architecture-layers.md)
- [Web ADR-08: Clean Architecture Pattern](/ko/adr/web/08-clean-architecture-pattern.md)
