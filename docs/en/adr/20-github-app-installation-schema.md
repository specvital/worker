---
title: GitHub App Installation Schema
description: ADR for database schema design for GitHub App installation lifecycle management
---

# ADR-20: GitHub App Installation Schema

> 🇰🇷 [한국어 버전](/ko/adr/20-github-app-installation-schema.md)

| Date       | Author     | Repos      |
| ---------- | ---------- | ---------- |
| 2026-01-19 | @specvital | infra, web |

## Context

### Problem

OAuth App authentication requires organization admin approval, preventing regular users from accessing organization repositories. Additionally:

| Issue                                   | Impact                                                                  |
| --------------------------------------- | ----------------------------------------------------------------------- |
| Organization admin approval requirement | Regular users cannot access org repositories without admin intervention |
| Long-lived OAuth token storage          | Security risk from persistent credentials in database                   |
| Shared rate limits                      | Single 5000/hr pool per user exhausted quickly with multiple repos      |
| Background processing dependency        | Workers require stored user tokens for private repo access              |

### Requirements

1. Organization repository access without user membership requirement
2. Independent rate limits per installation (5000/hr each)
3. Secure token handling - minimize credential storage
4. Background worker support without long-lived stored tokens
5. Webhook-based installation lifecycle tracking

### Relation to ADR-09

This schema implements the "Installation Store" component from [ADR-09: GitHub App Integration Strategy](/en/adr/09-github-app-integration.md). ADR-09 established the integration strategy; this ADR defines the specific schema design that realizes that strategy.

## Decision

**Store installation metadata only (`installation_id`, `account_type`, `account_id`) without access tokens. Generate short-lived installation tokens on-demand via JWT authentication with the GitHub App's private key.**

### Schema Design

```sql
CREATE TYPE "public"."github_account_type" AS ENUM ('organization', 'user');

CREATE TABLE "public"."github_app_installations" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "installation_id" bigint NOT NULL,
  "account_type" "public"."github_account_type" NOT NULL,
  "account_id" bigint NOT NULL,
  "account_login" character varying(255) NOT NULL,
  "account_avatar_url" text NULL,
  "installer_user_id" uuid NULL,
  "suspended_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "uq_github_app_installations_account" UNIQUE ("account_type", "account_id"),
  CONSTRAINT "uq_github_app_installations_installation_id" UNIQUE ("installation_id"),
  CONSTRAINT "fk_github_app_installations_installer" FOREIGN KEY ("installer_user_id")
    REFERENCES "public"."users" ("id") ON DELETE SET NULL
);

CREATE INDEX "idx_github_app_installations_installer"
  ON "public"."github_app_installations" ("installer_user_id")
  WHERE (installer_user_id IS NOT NULL);
```

### Column Specification

| Column               | Type               | Purpose                              |
| -------------------- | ------------------ | ------------------------------------ |
| `id`                 | UUID               | Internal primary key                 |
| `installation_id`    | BIGINT UNIQUE      | GitHub's installation identifier     |
| `account_type`       | ENUM               | 'organization' or 'user'             |
| `account_id`         | BIGINT             | GitHub account ID                    |
| `account_login`      | VARCHAR(255)       | GitHub username/org name (display)   |
| `account_avatar_url` | TEXT               | Avatar URL (nullable)                |
| `installer_user_id`  | UUID FK (nullable) | User who initiated installation      |
| `suspended_at`       | TIMESTAMPTZ        | Suspension timestamp (null = active) |

### Key Design Decisions

1. **No token storage**: Tokens generated on-demand via JWT, never persisted
2. **PostgreSQL enum for account_type**: Type safety over VARCHAR
3. **Dual unique constraints**: Both `installation_id` and `(account_type, account_id)`
4. **Partial index on installer**: Optimize "my installations" queries without indexing nulls
5. **Soft suspension**: `suspended_at` timestamp vs boolean for audit trail

### Token Generation Flow

```
installation_id → JWT signed with private key → GitHub API → 1-hour access token
```

## Options Considered

