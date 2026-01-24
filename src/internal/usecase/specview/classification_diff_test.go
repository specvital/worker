package specview

import (
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestCalculateTestDiff(t *testing.T) {
	t.Run("no changes returns empty diff", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
					{Index: 1, Name: "TestLogout", SuitePath: "AuthSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"):  {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogout"): {DomainIndex: 0, FeatureIndex: 1, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 0 {
			t.Errorf("expected 0 new tests, got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 0 {
			t.Errorf("expected 0 deleted tests, got %d", len(diff.DeletedTests))
		}
	})

	t.Run("additions only returns new tests", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
					{Index: 1, Name: "TestLogout", SuitePath: "AuthSuite"},
					{Index: 2, Name: "Test2FA", SuitePath: "AuthSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"):  {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogout"): {DomainIndex: 0, FeatureIndex: 1, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 1 {
			t.Errorf("expected 1 new test, got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 0 {
			t.Errorf("expected 0 deleted tests, got %d", len(diff.DeletedTests))
		}
		if diff.NewTests[0].Name != "Test2FA" {
			t.Errorf("expected new test name 'Test2FA', got %q", diff.NewTests[0].Name)
		}
	})

	t.Run("deletions only returns deleted tests", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"):  {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogout"): {DomainIndex: 0, FeatureIndex: 1, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 0 {
			t.Errorf("expected 0 new tests, got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 1 {
			t.Errorf("expected 1 deleted test, got %d", len(diff.DeletedTests))
		}
	})

	t.Run("additions and deletions combined", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
					{Index: 2, Name: "Test2FA", SuitePath: "AuthSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"):  {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogout"): {DomainIndex: 0, FeatureIndex: 1, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 1 {
			t.Errorf("expected 1 new test, got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 1 {
			t.Errorf("expected 1 deleted test, got %d", len(diff.DeletedTests))
		}
	})

	t.Run("nil cache map treated as empty", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
				},
			},
		}

		diff := CalculateTestDiff(nil, files)

		if len(diff.NewTests) != 1 {
			t.Errorf("expected 1 new test, got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 0 {
			t.Errorf("expected 0 deleted tests, got %d", len(diff.DeletedTests))
		}
	})

	t.Run("empty current files with cached data", func(t *testing.T) {
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"): {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, []specview.FileInfo{})

		if len(diff.NewTests) != 0 {
			t.Errorf("expected 0 new tests, got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 1 {
			t.Errorf("expected 1 deleted test, got %d", len(diff.DeletedTests))
		}
	})

	t.Run("test renamed counts as delete and add", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLoginWithPassword", SuitePath: "AuthSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"): {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 1 {
			t.Errorf("expected 1 new test (renamed), got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 1 {
			t.Errorf("expected 1 deleted test (old name), got %d", len(diff.DeletedTests))
		}
	})

	t.Run("test moved to different suite counts as delete and add", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "NewSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"): {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 1 {
			t.Errorf("expected 1 new test (moved suite), got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 1 {
			t.Errorf("expected 1 deleted test (old suite), got %d", len(diff.DeletedTests))
		}
	})

	t.Run("test moved to different file counts as delete and add", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_v2_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"): {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		if len(diff.NewTests) != 1 {
			t.Errorf("expected 1 new test (moved file), got %d", len(diff.NewTests))
		}
		if len(diff.DeletedTests) != 1 {
			t.Errorf("expected 1 deleted test (old file), got %d", len(diff.DeletedTests))
		}
	})

	t.Run("multiple files with mixed changes", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Path: "test/auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin", SuitePath: "AuthSuite"},
					{Index: 3, Name: "TestPasswordReset", SuitePath: "AuthSuite"},
				},
			},
			{
				Path: "test/payment_test.go",
				Tests: []specview.TestInfo{
					{Index: 4, Name: "TestCheckout", SuitePath: "PaymentSuite"},
				},
			},
		}
		cachedMap := map[string]specview.TestIdentity{
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogin"):    {DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
			specview.TestKey("test/auth_test.go", "AuthSuite", "TestLogout"):   {DomainIndex: 0, FeatureIndex: 1, TestIndex: 0},
			specview.TestKey("test/user_test.go", "UserSuite", "TestRegister"): {DomainIndex: 1, FeatureIndex: 0, TestIndex: 0},
		}

		diff := CalculateTestDiff(cachedMap, files)

		// New: TestPasswordReset, TestCheckout
		if len(diff.NewTests) != 2 {
			t.Errorf("expected 2 new tests, got %d", len(diff.NewTests))
		}
		// Deleted: TestLogout, TestRegister
		if len(diff.DeletedTests) != 2 {
			t.Errorf("expected 2 deleted tests, got %d", len(diff.DeletedTests))
		}
	})
}

