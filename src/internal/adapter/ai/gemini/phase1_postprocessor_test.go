package gemini

import (
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestValidateNoUncategorized(t *testing.T) {
	tests := []struct {
		name           string
		results        []v3BatchResult
		wantViolations int
	}{
		{
			name: "no violations",
			results: []v3BatchResult{
				{Domain: "Authentication", Feature: "Login"},
				{Domain: "Navigation", Feature: "Routing"},
			},
			wantViolations: 0,
		},
		{
			name: "uncategorized domain",
			results: []v3BatchResult{
				{Domain: "Authentication", Feature: "Login"},
				{Domain: "Uncategorized", Feature: "General"},
			},
			wantViolations: 1,
		},
		{
			name: "general feature only",
			results: []v3BatchResult{
				{Domain: "Authentication", Feature: "General"},
			},
			wantViolations: 1,
		},
		{
			name: "other feature",
			results: []v3BatchResult{
				{Domain: "Forms", Feature: "Other"},
			},
			wantViolations: 1,
		},
		{
			name: "multiple violations",
			results: []v3BatchResult{
				{Domain: "Uncategorized", Feature: "General"},
				{Domain: "Other", Feature: "Misc"},
				{Domain: "Authentication", Feature: "Login"},
			},
			wantViolations: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			violations := validateNoUncategorized(tc.results)
			if len(violations) != tc.wantViolations {
				t.Errorf("got %d violations, want %d", len(violations), tc.wantViolations)
			}
			for _, v := range violations {
				if v.Type != ViolationUncategorized {
					t.Errorf("got violation type %v, want %v", v.Type, ViolationUncategorized)
				}
			}
		})
	}
}

func TestFindOrphanedTests(t *testing.T) {
	tests := []struct {
		name           string
		results        []v3BatchResult
		testsCount     int
		wantViolations int
	}{
		{
			name: "matching counts",
			results: []v3BatchResult{
				{Domain: "Auth", Feature: "Login"},
				{Domain: "Auth", Feature: "Logout"},
			},
			testsCount:     2,
			wantViolations: 0,
		},
		{
			name: "fewer results than tests",
			results: []v3BatchResult{
				{Domain: "Auth", Feature: "Login"},
			},
			testsCount:     3,
			wantViolations: 1,
		},
		{
			name: "more results than tests",
			results: []v3BatchResult{
				{Domain: "Auth", Feature: "Login"},
				{Domain: "Auth", Feature: "Logout"},
				{Domain: "Auth", Feature: "Session"},
			},
			testsCount:     2,
			wantViolations: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testList := make([]specview.TestForAssignment, tc.testsCount)
			violations := findOrphanedTests(tc.results, testList)
			if len(violations) != tc.wantViolations {
				t.Errorf("got %d violations, want %d", len(violations), tc.wantViolations)
			}
		})
	}
}

func TestNormalizeDomains(t *testing.T) {
	tests := []struct {
		name    string
		results []v3BatchResult
		want    map[string]string
	}{
		{
			name: "auth variants merged",
			results: []v3BatchResult{
				{Domain: "Authentication", Feature: "Login"},
				{Domain: "Auth", Feature: "Logout"},
				{Domain: "auth", Feature: "Session"},
			},
			want: map[string]string{
				"Login":   "Authentication",
				"Logout":  "Authentication",
				"Session": "Authentication",
			},
		},
		{
			name: "no merging needed",
			results: []v3BatchResult{
				{Domain: "Navigation", Feature: "Routing"},
				{Domain: "Forms", Feature: "Validation"},
			},
			want: map[string]string{
				"Routing":    "Navigation",
				"Validation": "Forms",
			},
		},
		{
			name:    "empty results",
			results: []v3BatchResult{},
			want:    map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalized := normalizeDomains(tc.results)
			if len(normalized) != len(tc.results) {
				t.Errorf("result count mismatch: got %d, want %d", len(normalized), len(tc.results))
			}
			for i, r := range normalized {
				wantDomain := tc.want[tc.results[i].Feature]
				if r.Domain != wantDomain {
					t.Errorf("result[%d].Domain = %q, want %q", i, r.Domain, wantDomain)
				}
			}
		})
	}
}