### Option A: Store installation_id Only (Selected)

Generate access tokens on-demand using JWT + private key.

| Pros                                                         | Cons                                             |
| ------------------------------------------------------------ | ------------------------------------------------ |
| Zero credential storage - eliminates token exfiltration risk | Token generation latency (~50-200ms per request) |
| Always-fresh tokens (no staleness/refresh logic)             | Private key management complexity                |
| Aligns with GitHub security best practices                   | Requires in-memory caching for performance       |

### Option B: Store Full access_token

Store generated tokens with expiry, refresh before use.

| Pros                                    | Cons                                               |
| --------------------------------------- | -------------------------------------------------- |
| No generation latency for cached tokens | Stored credentials vulnerable to database breach   |
| Simpler code path for token retrieval   | Token refresh logic and expiry monitoring required |
|                                         | Stale token handling edge cases                    |

**Rejected**: Security risk from stored credentials outweighs latency benefits. GitHub explicitly recommends against storing installation access tokens.

### Option C: Extend Users Table

Add installation columns directly to the users table.

| Pros                                   | Cons                                                    |
| -------------------------------------- | ------------------------------------------------------- |
| No new table, simpler schema           | Cannot support organization installations (no user row) |
| Natural user-installation relationship | One installation per user limitation                    |
|                                        | Conflates user identity with installation identity      |

**Rejected**: Organization installations have no corresponding user row; separate entity required.

## Consequences

### Positive

| Area                 | Benefit                                                                  |
| -------------------- | ------------------------------------------------------------------------ |
| Security             | No long-lived credentials stored; short-lived tokens limit breach impact |
| Scalability          | Independent 5000/hr rate limit per installation                          |
| Organization support | First-class support without user membership requirement                  |
| Webhook alignment    | Schema maps directly to GitHub lifecycle events (install/delete/suspend) |

### Negative

| Area        | Trade-off                              | Mitigation                                 |
| ----------- | -------------------------------------- | ------------------------------------------ |
| Latency     | Token generation adds ~50-200ms        | In-memory cache with ~55-minute TTL        |
| Operational | Private key becomes critical secret    | Key vault storage, rotation procedures     |
| UX          | Additional installation step post-auth | Clear onboarding flow, installation prompt |

### Technical Implications

- **Database Design**: PostgreSQL enum for type safety; partial index optimizes common query pattern
- **Cross-service Coordination**: Web service tracks installations, Worker requests tokens via installation_id
- **Webhook Processing**: Events (`installation.created`, `installation.deleted`, `installation.suspend`) map to table operations

## Implementation Files

### specvital/infra

| File                                                               | Purpose                  |
| ------------------------------------------------------------------ | ------------------------ |
| `db/schema/migrations/20251226154124_github_app_installations.sql` | Table creation migration |

### specvital/web

| File                                                   | Purpose                                   |
| ------------------------------------------------------ | ----------------------------------------- |
| `internal/client/github_app.go`                        | GitHubAppClient with JWT token generation |
| `modules/github-app/domain/entity/installation.go`     | Installation domain entity                |
| `modules/github-app/domain/port/repository.go`         | InstallationRepository port               |
| `modules/github-app/adapter/repository_postgres.go`    | PostgreSQL repository adapter             |
| `modules/github-app/handler/http.go`                   | Webhook handler                           |
| `modules/github-app/usecase/get_installation_token.go` | Token generation use case                 |

## References

- [cd33ecb](https://github.com/specvital/infra/commit/cd33ecb): feat(db): add GitHub App Installation table
- [0db7539](https://github.com/specvital/infra/commit/0db7539): feat(db): add refresh token table for hybrid authentication
- [ADR-09: GitHub App Integration Strategy](/en/adr/09-github-app-integration.md) - Parent strategy ADR
- [ADR-08: External Repository ID-Based Data Integrity](/en/adr/08-external-repo-id-integrity.md) - Data integrity patterns
- [GitHub: Best practices for creating a GitHub App](https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/best-practices-for-creating-a-github-app)
