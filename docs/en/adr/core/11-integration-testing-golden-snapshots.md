---
title: Integration Testing with Golden Snapshots
description: ADR on using real-world repositories and snapshot comparison for regression testing
---

# ADR-11: Integration Testing with Golden Snapshots

> 🇰🇷 [한국어 버전](/ko/adr/core/11-integration-testing-golden-snapshots.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

SpecVital Core parser must accurately detect tests across:

- **Multiple test frameworks** (Jest, Vitest, Playwright, JUnit, pytest, etc.)
- **Multiple programming languages** (JavaScript, TypeScript, Python, Go, Java, C#, etc.)
- **Complex repository structures** (monorepos, mixed frameworks, polyglot projects)

### Challenges with Unit Tests Alone

1. **Limited Coverage**: Synthetic test fixtures cannot anticipate all real-world code patterns
2. **Regression Risk**: Parser changes may silently break detection in frameworks not covered by unit tests
3. **Edge Cases**: Monorepos with nested configs, multiple frameworks in single repo, unconventional file structures

### Requirements

1. **High Confidence**: Verify parser behavior against real codebases
2. **Regression Detection**: Automatically detect when parser output changes
3. **Fast Feedback**: Re-runs should be quick despite testing many repositories
4. **Deterministic Results**: Same input should produce same output

## Decision

**Use real open source GitHub repositories with golden snapshot comparison for integration testing.**

Integration tests:

1. Clone real repositories from GitHub
2. Run the full parsing pipeline
3. Compare results against pre-recorded golden snapshots
4. Fail if any differences are detected (unless intentionally updated)

## Options Considered

### Option A: Golden Snapshots with Real Repositories (Selected)

Clone real open source projects, parse them, and compare against saved snapshots.

**Pros:**

- **Real-world validation**: Tests against actual codebases, not synthetic fixtures
- **Automatic regression detection**: Any parser change that affects output is immediately visible
- **Edge case coverage**: Real projects naturally include edge cases developers wouldn't think to create
- **Framework diversity**: Test all supported frameworks with their canonical projects
- **Documentation value**: Repository list serves as proof of framework support

**Cons:**

- **Initial execution time**: Full test suite requires cloning many repositories
- **External dependency**: Tests depend on GitHub repository availability
- **Maintenance burden**: Snapshot updates required when parser behavior intentionally changes
- **Network requirement**: Initial run requires internet access

### Option B: Synthetic Test Fixtures Only

Create artificial test files that exercise known patterns.

**Pros:**

- Fast execution, no network dependency
- Full control over test cases
- Simple to understand

**Cons:**

- **Coverage gaps**: Cannot anticipate all real-world patterns
- **False confidence**: Tests pass but parser fails on real code
- **Maintenance overhead**: Must manually create fixtures for each framework
- **Missing edge cases**: Real-world complexity not captured

### Option C: Runtime Output Verification Only

Run parser and verify basic invariants (non-empty output, no errors) without snapshot comparison.

**Pros:**

- No snapshot maintenance
- Simple implementation

**Cons:**

- **Silent regressions**: Output changes go undetected
- **Low confidence**: Passing test doesn't mean correct output
- **No baseline**: No record of expected behavior

## Implementation Details

### Repository Configuration

Repositories are defined in `repos.yaml`:

```yaml
repositories:
  - name: vite
    url: https://github.com/vitejs/vite
    ref: v6.0.0
    frameworks:
      - vitest

  - name: grafana
    url: https://github.com/grafana/grafana
    ref: v11.3.1
    frameworks:
      - cypress
      - go-testing
      - jest
      - playwright
    complex: true
    nondeterministic: true
```

**Fields:**

| Field              | Description                                    |
| ------------------ | ---------------------------------------------- |
| `name`             | Repository identifier                          |
| `url`              | GitHub repository URL                          |
| `ref`              | Git tag or branch (pinned for reproducibility) |
| `frameworks`       | Expected frameworks to detect                  |
| `complex`          | Flag for monorepos or mixed framework projects |
| `nondeterministic` | Skip strict snapshot comparison                |

### Shallow Clone with Caching

```
tests/integration/testdata/
├── cache/              # Cloned repositories
│   ├── vite-v6.0.0/
│   ├── grafana-v11.3.1/
│   └── .clone_complete # Marker files
└── golden/             # JSON snapshots
    ├── vite-v6.0.0.json
    └── grafana-v11.3.1.json
```

**Optimization strategies:**

- **Shallow clone** (`--depth=1`): Minimize download size and time
- **Single branch** (`--single-branch`): Only fetch specified ref
- **Completion marker**: `.clone_complete` file prevents re-clone of partial downloads
- **Cache persistence**: Cloned repos reused across test runs

### Golden Snapshot Structure

```json
{
  "repository": "gin",
  "ref": "v1.10.0",
  "expectedFrameworks": ["go-testing"],
  "fileCount": 38,
  "testCount": 483,
  "frameworkCounts": {
    "go-testing": 38
  },
  "sampleFiles": [
    {
      "path": "auth_test.go",
      "framework": "go-testing",
      "suiteCount": 0,
      "testCount": 9
    }
  ],
  "stats": {
    "filesMatched": 38,
    "filesScanned": 38
  }
}
```

**Comparison points:**

- `fileCount`: Total number of test files detected
- `testCount`: Total number of test cases detected
- `frameworkCounts`: Distribution of files per framework
- `sampleFiles`: First N files (sorted by path for determinism)

### Nondeterministic Repositories

Some repositories exhibit non-deterministic parsing results due to:

- **Parallel execution race conditions**: File discovery order varies
- **Dynamic test generation**: Runtime-dependent test counts
- **Build artifacts**: Generated files may or may not be present

For these repositories, set `nondeterministic: true` to:

- Run full parsing pipeline
- Verify framework detection
- Skip strict snapshot comparison

### Framework Validation

Multi-framework repositories validate all expected frameworks:

```go
func validateFrameworkMatch(t *testing.T, expected []string, actual map[string]int) {
    // Bidirectional check: missing expected + unexpected detected
    // Prevents silent failures where secondary frameworks go undetected
}
```

This ensures parser changes don't silently break detection of secondary frameworks.

## Test Workflow

### Running Integration Tests

```bash
just test integration         # Run all integration tests
```

### Updating Snapshots

**When**: After intentional parser changes (new features, bug fixes)

**Never**: To "fix" failing tests without understanding why output changed

```bash
just snapshot-update              # Update all snapshots
just snapshot-update repo=vite    # Update specific repository
```

### CI/CD Integration

Integration tests should run:

- On pull requests (detect regressions before merge)
- On main branch (verify release candidates)
- With timeout (~15 minutes for full suite)

## Consequences

### Positive

1. **High Confidence Testing**
   - Real-world validation beyond synthetic fixtures
   - Coverage of edge cases developers wouldn't anticipate
   - Proof of framework support via canonical projects

2. **Automatic Regression Detection**
   - Any output change immediately visible
   - Diff output shows exact changes
   - Prevents silent parser degradation

3. **Fast Re-runs**
   - Cached clones eliminate network overhead
   - Parallel test execution via `t.Parallel()`
   - Typical re-run: seconds (vs minutes for initial clone)

4. **Self-Documenting**
   - `repos.yaml` serves as supported framework documentation
   - Golden snapshots show expected parser behavior
   - Test failures provide actionable diff output

### Negative

1. **Initial Execution Time**
   - First run requires cloning all repositories
   - **Mitigation**: Shallow clones, parallel execution, CI caching

2. **External Dependency**
   - Tests fail if GitHub is unreachable
   - **Mitigation**: Cached clones, offline fallback possible

3. **Snapshot Maintenance**
   - Intentional changes require snapshot updates
   - **Mitigation**: Clear update command, documented workflow

4. **Disk Space**
   - Cached repositories consume storage
   - **Mitigation**: Shallow clones minimize size, `.gitignore` excludes cache

### Trade-off Summary

| Aspect              | Golden Snapshots   | Synthetic Fixtures |
| ------------------- | ------------------ | ------------------ |
| Real-world coverage | Excellent          | Limited            |
| Regression detect   | Automatic          | Manual             |
| Initial setup       | Slow (clone repos) | Fast               |
| Re-run speed        | Fast (cached)      | Fast               |
| Maintenance         | Update on changes  | Add new fixtures   |
| Confidence level    | High               | Medium             |

## References

- Test infrastructure: `tests/integration/`
- Repository definitions: `tests/integration/repos.yaml`
- Golden snapshots: `tests/integration/testdata/golden/`
