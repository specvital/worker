---
title: GitHub API 캐시 테이블
description: GitHub API 응답을 위한 데이터베이스 캐시 테이블에 관한 ADR
---

# ADR-18: GitHub API 캐시 테이블

> 🇺🇸 [English Version](/en/adr/18-github-api-cache-tables.md)

| 날짜       | 작성자     | 리포지토리 |
| ---------- | ---------- | ---------- |
| 2025-12-24 | @specvital | infra, web |

## 컨텍스트

GitHub API의 엄격한 Rate Limit: 인증된 사용자 기준 시간당 5,000 요청. 대시보드 리포지토리 선택 기능에서 UX 및 안정성 이슈 발생.

| 이슈            | 영향                                   |
| --------------- | -------------------------------------- |
| Rate Limit 소진 | 한도 도달 시 대시보드 사용 불가        |
| 반복 API 호출   | 대시보드 방문마다 리포지토리 목록 조회 |
| 지연 시간 변동  | GitHub API 응답 시간 100-500ms         |
| 중복 요청 낭비  | 여러 브라우저 탭에서 동일 API 호출     |

### UX 문제점

- 느린 초기 로드 (리포지토리 목록 대기 200-500ms)
- 리포지토리가 많은 사용자의 빠른 Rate Limit 소진
- 데이터 신선도 가시성 부재

[ADR-09: GitHub App 통합 전략](/ko/adr/09-github-app-integration.md)에서 확립한 5,000/hr Rate Limit 제약 조건 확장.

## 결정

**하이브리드 정규화 및 사용자 제어 새로고침 전략의 데이터베이스 캐시 채택.**

### 스키마

```sql
-- 공유 조직 메타데이터 (전역 중복 제거)
CREATE TABLE github_organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  github_org_id BIGINT NOT NULL UNIQUE,
  login VARCHAR(255) NOT NULL,
  avatar_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 사용자-조직 N:N 관계
CREATE TABLE user_github_org_memberships (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  github_org_id UUID NOT NULL REFERENCES github_organizations(id) ON DELETE CASCADE,
  CONSTRAINT uq_user_org UNIQUE (user_id, github_org_id)
);

-- 사용자별 통합 리포지토리 캐시
CREATE TABLE user_github_repositories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  github_repo_id BIGINT NOT NULL,
  full_name VARCHAR(511) NOT NULL,
  source_type VARCHAR(50) NOT NULL, -- 'personal' | 'organization'
  source_org_id UUID REFERENCES github_organizations(id),
  CONSTRAINT uq_user_repo UNIQUE (user_id, github_repo_id)
);
```

### 접근 패턴

```
사용자 요청 → 캐시 확인 → 히트? → 캐시 데이터 반환 (즉시)
                    ↓ 미스
              Singleflight 그룹
                    ↓
              GitHub API 호출 → 저장 → 반환
```

### 새로고침 전략

- **캐시 우선**: GitHub API 호출 전 DB 확인
- **사용자 제어**: `?refresh=true`로 무효화 강제
- **TTL 없음**: 명시적 새로고침까지 캐시 유지
- **Singleflight**: 중복 동시 요청 방지

## 검토한 옵션

### 옵션 A: 캐시 없음 (직접 API)

| 장점             | 단점                  |
| ---------------- | --------------------- |
| 항상 최신 데이터 | 빠른 Rate Limit 소진  |
| 단순함           | 요청당 200-500ms 지연 |
|                  | 소진 시 연쇄 장애     |

**기각**: 용납할 수 없는 UX 및 Rate Limit 위험.

### 옵션 B: TTL 기반 캐시

자동 시간 기반 만료 캐시 (예: 15분 TTL).

| 장점                  | 단점                          |
| --------------------- | ----------------------------- |
| 자동 신선도 보장      | TTL 튜닝 복잡성               |
| 예측 가능한 부실 범위 | TTL 경계에서 캐시 스탬피드    |
| 업계 표준 패턴        | 백그라운드 새로고침 작업 필요 |

**기각**: TTL 경계가 스탬피드 위험 생성; 임의 값이 실제 변경 빈도와 불일치.

### 옵션 C: 사용자 제어 새로고침 (채택)

사용자가 명시적으로 새로고침할 때까지 캐시 유지.

| 장점                 | 단점                              |
| -------------------- | --------------------------------- |
| 즉시 로드 (0ms 히트) | 데이터 부실 가능                  |
| 사용자 통제권        | 새로고침 필요성 인지 필요         |
| 최소 API 소비        | 새 리포지토리 새로고침까지 미표시 |
| TTL 복잡성 없음      | 사용자 증가에 따른 저장 공간      |

**채택**: 주요 사용 사례(기존 리포지토리 선택)에 최적 UX 제공.

### 옵션 D: Redis 캐시

외부 Redis를 리포지토리 데이터에 활용.

| 장점             | 단점                  |
| ---------------- | --------------------- |
| 서브밀리초 읽기  | 추가 인프라           |
| 내장 TTL         | 재시작 시 데이터 손실 |
| 인스턴스 간 공유 | 운영 복잡성           |

**기각**: 과잉 엔지니어링; PostgreSQL로 충분.

## 결과

**긍정적:**

- 캐시에서 리포지토리 목록 즉시 표시
- 명시적 새로고침 또는 첫 방문 시에만 API 호출
- 데이터 신선도에 대한 사용자 통제
- 하이브리드 정규화로 사용자 간 조직 데이터 공유
- Singleflight로 중복 진행 중 요청 방지
- GitHub 장애 중에도 캐시가 요청 처리

**부정적:**

- 새 리포지토리 새로고침까지 미표시 (새로고침 버튼으로 완화)
- 사용자당 저장 비용 (사용자 삭제 시 정리)
- 새로고침 패턴 학습 필요 (온보딩 툴팁)

## 스키마 설계 근거

### 하이브리드 정규화

| 테이블                              | 근거                                                                      |
| ----------------------------------- | ------------------------------------------------------------------------- |
| 전역 `github_organizations`         | 조직은 공유 엔티티; 동일 조직 1000명 사용자가 메타데이터 1000번 복제 방지 |
| 사용자별 `user_github_repositories` | 가시성이 사용자별로 다름; 권한 차이                                       |
| 연결 테이블                         | "이 사용자가 어떤 조직에 속해 있는지" 쿼리를 위한 깔끔한 N:N              |

### UNIQUE (user_id, github_repo_id)

- 중복 캐시 항목 방지
- Upsert 패턴 활성화 (`INSERT ... ON CONFLICT`)
- 참조 무결성 유지

### source_type 컬럼

- UI 그룹핑: "개인" vs "조직" 섹션
- 조직 전용 뷰 쿼리 필터링
- 향후 권한 로직 차별화

### TTL 컬럼 없음

- 사용자 제어 새로고침으로 자동 만료 불필요
- TTL 확인 없는 단순 쿼리
- 적용을 위한 백그라운드 작업 불필요

## 참조

- [커밋 16056864](https://github.com/specvital/infra/commit/16056864) - 캐시 테이블 마이그레이션
- [커밋 a4db76e8](https://github.com/specvital/web/commit/a4db76e8) - GitHub API 모듈
- [ADR-09: GitHub App 통합](/ko/adr/09-github-app-integration.md)
