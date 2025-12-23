---
title: NaCl SecretBox Encryption
description: ADR on choosing NaCl SecretBox for OAuth token encryption shared across services
---

# ADR-13: NaCl SecretBox Encryption

> 🇰🇷 [한국어 버전](/ko/adr/core/13-nacl-secretbox-encryption.md)

| Date       | Author       | Repos                |
| ---------- | ------------ | -------------------- |
| 2025-12-23 | @KubrickCode | core, web, collector |

## Context

### Problem Statement

OAuth tokens stored in the database require encryption at rest. Multiple services need to encrypt/decrypt these tokens:

1. **Web Service**: Encrypts tokens when storing user OAuth credentials
2. **Collector Service**: Decrypts tokens when accessing GitHub API

### Requirements

| Requirement              | Description                                      |
| ------------------------ | ------------------------------------------------ |
| Authenticated encryption | Prevent tampering and ensure data integrity      |
| Symmetric key            | Same key encrypts and decrypts                   |
| Thread-safe              | Concurrent encryption in multi-goroutine context |
| Cross-service sharing    | Same codebase usable by web and collector        |
| Key rotation support     | Ability to rotate keys without data loss         |

## Decision

**Use NaCl SecretBox (XSalsa20 + Poly1305) for symmetric authenticated encryption.**

The `pkg/crypto` package provides:

- `Encryptor` interface for encrypt/decrypt operations
- Base64-encoded output format: `Base64(nonce || ciphertext)`
- Sentinel errors for type-safe error handling
- Thread-safe implementation

## Options Considered

### Option A: NaCl SecretBox (Selected)

XSalsa20 stream cipher with Poly1305 MAC.

**Pros:**

- **Misuse-resistant**: 192-bit nonce virtually eliminates collision risk
- **Authenticated**: Poly1305 MAC detects tampering
- **Simple API**: Single function, hard to misuse
- **Battle-tested**: libsodium implementation, widely audited
- **No IV management**: Random nonce per encryption

**Cons:**

- Not FIPS-140 compliant (if compliance required)
- Less common than AES in enterprise environments

### Option B: AES-GCM

AES with Galois/Counter Mode.

**Pros:**

- FIPS-140 compliant
- Hardware acceleration (AES-NI)
- Industry standard

**Cons:**

- **96-bit nonce**: Higher collision risk, requires nonce tracking
- **Nonce reuse catastrophic**: Reusing nonce leaks authentication key
- More complex API, easier to misuse

### Option C: AES-CBC + HMAC

AES-CBC for encryption, separate HMAC for authentication.

**Pros:**

- FIPS-140 compliant
- Well understood

**Cons:**

- **Encrypt-then-MAC ordering critical**: Wrong order is insecure
- **IV management required**: Must ensure uniqueness
- **Padding oracle attacks**: Requires careful implementation
- More code, more room for error

## Consequences

### Positive

1. **Simplicity**
   - Single `Seal`/`Open` function pair
   - Random nonce generated automatically
   - No mode selection or parameter tuning

2. **Security Margin**
   - 256-bit key, 192-bit nonce
   - XSalsa20 extends nonce space vs original Salsa20
   - No known practical attacks

3. **Portability**
   - Pure Go implementation in `golang.org/x/crypto`
   - No CGO dependency
   - Cross-platform builds

### Negative

1. **Compliance Limitation**
   - Not FIPS-140 certified
   - **Mitigation**: Acceptable for current use case (OAuth tokens)

2. **Algorithm Lock-in**
   - Switching algorithms requires re-encrypting all data
   - **Mitigation**: Interface abstraction allows implementation swap

### Implementation Details

**Output Format:**

```
Base64(nonce || ciphertext)
       24 bytes   plaintext + 16 bytes overhead
```

**Key Management:**

```bash
# Generate 32-byte key
openssl rand -base64 32
```

**Interface Design:**

```go
type Encryptor interface {
    Encrypt(plaintext string) (string, error)
    Decrypt(ciphertext string) (string, error)
    Close() error  // Zero key from memory
}
```

## References

- [NaCl: Networking and Cryptography library](https://nacl.cr.yp.to/)
- [golang.org/x/crypto/nacl/secretbox](https://pkg.go.dev/golang.org/x/crypto/nacl/secretbox)
- [pkg/crypto/doc.go](https://github.com/specvital/core/blob/main/pkg/crypto/doc.go) - Package documentation with usage examples
