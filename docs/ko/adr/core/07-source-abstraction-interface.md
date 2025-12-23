---
title: Source 추상화 인터페이스
description: 소스 독립적 스캐닝을 위한 데이터 소스 추상화 결정
---

# ADR-07: Source 추상화 인터페이스

> 🇺🇸 [English Version](/en/adr/core/07-source-abstraction-interface.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-23 | @KubrickCode | core            |

**상태**: 승인됨

## Context

### 문제 정의

Scanner는 여러 데이터 소스에서 테스트 파일을 파싱해야 함:

1. **로컬 파일시스템**: CLI 도구 및 로컬 개발을 위한 직접 접근
2. **원격 Git 저장소**: 웹 서비스 연동을 위한 클론 및 스캔
3. **향후 소스**: S3 버킷, GitLab API 등 다른 스토리지 백엔드

### 기존 설계 문제

Scanner가 파일시스템 경로에 강하게 결합되어 있었음:

```go
// Before: Scanner가 직접 파일시스템에 접근함
func (s *Scanner) Scan(rootPath string) (*ScanResult, error) {
    filepath.WalkDir(rootPath, func(path string, ...) {
        content, _ := os.ReadFile(path)  // 직접 파일시스템 의존성
        // ...
    })
}
```

**문제점**:

- 원격 저장소 지원 추가 시 Scanner 내부 수정이 필요함
- 임시 클론에 대한 깔끔한 리소스 정리 메커니즘이 없음
- 실제 파일시스템 없이 단위 테스트가 어려움

### 요구사항

1. **소스 독립성**: Scanner가 수정 없이 어떤 데이터 소스와도 동작해야 함
2. **리소스 관리**: 임시 리소스(클론된 저장소)를 안정적으로 정리해야 함
3. **보안**: 경로 탈출 공격을 방지해야 함
4. **테스트 용이성**: 단위 테스트를 위한 Mock 소스를 사용할 수 있어야 함

## Decision

**데이터 소스 접근을 추상화하는 `Source` 인터페이스를 정의함.**

Scanner는 Source 구현체를 받아서 파일이 물리적으로 어디에 있든 그것을 통해 파일을 읽음.

```go
type Source interface {
    Root() string
    Open(ctx context.Context, path string) (io.ReadCloser, error)
    Stat(ctx context.Context, path string) (fs.FileInfo, error)
    Close() error
}
```

## Options Considered

### Option A: Source Interface 추상화 (선택됨)

파일 접근을 추상화하는 인터페이스를 도입하고, 각 소스 타입에 대한 구현체를 제공함.

**장점:**

- **소스 독립적 스캐닝**: Scanner 변경 없이 새 소스 추가 가능
- **명확한 리소스 생명주기**: `Close()`가 정리를 보장함 (임시 디렉토리, 연결)
- **테스트 용이성**: 단위 테스트를 위한 Mock 소스 사용 가능
- **확장성**: 향후 소스(S3, GitLab)가 동일한 인터페이스에 맞음

**단점:**

- 추가 추상화 레이어 존재
- GitSource는 전체 클론이 필요함 (부분 파일 접근 불가)

### Option B: Scanner에 소스 로직 포함

Scanner가 내부적으로 로컬 및 원격 접근을 처리함.

**장점:**

- 단순한 초기 구현
- 추상화 오버헤드 없음

**단점:**

- **SRP 위반**: Scanner가 파싱과 소스 접근 둘 다 처리함
- **확장성 제한**: 새 소스 타입에 Scanner 수정이 필요함
- **테스트 어려움**: 실제 파일시스템이나 네트워크가 필요함

### Option C: 원격 API 직접 접근

클론 없이 GitHub/GitLab API를 사용하여 온디맨드로 파일을 가져옴.

**장점:**

- 로컬 클론이 필요 없음
- 단일 파일 접근에 효율적

**단점:**

- **API 요청 제한**: 대규모 저장소에서 빠르게 한도에 도달함
- **디렉토리 탐색 불가**: API가 효율적인 트리 탐색을 지원하지 않음
- **복잡한 인증**: 각 제공자마다 커스텀 구현이 필요함
- **불일치 상태**: 요청 사이에 파일이 변경될 수 있음

## Interface Design

### 핵심 인터페이스

```go
type Source interface {
    // Root는 소스의 기본 경로를 반환함.
    Root() string

    // Open은 Root에 상대적인 파일을 읽기용으로 열음.
    // 호출자가 반환된 ReadCloser를 닫아야 함.
    Open(ctx context.Context, path string) (io.ReadCloser, error)

    // Stat은 Root에 상대적인 파일 메타데이터를 반환함.
    Stat(ctx context.Context, path string) (fs.FileInfo, error)

    // Close는 소스가 보유한 리소스를 해제함.
    // 멱등성: 여러 번 호출해도 안전함.
    Close() error
}
```

### 설계 원칙

1. **표준 Go 인터페이스**: 호환성을 위해 `io.ReadCloser`와 `fs.FileInfo` 사용
2. **Context 지원**: 모든 I/O 작업이 취소를 지원함
3. **상대 경로**: 모든 작업이 `Root()`에 상대적인 경로를 사용함
4. **호출자 책임**: 호출자가 파일 핸들과 소스 자체를 모두 닫아야 함

### Sentinel 에러

```go
var (
    ErrInvalidPath        = errors.New("source: invalid path")
    ErrGitCloneFailed     = errors.New("source: git clone failed")
    ErrRepositoryNotFound = errors.New("source: repository not found")
)
```

## Implementations

### LocalSource

보안 조치가 적용된 직접 파일시스템 접근임.

```go
type LocalSource struct {
    root string
}
```

**특징**:

- **경로 검증**: 루트 디렉토리를 벗어나는 경로를 거부함
- **Symlink 안전성**: 심볼릭 링크를 해석하고 최종 경로를 검증함
- **No-op Close**: 해제할 리소스가 없음

**보안**: `resolvePath()`가 디렉토리 탈출을 방지함:

```go
func (s *LocalSource) resolvePath(path string) (string, error) {
    fullPath := filepath.Join(s.root, filepath.Clean(path))

    // ".."를 통한 경로 탈출 차단
    if !strings.HasPrefix(fullPath, s.root+string(filepath.Separator)) {
        return "", ErrInvalidPath
    }

    // 심볼릭 링크를 통한 탈출 차단
    resolvedPath, _ := filepath.EvalSymlinks(fullPath)
    resolvedRoot, _ := filepath.EvalSymlinks(s.root)
    if !strings.HasPrefix(resolvedPath, resolvedRoot+string(filepath.Separator)) {
        return "", ErrInvalidPath
    }

    return fullPath, nil
}
```

### GitSource

shallow clone을 통한 원격 저장소 접근임.

```go
type GitSource struct {
    local     *LocalSource  // 파일 접근을 위임함
    tempDir   string
    commitSHA string
    branch    string
}
```

**특징**:

- **Shallow clone**: 최소 전송을 위해 `--depth 1 --single-branch` 사용
- **임시 디렉토리**: `Close()` 시 자동 정리됨
- **자격 증명 안전성**: 에러 메시지에서 자격 증명을 제거함
- **메타데이터 접근**: `CommitSHA()`와 `Branch()` 제공

**Clone 옵션**:

```go
type GitOptions struct {
    Branch      string          // 대상 브랜치 (기본값: 기본 브랜치)
    Depth       int             // 클론 깊이 (기본값: 1)
    Credentials *GitCredentials // 선택적 인증
}
```

**위임 패턴**: 클론 후 GitSource는 모든 파일 작업을 내부 LocalSource에 위임함:

```go
func (s *GitSource) Open(ctx context.Context, path string) (io.ReadCloser, error) {
    return s.local.Open(ctx, path)
}
```

### Scanner에서의 사용

```go
func (s *Scanner) Scan(ctx context.Context, src source.Source) (*ScanResult, error) {
    rootPath := src.Root()

    // 탐색은 여전히 로컬 파일시스템을 순회함
    // (GitSource는 먼저 임시 디렉토리에 클론함)
    testFiles := s.discoverTestFiles(ctx, src)

    for _, file := range testFiles {
        content, _ := readFileFromSource(ctx, src, file)
        // 콘텐츠 파싱...
    }

    return result, nil
}

// 호출자가 정리를 담당함
func main() {
    src, _ := source.NewGitSource(ctx, repoURL, nil)
    defer src.Close()  // 임시 디렉토리를 삭제함

    result, _ := scanner.Scan(ctx, src)
}
```

## Consequences

### Positive

1. **소스 독립성**
   - Scanner가 LocalSource, GitSource 또는 향후 구현체와 동작함
   - S3Source나 GitLabSource 추가 시 Scanner 변경이 필요 없음

2. **명확한 리소스 관리**
   - `defer src.Close()` 패턴이 정리를 보장함
   - GitSource가 클론된 임시 디렉토리를 제거함
   - 장기 실행 서비스에서 리소스 누수를 방지함

3. **향상된 테스트 용이성**
   - 단위 테스트에서 Mock Source 구현체를 사용할 수 있음
   - Scanner 테스트에 실제 파일시스템이나 네트워크가 필요 없음

4. **설계에 내장된 보안**
   - LocalSource에 경로 탈출 방지가 내장됨
   - GitSource 에러 메시지에서 자격 증명이 정리됨

### Negative

1. **전체 클론 필요**
   - GitSource가 전체 저장소를 클론해야 함 (shallow이지만 여전히 전체 트리)
   - 클론 없이 단일 파일 접근 불가
   - **완화**: Shallow clone(`--depth 1`)이 전송을 최소화함

2. **디렉토리 탐색은 여전히 로컬**
   - `discoverTestFiles`가 로컬 파일시스템에서 `filepath.WalkDir`를 사용함
   - GitSource가 임시 디렉토리에 클론하므로 동작함
   - **완화**: 수용 가능한 트레이드오프임. 대안은 더 나쁜 제한이 있음

### 트레이드오프 요약

| 측면          | Source Interface | 직접 파일시스템 | 원격 API |
| ------------- | ---------------- | --------------- | -------- |
| 확장성        | 우수             | 부족            | 보통     |
| 리소스 안전성 | 우수             | 부족            | 양호     |
| 테스트 용이성 | 우수             | 부족            | 양호     |
| 성능          | 양호             | 최고            | 가변적   |
| 구현 복잡도   | 보통             | 단순            | 복잡     |

## References

- [Go io.Reader 인터페이스](https://pkg.go.dev/io#Reader)
- [Go fs.FS 인터페이스](https://pkg.go.dev/io/fs#FS)
