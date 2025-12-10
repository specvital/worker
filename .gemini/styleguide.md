# Coding Convention

## Common

**One function does one thing**

- For example, if a function name requires "and" or "or" connections, it's a signal to separate
- If test cases are needed for each if branch, it's a separation signal

**Conditional and loop depth limited to 2 levels**

- Reduce depth as much as possible with early returns, and if even that becomes heavy, it's a signal to separate into a separate function

**Explicitly state function side effects**

- For example, avoid side effects like updating access information in a function with an obvious name like `getUser` that executes `updateLastAccess(...)` before `return db.user.find(...)`, rather than just returning user information.

**Convert magic numbers/strings to constants when possible**

- Usually declared at the top of the usage file or class
- Consider separating constants file if reuse is needed or the amount of constants in a file or class grows

**Function order follows call order**

- If there are clear conventions for access modifier declaration order within classes by language, follow those rules. Otherwise, write functions in call order from top to bottom for easy reading within files

**Review external library usage when implementation becomes complex**

- Review library usage when logic complexity (not simple calculations) makes test code bloated
- Use industry-standard-level libraries when available
- Use major libraries that help with security, accuracy, and performance optimization when available
- Review libraries when implementation itself is difficult due to browser/platform compatibility or countless edge cases

**Modularization (prevent code duplication and pattern repetition)**

- While it might be missed when contexts are far apart, code repetition is absolutely prohibited in recognized situations
- Modularize similar pattern repetitions (not just identical code) into reusable forms
- Allow pre-emptive modularization when reuse is almost certain, even if code hasn't repeated. However, exercise restraint when business logic is still changing
- However, don't modularize if the separated module becomes complex enough to violate other coding conventions (excessive abstraction) or cases are too simple
- Modularization levels defined as follows:
  - Within the same file, extract appropriately into separate functions
  - Separate into separate files when reused across multiple files
  - Separate into packages when reused across multiple projects or domains (same within monorepo)
- While other clear standards are difficult to define, exceptionally consider separating specific functions into separate files when they become too bloated and severely reduce code readability

**Variable, function naming**

- Variable and function names should always be clear in purpose yet concise. In ambiguous situations, first review structurally whether concerns are properly separated and purpose is unclear; if still ambiguous, err on the side of clarity rather than being too concise
- Prohibit abbreviations except industry-standard acceptable abbreviations (id, api, db, err, etc.)
- Don't repeat information already in higher context. For example, within User entity: `User.userName` -> `User.name`, within User service: `userService.createUser(...)` -> `userService.create(...)`
- Enforce prefixes like `is`, `has`, `should` for boolean variables. However, when external library interfaces differ from this rule, follow that library's rules (e.g., Chakra UI uses `disabled` not `isDisabled`)
- Prefer verb or verb+noun form for function names when possible. However, allow noun-form function names for industry-standard exceptions like GraphQL resolver fields.
- Plural rules:
  - Use "s" suffix plural variable names for pure arrays or lists. For example, type is T[] form, no metadata, directly iterable.
  - Use "list" suffix variable names for wrapped objects. For example, includes pagination info, includes metadata (count, cursor, etc.), or array nested in keys like data, items, nodes
  - Specify data structure name when using specific data structures (Set, Map, Queue): `userSet`, `userMap`, ...
  - Use as-is for already plural words (data, series, ...)

**Field order**

- All fields in objects, types, structs, interfaces, classes, etc. are defined in alphabetical ascending order by default, unless there are ordering rules or readability reasons
- Even if declaration order is well-defined for objects, structs, etc., order can be ignored at usage sites, so always maintain consistency at usage sites
- Maintain alphabetical order during destructuring assignments

**Error handling**

- Error handling level: Handle where meaningful responses (retry, fallback, user feedback) are possible; if not, propagate upward. Don't catch just to throw again.
- Error messages: Write for audience—technical details in logs, actionable guides for users. Include relevant context when wrapping errors (attempted operation, input values, system state).
- Error classification: Distinguish expected errors (validation failure, 404) from unexpected errors (network timeout, system failure). Handle each category consistently across codebase.
- Error propagation: Add context when propagating up call stack. Each layer adds its domain information while maintaining root cause.
- Recovery vs fail fast: Recover expected errors with fallback. Fail fast on unexpected errors or incorrect state—don't continue with corrupted data.

## Go

**Element order within files**

1. package declaration
2. import statements (grouped)
3. Constant definitions (const)
4. Variable definitions (var)
5. Type/Interface/Struct definitions
6. Constructor functions (New\*)
7. Methods (grouped by receiver type, alphabetical)
8. Helper functions (alphabetical)

**Error rules**

- Use %w for error chains, %v for simple logging
- Wrap internal errors that shouldn't be exposed externally with %v
- Never ignore return error with underscore for functions returning errors; handle explicitly
- Sentinel errors: Define package-level sentinel errors for expected conditions callers should handle (`var ErrNotFound = errors.New("not found")`).

