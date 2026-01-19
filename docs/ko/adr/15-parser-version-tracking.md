---
title: 파서 버전 추적
description: 파서 버전 변경 기반 자동 재분석 트리거에 대한 ADR
---

# ADR-15: 재분석을 위한 파서 버전 추적

> 🇺🇸 [English Version](/en/adr/15-parser-version-tracking.md)

| 날짜       | 작성자       | 리포지토리    |
| ---------- | ------------ | ------------- |
| 2026-01-18 | @KubrickCode | infra, worker |

## 배경

### 파서 업그레이드 사각지대

예약 재수집 시스템(Worker ADR-01)은 커밋 SHA 비교를 통해 리포지토리 변경 감지. 그러나 specvital/core가 파서 개선으로 업그레이드될 때, 리포지토리 커밋이 변경되지 않아 기존 분석이 그대로 유지되는 사각지대 발생.

**문제 시나리오:**

```
1. 사용자가 커밋 `abc123`에서 리포지토리 분석 → 테스트 수 50으로 저장
2. Core v1.2.0 릴리즈로 Jest 감지 개선
3. 동일 리포지토리, 동일 커밋 `abc123`이 이제 55개 테스트 보고 예정
4. 사용자는 리포지토리가 새 커밋을 만들 때까지 오래된 수치(50) 확인
```

**영향:**

| 문제               | 결과                             |
| ------------------ | -------------------------------- |
| 파서 버그 수정     | 기존 분석에 반영 불가            |
| 새 프레임워크 지원 | 수동 재분석 전까지 사용 불가     |
| 정확도 개선        | 사용자가 데이터 노후화 인지 불가 |

### 요구사항

| 요구사항    | 설명                              |
| ----------- | --------------------------------- |
| 자동 감지   | 파서 버전과 분석 버전 불일치 식별 |
| 무개입 운영 | 배포 시 설정 불필요               |
| 하위 호환성 | 버전 정보 없는 기존 분석 처리     |
| 통합        | 기존 자동 새로고침 인프라와 연동  |

## 결정

**Go의 `debug.ReadBuildInfo()`를 사용한 런타임 파서 버전 추출과 데이터베이스 기반 버전 비교 구현.**

### 1. 버전 등록

Worker 시작 시 core 모듈 버전 추출, `system_config` 테이블에 UPSERT:

```go
// buildinfo/version.go
func ExtractCoreVersion() string {
    info, ok := debug.ReadBuildInfo()
    if !ok {
        return "unknown"
    }
    for _, dep := range info.Deps {
        if dep.Path == "github.com/specvital/core" {
            return dep.Version  // e.g., "v1.2.3"
        }
    }
    return "unknown"
}
```

### 2. 버전 기록

각 분석 레코드에 해당 분석을 생성한 파서 버전 저장:

```sql
ALTER TABLE analyses ADD COLUMN parser_version VARCHAR(100) DEFAULT 'legacy';
```

### 3. 버전 비교

자동 새로고침 로직에서 분석 파서 버전과 현재 시스템 버전 비교:

```go
func (uc *AutoRefreshUseCase) shouldEnqueueRefresh(
    codebase CodebaseRefreshInfo,
    headCommitSHA string,
    currentParserVersion string,
) bool {
    // 트리거 1: 새 커밋 감지
    if codebase.LastCommitSHA != headCommitSHA {
        return true
    }
    // 트리거 2: 파서 버전 변경
    if currentParserVersion != "" &&
       codebase.LastParserVersion != currentParserVersion {
        return true
    }
    return false
}
```

### 4. 제약조건 변경

유니크 제약조건에 파서 버전 포함, 동일 커밋에 대한 복수 분석 허용:

```sql
-- 기존: (codebase_id, commit_sha)
-- 변경: (codebase_id, commit_sha, parser_version)
CREATE UNIQUE INDEX uq_analyses_completed_commit_version
    ON analyses (codebase_id, commit_sha, parser_version)
    WHERE status = 'completed';
```

## 아키텍처

