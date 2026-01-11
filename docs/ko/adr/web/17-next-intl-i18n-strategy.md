---
title: next-intl i18n 전략
description: URL 기반 라우팅을 사용한 next-intl 국제화 선택에 대한 ADR
---

# ADR-17: next-intl i18n 전략

> [English Version](/en/adr/web/17-next-intl-i18n-strategy.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2025-12-06 | @KubrickCode | web    |

## 맥락

### 국제화 요구사항

SpecVital은 한국어와 영어권 개발자 커뮤니티를 대상으로 함. 적절한 i18n 솔루션 필요:

1. 한국어/영어 초기 지원
2. SEO를 위한 URL 기반 라우팅 (`/ko/...`, `/en/...`)
3. Next.js App Router 및 Server Components와의 원활한 통합
4. 첫 방문자를 위한 브라우저 언어 감지
5. 번역 키에 대한 타입 안전성 유지

### App Router 과제

Next.js 13+ App Router는 Pages Router가 제공하던 내장 `i18n` 설정을 제거. 이로 인해 새로운 요구사항 발생:

- `[locale]` 동적 세그먼트를 통한 수동 로케일 라우팅 설정
- Server Component 호환성 (`useEffect`나 클라이언트 감지 불가)
- 모든 번역을 클라이언트에 전송하지 않는 적절한 메시지 로딩
- 시간 민감 콘텐츠(상대 날짜)에 대한 하이드레이션 안전성

## 결정

**Next.js 프론트엔드의 국제화 라이브러리로 next-intl 채택.**

핵심 구현:

1. **URL 기반 라우팅**: 모든 라우트에 `/[locale]/` 접두사
2. **서버 우선**: Server Components에서 `getTranslations()` 사용
3. **ICU 메시지 형식**: 복수형 및 보간 지원
4. **브라우저 감지**: `Accept-Language` 기반 자동 리다이렉트

## 고려된 대안

### 대안 A: next-intl (선택됨)

**작동 방식:**

- Next.js App Router 전용 설계
- `getTranslations()`를 통한 네이티브 Server Component 지원
- `useTranslations()`를 통한 Client Component 훅
- 미들웨어가 로케일 감지 및 라우팅 처리

**장점:**

- **App Router 네이티브**: RSC와 App Router를 위해 특별히 설계
- **타입 안전성**: 번역 키에 대한 TypeScript 자동완성
- **번들 최적화**: 서버에서 현재 로케일의 메시지만 로드
- **간단한 API**: Server/Client Components 모두 동일한 훅 API
- **활발한 유지보수**: Next.js 릴리스에 따른 정기 업데이트

**단점:**

- i18next 대비 작은 생태계
- 정적 렌더링에 명시적 `setRequestLocale()` 호출 필요
- react-i18next 대비 문서 부족

### 대안 B: react-i18next + next-i18next

**작동 방식:**

- React 바인딩을 갖춘 성숙한 i18next 생태계
- App Router 호환성을 위한 추가 설정 필요
- 기능 확장을 위한 플러그인 기반 아키텍처

**장점:**

- 대규모 생태계와 커뮤니티
- 광범위한 기능 세트 (네임스페이스, 백엔드, 플러그인)
- 대규모 프로덕션에서 검증됨

**단점:**

- **App Router 마찰**: 원래 Pages Router용으로 설계
- Server Components를 위한 복잡한 설정
- 전체 i18next 코어로 인한 큰 번들 사이즈
- RSC 호환성을 위한 우회 방법 필요

### 대안 C: next-translate

**작동 방식:**

- 자동 코드 분할을 갖춘 파일 기반 번역
- i18next보다 단순한 기능 세트

**평가:**

- 결정 시점에 제한적인 App Router 지원
- next-intl 대비 적은 업데이트
- 일부 RSC 특화 최적화 부재
- **기각**: App Router 통합 불충분

### 대안 D: 내장 Next.js + 커스텀 솔루션

**작동 방식:**

- Next.js 미들웨어를 사용한 수동 구현
- 커스텀 번역 로딩 및 훅

**평가:**

- 최대 유연성이나 높은 유지보수 비용
- 복수형, 보간을 수동 구현해야 함
- 미묘한 하이드레이션 불일치 위험
- **기각**: 이미 해결된 문제 재발명

## 구현 상세

### 파일 구조

```
src/frontend/
├── i18n/
│   ├── config.ts      # 로케일 정의
│   ├── navigation.ts  # 지역화된 Link, useRouter
│   ├── request.ts     # 서버 측 메시지 로딩
│   └── routing.ts     # 라우팅 설정
├── messages/
│   ├── en.json        # 영어 번역
│   └── ko.json        # 한국어 번역
├── middleware.ts      # 로케일 감지, 리다이렉트
└── app/[locale]/      # 로케일 접두사 라우트
```

### 로케일 설정

영어를 기본으로 두 개의 로케일 지원:

- `en` (기본값): English
- `ko`: 한국어

### 미들웨어 동작

1. URL에서 로케일 접두사 확인
2. 없으면 `Accept-Language` 헤더에서 감지
3. 적절한 로케일 경로로 리다이렉트
4. 후속 요청을 위해 응답 쿠키 설정

### 번역 사용 패턴

**Server Components:**

```tsx
const t = await getTranslations("namespace");
return <h1>{t("key")}</h1>;
```

**Client Components:**

```tsx
const t = useTranslations("namespace");
return <button>{t("action")}</button>;
```

**ICU 복수형:**

```json
{
  "tests": "{count, plural, =0 {테스트 없음} =1 {1개 테스트} other {#개 테스트}}"
}
```

### 하이드레이션 안전성

상대 날짜와 같은 시간 민감 콘텐츠에는 `useNow()` 훅 사용:

```tsx
const now = useNow({ updateInterval: 60000 });
const formatted = formatRelativeTime(date, now);
```

서버와 클라이언트 렌더 시간 차이로 인한 하이드레이션 불일치 방지 가능.

## 결과

### 긍정적

**개발자 경험:**

- 번역 키에 대한 TypeScript 자동완성
- Server/Client Components 전반에 걸친 일관된 API
- 로케일별 번역 파일의 명확한 분리

**성능:**

- 서버 측 메시지 해결 (클라이언트 번들 비대화 없음)
- 라우트별 번역 로딩
- 필요시 클라이언트 컴포넌트용 지연 로딩

**SEO 이점:**

- 검색 엔진을 위한 로케일 접두사 URL
- `<html>`에 적절한 `lang` 속성
- 메타데이터에 `hreflang` 대안

**사용자 경험:**

- 첫 방문 시 자동 언어 감지
- 공유 가능한 지역화 URL
- 헤더의 언어 선택기

### 부정적

**정적 렌더링 제한:**

- 현재 정적 렌더링에 `setRequestLocale()` 필요
- 각 레이아웃과 페이지 컴포넌트에서 처리
- **완화**: 향후 next-intl 버전에서 이 요구사항 제거 목표

**학습 곡선:**

- 팀이 복수형을 위한 ICU 메시지 형식 학습 필요
- async vs sync 컨텍스트에 대한 다른 API
- **완화**: CLAUDE.md에 패턴 문서화

**메시지 관리:**

- 언어 파일 간 수동 동기화
- 한 로케일에서 번역 누락 위험
- **완화**: 누락 키에 대한 CI 체크 (향후 개선)

## 참고자료

- [next-intl 문서](https://next-intl.dev/)
- [ICU 메시지 형식](https://unicode-org.github.io/icu/userguide/format_parse/messages/)
