---
title: 간접 Import Alias 감지 미지원
description: 단일 파일 파싱 한계로 인한 간접 import 체인을 통한 테스트 감지 미지원에 대한 ADR
---

# ADR-14: 간접 Import Alias 감지 미지원

> 🇺🇸 [English Version](/en/adr/core/14-indirect-import-unsupported.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2025-12-29 | @KubrickCode | core   |

**상태**: 승인됨

## 배경

### 문제 정의

SpecVital Core 파서는 정적 AST 분석을 사용하여 **단일 파일 단위**로 동작한다. 이로 인해 프로젝트가 테스트 유틸리티를 re-export하는 간접 import 패턴을 사용할 때 근본적인 한계가 발생한다.

### 발견 경위

`microsoft/playwright` 저장소 검증 결과:

- Ground Truth (CLI): 4,332개 테스트
- 파서 결과: 3,598개 테스트
- 차이: -734개 (16.9%)

원인 분석 결과 유효한 테스트 코드가 있음에도 22개 파일에서 0개 테스트가 감지되었다. 이 파일들은 간접 import 패턴을 사용했다:

```typescript
// tests/page/browsercontext-add-cookies.spec.ts
import type { Cookie } from "@playwright/test";
import { contextTest as it, expect } from "../config/browserTest";

it("should work @smoke", async ({ context, page, server }) => {
  // 테스트 코드
});
```

파서는 `../config/browserTest`의 `contextTest`가 궁극적으로 `@playwright/test`에서 re-export된다는 것을 추적할 수 없다.

### 기술 분석

```
직접 Import (지원됨):
  file.spec.ts → @playwright/test
  ✅ 파서가 "test" 함수 감지

간접 Import (미지원):
  file.spec.ts → ../config/browserTest → @playwright/test
  ❌ 파서가 import 체인을 따라갈 수 없음
```

## 결정

**간접 import alias 감지는 명시적으로 지원하지 않는다.**

파서는 프레임워크의 정규 import 경로(예: Playwright의 `@playwright/test`, Jest의 `@jest/globals`)에서 직접 import된 테스트 함수만 인식한다.

### 근거

1. **단일 파일 파싱 제약**: Core 아키텍처는 성능과 단순성을 위해 파일을 독립적으로 파싱
2. **다중 파일 분석 복잡도**: import 체인을 따라가려면 의존성 그래프를 구축해야 하며, 파싱 접근 방식이 근본적으로 변경됨
3. **휴리스틱의 불안정성**: 대안적 접근(네이밍 컨벤션 매칭)은 false positive와 프레임워크별 지식을 범용 파싱 로직에 도입함
4. **감지는 동작함**: 파일은 config scope를 통해 해당 프레임워크에 속하는 것으로 올바르게 감지됨; 테스트 추출만 실패

## 검토된 옵션

### 옵션 A: 한계 수용 (선택됨)

간접 import가 지원되지 않음을 문서화. re-export 패턴에 의존하는 사용자는 테스트 수가 적게 표시됨.

**장점:**

- 단일 파일 파싱의 단순성 유지
- 파서에 프레임워크별 휴리스틱 없음
- 명확하고 문서화된 한계
- 감지 레이어는 여전히 파일을 올바르게 식별

**단점:**

- 특정 코드베이스에서 일부 테스트가 카운트되지 않음
- re-export를 사용하는 프로젝트의 경우 파서 카운트가 CLI 카운트와 크게 다를 수 있음

### 옵션 B: 다중 파일 Import 해석

의존성 그래프를 구축하고 import 체인을 따라가 alias를 해석.

**장점:**

- 간접 import를 올바르게 처리
- 정적 import에 대해 100% 정확도

**단점:**

- **근본적인 아키텍처 변경**: 파일별이 아닌 전체 프로젝트 분석 필요
- **성능 영향**: 모든 import된 파일을 파싱하고 캐시해야 함
- **복잡도 폭발**: 순환 import, 조건부 export, re-export
- **범위 확대**: 완전한 TypeScript/JavaScript 타입 리졸버에 근접

### 옵션 C: 네이밍 컨벤션 휴리스틱

Playwright 전용 fixture 이름(`contextTest`, `browserTest` 등)을 모든 import 소스에서 감지.

**장점:**

- microsoft/playwright 및 유사한 코드베이스에서 동작
- 다중 파일 분석 불필요

**단점:**

- **파서가 프레임워크를 인식**: fixture 이름 하드코딩은 관심사 분리 위반
- **유지보수 부담**: 새 fixture 이름에 파서 업데이트 필요
- **False positive**: 다른 프로젝트에서 이름 충돌 가능
- **일반화 불가**: 프레임워크마다 다른 휴리스틱 필요
- **잘못된 레이어**: 이것은 파싱 로직이 아닌 감지 로직임

### 옵션 D: 사용자 설정

사용자가 설정에서 커스텀 alias를 지정할 수 있도록 허용.

**장점:**

- 유연하고 사용자 제어 가능
- 휴리스틱 불필요

**단점:**

- 설정 복잡도 증가
- 사용자가 내부 파서 동작을 이해해야 함
- 잘못된 설정 가능성

## 결과

### 긍정적

1. **아키텍처 무결성**: 단일 파일 파싱 모델 유지
2. **명확한 경계**: 파서는 파싱을 처리하고, 감지는 프레임워크 매칭을 처리
3. **문서화된 한계**: 사용자가 예상 동작을 이해함
4. **유지보수성**: 공유 파싱 코드에 프레임워크별 지식 없음

### 부정적

1. **정확도 차이**: re-export 패턴을 사용하는 프로젝트는 테스트가 적게 카운트됨
2. **microsoft/playwright 특히**: ~17% 테스트 미감지

### 완화 방안

1. **Config scope 감지**: 파일은 여전히 해당 프레임워크에 속하는 것으로 올바르게 식별됨
2. **정확한 카운트를 위한 CLI**: 정확한 카운트가 필요한 사용자는 프레임워크의 네이티브 CLI 사용
3. **일반적인 패턴은 동작**: 직접 `@playwright/test` import(권장 패턴)는 올바르게 동작

## 프레임워크 영향

| 프레임워크 | 정규 Import 경로   | Re-export 패턴    | 영향                      |
| ---------- | ------------------ | ----------------- | ------------------------- |
| Playwright | `@playwright/test` | Fixture re-export | microsoft/playwright ~17% |
| Jest       | `@jest/globals`    | 드묾              | 최소                      |
| Vitest     | `vitest`           | 드묾              | 최소                      |
| Mocha      | `mocha`            | 드묾              | 최소                      |
| Cypress    | `cypress`          | 드묾              | 최소                      |

대부분의 프레임워크는 정규 경로에서 직접 import를 권장하므로, 이 한계는 주로 microsoft/playwright의 내부 테스트 유틸리티와 같은 커스텀 테스트 인프라를 가진 프로젝트에만 해당된다.

## 관련 ADR

- [ADR-02: 동적 테스트 카운팅 정책](./02-dynamic-test-counting-policy.md) - 또 다른 정확도 한계
- [ADR-03: Tree-sitter AST 파싱 엔진](./03-tree-sitter-ast-parsing-engine.md) - 단일 파일 파싱 기반
- [ADR-08: 공유 파서 모듈](./08-shared-parser-modules.md) - 언어 수준 파싱 유틸리티

## 참고 자료

- [microsoft/playwright 테스트 인프라](https://github.com/microsoft/playwright/tree/main/tests/config)
- 검증 리포트: `realworld-test-report.md`
