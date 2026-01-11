---
title: Glossary
description: Domain terminology used across the Specvital platform
---

# Glossary

> 🇰🇷 [한국어 버전](/ko/glossary.md)

Domain terminology used across the Specvital platform.

## Core Domain

| Term           | Definition                                                             |
| -------------- | ---------------------------------------------------------------------- |
| **Codebase**   | A GitHub repository registered for analysis (host + owner + name)      |
| **Analysis**   | A single parsing job for a codebase at a specific commit               |
| **Inventory**  | Complete collection of test files parsed from a codebase               |
| **Test Suite** | A group of related tests (e.g., `describe` block in Jest)              |
| **Test Case**  | An individual test (e.g., `it`/`test` block)                           |
| **Framework**  | Test library used (Jest, Vitest, Playwright, Go testing, pytest, etc.) |

## Test Status

| Term        | Definition                                      |
| ----------- | ----------------------------------------------- |
| **active**  | Normal test that will run                       |
| **skipped** | Test marked to be skipped (`.skip`, `t.Skip()`) |
| **todo**    | Placeholder test not yet implemented (`.todo`)  |
| **focused** | Test marked to run exclusively (`.only`)        |
| **xfail**   | Expected failure test (`@pytest.mark.xfail`)    |

## Analysis Status

| Term          | Definition                      |
| ------------- | ------------------------------- |
| **pending**   | Analysis queued but not started |
| **running**   | Analysis in progress            |
| **completed** | Analysis finished successfully  |
| **failed**    | Analysis encountered an error   |

## Architecture

| Term          | Definition                                                                |
| ------------- | ------------------------------------------------------------------------- |
| **Core**      | Test parser library (Go) - parses source code into test inventory         |
| **Worker**    | Worker repository - contains analyzer, scheduler, spec-generator binaries |
| **Web**       | Frontend (Next.js) + Backend API (Go Chi)                                 |
| **Infra**     | Database schema and local development infrastructure                      |
| **Analyzer**  | River-based process that consumes analysis tasks from PostgreSQL queue    |
| **Scheduler** | Cron-based process that enqueues periodic refresh tasks                   |

## Technical Terms

| Term            | Definition                                                      |
| --------------- | --------------------------------------------------------------- |
| **Tree-sitter** | Incremental parsing library used for AST-based code analysis    |
| **AST**         | Abstract Syntax Tree - structured representation of source code |
| **River**       | Go library for distributed task queue using PostgreSQL          |
| **sqlc**        | Generates type-safe Go code from SQL queries                    |
| **Atlas**       | Database schema migration tool                                  |

## Source Types

| Term            | Definition                               |
| --------------- | ---------------------------------------- |
| **LocalSource** | Parser reads from local filesystem       |
| **GitSource**   | Parser clones from remote Git repository |

## Related Documents

- [Architecture](./prd/01-architecture.md)
- [Core Engine](./prd/02-core-engine.md)
- [Database Design](./prd/05-database-design.md)
