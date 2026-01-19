---
title: Hierarchical Spec Document Schema
description: ADR for 4-table normalized database schema supporting BDD-aligned specification document storage
---

# ADR-19: Hierarchical Spec Document Schema

> 🇰🇷 [한국어 버전](/ko/adr/19-hierarchical-spec-document-schema.md)

| Date       | Author     | Repos              |
| ---------- | ---------- | ------------------ |
| 2026-01-12 | @specvital | infra, worker, web |

## Context

The existing `spec_view_cache` table was a flat key-value store designed for simple AI conversion result caching. The Document View feature requires:

1. **Business domain-based hierarchical organization** aligned with BDD/Specification concepts
2. **Level-independent queries** (fetch domains without loading all behaviors)
3. **Test case traceability** back to source analysis results
4. **Content-hash-based caching** with AI model version awareness

The flat cache structure cannot represent the natural hierarchy of specification documents: Domain → Feature → Behavior.

### Requirements

| Requirement                 | Description                                                 |
| --------------------------- | ----------------------------------------------------------- |
| Hierarchical Representation | Domain → Feature → Behavior structure matching BDD concepts |
| Level-Independent Queries   | Fetch domains for overview without loading all behaviors    |
| Cascade Deletion            | Analysis deletion propagates through entire document tree   |
| Test Traceability           | Link behaviors back to source test_cases for navigation     |
| Cache Efficiency            | Prevent redundant AI API calls via content-hash keying      |
| Model Versioning            | Different AI model versions produce different documents     |

### Constraints

| Constraint               | Impact                                      |
| ------------------------ | ------------------------------------------- |
| PostgreSQL Backend       | Must use relational schema patterns         |
| Existing Analysis Schema | Foreign key to analyses table required      |
| sqlc Code Generation     | No VIEWs, inline JOINs in queries preferred |

## Decision

**Adopt a 4-table normalized hierarchical schema aligned with BDD specification structure.**

```sql
-- Level 0: Document (per analysis)
CREATE TABLE spec_documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  analysis_id UUID NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
  content_hash BYTEA NOT NULL,
  language VARCHAR(10) NOT NULL DEFAULT 'en',
  executive_summary TEXT,
  model_id VARCHAR(100) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_spec_documents_hash_lang_model UNIQUE (content_hash, language, model_id)
);

-- Level 1: Domain (business classification)
CREATE TABLE spec_domains (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id UUID NOT NULL REFERENCES spec_documents(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  classification_confidence NUMERIC(3,2),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Level 2: Feature (functional grouping)
CREATE TABLE spec_features (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id UUID NOT NULL REFERENCES spec_domains(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Level 3: Behavior (leaf test specifications)
CREATE TABLE spec_behaviors (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_id UUID NOT NULL REFERENCES spec_features(id) ON DELETE CASCADE,
  source_test_case_id UUID REFERENCES test_cases(id) ON DELETE SET NULL,
  original_name VARCHAR(2000) NOT NULL,
  converted_description TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Key Design Decisions

| Decision                                               | Rationale                                                                                            |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| `content_hash + language + model_id` unique constraint | Cache key for deduplication; same content with different language/model produces different documents |
| `classification_confidence` at domain level only       | AI assigns domains during Phase 1 classification; features are grouped deterministically             |
| `source_test_case_id` with SET NULL on delete          | Maintains traceability while allowing test_case cleanup without breaking spec documents              |
| `sort_order` per level                                 | Preserves AI-assigned ordering for consistent UI rendering                                           |
| No VIEWs                                               | sqlc generates type-safe Go code; inline JOINs preferred                                             |

### Table Relationships

```
spec_documents (document level)
    │
    │ content_hash + language + model_id → unique
    │ analysis_id → FK to analyses (CASCADE delete)
    │
    └──► spec_domains (business domain classification)
            │
            │ document_id → FK to spec_documents (CASCADE delete)
            │ classification_confidence → AI confidence score
            │
            └──► spec_features (feature grouping)
                    │
                    │ domain_id → FK to spec_domains (CASCADE delete)
                    │
                    └──► spec_behaviors (individual test behaviors)
                            │
                            │ feature_id → FK to spec_features (CASCADE delete)
                            │ source_test_case_id → FK to test_cases (SET NULL)
