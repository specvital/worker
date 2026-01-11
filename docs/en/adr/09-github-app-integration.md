---
title: GitHub App Integration
description: ADR on GitHub App authentication strategy for repository access
---

# ADR-09: GitHub App Integration Strategy

> :kr: [한국어 버전](/ko/adr/09-github-app-integration.md)

| Date       | Author       | Repos       |
| ---------- | ------------ | ----------- |
| 2024-12-29 | @KubrickCode | web, worker |

## Context

### Problem

Specvital needs authenticated GitHub API access for:

1. **Organization Repository Access**: List and analyze organization repositories
2. **Private Repository Analysis**: Access private repositories with appropriate permissions
3. **Higher Rate Limits**: Authenticated requests get 5000/hr vs 60/hr unauthenticated
4. **Repository Metadata**: Fetch repository information (external_repo_id, default branch)

### Authentication Options

| Feature               | OAuth App                  | GitHub App                      |
| --------------------- | -------------------------- | ------------------------------- |
| Permission Scope      | User-level (broad)         | Installation-level (granular)   |
| Organization Access   | Requires user membership   | Direct installation on org      |
| Token Lifetime        | Long-lived                 | Short-lived (1hr, auto-refresh) |
| Rate Limit            | 5000/hr per user           | 5000/hr per installation        |
| Private Repo Access   | All user's repos           | Only selected repos             |
| Webhook Support       | Limited                    | Installation lifecycle events   |
| Background Processing | Requires stored user token | Installation token on-demand    |

### Goals

1. **Secure Access**: Minimal necessary permissions, short-lived tokens
2. **Organization Support**: Enable org repository analysis without user context
3. **Scalability**: Independent rate limits per installation
4. **Maintainability**: Clear separation between web (token issuer) and worker (token consumer)

## Decision

**Adopt GitHub App with Installation Token pattern for repository access.**

### Architecture

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

### Token Flow

**Web Service (Token Issuer):**

```go
// GitHubAppClient creates installation tokens using App credentials
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

**Worker Service (Token Consumer):**

```go
// GitHubAPIClient uses token as optional Bearer auth
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

## Options Considered

### Option A: OAuth App Only (Rejected)

**Description:** Use OAuth tokens stored from user authentication.

**Pros:**

- Simpler implementation
- Already have OAuth flow for user login

**Cons:**

- Cannot access org repos without user re-auth
- Long-lived tokens require secure storage
- Background jobs need stored user tokens
- Single rate limit pool per user

### Option B: GitHub App Only (Selected)

**Description:** Use GitHub App installation tokens for all repository access.

**Pros:**

- Granular permissions (only repo contents, metadata)
- Organization-level installation
- Short-lived tokens (1 hour, auto-refresh)
- Independent rate limits per installation
- Webhook-based lifecycle management

**Cons:**

- Requires app registration and configuration
- Users must install app on their account/org
- Token generation requires private key management

### Option C: Hybrid OAuth + GitHub App (Rejected)

**Description:** OAuth for user context, GitHub App for background processing.

**Pros:**

- Best of both worlds
- Fallback options

**Cons:**

- Increased complexity
- Two authentication flows to maintain
- Confusing user experience

## Implementation

### Web Service Components

```
src/backend/
├── internal/client/
│   └── github_app.go          # GitHubAppClient implementation
├── modules/github-app/
│   ├── domain/
│   │   ├── entity/            # Installation entity
│   │   ├── errors.go          # ErrInstallationSuspended, etc.
│   │   └── port/
│   │       ├── github_app_client.go   # CreateInstallationToken interface
│   │       └── installation_repo.go   # Installation repository port
│   ├── handler/
│   │   ├── http.go            # Webhook handler
│   │   └── http_api.go        # REST API handler
│   └── usecase/
│       ├── get_installation_token.go  # Token generation usecase
│       ├── handle_webhook.go          # Webhook event processing
│       └── list_installations.go      # User's installations
└── modules/github/adapter/
    └── installation_adapter.go  # Cross-module token provider
```

### Webhook Events

