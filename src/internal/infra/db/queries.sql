-- name: UpsertCodebase :one
INSERT INTO codebases (host, owner, name, default_branch, external_repo_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (host, external_repo_id)
DO UPDATE SET
    owner = EXCLUDED.owner,
    name = EXCLUDED.name,
    default_branch = COALESCE(EXCLUDED.default_branch, codebases.default_branch),
    is_stale = false,
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
INSERT INTO analyses (id, codebase_id, commit_sha, branch_name, status, started_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateAnalysisCompleted :exec
UPDATE analyses
SET status = 'completed', total_suites = $2, total_tests = $3, completed_at = $4
WHERE id = $1;

-- name: UpdateAnalysisFailed :exec
UPDATE analyses
SET status = 'failed', error_message = $2, completed_at = $3
WHERE id = $1;

-- name: CreateTestSuite :one
INSERT INTO test_suites (analysis_id, parent_id, name, file_path, line_number, framework, depth)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateTestCase :one
INSERT INTO test_cases (suite_id, name, line_number, status, tags, modifier)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTestSuitesByAnalysisID :many
SELECT * FROM test_suites WHERE analysis_id = $1 ORDER BY file_path, line_number;

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
        commit_sha
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
    COALESCE(fc.failure_count, 0)::int as consecutive_failures
FROM codebases c
LEFT JOIN latest_completions lc ON c.id = lc.codebase_id
LEFT JOIN failure_counts fc ON c.id = fc.codebase_id
WHERE c.last_viewed_at IS NOT NULL
  AND c.last_viewed_at > now() - interval '90 days';
