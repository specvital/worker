package postgres

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed schema.sql
var rawSchema string

// Schema returns the processed schema SQL ready for test database initialization.
// It transforms pg_dump output into executable SQL for fresh PostgreSQL containers.
func Schema() string {
	schema := rawSchema

	// Remove CREATE SCHEMA public (already exists in fresh PostgreSQL)
	schema = regexp.MustCompile(`(?i)CREATE SCHEMA public;`).ReplaceAllString(schema, "")

	// Remove pg_dump header comments but keep table/column comments
	lines := strings.Split(schema, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip pg_dump metadata comments
		if strings.HasPrefix(trimmed, "-- Dumped from") ||
			strings.HasPrefix(trimmed, "-- Dumped by") ||
			strings.HasPrefix(trimmed, "-- PostgreSQL database dump") {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
}
