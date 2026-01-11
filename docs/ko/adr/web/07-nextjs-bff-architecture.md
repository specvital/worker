---
title: Next.js BFF 아키텍처
description: 모든 비즈니스 로직은 Go 백엔드에 두고 Next.js를 얇은 BFF 레이어로 사용하는 결정
---

# ADR-07: Next.js BFF 아키텍처

> [English Version](/en/adr/web/07-nextjs-bff-architecture.md)

| 일자       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-01-03 | @KubrickCode | web        |

## 컨텍스트

### 프론트엔드-백엔드 통신 문제

현대 웹 애플리케이션은 근본적인 아키텍처 질문에 직면: 프론트엔드가 백엔드 서비스와 어떻게 통신해야 하는가?

SpecVital 생태계 구성:

- **Go 백엔드**: 비즈니스 로직, 데이터베이스 접근, GitHub API 연동, 분석 오케스트레이션
- **프론트엔드**: 저장소 분석 시각화를 위한 React 기반 UI
- **외부 서비스**: GitHub OAuth, GitHub API, River 큐 (Worker와 공유)

### 클라이언트-백엔드 직접 통신의 문제점

| 문제               | 영향                                       |
| ------------------ | ------------------------------------------ |
| 보안 노출          | 브라우저 DevTools에서 토큰/API 키 노출     |
| 다중 네트워크 요청 | 여러 엔드포인트 집계 시 N+1 문제           |
| SSR/SSG 불가       | SEO 제한, 느린 초기 페이지 로드            |
| CORS 복잡성        | 프론트엔드-백엔드 도메인 간 교차 출처 문제 |
| 백엔드 API 결합    | 프론트엔드가 백엔드 API 구조에 강하게 결합 |

### 관련 아키텍처 결정

- [ADR-01: Go 백엔드 언어](/ko/adr/web/01-go-backend-language.md) - 백엔드 언어 선택
- [ADR-02: Next.js + React 선택](/ko/adr/web/02-nextjs-react-selection.md) - 프론트엔드 프레임워크 선택
- [ADR-03: API와 Worker 서비스 분리](/ko/adr/03-api-worker-service-separation.md) - 서비스 경계

## 결정

**모든 비즈니스 로직은 Go 백엔드에 유지하면서 Next.js를 얇은 BFF(Backend-for-Frontend) 레이어로 채택.**

### 아키텍처

```
브라우저 <-> Next.js 서버 (BFF) <-> Go 백엔드 API <-> 데이터베이스
                                          |
                                          v
                                    Worker 서비스
```

### 핵심 원칙

1. **Next.js는 번역 레이어**: 클라이언트 특화 로직만 (데이터 형변환, SSR, 캐싱)
2. **BFF에 비즈니스 로직 금지**: 모든 도메인 로직은 Go 백엔드에 위치
3. **데이터베이스 접근 금지**: Next.js는 PostgreSQL에 직접 접근하지 않음
4. **API 프록시 패턴**: 프론트엔드는 `/api/*`를 호출하고 Next.js가 Go 백엔드로 재작성

### BFF 책임 범위

| 허용                         | 금지                         |
| ---------------------------- | ---------------------------- |
| 서버 사이드 렌더링 (SSR/SSG) | 비즈니스 로직 구현           |
| API 요청 집계                | 직접 데이터베이스 쿼리       |
| 응답 캐싱                    | 입력 정제 이상의 데이터 검증 |
| 세션/쿠키 관리               | 도메인 엔티티 정의           |
| 데이터 형태 변환             | 큐 작업 생성                 |

## 고려한 옵션

### 옵션 A: Next.js를 얇은 BFF로 사용 (선택됨)

**동작 방식:**

- Next.js Server Component가 Go 백엔드에서 데이터 페치
- `next.config.ts` rewrites를 통한 API 프록시 (`/api/*` -> 백엔드)
- 뮤테이션을 위한 Server Actions는 백엔드 엔드포인트 호출
- Route Handlers는 외부 웹훅(OAuth 콜백)에만 사용

**장점:**

- **보안**: 토큰이 httpOnly 쿠키로 서버 사이드에 유지
- **성능**: SSR로 클라이언트 사이드 로딩 상태 제거, TTFB 감소
- **집계**: 여러 백엔드 호출을 단일 프론트엔드 요청으로 결합
- **캐싱**: Next.js 캐시 지시어로 세밀한 제어
- **타입 안전성**: BFF와 프론트엔드 간 OpenAPI 생성 타입 공유

**단점:**

- 추가 네트워크 홉 (브라우저 -> Next.js -> Go)
- 인프라 복잡성 (배포할 서비스 2개)
- 잠재적 단일 장애점
- 팀이 TypeScript와 Go 모두 이해해야 함

### 옵션 B: 직접 API 호출하는 SPA

**동작 방식:**

- React SPA가 CORS를 통해 Go 백엔드 직접 호출
- 모든 렌더링이 클라이언트 사이드에서 발생
- 토큰은 localStorage 또는 쿠키에 저장

**장점:**

- 단순한 아키텍처 (서비스 하나 적음)
- 추가 네트워크 홉 없음
- 낮은 인프라 비용

**단점:**

- **보안 위험**: 브라우저에 토큰 노출 (XSS 취약점)
- **SSR 불가**: 열악한 SEO, 느린 체감 성능
- **CORS 복잡성**: 허용 출처 설정 필요
- **N+1 요청**: 집계된 뷰를 위해 클라이언트가 다중 호출
- **로딩 상태**: 사용자가 콘텐츠 대신 로딩 스피너 봄

