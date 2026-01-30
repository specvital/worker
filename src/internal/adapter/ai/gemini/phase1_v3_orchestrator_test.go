package gemini

import (
	"testing"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/domain/specview"
)

func TestFlattenTests_Empty(t *testing.T) {
	result := flattenTests(nil)

	if len(result) != 0 {
		t.Errorf("expected 0 tests, got %d", len(result))
	}
}

func TestFlattenTests_SingleFile(t *testing.T) {
	files := []specview.FileInfo{
		{
			Path: "auth/login_test.ts",
			Tests: []specview.TestInfo{
				{Index: 0, Name: "should login", SuitePath: "LoginPage"},
				{Index: 1, Name: "should logout", SuitePath: "LoginPage"},
			},
		},
	}

	result := flattenTests(files)

	if len(result) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(result))
	}
	if result[0].FilePath != "auth/login_test.ts" {
		t.Errorf("expected file path 'auth/login_test.ts', got %q", result[0].FilePath)
	}
	if result[0].Name != "should login" {
		t.Errorf("expected name 'should login', got %q", result[0].Name)
	}
	if result[0].SuitePath != "LoginPage" {
		t.Errorf("expected suite 'LoginPage', got %q", result[0].SuitePath)
	}
	if result[0].Index != 0 {
		t.Errorf("expected index 0, got %d", result[0].Index)
	}
	if result[1].Index != 1 {
		t.Errorf("expected index 1, got %d", result[1].Index)
	}
}

func TestFlattenTests_MultipleFiles(t *testing.T) {
	files := []specview.FileInfo{
		{
			Path:  "auth/login_test.ts",
			Tests: []specview.TestInfo{{Index: 0, Name: "test1"}},
		},
		{
			Path:  "payment/checkout_test.ts",
			Tests: []specview.TestInfo{{Index: 1, Name: "test2"}, {Index: 2, Name: "test3"}},
		},
		{
			Path:  "user/profile_test.ts",
			Tests: []specview.TestInfo{{Index: 3, Name: "test4"}},
		},
	}

	result := flattenTests(files)

	if len(result) != 4 {
		t.Fatalf("expected 4 tests, got %d", len(result))
	}
	if result[0].FilePath != "auth/login_test.ts" {
		t.Errorf("expected first file 'auth/login_test.ts', got %q", result[0].FilePath)
	}
	if result[1].FilePath != "payment/checkout_test.ts" {
		t.Errorf("expected second file 'payment/checkout_test.ts', got %q", result[1].FilePath)
	}
	if result[3].FilePath != "user/profile_test.ts" {
		t.Errorf("expected fourth file 'user/profile_test.ts', got %q", result[3].FilePath)
	}
}

func TestSplitIntoBatches_Empty(t *testing.T) {
	result := splitIntoBatches(nil, 20)

	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestSplitIntoBatches_SingleBatch(t *testing.T) {
	tests := makeTestsForAssignment(15)

	result := splitIntoBatches(tests, 20)

	if len(result) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(result))
	}
	if len(result[0]) != 15 {
		t.Errorf("expected 15 tests in batch, got %d", len(result[0]))
	}
}

func TestSplitIntoBatches_ExactMultiple(t *testing.T) {
	tests := makeTestsForAssignment(40)

	result := splitIntoBatches(tests, 20)

	if len(result) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(result))
	}
	if len(result[0]) != 20 {
		t.Errorf("expected 20 tests in first batch, got %d", len(result[0]))
	}
	if len(result[1]) != 20 {
		t.Errorf("expected 20 tests in second batch, got %d", len(result[1]))
	}
}

func TestSplitIntoBatches_PartialLast(t *testing.T) {
	tests := makeTestsForAssignment(55)

	result := splitIntoBatches(tests, 20)

	if len(result) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(result))
	}
	if len(result[0]) != 20 {
		t.Errorf("expected 20 tests in first batch, got %d", len(result[0]))
	}
	if len(result[1]) != 20 {
		t.Errorf("expected 20 tests in second batch, got %d", len(result[1]))
	}
	if len(result[2]) != 15 {
		t.Errorf("expected 15 tests in last batch, got %d", len(result[2]))
	}
}

