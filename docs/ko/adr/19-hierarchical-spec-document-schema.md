---
title: 계층적 스펙 문서 스키마
description: BDD 정렬 사양 문서 저장을 위한 4-테이블 정규화 데이터베이스 스키마 ADR
---

# ADR-19: 계층적 스펙 문서 스키마

> 🇺🇸 [English Version](/en/adr/19-hierarchical-spec-document-schema.md)

| 날짜       | 작성자     | 레포지토리         |
| ---------- | ---------- | ------------------ |
| 2026-01-12 | @specvital | infra, worker, web |

## 컨텍스트

기존 `spec_view_cache` 테이블은 AI 변환 결과를 단순 캐싱하기 위한 플랫 키-값 저장소. Document View 기능 요구사항:

1. **비즈니스 도메인 기반 계층적 구조** - BDD/Specification 개념 정렬
2. **레벨별 독립 쿼리** - 전체 behavior 로딩 없이 domain만 조회
3. **테스트 케이스 추적성** - 원본 분석 결과로 역추적
4. **콘텐츠 해시 기반 캐싱** - AI 모델 버전 인식

플랫 캐시 구조로는 사양 문서의 자연스러운 계층 표현 불가: Domain → Feature → Behavior.

### 요구사항

| 요구사항         | 설명                                             |
| ---------------- | ------------------------------------------------ |
| 계층적 표현      | BDD 개념에 맞는 Domain → Feature → Behavior 구조 |
| 레벨별 독립 쿼리 | 전체 behavior 없이 domain 개요만 조회            |
| 캐스케이드 삭제  | analysis 삭제 시 전체 문서 트리 전파             |
| 테스트 추적성    | behavior에서 원본 test_cases로 네비게이션        |
| 캐시 효율성      | 콘텐츠 해시 키로 중복 AI API 호출 방지           |
| 모델 버전 관리   | 다른 AI 모델 버전은 다른 문서 생성               |

### 제약사항

| 제약사항             | 영향                               |
| -------------------- | ---------------------------------- |
| PostgreSQL 백엔드    | 관계형 스키마 패턴 사용 필수       |
| 기존 Analysis 스키마 | analyses 테이블 외래키 필요        |
| sqlc 코드 생성       | VIEW 미사용, 인라인 JOIN 쿼리 선호 |

## 결정

**BDD 사양 구조에 정렬된 4-테이블 정규화 계층 스키마 채택.**

```sql
-- Level 0: Document (분석당 1개)
CREATE TABLE spec_documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  analysis_id UUID NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
  content_hash BYTEA NOT NULL,
  language VARCHAR(10) NOT NULL DEFAULT 'en',
  executive_summary TEXT,
  model_id VARCHAR(100) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_spec_documents_hash_lang_model UNIQUE (content_hash, language, model_id)
);

-- Level 1: Domain (비즈니스 분류)
CREATE TABLE spec_domains (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id UUID NOT NULL REFERENCES spec_documents(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  classification_confidence NUMERIC(3,2),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Level 2: Feature (기능 그룹)
CREATE TABLE spec_features (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id UUID NOT NULL REFERENCES spec_domains(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Level 3: Behavior (리프 테스트 사양)
CREATE TABLE spec_behaviors (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_id UUID NOT NULL REFERENCES spec_features(id) ON DELETE CASCADE,
  source_test_case_id UUID REFERENCES test_cases(id) ON DELETE SET NULL,
  original_name VARCHAR(2000) NOT NULL,
  converted_description TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 핵심 설계 결정

| 결정                                             | 근거                                                        |
| ------------------------------------------------ | ----------------------------------------------------------- |
| `content_hash + language + model_id` 유니크 제약 | 중복 방지 캐시 키; 동일 콘텐츠도 언어/모델 다르면 다른 문서 |
| `classification_confidence` 도메인 레벨만        | AI가 Phase 1에서 도메인 할당; feature는 결정론적 그룹핑     |
| `source_test_case_id` SET NULL 삭제              | test_case 정리 시에도 스펙 문서 유지하며 추적성 보존        |
| 레벨별 `sort_order`                              | AI 할당 순서 보존으로 일관된 UI 렌더링                      |
| VIEW 미사용                                      | sqlc가 타입 안전 Go 코드 생성; 인라인 JOIN 선호             |

### 테이블 관계

```
spec_documents (문서 레벨)
    │
    │ content_hash + language + model_id → unique
    │ analysis_id → FK to analyses (CASCADE delete)
    │
    └──► spec_domains (비즈니스 도메인 분류)
            │
            │ document_id → FK to spec_documents (CASCADE delete)
            │ classification_confidence → AI 신뢰도 점수
            │
            └──► spec_features (기능 그룹핑)
                    │
                    │ domain_id → FK to spec_domains (CASCADE delete)
                    │
                    └──► spec_behaviors (개별 테스트 동작)
                            │
                            │ feature_id → FK to spec_features (CASCADE delete)
                            │ source_test_case_id → FK to test_cases (SET NULL)
