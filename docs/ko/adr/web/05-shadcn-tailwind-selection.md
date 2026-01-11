---
title: shadcn/ui + Tailwind CSS 선택
description: UI 컴포넌트 라이브러리로 shadcn/ui와 Tailwind CSS 선택에 관한 ADR
---

# ADR-05: shadcn/ui + Tailwind CSS 선택

> [English Version](/en/adr/web/05-shadcn-tailwind-selection.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2025-12-04 | @KubrickCode | web    |

## 배경

### UI 컴포넌트 라이브러리 선택 문제

웹 플랫폼은 접근성 있고, 일관되며, 유지보수 가능한 인터페이스를 구축하기 위한 UI 컴포넌트 라이브러리 필요. 핵심 요구사항:

1. **AI 기반 개발**: Claude, Cursor, v0 등 AI 코딩 워크플로우에 최적화
2. **React 19 호환성**: 최신 React 기능 완전 지원
3. **Server Components**: Next.js 16 App Router RSC 패턴과 원활한 연동
4. **접근성**: 기본 제공 WCAG 호환 컴포넌트
5. **커스터마이징**: 스타일링과 동작에 대한 완전한 제어
6. **다크 모드**: 시스템 선호도 감지 및 수동 토글

### 아키텍처 제약사항

- **BFF 패턴**: 얇은 프레젠테이션 레이어; UI 라이브러리가 백엔드 복잡성을 추가하면 안 됨
- **TypeScript**: 컴포넌트 props와 variants에 대한 강한 타이핑
- **Tailwind CSS**: 유틸리티 우선 스타일링 접근 방식으로 이미 선택됨
- **Vercel 배포**: Edge 최적화 번들 크기가 중요

### 평가 후보

1. **shadcn/ui + Tailwind CSS**: Radix UI 기반 copy-paste 컴포넌트
2. **MUI (Material UI)**: Google의 Material Design 구현체
3. **Chakra UI v3**: 시맨틱 토큰이 있는 제로 런타임 CSS-in-JS
4. **Ant Design v5**: 엔터프라이즈 중심 컴포넌트 라이브러리
5. **Headless UI**: Tailwind Labs의 스타일 없는 접근성 컴포넌트

## 결정

**AI 기반 개발 생산성 극대화를 위해 shadcn/ui와 Tailwind CSS를 AI 네이티브 UI 컴포넌트 라이브러리로 채택.**

핵심 원칙:

1. **AI-Ready 아키텍처**: LLM이 읽고, 이해하고, 개선할 수 있는 오픈 코드
2. **코드 소유권**: 프로젝트에 컴포넌트 복사; 외부 런타임 의존성 없음
3. **Radix 기반**: 검증된 접근성 프리미티브 활용
4. **CSS 변수 테마**: `globals.css`의 OKLCH 기반 디자인 토큰
5. **Server Component 우선**: RSC 패턴과 호환되는 컴포넌트

## 검토된 옵션

### 옵션 A: shadcn/ui + Tailwind CSS (선택됨)

**작동 방식:**

- CLI를 통해 프로젝트에 컴포넌트 복사 (`pnpm dlx shadcn@latest add [component]`)
- 접근성을 위해 Radix UI 프리미티브 기반
- Tailwind CSS 유틸리티 클래스로 스타일링
- CSS 변수와 `class-variance-authority` (cva)를 통한 테마

**장점:**

- **AI 네이티브 스택**: Vercel v0가 오직 shadcn/ui + Tailwind만 생성; AI 코딩 업계 표준
- **LLM 최적화 스타일링**: Tailwind 인라인 유틸리티 = 단일 파일에 완전한 컨텍스트; 숨겨진 CSS 캐스케이드 없음
- **완전한 코드 소유권**: 저장소에 컴포넌트 포함; AI가 자유롭게 읽고 이해하고 수정
- **Radix 접근성**: WAI-ARIA 호환, 키보드 네비게이션, 스크린 리더 테스트 완료
- **React 19 네이티브**: 퍼스트 클래스 지원; `data-slot` 속성 사용, deprecated API 없음
- **Tailwind v4 준비 완료**: CSS 변수, `@theme` 지시어, OKLCH 색상 지원
- **RSC 호환**: Server Component 패턴에서 컴포넌트 작동
- **제로 번들 오버헤드**: 라이브러리 런타임 없음; 사용하는 코드만 배포 (MUI 80+ KB 대비 2.3 KB 초기)

**단점:**

- 수동 업데이트 필요 (새 버전을 선택적으로 복사)
- 엔터프라이즈 라이브러리보다 사전 구축된 복잡한 컴포넌트 적음
- 팀이 Tailwind CSS 패러다임을 이해해야 함

### 옵션 B: MUI (Material UI v6)

**작동 방식:**

- Emotion CSS-in-JS와 함께 npm 의존성으로 설치
- 50개 이상의 사전 구축 Material Design 컴포넌트
- `createTheme()`으로 테마 객체 설정

**평가:**

- **번들 크기**: 80-90 KB gzipped 코어; 단일 Button이 91.7 KB 초기 JS 추가
- **RSC 비호환**: Emotion이 React Context 사용; 모든 컴포넌트가 Client Components여야 함
- **AI 비친화적**: CSS-in-JS는 AI가 별도의 런타임 컨텍스트 전반에서 스타일을 추적해야 함
- **Material Design 종속**: 벗어나기 어려운 고집스러운 미학
- **Barrel Import 문제**: 경로 임포트 없이 개발 성능 저하
- **기각**: AI 워크플로우 마찰; RSC 비호환성; 디자인 시스템 종속

### 옵션 C: Chakra UI v3

**작동 방식:**

- 제로 런타임 CSS-in-JS (v3에서 런타임에서 마이그레이션)
- 자동 다크/라이트 모드가 있는 시맨틱 토큰
- Panda CSS에서 영감받은 레시피 시스템

**평가:**

- **번들 크기**: ~50 KB gzipped (v2에서 개선)
- **v3 브레이킹 체인지**: v2에서 주요 마이그레이션; 생태계 아직 안정화 중
- **RSC**: 컴포넌트 임포트 가능하지만 Client Components로 하이드레이션
- **next-themes 필요**: 다크 모드가 내장에서 외부로 이동
- **기각**: 마이그레이션 불안정성; shadcn/ui보다 작은 생태계

### 옵션 D: Ant Design v5

**작동 방식:**

- 60개 이상의 엔터프라이즈 중심 컴포넌트
- `@ant-design/cssinjs`로 디자인 토큰
- 테마 및 i18n용 ConfigProvider

**평가:**

- **번들 크기**: 단일 컴포넌트에 126 KB+; ConfigProvider가 트리 쉐이킹 방지
- **엔터프라이즈 기능**: Pro 컴포넌트 (ProTable, ProForm) 사용 가능
- **내장 i18n**: 50개 이상 로케일 기본 제공
- **고집스러운 디자인**: 커스터마이징하기 어려운 강한 Ant 미학
- **RSC 비호환**: CSS-in-JS 아키텍처로 Client Components 필수
- **기각**: 거대한 번들; 제한된 커스터마이징; RSC 비호환성

### 옵션 E: Headless UI + Tailwind

**작동 방식:**

- Tailwind Labs의 16개 스타일 없는 접근성 컴포넌트
- Tailwind 유틸리티 클래스로 스타일링
- React와 Vue 프레임워크 지원

**평가:**

- **컴포넌트 격차**: Radix의 32개+ 대비 16개 컴포넌트만
- **누락**: Accordion, Context Menu, Toast, Slider, Tooltip, Progress
- **덜 발전됨**: Radix보다 포커스 트래핑과 스크롤 잠금이 덜 견고
- **기각**: 불충분한 컴포넌트 커버리지; 접근성에서 Radix 우위

## 구현 세부사항

### 컴포넌트 도입

| 컴포넌트                    | 목적               |
| --------------------------- | ------------------ |
| Button, Input               | 초기 폼 구현       |
| Card, Badge, Tooltip, Alert | 대시보드 UI 기반   |
| Dialog, Dropdown, Tabs      | 필터 및 네비게이션 |
| Scroll Area                 | 수평 스크롤 UI     |
| Sheet, Command              | 모바일 네비게이션  |

### 테마 시스템

- OKLCH 색상 공간으로 `globals.css`에 CSS 변수 정의
- 시스템 감지와 함께 `next-themes`로 다크 모드
- `class-variance-authority` (cva)로 타입 안전 variants

**색상 시스템 발전:**

| 변경 사항                                                |
| -------------------------------------------------------- |
| 초기 neutral 그레이스케일                                |
| Cloud Dancer 팔레트로 OKLCH 마이그레이션                 |
| 테스트 상태 색상 (active, focused, skipped, todo, xfail) |

### Radix UI 의존성

현재 사용 중인 프리미티브:

| 패키지                          | 컴포넌트      |
| ------------------------------- | ------------- |
| `@radix-ui/react-checkbox`      | Checkbox      |
| `@radix-ui/react-dialog`        | Dialog, Sheet |
| `@radix-ui/react-dropdown-menu` | Dropdown Menu |
| `@radix-ui/react-popover`       | Popover       |
| `@radix-ui/react-scroll-area`   | Scroll Area   |
| `@radix-ui/react-tabs`          | Tabs          |
| `@radix-ui/react-toggle`        | Toggle        |
| `@radix-ui/react-tooltip`       | Tooltip       |

## AI 기반 개발 시너지

### 이 스택이 AI 네이티브인 이유

shadcn/ui + Tailwind CSS 조합은 AI 기반 개발의 신흥 업계 표준. 이것이 이 기술 선택의 **핵심 전략적 이유**.

### Vercel v0 검증

Vercel의 플래그십 AI 제품 v0는 **오직** shadcn/ui + Tailwind 코드만 생성:

> "v0는 React, Tailwind, shadcn/ui의 모범 사례로 학습되었습니다. v0가 생성하는 모든 컴포넌트는 React, Next.js, Tailwind CSS, shadcn/ui를 사용합니다."
> — [Vercel 공식 블로그](https://vercel.com/blog/announcing-v0-generative-ui)

v0를 사용하는 팀들은 디자인 시스템이 shadcn/ui로 구축되었을 때 **3배 빠른** 디자인-구현 속도 보고.

### Tailwind: 인간에게 "못생김", AI에게 완벽

기존 CSS는 AI가 여러 파일에 걸쳐 관계를 추적해야 함:

| 시맨틱 CSS 문제         | Tailwind 해결책    | AI 이점                     |
| ----------------------- | ------------------ | --------------------------- |
| CSS 캐스케이드 부작용   | 자체 포함 유틸리티 | 예상치 못한 상속 없음       |
| 분리된 파일 의존성      | 인라인 선언        | 단일 파일에 완전한 컨텍스트 |
| 창의적 클래스 명명 필요 | 표준 어휘          | 일관된 생성                 |
| 숨겨진 스타일 관계      | 요소별 명시적      | 예측 가능한 수정            |

> "TailwindCSS의 유틸리티 우선 철학은 AI에게 놀이터와 같습니다. 복잡한 CSS 규칙을 처음부터 만드는 대신, AI는 Tailwind의 방대한 사전 정의 클래스 라이브러리를 활용할 수 있습니다."
> — [DEV Community](https://dev.to/brolag/tailwindcss-a-game-changer-for-ai-driven-code-generation-and-design-systems-18m7)

> "Tailwind는 AI가 실제로 정말 잘 사용하는 효과적인 스타일링 메커니즘입니다."
> — [Glide 블로그](https://www.glideapps.com/blog/tailwind-css)

### shadcn/ui: LLM을 위해 설계됨

shadcn/ui의 공식 설계 원칙:

> "AI-Ready: LLM이 읽고, 이해하고, 개선할 수 있는 오픈 코드."
> — [shadcn.io](https://www.shadcn.io/)

**copy-paste가 AI에 효과적인 이유:**

- **완전한 컨텍스트 가시성**: AI가 추상화된 API가 아닌 완전한 컴포넌트 소스를 봄
- **라이브러리 환각 없음**: 존재하지 않는 props 추측 없음
- **무제한 커스터마이징**: AI가 라이브러리 제약 없이 모든 라인 수정 가능
- **MCP 통합**: 공식 shadcn MCP 서버가 Claude, Cursor, VS Code에 실시간 컴포넌트 스펙 제공

### AI용 CSS-in-JS 비교

| 접근 방식             | AI 코드 생성                | 컨텍스트 요구사항  |
| --------------------- | --------------------------- | ------------------ |
| **Tailwind CSS**      | 우수 - 인라인, 예측 가능    | 단일 파일          |
| **styled-components** | 불량 - 런타임 컨텍스트 필요 | 다중 파일 + 런타임 |
| **CSS Modules**       | 보통 - 별도 파일            | CSS + JSX 파일     |

> "CSS Modules는 AI 제안에 덜 최적화되어 있습니다."
> — [Superflex AI 블로그](https://www.superflex.ai/blog/css-modules-vs-styled-components-vs-tailwind)

### AI 생산성 지표

| 지표                   | shadcn/ui + Tailwind | 기존 라이브러리      |
| ---------------------- | -------------------- | -------------------- |
| 디자인-구현            | v0로 3배 빠름        | 기준선               |
| AI 제안 정확도         | Tailwind 95%+        | CSS-in-JS 60-70%     |
| 컴포넌트 커스터마이징  | 즉시 (코드 소유)     | 래퍼/오버라이드 패턴 |
| 컨텍스트 윈도우 효율성 | 단일 파일            | 다중 파일 추적       |

## 결과

### 긍정적

**AI 개발 생산성:**

- AI 도구(v0, Claude, Cursor)로 3배 빠른 디자인-구현
- AI 코딩의 업계 표준 스택; 광범위한 학습 데이터
- AI가 라이브러리 API 제약 없이 소유된 코드를 자유롭게 수정
- Tailwind의 명시성이 숨겨진 스타일에 대한 AI 환각 제거

**커스터마이징 자유:**

- 완전한 소스 코드 소유권; 모든 컴포넌트 동작 수정
- 이슈 수정을 위해 라이브러리 업데이트 기다릴 필요 없음
- 빠른 스타일 반복을 위한 Tailwind 유틸리티 클래스
- AI가 예측 가능한 클래스 패턴으로 Tailwind 커스터마이징에 탁월

**번들 효율성:**

- MUI 80+ KB 또는 Ant Design 126 KB+ 대비 2.3 KB 초기 JS
- 사용된 컴포넌트만 번들링; 라이브러리 런타임 없음
- 더 빠른 First Contentful Paint (무거운 라이브러리 1.6초 대비 0.8초)

**기본 접근성:**

- Radix UI가 ARIA 속성, 포커스 관리, 키보드 네비게이션 처리
- 접근성 전문 지식 없이 WAI-ARIA 호환
- 스크린 리더 테스트된 프리미티브

**React 19 정렬:**

- deprecated `forwardRef` 사용 안 함
- 스타일링 훅을 위한 `data-slot` 속성
- React Compiler 최적화와 호환

### 부정적

**업데이트 오버헤드:**

- 업스트림 변경 시 수동으로 업데이트된 컴포넌트 복사 필요
- npm update로 자동 보안 패치 없음
- **완화**: Radix 버전 고정; 선택적 컴포넌트 업데이트; shadcn/ui 릴리스 모니터링

**컴포넌트 커버리지:**

- Ant Design보다 사전 구축된 복잡한 컴포넌트 적음
- 고급 사용 사례에 커스텀 컴포넌트 구축 필요할 수 있음
- **완화**: 추가 Radix 프리미티브로 확장; 커뮤니티 기여

**팀 학습:**

- Tailwind CSS 숙련도 필요
- cva로 variant 패턴 이해
- **완화**: Tailwind 문서; 페어 프로그래밍; 컴포넌트 문서화

### 번들 크기 비교

| 라이브러리    | 초기 번들 | 참고                              |
| ------------- | --------- | --------------------------------- |
| shadcn/ui     | 2.3 KB    | 복사된 컴포넌트만                 |
| Chakra UI v3  | ~50 KB    | 제로 런타임 개선                  |
| MUI v6        | ~80-90 KB | 코어 + Emotion                    |
| Ant Design v5 | 126 KB+   | ConfigProvider가 트리 쉐이킹 차단 |

### 컴포넌트 가용성

| 기능        | shadcn/ui    | MUI  | Chakra      | Ant Design     |
| ----------- | ------------ | ---- | ----------- | -------------- |
| 컴포넌트 수 | 40+          | 50+  | 30+         | 60+            |
| 접근성      | Radix (WCAG) | WCAG | WCAG        | WCAG           |
| 다크 모드   | next-themes  | 내장 | next-themes | ConfigProvider |
| i18n        | 외부         | 외부 | 외부        | 내장           |
| RSC 호환    | Yes          | No   | 부분적      | No             |

## 참고자료

### 내부

- [ADR-02: Next.js 16 + React 19 선택](/ko/adr/web/02-nextjs-react-selection.md)
- [ADR-06: PaaS 우선 인프라](/ko/adr/06-paas-first-infrastructure.md)

### 외부

- [shadcn/ui 문서](https://ui.shadcn.com/)
- [shadcn/ui Tailwind v4 지원](https://ui.shadcn.com/docs/tailwind-v4)
- [Radix UI 프리미티브](https://www.radix-ui.com/primitives)
- [Tailwind CSS v4](https://tailwindcss.com/)
- [class-variance-authority](https://cva.style/docs)
- [next-themes](https://github.com/pacocoursey/next-themes)

### AI 개발 리소스

- [Announcing v0: Generative UI - Vercel](https://vercel.com/blog/announcing-v0-generative-ui)
- [TailwindCSS: AI 기반 코드 생성의 게임 체인저 - DEV Community](https://dev.to/brolag/tailwindcss-a-game-changer-for-ai-driven-code-generation-and-design-systems-18m7)
- [Tailwind CSS가 AI 코딩에 중요한 이유 - Glide](https://www.glideapps.com/blog/tailwind-css)
- [shadcn MCP 문서](https://ui.shadcn.com/docs/mcp)
- [AI 네이티브 shadcn/ui 컴포넌트 라이브러리](https://www.shadcn.io/)
