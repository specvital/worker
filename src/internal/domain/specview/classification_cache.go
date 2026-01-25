package specview

import (
	"crypto/sha256"
	"sort"
	"time"
)

// ClassificationCache represents a cached Phase 1 classification result.
// Used for incremental caching: when tests change, only new tests are classified.
type ClassificationCache struct {
	ClassificationResult *Phase1Output          // Phase 1 output (domain/feature structure)
	CreatedAt            time.Time              // cache creation timestamp
	ExpiresAt            time.Time              // cache expiration timestamp
	FileSignature        []byte                 // deterministic hash of file paths
	ID                   string                 // unique identifier
	Language             Language               // language for classification
	ModelID              string                 // AI model used for classification
	TestIndexMap         map[string]TestIdentity // test key -> test identity mapping
}

// TestIdentity represents the identity and position of a test within Phase 1 output.
// Used to track test positions for incremental updates.
type TestIdentity struct {
	DomainIndex  int    // index in Phase1Output.Domains
	FeatureIndex int    // index in Phase1Output.Domains[].Features
	FilePath     string // file containing the test
	SuitePath    string // suite path within file
	TestIndex    int    // index in Phase1Output.Domains[].Features[].TestIndices
}

// TestDiff represents the difference between cached tests and current tests.
type TestDiff struct {
	DeletedTests []TestIdentity // tests that were removed
	NewTests     []TestInfo     // tests that were added (need placement)
}

// PlacementInput represents input for the new test placement AI call.
type PlacementInput struct {
	ExistingStructure *Phase1Output // current domain/feature structure (immutable)
	Language          Language      // target language
	NewTests          []TestInfo    // tests to place into existing structure
}

// PlacementOutput represents the result of new test placement.
type PlacementOutput struct {
	Placements []TestPlacement
}

// TestPlacement represents where a single test should be placed.
type TestPlacement struct {
	DomainName  string // target domain name (or "Uncategorized")
	FeatureName string // target feature name (or "Uncategorized")
	TestIndex   int    // test index from PlacementInput.NewTests
}

// GenerateFileSignature creates a deterministic hash from file paths.
// Used as cache key component: same set of files -> same signature.
// Hash = SHA256(sorted_normalized_file_paths)
func GenerateFileSignature(files []FileInfo) []byte {
	if len(files) == 0 {
		return sha256.New().Sum(nil)
	}

	// Collect and normalize file paths
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = normalizeFilePath(f.Path)
	}

	// Sort for deterministic ordering
	sort.Strings(paths)

	// Generate hash
	h := sha256.New()
	for i, p := range paths {
		if i > 0 {
			h.Write([]byte{0}) // null separator between paths
		}
		h.Write([]byte(p))
	}

	return h.Sum(nil)
}

// testKeyDelimiter is the delimiter used in test keys.
// Uses a string that won't appear in file paths, suite paths, or test names.
const testKeyDelimiter = "\x1f" // ASCII Unit Separator (valid in JSON, unlike \x00)

// TestKey generates a unique key for a test based on its identity.
// Key = normalized(filePath) + delimiter + suitePath + delimiter + normalized(testName)
func TestKey(filePath, suitePath, testName string) string {
	normalizedPath := normalizeFilePath(filePath)
	normalizedName := normalizeTestName(testName)
	return normalizedPath + testKeyDelimiter + suitePath + testKeyDelimiter + normalizedName
}

// BuildTestIndexMap creates a test key -> TestIdentity mapping from Phase1Output.
// Used to quickly look up test positions when calculating diffs.
func BuildTestIndexMap(output *Phase1Output, files []FileInfo) map[string]TestIdentity {
	if output == nil || len(files) == 0 {
		return make(map[string]TestIdentity)
	}

	// Build lookups: testIndex -> TestInfo and testIndex -> filePath
	testInfoByIndex := make(map[int]TestInfo)
	filePathByTestIndex := make(map[int]string)
	for _, f := range files {
		for _, t := range f.Tests {
			testInfoByIndex[t.Index] = TestInfo{
				Index:     t.Index,
				Name:      t.Name,
				SuitePath: t.SuitePath,
			}
			filePathByTestIndex[t.Index] = f.Path
		}
	}

	// Build test index map from Phase1Output structure
	indexMap := make(map[string]TestIdentity)
	for di, domain := range output.Domains {
		for fi, feature := range domain.Features {
			for ti, testIdx := range feature.TestIndices {
				testInfo, ok := testInfoByIndex[testIdx]
				if !ok {
					continue
				}
				filePath := filePathByTestIndex[testIdx]

				key := TestKey(filePath, testInfo.SuitePath, testInfo.Name)
				indexMap[key] = TestIdentity{
					DomainIndex:  di,
					FeatureIndex: fi,
					FilePath:     filePath,
					SuitePath:    testInfo.SuitePath,
					TestIndex:    ti,
				}
			}
		}
	}

	return indexMap
}