```

## Options Considered

### Option A: Hierarchical 4-Table Normalized Structure (Selected)

Four tables with proper foreign key relationships representing Document → Domain → Feature → Behavior hierarchy.

| Pros                                        | Cons                                          |
| ------------------------------------------- | --------------------------------------------- |
| Level-independent queries                   | More complex queries requiring JOINs          |
| Proper FK constraints with cascade deletion | 4 tables increase schema maintenance surface  |
| BDD/Specification concept alignment         | INSERT requires 4 sequential operations       |
| Statistics via aggregate JOINs              | Ordering requires sort_order column per level |
| Test case traceability via FK               |                                               |

### Option B: Single Denormalized Table

All hierarchy levels in one table with nullable parent columns and `item_type` discriminator.

| Pros                        | Cons                                           |
| --------------------------- | ---------------------------------------------- |
| Simple schema (1 table)     | Cannot enforce level-specific constraints      |
| Easy writes (single INSERT) | No type safety for domain vs feature fields    |
|                             | Cannot query "all domains" efficiently         |
|                             | Recursive CTE required for hierarchy traversal |

**Rejected**: Cannot represent hierarchy properly; no level-specific queries possible without complex filtering.

### Option C: JSON Column Storage

Store entire document as JSON blob in single table.

| Pros                         | Cons                                             |
| ---------------------------- | ------------------------------------------------ |
| Schema flexibility           | Cannot query domains/features independently      |
| Single row per document      | No FK constraints to test_cases                  |
| Natural for document storage | Difficult aggregations (count by domain)         |
|                              | JSON path queries less efficient than relational |

**Rejected**: Eliminates relational query capabilities; no independent domain/feature statistics possible.

### Option D: 2-Table Structure (Document + Behaviors)

Only top-level document and leaf behaviors, losing intermediate hierarchy.

| Pros                                     | Cons                                               |
| ---------------------------------------- | -------------------------------------------------- |
| Simpler than 4 tables                    | Loses domain/feature as first-class entities       |
| Direct document-to-behavior relationship | Cannot fetch distinct domains efficiently          |
|                                          | Domain/feature counts require GROUP BY             |
|                                          | No domain-level metadata (confidence, description) |

**Rejected**: Loses domain/feature classification context; full document scan required for statistics.

## Consequences

### Positive

| Area              | Benefit                                                        |
| ----------------- | -------------------------------------------------------------- |
| Query Flexibility | Fetch domains for overview, expand to features on demand       |
| BDD Alignment     | Schema mirrors specification document mental model             |
| Test Traceability | `source_test_case_id` FK enables "view source" navigation      |
| Cascade Deletion  | DELETE analysis → document → domains → features → behaviors    |
| Cache Efficiency  | (content_hash, language, model_id) prevents redundant AI calls |
| Statistics        | COUNT/GROUP BY at each level without materialized views        |
| Type Safety       | sqlc generates distinct types per table                        |

### Negative

| Area                 | Trade-off                                      | Mitigation                           |
| -------------------- | ---------------------------------------------- | ------------------------------------ |
| Query Complexity     | Multi-table JOINs for full document retrieval  | sqlc named queries encapsulate JOINs |
| Write Complexity     | 4 INSERTs per document within transaction      | Single transaction, batch INSERTs    |
| Schema Surface       | 4 tables to maintain, migrate, index           | Clear table responsibilities         |
| Ordering             | sort_order column at each level                | AI pipeline assigns sort_order       |
| Confidence Asymmetry | classification_confidence only at domain level | Feature confidence addable if needed |

### Indexes

| Index                             | Purpose                                  |
| --------------------------------- | ---------------------------------------- |
| `idx_spec_documents_analysis`     | Fast lookup by analysis_id               |
| `idx_spec_domains_document_sort`  | Ordered domain retrieval                 |
| `idx_spec_features_domain_sort`   | Ordered feature retrieval                |
| `idx_spec_behaviors_feature_sort` | Ordered behavior retrieval               |
| `idx_spec_behaviors_source`       | Partial index for test case traceability |

## References

- [ADR-14: AI-Based Spec Document Generation Pipeline](/en/adr/14-ai-spec-generation-pipeline.md)
- [ADR-13: Billing and Quota Architecture](/en/adr/13-billing-quota-architecture.md)
- [Worker ADR-08: SpecView Worker Binary Separation](/en/adr/worker/08-specview-worker-separation.md)
- [Commit 38a33ad](https://github.com/specvital/infra/commit/38a33ad) - feat(db): replace spec_view_cache with hierarchical spec document schema
