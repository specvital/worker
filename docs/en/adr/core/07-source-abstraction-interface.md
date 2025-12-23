---
title: Source Abstraction Interface
description: ADR on abstracting data sources for source-agnostic scanning
---

# ADR-07: Source Abstraction Interface

> 🇰🇷 [한국어 버전](/ko/adr/core/07-source-abstraction-interface.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

The Scanner needs to parse test files from multiple data sources:

1. **Local filesystem**: Direct access for CLI tools and local development
2. **Remote Git repositories**: Clone and scan for web service integration
3. **Future sources**: S3 buckets, GitLab API, or other storage backends

### Original Design Issue

Scanner was tightly coupled to filesystem paths:

```go
// Before: Scanner directly accessed filesystem
func (s *Scanner) Scan(rootPath string) (*ScanResult, error) {
    filepath.WalkDir(rootPath, func(path string, ...) {
        content, _ := os.ReadFile(path)  // Direct filesystem dependency
        // ...
    })
}
```

**Problems**:

- Adding remote repository support required modifying Scanner internals
- No clean resource cleanup mechanism for temporary clones
- Difficult to unit test without real filesystem

### Requirements

1. **Source Independence**: Scanner should work with any data source without modification
2. **Resource Management**: Clean up temporary resources (cloned repos) reliably
3. **Security**: Prevent path traversal attacks
4. **Testability**: Enable mock sources for unit testing

## Decision

**Define a `Source` interface to abstract data source access.**

The Scanner receives a `Source` implementation and reads files through it, regardless of where files physically reside.

```go
type Source interface {
    Root() string
    Open(ctx context.Context, path string) (io.ReadCloser, error)
    Stat(ctx context.Context, path string) (fs.FileInfo, error)
    Close() error
}
```

## Options Considered

### Option A: Source Interface Abstraction (Selected)

Introduce an interface that abstracts file access, with implementations for different source types.

**Pros:**

- **Source-agnostic scanning**: Add new sources without Scanner changes
- **Clear resource lifecycle**: `Close()` ensures cleanup (temp directories, connections)
- **Testability**: Mock sources for unit tests
- **Extensibility**: Future sources (S3, GitLab) fit the same interface

**Cons:**

- Additional abstraction layer
- GitSource requires full clone (no partial file access)

### Option B: Embed Source Logic in Scanner

Scanner handles local and remote access internally.

**Pros:**

- Simple initial implementation
- No abstraction overhead

**Cons:**

- **SRP violation**: Scanner handles both parsing and source access
- **Limited extensibility**: New source types require Scanner modification
- **Testing difficulty**: Requires real filesystem or network

### Option C: Direct Remote API Access

Use GitHub/GitLab APIs to fetch files on-demand without cloning.

**Pros:**

- No local clone needed
- Efficient for single-file access

**Cons:**

- **API rate limits**: Large repos hit limits quickly
- **No directory traversal**: APIs don't support efficient tree walking
- **Complex authentication**: Each provider needs custom implementation
- **Inconsistent state**: Files may change between requests

## Interface Design

### Core Interface

```go
type Source interface {
    // Root returns the base path for the source.
    Root() string

    // Open opens a file relative to Root for reading.
    // Caller must close the returned ReadCloser.
    Open(ctx context.Context, path string) (io.ReadCloser, error)

    // Stat returns file metadata relative to Root.
    Stat(ctx context.Context, path string) (fs.FileInfo, error)

    // Close releases resources held by the source.
    // Idempotent: safe to call multiple times.
    Close() error
}
```

### Design Principles

1. **Standard Go interfaces**: Uses `io.ReadCloser` and `fs.FileInfo` for compatibility
2. **Context support**: All I/O operations support cancellation
3. **Relative paths**: All operations use paths relative to `Root()`
4. **Caller responsibility**: Caller must close both file handles and the source itself

### Sentinel Errors

```go
var (
    ErrInvalidPath        = errors.New("source: invalid path")
    ErrGitCloneFailed     = errors.New("source: git clone failed")
    ErrRepositoryNotFound = errors.New("source: repository not found")
)
```

## Implementations

### LocalSource

Direct filesystem access with security measures.

```go
type LocalSource struct {
    root string
}
```

**Features**:

- **Path validation**: Rejects paths escaping root directory
- **Symlink safety**: Resolves symlinks and validates final path
- **No-op Close**: No resources to release

**Security**: `resolvePath()` prevents directory traversal:

```go
func (s *LocalSource) resolvePath(path string) (string, error) {
    fullPath := filepath.Join(s.root, filepath.Clean(path))

    // Block path escape via ".."
    if !strings.HasPrefix(fullPath, s.root+string(filepath.Separator)) {
        return "", ErrInvalidPath
    }

    // Block symlink escape
    resolvedPath, _ := filepath.EvalSymlinks(fullPath)
    resolvedRoot, _ := filepath.EvalSymlinks(s.root)
    if !strings.HasPrefix(resolvedPath, resolvedRoot+string(filepath.Separator)) {
        return "", ErrInvalidPath
    }

    return fullPath, nil
}
```

### GitSource

Remote repository access via shallow clone.

```go
type GitSource struct {
    local     *LocalSource  // Delegates file access
    tempDir   string
    commitSHA string
    branch    string
}
```

**Features**:

- **Shallow clone**: `--depth 1 --single-branch` for minimal transfer
- **Temporary directory**: Auto-cleaned on `Close()`
- **Credential safety**: Strips credentials from error messages
- **Metadata access**: Provides `CommitSHA()` and `Branch()`

**Clone Options**:

```go
type GitOptions struct {
    Branch      string          // Target branch (default: default branch)
    Depth       int             // Clone depth (default: 1)
    Credentials *GitCredentials // Optional authentication
}
```

**Delegation Pattern**: After cloning, GitSource delegates all file operations to an internal LocalSource:

```go
func (s *GitSource) Open(ctx context.Context, path string) (io.ReadCloser, error) {
    return s.local.Open(ctx, path)
}
```

### Usage in Scanner

```go
func (s *Scanner) Scan(ctx context.Context, src source.Source) (*ScanResult, error) {
    rootPath := src.Root()

    // Discovery still walks local filesystem
    // (GitSource clones to temp directory first)
    testFiles := s.discoverTestFiles(ctx, src)

    for _, file := range testFiles {
        content, _ := readFileFromSource(ctx, src, file)
        // Parse content...
    }

    return result, nil
}

// Caller is responsible for cleanup
func main() {
    src, _ := source.NewGitSource(ctx, repoURL, nil)
    defer src.Close()  // Removes temp directory

    result, _ := scanner.Scan(ctx, src)
}
```

## Consequences

### Positive

1. **Source Independence**
   - Scanner works with LocalSource, GitSource, or future implementations
   - Adding S3Source or GitLabSource requires no Scanner changes

2. **Clear Resource Management**
   - `defer src.Close()` pattern ensures cleanup
   - GitSource removes cloned temp directory
   - Prevents resource leaks in long-running services

3. **Improved Testability**
   - Unit tests can use mock Source implementations
   - No real filesystem or network required for Scanner tests

4. **Security by Design**
   - Path traversal prevention built into LocalSource
   - Credential sanitization in GitSource error messages

### Negative

1. **Full Clone Required**
   - GitSource must clone entire repository (shallow, but still full tree)
   - Cannot access single file without cloning
   - **Mitigation**: Shallow clone (`--depth 1`) minimizes transfer

2. **Directory Discovery Still Local**
   - `discoverTestFiles` uses `filepath.WalkDir` on local filesystem
   - Works because GitSource clones to temp directory
   - **Mitigation**: Acceptable trade-off; alternatives have worse limitations

### Trade-off Summary

| Aspect          | Source Interface | Direct Filesystem | Remote API |
| --------------- | ---------------- | ----------------- | ---------- |
| Extensibility   | Excellent        | Poor              | Moderate   |
| Resource safety | Excellent        | Poor              | Good       |
| Testability     | Excellent        | Poor              | Good       |
| Performance     | Good             | Best              | Variable   |
| Implementation  | Moderate         | Simple            | Complex    |

## References

- [Go io.Reader interface](https://pkg.go.dev/io#Reader)
- [Go fs.FS interface](https://pkg.go.dev/io/fs#FS)
