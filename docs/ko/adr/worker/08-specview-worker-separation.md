---
title: SpecView Worker 바이너리 분리
description: SpecViewWorker를 AnalyzerWorker와 독립 바이너리로 분리하는 ADR
---

# ADR-08: SpecView Worker 바이너리 분리

> 🇺🇸 [English Version](/en/adr/worker/08-specview-worker-separation)

| 날짜       | 작성자     | 리포지토리 |
| ---------- | ---------- | ---------- |
| 2026-01-13 | @specvital | worker     |

## 맥락

### 설계 의도 위반

Specvital 워커 아키텍처는 바이너리 분리 패턴([ADR-05](./05-worker-scheduler-separation.md)) 준수: 각 워크로드 타입별 독립 프로세스 운영 및 전용 설정과 의존성 관리.

초기 SpecView 구현에서 SpecViewWorker를 AnalyzerContainer에 통합하여 아키텍처 문제 발생.

### 통합 방식의 문제점

| 문제            | 영향                                                   |
| --------------- | ------------------------------------------------------ |
| 시크릿 오염     | Gemini API 미사용에도 Analyzer에 GEMINI_API_KEY 필요   |
| 큐 라우팅 실패  | 잘못된 워커로 라우팅 시 "Unhandled job kind" 에러 발생 |
| 스케일링 불일치 | CPU 바운드 파싱과 I/O 바운드 API 워크로드 결합         |
| 비용 예측 불가  | 토큰 기반 AI 비용과 예측 가능한 파싱 비용 혼재         |

### 워크로드 특성 비대칭

| 항목          | Analyzer           | Spec-Generator          |
| ------------- | ------------------ | ----------------------- |
| 외부 API      | 없음               | Gemini API              |
| 시크릿        | ENCRYPTION_KEY     | GEMINI_API_KEY          |
| 스케일링 특성 | CPU 바운드 (파싱)  | I/O 바운드 (API 호출)   |
| 비용 특성     | 예측 가능 (컴퓨트) | 가변적 (토큰당 과금)    |
| 타임아웃      | 짧음 (~30초)       | 김 (~10분)              |
| 장애 모드     | 메모리 고갈        | Rate limiting, API 에러 |

핵심 불일치: 테스트 파일 파싱은 결정론적 로컬 연산, 스펙 생성은 비결정론적 네트워크 의존 AI 태스크.

## 결정

**AnalyzeWorker와 SpecViewWorker를 전용 큐 및 설정 요구사항을 가진 독립 바이너리로 분리.**

### 아키텍처

```
src/cmd/
├── analyzer/main.go       # 테스트 파일 파싱 (Tree-sitter, ENCRYPTION_KEY)
├── spec-generator/main.go # AI 문서 생성 (Gemini API, GEMINI_API_KEY)
├── scheduler/main.go      # Cron 기반 작업 스케줄링
└── enqueue/main.go        # 수동 인큐 유틸리티

River Queues:
├── analyze_repository     # analyzer 바이너리 전용
└── generate_spec_document # spec-generator 바이너리 전용
```

### 바이너리 책임

**analyzer/main.go:**

- River 큐에서 `analyze_repository` 작업 소비
- 리포지토리 클론, Tree-sitter 파싱, 테스트 메타데이터 추출
- 필수: `DATABASE_URL`, `ENCRYPTION_KEY` (OAuth 토큰 복호화용)
- 불필요: `GEMINI_API_KEY`

**spec-generator/main.go:**

- River 큐에서 `generate_spec_document` 작업 소비
- Gemini API 호출로 분류 및 변환 수행 ([ADR-14](/ko/adr/14-ai-spec-generation-pipeline))
- 필수: `DATABASE_URL`, `GEMINI_API_KEY`
- 불필요: `ENCRYPTION_KEY`

### 큐 격리

각 바이너리는 지원하는 작업 종류만 등록:

