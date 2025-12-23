---
title: OAuth Token Graceful Degradation
description: ADR on graceful degradation for OAuth tokens with public access fallback
---

# ADR-04: OAuth Token Graceful Degradation

> 🇰🇷 [한국어 버전](/ko/adr/collector/04-oauth-token-graceful-degradation.md)

| Date       | Author       | Repos     |
| ---------- | ------------ | --------- |
| 2024-12-18 | @KubrickCode | collector |

## Context

### The Private Repository Challenge

Repository analysis services need to support both public and private repositories:

**Public Repositories:**

- Accessible without authentication
- Available to all users

**Private Repositories:**

- Require user authentication (OAuth tokens)
- Only accessible to authorized users

### Design Tension

The naive approach of requiring authentication for all analysis creates friction:

| Approach            | Public Repos            | Private Repos         | User Experience       |
| ------------------- | ----------------------- | --------------------- | --------------------- |
| Always require auth | Works (but unnecessary) | Works                 | High friction         |
| Never require auth  | Works                   | Fails                 | Limited functionality |
| Auth when available | Works                   | Works when authorized | Optimal               |

**The goal: maximize accessibility while supporting private repository access when credentials are available.**

### Error Handling Complexity

Token-related failures have fundamentally different causes:

**Expected Conditions:**

- User hasn't connected their account (no token stored)
- Token intentionally not provided for public-only access

**Infrastructure Failures:**

- Token storage service unavailable
- Decryption failure
- Network timeout during token retrieval

These require different handling strategies—treating all token errors identically either blocks users unnecessarily or masks real problems.

## Decision

**Adopt graceful degradation for OAuth token handling with explicit error categorization.**

### Core Principles

**1. Domain Exception Separation**

Distinguish between expected conditions and infrastructure failures:

| Exception Type      | Meaning                             | Handling           |
| ------------------- | ----------------------------------- | ------------------ |
| Token Not Found     | User has no stored token (expected) | Fallback to public |
| Token Lookup Failed | Infrastructure error (unexpected)   | Fail the operation |

**Why This Matters:**

- Expected conditions don't indicate system problems
- Infrastructure failures need alerting and investigation
- Log levels differ (info vs error)
- Retry strategies differ (no retry vs retry with backoff)

**2. Public Access Fallback**

When no token is available (expected condition):

- Proceed with unauthenticated access
- Log at info level for observability
- Public repositories work normally
- Private repositories fail at clone time (expected)

**3. Fail-Fast on Infrastructure Errors**

When token lookup fails unexpectedly:

- Fail the operation immediately
- Log at error level
- Enable monitoring/alerting
- Don't proceed with potentially corrupted state

**4. Optional Token Lookup**

Support deployments without token storage:

- If token lookup is not configured, use public access only
- Enables simpler deployment for public-only use cases
- No runtime errors for unconfigured components

## Options Considered

### Option A: Graceful Degradation with Exception Separation (Selected)

**Description:**

Separate token-related errors into expected vs unexpected categories. Fall back to public access for expected conditions, fail for infrastructure errors.

**Pros:**

- Optimal user experience for public repositories
- Clear error semantics enable proper monitoring
- Supports both authenticated and unauthenticated use cases
- Infrastructure problems surface immediately

**Cons:**

- More complex error handling logic
- Requires careful exception design
- Testing must cover multiple paths

### Option B: Mandatory Authentication

**Description:**

Require valid authentication for all analysis requests.

```
No token → Reject request
Invalid token → Reject request
```

**Pros:**

- Simple logic (token required, period)
- Higher rate limits for all requests
- Consistent behavior

**Cons:**

- Blocks users who only want public repository analysis
- Creates unnecessary friction for common use case
- Forces account connection before first use

### Option C: Treat All Token Errors as Non-Fatal

**Description:**

Fall back to public access on any token-related error.

```
No token → Public access
Lookup failed → Public access
Decryption failed → Public access
```

**Pros:**

- Maximum availability
- Simple fallback logic

**Cons:**

- Masks infrastructure problems
- Decryption failures could indicate security issues
- No differentiation in alerting/monitoring
- May proceed with corrupted state

## Implementation Principles

### Exception Hierarchy

Design domain exceptions that communicate intent:

| Exception         | Semantic                             | Action              |
| ----------------- | ------------------------------------ | ------------------- |
| TokenNotFound     | No token exists (business condition) | Info log + fallback |
| TokenLookupFailed | Storage/infra error (system problem) | Error log + fail    |

### Token Processing Flow

```
Request arrives
  └── Token lookup configured?
        ├── No  → Use public access
        └── Yes → Attempt token retrieval
                    ├── Success → Use authenticated access
                    ├── Not found → Use public access (info)
                    └── Lookup failed → Fail operation (error)
```

### Encryption Considerations

When tokens are stored encrypted:

- Decryption failures are infrastructure errors (not "token not found")
- Key mismatch indicates configuration problem
- Should fail fast, not silently degrade

### Logging Strategy

| Scenario        | Level | Content                                  |
| --------------- | ----- | ---------------------------------------- |
| Token not found | INFO  | User ID, proceeding with public access   |
| Token found     | DEBUG | User ID, using authenticated access      |
| Lookup failed   | ERROR | User ID, error details, operation failed |

## Consequences

### Positive

**User Experience:**

- No forced authentication for public repositories
- Private repository support when credentials available
- Graceful handling of missing credentials

**Operational Clarity:**

- Clear distinction between expected and error conditions
- Appropriate alerting (no false alarms for expected conditions)
- Infrastructure problems surface immediately

**Flexibility:**

- Supports public-only deployments
- Supports authenticated deployments
- Gradual rollout possible

**Security:**

- Decryption failures don't silently degrade
- Token storage issues are flagged
- No authentication bypass on errors

### Negative

**Complexity:**

- Two exception types instead of one
- Multiple code paths to test
- Documentation needed for behavior

**Potential Confusion:**

- "Token not found" vs "lookup failed" distinction requires explanation
- Developers must choose correct exception type
- Log analysis requires understanding the difference

**Silent Degradation Risk:**

- Public fallback means some private repo failures are "expected"
- Users may not realize they're hitting public limits
- Need clear feedback when degradation occurs

## References

- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md) (Domain exception design)
- [ADR-01: Scheduled Re-collection](./01-scheduled-recollection.md) (Token handling in scheduler context)
