---
title: Clean Architecture 패턴
description: 관심사 분리, 테스트 용이성, AI 지원 개발을 위한 5계층 Clean Architecture 채택 ADR
---

# ADR-08: Clean Architecture 패턴

> [English Version](/en/adr/web/08-clean-architecture-pattern.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-01-03 | @KubrickCode | web        |

## 배경

### 초기 아키텍처의 문제점

코드베이스가 성장함에 따라 초기 서비스 지향 구조에서 여러 문제 발생:

**도메인-인프라 결합:**

- 비즈니스 로직이 데이터베이스 쿼리 및 HTTP 처리와 얽혀 있음
- 인프라 세부사항(PostgreSQL, River 큐) 변경 시 서비스 계층 수정 필요
- 서비스 계층에서 HTTP 상태 코드를 직접 반환하여 관심사 분리 위반

**제한된 테스트 용이성:**

- 구체적인 구현체에 대한 직접 의존성으로 인해 단위 테스트 어려움
- 간단한 비즈니스 규칙 검증에도 통합 테스트 필요
- 상당한 리팩토링 없이는 목 주입 불가능

**AI 지원 개발의 제약:**

- 대규모 서비스 파일이 AI 컨텍스트 윈도우를 초과하여 LLM 코딩 효율성 저하
- 횡단 관심사로 인해 AI 도구가 수정 범위를 파악하기 어려움
- AI 에이전트가 작업할 수 있는 명확한 경계 부재

### 진화 타임라인

| 변경 사항                                 | 동기                                   |
| ----------------------------------------- | -------------------------------------- |
| Service 계층 + StrictServerInterface 도입 | 핸들러에서 비즈니스 로직 분리          |
| 서비스에서 HTTP 상태 코드 분리            | 서비스 계층이 HTTP 코드 반환하던 문제  |
| 에러 + 모델과 함께 도메인 계층 도입       | 도메인 정의 중앙화                     |
| Clean Architecture 도메인 계층 적용       | entity/ + port/ 분리                   |
| Clean Architecture 유스케이스 계층 적용   | 기능별 유스케이스                      |
| Clean Architecture 어댑터 계층 적용       | Repository, Queue, Client 구현         |
| Clean Architecture 핸들러 계층 적용       | handler -> usecase -> domain 흐름 완성 |

### Worker 서비스와의 정합성

Worker 서비스는 이미 6계층 Clean Architecture 채택([Worker ADR-02](/ko/adr/worker/02-clean-architecture-layers.md)). Web 백엔드에서 유사한 구조 채택으로:

- 리포지토리 간 일관된 멘탈 모델 유지
- 팀원들을 위한 재사용 가능한 패턴
- 공유 테스트 전략

## 결정

**Web 백엔드에 5계층 Clean Architecture 채택.**

### 계층 구조

| 계층        | 위치             | 책임                               |
| ----------- | ---------------- | ---------------------------------- |
| **Entity**  | `domain/entity/` | 순수 비즈니스 모델, 값 객체        |
| **Port**    | `domain/port/`   | 인터페이스 정의 (DIP 계약)         |
| **UseCase** | `usecase/`       | 비즈니스 로직, 기능 오케스트레이션 |
| **Adapter** | `adapter/`       | 외부 구현체 (DB, API, Queue)       |
| **Handler** | `handler/`       | HTTP 진입점, 요청/응답 처리        |

### 의존성 방향

```
handler -> usecase -> domain <- adapter
                        ^
                (port 구현)
```

- **Domain 계층**은 외부 의존성 없음
- **UseCase 계층**은 Domain 인터페이스에만 의존
- **Adapter 계층**은 Domain port 인터페이스를 구현
- **Handler 계층**은 UseCase를 직접 주입

### 왜 6계층이 아닌 5계층인가?

Worker는 별도의 Application과 Infrastructure 계층을 포함한 6계층 사용. Web은 이를 단순화:

| Worker (6계층) | Web (5계층)      | 근거                                   |
| -------------- | ---------------- | -------------------------------------- |
| Application    | (Handler에 병합) | Web은 단일 진입점(HTTP)만 있음         |
| Infrastructure | (Adapter에 병합) | Web 컨텍스트에서 더 단순한 DI 와이어링 |

Web 백엔드의 단순한 요구사항(HTTP 전용 진입점, 소규모 팀)은 추가적인 Infrastructure/Application 분리를 정당화하지 않음.

## 고려한 옵션

### 옵션 A: 5계층 Clean Architecture (선택됨)

**작동 방식:**

- Domain 계층이 순수 엔티티와 port 인터페이스 정의
- UseCase 계층이 port를 사용하여 비즈니스 로직 오케스트레이션
- Adapter 계층이 특정 기술로 port 구현
- Handler 계층이 HTTP 요청을 UseCase에 매핑

**장점:**

- **테스트 용이성**: 간단한 port 목으로 UseCase 테스트 가능
- **유지보수성**: 명확한 경계로 인지 부하 감소
- **AI 친화성**: 고립된 파일이 LLM 컨텍스트 윈도우에 적합
- **유연성**: 기술 변경이 adapter 계층에 격리됨
- **일관성**: Worker 아키텍처 패턴과 정합

**단점:**

- 모놀리식 접근보다 더 많은 파일과 패키지
- 의존성 흐름 이해에 문서화 필요
- 단순 CRUD 작업에도 오버헤드

### 옵션 B: 전통적인 계층형 아키텍처

**작동 방식:**

- Handler -> Service -> Repository 패턴
- Service 계층에 모든 비즈니스 로직 포함
- Repository가 데이터베이스 접근 처리

**장점:**

- 더 단순한 초기 구조
- 간접 참조 적음
- 널리 이해되는 일반적인 패턴

**단점:**

- 기능 증가에 따라 Service 파일 비대화
- 구체 클래스 모킹 필요한 테스트
- Service 계층에 HTTP 관심사 누출
- Service 계층의 기술 결합

### 옵션 C: 헥사고날 아키텍처

**작동 방식:**

- Ports and Adapters 패턴
- 내부 구조에 대한 규정이 적음
- Inbound/Outbound 어댑터 구분

**장점:**

- 유연한 내부 조직
- 잘 문서화된 패턴
- 명확한 경계 개념

**단점:**

- 내부 계층 구조에 대한 가이드 부족
- "Application hexagon" 정의 모호
- Clean Architecture가 더 실행 가능한 구조 제공

### 옵션 D: 서비스 지향 구조 유지

**작동 방식:**

- Handler -> Service 패턴 계속 사용
- 필요시 점진적 리팩토링

**평가:**

- 서비스 계층의 HTTP 상태 코드가 분리 원칙 위반
- 시간이 지남에 따라 테스트 복잡성 증가
- AI 에이전트가 대규모 서비스 파일 처리에 어려움

## 구현

### Port 인터페이스 패턴

인터페이스는 구현체가 아닌 Domain 계층에 정의:

```
modules/{module}/
├── domain/
│   ├── entity/        # 순수 Go 모델
│   │   └── analysis.go
│   └── port/          # 인터페이스 정의
│       └── repository.go
├── usecase/           # 기능당 하나의 파일
│   └── get_analysis.go
├── adapter/           # 외부 구현체
│   ├── repository_postgres.go
│   └── mapper/
│       └── response.go
└── handler/
    └── http.go        # StrictServerInterface 구현
```

### 에러 처리 패턴

도메인 에러는 Handler 계층에서 HTTP 상태 코드로 매핑:

| 도메인 에러        | HTTP 상태 | 용도           |
| ------------------ | --------- | -------------- |
| `ErrNotFound`      | 404       | 분석 결과 없음 |
| `ErrAlreadyQueued` | 409       | 중복 요청      |
| `ErrRateLimited`   | 429       | 요청 제한 초과 |
| (예상치 못한 에러) | 500       | 내부 오류      |

### UseCase 패턴

각 유스케이스는 port 의존성을 가진 집중된 구조체입니다:

```go
type GetAnalysisUseCase struct {
    queue      port.QueueService
    repository port.Repository
}

type GetAnalysisInput struct {
    Owner string
    Repo  string
}

func (uc *GetAnalysisUseCase) Execute(ctx context.Context, input GetAnalysisInput) (*AnalyzeResult, error)
```

### 임포트 규칙 (depguard로 강제)

| 계층          | 허용되는 임포트          |
| ------------- | ------------------------ |
| domain/entity | 외부 의존성 없음         |
| domain/port   | entity만                 |
| usecase       | domain만 (entity + port) |
| adapter       | domain + 외부 라이브러리 |
| handler       | usecase + adapter/mapper |

## 결과

### 긍정적

**테스트 용이성:**

- 목 없이 도메인 로직 테스트 가능
- 간단한 port 목으로 UseCase 테스트 가능
- 비즈니스 규칙 검증에 데이터베이스/큐 불필요
- 단위 테스트로 90%+ 커버리지 달성 가능

**유지보수성:**

- 명확한 경계로 인지 부하 감소
- 한 계층의 변경이 다른 계층에 거의 영향 없음
- 잘 정의된 책임으로 온보딩 용이
- 코드 탐색이 예측 가능한 패턴을 따름

**AI 지원 개발:**

- 각 파일이 LLM 컨텍스트 윈도우 내에 자체 완결적
- AI 에이전트가 전체 모듈을 이해하고 재생성 가능
- 명시적 인터페이스로 파일 간 의존성 스캔 감소
- 경계가 있는 컨텍스트로 효과적인 AI 기반 리팩토링 가능

**유연성:**

- 데이터베이스 마이그레이션: adapter 계층만 변경
- 큐 시스템 교체: adapter 계층만 변경
- 새 유스케이스: usecase 파일 추가, handler에 와이어링

### 부정적

**초기 복잡성:**

- 서비스 지향 접근보다 더 많은 패키지와 파일
- 의존성 흐름 이해에 문서화 필요
- **완화**: CLAUDE.md에 계층 구조 문서화; depguard로 규칙 강제

**간접 참조:**

- HTTP 요청과 비즈니스 로직 사이에 더 많은 계층
- 여러 패키지를 통한 디버깅 추적 필요
- **완화**: 컨텍스트가 있는 구조화된 로깅; 명확한 명명 규칙

**단순 작업에도 오버헤드:**

- 간단한 CRUD에도 전체 계층 탐색 필요
- 간단한 기능에는 과도하게 느껴질 수 있음
- **완화**: 장기적 유지보수성에 대한 투자로 오버헤드 수용

### 마이그레이션 고려사항

기존 모듈은 점진적으로 마이그레이션:

1. **analyzer**: 첫 번째 마이그레이션 모듈
2. **auth**: 5개 port 인터페이스로 전체 마이그레이션
3. **github**: Service 계층을 usecase로 교체
4. **user**: 북마크 및 히스토리 기능 재구조화

각 마이그레이션은 동일한 패턴: domain/entity -> domain/port -> usecase -> adapter -> handler.

## 참고자료

- [The Clean Architecture - Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Clean Architecture in Go - Three Dots Labs](https://threedots.tech/post/introducing-clean-architecture/)
- [AI-Optimizing Codebase Architecture for AI Coding Tools](https://medium.com/@richardhightower/ai-optimizing-codebase-architecture-for-ai-coding-tools-ff6bb6fdc497)
- [Worker ADR-02: Clean Architecture Layers](/ko/adr/worker/02-clean-architecture-layers.md)
- [Worker ADR-07: Repository Pattern](/ko/adr/worker/07-repository-pattern.md)