```

## 고려한 옵션

### 옵션 A: 계층적 4-테이블 정규화 구조 (선택됨)

Document → Domain → Feature → Behavior 계층을 표현하는 적절한 외래키 관계의 4개 테이블.

| 장점                                | 단점                            |
| ----------------------------------- | ------------------------------- |
| 레벨별 독립 쿼리                    | JOIN 필요한 복잡한 쿼리         |
| 캐스케이드 삭제 포함 적절한 FK 제약 | 4개 테이블 스키마 유지보수 부담 |
| BDD/Specification 개념 정렬         | INSERT 시 4번 순차 작업 필요    |
| 집계 JOIN으로 통계                  | 레벨별 sort_order 컬럼 필요     |
| FK로 테스트 케이스 추적성           |                                 |

### 옵션 B: 단일 비정규화 테이블

nullable 부모 컬럼과 `item_type` 구분자로 모든 계층 레벨을 하나의 테이블에 저장.

| 장점                    | 단점                                    |
| ----------------------- | --------------------------------------- |
| 단순 스키마 (1 테이블)  | 레벨별 제약조건 적용 불가               |
| 쉬운 쓰기 (단일 INSERT) | domain vs feature 필드 타입 안전성 없음 |
|                         | "모든 domain" 효율적 쿼리 불가          |
|                         | 계층 탐색에 재귀 CTE 필요               |

**기각**: 계층 적절히 표현 불가; 복잡한 필터링 없이 레벨별 쿼리 불가능.

### 옵션 C: JSON 컬럼 저장

전체 문서를 단일 테이블의 JSON blob으로 저장.

| 장점                   | 단점                               |
| ---------------------- | ---------------------------------- |
| 스키마 유연성          | domain/feature 독립 쿼리 불가      |
| 문서당 단일 행         | test_cases FK 제약 없음            |
| 문서 저장에 자연스러움 | 집계 쿼리 어려움 (도메인별 카운트) |
|                        | JSON 경로 쿼리가 관계형보다 비효율 |

**기각**: 관계형 쿼리 기능 제거; 독립적 도메인/feature 통계 불가능.

### 옵션 D: 2-테이블 구조 (Document + Behaviors)

중간 계층 없이 최상위 document와 리프 behaviors만 저장.

| 장점                        | 단점                                       |
| --------------------------- | ------------------------------------------ |
| 4 테이블보다 단순           | domain/feature 일급 엔티티 상실            |
| document-behavior 직접 관계 | 고유 domain 효율적 조회 불가               |
|                             | domain/feature 카운트에 GROUP BY 필요      |
|                             | 도메인 레벨 메타데이터 없음 (신뢰도, 설명) |

**기각**: 도메인/feature 분류 컨텍스트 상실; 통계에 전체 문서 스캔 필요.

## 결과

### 긍정적

| 영역            | 이점                                                        |
| --------------- | ----------------------------------------------------------- |
| 쿼리 유연성     | 개요용 domain 조회, 필요 시 feature로 확장                  |
| BDD 정렬        | 스키마가 사양 문서 멘탈 모델 반영                           |
| 테스트 추적성   | `source_test_case_id` FK로 "소스 보기" 네비게이션           |
| 캐스케이드 삭제 | DELETE analysis → document → domains → features → behaviors |
| 캐시 효율성     | (content_hash, language, model_id)로 중복 AI 호출 방지      |
| 통계            | 매터리얼라이즈드 뷰 없이 레벨별 COUNT/GROUP BY              |
| 타입 안전성     | sqlc가 테이블별 고유 타입 생성                              |

### 부정적

| 영역          | 트레이드오프                              | 완화책                                  |
| ------------- | ----------------------------------------- | --------------------------------------- |
| 쿼리 복잡성   | 전체 문서 조회에 다중 테이블 JOIN         | sqlc 명명 쿼리로 JOIN 캡슐화            |
| 쓰기 복잡성   | 문서당 트랜잭션 내 4번 INSERT             | 단일 트랜잭션, 배치 INSERT              |
| 스키마 범위   | 4개 테이블 유지보수, 마이그레이션, 인덱싱 | 명확한 테이블 책임 분리                 |
| 정렬          | 레벨별 sort_order 컬럼                    | AI 파이프라인이 생성 시 sort_order 할당 |
| 신뢰도 비대칭 | classification_confidence 도메인 레벨만   | 필요 시 feature 신뢰도 추가 가능        |

### 인덱스

| 인덱스                            | 목적                             |
| --------------------------------- | -------------------------------- |
| `idx_spec_documents_analysis`     | analysis_id로 빠른 조회          |
| `idx_spec_domains_document_sort`  | 정렬된 domain 조회               |
| `idx_spec_features_domain_sort`   | 정렬된 feature 조회              |
| `idx_spec_behaviors_feature_sort` | 정렬된 behavior 조회             |
| `idx_spec_behaviors_source`       | 테스트 케이스 추적용 부분 인덱스 |

## 참고 자료

- [ADR-14: AI 기반 스펙 문서 생성 파이프라인](/ko/adr/14-ai-spec-generation-pipeline.md)
- [ADR-13: 빌링 및 쿼터 아키텍처](/ko/adr/13-billing-quota-architecture.md)
- [Worker ADR-08: SpecView Worker 바이너리 분리](/ko/adr/worker/08-specview-worker-separation.md)
- [Commit 38a33ad](https://github.com/specvital/infra/commit/38a33ad) - feat(db): replace spec_view_cache with hierarchical spec document schema
