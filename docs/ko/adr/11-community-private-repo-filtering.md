---
title: Repository Visibility 기반 접근 제어
description: git ls-remote 결과로 visibility 판단하여 private repository 데이터를 비소유자로부터 격리하는 ADR
---

# ADR-11: Repository Visibility 기반 접근 제어

> 🇺🇸 [English Version](/en/adr/11-community-private-repo-filtering)

| 날짜       | 작성자       | 리포지토리         |
| ---------- | ------------ | ------------------ |
| 2026-01-03 | @KubrickCode | infra, worker, web |

## 배경

### 보안 이슈 발견

분석 데이터를 타 사용자에게 공개하는 기능 구현 중 보안 문제 발견:

**문제**: 사용자가 분석한 private repository 데이터가 비소유자에게 노출 가능

- Private repository 이름/소유자 노출
- 테스트 통계(개수, 성공/실패율) 노출

**핵심**: Repository 소유자/분석 요청자가 아닌 사용자는 private repository 데이터에 접근 불가해야 함

### `is_private` 미저장 이유

`codebases` 테이블에 `is_private`을 의도적으로 넣지 않은 이유:

- Repository visibility의 언제든 변경 가능성 (public ↔ private)
- 저장된 값의 stale 가능성
- GitHub의 visibility 변경 webhook 미제공 (App 미설치 시)

### 위험도 평가

| 시나리오                     | 빈도 | 위험도                                 |
| ---------------------------- | ---- | -------------------------------------- |
| 처음부터 private인 repo 분석 | 흔함 | **높음** - 커뮤니티에 절대 비노출 필수 |
| public → 분석 → private 전환 | 드묾 | **중간** - 엣지 케이스                 |
| private → 분석 → public 전환 | 드묾 | **낮음** - 노출 무방                   |

## 결정

**git ls-remote 결과로 visibility 판단 + `is_private` 저장**

### 핵심 아이디어

worker에서 분석 시 `git ls-remote`로 최신 커밋 조회. 이 로직 활용:

1. **토큰 없이 먼저 시도** → 성공 시 **public**
2. **실패 시 사용자 토큰으로 시도** → 성공 시 **private**

별도 GitHub API 호출 없이 자연스러운 visibility 판단 가능.

### 핵심 원칙

1. **토큰 없이 먼저**: 항상 public access 먼저 시도
2. **필요 시에만 토큰 사용**: 실패 시에만 사용자 토큰 사용
3. **분석 시점 캡처**: 결과를 `is_private`으로 저장
4. **쿼리에서 필터링**: Community 뷰에서 `is_private = true` 제외

## 고려된 대안

### 옵션 A: "커뮤니티에 공유" 체크박스 (기각)

사용자가 분석 시 명시적으로 공개 동의.

**장점**: 완벽한 프라이버시, 사용자 동의 기반

**단점**:

- UX 마찰로 콘텐츠 감소 (opt-in 비율 5-15% 수준)
- 새 플랫폼의 커뮤니티 성장 저해

**결정**: 초기 구현에서 제외. 추후 opt-out으로 추가 가능.

### 옵션 B: 실시간 GitHub API 확인 (기각)

매 요청마다 GitHub API로 현재 visibility 확인.

**장점**: 항상 정확한 정보

**단점**:

- Rate limit (시간당 5000회)
- 페이지 로딩 지연
- 복잡성 증가

**결정**: 기각. 스케일에서 비현실적.

### 옵션 C: git ls-remote 기반 탐지 (선택)

기존 git ls-remote 호출 활용하여 visibility 판단.

**장점**:

- 추가 API 호출 불필요
- 분석 시점 기준 정확한 정보
- 단순한 구현

**단점**:

- 분석 후 visibility 변경 시 stale 가능성

## 구현

### 데이터베이스 스키마 (infra)

```sql
-- codebases 테이블에 is_private 추가
ALTER TABLE codebases
ADD COLUMN is_private BOOLEAN NOT NULL DEFAULT false;

-- 효율적인 필터링을 위한 부분 인덱스
CREATE INDEX idx_codebases_is_private
ON codebases(is_private)
WHERE is_private = false;
```

### git ls-remote 로직 변경 (worker)

**파일**: `src/internal/adapter/vcs/git.go`

