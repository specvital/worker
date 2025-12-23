---
title: Web Platform
description: Web dashboard and REST API specification for Specvital
---

# Web Platform

> 🇰🇷 [한국어 버전](/ko/prd/03-web-platform.md)

> Web dashboard and REST API

## Components

| Component | Role            |
| --------- | --------------- |
| Backend   | REST API, OAuth |
| Frontend  | Dashboard SPA   |

## Backend API Areas

- **Authentication**: GitHub OAuth
- **Codebases**: CRUD
- **Analysis**: Request/result retrieval

## Frontend Main Screens

| Screen    | Function         |
| --------- | ---------------- |
| Home      | GitHub URL input |
| Dashboard | Codebase list    |
| Detail    | Test tree view   |

## Authentication Flow

```
GitHub OAuth → JWT issuance → Cookie storage
```

> See OpenAPI spec or web repository for API details
