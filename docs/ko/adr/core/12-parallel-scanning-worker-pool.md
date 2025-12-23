---
title: 병렬 스캐닝
description: 대규모 테스트 파일 스캔을 위한 제한된 병렬 처리 전략
---

# ADR-12: Worker Pool 병렬 스캔

> [English Version](/en/adr/core/12-parallel-scanning-worker-pool.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-23 | @KubrickCode | core            |

## Context

### 문제 정의

대규모 리포지토리는 수천 개의 테스트 파일을 포함함. 순차 파싱은 허용할 수 없는 지연을 유발함:

1. **규모**: 5,000개 이상의 테스트 파일을 가진 모노레포
2. **사용자 경험**: 동기 API 호출은 합리적인 시간 내에 응답해야 함
3. **리소스 효율성**: 현대 머신은 다중 코어를 보유함

### 기술 요구사항

- **제한된 동시성**: 무제한 병렬 처리로 인한 리소스 고갈 방지
- **Context 전파**: 타임아웃과 취소 지원
- **에러 회복력**: 부분 실패가 전체 스캔을 중단시키지 않아야 함
- **결정적 출력**: 고루틴 스케줄링과 무관하게 재현 가능한 결과

## Decision

**errgroup + semaphore 패턴을 사용하여 제한된 병렬 처리와 결정적 출력 순서를 보장함.**

```go
sem := semaphore.NewWeighted(int64(workers))
g, gCtx := errgroup.WithContext(ctx)

for _, file := range files {
    g.Go(func() error {
        if err := sem.Acquire(gCtx, 1); err != nil {
            return nil
        }
        defer sem.Release(1)
        // parse file...
        return nil
    })
}
_ = g.Wait()
sort.Slice(results, ...)  // 결정적 순서 보장
```

## Options Considered

### Option A: errgroup + semaphore (선택됨)

고루틴 라이프사이클 관리를 위한 errgroup과 제한된 동시성을 위한 semaphore 조합.

**장점:**

- **제한된 동시성**: 설정 가능한 워커 수로 리소스 고갈 방지
- **Context 통합**: 네이티브 context 취소 및 타임아웃 지원
- **에러 전파**: errgroup이 첫 에러를 자동 수집
- **표준 라이브러리**: `golang.org/x/sync` 사용 (준표준)

**단점:**

- Semaphore 획득에 약간의 오버헤드 추가
- 두 개의 동기화 프리미티브 이해 필요

### Option B: 채널 기반 Worker Pool

고정 워커 고루틴을 사용하는 전통적인 생산자-소비자 패턴.

```go
jobs := make(chan string, len(files))
results := make(chan TestFile)

for w := 0; w < workers; w++ {
    go worker(jobs, results)
}
```

**장점:**

- 익숙한 패턴
- 외부 의존성 없음

**단점:**

- 더 많은 보일러플레이트 코드
- 복잡한 채널 조율
- 에러 처리에 추가 채널 필요
- Context 취소에 수동 전파 필요

### Option C: 무제한 병렬 처리

동시성 제어 없이 파일당 고루틴 실행.

**장점:**

- 가장 단순한 구현
- 이론적 최대 처리량

**단점:**

- **리소스 고갈**: 수천 개의 동시 고루틴
- **메모리 압박**: 각 파서가 AST를 메모리에 유지
- **파일 디스크립터 제한**: 열린 파일에 대한 OS 제한
- 부하 시 예측 불가능한 성능

## Consequences

### Positive

1. **설정 가능한 성능**
   - 기본값: GOMAXPROCS (CPU 바운드 최적화)
   - `WithWorkers(n)` 옵션으로 설정 가능
   - 상한선(MaxWorkers)으로 잘못된 설정 방지

2. **우아한 성능 저하**
   - Context 취소 시 새 작업 수락 중단
   - 진행 중인 파일은 완료 또는 타임아웃
   - 에러 목록과 함께 부분 결과 반환

3. **결정적 결과**
   - 경로별 후처리 정렬로 재현 가능한 출력 보장
   - 테스트와 캐싱 전략에 필수적

4. **리소스 안전성**
   - 메모리 사용량이 워커 수에 의해 제한됨
   - 파일 디스크립터가 워커별로 관리됨
   - CPU 사용률 예측 가능

### Negative

1. **정렬 오버헤드**
   - 병렬 수집 후 결과 정렬
   - **완화**: 일반적인 파일 수에서 O(n log n)은 수용 가능

2. **Semaphore 경합**
   - 높은 워커 수에서 획득 지연 발생 가능
   - **완화**: 기본값 GOMAXPROCS로 처리량 대 경합 균형

### 설정

| 옵션    | 기본값     | 최대 | 설명               |
| ------- | ---------- | ---- | ------------------ |
| Workers | GOMAXPROCS | 1024 | 동시 파일 파서 수  |
| Timeout | 5분        | -    | 전체 스캔 타임아웃 |

### 사용법

```go
result, err := parser.Scan(ctx, src,
    parser.WithWorkers(8),
    parser.WithTimeout(2*time.Minute),
)
```

## References

- `pkg/parser/scanner.go` - parseFilesParallel 구현
- `pkg/parser/options.go` - Functional options
- `golang.org/x/sync/errgroup` - 고루틴 그룹 관리
- `golang.org/x/sync/semaphore` - 가중치 세마포어