func TestDeriveDomainFromPath(t *testing.T) {
	tests := []struct {
		path        string
		wantDomain  string
		wantFeature string
	}{
		{
			path:        "",
			wantDomain:  "Project Root",
			wantFeature: "General Tests",
		},
		{
			path:        "test.spec.ts",
			wantDomain:  "Project Root",
			wantFeature: "General Tests",
		},
		{
			path:        "src/auth/login.test.ts",
			wantDomain:  "Auth",
			wantFeature: "Core", // Single significant part -> Core
		},
		{
			path:        "src/components/Button/Button.test.tsx",
			wantDomain:  "Components",
			wantFeature: "Button", // Last significant directory segment
		},
		{
			path:        "__tests__/utils/helpers.test.ts",
			wantDomain:  "Utils",
			wantFeature: "Core", // Single significant part (utils) -> Core
		},
		{
			path:        "packages/core/src/api/client.test.ts",
			wantDomain:  "Core",
			wantFeature: "Api", // [packages,core,src,api] -> significant=[core,api] -> feature=Api
		},
		{
			path:        "tests/integration/auth-flow.test.ts",
			wantDomain:  "Integration",
			wantFeature: "Core", // [tests,integration] -> significant=[integration] -> single -> Core
		},
		{
			path:        "lib/database/connection.test.ts",
			wantDomain:  "Database",
			wantFeature: "Core", // [lib,database] -> significant=[database] -> single -> Core
		},
		{
			path:        "src/user-management/profile/settings.test.ts",
			wantDomain:  "User Management",
			wantFeature: "Profile", // [src,user-management,profile] -> significant=[user-management,profile] -> feature=Profile
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			domain, feature := deriveDomainFromPath(tc.path)
			if domain != tc.wantDomain {
				t.Errorf("domain = %q, want %q", domain, tc.wantDomain)
			}
			if feature != tc.wantFeature {
				t.Errorf("feature = %q, want %q", feature, tc.wantFeature)
			}
		})
	}
}