```
┌─────────────────────────────────────────────────────────────────┐
│                     Worker 시작                                  │
├─────────────────────────────────────────────────────────────────┤
│  1. 추출: runtime/debug.ReadBuildInfo()                          │
│  2. 탐색: specvital/core 모듈 버전                               │
│  3. UPSERT: system_config {key: "parser_version", value: X}      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    분석 생성                                     │
├─────────────────────────────────────────────────────────────────┤
│  analyses.parser_version = ContainerConfig에서 주입된 버전       │
│  → 분석별 이력 추적 가능                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   예약 자동 새로고침                              │
├─────────────────────────────────────────────────────────────────┤
│  1. 조회: system_config.parser_version (현재)                    │
│  2. 조회: codebases의 last_parser_version (분석에서)             │
│  3. 비교: 다르면 → 재분석 큐 등록                                │
│  4. Graceful degradation: 조회 실패 시 버전 체크 건너뛰기        │
└─────────────────────────────────────────────────────────────────┘
```

## 검토한 옵션

### Option A: 런타임 버전 추출 (선택)

Worker 시작 시 Go 빌드 정보에서 specvital/core 모듈 버전 추출.

| 측면   | 평가                                 |
| ------ | ------------------------------------ |
| 자동화 | 컴파일된 바이너리에서 자동 감지      |
| 정확성 | 실제 사용 중인 모듈 버전 반영        |
| 디버깅 | 시맨틱 버전과 릴리즈 노트 연관 가능  |
| 제한   | Go 전용, 로컬 빌드 시 `(devel)` 반환 |

### Option B: 수동 버전 설정

환경 변수 또는 설정 파일로 파서 버전 설정.

| 측면   | 평가                      |
| ------ | ------------------------- |
| 이식성 | 언어 무관                 |
| 제어   | 명시적 버전 문자열        |
| 위험   | 배포 시 설정 누락 가능    |
| 마찰   | 릴리즈마다 추가 단계 필요 |

### Option C: 콘텐츠 기반 해싱

참조 코드베이스에서 파서 출력 해시로 동작 변경 감지.

| 측면     | 평가                         |
| -------- | ---------------------------- |
| 감지     | 실제 동작 변경 감지          |
| 오버헤드 | 컴퓨팅 집약적                |
| 디버깅   | 해시로는 변경 내용 파악 불가 |
| 신뢰성   | 비결정적 엣지 케이스 존재    |

## 결과

### 긍정적

| 영역            | 이점                           |
| --------------- | ------------------------------ |
| 파서 업그레이드 | 사용자가 개선된 분석 자동 수신 |
| 운영            | 배포 시 설정 불필요            |
| 감사 추적       | 각 분석이 생성 버전과 연결     |
| 데이터 모델     | 커밋당 복수 분석 지원          |

### 부정적

| 영역         | 트레이드오프                         |
| ------------ | ------------------------------------ |
| 이식성       | Go 전용 `runtime/debug` 의존         |
| 개발         | `(devel)` 버전에 대한 폴백 로직 필요 |
| 스토리지     | 분석 행당 추가 ~20바이트             |
| 마이그레이션 | 레거시 분석은 `'legacy'` 기본값      |

### 기술적 시사점

| 측면                 | 시사점                                     |
| -------------------- | ------------------------------------------ |
| 스키마 마이그레이션  | 기존 분석에 대한 기본값 처리 필요          |
| 빌드 요구사항        | 모듈 정보 유지를 위해 `CGO_ENABLED=0` 필요 |
| Graceful Degradation | 조회 실패 시 버전 체크 건너뛰기            |
| A/B 테스트           | 동일 커밋을 다른 파서 버전으로 분석 가능   |

## 참조

- [Worker ADR-01: 스케줄 기반 재분석 아키텍처](/ko/adr/worker/01-scheduled-recollection.md)
- [Go debug.ReadBuildInfo 문서](https://pkg.go.dev/runtime/debug)
- 커밋: `a681e0d0` (infra), `290a5efa`, `aa47dab0`, `4cc6cc43` (worker)
