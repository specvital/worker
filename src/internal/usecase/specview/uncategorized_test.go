package specview

import (
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestEnsureUncategorizedExists(t *testing.T) {
	t.Run("creates domain and feature when output is nil", func(t *testing.T) {
		result, domainIdx, featureIdx := ensureUncategorizedExists(nil)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.Domains) != 1 {
			t.Errorf("expected 1 domain, got %d", len(result.Domains))
		}
		if result.Domains[0].Name != UncategorizedName {
			t.Errorf("expected domain name %q, got %q", UncategorizedName, result.Domains[0].Name)
		}
		if result.Domains[0].Confidence != UncategorizedConfidence {
			t.Errorf("expected confidence %f, got %f", UncategorizedConfidence, result.Domains[0].Confidence)
		}
		if result.Domains[0].Description != UncategorizedDescription {
			t.Errorf("expected description %q, got %q", UncategorizedDescription, result.Domains[0].Description)
		}
		if domainIdx != 0 {
			t.Errorf("expected domain index 0, got %d", domainIdx)
		}
		if featureIdx != 0 {
			t.Errorf("expected feature index 0, got %d", featureIdx)
		}
	})

	t.Run("creates domain and feature when output is empty", func(t *testing.T) {
		result, domainIdx, featureIdx := ensureUncategorizedExists(&specview.Phase1Output{})

		if len(result.Domains) != 1 {
			t.Errorf("expected 1 domain, got %d", len(result.Domains))
		}
		if len(result.Domains[0].Features) != 1 {
			t.Errorf("expected 1 feature, got %d", len(result.Domains[0].Features))
		}
		if domainIdx != 0 {
			t.Errorf("expected domain index 0, got %d", domainIdx)
		}
		if featureIdx != 0 {
			t.Errorf("expected feature index 0, got %d", featureIdx)
		}
	})

	t.Run("appends to existing domains", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
				{Name: "User", Confidence: 0.8},
			},
		}

		result, domainIdx, featureIdx := ensureUncategorizedExists(output)

		if len(result.Domains) != 3 {
			t.Errorf("expected 3 domains, got %d", len(result.Domains))
		}
		if domainIdx != 2 {
			t.Errorf("expected domain index 2, got %d", domainIdx)
		}
		if featureIdx != 0 {
			t.Errorf("expected feature index 0, got %d", featureIdx)
		}
		if result.Domains[2].Name != UncategorizedName {
			t.Errorf("expected last domain name %q, got %q", UncategorizedName, result.Domains[2].Name)
		}
	})

	t.Run("finds existing Uncategorized domain and creates feature", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
				{Name: UncategorizedName, Confidence: UncategorizedConfidence},
			},
		}

		result, domainIdx, featureIdx := ensureUncategorizedExists(output)

		if len(result.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(result.Domains))
		}
		if domainIdx != 1 {
			t.Errorf("expected domain index 1, got %d", domainIdx)
		}
		if featureIdx != 0 {
			t.Errorf("expected feature index 0, got %d", featureIdx)
		}
		if len(result.Domains[1].Features) != 1 {
			t.Errorf("expected 1 feature, got %d", len(result.Domains[1].Features))
		}
	})

	t.Run("finds existing Uncategorized domain and feature", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
				{
					Name:       UncategorizedName,
					Confidence: UncategorizedConfidence,
					Features: []specview.FeatureGroup{
						{Name: "SomeFeature", Confidence: 0.5},
						{Name: UncategorizedName, Confidence: UncategorizedConfidence, TestIndices: []int{10}},
					},
				},
			},
		}

		result, domainIdx, featureIdx := ensureUncategorizedExists(output)

		if len(result.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(result.Domains))
		}
		if domainIdx != 1 {
			t.Errorf("expected domain index 1, got %d", domainIdx)
		}
		if featureIdx != 1 {
			t.Errorf("expected feature index 1, got %d", featureIdx)
		}
		if len(result.Domains[1].Features) != 2 {
			t.Errorf("expected 2 features, got %d", len(result.Domains[1].Features))
		}
	})

	t.Run("does not modify original output", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
			},
		}

		result, _, _ := ensureUncategorizedExists(output)

		if len(output.Domains) != 1 {
			t.Errorf("original should have 1 domain, got %d", len(output.Domains))
		}
		if len(result.Domains) != 2 {
			t.Errorf("result should have 2 domains, got %d", len(result.Domains))
		}
	})
}

