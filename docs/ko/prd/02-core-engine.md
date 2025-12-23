---
title: 코어 엔진
description: Tree-sitter 기반 다중 언어 테스트 파서 라이브러리
---

# Core Engine (테스트 파서)

> 🇺🇸 [English Version](/en/prd/02-core-engine.md)

> Tree-sitter 기반 다중 언어 테스트 파서 라이브러리

## 핵심 역할

- 다중 테스트 프레임워크 지원
- Tree-sitter AST 기반 정확한 파싱
- Go 라이브러리 / CLI / Docker 형태 제공

## 도메인 모델

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

## Source 추상화

| 타입        | 용도                       |
| ----------- | -------------------------- |
| LocalSource | 로컬 파일시스템            |
| GitSource   | GitHub URL → shallow clone |

## 성능 최적화

- 파서 풀링 (재사용)
- 쿼리 캐싱
- 병렬 파일 파싱

> 지원 프레임워크 목록은 core 리포지토리 참조
