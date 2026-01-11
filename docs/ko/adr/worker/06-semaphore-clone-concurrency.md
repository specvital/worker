---
title: 세마포어 기반 클론 동시성 제어
description: UseCase 레벨 weighted semaphore를 통한 Git clone 동시성 제한
---

# ADR-06: 세마포어 기반 클론 동시성 제어

> 🇺🇸 [English Version](/en/adr/worker/06-semaphore-clone-concurrency.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2024-12-18 | @KubrickCode | worker     |

## 컨텍스트

### 문제

Git clone 작업은 리소스 집약적:

- **네트워크 I/O**: 전체 저장소 히스토리 다운로드
- **디스크 I/O**: 파일시스템 쓰기 (코드 + .git 디렉토리)
- **메모리**: 대규모 저장소는 수백 MB 소비

동시성 제어 없이 무제한 병렬 clone 시:

- 제한된 환경에서 OOM(Out-of-memory) 오류
- 네트워크 대역폭 고갈
- 모든 동시 작업 성능 저하

### 제약 조건

- **배포 대상**: 소규모 VM (512MB-2GB RAM)
- **큐 아키텍처**: 설정 가능한 동시성을 가진 River worker (기본값: 5)
- **워크로드**: 다양한 저장소 크기 (소규모 라이브러리 ~ 대형 모노레포)

### 목표

1. 동시 clone 작업으로 인한 OOM 방지
2. 리소스 제한 내에서 처리량 최대화
3. context 취소 및 타임아웃 준수
4. 배포 환경별 런타임 설정 허용

## 결정

**UseCase 레벨에서 weighted semaphore를 적용하여 동시 clone 작업 제한.**

### 구현

```go
type AnalyzeUseCase struct {
    cloneSem *semaphore.Weighted
    // ... 기타 의존성
}

func NewAnalyzeUseCase(..., opts ...Option) *AnalyzeUseCase {
    return &AnalyzeUseCase{
        cloneSem: semaphore.NewWeighted(cfg.MaxConcurrentClones),
    }
}

func (uc *AnalyzeUseCase) cloneWithSemaphore(ctx context.Context, url string, token *string) (Source, error) {
    if err := uc.cloneSem.Acquire(ctx, 1); err != nil {
        return nil, err
    }
    defer uc.cloneSem.Release(1)

    return uc.vcs.Clone(ctx, url, token)
}
```

### 주요 특성

| 항목         | 값                                |
| ------------ | --------------------------------- |
| 라이브러리   | `golang.org/x/sync/semaphore`     |
| 기본 제한    | 동시 2개 clone                    |
| 위치         | UseCase 레이어 (Adapter 아님)     |
| 설정         | `WithMaxConcurrentClones(n)` 옵션 |
| Context 처리 | 자동 타임아웃/취소 전파           |

## 검토한 대안

### 옵션 A: UseCase 레벨 Weighted Semaphore (선택됨)

**설명:**

UseCase에서 `golang.org/x/sync/semaphore.Weighted`를 사용하여 clone 호출 래핑.

**장점:**

- 명시적 의도: "N개 동시 작업 제한"을 명확히 표현
- Context 인식: 내장된 타임아웃/취소 처리
- FIFO 순서로 기아(starvation) 방지
- UseCase 인스턴스별 설정 가능
- 검증된 stdlib 확장

**단점:**

- 인스턴스별 제한 (클러스터 전체 아님)
- 정적 제한 (가용 메모리 기반 동적 조정 불가)

### 옵션 B: Git Adapter 레벨 Semaphore

**설명:**

동시성 제어를 VCS adapter로 이동.

**장점:**

- 모든 VCS 작업이 자동으로 제한됨
- 단일 제어 지점

**단점:**

- 잘못된 추상화 레이어: 리소스 관리는 비즈니스 정책, I/O 세부사항 아님
- 전역 제한: usecase별 다른 제한 불가
- Adapter가 상태를 가지게 되어 단일 책임 원칙 위반
- UseCase 동시성 동작 테스트 어려움

### 옵션 C: 전역 Rate Limiter

**설명:**

`golang.org/x/time/rate`를 사용하여 clone 요청 속도 제한.

**장점:**

- 간단한 API
- 잘 알려진 패턴

**단점:**

- 시간당 요청 수 제어, 동시 작업 수 아님
- N개 토큰 사용 가능 시 N개 clone 동시 시작 방지 불가
- 리소스 고갈 문제에 대한 잘못된 추상화

### 옵션 D: 채널 기반 Worker Pool

**설명:**

버퍼 채널을 사용한 전용 clone worker pool 생성.

**장점:**

- worker 라이프사이클에 대한 세밀한 제어
- 커스텀 스케줄링 로직 구현 가능

**단점:**

- 과잉 엔지니어링: River가 이미 worker pool 제공
- 중첩된 worker pool이 관찰성 복잡화
- 수동 context 처리 필요 (`select` 문)
- semaphore보다 더 많은 보일러플레이트

## 구현 원칙

### UseCase 레벨 선택 이유

동시성 제어는 **비즈니스 정책 결정**:

```
┌─────────────────────────────────────┐
│   UseCase (AnalyzeUseCase)          │
│   ┌─────────────────────────────┐   │ ← Semaphore: 비즈니스 결정
│   │ Semaphore 제어              │   │   "최대 N개 동시 clone 허용"
│   │  • clone 전 Acquire         │   │
│   │  • clone 후 Release         │   │
│   └─────────────────────────────┘   │
│              │                       │
│              ▼                       │
│     vcs.Clone(ctx, url, token)      │ ← Adapter 호출 (thin wrapper)
└─────────────────────────────────────┘
```

- **UseCase는 실행 컨텍스트 인식**: River worker 동시성, 메모리 제약 파악
- **Adapter는 무상태 유지**: 순수 I/O, 리소스 관리 없음
- **설정 유연성**: 다른 usecase는 다른 제한 가능

### 기본값 = 2인 이유

| 제한  | 메모리 (추정) | 네트워크 | 평가                    |
| ----- | ------------- | -------- | ----------------------- |
| 1     | ~500MB        | 미활용   | 너무 보수적             |
| **2** | **~1GB**      | **균형** | **2GB 인스턴스에 안전** |
| 3     | ~1.5GB        | 높음     | OOM 위험                |
| 5     | ~2.5GB        | 최대     | 소규모 VM에서 OOM 확정  |

가정:

- 평균 저장소 clone: ~500MB (코드 + .git 히스토리)
- 배포 대상: 512MB-2GB RAM 인스턴스
- parser (tree-sitter), DB 연결, OS를 위한 여유 공간 필요

### Context 전파

```go
// Execute가 15분 타임아웃 설정
timeoutCtx, cancel := context.WithTimeout(ctx, uc.timeout)
defer cancel()

// Semaphore Acquire가 context 준수
if err := uc.cloneSem.Acquire(ctx, 1); err != nil {
    return nil, err  // 타임아웃 시 context.DeadlineExceeded
}
```

장점:

- **타임아웃 전파**: 작업이 semaphore 대기 중 멈추지 않음
- **Graceful shutdown**: Worker 종료 시 context 취소, 대기자 해제
- **고루틴 누수 없음**: 취소 시 자동 정리

## 결과

### 긍정적

**메모리 안전성:**

- 최대 2개 동시 clone으로 최대 메모리 사용량 제한
- 제한된 환경에서 OOM 방지

**예측 가능한 동작:**

- FIFO 큐 순서: 기아 없음
- 부하 상황에서 결정론적 처리량

**Context 통합:**

- 자동 타임아웃 처리
- 깔끔한 취소 전파
- 수동 정리 불필요

**운영 단순성:**

- 단일 설정 옵션
- 외부 의존성 없음
- 표준 로깅으로 관찰 가능

### 부정적

**큐 대기 시간:**

- 버스트 트래픽 시 작업이 semaphore 대기
- 완화: River 큐 깊이 모니터링

**인스턴스별 제한:**

- 클러스터 전체 제한 아님
- 3개 worker × 2개 clone = 총 6개 동시 clone
- 현재 규모에서는 허용 가능

**정적 설정:**

- 런타임 메모리 기반 동적 조정 불가
- 향후 개선: 리소스 모니터링과 통합

## 스케일링 가이드라인

| 인스턴스 크기     | 권장 제한 | 비고                    |
| ----------------- | --------- | ----------------------- |
| Small (512MB)     | 1         | 무료 티어용 보수적 설정 |
| Medium (2GB)      | 2         | 기본 설정               |
| Large (8GB)       | 4         | 높은 처리량             |
| Dedicated (32GB+) | 8         | 최대 병렬 I/O           |

```go
// 설정 예시
uc := NewAnalyzeUseCase(
    repo, vcs, parser, tokenLookup,
    WithMaxConcurrentClones(4),
)
```

## 참조

- [golang.org/x/sync/semaphore](https://pkg.go.dev/golang.org/x/sync/semaphore)
- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md) - 레이어 배치 근거
- [ADR-03: Graceful Shutdown](./03-graceful-shutdown.md) - Context 전파 패턴