**Interface definition location**

- Define interfaces in the package that uses them (Accept interfaces, return structs)
- Separate package only for interfaces commonly used across multiple packages

**Testing libraries**

- Prefer standard library's if + t.Errorf over assertion libraries (testify, etc.)
- Prefer manual implementation over gomock for mocks

**Pointer receiver rules**

- Pointer receiver for state changes, large structs (3+ fields), cases needing consistency
- Value receiver otherwise

**Context parameter**

- Always pass as first parameter
- Use context.Background() only in main and tests

**init() function**

- Avoid unless necessary for registration patterns (database drivers, plugins)
- Prefer explicit initialization functions for business logic
- Acceptable uses:
  - Driver/plugin registration (e.g., `database/sql` drivers)
  - Static route/handler registration with no I/O
  - Complex constant initialization without side effects
- Forbidden uses:
  - External I/O (database, file, network)
  - Global state mutation
  - Error-prone initialization (use constructors that return errors)

**internal package**

- Actively use for libraries, only when necessary for applications

**Recommended libraries**

- Web: Chi
- DB: Bun, SQLBoiler (when managing external migrations)
- Logging: slog
- CLI: cobra
- Utility: samber/lo, golang.org/x/sync
- Config management: koanf (viper if cobra integration needed)
- Validation: go-playground/validator/v10
- Scheduling: github.com/go-co-op/gocron
- Image processing: github.com/h2non/bimg

# Test Guide

## Common Principles

### Test File Structure

1:1 matching with target file. Test files located in same directory as target files.

### Test Hierarchy

Organize major sections by method (function) units, write minor sections for each case. Complex methods can add intermediate sections by scenario.

### Test Scope Selection

Omit obvious or overly simple logic (simple getters, constant returns). Prioritize testing business logic, conditional branches, code with external dependencies.

### Test Case Composition

Minimum 1 basic success case required. Main focus on failure cases, boundaries, edge cases, exception scenarios.

### Test Independence

Each test must be independently executable. Prohibit dependency on execution order between tests. Initialize for each test when using shared state.

### Given-When-Then Pattern

Structure test code in 3 stages—Given (setup), When (execution), Then (verification). Distinguish stages with comments or blank lines for complex tests.

### Test Data

Use hardcoded meaningful values. Avoid random data as it causes irreproducible failures. Fix seed if necessary.

### Mocking Principles

Mock external dependencies (API, DB, file system). Use actual modules within same project when possible, mock only when complexity is high.

### Test Reusability

Extract repeated mocking setups, fixtures, helper functions as common utilities. However, be careful not to harm test readability with excessive abstraction.

### Integration/E2E Tests

Unit tests take priority. Write integration/E2E when complex flows or multi-module interactions are difficult to understand from code alone. Located in separate directories (`tests/integration`, `tests/e2e`).

### Test Naming

Test names should clearly express "what is being tested". Recommend "when ~ should ~" format. Focus on behavior rather than implementation details.

### Assertion Count

Allow multiple related assertions in one test, but separate tests when verifying different concepts.

---

## Go

### File Naming

`{target-filename}_test.go` format.

**Example:** `user.go` → `user_test.go`

### Test Functions

`func TestXxx(t *testing.T)` format. Write `TestMethodName` function per method, compose subtests with `t.Run()`.

### Subtests

`t.Run("case name", func(t *testing.T) {...})` pattern. Each case independently executable, call `t.Parallel()` for parallel execution.

### Table-Driven Tests

Recommend table-driven tests when multiple cases have similar structure. Define cases with `[]struct{ name, input, want, wantErr }`.

**Example:**

```go
tests := []struct {
    name    string
    input   int
    want    int
    wantErr bool
}{
    {"normal case", 5, 10, false},
    {"negative input", -1, 0, true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Func(tt.input)
        if (err != nil) != tt.wantErr { ... }
        if got != tt.want { ... }
    })
}
```

### Mocking

Utilize interface-based dependency injection. Prioritize manual mocking, consider gomock for complex cases. Define test-only implementations within `_test.go`.

### Error Verification

Use `errors.Is()`, `errors.As()`. Avoid error message string comparison, verify with sentinel errors or error types.

### Setup/Teardown

Global setup/teardown with `TestMain(m *testing.M)`. Individual test preparation within each Test function or extracted as helper functions.

### Test Helpers

Extract repeated preparation/verification as `testXxx(t *testing.T, ...)` helpers. Receive `*testing.T` as first argument and call `t.Helper()`.

### Benchmarks

Write `func BenchmarkXxx(b *testing.B)` for performance-critical code. Repeat with `b.N` loop, exclude preparation time with `b.ResetTimer()`.
