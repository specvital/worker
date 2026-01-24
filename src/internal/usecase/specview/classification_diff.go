package specview

import (
	"sort"

	"github.com/specvital/worker/internal/domain/specview"
)

// CalculateTestDiff computes the difference between cached test state and current files.
// Returns:
// - NewTests: tests in current files that are not in cache (need AI placement)
// - DeletedTests: tests in cache that are no longer in current files (need index removal)
func CalculateTestDiff(
	cachedIndexMap map[string]specview.TestIdentity,
	currentFiles []specview.FileInfo,
) specview.TestDiff {
	if cachedIndexMap == nil {
		cachedIndexMap = make(map[string]specview.TestIdentity)
	}

	currentKeySet := buildCurrentKeySet(currentFiles)

	var newTests []specview.TestInfo
	var deletedTests []specview.TestIdentity

	// Find new tests: in current but not in cache
	for _, file := range currentFiles {
		for _, test := range file.Tests {
			key := specview.TestKey(file.Path, test.SuitePath, test.Name)
			if _, exists := cachedIndexMap[key]; !exists {
				newTests = append(newTests, test)
			}
		}
	}

	// Find deleted tests: in cache but not in current
	for key, identity := range cachedIndexMap {
		if _, exists := currentKeySet[key]; !exists {
			deletedTests = append(deletedTests, identity)
		}
	}

	// Sort deleted tests for deterministic output
	sortDeletedTests(deletedTests)

	return specview.TestDiff{
		DeletedTests: deletedTests,
		NewTests:     newTests,
	}
}

// sortDeletedTests sorts by (DomainIndex, FeatureIndex, TestIndex) for deterministic order.
func sortDeletedTests(tests []specview.TestIdentity) {
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].DomainIndex != tests[j].DomainIndex {
			return tests[i].DomainIndex < tests[j].DomainIndex
		}
		if tests[i].FeatureIndex != tests[j].FeatureIndex {
			return tests[i].FeatureIndex < tests[j].FeatureIndex
		}
		return tests[i].TestIndex < tests[j].TestIndex
	})
}

// buildCurrentKeySet creates a set of test keys from current files.
func buildCurrentKeySet(files []specview.FileInfo) map[string]struct{} {
	keySet := make(map[string]struct{})
	for _, file := range files {
		for _, test := range file.Tests {
			key := specview.TestKey(file.Path, test.SuitePath, test.Name)
			keySet[key] = struct{}{}
		}
	}
	return keySet
}

// RemoveDeletedTestIndices removes deleted tests from Phase1Output and compacts indices.
// Returns a new Phase1Output with:
// - Deleted test indices removed from feature TestIndices
// - Empty features removed
// - Empty domains removed
func RemoveDeletedTestIndices(
	output *specview.Phase1Output,
	deletedTests []specview.TestIdentity,
) *specview.Phase1Output {
	if output == nil || len(deletedTests) == 0 {
		return output
	}

	// Build deletion lookup: (domainIdx, featureIdx, testIdx) -> true
	deleteSet := buildDeletionSet(deletedTests)

	var newDomains []specview.DomainGroup
	for di, domain := range output.Domains {
		newDomain := specview.DomainGroup{
			Confidence:  domain.Confidence,
			Description: domain.Description,
			Name:        domain.Name,
		}

		var newFeatures []specview.FeatureGroup
		for fi, feature := range domain.Features {
			newFeature := specview.FeatureGroup{
				Confidence:  feature.Confidence,
				Description: feature.Description,
				Name:        feature.Name,
			}

			var newTestIndices []int
			for ti, testIdx := range feature.TestIndices {
				key := deletionKey{
					domainIndex:  di,
					featureIndex: fi,
					testIndex:    ti,
				}
				if !deleteSet[key] {
					newTestIndices = append(newTestIndices, testIdx)
				}
			}

			// Skip empty features
			if len(newTestIndices) > 0 {
				newFeature.TestIndices = newTestIndices
				newFeatures = append(newFeatures, newFeature)
			}
		}

		// Skip empty domains
		if len(newFeatures) > 0 {
			newDomain.Features = newFeatures
			newDomains = append(newDomains, newDomain)
		}
	}

	return &specview.Phase1Output{
		Domains: newDomains,
	}
}

// deletionKey identifies a specific test position for deletion lookup.
type deletionKey struct {
	domainIndex  int
	featureIndex int
	testIndex    int
}

// buildDeletionSet creates a lookup set from deleted test identities.
func buildDeletionSet(deletedTests []specview.TestIdentity) map[deletionKey]bool {
	set := make(map[deletionKey]bool)
	for _, t := range deletedTests {
		key := deletionKey{
			domainIndex:  t.DomainIndex,
			featureIndex: t.FeatureIndex,
			testIndex:    t.TestIndex,
		}
		set[key] = true
	}
	return set
}