func TestPlaceAllToUncategorized(t *testing.T) {
	t.Run("places all tests into Uncategorized", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
			},
		}
		newTests := []specview.TestInfo{
			{Index: 5, Name: "TestNew1"},
			{Index: 6, Name: "TestNew2"},
			{Index: 7, Name: "TestNew3"},
		}

		result := placeAllToUncategorized(output, newTests)

		if len(result.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(result.Domains))
		}

		uncatDomain := result.Domains[1]
		if uncatDomain.Name != UncategorizedName {
			t.Errorf("expected domain name %q, got %q", UncategorizedName, uncatDomain.Name)
		}
		if len(uncatDomain.Features) != 1 {
			t.Errorf("expected 1 feature, got %d", len(uncatDomain.Features))
		}

		uncatFeature := uncatDomain.Features[0]
		if len(uncatFeature.TestIndices) != 3 {
			t.Errorf("expected 3 test indices, got %d", len(uncatFeature.TestIndices))
		}
		if uncatFeature.TestIndices[0] != 5 || uncatFeature.TestIndices[1] != 6 || uncatFeature.TestIndices[2] != 7 {
			t.Errorf("expected test indices [5,6,7], got %v", uncatFeature.TestIndices)
		}
	})

	t.Run("returns original when no new tests", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
			},
		}

		result := placeAllToUncategorized(output, []specview.TestInfo{})

		if result != output {
			t.Error("expected same output reference when no new tests")
		}
	})

	t.Run("appends to existing Uncategorized feature", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: UncategorizedName,
					Features: []specview.FeatureGroup{
						{Name: UncategorizedName, TestIndices: []int{1, 2}},
					},
				},
			},
		}
		newTests := []specview.TestInfo{
			{Index: 5, Name: "TestNew"},
		}

		result := placeAllToUncategorized(output, newTests)

		if len(result.Domains[0].Features[0].TestIndices) != 3 {
			t.Errorf("expected 3 test indices, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
	})

	t.Run("does not modify original output", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Confidence: 0.9},
			},
		}
		newTests := []specview.TestInfo{
			{Index: 5, Name: "TestNew"},
		}

		placeAllToUncategorized(output, newTests)

		if len(output.Domains) != 1 {
			t.Errorf("original should have 1 domain, got %d", len(output.Domains))
		}
	})
}

