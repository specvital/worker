package specview

import (
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	UncategorizedName        = "Uncategorized"
	UncategorizedDescription = "Uncategorized tests"
	UncategorizedConfidence  = 0.0
)

// ensureUncategorizedExists returns a Phase1Output with an Uncategorized domain/feature.
// If the domain or feature doesn't exist, they are created at the end.
// Returns: (modified output, domain index, feature index)
func ensureUncategorizedExists(output *specview.Phase1Output) (*specview.Phase1Output, int, int) {
	if output == nil {
		output = &specview.Phase1Output{}
	}

	newOutput := copyPhase1Output(output)

	domainIdx := findDomainIndex(newOutput, UncategorizedName)
	if domainIdx == -1 {
		newOutput.Domains = append(newOutput.Domains, specview.DomainGroup{
			Confidence:  UncategorizedConfidence,
			Description: UncategorizedDescription,
			Features:    nil,
			Name:        UncategorizedName,
		})
		domainIdx = len(newOutput.Domains) - 1
	}

	featureIdx := findFeatureIndex(newOutput.Domains[domainIdx], UncategorizedName)
	if featureIdx == -1 {
		newOutput.Domains[domainIdx].Features = append(
			newOutput.Domains[domainIdx].Features,
			specview.FeatureGroup{
				Confidence:  UncategorizedConfidence,
				Description: UncategorizedDescription,
				Name:        UncategorizedName,
				TestIndices: nil,
			},
		)
		featureIdx = len(newOutput.Domains[domainIdx].Features) - 1
	}

	return newOutput, domainIdx, featureIdx
}

// placeAllToUncategorized places all new tests into the Uncategorized domain/feature.
// Used as a fallback when placement AI call fails.
func placeAllToUncategorized(output *specview.Phase1Output, newTests []specview.TestInfo) *specview.Phase1Output {
	if len(newTests) == 0 {
		return output
	}

	newOutput, domainIdx, featureIdx := ensureUncategorizedExists(output)

	for _, test := range newTests {
		newOutput.Domains[domainIdx].Features[featureIdx].TestIndices = append(
			newOutput.Domains[domainIdx].Features[featureIdx].TestIndices,
			test.Index,
		)
	}

	return newOutput
}

// applyPlacements applies AI placement results to Phase1Output.
// For each placement, finds matching domain/feature and appends the test index.
// If domain/feature not found, places test in Uncategorized.
func applyPlacements(
	output *specview.Phase1Output,
	placements []specview.TestPlacement,
) *specview.Phase1Output {
	if len(placements) == 0 {
		return output
	}

	newOutput := copyPhase1Output(output)
	uncategorizedDomainIdx := -1
	uncategorizedFeatureIdx := -1

	for _, placement := range placements {
		domainIdx := findDomainIndex(newOutput, placement.DomainName)
		if domainIdx == -1 {
			newOutput, uncategorizedDomainIdx, uncategorizedFeatureIdx = ensureUncategorizedIfNeeded(
				newOutput,
				uncategorizedDomainIdx,
				uncategorizedFeatureIdx,
			)
			newOutput.Domains[uncategorizedDomainIdx].Features[uncategorizedFeatureIdx].TestIndices = append(
				newOutput.Domains[uncategorizedDomainIdx].Features[uncategorizedFeatureIdx].TestIndices,
				placement.TestIndex,
			)
			continue
		}

		featureIdx := findFeatureIndex(newOutput.Domains[domainIdx], placement.FeatureName)
		if featureIdx == -1 {
			newOutput, uncategorizedDomainIdx, uncategorizedFeatureIdx = ensureUncategorizedIfNeeded(
				newOutput,
				uncategorizedDomainIdx,
				uncategorizedFeatureIdx,
			)
			newOutput.Domains[uncategorizedDomainIdx].Features[uncategorizedFeatureIdx].TestIndices = append(
				newOutput.Domains[uncategorizedDomainIdx].Features[uncategorizedFeatureIdx].TestIndices,
				placement.TestIndex,
			)
			continue
		}

		newOutput.Domains[domainIdx].Features[featureIdx].TestIndices = append(
			newOutput.Domains[domainIdx].Features[featureIdx].TestIndices,
			placement.TestIndex,
		)
	}

	return newOutput
}

func copyPhase1Output(output *specview.Phase1Output) *specview.Phase1Output {
	if output == nil {
		return &specview.Phase1Output{}
	}

	newDomains := make([]specview.DomainGroup, len(output.Domains))
	for di, domain := range output.Domains {
		newFeatures := make([]specview.FeatureGroup, len(domain.Features))
		for fi, feature := range domain.Features {
			newTestIndices := make([]int, len(feature.TestIndices))
			copy(newTestIndices, feature.TestIndices)
			newFeatures[fi] = specview.FeatureGroup{
				Confidence:  feature.Confidence,
				Description: feature.Description,
				Name:        feature.Name,
				TestIndices: newTestIndices,
			}
		}
		newDomains[di] = specview.DomainGroup{
			Confidence:  domain.Confidence,
			Description: domain.Description,
			Features:    newFeatures,
			Name:        domain.Name,
		}
	}

	return &specview.Phase1Output{
		Domains: newDomains,
	}
}

func ensureUncategorizedIfNeeded(
	output *specview.Phase1Output,
	cachedDomainIdx int,
	cachedFeatureIdx int,
) (*specview.Phase1Output, int, int) {
	if cachedDomainIdx >= 0 && cachedFeatureIdx >= 0 {
		return output, cachedDomainIdx, cachedFeatureIdx
	}
	return ensureUncategorizedExists(output)
}

func findDomainIndex(output *specview.Phase1Output, name string) int {
	for i, domain := range output.Domains {
		if domain.Name == name {
			return i
		}
	}
	return -1
}

func findFeatureIndex(domain specview.DomainGroup, name string) int {
	for i, feature := range domain.Features {
		if feature.Name == name {
			return i
		}
	}
	return -1
}