```go
// analyzer/main.go
river.AddWorker(client, &AnalyzeRepositoryWorker{})
// 처리: analyze_repository

// spec-generator/main.go
river.AddWorker(client, &GenerateSpecDocumentWorker{})
// 처리: generate_spec_document
```

## 검토한 옵션

### 옵션 A: 바이너리 분리 (선택됨)

전용 큐와 설정 검증을 가진 별도 바이너리 (`cmd/analyzer`, `cmd/spec-generator`).

**장점:**

- 시크릿 격리 - 각 바이너리는 필요한 시크릿만 로드
- 독립 스케일링 - AI 워크로드와 파싱 워크로드 별도 확장
- 비용 귀속 - 컴퓨트 vs API 비용 명확 분리
- 장애 격리 - Gemini rate limit이 테스트 파싱에 영향 없음
- 큐 명확성 - 각 큐는 정확히 하나의 소비자 바이너리에 매핑

**단점:**

- 빌드, 배포, 모니터링할 바이너리 2개
- 공유 코드를 internal 패키지로 추출 필요
- 공통 설정 중복

### 옵션 B: 런타임 모드 단일 바이너리

`--mode=analyzer` 또는 `--mode=spec-generator` 플래그를 가진 단일 바이너리.

**장점:**

- 단일 빌드 아티팩트
- 간단한 CI/CD 파이프라인

**단점:**

- 모든 의존성 포함 (analyzer 모드에서도 Gemini SDK 로드)
- 런타임 설정 오류 위험
- 시크릿 검증이 시작 시점이 아닌 런타임에 수행
- 바이너리 크기 비대

### 옵션 C: 고루틴 결합 프로세스

단일 프로세스에서 양쪽 워커를 별도 고루틴으로 실행.

**장점:**

- 가장 단순한 배포
- 공유 커넥션 풀

**단점:**

- 시크릿 노출 - 모든 인스턴스가 양쪽 키 보유
- 독립 스케일링 불가
- CPU 바운드와 I/O 바운드 태스크 간 리소스 경합
- 장애 결합
- [ADR-05](./05-worker-scheduler-separation.md) 패턴 위반

## 결과

### 긍정적

| 영역        | 이점                                                                     |
| ----------- | ------------------------------------------------------------------------ |
| 보안        | Analyzer는 GEMINI_API_KEY 불접촉; spec-generator는 ENCRYPTION_KEY 불접촉 |
| 스케일링    | AI 큐 깊이 기반 spec-generator 독립 확장                                 |
| 비용 가시성 | Gemini API 비용이 spec-generator 서비스 메트릭에 격리                    |
| 신뢰성      | Gemini 장애가 테스트 파싱 파이프라인에 영향 없음                         |
| 타임아웃    | Analyzer: 30초 (빠른 실패), Spec-generator: 10분 (AI 허용)               |
| PaaS 최적화 | 워크로드 프로필별 다른 인스턴스 크기 사용                                |

### 부정적

| 영역            | 트레이드오프                                |
| --------------- | ------------------------------------------- |
| 운영 복잡도     | 별도 헬스 체크를 가진 2개 서비스 모니터링   |
| 빌드 파이프라인 | 2개 Docker 이미지 빌드 및 푸시              |
| 공유 코드       | 공통 유틸리티를 internal 패키지로 추출 필요 |
| 디버깅          | 관련 작업의 크로스 서비스 트레이싱          |

## 참조

- [ADR-05: Worker-Scheduler 프로세스 분리](./05-worker-scheduler-separation.md)
- [ADR-14: AI 기반 스펙 문서 생성 파이프라인](/ko/adr/14-ai-spec-generation-pipeline.md)
- [ADR-04: 큐 기반 비동기 처리](/ko/adr/04-queue-based-async-processing.md)
- 커밋 `f3fae45`: refactor(worker): separate AnalyzeWorker and SpecViewWorker into independent binaries
- 커밋 `3cfee6f`: fix(queue): isolate dedicated queues per worker to resolve Unhandled job kind error
