---
title: 기능 기반 모듈 구조
description: 백엔드와 프론트엔드 코드베이스에 기능 기반 수직 슬라이스 아키텍처를 도입한 ADR
---

# ADR-11: 기능 기반 모듈 구조

> [English Version](/en/adr/web/11-feature-based-module-organization.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2025-01-03 | @KubrickCode | web    |

## 컨텍스트

### 초기 구조의 문제점

백엔드와 프론트엔드 모두 초기에는 평면적인 계층 기반 구조:

**백엔드 (변경 전):**

```
src/backend/
├── analyzer/
│   ├── handler.go
│   ├── service.go
│   └── types.go
├── github/
└── health/
```

**프론트엔드 (변경 전):**

```
src/frontend/
├── components/       # 16개 파일이 평면적으로 배치, 도메인 혼재
└── types/
```

**확인된 문제점:**

- **낮은 응집도**: 관련 없는 파일들이 기술 계층별로 그룹화됨
- **높은 결합도**: 하나의 기능 수정 시 여러 디렉토리에 걸쳐 변경 필요
- **불명확한 경계**: 어떤 코드가 어떤 도메인에 속하는지 파악하기 어려움
- **확장성 부족**: 기능 추가 시 관련 없는 여러 디렉토리를 수정해야 함
- **탐색 오버헤드**: 개발자가 멀리 떨어진 폴더 사이를 이동해야 함

### 클린 아키텍처와의 정합성

이 결정은 [ADR-08: 클린 아키텍처 패턴](/ko/adr/web/08-clean-architecture-pattern.md)을 보완. 클린 아키텍처가 모듈 내부의 계층 구조를 정의하고, 기능 기반 구조는 최상위 수준에서 모듈 그룹화 방식을 정의.

## 결정

**백엔드와 프론트엔드 모두에 기능 기반 모듈 구조를 도입하고, 각 모듈 내부에는 클린 아키텍처 계층 적용.**

### 백엔드 구조: `modules/{module}/`

각 도메인 모듈은 자체 클린 아키텍처 계층 포함:

```
modules/{module}/
├── domain/
│   ├── entity/      # 비즈니스 엔티티
│   └── port/        # 인터페이스 정의
├── usecase/         # 비즈니스 로직
├── adapter/         # 외부 구현체
└── handler/         # HTTP 진입점
```

**현재 모듈:** `analyzer`, `auth`, `github`, `github-app`, `user`

### 프론트엔드 구조: `features/{feature}/`

각 기능은 자체 내부 구조를 가진 독립적인 단위:

```
features/{feature}/
├── components/      # UI 컴포넌트
├── hooks/           # React hooks
├── api/             # API 호출
├── types/           # TypeScript 타입
└── index.ts         # 배럴 export (공개 API)
```

**현재 기능:** `analysis`, `auth`, `dashboard`, `home`

### 공유 코드 구조

여러 모듈에 걸쳐 사용되는 코드는 전용 공유 디렉토리에 배치:

| 위치                      | 용도                                 |
| ------------------------- | ------------------------------------ |
| 백엔드: `common/`         | 미들웨어, 헬스 체크, 공유 클라이언트 |
| 프론트엔드: `components/` | 레이아웃, 테마, 피드백 컴포넌트      |
| 프론트엔드: `lib/`        | API 클라이언트, 유틸리티, 스타일     |

## 검토한 대안

### 대안 A: 기능 기반 (수직 슬라이스) 구조 (선택됨)

**작동 방식:**

- 최상위 디렉토리가 비즈니스 도메인을 나타냄
- 각 도메인 내부에 모든 기술 계층을 포함
- "Screaming Architecture" 원칙을 따름

**장점:**

- **높은 응집도**: 관련 코드가 함께 위치함
- **최소 결합도**: 모듈이 대체로 독립적
- **쉬운 삭제**: 하나의 디렉토리를 삭제하여 기능 제거
- **명확한 소유권**: 팀이 전체 기능을 소유할 수 있음
- **자연스러운 마이크로서비스 경계**: 분리가 간단함
- **AI 친화적**: 제한된 컨텍스트가 LLM 컨텍스트 윈도우에 맞음

**단점:**

- 공유 코드 배치에 대한 명시적 결정 필요
- 초기 설정 시 더 많은 구조화 필요

### 대안 B: 계층 기반 (수평) 구조

**작동 방식:**

- 최상위 디렉토리가 기술 계층을 나타냄
- 모든 컨트롤러는 한 폴더에, 모든 서비스는 다른 폴더에

**예시:**

```
src/backend/
├── controllers/
│   ├── analyzer.go
│   ├── auth.go
│   └── user.go
├── services/
│   ├── analyzer.go
│   ├── auth.go
│   └── user.go
└── repositories/
    └── ...
```

**장점:**

- 단순한 초기 구조
- 명확한 계층 분리
- 전통적인 프레임워크에서 흔함

**단점:**

- **낮은 응집도**: 관련 없는 파일이 유형별로 그룹화됨
- **높은 결합도**: 기능 변경이 여러 디렉토리에 걸쳐 발생
- **어려운 분리**: 마이크로서비스 마이그레이션에 대규모 재구성 필요
- **탐색 오버헤드**: 개발자가 멀리 떨어진 폴더 사이를 이동해야 함
- **넓은 컨텍스트**: AI 도구가 분산된 코드를 처리하기 어려움

### 대안 C: 평면 컴포넌트 구조

**작동 방식:**

- 모든 컴포넌트가 단일 디렉토리에 위치
- 최소한의 폴더 계층

**장점:**

- 조직화 오버헤드 없음
- 소규모 프로젝트에 적합

**단점:**

- 규모가 커지면 관리 불가능
- 명확한 도메인 경계 없음
- 파일이 많아지면 탐색 어려움

## 구현

### 배럴 Export 패턴

각 기능은 `index.ts`를 통해 공개 API를 노출하여 `@/features/analysis`와 같은 깔끔한 import 가능.

### 모듈 간 의존성

| 스택       | 접근 방식                        |
| ---------- | -------------------------------- |
| 백엔드     | 다른 모듈의 port 인터페이스 사용 |
| 프론트엔드 | 배럴 export를 통해 import        |

## 결과

### 긍정적

**개발자 경험:**

- 명확한 멘탈 모델: "사용자 북마크 로직이 어디에?" → `modules/user/usecase/bookmark/`
- 빠른 탐색: 모든 관련 코드가 한 곳에 위치
- 쉬운 온보딩: 신규 개발자가 단일 기능에 집중 가능

**코드 품질:**

- 모듈 내 높은 응집도
- 모듈 간 낮은 결합도
- 코드 리뷰를 위한 자연스러운 경계

**확장성:**

- 기능 추가: 표준 구조로 새 디렉토리 생성
- 팀 확장: 모듈별 팀 소유권 할당
- 마이크로서비스 분리: 모듈 경계가 이미 정의됨

**AI 지원 개발:**

- 각 기능이 LLM 컨텍스트 윈도우에 맞음
- AI 에이전트가 작업할 명확한 경계
- 파일 간 의존성 스캔 감소

### 부정적

**공유 코드 결정:**

- 코드가 기능에 속하는지 공유 위치에 속하는지 결정 필요
- **완화**: 기본적으로 기능별로 배치, 3회 이상 재사용 시 추출

**초기 학습 곡선:**

- 신규 팀원이 모듈 구조를 이해해야 함
- **완화**: CLAUDE.md에 패턴 문서화; 모듈 간 일관된 구조

**잠재적 중복:**

- 유사한 패턴이 여러 모듈에 존재할 수 있음
- **완화**: 중복 발견 시 `common/` 또는 `lib/`로 추출

## 참고 자료

- [Screaming Architecture - Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2011/09/30/Screaming-Architecture.html)
- [Vertical Slice Architecture - Jimmy Bogard](https://www.jimmybogard.com/vertical-slice-architecture/)
- [Colocation - Kent C. Dodds](https://kentcdodds.com/blog/colocation)
- [Practical DDD in Golang: Module](https://www.ompluscator.com/article/golang/practical-ddd-module/)
- [React Folder Structure - Robin Wieruch](https://www.robinwieruch.de/react-folder-structure/)
- [NestJS Modules Documentation](https://docs.nestjs.com/modules)
- [ADR-08: 클린 아키텍처 패턴](/ko/adr/web/08-clean-architecture-pattern.md)
