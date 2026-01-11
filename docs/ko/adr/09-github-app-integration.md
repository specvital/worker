---
title: GitHub App 통합
description: 리포지토리 접근을 위한 GitHub App 인증 전략 ADR
---

# ADR-09: GitHub App 통합 인증 전략

> :us: [English Version](/en/adr/09-github-app-integration.md)

| 날짜       | 작성자       | 관련 리포지토리 |
| ---------- | ------------ | --------------- |
| 2024-12-29 | @KubrickCode | web, worker     |

## 맥락

### 문제

Specvital은 다음 목적으로 인증된 GitHub API 접근 필요:

1. **조직 리포지토리 접근**: 조직 리포지토리 목록 조회 및 분석
2. **Private 리포지토리 분석**: 적절한 권한으로 private 리포지토리 접근
3. **높은 Rate Limit**: 인증 요청 5000/hr vs 미인증 60/hr
4. **리포지토리 메타데이터**: 리포지토리 정보 조회 (external_repo_id, 기본 브랜치)

### 인증 옵션

| 기능              | OAuth App               | GitHub App               |
| ----------------- | ----------------------- | ------------------------ |
| 권한 범위         | 사용자 수준 (광범위)    | 설치 수준 (세분화)       |
| 조직 접근         | 사용자 멤버십 필요      | 조직에 직접 설치         |
| 토큰 수명         | 장기                    | 단기 (1시간, 자동 갱신)  |
| Rate Limit        | 사용자당 5000/hr        | 설치당 5000/hr           |
| Private Repo 접근 | 사용자의 모든 리포      | 선택된 리포만            |
| Webhook 지원      | 제한적                  | 설치 라이프사이클 이벤트 |
| 백그라운드 처리   | 저장된 사용자 토큰 필요 | 온디맨드 설치 토큰       |

### 목표

1. **보안 접근**: 최소 필요 권한, 단기 토큰
2. **조직 지원**: 사용자 컨텍스트 없이 조직 리포지토리 분석 가능
3. **확장성**: 설치별 독립적 rate limit
4. **유지보수성**: web(토큰 발급자)과 worker(토큰 소비자) 간 명확한 분리

## 결정

**리포지토리 접근을 위해 GitHub App과 Installation Token 패턴 채택.**

### 아키텍처

```
┌─────────────────────────────────────────────────────────────┐
│                        GitHub                                │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────┐  │
│  │  GitHub App  │    │ Installation │    │  Repositories │  │
│  │  (Specvital) │───▶│    Token     │───▶│    Access     │  │
│  └──────────────┘    └──────────────┘    └───────────────┘  │
│         │                                        ▲           │
│         │ Webhook                                │           │
│         ▼                                        │           │
└─────────┼────────────────────────────────────────┼──────────┘
          │                                        │
┌─────────┼────────────────────────────────────────┼──────────┐
│         │                 Web Service            │           │
│         ▼                                        │           │
│  ┌──────────────┐    ┌──────────────┐    ┌──────┴───────┐   │
│  │   Webhook    │    │ Installation │    │   GitHub     │   │
│  │   Handler    │───▶│    Store     │◀───│  API Client  │   │
│  └──────────────┘    └──────────────┘    └──────────────┘   │
│                             │                                │
│                             │ Token Provider                 │
│                             ▼                                │
│                      ┌──────────────┐                        │
│                      │    River     │                        │
│                      │    Queue     │                        │
│                      └──────────────┘                        │
│                             │                                │
└─────────────────────────────┼────────────────────────────────┘
                              │
┌─────────────────────────────┼────────────────────────────────┐
│                             │        Worker Service       │
│                             ▼                                │
│                      ┌──────────────┐    ┌──────────────┐   │
│                      │   Analyze    │───▶│   GitHub     │   │
│                      │   Worker     │    │   API        │   │
│                      └──────────────┘    └──────────────┘   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 토큰 흐름

**Web Service (토큰 발급자):**

```go
// GitHubAppClient: App 자격증명으로 설치 토큰 생성
type GitHubAppClient struct {
    appID        int64
    appTransport *ghinstallation.AppsTransport
}

