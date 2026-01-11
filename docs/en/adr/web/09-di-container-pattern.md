---
title: DI Container Pattern
description: ADR on migrating from sync.Once singleton to Container-based dependency injection for improved testability
---

# ADR-09: DI Container Pattern

> [한국어 버전](/ko/adr/web/09-di-container-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### Initial Pattern: sync.Once Singleton

The initial codebase used Go's `sync.Once` pattern for initializing shared dependencies:

**Problems Identified:**

- **Testing Difficulty**: Singleton instances couldn't be replaced with mocks
- **Hidden Dependencies**: Modules accessed global singletons without explicit declaration
- **Initialization Order**: Implicit dependency order made debugging initialization failures difficult
- **Lifecycle Management**: No centralized cleanup mechanism for resources

### Evolution with Clean Architecture

As the codebase migrated to Clean Architecture ([ADR-08](/en/adr/web/08-clean-architecture-pattern.md)), the need for explicit dependency injection became critical:

- **UseCase layer** requires port interfaces to be injected
- **Handler layer** requires UseCases to be injected
- **Adapter layer** requires infrastructure clients (DB, Queue) to be injected

### Worker Service Alignment

The Worker service uses a Container pattern with separate containers per entry point (Worker, Scheduler). Adopting a similar pattern ensures consistency across repositories.

## Decision

**Migrate from sync.Once singleton to Container-based dependency injection.**

### Two-Tier Container Architecture

| Container           | Location                      | Responsibility                                            |
| ------------------- | ----------------------------- | --------------------------------------------------------- |
| **infra.Container** | `internal/infra/container.go` | Infrastructure dependencies (DB, Queue, OAuth, Encryptor) |
| **server.App**      | `common/server/app.go`        | Application dependencies (Handlers, UseCases, Adapters)   |

### Dependency Flow

```
main.go
    └─→ server.NewApp()
            └─→ infra.NewContainer() → Container{DB, River, OAuth, JWT, ...}
            └─→ initHandlers(container)
                    └─→ Adapter (using container.DB)
                    └─→ UseCase (using ports)
                    └─→ Handler (using usecases)
            └─→ App{Handlers, Middleware, infra}
```

## Options Considered

### Option A: Container-Based DI (Selected)

**How It Works:**

- `Config` struct separates configuration from container creation
- `ConfigFromEnv()` loads environment variables into Config
- `NewContainer()` creates all infrastructure dependencies
- `App` assembles handlers and usecases using container
- `Close()` method handles cleanup in reverse order

**Pros:**

- **Testability**: Dependencies injected via constructors; easily mocked
- **Explicit Dependencies**: All dependencies visible in Container struct
- **Lifecycle Control**: Centralized cleanup with Close() method
- **Consistency**: Matches Worker service pattern

**Cons:**

- Initial setup complexity
- Boilerplate code for dependency wiring
- All dependencies created upfront (even if unused)

### Option B: sync.Once Singleton (Previous)

**How It Works:**

- Each module initializes its singleton via `sync.Once`
- Singletons accessed via package-level `Get*()` functions
- No central registry of dependencies

**Evaluation:**

- Simpler initial implementation
- No explicit wiring required
- Testing requires global state manipulation
- Dependency order bugs appear at runtime
- **Rejected**: Insufficient for Clean Architecture testing requirements

### Option C: Wire/Fx DI Framework

**How It Works:**

- Google Wire: Compile-time dependency injection via code generation
- Uber Fx: Runtime DI container with lifecycle hooks

**Evaluation:**

- Reduces boilerplate code
- Adds external dependency
- Magic code generation (Wire) or reflection (Fx)
- Overkill for current scale (~10 dependencies)
- **Rejected**: Added complexity without proportional benefit

### Option D: Manual Constructor Injection

**How It Works:**

- Each module defines constructors accepting dependencies
- No central container; dependencies passed through call chain
- main.go constructs entire dependency graph

**Evaluation:**

- Maximum explicitness
- Long constructor parameter lists
- Requires threading dependencies through multiple layers
- **Rejected**: Container provides better organization

## Implementation

### Config Separation

Configuration is separated from container creation to enable:

- Environment-specific configuration
- Test configuration injection
- Validation before resource allocation

```go
type Config struct {
    DatabaseURL   string
    EncryptionKey string
    JWTSecret     string
    // ... other fields
}

func ConfigFromEnv() Config {
    return Config{
        DatabaseURL:   os.Getenv("DATABASE_URL"),
        EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
        // ...
    }
}
```

### Container Creation

Container creates and holds all infrastructure dependencies:

```go
type Container struct {
    DB             *pgxpool.Pool
    River          *RiverClient
    Encryptor      crypto.Encryptor
    JWTManager     authport.TokenManager
    GitHubOAuth    authport.OAuthClient
    GitHubAppClient ghappport.GitHubAppClient
    // ... other dependencies
}

func NewContainer(ctx context.Context, cfg Config) (*Container, error) {
    // Validation
    if err := validateConfig(cfg); err != nil {
        return nil, err
    }

    // Create dependencies in order
    pool, err := NewPostgresPool(ctx, PostgresConfig{URL: cfg.DatabaseURL})
    // ... create other dependencies

    return &Container{DB: pool, ...}, nil
}
```

### Application Assembly

App uses Container to create business-layer dependencies:

```go
type App struct {
    AuthMiddleware *middleware.AuthMiddleware
    Handlers       *Handlers
    infra          *infra.Container
}

func NewApp(ctx context.Context) (*App, error) {
    cfg := infra.ConfigFromEnv()
    container, err := infra.NewContainer(ctx, cfg)

    handlers, err := initHandlers(container)
    // handlers creates: adapters → usecases → handlers

    return &App{Handlers: handlers, infra: container}, nil
}
```

### RouteRegistrar Interface

Standardizes route registration across modules:

```go
type RouteRegistrar interface {
    RegisterRoutes(r chi.Router)
}
```

### Resource Cleanup

Cleanup in reverse creation order:

```go
func (c *Container) Close() error {
    if c.DB != nil {
        c.DB.Close()
    }
    return nil
}

func (a *App) Close() error {
    return a.infra.Close()
}
```

## Consequences

### Positive

**Testability:**

- Handlers testable with mock UseCases
- UseCases testable with mock Ports
- No global state manipulation required
- Test setup isolated per test case

**Explicit Dependencies:**

- All dependencies visible in Container/App structs
- Dependency graph clear from code structure
- No hidden coupling via global singletons

**Lifecycle Management:**

- Resources created in defined order
- Cleanup guaranteed via Close() chain
- Graceful shutdown support in main.go

**Consistency:**

- Matches Worker service Container pattern
- Familiar structure for developers working across repositories
- Shared mental model for dependency management

### Negative

**Initial Complexity:**

- More files than singleton approach
- Understanding container flow requires documentation
- **Mitigation**: CLAUDE.md documents pattern; clear naming conventions

**Boilerplate:**

- Explicit wiring in initHandlers() (~150 lines)
- Each new module requires wiring additions
- **Mitigation**: Acceptable trade-off for explicitness and testability

**Upfront Creation:**

- All dependencies created at startup
- Unused dependencies still consume resources
- **Mitigation**: Current scale doesn't warrant lazy initialization

## References

- [Dependency Injection in Go - Alex Edwards](https://www.alexedwards.net/blog/organising-database-access)
- [Worker ADR-02: Clean Architecture Layers](/en/adr/worker/02-clean-architecture-layers.md)
- [Web ADR-08: Clean Architecture Pattern](/en/adr/web/08-clean-architecture-pattern.md)
