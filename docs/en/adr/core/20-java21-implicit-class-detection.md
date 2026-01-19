---
title: Java 21+ Implicit Class Detection
description: ADR for extending the JUnit5 parser to support Java 21+ implicit classes (JEP 445/JEP 463)
---

# ADR-20: Java 21+ Implicit Class Detection

> 🇰🇷 [한국어 버전](/ko/adr/core/20-java21-implicit-class-detection)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2026-01-04 | @specvital | core  |

## Context

### Problem Statement

SpecVital Core's JUnit5 parser only traversed `class_declaration` nodes to find `@Test` annotated methods. Java 21 introduced implicit classes (JEP 445, finalized as JEP 463 in Java 22) where methods can exist at file level without explicit class wrappers.

### Background

Java 21+ Implicitly Declared Classes (JEP 445/JEP 463) allow:

- Source files without explicit class declarations
- Methods declared directly at file level
- Compiler auto-wraps as unnamed top-level class at compile time

This feature targets beginner-friendly Java programs but also enables cleaner test file organization:

**Traditional Java (supported):**

```java
public class HelloTests {
    @Test
    void testHello() {
        // ...
    }
}
```

**Java 21+ Implicit Class (previously unsupported):**

```java
// HelloTests.java - no class declaration
import org.junit.jupiter.api.Test;

@Test
void testHello() {
    // ...
}
```

| Pattern           | AST Structure                                          | Parser Status          |
| ----------------- | ------------------------------------------------------ | ---------------------- |
| Traditional class | `program` → `class_declaration` → `method_declaration` | Supported              |
| Implicit class    | `program` → `method_declaration`                       | Previously unsupported |

### Technical Analysis

The tree-sitter Java grammar correctly parses implicit classes, producing `method_declaration` nodes directly under `program` node. The limitation was purely in parser traversal logic, not grammar support.

```
Traditional:
program
└── class_declaration ("HelloTests")
    └── class_body
        └── method_declaration ("testHello")
            └── modifiers
                └── marker_annotation ("@Test")

Implicit Class:
program
└── method_declaration ("testHello")
    └── modifiers
        └── marker_annotation ("@Test")
```

### Requirements

1. Detect `@Test` methods under `program` node (file-level methods)
2. Create synthetic `TestSuite` with filename-derived name
3. Handle mixed scenarios (explicit class + file-level methods)
4. Maintain backward compatibility with traditional class patterns
5. No performance regression for traditional files

## Decision

**Extend JUnit5 parser to traverse `method_declaration` nodes under `program` node.**

The `parseTestClasses()` function is extended to:

1. Check for `method_declaration` children directly under `program` root node
2. Create synthetic `TestSuite` using filename (e.g., `HelloTests.java` → `HelloTests`)
3. Process annotations and extract test methods using existing infrastructure

### Implementation

```go
// pkg/parser/strategies/junit5/definition.go

func parseTestClasses(root *sitter.Node, source []byte, filename string) []domain.TestSuite {
    var suites []domain.TestSuite
    var implicitClassTests []domain.Test

    parser.WalkTree(root, func(node *sitter.Node) bool {
        switch node.Type() {
        case javaast.NodeClassDeclaration:
            // Traditional: class_declaration nodes
            if suite := parseTestClassWithDepth(node, source, filename, 0); suite != nil {
                suites = append(suites, *suite)
            }
            return false

        case javaast.NodeMethodDeclaration:
            // NEW: Handle Java 21+ implicit classes
            if node.Parent() != nil && node.Parent().Type() == "program" {
                if test := parseTestMethod(node, source, filename, domain.TestStatusActive, ""); test != nil {
                    implicitClassTests = append(implicitClassTests, *test)
                }
            }
        }
        return true
    })

    // Create synthetic suite for implicit class tests
    if len(implicitClassTests) > 0 {
        suites = append(suites, domain.TestSuite{
            Name:     getImplicitClassName(filename),
            Status:   domain.TestStatusActive,
            Location: parser.GetLocation(root, filename),
            Tests:    implicitClassTests,
        })
    }

    return suites
}
```