### 옵션 C: API Gateway + SPA

**동작 방식:**

- API Gateway(Kong, AWS API Gateway)가 라우팅과 인증 처리
- SPA가 게이트웨이를 통해 통신
- 게이트웨이가 Go 백엔드로 프록시

**장점:**

- 중앙화된 인증 및 속도 제한
- 프로토콜 변환 가능
- 게이트웨이 레벨에서 모니터링 및 로깅

**단점:**

- SSR 기능 없음
- UI를 위한 데이터 집계/변환 불가
- 일반적인 API 표면, 프론트엔드에 최적화되지 않음
- 관리할 추가 인프라 컴포넌트

### 옵션 D: 풀스택 Next.js (BFF에 비즈니스 로직)

**동작 방식:**

- Next.js가 UI와 비즈니스 로직 모두 처리
- Prisma나 Drizzle을 통한 직접 데이터베이스 접근
- 모든 뮤테이션에 Server Actions 사용

**장점:**

- 단일 코드베이스, 단일 배포
- 단순한 멘탈 모델
- 서비스 간 통신 없음

**단점:**

- **기존 아키텍처 위반**: Go 기반 생태계와 충돌
- **Core 라이브러리 공유 불가**: 파서, 암호화 유틸리티가 Go에 있음
- **큐 비호환성**: River는 PostgreSQL 기반 Go 라이브러리
- **로직 중복**: 암호화, 검증을 TypeScript로 재구현 필요
- 백엔드 독립 확장 어려움

## 구현

### API 프록시 설정

```typescript
// next.config.ts
const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

const nextConfig: NextConfig = {
  rewrites: async () => [
    {
      destination: `${API_URL}/api/:path*`,
      source: "/api/:path*",
    },
  ],
};
```

### Server Component 데이터 페칭

```typescript
// Server Component - 프록시를 통해 Go 백엔드에서 페치
export default async function Page() {
  const response = await fetch('/api/analyze/owner/repo', {
    cache: 'no-store'
  });
  const data = await response.json();
  return <AnalysisView data={data} />;
}
```

### Client Component 패턴

```typescript
// Client Component - BFF 프록시된 API 사용
'use client';

export function AnalysisContent({ dataPromise }) {
  const data = use(dataPromise);  // React 19 use() hook
  return <Display data={data} />;
}
```

### 폴더 구조

```
features/[name]/
├── components/     # UI 컴포넌트
├── hooks/          # React hooks (TanStack Query)
├── api/            # API 클라이언트 함수 (/api/* 프록시 호출)
└── index.ts        # 배럴 내보내기
```

## 결과

### 긍정적

**보안:**

- 토큰과 시크릿이 브라우저에 도달하지 않음
- 세션 관리를 위한 httpOnly 쿠키
- 서버 레벨에서 CSP 헤더 적용
- 공격 표면 감소 (단일 신뢰 애플리케이션)

**성능:**

- 서버 사이드 렌더링으로 클라이언트 로딩 상태 제거
- 데이터 집계로 네트워크 왕복 감소
- 재검증 제어가 있는 내장 캐싱
- React 19 Suspense로 스트리밍

**개발자 경험:**

- 프론트엔드와 BFF를 위한 통합 코드베이스
- OpenAPI 생성으로 타입 안전성
- 통합 로깅으로 단순화된 디버깅
- 일관된 에러 처리 패턴

**확장성:**

- 프론트엔드와 백엔드가 독립적으로 확장
- 해당되는 경우 CDN 친화적 정적 페이지
- 일반적인 요청에 대한 엣지 캐싱

### 부정적

**추가 네트워크 홉:**

- 모든 요청에 ~1-5ms 지연 추가
- **완화**: Server Components가 총 왕복 감소; 캐싱이 백엔드 호출 최소화

**인프라 복잡성:**

- 배포하고 모니터링할 서비스 2개 (Next.js + Go)
- **완화**: 컨테이너화된 배포, 통합 CI/CD, 공유 모니터링

**단일 장애점:**

- BFF 불가용 시 모든 프론트엔드 접근 차단
- **완화**: 헬스 체크, 다중 레플리카, 서킷 브레이커

**학습 곡선:**

- 팀이 Next.js 패턴과 Go 백엔드 모두 이해해야 함
- **완화**: 명확한 문서화 (CLAUDE.md, nextjs.md), 코드 리뷰

### 적용 규칙

프로젝트의 `nextjs.md`에서:

| 규칙                   | 적용 방법                        |
| ---------------------- | -------------------------------- |
| 데이터베이스 접근 금지 | 코드 리뷰, ORM 의존성 없음       |
| 비즈니스 로직 금지     | 핸들러 함수는 백엔드만 호출      |
| 명시적 캐시 선언       | ESLint 규칙, PR 체크리스트       |
| Server Components 기본 | `'use client'`는 리프 노드에서만 |

## 참고 자료

- [Next.js BFF 가이드](https://nextjs.org/docs/app/guides/backend-for-frontend)
- [Sam Newman - BFF 패턴](https://samnewman.io/patterns/architectural/bff/)
- [Microsoft Azure - BFF 패턴](https://learn.microsoft.com/en-us/azure/architecture/patterns/backends-for-frontends)
- [ADR-01: Go 백엔드 언어](/ko/adr/web/01-go-backend-language.md)
- [ADR-02: Next.js + React 선택](/ko/adr/web/02-nextjs-react-selection.md)
- [ADR-03: API와 Worker 서비스 분리](/ko/adr/03-api-worker-service-separation.md)
