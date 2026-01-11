---
title: Core ADR
description: Architecture Decision Records for the Core parser library
---

# Core Repository ADR

> 🇰🇷 [한국어 버전](/ko/adr/core/)

Architecture Decision Records for the [specvital/core](https://github.com/specvital/core) repository.

## ADR Index

| #   | Title                                                                                                    | Date       |
| --- | -------------------------------------------------------------------------------------------------------- | ---------- |
| 01  | [Core Library Separation](./01-core-library-separation.md)                                               | 2024-12-17 |
| 02  | [Dynamic Test Counting Policy](./02-dynamic-test-counting-policy.md)                                     | 2024-12-22 |
| 03  | [Tree-sitter as AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md)                              | 2024-12-23 |
| 04  | [Early-Return Framework Detection](./04-early-return-framework-detection.md)                             | 2024-12-23 |
| 05  | [Parser Pooling Disabled](./05-parser-pooling-disabled.md)                                               | 2024-12-23 |
| 06  | [Unified Framework Definition](./06-unified-framework-definition.md)                                     | 2024-12-23 |
| 07  | [Source Abstraction Interface](./07-source-abstraction-interface.md)                                     | 2024-12-23 |
| 08  | [Shared Parser Modules](./08-shared-parser-modules.md)                                                   | 2024-12-23 |
| 09  | [Config Scope Resolution](./09-config-scope-resolution.md)                                               | 2024-12-23 |
| 10  | [Standard Go Project Layout](./10-standard-go-project-layout.md)                                         | 2024-12-23 |
| 11  | [Integration Testing with Golden Snapshots](./11-integration-testing-golden-snapshots.md)                | 2024-12-23 |
| 12  | [Parallel Scanning with Worker Pool](./12-parallel-scanning-worker-pool.md)                              | 2024-12-23 |
| 13  | [NaCl SecretBox Encryption](./13-nacl-secretbox-encryption.md)                                           | 2024-12-23 |
| 14  | [Indirect Import Alias Detection Unsupported](./14-indirect-import-unsupported.md)                       | 2025-12-29 |
| 15  | [C# Preprocessor Block Attribute Detection Limitation](./15-csharp-preprocessor-attribute-limitation.md) | 2026-01-04 |

## Related

- [All ADRs](/en/adr/)
- [Core PRD](/en/prd/02-core-engine.md)
