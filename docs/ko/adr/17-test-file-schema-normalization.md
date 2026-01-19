---
title: 테스트 파일 스키마 정규화
description: 파일 레벨 메타데이터를 위한 test_files 테이블 도입 스키마 정규화 ADR
---

# ADR-17: 테스트 파일 스키마 정규화

> 🇬🇧 [English Version](/en/adr/17-test-file-schema-normalization.md)

| 날짜       | 작성자       | 관련 레포          |
| ---------- | ------------ | ------------------ |
| 2026-01-19 | @KubrickCode | infra, worker, web |

## 컨텍스트

### 파일 레벨 메타데이터 문제

기존 테스트 데이터 스키마의 3-tier 계층 구조:

```
analyses → test_suites (file_path, framework) → test_cases
```

두 가지 구조적 결함 존재:

| 문제          | 설명                                                                                           |
| ------------- | ---------------------------------------------------------------------------------------------- |
| 데이터 중복   | `file_path`와 `framework`가 test_suite마다 저장되어 단일 파일에 여러 suite가 있을 때 중복 발생 |
| 누락된 엔티티 | 파일 레벨 메타데이터를 저장할 논리적 위치 부재                                                 |

### DomainHints 요구사항

AI 기반 SpecView 생성 파이프라인([ADR-14](/ko/adr/14-ai-spec-generation-pipeline.md))은 테스트 파일에서 추출된 `DomainHints`([Core ADR-16](/ko/adr/core/16-domain-hints-extraction.md))를 활용한 도메인 분류 필요. 이 힌트는 본질적으로 파일 레벨 데이터:

```go
type DomainHints struct {
    Imports []string  // 파일별 import 문
    Calls   []string  // 파일별 함수 호출
}
```

스키마 정규화 없이 `domain_hints`를 `test_suites`에 저장 시:

- 파일 내 각 suite마다 JSONB 데이터 중복
- 힌트 변경 시 업데이트 이상 발생
- suite 수에 비례한 스토리지 낭비

### 제약 조건

| 제약           | 영향                                               |
| -------------- | -------------------------------------------------- |
| 하위 호환성    | 기존 모든 분석 데이터 손실 없이 마이그레이션 필수  |
| CASCADE DELETE | FK 관계를 통한 전체 계층 정리 필수                 |
| 쿼리 성능      | Web API 쿼리 성능 저하 최소화                      |
| 저장 흐름      | Worker가 suite 전에 file 먼저 삽입 (순차적 의존성) |

## 결정

**`analyses`와 `test_suites` 사이에 `test_files` 테이블 도입으로 3-tier에서 4-tier 스키마로 정규화.**

### 스키마 설계

**신규 test_files 테이블:**

```sql
CREATE TABLE test_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id UUID NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    framework TEXT NOT NULL,
    domain_hints JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (analysis_id, file_path)
);

CREATE INDEX idx_test_files_analysis_id ON test_files(analysis_id);
```

**수정된 test_suites:**

```sql
-- 마이그레이션 후:
ALTER TABLE test_suites
    ADD COLUMN file_id UUID REFERENCES test_files(id) ON DELETE CASCADE,
    DROP COLUMN analysis_id,
    DROP COLUMN file_path,
    DROP COLUMN framework;
```

### 새로운 계층 구조

```
analyses
    └── test_files (file_path, framework, domain_hints)
            └── test_suites (suite_name)
                    └── test_cases (test_name, status)
```

### 마이그레이션 전략

| 단계 | 작업                                         | 위험도                |
| ---- | -------------------------------------------- | --------------------- |
| 1    | `test_files` 테이블 생성                     | 없음                  |
| 2    | 데이터 채우기: `INSERT FROM SELECT DISTINCT` | 낮음 - 멱등성         |
| 3    | test_suites에 `file_id` FK 추가              | 중간                  |
| 4    | 모든 test_suites가 유효한 file_id 보유 검증  | 없음                  |
| 5    | test_suites에서 중복 컬럼 제거               | 높음 - 되돌릴 수 없음 |
| 6    | file_id에 NOT NULL 제약 추가                 | 없음                  |

**롤백 전략**: 5단계 이전은 롤백 용이. 5단계 이후는 데이터 재구성 필요.

## 검토 옵션

### Option A: test_files 정규화 레이어 (선택)

파일 레벨 데이터 정규화를 위한 중간 `test_files` 테이블 도입.

**장점:**