func TestSplitIntoBatches_DefaultBatchSize(t *testing.T) {
	tests := makeTestsForAssignment(25)

	result := splitIntoBatches(tests, 0)

	// Default batch size is v3BatchSize (20)
	if len(result) != 2 {
		t.Fatalf("expected 2 batches with default size, got %d", len(result))
	}
	if len(result[0]) != 20 {
		t.Errorf("expected 20 tests in first batch, got %d", len(result[0]))
	}
	if len(result[1]) != 5 {
		t.Errorf("expected 5 tests in second batch, got %d", len(result[1]))
	}
}

func TestExtractDomainSummaries_Empty(t *testing.T) {
	result := extractDomainSummaries(nil, nil)

	if len(result) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(result))
	}
}

func TestExtractDomainSummaries_NewDomains(t *testing.T) {
	results := []v3BatchResult{
		{Domain: "Authentication", Feature: "Login"},
		{Domain: "Authentication", Feature: "Logout"},
		{Domain: "Payment", Feature: "Checkout"},
	}

	summaries := extractDomainSummaries(results, nil)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(summaries))
	}

	domainMap := make(map[string]prompt.DomainSummary)
	for _, s := range summaries {
		domainMap[s.Name] = s
	}

	authDomain, ok := domainMap["Authentication"]
	if !ok {
		t.Fatal("expected Authentication domain")
	}
	if len(authDomain.Features) != 2 {
		t.Errorf("expected 2 features in Authentication, got %d", len(authDomain.Features))
	}

	paymentDomain, ok := domainMap["Payment"]
	if !ok {
		t.Fatal("expected Payment domain")
	}
	if len(paymentDomain.Features) != 1 {
		t.Errorf("expected 1 feature in Payment, got %d", len(paymentDomain.Features))
	}
}

func TestExtractDomainSummaries_MergeWithExisting(t *testing.T) {
	existing := []prompt.DomainSummary{
		{Name: "Authentication", Features: []string{"Login"}},
	}
	results := []v3BatchResult{
		{Domain: "Authentication", Feature: "Register"},
		{Domain: "Payment", Feature: "Refund"},
	}

	summaries := extractDomainSummaries(results, existing)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(summaries))
	}

	domainMap := make(map[string]prompt.DomainSummary)
	for _, s := range summaries {
		domainMap[s.Name] = s
	}

	authDomain := domainMap["Authentication"]
	if len(authDomain.Features) != 2 {
		t.Errorf("expected 2 features after merge, got %d", len(authDomain.Features))
	}

	hasLogin := false
	hasRegister := false
	for _, f := range authDomain.Features {
		if f == "Login" {
			hasLogin = true
		}
		if f == "Register" {
			hasRegister = true
		}
	}
	if !hasLogin || !hasRegister {
		t.Errorf("expected Login and Register features, got %v", authDomain.Features)
	}
}

func TestExtractDomainSummaries_NoDuplicateFeatures(t *testing.T) {
	results := []v3BatchResult{
		{Domain: "Auth", Feature: "Login"},
		{Domain: "Auth", Feature: "Login"}, // duplicate
		{Domain: "Auth", Feature: "Login"}, // duplicate
	}

	summaries := extractDomainSummaries(results, nil)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(summaries))
	}
	if len(summaries[0].Features) != 1 {
		t.Errorf("expected 1 unique feature, got %d", len(summaries[0].Features))
	}
}

func TestMergeV3Results_Empty(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:  "test.ts",
				Tests: []specview.TestInfo{{Index: 0, Name: "test"}},
			},
		},
	}

	result := mergeV3Results(nil, input)

	if len(result.Domains) != 1 {
		t.Fatalf("expected 1 fallback domain, got %d", len(result.Domains))
	}
	if result.Domains[0].Name != uncategorizedDomainName {
		t.Errorf("expected %q domain, got %q", uncategorizedDomainName, result.Domains[0].Name)
	}
}