func TestApplyPlacements(t *testing.T) {
	t.Run("applies placements to correct domains and features", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0, 1}},
						{Name: "Logout", TestIndices: []int{2}},
					},
				},
				{
					Name: "Payment",
					Features: []specview.FeatureGroup{
						{Name: "Checkout", TestIndices: []int{3}},
					},
				},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 10},
			{DomainName: "Payment", FeatureName: "Checkout", TestIndex: 11},
		}

		result := applyPlacements(output, placements)

		if len(result.Domains[0].Features[0].TestIndices) != 3 {
			t.Errorf("expected 3 tests in Auth.Login, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
		if result.Domains[0].Features[0].TestIndices[2] != 10 {
			t.Errorf("expected test index 10 appended to Auth.Login, got %d", result.Domains[0].Features[0].TestIndices[2])
		}
		if len(result.Domains[1].Features[0].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Payment.Checkout, got %d", len(result.Domains[1].Features[0].TestIndices))
		}
	})

	t.Run("places to Uncategorized when domain not found", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "NonExistent", FeatureName: "SomeFeature", TestIndex: 10},
		}

		result := applyPlacements(output, placements)

		if len(result.Domains) != 2 {
			t.Errorf("expected 2 domains (original + Uncategorized), got %d", len(result.Domains))
		}
		uncatDomain := result.Domains[1]
		if uncatDomain.Name != UncategorizedName {
			t.Errorf("expected Uncategorized domain, got %q", uncatDomain.Name)
		}
		if len(uncatDomain.Features[0].TestIndices) != 1 {
			t.Errorf("expected 1 test in Uncategorized, got %d", len(uncatDomain.Features[0].TestIndices))
		}
		if uncatDomain.Features[0].TestIndices[0] != 10 {
			t.Errorf("expected test index 10 in Uncategorized, got %d", uncatDomain.Features[0].TestIndices[0])
		}
	})

	t.Run("places to Uncategorized when feature not found", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "NonExistentFeature", TestIndex: 10},
		}

		result := applyPlacements(output, placements)

		if len(result.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(result.Domains))
		}
		uncatDomain := result.Domains[1]
		if uncatDomain.Name != UncategorizedName {
			t.Errorf("expected Uncategorized domain, got %q", uncatDomain.Name)
		}
		if uncatDomain.Features[0].TestIndices[0] != 10 {
			t.Errorf("expected test index 10 in Uncategorized, got %d", uncatDomain.Features[0].TestIndices[0])
		}
	})

	t.Run("handles mixed valid and invalid placements", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login", TestIndices: []int{0}}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 10},
			{DomainName: "NonExistent", FeatureName: "Whatever", TestIndex: 11},
			{DomainName: "Auth", FeatureName: "NonExistent", TestIndex: 12},
		}

		result := applyPlacements(output, placements)

		if len(result.Domains[0].Features[0].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Auth.Login, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
		uncatDomain := result.Domains[1]
		if len(uncatDomain.Features[0].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Uncategorized, got %d", len(uncatDomain.Features[0].TestIndices))
		}
	})

	t.Run("returns original when no placements", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth"},
			},
		}

		result := applyPlacements(output, []specview.TestPlacement{})

		if result != output {
			t.Error("expected same output reference when no placements")
		}
	})

	t.Run("does not modify original output", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login", TestIndices: []int{0}}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 10},
		}

		applyPlacements(output, placements)

		if len(output.Domains[0].Features[0].TestIndices) != 1 {
			t.Errorf("original should have 1 test, got %d", len(output.Domains[0].Features[0].TestIndices))
		}
	})

	t.Run("places to existing Uncategorized domain when specified", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
				{
					Name: UncategorizedName,
					Features: []specview.FeatureGroup{
						{Name: UncategorizedName, TestIndices: []int{5}},
					},
				},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: UncategorizedName, FeatureName: UncategorizedName, TestIndex: 10},
		}

		result := applyPlacements(output, placements)

		if len(result.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(result.Domains))
		}
		uncatFeature := result.Domains[1].Features[0]
		if len(uncatFeature.TestIndices) != 2 {
			t.Errorf("expected 2 tests in Uncategorized, got %d", len(uncatFeature.TestIndices))
		}
	})
}

