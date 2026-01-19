---
title: GitHub App 설치 스키마
description: GitHub App 설치 라이프사이클 관리를 위한 데이터베이스 스키마 설계 ADR
---

# ADR-20: GitHub App 설치 스키마

> 🇺🇸 [English Version](/en/adr/20-github-app-installation-schema.md)

| 날짜       | 작성자     | 레포       |
| ---------- | ---------- | ---------- |
| 2026-01-19 | @specvital | infra, web |

## 컨텍스트

### 문제

OAuth App 인증은 조직 관리자 승인이 필요하여 일반 사용자의 조직 리포지토리 접근 제한:

| 이슈                   | 영향                                                        |
| ---------------------- | ----------------------------------------------------------- |
| 조직 관리자 승인 필수  | 관리자 개입 없이 일반 사용자의 조직 리포지토리 접근 불가    |
| 장기 OAuth 토큰 저장   | 데이터베이스 내 영구 자격 증명으로 인한 보안 위험           |
| 공유 Rate Limit        | 사용자당 단일 5000/hr 풀로 다중 리포지토리 시 빠른 소진     |
| 백그라운드 처리 의존성 | Worker가 프라이빗 리포지토리 접근에 저장된 사용자 토큰 필요 |

### 요구사항

1. 사용자 멤버십 없이 조직 리포지토리 접근
2. 설치당 독립적 Rate Limit (각 5000/hr)
3. 안전한 토큰 처리 - 자격 증명 저장 최소화
4. 장기 저장 토큰 없이 백그라운드 Worker 지원
5. 웹훅 기반 설치 라이프사이클 추적

### ADR-09와의 관계

이 스키마는 [ADR-09: GitHub App 통합 전략](/ko/adr/09-github-app-integration.md)의 "Installation Store" 컴포넌트 구현. ADR-09가 통합 전략 수립; 이 ADR은 해당 전략을 실현하는 구체적 스키마 설계 정의.

## 결정

**설치 메타데이터만 저장 (`installation_id`, `account_type`, `account_id`) - 액세스 토큰 미저장. GitHub App의 개인 키를 사용한 JWT 인증으로 단기 설치 토큰 온디맨드 생성.**

### 스키마 설계

```sql
CREATE TYPE "public"."github_account_type" AS ENUM ('organization', 'user');

CREATE TABLE "public"."github_app_installations" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "installation_id" bigint NOT NULL,
  "account_type" "public"."github_account_type" NOT NULL,
  "account_id" bigint NOT NULL,
  "account_login" character varying(255) NOT NULL,
  "account_avatar_url" text NULL,
  "installer_user_id" uuid NULL,
  "suspended_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "uq_github_app_installations_account" UNIQUE ("account_type", "account_id"),
  CONSTRAINT "uq_github_app_installations_installation_id" UNIQUE ("installation_id"),
  CONSTRAINT "fk_github_app_installations_installer" FOREIGN KEY ("installer_user_id")
    REFERENCES "public"."users" ("id") ON DELETE SET NULL
);

CREATE INDEX "idx_github_app_installations_installer"
  ON "public"."github_app_installations" ("installer_user_id")
  WHERE (installer_user_id IS NOT NULL);
```

### 컬럼 명세

| 컬럼                 | 타입               | 목적                            |
| -------------------- | ------------------ | ------------------------------- |
| `id`                 | UUID               | 내부 기본 키                    |
| `installation_id`    | BIGINT UNIQUE      | GitHub 설치 식별자              |
| `account_type`       | ENUM               | 'organization' 또는 'user'      |
| `account_id`         | BIGINT             | GitHub 계정 ID                  |
| `account_login`      | VARCHAR(255)       | GitHub 사용자명/조직명 (표시용) |
| `account_avatar_url` | TEXT               | 아바타 URL (nullable)           |
| `installer_user_id`  | UUID FK (nullable) | 설치를 시작한 사용자            |
| `suspended_at`       | TIMESTAMPTZ        | 정지 타임스탬프 (null = 활성)   |

### 핵심 설계 결정

1. **토큰 미저장**: JWT를 통해 온디맨드 생성, 영구 저장 안 함
2. **account_type에 PostgreSQL enum**: VARCHAR 대비 타입 안전성
3. **이중 유니크 제약**: `installation_id`와 `(account_type, account_id)` 모두
4. **installer에 부분 인덱스**: null 제외하고 "내 설치 목록" 쿼리 최적화
5. **소프트 정지**: 감사 추적을 위해 boolean 대신 `suspended_at` 타임스탬프

### 토큰 생성 흐름

```
installation_id → 개인 키로 JWT 서명 → GitHub API → 1시간 액세스 토큰
```

## 고려한 옵션

### 옵션 A: installation_id만 저장 (채택)

JWT + 개인 키를 사용하여 액세스 토큰 온디맨드 생성.