func (c *GitHubAppClient) CreateInstallationToken(
    ctx context.Context,
    installationID int64,
) (*InstallationToken, error) {
    itr := ghinstallation.NewFromAppsTransport(c.appTransport, installationID)
    token, _ := itr.Token(ctx)
    expiresAt, _, _ := itr.Expiry()
    return &InstallationToken{Token: token, ExpiresAt: expiresAt}, nil
}
```

**Worker Service (토큰 소비자):**

```go
// GitHubAPIClient: 토큰을 선택적 Bearer 인증으로 사용
func (c *GitHubAPIClient) GetRepoInfo(
    ctx context.Context,
    host, owner, repo string,
    token *string,
) (RepoInfo, error) {
    req.Header.Set("Accept", "application/vnd.github+json")
    if token != nil && *token != "" {
        req.Header.Set("Authorization", "Bearer "+*token)
    }
    // ...
}
```

## 고려한 옵션

### 옵션 A: OAuth App만 사용 (기각)

**설명:** 사용자 인증에서 저장한 OAuth 토큰 사용.

**장점:**

- 구현 단순
- 사용자 로그인용 OAuth 플로우 이미 존재

**단점:**

- 사용자 재인증 없이 조직 리포 접근 불가
- 장기 토큰 보안 저장 필요
- 백그라운드 작업에 저장된 사용자 토큰 필요
- 사용자당 단일 rate limit 풀

### 옵션 B: GitHub App만 사용 (선택)

**설명:** 모든 리포지토리 접근에 GitHub App 설치 토큰 사용.

**장점:**

- 세분화된 권한 (리포 컨텐츠, 메타데이터만)
- 조직 수준 설치
- 단기 토큰 (1시간, 자동 갱신)
- 설치별 독립적 rate limit
- Webhook 기반 라이프사이클 관리

**단점:**

- 앱 등록 및 설정 필요
- 사용자가 계정/조직에 앱 설치 필요
- 토큰 생성에 private key 관리 필요

### 옵션 C: OAuth + GitHub App 하이브리드 (기각)

**설명:** 사용자 컨텍스트에 OAuth, 백그라운드 처리에 GitHub App.

**장점:**

- 양쪽 장점 활용
- 폴백 옵션

**단점:**

- 복잡도 증가
- 두 인증 플로우 유지 필요
- 혼란스러운 사용자 경험

## 구현

### Web Service 컴포넌트

```
src/backend/
├── internal/client/
│   └── github_app.go          # GitHubAppClient 구현
├── modules/github-app/
│   ├── domain/
│   │   ├── entity/            # Installation 엔티티
│   │   ├── errors.go          # ErrInstallationSuspended 등
│   │   └── port/
│   │       ├── github_app_client.go   # CreateInstallationToken 인터페이스
│   │       └── installation_repo.go   # Installation 리포지토리 포트
│   ├── handler/
│   │   ├── http.go            # Webhook 핸들러
│   │   └── http_api.go        # REST API 핸들러
│   └── usecase/
│       ├── get_installation_token.go  # 토큰 생성 유스케이스
│       ├── handle_webhook.go          # Webhook 이벤트 처리
│       └── list_installations.go      # 사용자의 설치 목록
└── modules/github/adapter/
    └── installation_adapter.go  # 크로스 모듈 토큰 제공자
