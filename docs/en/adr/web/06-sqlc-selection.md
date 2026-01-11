---
title: SQLc Selection
description: ADR on choosing SQLc for compile-time type-safe database access in Go backend
---

# ADR-06: SQLc Selection

> [한국어 버전](/ko/adr/web/06-sqlc-selection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-18 | @KubrickCode | web   |

## Context

### Database Access Layer Requirements

The web platform requires a database access strategy that meets the following criteria:

1. **Type Safety**: Compile-time error detection for SQL queries
2. **SQL Control**: Full access to PostgreSQL features (LATERAL JOINs, CTEs, window functions, cursor pagination)
3. **Performance**: Minimal runtime overhead
4. **Clean Architecture Compatibility**: Generated code must fit the port/adapter pattern
5. **PostgreSQL-Specific Support**: Native handling of enums, arrays, UUIDs, JSONB

### The Challenge with ORMs

Traditional ORMs abstract SQL behind object-oriented interfaces. While this simplifies CRUD operations, it creates friction for complex queries:

| Query Pattern                        | ORM Approach                  | Raw SQL Approach |
| ------------------------------------ | ----------------------------- | ---------------- |
| LATERAL JOIN                         | Not supported or escape hatch | Native           |
| Cursor pagination with compound keys | Complex custom code           | Straightforward  |
| Dynamic sort order                   | Multiple query methods        | CASE expressions |
| PostgreSQL-specific types            | Manual type registration      | Native support   |

### Existing Infrastructure

The project already adopted:

- **PostgreSQL** as the primary database (NeonDB in production)
- **River** as PostgreSQL-backed job queue (chosen for transactional consistency)
- **pgx/v5** as the PostgreSQL driver (connection pooling, native types)

The database access layer must integrate seamlessly with this stack.

### Migration Architecture

Database migrations are managed in a separate `infra` repository, shared by both `web` and `worker` services. This means:

- Web service only needs database **connection**, not migration management
- Schema is the single source of truth, maintained externally
- Tools with built-in migration features (GORM, Ent) add unnecessary complexity

### AI-Assisted Development Considerations

In the era of AI-assisted development (Claude Code, GitHub Copilot, etc.):

- **AI writes SQL naturally**: LLMs excel at generating optimized SQL queries directly
- **No abstraction overhead**: AI doesn't need ORM abstractions to be productive
- **Human readability**: Developers can infer query intent from generated method names (e.g., `GetPaginatedRepositoriesByRecent`)
- **Bidirectional clarity**: AI writes raw SQL → SQLc generates typed methods → Developers understand intent

## Decision

**Adopt SQLc with pgx/v5 for compile-time type-safe database access.**

Core principles:

1. **SQL-First**: Write optimized SQL queries directly, not through ORM abstractions
2. **Compile-Time Safety**: Type errors caught before runtime via generated Go code
3. **Zero Abstraction Overhead**: No query building or reflection at runtime
4. **PostgreSQL Native**: Direct pgx driver integration for maximum performance

Configuration:

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

### Option A: SQLc (Selected)

**How It Works:**

1. Write SQL queries in `.sql` files with annotations
2. Run `sqlc generate` to create type-safe Go code
3. Call generated functions with proper types

**Pros:**

- **Compile-Time Type Safety**: Column/type mismatches caught at build time
- **Full SQL Control**: Any PostgreSQL feature available without escape hatches
- **Zero Runtime Overhead**: No reflection, no query building
- **PostgreSQL Native Types**: Enums, arrays, UUIDs work seamlessly with pgx
- **Clean Generated Code**: Idiomatic Go, easy to understand and debug
- **Active Community**: 16,600+ GitHub stars, regular releases

**Cons:**

- **Dynamic Queries Limited**: Requires boolean flag patterns or multiple queries
- **SQL Knowledge Required**: Team must write and optimize SQL directly (mitigated by AI assistance)
- **Regeneration Required**: Schema changes require running `sqlc generate`
- **No Migration Support**: Migrations handled separately (fits our infra repository pattern)

### Option B: GORM

**How It Works:**

- Runtime reflection-based ORM
- Define structs with tags, ORM generates queries
- Auto-migration, associations, hooks

**Pros:**

- Largest Go ORM community (39,000+ stars)
- Feature-rich ecosystem
- Easy onboarding for ORM-familiar developers

**Cons:**

- **30-50% Performance Overhead**: Reflection-based query building
- **Runtime Errors**: Column mismatches discovered only at runtime
- **Complex Query Limitations**: LATERAL JOINs, CTEs require raw SQL escape
- **N+1 Query Problems**: Easy to introduce without explicit preloading
- **Type Safety Gap**: Struct tags not validated at compile time

### Option C: Ent (Facebook)

**How It Works:**

- Code-generation based ORM from Facebook
- Define schemas in Go, generate CRUD operations
- Graph-based relationship traversal

**Pros:**

- Compile-time type safety (similar to SQLc)
- Elegant handling of entity relationships
- No reflection overhead

**Cons:**

- **Steeper Learning Curve**: Custom DSL and graph concepts
- **Complex Custom Queries**: "Break the glass" escapes to raw SQL
- **Generated Code Bloat**: Many generated files for entity graph
- **PostgreSQL-Specific Features**: Requires workarounds for advanced features

### Option D: Bun

**How It Works:**

- SQL-first query builder with ORM features
- Thin layer over `database/sql`
- Explicit by design

**Pros:**

- Excellent performance (near raw SQL)
- Good PostgreSQL support
- Less abstraction than GORM

**Cons:**

- **No Compile-Time Safety**: Query errors at runtime
- **Smaller Community**: ~4,000 stars vs SQLc's 16,000
- **Type Inference Limited**: Manual struct mapping required

### Option E: Raw database/sql

**How It Works:**

- Standard library approach
- Manual query writing and row scanning
- Full SQL control

**Pros:**

- Zero dependencies
- Maximum performance
- Complete control

**Cons:**

- **No Type Safety**: Runtime errors for column mismatches
- **Boilerplate Heavy**: Manual struct scanning for every query
- **Maintenance Burden**: Schema changes require manual updates everywhere
- **Error-Prone**: Easy to miss columns or mistype names

## Implementation Details

### Query Organization

```
queries/
├── analysis.sql      # Analysis-related queries
├── auth.sql          # Authentication queries
├── bookmark.sql      # User bookmarks
├── github.sql        # GitHub repository data
├── github_app.sql    # GitHub App installations
├── river_job.sql     # Job queue queries
└── user_analysis_history.sql
```

### Complex Query Example

The project uses advanced PostgreSQL features that ORMs cannot handle elegantly:

```sql
-- Cursor-based pagination with LATERAL JOIN and dynamic sort
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

Key features used:

- **LATERAL JOIN**: Correlated subquery for "latest per group"
- **Cursor Pagination**: Compound key `(completed_at, id)` for stable ordering
- **Dynamic Sort**: CASE expressions for ascending/descending
- **Type-Safe Parameters**: `sqlc.arg()` generates typed function arguments

### Generated Code Quality

SQLc generates idiomatic Go that integrates cleanly with the repository pattern:

```go
// Generated struct with JSON tags
type GetPaginatedRepositoriesRow struct {
    CodebaseID   pgtype.UUID        `json:"codebase_id"`
    Owner        string             `json:"owner"`
    Name         string             `json:"name"`
    AnalysisID   pgtype.UUID        `json:"analysis_id"`
    AnalyzedAt   pgtype.Timestamptz `json:"analyzed_at"`
    TotalTests   int32              `json:"total_tests"`
}

// Generated function with proper context and error handling
func (q *Queries) GetPaginatedRepositories(ctx context.Context, arg GetPaginatedRepositoriesParams) ([]GetPaginatedRepositoriesRow, error)
```

### Clean Architecture Integration

```
modules/{module}/
├── domain/port/
│   └── repository.go         # Interface definition
├── adapter/
│   └── repository_postgres.go  # Uses db.Queries
└── internal/db/
    └── *.sql.go              # SQLc generated
```

The adapter implements the port interface using SQLc's generated `Queries`:

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

### Positive

**Type Safety:**

- Query column/type mismatches caught at compile time
- Refactoring confidence: IDE can track all usages
- No runtime SQL parsing errors

**Performance:**

- pgx/v5 provides 30-50% better throughput than `database/sql` with reflection ORMs
- No query building overhead at runtime
- Connection pooling via pgxpool

**Developer Experience:**

- Write optimized SQL directly
- Generated code is readable and debuggable
- Familiar tools: any SQL editor, EXPLAIN ANALYZE

**Architecture:**

- Clean separation: SQL in `.sql` files, Go in adapters
- Generated code fits port/adapter pattern naturally
- No ORM-specific abstractions leak into domain layer
- No migration coupling: SQLc is read-only, migrations managed in `infra` repo

**AI-Assisted Development:**

- AI agents write optimized raw SQL without abstraction friction
- Method names are self-documenting: `GetPaginatedRepositoriesByRecent` clearly indicates behavior
- Developers review AI-generated queries through readable method signatures
- No ORM "magic" to debug: what you write is what you execute

### Negative

**Dynamic Query Limitations:**

- Truly dynamic queries (variable WHERE clauses) require multiple query files
- **Mitigation**: Boolean flag pattern handles most cases; Squirrel query builder for extreme cases

**SQL Knowledge Required:**

- Team must be comfortable writing and optimizing SQL
- **Mitigation**: Team already proficient; SQL is a transferable skill

**Regeneration Workflow:**

- Schema changes require running `sqlc generate`
- **Mitigation**: Integrated into `just gen-sqlc` command; CI validates generated code is up-to-date

## References

- [ADR-01: Go as Backend Language](/en/adr/web/01-go-backend-language.md)
- [ADR-04: Queue-Based Asynchronous Processing](/en/adr/04-queue-based-async-processing.md)
- [ADR-07: Shared Infrastructure Strategy](/en/adr/07-shared-infrastructure.md)
- [SQLc Documentation](https://docs.sqlc.dev/)
