---
title: Core Engine
description: Tree-sitter based multi-language test parser library specification
---

# Core Engine (Test Parser)

> 🇰🇷 [한국어 버전](/ko/prd/02-core-engine.md)

> Tree-sitter based multi-language test parser library

## Core Responsibilities

- Multi-framework test support
- Accurate parsing via Tree-sitter AST
- Available as Go library / CLI / Docker

## Domain Model

```
Inventory
└── TestFile[]
    ├── framework (jest, pytest, junit, ...)
    ├── language
    ├── path
    └── TestSuite[]
        └── Test[]
            ├── name
            ├── location (file, line)
            └── status (active, skipped, todo, ...)
```

## Source Abstraction

| Type        | Purpose                    |
| ----------- | -------------------------- |
| LocalSource | Local filesystem           |
| GitSource   | GitHub URL → shallow clone |

## Performance Optimization

- Parser pooling (reuse)
- Query caching
- Parallel file parsing

> See core repository for supported frameworks list
