package specview

import (
	"crypto/sha256"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// GenerateContentHash creates a deterministic hash from test files and language.
// Hash = SHA256(sorted_file_paths + sorted_test_names + language)
func GenerateContentHash(files []FileInfo, language Language) []byte {
	h := sha256.New()

	// Sort files by normalized path for deterministic ordering
	sortedFiles := make([]FileInfo, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return normalizeFilePath(sortedFiles[i].Path) < normalizeFilePath(sortedFiles[j].Path)
	})

	for _, f := range sortedFiles {
		// Write normalized file path
		h.Write([]byte(normalizeFilePath(f.Path)))
		h.Write([]byte{0}) // null separator

		// Sort tests by normalized name for deterministic ordering
		sortedTests := make([]TestInfo, len(f.Tests))
		copy(sortedTests, f.Tests)
		sort.Slice(sortedTests, func(i, j int) bool {
			return normalizeTestName(sortedTests[i].Name) < normalizeTestName(sortedTests[j].Name)
		})

		for _, t := range sortedTests {
			// Write suite path if present (for uniqueness across suites)
			if t.SuitePath != "" {
				h.Write([]byte(t.SuitePath))
				h.Write([]byte{0}) // null separator
			}
			// Write normalized test name
			h.Write([]byte(normalizeTestName(t.Name)))
			h.Write([]byte{0}) // null separator
		}
	}

	// Write language as final component
	h.Write([]byte(language))

	return h.Sum(nil)
}

// normalizeFilePath normalizes a file path for consistent hashing.
// Converts backslashes to forward slashes and cleans the path.
func normalizeFilePath(path string) string {
	// Convert Windows-style backslashes to forward slashes
	normalized := strings.ReplaceAll(path, "\\", "/")
	// Clean the path to remove redundant separators and dots
	normalized = filepath.ToSlash(filepath.Clean(normalized))
	// Remove leading slash for consistency
	normalized = strings.TrimPrefix(normalized, "/")
	return normalized
}

// normalizeTestName normalizes a test name for consistent hashing.
// Trims whitespace and normalizes internal whitespace.
func normalizeTestName(name string) string {
	// Trim leading and trailing whitespace
	normalized := strings.TrimSpace(name)
	// Normalize internal whitespace (multiple spaces to single space)
	fields := strings.Fields(normalized)
	return strings.Join(fields, " ")
}

// GenerateCacheKeyHash creates a deterministic SHA-256 hash for behavior caching.
// Hash = SHA256(NFC(test_name) + "\x00" + NFC(suite_path) + "\x00" + NFC(file_path) + "\x00" + NFC(language) + "\x00" + NFC(model_id))
// Unicode NFC normalization ensures equivalent Unicode sequences produce the same hash.
func GenerateCacheKeyHash(key BehaviorCacheKey) []byte {
	h := sha256.New()

	// Apply NFC normalization to all string components
	h.Write(norm.NFC.Bytes([]byte(normalizeTestName(key.TestName))))
	h.Write([]byte{0}) // null separator

	h.Write(norm.NFC.Bytes([]byte(strings.TrimSpace(key.SuitePath))))
	h.Write([]byte{0})

	h.Write(norm.NFC.Bytes([]byte(normalizeFilePath(key.FilePath))))
	h.Write([]byte{0})

	h.Write(norm.NFC.Bytes([]byte(key.Language)))
	h.Write([]byte{0})

	h.Write(norm.NFC.Bytes([]byte(key.ModelID)))

	return h.Sum(nil)
}
