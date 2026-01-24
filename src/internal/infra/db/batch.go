package db

const InsertTestSuiteBatch = `
INSERT INTO test_suites (file_id, parent_id, name, line_number, depth)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`

var TestCaseCopyColumns = []string{"suite_id", "name", "line_number", "status", "tags", "modifier"}

const InsertSpecDomainBatch = `
INSERT INTO spec_domains (document_id, name, description, sort_order, classification_confidence)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`

const InsertSpecFeatureBatch = `
INSERT INTO spec_features (domain_id, name, description, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING id`

var SpecBehaviorCopyColumns = []string{
	"feature_id",
	"source_test_case_id",
	"original_name",
	"converted_description",
	"sort_order",
}

const UpsertBehaviorCacheBatch = `
INSERT INTO behavior_caches (cache_key_hash, converted_description)
VALUES ($1, $2)
ON CONFLICT (cache_key_hash) DO UPDATE
SET converted_description = EXCLUDED.converted_description`
