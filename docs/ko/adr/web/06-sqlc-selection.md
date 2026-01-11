---
title: SQLc 선택
description: Go 백엔드에서 컴파일 타임 타입 안전 데이터베이스 접근을 위한 SQLc 선택 ADR
---

# ADR-06: SQLc 선택

> [English Version](/en/adr/web/06-sqlc-selection.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2024-12-18 | @KubrickCode | web        |

## Context

### 데이터베이스 접근 계층 요구사항

웹 플랫폼은 다음 기준을 충족하는 데이터베이스 접근 전략이 필요:

1. **타입 안전성**: SQL 쿼리에 대한 컴파일 타임 오류 감지
2. **SQL 제어**: PostgreSQL 기능 완전 활용 (LATERAL JOIN, CTE, 윈도우 함수, 커서 페이지네이션)
3. **성능**: 최소한의 런타임 오버헤드
4. **클린 아키텍처 호환**: 생성된 코드가 port/adapter 패턴에 적합
5. **PostgreSQL 전용 기능 지원**: enum, 배열, UUID, JSONB 네이티브 처리

### ORM의 한계

전통적인 ORM은 객체 지향 인터페이스 뒤에 SQL을 추상화. CRUD 작업은 단순해지지만, 복잡한 쿼리에서 마찰 발생:

| 쿼리 패턴                 | ORM 접근 방식            | 원시 SQL 접근 방식 |
| ------------------------- | ------------------------ | ------------------ |
| LATERAL JOIN              | 미지원 또는 escape hatch | 네이티브           |
| 복합 키 커서 페이지네이션 | 복잡한 커스텀 코드       | 직관적             |
| 동적 정렬 순서            | 다중 쿼리 메서드         | CASE 표현식        |
| PostgreSQL 전용 타입      | 수동 타입 등록           | 네이티브 지원      |

### 기존 인프라

프로젝트는 이미 다음을 채택:

- **PostgreSQL**: 주 데이터베이스 (프로덕션에서 NeonDB)
- **River**: PostgreSQL 기반 작업 큐 (트랜잭션 일관성을 위해 선택)
- **pgx/v5**: PostgreSQL 드라이버 (커넥션 풀링, 네이티브 타입)

데이터베이스 접근 계층은 이 스택과 원활하게 통합되어야 함.

### 마이그레이션 아키텍처

데이터베이스 마이그레이션은 별도의 `infra` 리포지토리에서 관리되며, `web`과 `worker` 서비스가 공유:

- Web 서비스는 데이터베이스 **연결**만 필요, 마이그레이션 관리 불필요
- 스키마는 외부에서 관리되는 단일 진실 원천
- 마이그레이션 기능이 내장된 도구(GORM, Ent)는 불필요한 복잡성만 추가

### AI 기반 개발 고려사항

AI 기반 개발 시대(Claude Code, GitHub Copilot 등)에서:

- **AI는 SQL을 자연스럽게 작성**: LLM은 최적화된 SQL 쿼리를 직접 생성하는 데 탁월
- **추상화 오버헤드 없음**: AI는 생산성을 위해 ORM 추상화가 필요하지 않음
- **인간 가독성**: 개발자는 생성된 메서드명에서 쿼리 의도 유추 가능 (예: `GetPaginatedRepositoriesByRecent`)
- **양방향 명확성**: AI가 원시 SQL 작성 → SQLc가 타입 메서드 생성 → 개발자가 의도 이해

## Decision

**컴파일 타임 타입 안전 데이터베이스 접근을 위해 SQLc와 pgx/v5 채택.**

핵심 원칙:

1. **SQL 우선**: ORM 추상화 없이 최적화된 SQL 쿼리 직접 작성
2. **컴파일 타임 안전성**: 생성된 Go 코드를 통해 런타임 전에 타입 오류 포착
3. **무추상화 오버헤드**: 런타임에 쿼리 빌딩이나 리플렉션 없음
4. **PostgreSQL 네이티브**: 최대 성능을 위한 pgx 드라이버 직접 통합

설정:

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "internal/db/schema.sql"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
```

## Options Considered

### Option A: SQLc (선택됨)

**작동 방식:**

1. 어노테이션이 포함된 `.sql` 파일에 SQL 쿼리 작성
2. `sqlc generate` 실행하여 타입 안전 Go 코드 생성
3. 적절한 타입으로 생성된 함수 호출

**장점:**

- **컴파일 타임 타입 안전성**: 컬럼/타입 불일치가 빌드 시 포착
- **완전한 SQL 제어**: escape hatch 없이 모든 PostgreSQL 기능 사용 가능
- **무런타임 오버헤드**: 리플렉션 없음, 쿼리 빌딩 없음
- **PostgreSQL 네이티브 타입**: enum, 배열, UUID가 pgx와 원활하게 작동
- **깔끔한 생성 코드**: 관용적 Go, 이해하고 디버그하기 쉬움
- **활발한 커뮤니티**: GitHub 스타 16,600+, 정기 릴리스

**단점:**

- **동적 쿼리 제한**: boolean 플래그 패턴 또는 다중 쿼리 필요
- **SQL 지식 필요**: 팀이 SQL을 직접 작성하고 최적화해야 함 (AI 지원으로 완화)
- **재생성 필요**: 스키마 변경 시 `sqlc generate` 실행 필요
- **마이그레이션 미지원**: 마이그레이션은 별도 관리 (infra 리포지토리 패턴에 적합)

### Option B: GORM

**작동 방식:**

- 런타임 리플렉션 기반 ORM
- 태그가 있는 구조체 정의, ORM이 쿼리 생성
- 자동 마이그레이션, 연관관계, 훅

**장점:**

- 가장 큰 Go ORM 커뮤니티 (스타 39,000+)
- 기능이 풍부한 생태계
- ORM에 익숙한 개발자 쉬운 온보딩

**단점:**

- **30-50% 성능 오버헤드**: 리플렉션 기반 쿼리 빌딩
- **런타임 오류**: 컬럼 불일치가 런타임에서만 발견
- **복잡한 쿼리 제한**: LATERAL JOIN, CTE는 원시 SQL escape 필요
- **N+1 쿼리 문제**: 명시적 프리로딩 없이 쉽게 발생
- **타입 안전성 부재**: 구조체 태그가 컴파일 타임에 검증되지 않음

### Option C: Ent (Facebook)

**작동 방식:**

- Facebook의 코드 생성 기반 ORM
- Go에서 스키마 정의, CRUD 연산 생성
- 그래프 기반 관계 탐색

**장점:**

- 컴파일 타임 타입 안전성 (SQLc와 유사)
- 엔티티 관계의 우아한 처리
- 리플렉션 오버헤드 없음

**단점:**

- **가파른 학습 곡선**: 커스텀 DSL과 그래프 개념
- **복잡한 커스텀 쿼리**: "Break the glass"로 원시 SQL 사용 필요
- **생성 코드 비대화**: 엔티티 그래프를 위한 많은 생성 파일
- **PostgreSQL 전용 기능**: 고급 기능에 우회 방법 필요

### Option D: Bun

**작동 방식:**

- ORM 기능이 있는 SQL 우선 쿼리 빌더
- `database/sql` 위의 얇은 레이어
- 명시적 설계

**장점:**

- 우수한 성능 (원시 SQL에 가까움)
- 좋은 PostgreSQL 지원
- GORM보다 적은 추상화

**단점:**

- **컴파일 타임 안전성 없음**: 런타임에 쿼리 오류 발생
- **작은 커뮤니티**: SQLc의 16,000 대비 ~4,000 스타
- **제한된 타입 추론**: 수동 구조체 매핑 필요

### Option E: 원시 database/sql

**작동 방식:**

- 표준 라이브러리 접근 방식
- 수동 쿼리 작성 및 행 스캐닝
- 완전한 SQL 제어

**장점:**

- 의존성 없음
- 최대 성능
- 완전한 제어

**단점:**

- **타입 안전성 없음**: 컬럼 불일치에 대한 런타임 오류
- **많은 보일러플레이트**: 모든 쿼리에 수동 구조체 스캐닝
- **유지보수 부담**: 스키마 변경 시 모든 곳에서 수동 업데이트 필요
- **오류 발생 쉬움**: 컬럼을 놓치거나 이름을 잘못 입력하기 쉬움

## Implementation Details

### 쿼리 구성

```
queries/
├── analysis.sql      # 분석 관련 쿼리
├── auth.sql          # 인증 쿼리
├── bookmark.sql      # 사용자 북마크
├── github.sql        # GitHub 저장소 데이터
├── github_app.sql    # GitHub App 설치
├── river_job.sql     # 작업 큐 쿼리
└── user_analysis_history.sql
```

### 복잡한 쿼리 예시

프로젝트는 ORM이 우아하게 처리할 수 없는 고급 PostgreSQL 기능 사용:

```sql
-- LATERAL JOIN과 동적 정렬을 사용한 커서 기반 페이지네이션
SELECT
    c.id AS codebase_id,
    c.owner,
    c.name,
    a.id AS analysis_id,
    a.completed_at AS analyzed_at,
    a.total_tests
FROM codebases c
JOIN LATERAL (
    SELECT id, commit_sha, completed_at, total_tests
    FROM analyses
    WHERE codebase_id = c.id AND status = 'completed'
    ORDER BY created_at DESC
    LIMIT 1
) a ON true
WHERE c.last_viewed_at IS NOT NULL
  AND (
    sqlc.arg(cursor_analyzed_at)::timestamptz IS NULL
    OR (a.completed_at, c.id) < (sqlc.arg(cursor_analyzed_at), sqlc.arg(cursor_id))
  )
ORDER BY
  CASE WHEN sqlc.arg(sort_order)::text = 'desc' THEN a.completed_at END DESC,
  CASE WHEN sqlc.arg(sort_order)::text = 'asc' THEN a.completed_at END ASC
LIMIT sqlc.arg(page_limit);
```

사용된 주요 기능:

- **LATERAL JOIN**: "그룹별 최신" 패턴을 위한 상관 서브쿼리
- **커서 페이지네이션**: 안정적인 정렬을 위한 복합 키 `(completed_at, id)`
- **동적 정렬**: 오름차순/내림차순을 위한 CASE 표현식
- **타입 안전 파라미터**: `sqlc.arg()`가 타입이 지정된 함수 인자 생성

### 생성 코드 품질

SQLc는 repository 패턴과 깔끔하게 통합되는 관용적 Go 코드 생성:

```go
// JSON 태그가 있는 생성된 구조체
type GetPaginatedRepositoriesRow struct {
    CodebaseID   pgtype.UUID        `json:"codebase_id"`
    Owner        string             `json:"owner"`
    Name         string             `json:"name"`
    AnalysisID   pgtype.UUID        `json:"analysis_id"`
    AnalyzedAt   pgtype.Timestamptz `json:"analyzed_at"`
    TotalTests   int32              `json:"total_tests"`
}

// 적절한 context와 오류 처리가 있는 생성된 함수
func (q *Queries) GetPaginatedRepositories(ctx context.Context, arg GetPaginatedRepositoriesParams) ([]GetPaginatedRepositoriesRow, error)
```

### 클린 아키텍처 통합

```
modules/{module}/
├── domain/port/
│   └── repository.go         # 인터페이스 정의
├── adapter/
│   └── repository_postgres.go  # db.Queries 사용
└── internal/db/
    └── *.sql.go              # SQLc 생성 파일
```

adapter는 SQLc가 생성한 `Queries`를 사용하여 port 인터페이스 구현:

```go
// adapter/repository_postgres.go
type PostgresRepository struct {
    queries *db.Queries
}

func (r *PostgresRepository) GetAnalysis(ctx context.Context, id string) (*entity.Analysis, error) {
    row, err := r.queries.GetLatestCompletedAnalysis(ctx, db.GetLatestCompletedAnalysisParams{...})
    if err != nil {
        return nil, err
    }
    return mapToEntity(row), nil
}
```

## Consequences

### 긍정적

**타입 안전성:**

- 쿼리 컬럼/타입 불일치가 컴파일 타임에 포착
- 리팩토링 신뢰도: IDE가 모든 사용처 추적 가능
- 런타임 SQL 파싱 오류 없음

**성능:**

- pgx/v5가 리플렉션 ORM을 사용하는 `database/sql`보다 30-50% 더 높은 처리량 제공
- 런타임에 쿼리 빌딩 오버헤드 없음
- pgxpool을 통한 커넥션 풀링

**개발자 경험:**

- 최적화된 SQL을 직접 작성
- 생성된 코드가 읽기 쉽고 디버그 가능
- 친숙한 도구: 모든 SQL 편집기, EXPLAIN ANALYZE

**아키텍처:**

- 깔끔한 분리: SQL은 `.sql` 파일에, Go는 adapter에
- 생성된 코드가 자연스럽게 port/adapter 패턴에 적합
- ORM 특정 추상화가 도메인 레이어에 누출되지 않음
- 마이그레이션 결합 없음: SQLc는 읽기 전용, 마이그레이션은 `infra` 리포에서 관리

**AI 기반 개발:**

- AI 에이전트가 추상화 마찰 없이 최적화된 원시 SQL 작성
- 메서드명이 자체 문서화: `GetPaginatedRepositoriesByRecent`가 동작을 명확히 표현
- 개발자는 읽기 쉬운 메서드 시그니처로 AI 생성 쿼리 검토
- ORM "마법" 없음: 작성한 대로 실행됨

### 부정적

**동적 쿼리 제한:**

- 진정한 동적 쿼리 (가변 WHERE 절)는 여러 쿼리 파일 필요
- **완화**: boolean 플래그 패턴이 대부분의 케이스 처리; 극단적 케이스에는 Squirrel 쿼리 빌더 사용

**SQL 지식 필요:**

- 팀이 SQL 작성 및 최적화에 익숙해야 함
- **완화**: 팀이 이미 능숙함; SQL은 이전 가능한 기술

**재생성 워크플로우:**

- 스키마 변경 시 `sqlc generate` 실행 필요
- **완화**: `just gen-sqlc` 명령에 통합; CI에서 생성된 코드가 최신인지 검증

## References

- [ADR-01: Go 백엔드 언어](/ko/adr/web/01-go-backend-language.md)
- [ADR-04: 큐 기반 비동기 처리](/ko/adr/04-queue-based-async-processing.md)
- [ADR-07: 공유 인프라 전략](/ko/adr/07-shared-infrastructure.md)
- [SQLc 문서](https://docs.sqlc.dev/)
