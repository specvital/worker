---
title: OKLCH 디자인 토큰 시스템 + Cloud Dancer 테마
description: Pantone Cloud Dancer 기반 따뜻한 톤의 OKLCH 색상 공간 도입
---

# ADR-19: OKLCH 디자인 토큰 시스템 + Cloud Dancer 테마

> 🇺🇸 [English Version](/en/adr/web/19-css-variable-design-token-system.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2024-12-20 | @KubrickCode | web        |

## Context

### 기본 테마의 한계

shadcn/ui 기본 테마는 chroma가 0인 neutral grayscale 색상을 사용하여 차갑고 임상적인 느낌. 시각적 따뜻함과 개성 부족.

### Dark/Light 모드 불일치

하드코딩된 Tailwind 색상 클래스(예: `text-green-600`, `bg-yellow-500`)로 인해 light와 dark 모드 간 일관성 없는 외관 발생. light 모드에서 잘 보이던 색상이 dark 모드에서는 너무 밝거나 흐리게 보임.

### 색상 공간의 한계

전통적인 색상 공간(RGB, HSL)의 근본적 한계:

- **RGB/Hex**: 팔레트 생성을 위한 프로그래밍적 조작이 어려움
- **HSL**: 지각적으로 비균일 - 동일한 L 값이 hue마다 다르게 보임

### 테스트 상태 시각화

애플리케이션은 여러 상태(active, focused, skipped, todo, xfail)로 테스트 결과를 표시. 테마 간 일관되게 작동하는 상태별 색상에 대한 체계적 접근 필요.

## Decision

**일관되고 접근성 있는 테마를 위해 OKLCH 색상 공간과 CSS 변수를 디자인 토큰 시스템으로 채택.**

핵심 원칙:

1. **OKLCH 색상 공간**: 지각적으로 균일한 색상 표현 사용
2. **따뜻한 팔레트**: Pantone 2026 Cloud Dancer에서 영감받은 따뜻한 톤 적용
3. **시맨틱 변수**: 스펙트럼 기반이 아닌 용도 기반 색상 토큰 정의
4. **Tailwind 통합**: @theme 디렉티브로 유틸리티 클래스 생성에 토큰 노출

### 토큰 카테고리

| 카테고리   | 변수                                                                     | 용도                   |
| ---------- | ------------------------------------------------------------------------ | ---------------------- |
| **Core**   | background, foreground, card, popover, primary, secondary, muted, accent | 기본 UI 표면 및 텍스트 |
| **State**  | destructive, border, input, ring                                         | 인터랙티브 요소 상태   |
| **Chart**  | chart-1 ~ chart-5                                                        | 데이터 시각화          |
| **Status** | status-active, status-focused, status-skipped, status-todo, status-xfail | 테스트 결과 표시       |
| **Custom** | input-bg, hero-gradient-center, hero-gradient-edge                       | 컴포넌트별 오버라이드  |

### 색상 시스템 파라미터

| 파라미터      | Light 모드    | Dark 모드     | 용도                    |
| ------------- | ------------- | ------------- | ----------------------- |
| **Lightness** | 0.885 - 0.975 | 0.185 - 0.320 | 기본 밝기 레벨          |
| **Chroma**    | 0.004 - 0.010 | 0.010 - 0.014 | 미묘한 따뜻함 채도      |
| **Hue**       | 95° - 98°     | 95° - 98°     | 따뜻한 베이지/샌드 방향 |

## Options Considered

### Option A: OKLCH 색상 공간 (선택됨)

OKLCH는 Oklab 색상 모델을 기반으로 지각적 균일성을 갖춘 Lightness, Chroma, Hue 사용.

**장점:**

- **지각적 균일성**: 동일한 수치 단계가 동일한 시각적 변화를 생성
- **일관된 Lightness**: 동일 L 값이 모든 hue에서 동일하게 밝게 보임
- **Wide Gamut 지원**: Display P3 이상의 색상 표현 가능
- **예측 가능한 접근성**: 신뢰할 수 있는 WCAG 대비율 계산
- **더 나은 그라데이션**: 색상 보간 시 탁한 중간색 없음
- **브라우저 지원**: 2025년 기준 92%+ 글로벌 지원

**단점:**

- OKLCH에 익숙하지 않은 개발자의 학습 곡선
- out-of-gamut 값에 대한 클리핑 인식 필요
- 레거시 브라우저 폴백 필요 (<8% 사용률)

### Option B: HSL 변수

**작동 방식:**

직관적인 Hue-Saturation-Lightness 모델을 활용한 HSL 값의 CSS 변수.

**평가:**

- 더 직관적인 hue 선택 (0-360° 색상환)
- **기각**: 지각적으로 비균일 - 동일 L 값에서 노란색이 파란색보다 더 밝게 보임
- 일관되지 않은 팔레트 생성 결과

### Option C: RGB/Hex + Tailwind 팔레트

**작동 방식:**

표준 Tailwind 색상 팔레트(slate, gray, zinc)를 하드코딩된 hex 값으로 적용.

**평가:**

- 설정 없이 바로 사용 가능
- **기각**: 테마 커스터마이징 불가, 차가운 neutral 외관
- dark/light 모드 균형 유지 어려움

## Implementation Considerations

### @theme 디렉티브 통합

토큰은 @theme inline 디렉티브를 통해 Tailwind에 노출:

```css
@theme inline {
  --color-background: var(--background);
  --color-status-active: var(--status-active);
  /* ... */
}
```

이를 통해 CSS 변수의 단일 진실 공급원을 유지하면서 유틸리티 클래스(`bg-background`, `text-status-active`) 생성 가능.

### 테마 모드 전략

Light와 dark 모드는 동일한 hue 각도(95°-98°)를 공유하되 lightness를 반전하고 chroma를 조정:

- **Light**: 높은 lightness (0.9+), 낮은 chroma (0.004-0.010)
- **Dark**: 낮은 lightness (0.2-0.3), 가시성을 위해 약간 높은 chroma (0.010-0.014)

### 상태 색상 매핑

| 상태    | Light 모드 Hue | Dark 모드 L 조정 | 시맨틱 의미       |
| ------- | -------------- | ---------------- | ----------------- |
| active  | 145° (녹색)    | +0.10            | 통과/성공         |
| focused | 310° (마젠타)  | +0.10            | 현재 선택됨       |
| skipped | 85° (노랑)     | +0.07            | 의도적으로 건너뜀 |
| todo    | 240° (파랑)    | +0.10            | 구현 대기 중      |
| xfail   | 25° (주황)     | +0.08            | 예상된 실패       |

## Consequences

### Positive

**시각적 일관성:**

- 모든 컴포넌트에 걸쳐 통일된 따뜻한 외관
- 원활한 light/dark 모드 전환
- 전문적이고 친근한 미학

**개발자 경험:**

- 시맨틱 네이밍이 인지 부하 감소 (`text-status-active` vs `text-green-600`)
- 색상 수정을 위한 단일 진실 공급원
- 토큰에서 Tailwind 유틸리티 클래스 자동 생성

**접근성:**

- 지각적 균일성으로 예측 가능한 대비율
- 상태 색상이 양쪽 모드에서 구분 유지
- 테마 간 일관된 읽기 경험

**미래 대비:**

- HDR 디스플레이용 wide gamut 준비
- Tailwind v4 네이티브 OKLCH 정렬

### Negative

**학습 곡선:**

- OKLCH가 HSL/RGB보다 덜 익숙함
- **완화책**: 일반적인 값 문서화 및 변환 도구 제공

**브라우저 호환성:**

- ~8% 브라우저가 OKLCH 미지원
- **완화책**: 타겟 사용자(최신 브라우저 사용 개발자)에게는 허용 가능

**Gamut 클리핑:**

- 일부 OKLCH 값이 sRGB 디스플레이 역량 초과
- **완화책**: 모든 프로덕션 값을 sRGB gamut 내에서 테스트

## References

- [Pantone Color of the Year 2026: Cloud Dancer](https://www.pantone.com/articles/press-releases/pantone-announces-color-of-the-year-2026-cloud-dancer)
- [OKLCH in CSS: Why We Moved from RGB and HSL](https://evilmartians.com/chronicles/oklch-in-css-why-quit-rgb-hsl)
- [Tailwind CSS v4.0 Release](https://tailwindcss.com/blog/tailwindcss-v4)
- [MDN: oklch()](https://developer.mozilla.org/en-US/docs/Web/CSS/color_value/oklch)