| 이점          | 설명                                                 |
| ------------- | ---------------------------------------------------- |
| 데이터 무결성 | 파일 메타데이터의 단일 진실 공급원                   |
| 스토리지 효율 | file_path, framework, domain_hints 중복 제거         |
| FK 계층       | 깔끔한 CASCADE DELETE 체인                           |
| 향후 확장성   | 파일 레벨 메트릭(커버리지, 복잡도)의 자연스러운 위치 |

**단점:**

| 트레이드오프      | 완화 방안                               |
| ----------------- | --------------------------------------- |
| 쿼리 복잡도       | JOIN 하나 추가; 무결성을 위해 수용 가능 |
| 마이그레이션 노력 | 일회성 무손실 마이그레이션              |
| 저장 흐름 변경    | Worker가 suite 전에 file 삽입           |

### Option B: test_suites에 domain_hints 저장

기존 test_suites 테이블에 직접 `domain_hints` 컬럼 추가.

**장점:**

- 마이그레이션 불필요
- 쿼리 변경 불필요

**단점:**

| 문제          | 심각도                                 |
| ------------- | -------------------------------------- |
| 데이터 중복   | 높음 - suite마다 힌트 반복             |
| 업데이트 이상 | 높음 - 힌트 변경 시 다중 업데이트 필요 |
| 스토리지 낭비 | 중간 - JSONB 중복                      |
| 3NF 위반      | 아키텍처 부채                          |

**판정:** 기각. 3NF 위반; 업데이트 이상 및 스토리지 낭비 발생.

### Option C: 별도 file_domain_hints 테이블

test_suites 수정 없이 도메인 힌트 전용 별도 테이블 생성.

**장점:**

- 힌트만 별도 정규화
- 추가적 변경만 필요

**단점:**

| 문제             | 심각도                              |
| ---------------- | ----------------------------------- |
| 병렬 구조        | 높음 - file_path가 두 테이블에 존재 |
| 참조 무결성 부재 | 중간 - 힌트와 suite 연결 끊어짐     |
| 기존 중복 미해결 | 높음 - file_path 여전히 중복        |

**판정:** 기각. 기존 중복 미해결; 아키텍처 불일치 발생.

## 결과

### 긍정적

**데이터 무결성:**

- `file_path`, `framework`, `domain_hints`의 단일 진실 공급원
- `(analysis_id, file_path)` UNIQUE 제약으로 중복 방지
- 깔끔한 CASCADE DELETE: analyses → test_files → test_suites → test_cases

**AI 파이프라인 통합:**

- `domain_hints`가 파일 레벨에 자연스러운 위치 확보
- Core ADR-16의 파일 레벨 추출 모델과 일치
- AI 파이프라인에서 파일별 캐싱 가능

**향후 확장성:**

- 파일 레벨 커버리지 메트릭 추가 가능
- 파일 복잡도 점수 추가 가능
- 파일별 분석 상태 관리 가능

### 부정적

**쿼리 복잡도:**

- 모든 테스트 쿼리에 test_files를 통한 추가 JOIN 필요
- 변경 예시:

```sql
-- 이전:
SELECT ts.file_path FROM test_suites ts
WHERE ts.analysis_id = $1

-- 이후:
SELECT tf.file_path FROM test_suites ts
JOIN test_files tf ON ts.file_id = tf.id
WHERE tf.analysis_id = $1
```

**마이그레이션 노력:**

- 일회성 조율된 마이그레이션 필요
- worker와 web 서비스 모두 인터페이스 업데이트 필요
- Repository 패턴 구현 변경

**Worker 저장 흐름:**

- suite 전에 file 먼저 삽입 필수 (순차적 의존성)
- 2단계 쓰기: `saveFiles()` 후 `saveSuitesBatch()`
- 저장 메서드 시그니처 변경

### 기술적 함의

| 측면              | 함의                                                        |
| ----------------- | ----------------------------------------------------------- |
| JSONB 저장        | domain_hints는 PostgreSQL JSONB 타입 사용                   |
| 인덱스 전략       | analysis_id 기준 주요 조회                                  |
| 쿼리 패턴         | JOIN 체인: test_cases → test_suites → test_files → analyses |
| Worker 인터페이스 | `saveSuitesBatch()`가 analysis_id 대신 file_id 사용         |

## 참조

- [ADR-14: AI 기반 스펙 문서 생성 파이프라인](/ko/adr/14-ai-spec-generation-pipeline.md) - DomainHints 요구사항 동기
- [Core ADR-16: 도메인 힌트 추출 시스템](/ko/adr/core/16-domain-hints-extraction.md) - DomainHints 구조 정의
- [Worker ADR-07: Repository 패턴](/ko/adr/worker/07-repository-pattern.md) - 저장 구현 영향