func TestCopyPhase1Output(t *testing.T) {
	t.Run("returns empty output for nil input", func(t *testing.T) {
		result := copyPhase1Output(nil)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.Domains) != 0 {
			t.Errorf("expected 0 domains, got %d", len(result.Domains))
		}
	})

	t.Run("creates deep copy of all nested structures", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name:        "Auth",
					Confidence:  0.9,
					Description: "Authentication",
					Features: []specview.FeatureGroup{
						{
							Name:        "Login",
							Confidence:  0.85,
							Description: "Login feature",
							TestIndices: []int{0, 1, 2},
						},
					},
				},
			},
		}

		result := copyPhase1Output(output)

		result.Domains[0].Name = "Modified"
		result.Domains[0].Features[0].TestIndices[0] = 999

		if output.Domains[0].Name != "Auth" {
			t.Errorf("original domain name should be 'Auth', got %q", output.Domains[0].Name)
		}
		if output.Domains[0].Features[0].TestIndices[0] != 0 {
			t.Errorf("original test index should be 0, got %d", output.Domains[0].Features[0].TestIndices[0])
		}
	})

	t.Run("preserves all metadata", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name:        "Auth",
					Confidence:  0.9,
					Description: "Authentication",
					Features: []specview.FeatureGroup{
						{
							Name:        "Login",
							Confidence:  0.85,
							Description: "Login feature",
							TestIndices: []int{0, 1},
						},
					},
				},
			},
		}

		result := copyPhase1Output(output)

		if result.Domains[0].Name != "Auth" {
			t.Errorf("expected domain name 'Auth', got %q", result.Domains[0].Name)
		}
		if result.Domains[0].Confidence != 0.9 {
			t.Errorf("expected confidence 0.9, got %f", result.Domains[0].Confidence)
		}
		if result.Domains[0].Description != "Authentication" {
			t.Errorf("expected description 'Authentication', got %q", result.Domains[0].Description)
		}
		if result.Domains[0].Features[0].Name != "Login" {
			t.Errorf("expected feature name 'Login', got %q", result.Domains[0].Features[0].Name)
		}
		if result.Domains[0].Features[0].Confidence != 0.85 {
			t.Errorf("expected feature confidence 0.85, got %f", result.Domains[0].Features[0].Confidence)
		}
	})

	t.Run("handles multiple domains and features", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0, 1}},
						{Name: "Logout", TestIndices: []int{2}},
					},
				},
				{
					Name: "User",
					Features: []specview.FeatureGroup{
						{Name: "Profile", TestIndices: []int{3, 4, 5}},
					},
				},
			},
		}

		result := copyPhase1Output(output)

		if len(result.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result.Domains))
		}
		if len(result.Domains[0].Features) != 2 {
			t.Fatalf("expected 2 features in first domain, got %d", len(result.Domains[0].Features))
		}
		if len(result.Domains[1].Features[0].TestIndices) != 3 {
			t.Errorf("expected 3 test indices in User/Profile, got %d", len(result.Domains[1].Features[0].TestIndices))
		}
	})
}

func TestEnsureUncategorizedExists_EdgeCases(t *testing.T) {
	t.Run("handles output with nil features slice", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: nil},
			},
		}

		result, domainIdx, featureIdx := ensureUncategorizedExists(output)

		if len(result.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result.Domains))
		}
		if domainIdx != 1 {
			t.Errorf("expected domainIdx 1, got %d", domainIdx)
		}
		if featureIdx != 0 {
			t.Errorf("expected featureIdx 0, got %d", featureIdx)
		}
	})

	t.Run("handles uncategorized domain with nil features slice", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: UncategorizedName, Features: nil},
			},
		}

		result, domainIdx, featureIdx := ensureUncategorizedExists(output)

		if len(result.Domains) != 1 {
			t.Fatalf("expected 1 domain, got %d", len(result.Domains))
		}
		if len(result.Domains[0].Features) != 1 {
			t.Fatalf("expected 1 feature, got %d", len(result.Domains[0].Features))
		}
		if domainIdx != 0 {
			t.Errorf("expected domainIdx 0, got %d", domainIdx)
		}
		if featureIdx != 0 {
			t.Errorf("expected featureIdx 0, got %d", featureIdx)
		}
	})

	t.Run("returns correct indices when uncategorized at different positions", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: UncategorizedName,
					Features: []specview.FeatureGroup{
						{Name: "Other1"},
						{Name: "Other2"},
						{Name: UncategorizedName},
						{Name: "Other3"},
					},
				},
			},
		}

		result, domainIdx, featureIdx := ensureUncategorizedExists(output)

		if domainIdx != 0 {
			t.Errorf("expected domainIdx 0, got %d", domainIdx)
		}
		if featureIdx != 2 {
			t.Errorf("expected featureIdx 2, got %d", featureIdx)
		}
		if len(result.Domains[0].Features) != 4 {
			t.Errorf("expected 4 features, got %d", len(result.Domains[0].Features))
		}
	})
}

