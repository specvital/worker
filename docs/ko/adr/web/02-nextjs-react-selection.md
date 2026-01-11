---
title: Next.js 16 + React 19 선택
description: BFF 아키텍처를 위한 Next.js 16과 React 19 프론트엔드 프레임워크 선택에 관한 ADR
---

# ADR-02: Next.js 16 + React 19 선택

> [English Version](/en/adr/web/02-nextjs-react-selection.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2025-12-04 | @KubrickCode | web    |

## 배경

### 프레임워크 선택 문제

웹 플랫폼은 BFF(Backend-for-Frontend) 아키텍처를 위한 프론트엔드 프레임워크 필요. 핵심 요구사항:

1. **SSR/SSG 지원**: 성능과 SEO를 위한 서버사이드 렌더링
2. **React 생태계**: 기존 컴포넌트 라이브러리 및 개발자 전문성 활용
3. **BFF 패턴**: Go 백엔드가 비즈니스 로직을 처리하는 얇은 프레젠테이션 레이어
4. **i18n**: 한국어 및 영어 지원
5. **실시간 업데이트**: 상태 추적이 포함된 폴링 기반 대시보드

### 기존 아키텍처 제약사항

- **Go 백엔드**: OpenAPI 명세가 있는 REST API
- **타입 생성**: `openapi.yaml` → `openapi-typescript`를 통한 TypeScript 타입
- **배포**: Vercel (프론트엔드) + Railway (백엔드) + Neon (PostgreSQL)
- **인증**: JWT 토큰을 사용한 GitHub OAuth

### 평가 후보

1. **Next.js 16 + React 19**: App Router, Server Components, Turbopack
2. **Remix**: React Router v7, 서버 우선 아키텍처
3. **SvelteKit**: Svelte 5 runes, 가장 작은 번들 크기
4. **Astro**: Islands 아키텍처, 콘텐츠 중심
5. **Nuxt 4**: Vue 기반, Nitro 엔진

## 결정

**BFF 아키텍처를 위한 프론트엔드 프레임워크로 Next.js 16과 React 19 채택.**

핵심 원칙:

1. **Server Components 우선**: 기본적으로 Server Components 사용; Client Components는 리프 노드에서만
2. **명시적 렌더링 전략**: 모든 페이지에서 `force-static`, `force-dynamic` 또는 `revalidate` 선언
3. **직접 데이터베이스 접근 금지**: 모든 데이터 작업은 Go 백엔드 API를 통해
4. **타입 안전성**: OpenAPI → TypeScript 생성 체인

## 검토된 옵션

### 옵션 A: Next.js 16 + React 19 (선택됨)

**작동 방식:**

- React Server Components가 포함된 App Router
- 개발용 Turbopack (2-5배 빠른 빌드)
- 뮤테이션용 Server Actions, 웹훅용 API 라우트만
- 데이터 스트리밍을 위한 React 19 `use()` 훅

**장점:**

- **시장 지배력**: 신규 React 앱의 78%가 Next.js 사용; 프론트엔드 시장 점유율 42%
- **React 19 기능**: `use()` 훅, View Transitions, React Compiler 최적화
- **생태계 성숙도**: TanStack Query, shadcn/ui, next-intl, next-themes 모두 최적화됨
- **Vercel 시너지**: 제로 설정 배포, Edge Functions, 프리뷰 배포
- **인재 풀**: 가장 큰 개발자 커뮤니티; 채용 이점

**단점:**

- Vercel 배포 최적화 편향
- 단순한 애플리케이션에는 과도한 복잡성
- Svelte 대안보다 큰 번들 크기

### 옵션 B: Remix

**작동 방식:**

- Server Components(프리뷰)가 포함된 React Router v7
- 데이터 페칭을 위한 Loader/Action 패턴
- 점진적 향상 중심

**평가:**

- RSC 지원이 프리뷰 단계만 (2025년 7월)
- Next.js보다 작은 생태계
- next-intl 대비 제한된 i18n 도구
- **기각**: RSC 불안정성; 생태계 격차

### 옵션 C: SvelteKit

**작동 방식:**

- runes 반응성이 있는 Svelte 5
- React 44 KB 대비 1.6 KB 런타임
- Vercel 어댑터로 네이티브 SSR

**평가:**

- 72.8% 개발자 만족도 (최고 평점)
- React 대 Svelte 채용 비율 122:1 불리
- 전체 코드베이스 재작성 필요 (~50%)
- 다른 컴포넌트 패러다임 (SFCs vs JSX)
- **기각**: 마이그레이션 비용; 채용 어려움

### 옵션 D: Astro

**작동 방식:**

- 부분 하이드레이션을 위한 Islands 아키텍처
- 다중 프레임워크 지원 (React, Vue, Svelte)
- 콘텐츠 우선 정적 생성

**평가:**

- 블로그 및 마케팅 사이트에 탁월
- 인터랙티브 대시보드에 부적합
- 실시간 폴링 처리 어려움
- 내장 전역 상태 관리 없음
- **기각**: 대시보드 UX와 아키텍처 불일치

### 옵션 E: Nuxt 4

**작동 방식:**

- Vue 3 Composition API
- 다중 런타임 지원 Nitro 엔진
- NuxtLabs가 Vercel에 인수됨 (2025년 7월)

**평가:**

- SSR/SSG/ISR에서 Next.js와 기능 동등
- Vue 기반; 전체 재작성 필요 (~50%)
- 다른 템플릿 문법과 패러다임
- **기각**: 마이그레이션 비용; 패러다임 전환

## 구현 세부사항

### 채택된 프레임워크 기능

| 기능             | 구현                                            |
| ---------------- | ----------------------------------------------- |
| App Router       | 레이아웃이 있는 `app/[locale]/` 구조            |
| React 19 `use()` | Server에서 Client Components로 Promise 스트리밍 |
| next-intl        | `/en`, `/ko` 접두사로 URL 기반 i18n             |
| next-themes      | 시스템 선호도 감지 + 수동 토글                  |
| TanStack Query   | 폴링 기반 분석 상태, 커서 페이지네이션          |
| shadcn/ui        | Radix 기반 접근성 컴포넌트                      |
| nuqs             | 타입 안전 URL 쿼리 상태 관리                    |

### BFF 아키텍처 구현

```
브라우저 ↔ Next.js 서버 (Vercel) ↔ Go 백엔드 (Railway) ↔ PostgreSQL (Neon)
              │
              └─→ GitHub API (OAuth 토큰)
```

**적용된 경계:**

- Next.js: SSR/SSG, API 집계, 세션 관리, 캐싱
- Go 백엔드: 비즈니스 로직, 데이터베이스 작업, 외부 API 호출

### 컴포넌트 전략

| 유형             | 사용 시점                          | 예시                         |
| ---------------- | ---------------------------------- | ---------------------------- |
| Server Component | 데이터 페칭, 인터랙티브 없음       | 페이지 레이아웃, 데이터 표시 |
| Client Component | useState, useEffect, 이벤트 핸들러 | 폼, 모달, 토글               |

```tsx
// Server Component (기본)
export default async function Page() {
  const data = await fetchData();
  return <Display data={data} />;
}

// Client Component (명시적)
("use client");
export function InteractiveForm() {
  const [state, setState] = useState();
}
```

### 렌더링 전략

```typescript
// 모든 페이지는 반드시 하나를 선언:
export const dynamic = "force-static"; // SSG
export const dynamic = "force-dynamic"; // SSR
export const revalidate = 3600; // ISR (초)
```

## 결과

### 긍정적

**개발 속도:**

- 제로 설정 Vercel 배포
- Turbopack이 개발 서버 시작을 2-5배 단축
- 광범위한 사전 구축 컴포넌트 생태계 (shadcn/ui, Radix)

**성능:**

- React Compiler 자동 메모이제이션 (25-40% 적은 리렌더링)
- React 19 `use()` 훅으로 스트리밍 SSR
- 글로벌 대시보드 성능을 위한 Edge 배포

**유지보수성:**

- 가장 큰 개발자 인재 풀로 채용 마찰 감소
- 광범위한 문서화 및 커뮤니티 지원
- codemods를 통한 명확한 업그레이드 경로

### 부정적

**Vercel 결합:**

- 심층 플랫폼 최적화로 전환 마찰 가능성
- **완화**: Railway, AWS, Cloudflare에서 표준 Node.js 배포 가능

**번들 크기:**

- SvelteKit 대안보다 큼 (~44 KB React 런타임)
- **완화**: 대시보드 애플리케이션에 허용 가능; 생태계 이점으로 상쇄

**복잡성:**

- Server Components 멘탈 모델은 팀 교육 필요
- **완화**: BFF 패턴이 프론트엔드 복잡성 격리; 명시적 렌더링 선언

### 마이그레이션 경로

향후 마이그레이션이 필요할 경우:

| 대상      | 노력  | 참고                                    |
| --------- | ----- | --------------------------------------- |
| Remix     | 중간  | 동일한 React 생태계; 라우트 컨벤션 다름 |
| Astro     | 중-상 | 정적 우선; 대시보드 아키텍처 불일치     |
| SvelteKit | 높음  | 전체 재작성 필요                        |

BFF 아키텍처는 프론트엔드 마이그레이션 시 백엔드 변경 없음 보장.

## 참고자료

### 내부

- [ADR-01: 백엔드 언어로 Go 선택](/ko/adr/web/01-go-backend-language.md)
- [PRD: 기술 스택](/ko/prd/06-tech-stack.md)
- [Tech Radar](/ko/tech-radar.md)

### 외부

- [Next.js 16 릴리스 블로그](https://nextjs.org/blog/next-16)
- [React 19 문서](https://react.dev/)
- [Next.js App Router](https://nextjs.org/docs/app)
- [next-intl 문서](https://next-intl.dev/)
- [TanStack Query](https://tanstack.com/query)
- [shadcn/ui](https://ui.shadcn.com/)