func TestMergeV3Results_SingleBatch(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "auth/login_test.ts",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "should login"},
					{Index: 1, Name: "should logout"},
				},
			},
		},
	}
	results := [][]v3BatchResult{
		{
			{Domain: "Authentication", Feature: "Login"},
			{Domain: "Authentication", Feature: "Session"},
		},
	}

	output := mergeV3Results(results, input)

	if len(output.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(output.Domains))
	}
	if output.Domains[0].Name != "Authentication" {
		t.Errorf("expected 'Authentication' domain, got %q", output.Domains[0].Name)
	}
	if len(output.Domains[0].Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(output.Domains[0].Features))
	}
}

func TestMergeV3Results_MultipleBatches(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.ts",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "test1"},
					{Index: 1, Name: "test2"},
					{Index: 2, Name: "test3"},
				},
			},
		},
	}
	results := [][]v3BatchResult{
		{{Domain: "Auth", Feature: "Login"}},
		{{Domain: "Payment", Feature: "Checkout"}},
		{{Domain: "Auth", Feature: "Login"}}, // Same domain/feature
	}

	output := mergeV3Results(results, input)

	// Should have 2 domains: Auth and Payment
	if len(output.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(output.Domains))
	}

	// Find Auth domain
	var authDomain *specview.DomainGroup
	for i := range output.Domains {
		if output.Domains[i].Name == "Auth" {
			authDomain = &output.Domains[i]
			break
		}
	}
	if authDomain == nil {
		t.Fatal("expected Auth domain")
	}

	// Auth/Login should have 2 test indices (0 and 2)
	var loginFeature *specview.FeatureGroup
	for i := range authDomain.Features {
		if authDomain.Features[i].Name == "Login" {
			loginFeature = &authDomain.Features[i]
			break
		}
	}
	if loginFeature == nil {
		t.Fatal("expected Login feature")
	}
	if len(loginFeature.TestIndices) != 2 {
		t.Errorf("expected 2 test indices for Login, got %d", len(loginFeature.TestIndices))
	}
}

func TestMergeV3Results_PreservesTestIndices(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.ts",
				Tests: []specview.TestInfo{
					{Index: 10, Name: "test1"}, // Non-zero index
					{Index: 20, Name: "test2"},
					{Index: 30, Name: "test3"},
				},
			},
		},
	}
	results := [][]v3BatchResult{
		{
			{Domain: "Auth", Feature: "Login"},
			{Domain: "Auth", Feature: "Login"},
			{Domain: "Auth", Feature: "Login"},
		},
	}

	output := mergeV3Results(results, input)

	indices := output.Domains[0].Features[0].TestIndices
	if len(indices) != 3 {
		t.Fatalf("expected 3 indices, got %d", len(indices))
	}

	// Should preserve original indices
	expectedIndices := map[int]bool{10: true, 20: true, 30: true}
	for _, idx := range indices {
		if !expectedIndices[idx] {
			t.Errorf("unexpected index %d", idx)
		}
	}
}

func TestMergeV3Results_Confidence(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:  "test.ts",
				Tests: []specview.TestInfo{{Index: 0, Name: "test"}},
			},
		},
	}
	results := [][]v3BatchResult{
		{{Domain: "Auth", Feature: "Login"}},
	}

	output := mergeV3Results(results, input)

	if output.Domains[0].Confidence != defaultClassificationConfidence {
		t.Errorf("expected domain confidence %v, got %v", defaultClassificationConfidence, output.Domains[0].Confidence)
	}
	if output.Domains[0].Features[0].Confidence != defaultClassificationConfidence {
		t.Errorf("expected feature confidence %v, got %v", defaultClassificationConfidence, output.Domains[0].Features[0].Confidence)
	}
}

func TestV3BatchConstants(t *testing.T) {
	// Verify batch size is 20 as per plan
	if v3BatchSize != 20 {
		t.Errorf("expected v3BatchSize=20, got %d", v3BatchSize)
	}
}