```

### Webhook 이벤트

| 이벤트                      | 액션        | 핸들러 응답                |
| --------------------------- | ----------- | -------------------------- |
| `installation`              | `created`   | 설치 레코드 저장           |
| `installation`              | `deleted`   | 설치 레코드 제거           |
| `installation`              | `suspended` | 설치를 suspended로 표시    |
| `installation`              | `unsuspend` | suspended 플래그 해제      |
| `installation_repositories` | `added`     | (향후) 접근 가능 리포 추적 |
| `installation_repositories` | `removed`   | (향후) 리포 접근 제거      |

### Webhook 보안

```go
func (h *Handler) HandleGitHubAppWebhookRaw(w http.ResponseWriter, r *http.Request) {
    signature := r.Header.Get("X-Hub-Signature-256")
    body, _ := io.ReadAll(r.Body)

    // HMAC-SHA256 검증
    if err := h.verifier.Verify(signature, body); err != nil {
        h.respondError(w, http.StatusUnauthorized, "invalid webhook signature")
        return
    }
    // Webhook 처리...
}
```

### 큐 메시지 (현재)

```go
// River job args - 토큰 미포함 (public 리포만)
type AnalyzeArgs struct {
    CommitSHA string  `json:"commit_sha"`
    Owner     string  `json:"owner"`
    Repo      string  `json:"repo"`
    UserID    *string `json:"user_id,omitempty"`
}
```

### 향후: Private 리포지토리 지원

```go
// 옵션 1: 큐 메시지에 토큰 포함
type AnalyzeArgs struct {
    CommitSHA       string  `json:"commit_sha"`
    Owner           string  `json:"owner"`
    Repo            string  `json:"repo"`
    InstallationID  *int64  `json:"installation_id,omitempty"`  // 토큰 조회용
}

// 옵션 2: Worker가 내부 API로 토큰 조회
func (w *AnalyzeWorker) getToken(ctx context.Context, installationID int64) (string, error) {
    return w.tokenClient.GetInstallationToken(ctx, installationID)
}
```

## 데이터베이스 스키마

### Web Service (installations 테이블)

```sql
CREATE TABLE github_app_installations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    installation_id   BIGINT NOT NULL UNIQUE,
    account_id        BIGINT NOT NULL,
    account_login     VARCHAR(255) NOT NULL,
    account_type      VARCHAR(50) NOT NULL,  -- 'User' 또는 'Organization'
    account_avatar_url TEXT,
    suspended_at      TIMESTAMP WITH TIME ZONE,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_installations_account ON github_app_installations(account_id);
```

## 결과

### 긍정적

**보안:**

- 단기 토큰 (1시간 만료)
- 장기 토큰 저장 불필요
- Webhook 서명 검증 (HMAC-SHA256)
- 세분화된 권한 범위

**조직 지원:**

- 조직에 직접 설치
- 조직 리포에 사용자 멤버십 불필요
- Suspended 설치 감지

**확장성:**

- 설치별 독립적 rate limit
- 온디맨드 토큰 생성
- 토큰 갱신 복잡도 없음 (항상 신선)

**개발자 경험:**

- 관심사의 명확한 분리 (web: 발급자, worker: 소비자)
- Webhook 기반 상태 동기화
- 테스트 가능한 컴포넌트 (모킹 가능한 인터페이스)

### 부정적

**사용자 경험:**

- 사용자에게 추가 설치 단계
- 각 조직에 별도 설치 필요

**운영 복잡도:**

- Private key 관리 필요
- Webhook 엔드포인트 공개 접근 필요
- 설치 상태 동기화 필요

**현재 제한:**

- Private 리포지토리 분석 미구현
- 토큰이 큐를 통해 전달되지 않음 (public 리포만)

## 마이그레이션 경로

### 1단계: 현재 상태 (구현 완료)

- GitHub App 등록 및 설정
- Webhook 핸들러가 설치 이벤트 처리
- 조직 리포지토리 목록에 설치 토큰 사용
- Public 리포지토리 분석 (토큰 불필요)

### 2단계: Private 리포지토리 지원 (향후)

- 큐 메시지에 `installation_id` 추가
- Worker가 온디맨드로 토큰 조회
- TTL과 함께 토큰 캐싱 구현

### 3단계: 향상된 기능 (향후)

- 리포지토리 수준 권한 추적
- push 이벤트 시 자동 재분석
- 설치 상태 모니터링

## 참고 자료

- [ADR-03: API-Worker 서비스 분리](./03-api-worker-service-separation.md) - 서비스 아키텍처
- [ADR-04: 큐 기반 비동기 처리](./04-queue-based-async-processing.md) - River 큐 통합
- [ADR-08: External Repo ID 무결성](./08-external-repo-id-integrity.md) - 리포지토리 식별
- [GitHub App 문서](https://docs.github.com/en/apps/creating-github-apps)
- [ghinstallation 라이브러리](https://github.com/bradleyfalzon/ghinstallation)