### Suite Naming Strategy

| Filename                              | Synthetic Suite Name |
| ------------------------------------- | -------------------- |
| `HelloTests.java`                     | `HelloTests`         |
| `UserServiceTest.java`                | `UserServiceTest`    |
| `src/test/java/IntegrationTests.java` | `IntegrationTests`   |

This matches the compiler's implicit class naming behavior.

## Options Considered

### Option A: Extend Existing Parser (Selected)

Add program-level method traversal to existing `parseTestClasses()` function.

**Pros:**

- Minimal code change (~50 lines in single file)
- Reuses existing annotation parsing and method extraction
- No performance impact for traditional files
- Consistent behavior across all JUnit5 patterns
- Follows Shared Parser Modules pattern (ADR-08)

**Cons:**

- Synthetic suite naming may differ from user expectations
- Mixed file handling (explicit + implicit) adds edge case complexity

### Option B: Separate Implicit Class Parser

Create independent `implicit_class_parser.go` module.

**Pros:**

- Clean separation of concerns
- Independent evolution of implicit class handling
- No regression risk to traditional parsing

**Cons:**

- ~80% code duplication with existing parser
- Violates DRY principle
- Bug fixes must be applied to both code paths
- Increases maintenance burden
- Against established patterns (ADR-08)

### Option C: No Support (Require Explicit Classes)

Document limitation and require users to wrap tests in explicit classes.

**Pros:**

- Zero implementation effort
- No code changes

**Cons:**

- Ignores valid Java 21+ language feature
- User friction for modern codebases
- Competitive disadvantage vs tools supporting Java 21+
- Parser appears outdated

### Option D: Two-Pass File Detection

First pass detects file type, second pass applies specialized parsing.

**Pros:**

- Explicit file type determination
- Could enable type-specific optimizations

**Cons:**

- 2x parsing overhead
- Unnecessary complexity
- Over-engineering for straightforward problem

## Consequences

### Positive

1. **Java 21+ Compatibility**
   - Full support for implicit class patterns
   - Parser stays current with language evolution
   - No user friction for modern Java codebases

2. **Minimal Implementation Risk**
   - Single function modification
   - Existing test infrastructure reused
   - Traditional file handling unchanged

3. **Architectural Consistency**
   - Follows Shared Parser Modules pattern (ADR-08)
   - Single code path for JUnit5 test extraction
   - Bug fixes automatically apply to both patterns

4. **Intuitive Behavior**
   - Filename-based suite naming matches compiler behavior
   - Users can predict output from file name
   - Consistent with mental model of "file = test suite"

### Negative

1. **Synthetic Suite Naming**
   - Suite name derived from filename, not explicit declaration
   - Mitigation: Matches Java compiler's implicit class naming; intuitive for users

2. **Mixed File Edge Case**
   - Files with both explicit class and file-level methods require careful handling
   - Mitigation: Explicit classes processed first; file-level methods grouped separately; rare pattern in practice

3. **Java Version Detection Absence**
   - Parser does not verify Java version compatibility
   - Mitigation: Tree-sitter parses syntax regardless of version; runtime validation is user's responsibility

## References

- [Issue #101: junit5 - add Java 21+ implicit class test detection](https://github.com/specvital/core/issues/101)
- [Commit d7c1218: feat(junit5): add Java 21+ implicit class test detection](https://github.com/specvital/core/commit/d7c1218)
- [JEP 445: Unnamed Classes and Instance Main Methods](https://openjdk.org/jeps/445)
- [JEP 463: Implicitly Declared Classes and Instance Main Methods](https://openjdk.org/jeps/463)
- [ADR-03: Tree-sitter AST Parsing Engine](/en/adr/core/03-tree-sitter-ast-parsing-engine)
- [ADR-08: Shared Parser Modules](/en/adr/core/08-shared-parser-modules)