func TestAreSimilarDomains(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"auth", "authentication", true},
		{"authn", "authentication", true},
		{"config", "configuration", true},
		{"nav", "navigation", true},
		{"util", "utilities", true},
		{"utils", "utilities", true},
		{"db", "database", true},
		{"doc", "documentation", true},
		{"docs", "documentation", true},
		{"authentication", "authentication", true},
		{"navigation", "forms", false},
		{"auth", "authorization", false},
	}

	for _, tc := range tests {
		t.Run(tc.a+"_"+tc.b, func(t *testing.T) {
			result := areSimilarDomains(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("areSimilarDomains(%q, %q) = %v, want %v", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestReplaceUncategorizedWithPathDomains(t *testing.T) {
	tests := []specview.TestForAssignment{
		{FilePath: "src/auth/login.test.ts", Index: 0, Name: "should login"},
		{FilePath: "src/forms/validation.test.ts", Index: 1, Name: "should validate"},
	}

	results := []v3BatchResult{
		{Domain: "Uncategorized", Feature: "General"},
		{Domain: "Forms", Feature: "Validation"},
	}

	replaced := replaceUncategorizedWithPathDomains(results, tests)

	if replaced[0].Domain != "Auth" {
		t.Errorf("expected domain 'Auth', got %q", replaced[0].Domain)
	}
	// src/auth -> significant=[auth] -> single element -> feature=Core
	if replaced[0].Feature != "Core" {
		t.Errorf("expected feature 'Core', got %q", replaced[0].Feature)
	}
	if replaced[1].Domain != "Forms" {
		t.Errorf("expected domain 'Forms' unchanged, got %q", replaced[1].Domain)
	}
}

func TestCreateDomainsFromPaths(t *testing.T) {
	tests := []specview.TestForAssignment{
		{FilePath: "src/auth/login.test.ts", Index: 0, Name: "should login"},
		{FilePath: "src/auth/logout.test.ts", Index: 1, Name: "should logout"},
		{FilePath: "src/forms/input.test.ts", Index: 2, Name: "should handle input"},
	}

	results := createDomainsFromPaths(tests)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Domain != "Auth" {
		t.Errorf("results[0].Domain = %q, want 'Auth'", results[0].Domain)
	}
	// src/auth -> significant=[auth] -> single element -> feature=Core
	if results[0].Feature != "Core" {
		t.Errorf("results[0].Feature = %q, want 'Core'", results[0].Feature)
	}
	if results[2].Domain != "Forms" {
		t.Errorf("results[2].Domain = %q, want 'Forms'", results[2].Domain)
	}
}

func TestPhase1PostProcessor_Process(t *testing.T) {
	t.Run("transforms uncategorized to path-derived", func(t *testing.T) {
		pp := NewPhase1PostProcessor(DefaultPostProcessorConfig())

		results := []v3BatchResult{
			{Domain: "Authentication", Feature: "Login"},
			{Domain: "Uncategorized", Feature: "General"},
		}
		tests := []specview.TestForAssignment{
			{FilePath: "src/auth/login.test.ts", Index: 0},
			{FilePath: "src/forms/validation.test.ts", Index: 1},
		}

		processed, violations := pp.Process(results, tests)

		if len(violations) != 1 {
			t.Errorf("expected 1 violation, got %d", len(violations))
		}
		if violations[0].Type != ViolationUncategorized {
			t.Errorf("expected uncategorized violation, got %v", violations[0].Type)
		}
		if processed[1].Domain != "Forms" {
			t.Errorf("expected path-derived domain 'Forms', got %q", processed[1].Domain)
		}
	})

	t.Run("no violations for clean results", func(t *testing.T) {
		pp := NewPhase1PostProcessor(DefaultPostProcessorConfig())

		results := []v3BatchResult{
			{Domain: "Authentication", Feature: "Login"},
			{Domain: "Navigation", Feature: "Routing"},
		}
		tests := []specview.TestForAssignment{
			{FilePath: "src/auth/login.test.ts", Index: 0},
			{FilePath: "src/nav/routes.test.ts", Index: 1},
		}

		processed, violations := pp.Process(results, tests)

		if len(violations) != 0 {
			t.Errorf("expected no violations, got %d", len(violations))
		}
		if len(processed) != 2 {
			t.Errorf("expected 2 processed results, got %d", len(processed))
		}
	})

	t.Run("disabled prohibition allows uncategorized", func(t *testing.T) {
		config := PostProcessorConfig{ProhibitUncategorized: false}
		pp := NewPhase1PostProcessor(config)

		results := []v3BatchResult{
			{Domain: "Uncategorized", Feature: "General"},
		}
		tests := []specview.TestForAssignment{
			{FilePath: "src/test.ts", Index: 0},
		}

		_, violations := pp.Process(results, tests)

		// No uncategorized violation since prohibition is disabled
		uncatViolations := 0
		for _, v := range violations {
			if v.Type == ViolationUncategorized {
				uncatViolations++
			}
		}
		if uncatViolations != 0 {
			t.Errorf("expected no uncategorized violations when disabled, got %d", uncatViolations)
		}
	})
}

func TestFormatDomainName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"auth", "Auth"},
		{"user-management", "User Management"},
		{"user_profile", "User Profile"},
		{"API", "Api"},
		{"", "Unknown"},
		{"hello-world_test", "Hello World Test"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := formatDomainName(tc.input)
			if got != tc.want {
				t.Errorf("formatDomainName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

