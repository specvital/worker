-- name: UpsertCodebase :one
INSERT INTO codebases (host, owner, name, default_branch, external_repo_id, is_private)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (host, external_repo_id)
DO UPDATE SET
    owner = EXCLUDED.owner,
    name = EXCLUDED.name,
    default_branch = COALESCE(EXCLUDED.default_branch, codebases.default_branch),
    is_stale = false,
    is_private = EXCLUDED.is_private,
    updated_at = now()
RETURNING *;

-- name: FindCodebaseByExternalID :one
SELECT * FROM codebases
WHERE host = $1 AND external_repo_id = $2;

-- name: FindCodebaseByOwnerName :one
SELECT * FROM codebases
WHERE host = $1 AND owner = $2 AND name = $3 AND is_stale = false;

-- name: GetCodebaseByID :one
SELECT * FROM codebases WHERE id = $1;

-- name: CreateAnalysis :one
INSERT INTO analyses (id, codebase_id, commit_sha, branch_name, status, started_at, parser_version)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateAnalysisCompleted :exec
UPDATE analyses
SET status = 'completed', total_suites = $2, total_tests = $3, completed_at = $4, committed_at = $5
WHERE id = $1;

-- name: UpdateAnalysisFailed :exec
UPDATE analyses
SET status = 'failed', error_message = $2, completed_at = $3
WHERE id = $1;

-- name: CreateTestCase :one
INSERT INTO test_cases (suite_id, name, line_number, status, tags, modifier)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTestSuitesByFileID :many
SELECT * FROM test_suites WHERE file_id = $1 ORDER BY line_number;

-- name: GetTestCasesBySuiteID :many
SELECT * FROM test_cases WHERE suite_id = $1 ORDER BY line_number;

-- name: GetOAuthAccountByUserAndProvider :one
SELECT * FROM oauth_accounts WHERE user_id = $1 AND provider = $2;

-- name: MarkCodebaseStale :exec
UPDATE codebases SET is_stale = true, updated_at = now() WHERE id = $1;

-- name: UnmarkCodebaseStale :one
UPDATE codebases
SET is_stale = false, owner = $2, name = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCodebaseOwnerName :one
UPDATE codebases
SET owner = $2, name = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCodebaseVisibility :exec
UPDATE codebases
SET is_private = $2, updated_at = now()
WHERE id = $1;

-- name: FindCodebaseWithLastCommitByOwnerName :one
SELECT
    c.*,
    COALESCE(a.commit_sha, '') as last_commit_sha
FROM codebases c
LEFT JOIN (
    SELECT DISTINCT ON (codebase_id) codebase_id, commit_sha
    FROM analyses
    WHERE status = 'completed'
    ORDER BY codebase_id, completed_at DESC
) a ON c.id = a.codebase_id
WHERE c.host = $1 AND c.owner = $2 AND c.name = $3 AND c.is_stale = false;

-- name: GetCodebasesForAutoRefresh :many
WITH latest_completions AS (
    SELECT DISTINCT ON (codebase_id)
        codebase_id,
        completed_at,
        commit_sha,
        parser_version
    FROM analyses
    WHERE status = 'completed'
    ORDER BY codebase_id, completed_at DESC
),
failure_counts AS (
    SELECT
        a.codebase_id,
        COUNT(*)::int as failure_count
    FROM analyses a
    LEFT JOIN latest_completions lc ON a.codebase_id = lc.codebase_id
    WHERE a.status = 'failed'
      AND a.created_at > COALESCE(lc.completed_at, '1970-01-01'::timestamptz)
    GROUP BY a.codebase_id
)
SELECT
    c.id, c.host, c.owner, c.name, c.last_viewed_at,
    lc.completed_at as last_completed_at,
    lc.commit_sha as last_commit_sha,
    COALESCE(lc.parser_version, 'legacy') as last_parser_version,
    COALESCE(fc.failure_count, 0)::int as consecutive_failures