| 장점                                      | 단점                            |
| ----------------------------------------- | ------------------------------- |
| 자격 증명 저장 제로 - 토큰 유출 위험 제거 | 토큰 생성 지연 (~50-200ms/요청) |
| 항상 신선한 토큰 (만료/갱신 로직 불필요)  | 개인 키 관리 복잡성             |
| GitHub 보안 모범 사례 준수                | 성능을 위한 인메모리 캐싱 필요  |

### 옵션 B: 전체 access_token 저장

생성된 토큰을 만료 시간과 함께 저장, 사용 전 갱신.

| 장점                              | 단점                                        |
| --------------------------------- | ------------------------------------------- |
| 캐시된 토큰에 대한 생성 지연 없음 | 저장된 자격 증명이 데이터베이스 침해에 취약 |
| 토큰 검색을 위한 단순한 코드 경로 | 토큰 갱신 로직과 만료 모니터링 필요         |
|                                   | 만료 토큰 처리 엣지 케이스                  |

**기각**: 저장된 자격 증명으로 인한 보안 위험이 지연 이점을 상회. GitHub은 설치 액세스 토큰 저장을 명시적으로 권장하지 않음.

### 옵션 C: Users 테이블 확장

설치 컬럼을 기존 users 테이블에 직접 추가.

| 장점                            | 단점                                      |
| ------------------------------- | ----------------------------------------- |
| 새 테이블 불필요, 단순한 스키마 | 조직 설치 지원 불가 (해당 사용자 행 없음) |
| 자연스러운 사용자-설치 관계     | 사용자당 하나의 설치만 가능               |
|                                 | 사용자 ID와 설치 ID 혼동                  |

**기각**: 조직 설치는 해당하는 사용자 행이 없음; 별도 엔티티 필요.

## 결과

### 긍정적

| 영역      | 이점                                                                     |
| --------- | ------------------------------------------------------------------------ |
| 보안      | 장기 자격 증명 미저장; 단기 토큰으로 침해 영향 제한                      |
| 확장성    | 설치당 독립적 5000/hr Rate Limit                                         |
| 조직 지원 | 사용자 멤버십 없이 일급 지원                                             |
| 웹훅 정렬 | 스키마가 GitHub 라이프사이클 이벤트 (install/delete/suspend)에 직접 매핑 |

### 부정적

| 영역 | 트레이드오프                   | 완화                              |
| ---- | ------------------------------ | --------------------------------- |
| 지연 | 토큰 생성에 ~50-200ms 추가     | ~55분 TTL의 인메모리 캐시         |
| 운영 | 개인 키가 핵심 시크릿으로 전환 | 키 볼트 저장, 로테이션 절차       |
| UX   | 인증 후 추가 설치 단계 필요    | 명확한 온보딩 흐름, 설치 프롬프트 |

### 기술적 함의

- **데이터베이스 설계**: 타입 안전성을 위한 PostgreSQL enum; 부분 인덱스로 일반 쿼리 패턴 최적화
- **크로스 서비스 조정**: Web 서비스가 설치 추적, Worker가 installation_id로 토큰 요청
- **웹훅 처리**: 이벤트 (`installation.created`, `installation.deleted`, `installation.suspend`)가 테이블 작업에 매핑

## 구현 파일

### specvital/infra

| 파일                                                               | 목적                     |
| ------------------------------------------------------------------ | ------------------------ |
| `db/schema/migrations/20251226154124_github_app_installations.sql` | 테이블 생성 마이그레이션 |

### specvital/web

| 파일                                                   | 목적                                   |
| ------------------------------------------------------ | -------------------------------------- |
| `internal/client/github_app.go`                        | JWT 토큰 생성이 포함된 GitHubAppClient |
| `modules/github-app/domain/entity/installation.go`     | Installation 도메인 엔티티             |
| `modules/github-app/domain/port/repository.go`         | InstallationRepository 포트            |
| `modules/github-app/adapter/repository_postgres.go`    | PostgreSQL 리포지토리 어댑터           |
| `modules/github-app/handler/http.go`                   | 웹훅 핸들러                            |
| `modules/github-app/usecase/get_installation_token.go` | 토큰 생성 유스케이스                   |

## 참조

- [cd33ecb](https://github.com/specvital/infra/commit/cd33ecb): feat(db): add GitHub App Installation table
- [0db7539](https://github.com/specvital/infra/commit/0db7539): feat(db): add refresh token table for hybrid authentication
- [ADR-09: GitHub App 통합 전략](/ko/adr/09-github-app-integration.md) - 상위 전략 ADR
- [ADR-08: External Repository ID 기반 데이터 무결성](/ko/adr/08-external-repo-id-integrity.md) - 데이터 무결성 패턴
- [GitHub: GitHub App 생성 모범 사례](https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/best-practices-for-creating-a-github-app)