func TestRemoveDeletedTestIndices(t *testing.T) {
	t.Run("removes deleted test indices", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0, 1, 2}},
					},
				},
			},
		}
		deleted := []specview.TestIdentity{
			{DomainIndex: 0, FeatureIndex: 0, TestIndex: 1},
		}

		result := RemoveDeletedTestIndices(output, deleted)

		if len(result.Domains[0].Features[0].TestIndices) != 2 {
			t.Errorf("expected 2 test indices, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
		// Should have indices 0 and 2
		if result.Domains[0].Features[0].TestIndices[0] != 0 {
			t.Errorf("expected first index 0, got %d", result.Domains[0].Features[0].TestIndices[0])
		}
		if result.Domains[0].Features[0].TestIndices[1] != 2 {
			t.Errorf("expected second index 2, got %d", result.Domains[0].Features[0].TestIndices[1])
		}
	})

	t.Run("removes empty features after deletion", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0}},
						{Name: "Logout", TestIndices: []int{1}},
					},
				},
			},
		}
		deleted := []specview.TestIdentity{
			{DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		result := RemoveDeletedTestIndices(output, deleted)

		if len(result.Domains[0].Features) != 1 {
			t.Errorf("expected 1 feature, got %d", len(result.Domains[0].Features))
		}
		if result.Domains[0].Features[0].Name != "Logout" {
			t.Errorf("expected 'Logout' feature, got %q", result.Domains[0].Features[0].Name)
		}
	})

	t.Run("removes empty domains after deletion", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0}},
					},
				},
				{
					Name: "User",
					Features: []specview.FeatureGroup{
						{Name: "Profile", TestIndices: []int{1}},
					},
				},
			},
		}
		deleted := []specview.TestIdentity{
			{DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		result := RemoveDeletedTestIndices(output, deleted)

		if len(result.Domains) != 1 {
			t.Errorf("expected 1 domain, got %d", len(result.Domains))
		}
		if result.Domains[0].Name != "User" {
			t.Errorf("expected 'User' domain, got %q", result.Domains[0].Name)
		}
	})

	t.Run("nil output returns nil", func(t *testing.T) {
		result := RemoveDeletedTestIndices(nil, []specview.TestIdentity{})

		if result != nil {
			t.Error("expected nil result for nil output")
		}
	})

	t.Run("empty deleted list returns same output", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0}},
					},
				},
			},
		}

		result := RemoveDeletedTestIndices(output, []specview.TestIdentity{})

		if result != output {
			t.Error("expected same output reference for empty deleted list")
		}
	})

	t.Run("preserves domain and feature metadata", func(t *testing.T) {
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
							TestIndices: []int{0, 1},
						},
					},
				},
			},
		}
		deleted := []specview.TestIdentity{
			{DomainIndex: 0, FeatureIndex: 0, TestIndex: 0},
		}

		result := RemoveDeletedTestIndices(output, deleted)

		if result.Domains[0].Confidence != 0.95 {
			t.Errorf("expected confidence 0.95, got %f", result.Domains[0].Confidence)
		}
		if result.Domains[0].Description != "Authentication domain" {
			t.Errorf("expected description preserved, got %q", result.Domains[0].Description)
		}
		if result.Domains[0].Features[0].Confidence != 0.9 {
			t.Errorf("expected feature confidence 0.9, got %f", result.Domains[0].Features[0].Confidence)
		}
	})

	t.Run("multiple deletions across domains", func(t *testing.T) {
		output := &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Name: "Auth",
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0, 1}},
						{Name: "Logout", TestIndices: []int{2, 3}},
					},
				},
				{
					Name: "User",
					Features: []specview.FeatureGroup{
						{Name: "Profile", TestIndices: []int{4, 5}},
					},
				},
			},
		}
		deleted := []specview.TestIdentity{
			{DomainIndex: 0, FeatureIndex: 0, TestIndex: 1},
			{DomainIndex: 0, FeatureIndex: 1, TestIndex: 0},
			{DomainIndex: 1, FeatureIndex: 0, TestIndex: 0},
		}

		result := RemoveDeletedTestIndices(output, deleted)

		// Auth.Login: was [0,1], deleted index 1 -> [0]
		if len(result.Domains[0].Features[0].TestIndices) != 1 {
			t.Errorf("expected 1 test in Auth.Login, got %d", len(result.Domains[0].Features[0].TestIndices))
		}
		// Auth.Logout: was [2,3], deleted index 0 -> [3]
		if len(result.Domains[0].Features[1].TestIndices) != 1 {
			t.Errorf("expected 1 test in Auth.Logout, got %d", len(result.Domains[0].Features[1].TestIndices))
		}
		// User.Profile: was [4,5], deleted index 0 -> [5]
		if len(result.Domains[1].Features[0].TestIndices) != 1 {
			t.Errorf("expected 1 test in User.Profile, got %d", len(result.Domains[1].Features[0].TestIndices))
		}
	})
}