FROM codebases c
LEFT JOIN latest_completions lc ON c.id = lc.codebase_id
LEFT JOIN failure_counts fc ON c.id = fc.codebase_id
WHERE c.last_viewed_at IS NOT NULL
  AND c.last_viewed_at > now() - interval '90 days'
  AND c.is_stale = false
  AND c.is_private = false;

-- name: RecordUserAnalysisHistory :exec
INSERT INTO user_analysis_history (user_id, analysis_id)
VALUES ($1, $2)
ON CONFLICT ON CONSTRAINT uq_user_analysis_history_user_analysis
DO UPDATE SET updated_at = now();

-- name: InsertTestFile :one
INSERT INTO test_files (analysis_id, file_path, framework, domain_hints)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: InsertTestSuite :one
INSERT INTO test_suites (file_id, parent_id, name, line_number, depth)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: UpsertSystemConfig :exec
INSERT INTO system_config (key, value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (key)
DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: GetSystemConfig :one
SELECT value FROM system_config WHERE key = $1;

-- =============================================================================
-- SPEC DOCUMENTS
-- =============================================================================

-- name: GetMaxVersionByUserAnalysisAndLanguage :one
SELECT COALESCE(MAX(version), 0)::int as max_version
FROM spec_documents
WHERE user_id = $1 AND analysis_id = $2 AND language = $3;

-- name: FindSpecDocumentByContentHash :one
SELECT sd.* FROM spec_documents sd
WHERE sd.user_id = $1
  AND sd.content_hash = $2
  AND sd.language = $3
  AND sd.model_id = $4
  AND sd.version = (
    SELECT MAX(version)
    FROM spec_documents
    WHERE user_id = sd.user_id
      AND analysis_id = sd.analysis_id
      AND language = sd.language
  );

-- name: InsertSpecDocument :one
INSERT INTO spec_documents (user_id, analysis_id, content_hash, language, model_id, version)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: InsertSpecDomain :one
INSERT INTO spec_domains (document_id, name, description, sort_order, classification_confidence)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: InsertSpecFeature :one
INSERT INTO spec_features (domain_id, name, description, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetTestDataByAnalysisID :many
SELECT
    tf.id as file_id,
    tf.file_path,
    tf.framework,
    tf.domain_hints,
    ts.id as suite_id,
    ts.parent_id as suite_parent_id,
    ts.name as suite_name,
    ts.depth as suite_depth,
    tc.id as test_case_id,
    tc.name as test_name
FROM test_files tf
JOIN test_suites ts ON ts.file_id = tf.id
JOIN test_cases tc ON tc.suite_id = ts.id
WHERE tf.analysis_id = $1
ORDER BY tf.file_path, ts.depth, ts.name, tc.name;

-- name: CheckAnalysisExists :one
SELECT EXISTS(SELECT 1 FROM analyses WHERE id = $1) as exists;

-- name: RecordUserSpecviewHistory :exec
INSERT INTO user_specview_history (user_id, document_id)
VALUES ($1, $2)
ON CONFLICT ON CONSTRAINT uq_user_specview_history_user_document
DO UPDATE SET updated_at = now();

-- =============================================================================
-- USAGE EVENTS
-- =============================================================================

-- name: RecordSpecViewUsageEvent :exec
INSERT INTO usage_events (user_id, event_type, document_id, quota_amount)
VALUES ($1, 'specview', $2, $3);

-- name: GetMonthlySpecViewUsage :one
SELECT COALESCE(SUM(quota_amount), 0)::int as total
FROM usage_events
WHERE user_id = $1
  AND event_type = 'specview'
  AND created_at >= date_trunc('month', CURRENT_DATE);

-- name: RecordAnalysisUsageEvent :exec
INSERT INTO usage_events (user_id, event_type, analysis_id, quota_amount)
VALUES ($1, 'analysis', $2, $3);

-- name: GetAnalysisContext :one
SELECT c.host, c.owner, c.name as repo
FROM analyses a
JOIN codebases c ON a.codebase_id = c.id
WHERE a.id = $1;