현재 로직:

```go
// 토큰 있으면 토큰으로 먼저 시도
if token != nil {
    sha, err := GetHeadCommit(ctx, url, token)
    if err == nil { return sha, nil }
}
// 실패하면 토큰 없이 시도
return GetHeadCommit(ctx, url, nil)
```

변경 후:

```go
// 1. 토큰 없이 먼저 시도 (public check)
sha, err := GetHeadCommit(ctx, url, nil)
if err == nil {
    return &CommitInfo{SHA: sha, IsPrivate: false}, nil
}

// 2. 실패 시 토큰으로 시도 (private repo)
if token != nil {
    sha, err = GetHeadCommit(ctx, url, token)
    if err == nil {
        return &CommitInfo{SHA: sha, IsPrivate: true}, nil
    }
}
```

### Community 쿼리 필터링 (web 백엔드)

**파일**: `queries/analysis.sql`

```sql
-- Community view WHERE 절에 추가
AND (
    sqlc.arg(view_filter)::text = 'community'
    AND c.is_private = false  -- public repo만
    AND NOT EXISTS(...)
)
```

### 사용자 공지 (web 프론트엔드)

```tsx
// explore-content.tsx
<p className="text-sm text-muted-foreground">{t("community.visibilityDisclosure")}</p>
```

```json
// messages/ko.json
{
  "explore": {
    "community": {
      "visibilityDisclosure": "분석 시점 기준 공개 저장소만 표시됩니다."
    }
  }
}
```

### 데이터 흐름

```
┌─────────────────────────────────────────────────────────────────────┐
│                        분석 흐름                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  사용자 분석 요청                                                     │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────┐                                                    │
│  │     web     │  Queue에 작업 등록                                  │
│  └──────┬──────┘                                                    │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                      worker                               │    │
│  │                                                              │    │
│  │  git ls-remote (토큰 없이)                                    │    │
│  │  ├─ 성공 → isPrivate = false (public)                        │    │
│  │  └─ 실패 → git ls-remote (토큰으로)                           │    │
│  │            └─ 성공 → isPrivate = true (private)              │    │
│  │                                                              │    │
│  │  git clone → 분석 → 저장 (is_private 포함)                    │    │
│  └─────────────────────────────────────────────────────────────┘    │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                      PostgreSQL                              │    │
│  │  codebases: { id, owner, name, is_private, ... }            │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                       커뮤니티 뷰 흐름                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  비로그인 사용자 /explore 방문                                        │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────┐                                                    │
│  │   web API   │                                                    │
│  └──────┬──────┘                                                    │
│         │  SELECT ... WHERE is_private = false                      │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                      PostgreSQL                              │    │
│  │  public repository만 반환                                     │    │
│  └─────────────────────────────────────────────────────────────┘    │
│         │                                                            │
│         ▼                                                            │
│  사용자는 커뮤니티 탭에서 public repo만 확인                          │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## 결과

### 긍정적

**보안**:

- Private repository 메타데이터가 더 이상 public에 노출되지 않음
- 대부분의 케이스 커버 (처음부터 private인 repo)
- 추가 API 오버헤드 없음

**단순성**:

- 기존 git ls-remote 호출 활용
- 단일 boolean 컬럼 추가
- 쿼리 수준 필터링

### 부정적

**실시간 동기화 불가**:

- 재분석 전까지 visibility 변경 미반영
- 단, 분석 시점 기준 public이었다면 해당 데이터는 그 시점에 공개 정보였으므로 노출에 문제 없음

## 보안 평가

| 기준    | 점수 | 비고               |
| ------- | ---- | ------------------ |
| 구현 후 | 8/10 | 대부분 케이스 커버 |

### 설계 원칙

- **분석 시점 기준 판단**: visibility는 분석 시점에 확정
- **과거 공개 데이터의 정당성**: 분석 시점에 public이었다면 해당 데이터는 당시 공개 정보였으므로 계속 노출해도 무방
- **private → public 전환**: 재분석 전까지 비공개 유지 (보수적 접근)

## 참조

- [GitHub REST API - Repositories](https://docs.github.com/en/rest/repos/repos)
- [git ls-remote 문서](https://git-scm.com/docs/git-ls-remote)
