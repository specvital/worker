package mock

import (
	"context"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestProvider_ClassifyDomains(t *testing.T) {
	provider := NewProvider()

	t.Run("empty input returns empty domains", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Files:      []specview.FileInfo{},
			Language:   "English",
		}

		output, usage, err := provider.ClassifyDomains(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage != nil {
			t.Error("expected nil token usage for mock provider")
		}
		if len(output.Domains) != 0 {
			t.Errorf("expected 0 domains, got %d", len(output.Domains))
		}
	})

	t.Run("single file creates default domain", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Files: []specview.FileInfo{
				{
					Path:      "test/auth_test.go",
					Framework: "go",
					Tests: []specview.TestInfo{
						{Index: 0, Name: "TestLogin"},
						{Index: 1, Name: "TestLogout"},
					},
				},
			},
			Language: "English",
		}

		output, _, err := provider.ClassifyDomains(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(output.Domains) == 0 {
			t.Fatal("expected at least 1 domain")
		}

		domain := output.Domains[0]
		if domain.Confidence != defaultConfidence {
			t.Errorf("expected confidence %v, got %v", defaultConfidence, domain.Confidence)
		}
		if len(domain.Features) == 0 {
			t.Error("expected at least 1 feature")
		}
	})

	t.Run("multiple files create separate domains by directory", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Files: []specview.FileInfo{
				{
					Path: "src/auth/login_test.go",
					Tests: []specview.TestInfo{
						{Index: 0, Name: "TestLogin"},
					},
				},
				{
					Path: "src/user/profile_test.go",
					Tests: []specview.TestInfo{
						{Index: 1, Name: "TestProfile"},
					},
				},
			},
			Language: "English",
		}

		output, _, err := provider.ClassifyDomains(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(output.Domains) < 2 {
			t.Errorf("expected at least 2 domains for different directories, got %d", len(output.Domains))
		}
	})

	t.Run("Korean language returns Korean domain names", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Files: []specview.FileInfo{
				{
					Path: "test_test.go",
					Tests: []specview.TestInfo{
						{Index: 0, Name: "TestSomething"},
					},
				},
			},
			Language: "Korean",
		}

		output, _, err := provider.ClassifyDomains(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(output.Domains) == 0 {
			t.Fatal("expected at least 1 domain")
		}

		domain := output.Domains[0]
		if domain.Name == "" {
			t.Error("domain name should not be empty")
		}
	})
}

func TestProvider_ConvertTestNames(t *testing.T) {
	provider := NewProvider()

	t.Run("converts test names to behaviors", func(t *testing.T) {
		input := specview.Phase2Input{
			DomainContext: "Authentication",
			FeatureName:   "Login",
			Language:      "English",
			Tests: []specview.TestForConversion{
				{Index: 0, Name: "TestLoginSuccess"},
				{Index: 1, Name: "TestLoginFailure"},
			},
		}

		output, usage, err := provider.ConvertTestNames(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage != nil {
			t.Error("expected nil token usage for mock provider")
		}
		if len(output.Behaviors) != 2 {
			t.Fatalf("expected 2 behaviors, got %d", len(output.Behaviors))
		}

		for i, behavior := range output.Behaviors {
			if behavior.TestIndex != input.Tests[i].Index {
				t.Errorf("behavior %d: expected index %d, got %d", i, input.Tests[i].Index, behavior.TestIndex)
			}
			if behavior.Description == "" {
				t.Errorf("behavior %d: description should not be empty", i)
			}
			if behavior.Confidence != defaultConfidence {
				t.Errorf("behavior %d: expected confidence %v, got %v", i, defaultConfidence, behavior.Confidence)
			}
		}
	})

	t.Run("Korean language returns Korean descriptions", func(t *testing.T) {
		input := specview.Phase2Input{
			DomainContext: "인증",
			FeatureName:   "로그인",
			Language:      "Korean",
			Tests: []specview.TestForConversion{
				{Index: 0, Name: "TestLogin"},
			},
		}

		output, _, err := provider.ConvertTestNames(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(output.Behaviors) != 1 {
			t.Fatal("expected 1 behavior")
		}

		behavior := output.Behaviors[0]
		if behavior.Description == "" {
			t.Error("description should not be empty")
		}
	})

	t.Run("empty tests returns empty behaviors", func(t *testing.T) {
		input := specview.Phase2Input{
			DomainContext: "Test",
			FeatureName:   "Feature",
			Language:      "English",
			Tests:         []specview.TestForConversion{},
		}

		output, _, err := provider.ConvertTestNames(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(output.Behaviors) != 0 {
			t.Errorf("expected 0 behaviors, got %d", len(output.Behaviors))
		}
	})
}

func TestProvider_Close(t *testing.T) {
	provider := NewProvider()
	err := provider.Close()
	if err != nil {
		t.Errorf("Close should return nil, got %v", err)
	}
}

func TestCamelCaseToReadable(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"TestLoginSuccess", "Login Success"},
		{"TestUserProfile", "User Profile"},
		{"test_login_success", "login success"},
		{"TestHTTPClient", "H T T P Client"},
		{"Test", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := camelCaseToReadable(tt.input)
			if got != tt.expected {
				t.Errorf("camelCaseToReadable(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