| Event                       | Action      | Handler Response                |
| --------------------------- | ----------- | ------------------------------- |
| `installation`              | `created`   | Store installation record       |
| `installation`              | `deleted`   | Remove installation record      |
| `installation`              | `suspended` | Mark installation as suspended  |
| `installation`              | `unsuspend` | Clear suspended flag            |
| `installation_repositories` | `added`     | (Future) Track accessible repos |
| `installation_repositories` | `removed`   | (Future) Remove repo access     |

### Webhook Security

```go
func (h *Handler) HandleGitHubAppWebhookRaw(w http.ResponseWriter, r *http.Request) {
    signature := r.Header.Get("X-Hub-Signature-256")
    body, _ := io.ReadAll(r.Body)

    // HMAC-SHA256 verification
    if err := h.verifier.Verify(signature, body); err != nil {
        h.respondError(w, http.StatusUnauthorized, "invalid webhook signature")
        return
    }
    // Process webhook...
}
```

### Queue Message (Current)

```go
// River job args - token NOT included (public repos only)
type AnalyzeArgs struct {
    CommitSHA string  `json:"commit_sha"`
    Owner     string  `json:"owner"`
    Repo      string  `json:"repo"`
    UserID    *string `json:"user_id,omitempty"`
}
```

### Future: Private Repository Support

```go
// Option 1: Include token in queue message
type AnalyzeArgs struct {
    CommitSHA       string  `json:"commit_sha"`
    Owner           string  `json:"owner"`
    Repo            string  `json:"repo"`
    InstallationID  *int64  `json:"installation_id,omitempty"`  // For token fetch
}

// Option 2: Worker fetches token via internal API
func (w *AnalyzeWorker) getToken(ctx context.Context, installationID int64) (string, error) {
    return w.tokenClient.GetInstallationToken(ctx, installationID)
}
```

## Database Schema

### Web Service (installations table)

```sql
CREATE TABLE github_app_installations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    installation_id   BIGINT NOT NULL UNIQUE,
    account_id        BIGINT NOT NULL,
    account_login     VARCHAR(255) NOT NULL,
    account_type      VARCHAR(50) NOT NULL,  -- 'User' or 'Organization'
    account_avatar_url TEXT,
    suspended_at      TIMESTAMP WITH TIME ZONE,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_installations_account ON github_app_installations(account_id);
```

## Consequences

### Positive

**Security:**

- Short-lived tokens (1 hour expiry)
- No long-term token storage required
- Webhook signature verification (HMAC-SHA256)
- Granular permission scope

**Organization Support:**

- Direct installation on organizations
- No user membership required for org repos
- Suspended installation detection

**Scalability:**

- Independent rate limits per installation
- On-demand token generation
- No token refresh complexity (always fresh)

**Developer Experience:**

- Clear separation of concerns (web: issuer, worker: consumer)
- Webhook-based state synchronization
- Testable components (mockable interfaces)

### Negative

**User Experience:**

- Additional installation step for users
- Must install app on each org separately

**Operational Complexity:**

- Private key management required
- Webhook endpoint must be publicly accessible
- Installation state must be synchronized

**Current Limitation:**

- Private repository analysis not yet implemented
- Token not passed through queue (public repos only)

## Migration Path

### Phase 1: Current State (Implemented)

- GitHub App registered and configured
- Webhook handler processing installation events
- Installation tokens used for org repository listing
- Public repository analysis (no token needed)

### Phase 2: Private Repository Support (Future)

- Add `installation_id` to queue message
- Worker fetches token on-demand
- Implement token caching with TTL

### Phase 3: Enhanced Features (Future)

- Repository-level permission tracking
- Automatic re-analysis on push events
- Installation health monitoring

## References

- [ADR-03: API-Worker Service Separation](./03-api-worker-service-separation.md) - Service architecture
- [ADR-04: Queue-Based Async Processing](./04-queue-based-async-processing.md) - River queue integration
- [ADR-08: External Repo ID Integrity](./08-external-repo-id-integrity.md) - Repository identification
- [GitHub App Documentation](https://docs.github.com/en/apps/creating-github-apps)
- [ghinstallation Library](https://github.com/bradleyfalzon/ghinstallation)
