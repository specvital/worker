package db

const InsertTestSuiteBatch = `
INSERT INTO test_suites (analysis_id, parent_id, name, file_path, line_number, framework, depth)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id`

var TestCaseCopyColumns = []string{"suite_id", "name", "line_number", "status", "tags", "modifier"}