func TestPlaceAllToUncategorized_EdgeCases(t *testing.T) {
	t.Run("handles nil output with tests", func(t *testing.T) {
		tests := []specview.TestInfo{
			{Index: 0, Name: "TestNew"},
		}

		result := placeAllToUncategorized(nil, tests)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.Domains) != 1 {
			t.Fatalf("expected 1 domain, got %d", len(result.Domains))
		}
		if len(result.Domains[0].Features[0].TestIndices) != 1 {
			t.Errorf("expected 1 test index, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
	})

	t.Run("preserves test index order", func(t *testing.T) {
		output := &specview.Phase1Output{}
		tests := []specview.TestInfo{
			{Index: 100, Name: "Test100"},
			{Index: 50, Name: "Test50"},
			{Index: 200, Name: "Test200"},
		}

		result := placeAllToUncategorized(output, tests)

		indices := result.Domains[0].Features[0].TestIndices
		if indices[0] != 100 || indices[1] != 50 || indices[2] != 200 {
			t.Errorf("expected indices [100, 50, 200], got %v", indices)
		}
	})

	t.Run("handles large number of tests", func(t *testing.T) {
		output := &specview.Phase1Output{}
		tests := make([]specview.TestInfo, 1000)
		for i := range tests {
			tests[i] = specview.TestInfo{Index: i, Name: "Test"}
		}

		result := placeAllToUncategorized(output, tests)

		if len(result.Domains[0].Features[0].TestIndices) != 1000 {
			t.Errorf("expected 1000 test indices, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
	})
}

func TestApplyPlacements_EdgeCases(t *testing.T) {
	t.Run("handles case sensitivity in domain name matching", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "auth", FeatureName: "Login", TestIndex: 1}, // lowercase domain
		}

		result := applyPlacements(output, placements)

		// Case mismatch should fallback to Uncategorized
		if len(result.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result.Domains))
		}
		if result.Domains[1].Name != UncategorizedName {
			t.Errorf("expected Uncategorized domain, got %q", result.Domains[1].Name)
		}
	})

	t.Run("handles case sensitivity in feature name matching", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "login", TestIndex: 1}, // lowercase feature
		}

		result := applyPlacements(output, placements)

		// Case mismatch should fallback to Uncategorized
		if len(result.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result.Domains))
		}
		if result.Domains[1].Name != UncategorizedName {
			t.Errorf("expected Uncategorized domain, got %q", result.Domains[1].Name)
		}
	})

	t.Run("places multiple tests to same feature in order", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login", TestIndices: []int{0}}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 1},
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 2},
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 3},
		}

		result := applyPlacements(output, placements)

		indices := result.Domains[0].Features[0].TestIndices
		if len(indices) != 4 {
			t.Fatalf("expected 4 test indices, got %d", len(indices))
		}
		expectedIndices := []int{0, 1, 2, 3}
		for i, expected := range expectedIndices {
			if indices[i] != expected {
				t.Errorf("expected index %d at position %d, got %d", expected, i, indices[i])
			}
		}
	})

	t.Run("handles all invalid placements", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "NonExistent1", FeatureName: "Feature1", TestIndex: 1},
			{DomainName: "NonExistent2", FeatureName: "Feature2", TestIndex: 2},
			{DomainName: "Auth", FeatureName: "NonExistent", TestIndex: 3},
		}

		result := applyPlacements(output, placements)

		// All should be in Uncategorized
		if len(result.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result.Domains))
		}
		uncatIndices := result.Domains[1].Features[0].TestIndices
		if len(uncatIndices) != 3 {
			t.Errorf("expected 3 tests in Uncategorized, got %d", len(uncatIndices))
		}
	})

	t.Run("preserves domain and feature metadata after placement", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name:        "Auth",
					Confidence:  0.95,
					Description: "Authentication domain",
					Features: []specview.FeatureGroup{
						{
							Name:        "Login",
							Confidence:  0.9,
							Description: "Login feature",
							TestIndices: []int{0},
						},
					},
				},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 1},
		}

		result := applyPlacements(output, placements)

		if result.Domains[0].Confidence != 0.95 {
			t.Errorf("expected domain confidence 0.95, got %f", result.Domains[0].Confidence)
		}
		if result.Domains[0].Description != "Authentication domain" {
			t.Errorf("expected domain description preserved, got %q", result.Domains[0].Description)
		}
		if result.Domains[0].Features[0].Confidence != 0.9 {
			t.Errorf("expected feature confidence 0.9, got %f", result.Domains[0].Features[0].Confidence)
		}
		if result.Domains[0].Features[0].Description != "Login feature" {
			t.Errorf("expected feature description preserved, got %q", result.Domains[0].Features[0].Description)
		}
	})

	t.Run("handles placements across multiple domains", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0}},
						{Name: "Logout", TestIndices: []int{1}},
					},
				},
				{
					Name: "Payment",
					Features: []specview.FeatureGroup{
						{Name: "Checkout", TestIndices: []int{2}},
						{Name: "Refund", TestIndices: []int{3}},
					},
				},
			},
		}
		placements := []specview.TestPlacement{
			{DomainName: "Auth", FeatureName: "Login", TestIndex: 10},
			{DomainName: "Auth", FeatureName: "Logout", TestIndex: 11},
			{DomainName: "Payment", FeatureName: "Checkout", TestIndex: 12},
			{DomainName: "Payment", FeatureName: "Refund", TestIndex: 13},
		}

		result := applyPlacements(output, placements)

		// Auth.Login: [0, 10]
		if len(result.Domains[0].Features[0].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Auth.Login, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
		// Auth.Logout: [1, 11]
		if len(result.Domains[0].Features[1].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Auth.Logout, got %d", len(result.Domains[0].Features[1].TestIndices))
		}
		// Payment.Checkout: [2, 12]
		if len(result.Domains[1].Features[0].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Payment.Checkout, got %d", len(result.Domains[1].Features[0].TestIndices))
		}
		// Payment.Refund: [3, 13]
		if len(result.Domains[1].Features[1].TestIndices) != 2 {
			t.Errorf("expected 2 tests in Payment.Refund, got %d", len(result.Domains[1].Features[1].TestIndices))
		}
	})

	t.Run("uses cached uncategorized indices efficiently", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{Name: "Auth", Features: []specview.FeatureGroup{{Name: "Login"}}},
			},
		}
		// All placements are invalid, should all go to same Uncategorized
		placements := []specview.TestPlacement{
			{DomainName: "X", FeatureName: "Y", TestIndex: 1},
			{DomainName: "A", FeatureName: "B", TestIndex: 2},
			{DomainName: "C", FeatureName: "D", TestIndex: 3},
		}

		result := applyPlacements(output, placements)

		// Should only have one Uncategorized domain created
		if len(result.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result.Domains))
		}
		if result.Domains[1].Name != UncategorizedName {
			t.Errorf("expected second domain to be Uncategorized")
		}
		if len(result.Domains[1].Features) != 1 {
			t.Errorf("expected 1 feature in Uncategorized, got %d", len(result.Domains[1].Features))
		}
		if len(result.Domains[1].Features[0].TestIndices) != 3 {
			t.Errorf("expected 3 tests in Uncategorized, got %d", len(result.Domains[1].Features[0].TestIndices))
		}
	})
}
