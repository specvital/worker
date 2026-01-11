---
title: next-themes 다크 모드
description: 시스템 테마 감지를 포함한 다크 모드 구현을 위한 next-themes 선택 ADR
---

# ADR-18: next-themes 다크 모드

> 🇺🇸 [English Version](/en/adr/web/18-next-themes-dark-mode.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-12-06 | @KubrickCode | web        |

## 맥락

### 다크 모드의 필요성

현대 웹 애플리케이션에서 다크 모드는 필수 기능:

1. **사용자 선호**: 테마 커스터마이징에 대한 기대
2. **접근성**: 저조도 환경에서의 눈 피로 감소
3. **시스템 통합**: OS 레벨 테마 설정 존중
4. **전문성**: 개발자 도구의 업계 표준 기능

### SSR 환경의 기술적 과제

Server-Side Rendered 애플리케이션에서 다크 모드 구현의 고유한 문제:

- **Hydration 불일치**: 서버는 한 테마로 렌더링, 클라이언트는 다른 테마 선호
- **테마 깜빡임 (FOIT)**: 페이지 로드 시 잘못된 테마가 잠깐 표시
- **상태 지속성**: 세션 간 사용자 설정 기억
- **시스템 감지**: OS 테마 변경에 대한 감지 및 반응

### 기존 아키텍처

프로젝트에서 이미 사용 중인 기술:

- **Next.js 16 App Router**: SSR/SSG를 지원하는 Server Components
- **Tailwind CSS v4**: `dark:` 변형을 지원하는 유틸리티 우선 스타일링
- **shadcn/ui**: next-themes를 권장하는 컴포넌트 라이브러리
- **OKLCH 색상 공간**: 테마를 위한 CSS 변수 (ADR-05)

## 결정

**SSR 안전 테마 관리와 시스템 테마 감지를 위해 next-themes를 다크 모드 솔루션으로 채택.**

핵심 원칙:

1. **Hydration 안전성**: 주입된 스크립트로 테마 깜빡임 방지
2. **시스템 통합**: `prefers-color-scheme` 자동 감지
3. **사용자 오버라이드**: 수동 라이트/다크/시스템 선택 허용
4. **Tailwind 호환성**: `dark:` 유틸리티를 위한 클래스 기반 다크 모드
5. **지속성**: LocalStorage 기반 설정 보존

## 검토한 옵션

### 옵션 A: next-themes (선택됨)

**작동 방식:**

- ThemeProvider가 애플리케이션을 감싸고 `<head>`에 블로킹 스크립트 주입
- 스크립트가 paint 전에 localStorage/시스템 설정 읽음
- `<html>` 요소에 `class="dark"` 동기적 설정
- 첫 렌더링 전 테마 결정으로 FOIT 방지

**장점:**

- **SSR 깜빡임 방지**: 동기 스크립트 주입으로 어려운 문제 해결
- **shadcn/ui 표준**: 다크 모드 공식 권장 솔루션
- **Tailwind 통합**: `dark:` 변형을 위한 네이티브 `attribute="class"`
- **제로 설정**: 합리적인 기본값으로 즉시 작동
- **경량**: ~2KB gzipped, 최소 런타임 오버헤드
- **탭 동기화**: 브라우저 탭 간 자동 동기화
- **업계 검증**: 480만+ 주간 다운로드, 6.1k GitHub stars

**단점:**

- 단일 메인테이너 (pacocoursey)
- `<html>` 요소에 `suppressHydrationWarning` 필요

### 옵션 B: CSS 전용 prefers-color-scheme

**작동 방식:**

- CSS 미디어 쿼리 `@media (prefers-color-scheme: dark)` 사용
- Tailwind 설정: `darkMode: 'media'`
- JavaScript 불필요

**평가:**

- **사용자 토글 없음**: 시스템 설정 오버라이드 불가
- **지속성 없음**: 사용자 선택 기억 불가
- **하이브리드 모드 없음**: 라이트/다크/시스템 옵션 제공 불가
- **기각**: 사용자 기대에 미치지 못하는 기능 세트

### 옵션 C: Custom Context + zustand

**작동 방식:**

- zustand를 사용한 ThemeContext 생성
- localStorage 지속성 수동 구현
- `_document.tsx` 또는 layout에 블로킹 스크립트 추가

**평가:**

- **바퀴의 재발명**: next-themes 기능 복제에 50+ 라인 필요
- **유지보수 부담**: 엣지 케이스 처리 필요 (SSR, hydration, 탭 동기화)
- **오류 가능성**: 미묘한 hydration 불일치 발생 쉬움
- **기각**: 검증된 라이브러리 대비 이점 없음

### 옵션 D: usehooks-ts useDarkMode

**작동 방식:**

- usehooks-ts 라이브러리에서 `useDarkMode` 훅 임포트
- `isDarkMode`, `toggle`, `enable`, `disable` API 제공

**평가:**

- **SSR 솔루션 없음**: hydration 깜빡임 미해결
- **시스템 감지 없음**: 시스템 설정에 `useTernaryDarkMode` 필요
- **라이브러리 오버헤드**: 단일 기능에 ~10KB
- **기각**: 불완전한 SSR 처리; 여전히 커스텀 스크립트 필요

## 구현 세부사항

### ThemeProvider 설정

Provider가 루트 레이아웃에서 애플리케이션을 감싸며 특정 옵션 사용:

- `attribute="class"`: Tailwind를 위해 `<html>`에 `.dark` 클래스 설정
- `defaultTheme="system"`: 기본값으로 OS 설정 존중
- `enableSystem`: `prefers-color-scheme` 감지 활성화
- `disableTransitionOnChange`: 전환 시 거슬리는 색상 전환 방지

### Hydration 안전 패턴

레이아웃에서 `<html>` 요소에 `suppressHydrationWarning` 적용:

```
html[suppressHydrationWarning] → ThemeProvider → App
```

next-themes가 hydration 전에 의도적으로 클래스를 수정하므로 서버/클라이언트 클래스 불일치에 대한 React 경고 억제.

### 토글 컴포넌트 패턴

ThemeToggle 컴포넌트는 mounted 상태 패턴 사용:

1. 서버가 플레이스홀더 렌더링 (정적 아이콘)
2. `useEffect`가 클라이언트에서 `mounted=true` 설정
3. 그 후에야 현재 테마로 인터랙티브 토글 렌더링
4. 토글 UI의 hydration 불일치 방지

### Tailwind v4 통합

Tailwind v4는 기본적으로 `prefers-color-scheme` 미디어 쿼리 사용. next-themes와 클래스 기반 다크 모드 적용 시:

```css
@custom-variant dark (&:where(.dark, .dark *));
```

이를 통해 `dark:` 유틸리티가 next-themes가 적용하는 `.dark` 클래스에 반응하도록 설정.

### CSS 변수 테마

라이트/다크 테마는 `globals.css`에 OKLCH 색상 공간으로 정의:

| 모드   | 배경         | 전경         |
| ------ | ------------ | ------------ |
| 라이트 | oklch(0.952) | oklch(0.25)  |
| 다크   | oklch(0.185) | oklch(0.950) |

전체 색상 팔레트는 시맨틱 토큰 포함: primary, secondary, muted, accent, destructive, status colors.

## 결과

### 긍정적

**사용자 경험:**

- 페이지 로드 시 잘못된 테마 깜빡임 없음
- 원활한 시스템 설정 감지
- 세션 간 사용자 선택 지속
- 부드러운 테마 전환 (활성화 시)

**개발자 경험:**

- ThemeProvider 두 줄 설정
- 표준 `useTheme()` 훅 API
- 커스텀 SSR 처리 불필요
- Tailwind `dark:` 유틸리티 직접 작동

**생태계 정렬:**

- shadcn/ui 공식 권장
- Vercel/Next.js 커뮤니티 표준
- 풍부한 문서와 예제

### 부정적

**단일 메인테이너:**

- 한 명의 개발자가 라이브러리 유지보수
- **완화**: 안정적인 API, 최소 업데이트 필요; 방치 시 포크 용이

**Hydration 경고 억제:**

- `<html>`에 `suppressHydrationWarning` 추가 필요
- **완화**: 잘 문서화된 패턴; 실제 hydration 문제 없음

**애니메이션 복잡성:**

- 테마 토글 애니메이션은 `setTheme`과 신중한 조율 필요
- **완화**: 애니메이션 완료까지 테마 변경 지연 (커밋 `6004d1e`)

## 참고자료

### 내부

- [ADR-02: Next.js 16 + React 19 선택](/ko/adr/web/02-nextjs-react-selection.md)
- [ADR-05: shadcn/ui + Tailwind CSS 선택](/ko/adr/web/05-shadcn-tailwind-selection.md)

### 외부

- [next-themes GitHub 저장소](https://github.com/pacocoursey/next-themes)
- [next-themes npm 패키지](https://www.npmjs.com/package/next-themes)
- [shadcn/ui 다크 모드 문서](https://ui.shadcn.com/docs/dark-mode/next)
- [Tailwind CSS 다크 모드](https://tailwindcss.com/docs/dark-mode)
