package specview

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/specvital/worker/internal/domain/specview"
)

type mockRepository struct {
	findDocumentByContentHashFn func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error)
	getTestDataByAnalysisIDFn   func(ctx context.Context, analysisID string) ([]specview.FileInfo, error)
	recordUsageEventFn          func(ctx context.Context, userID string, documentID string, quotaAmount int) error
	recordUserHistoryFn         func(ctx context.Context, userID string, documentID string) error
	saveDocumentFn              func(ctx context.Context, doc *specview.SpecDocument) error
}

func (m *mockRepository) FindDocumentByContentHash(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
	if m.findDocumentByContentHashFn != nil {
		return m.findDocumentByContentHashFn(ctx, contentHash, language, modelID)
	}
	return nil, nil
}

func (m *mockRepository) GetTestDataByAnalysisID(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
	if m.getTestDataByAnalysisIDFn != nil {
		return m.getTestDataByAnalysisIDFn(ctx, analysisID)
	}
	return nil, nil
}

func (m *mockRepository) RecordUsageEvent(ctx context.Context, userID string, documentID string, quotaAmount int) error {
	if m.recordUsageEventFn != nil {
		return m.recordUsageEventFn(ctx, userID, documentID, quotaAmount)
	}
	return nil
}

func (m *mockRepository) RecordUserHistory(ctx context.Context, userID string, documentID string) error {
	if m.recordUserHistoryFn != nil {
		return m.recordUserHistoryFn(ctx, userID, documentID)
	}
	return nil
}

func (m *mockRepository) SaveDocument(ctx context.Context, doc *specview.SpecDocument) error {
	if m.saveDocumentFn != nil {
		return m.saveDocumentFn(ctx, doc)
	}
	return nil
}

type mockAIProvider struct {
	classifyDomainsFn  func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error)
	convertTestNamesFn func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error)
}

func (m *mockAIProvider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
	if m.classifyDomainsFn != nil {
		return m.classifyDomainsFn(ctx, input)
	}
	return nil, nil, nil
}

func (m *mockAIProvider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
	if m.convertTestNamesFn != nil {
		return m.convertTestNamesFn(ctx, input)
	}
	return nil, nil, nil
}

func (m *mockAIProvider) Close() error {
	return nil
}

func newTestFiles() []specview.FileInfo {
	return []specview.FileInfo{
		{
			Path:      "test/auth_test.go",
			Framework: "go",
			Tests: []specview.TestInfo{
				{Index: 0, Name: "TestLogin", TestCaseID: "tc-001"},
				{Index: 1, Name: "TestLogout", TestCaseID: "tc-002"},
			},
		},
		{
			Path:      "test/user_test.go",
			Framework: "go",
			Tests: []specview.TestInfo{
				{Index: 2, Name: "TestCreateUser", TestCaseID: "tc-003"},
				{Index: 3, Name: "TestDeleteUser", TestCaseID: "tc-004"},
			},
		},
	}
}

func newPhase1Output() *specview.Phase1Output {
	return &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:        "Authentication",
				Description: "User authentication flows",
				Confidence:  0.95,
				Features: []specview.FeatureGroup{
					{
						Name:        "Login",
						Description: "User login functionality",
						Confidence:  0.9,
						TestIndices: []int{0},
					},
					{
						Name:        "Logout",
						Description: "User logout functionality",
						Confidence:  0.85,
						TestIndices: []int{1},
					},
				},
			},
			{
				Name:        "User Management",
				Description: "User CRUD operations",
				Confidence:  0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "User Creation",
						Description: "Create new users",
						Confidence:  0.88,
						TestIndices: []int{2, 3},
					},
				},
			},
		},
	}
}

func newValidRequest() specview.SpecViewRequest {
	return specview.SpecViewRequest{
		AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
		Language:   "Korean",
	}
}

func TestGenerateSpecViewUseCase_Execute(t *testing.T) {
	t.Run("success - complete workflow", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		savedDoc := &specview.SpecDocument{}
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				savedDoc = doc
				doc.ID = "doc-001"
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				usage := &specview.TokenUsage{
					Model:            "gemini-2.5-flash",
					PromptTokens:     1000,
					CandidatesTokens: 500,
					TotalTokens:      1500,
				}
				return phase1Output, usage, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				behaviors := make([]specview.BehaviorSpec, len(input.Tests))
				for i, test := range input.Tests {
					behaviors[i] = specview.BehaviorSpec{
						TestIndex:   test.Index,
						Description: "사용자가 " + test.Name + " 기능을 테스트한다",
						Confidence:  0.9,
					}
				}
				usage := &specview.TokenUsage{
					Model:            "gemini-2.5-flash-lite",
					PromptTokens:     200,
					CandidatesTokens: 100,
					TotalTokens:      300,
				}
				return &specview.Phase2Output{Behaviors: behaviors}, usage, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		result, err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if result.CacheHit {
			t.Error("expected cache miss")
		}
		if result.DocumentID != "doc-001" {
			t.Errorf("expected document ID 'doc-001', got '%s'", result.DocumentID)
		}

		if len(savedDoc.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(savedDoc.Domains))
		}
		if savedDoc.Language != "Korean" {
			t.Errorf("expected language KO, got %s", savedDoc.Language)
		}
	})

	t.Run("cache hit - returns cached document immediately", func(t *testing.T) {
		files := newTestFiles()
		cachedDoc := &specview.SpecDocument{
			ID:       "cached-doc-001",
			Language: "Korean",
		}

		classifyDomainsCalled := false
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return cachedDoc, nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				classifyDomainsCalled = true
				return nil, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		result, err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if !result.CacheHit {
			t.Error("expected cache hit")
		}
		if result.DocumentID != "cached-doc-001" {
			t.Errorf("expected document ID 'cached-doc-001', got '%s'", result.DocumentID)
		}
		if classifyDomainsCalled {
			t.Error("AI should not be called on cache hit")
		}
	})

	t.Run("invalid input - empty analysis ID", func(t *testing.T) {
		uc := NewGenerateSpecViewUseCase(&mockRepository{}, &mockAIProvider{}, "gemini-2.5-flash")

		req := specview.SpecViewRequest{
			AnalysisID: "",
			Language:   "Korean",
		}

		_, err := uc.Execute(context.Background(), req)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, specview.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("analysis not found", func(t *testing.T) {
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return nil, specview.ErrAnalysisNotFound
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, &mockAIProvider{}, "gemini-2.5-flash")

		_, err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrLoadInventoryFailed) {
			t.Errorf("expected ErrLoadInventoryFailed, got %v", err)
		}
	})

	t.Run("no test files found", func(t *testing.T) {
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return []specview.FileInfo{}, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, &mockAIProvider{}, "gemini-2.5-flash")

		_, err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrLoadInventoryFailed) {
			t.Errorf("expected ErrLoadInventoryFailed, got %v", err)
		}
	})

	t.Run("phase 1 failure", func(t *testing.T) {
		files := newTestFiles()
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return nil, nil, errors.New("AI service unavailable")
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		_, err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrAIProcessingFailed) {
			t.Errorf("expected ErrAIProcessingFailed, got %v", err)
		}
	})

	t.Run("partial phase 2 failure with fallback", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
		}

		var callCount atomic.Int32
		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				count := callCount.Add(1)
				if count == 1 {
					return nil, nil, errors.New("AI error")
				}
				behaviors := make([]specview.BehaviorSpec, len(input.Tests))
				for i, test := range input.Tests {
					behaviors[i] = specview.BehaviorSpec{
						TestIndex:   test.Index,
						Description: "Converted: " + test.Name,
						Confidence:  0.9,
					}
				}
				return &specview.Phase2Output{Behaviors: behaviors}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		result, err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
	})

	t.Run("phase 2 failure exceeds threshold", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return nil, nil, errors.New("AI error")
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash", WithFailureThreshold(0.3))

		_, err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrPartialFeatureFailure) {
			t.Errorf("expected ErrPartialFeatureFailure, got %v", err)
		}
	})

	t.Run("save document failure", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				return errors.New("database error")
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				behaviors := make([]specview.BehaviorSpec, len(input.Tests))
				for i, test := range input.Tests {
					behaviors[i] = specview.BehaviorSpec{
						TestIndex:   test.Index,
						Description: "Converted: " + test.Name,
						Confidence:  0.9,
					}
				}
				return &specview.Phase2Output{Behaviors: behaviors}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		_, err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrSaveFailed) {
			t.Errorf("expected ErrSaveFailed, got %v", err)
		}
	})

	t.Run("custom model ID override", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		var savedModelID string
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				savedModelID = modelID
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "default-model")

		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			ModelID:    "custom-model",
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if savedModelID != "custom-model" {
			t.Errorf("expected model ID 'custom-model', got '%s'", savedModelID)
		}
	})
}

func TestGenerateSpecViewUseCase_Options(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		uc := NewGenerateSpecViewUseCase(&mockRepository{}, &mockAIProvider{}, "gemini-2.5-flash")

		if uc.config.Phase1Timeout != DefaultPhase1Timeout {
			t.Errorf("expected default phase1 timeout %v, got %v", DefaultPhase1Timeout, uc.config.Phase1Timeout)
		}
		if uc.config.Phase2Timeout != DefaultPhase2Timeout {
			t.Errorf("expected default phase2 timeout %v, got %v", DefaultPhase2Timeout, uc.config.Phase2Timeout)
		}
		if uc.config.Phase2Concurrency != DefaultPhase2Concurrency {
			t.Errorf("expected default phase2 concurrency %d, got %d", DefaultPhase2Concurrency, uc.config.Phase2Concurrency)
		}
		if uc.config.FailureThreshold != DefaultFailureThreshold {
			t.Errorf("expected default failure threshold %f, got %f", DefaultFailureThreshold, uc.config.FailureThreshold)
		}
	})

	t.Run("custom options", func(t *testing.T) {
		uc := NewGenerateSpecViewUseCase(
			&mockRepository{},
			&mockAIProvider{},
			"gemini-2.5-flash",
			WithPhase1Timeout(5*time.Minute),
			WithPhase2Timeout(10*time.Minute),
			WithPhase2Concurrency(10),
			WithFailureThreshold(0.7),
		)

		if uc.config.Phase1Timeout != 5*time.Minute {
			t.Errorf("expected phase1 timeout 5m, got %v", uc.config.Phase1Timeout)
		}
		if uc.config.Phase2Timeout != 10*time.Minute {
			t.Errorf("expected phase2 timeout 10m, got %v", uc.config.Phase2Timeout)
		}
		if uc.config.Phase2Concurrency != 10 {
			t.Errorf("expected phase2 concurrency 10, got %d", uc.config.Phase2Concurrency)
		}
		if uc.config.FailureThreshold != 0.7 {
			t.Errorf("expected failure threshold 0.7, got %f", uc.config.FailureThreshold)
		}
	})

	t.Run("invalid options ignored", func(t *testing.T) {
		uc := NewGenerateSpecViewUseCase(
			&mockRepository{},
			&mockAIProvider{},
			"gemini-2.5-flash",
			WithPhase1Timeout(0),
			WithPhase2Timeout(-1*time.Minute),
			WithPhase2Concurrency(0),
			WithFailureThreshold(0),
			WithFailureThreshold(1.5),
		)

		if uc.config.Phase1Timeout != DefaultPhase1Timeout {
			t.Errorf("expected default phase1 timeout, got %v", uc.config.Phase1Timeout)
		}
		if uc.config.Phase2Timeout != DefaultPhase2Timeout {
			t.Errorf("expected default phase2 timeout, got %v", uc.config.Phase2Timeout)
		}
		if uc.config.Phase2Concurrency != DefaultPhase2Concurrency {
			t.Errorf("expected default phase2 concurrency, got %d", uc.config.Phase2Concurrency)
		}
		if uc.config.FailureThreshold != DefaultFailureThreshold {
			t.Errorf("expected default failure threshold, got %f", uc.config.FailureThreshold)
		}
	})
}

func TestBuildTestIndexMap(t *testing.T) {
	files := []specview.FileInfo{
		{
			Tests: []specview.TestInfo{
				{Index: 0, Name: "Test1", TestCaseID: "tc-001"},
				{Index: 1, Name: "Test2", TestCaseID: "tc-002"},
			},
		},
		{
			Tests: []specview.TestInfo{
				{Index: 2, Name: "Test3", TestCaseID: "tc-003"},
			},
		},
	}

	result := buildTestIndexMap(files)

	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}

	if result[0].Name != "Test1" {
		t.Errorf("expected Test1 at index 0, got %s", result[0].Name)
	}
	if result[2].TestCaseID != "tc-003" {
		t.Errorf("expected tc-003 at index 2, got %s", result[2].TestCaseID)
	}
}

func TestGenerateFallbackBehaviors(t *testing.T) {
	uc := &GenerateSpecViewUseCase{}
	tests := []specview.TestForConversion{
		{Index: 0, Name: "TestLogin"},
		{Index: 1, Name: "TestLogout"},
	}

	behaviors := uc.generateFallbackBehaviors(tests)

	if len(behaviors) != 2 {
		t.Errorf("expected 2 behaviors, got %d", len(behaviors))
	}
	if behaviors[0].TestIndex != 0 {
		t.Errorf("expected test index 0, got %d", behaviors[0].TestIndex)
	}
	if behaviors[0].Description != "TestLogin" {
		t.Errorf("expected description 'TestLogin', got '%s'", behaviors[0].Description)
	}
	if behaviors[0].Confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got %f", behaviors[0].Confidence)
	}
}

func TestAssembleDocument(t *testing.T) {
	uc := &GenerateSpecViewUseCase{}

	req := specview.SpecViewRequest{
		AnalysisID: "analysis-001",
		Language:   "English",
	}

	phase1Output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:        "Auth",
				Description: "Authentication domain",
				Confidence:  0.95,
				Features: []specview.FeatureGroup{
					{
						Name:        "Login",
						Description: "Login feature",
						Confidence:  0.9,
						TestIndices: []int{0},
					},
				},
			},
		},
	}

	phase2Results := []phase2Result{
		{
			domainIdx:  0,
			featureIdx: 0,
			behaviors: []specview.BehaviorSpec{
				{TestIndex: 0, Description: "User can login", Confidence: 0.85},
			},
		},
	}

	testIndexMap := map[int]specview.TestInfo{
		0: {Index: 0, Name: "TestLogin", TestCaseID: "tc-001"},
	}

	contentHash := []byte("test-hash")
	modelID := "gemini-2.5-flash"

	doc := uc.assembleDocument(req, modelID, contentHash, phase1Output, phase2Results, testIndexMap)

	if doc.AnalysisID != "analysis-001" {
		t.Errorf("expected analysis ID 'analysis-001', got '%s'", doc.AnalysisID)
	}
	if doc.Language != "English" {
		t.Errorf("expected language EN, got %s", doc.Language)
	}
	if doc.ModelID != "gemini-2.5-flash" {
		t.Errorf("expected model ID 'gemini-2.5-flash', got '%s'", doc.ModelID)
	}
	if len(doc.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(doc.Domains))
	}

	domain := doc.Domains[0]
	if domain.Name != "Auth" {
		t.Errorf("expected domain name 'Auth', got '%s'", domain.Name)
	}
	if len(domain.Features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(domain.Features))
	}

	feature := domain.Features[0]
	if len(feature.Behaviors) != 1 {
		t.Errorf("expected 1 behavior, got %d", len(feature.Behaviors))
	}

	behavior := feature.Behaviors[0]
	if behavior.Description != "User can login" {
		t.Errorf("expected description 'User can login', got '%s'", behavior.Description)
	}
	if behavior.TestCaseID != "tc-001" {
		t.Errorf("expected test case ID 'tc-001', got '%s'", behavior.TestCaseID)
	}
	if behavior.OriginalName != "TestLogin" {
		t.Errorf("expected original name 'TestLogin', got '%s'", behavior.OriginalName)
	}
}

func TestGenerateSpecViewUseCase_RecordUserHistory(t *testing.T) {
	t.Run("records history when userID is provided", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		var recordedUserID, recordedDocID string
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUserHistoryFn: func(ctx context.Context, userID string, documentID string) error {
				recordedUserID = userID
				recordedDocID = documentID
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		userID := "user-001"
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &userID,
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if recordedUserID != "user-001" {
			t.Errorf("expected user ID 'user-001', got '%s'", recordedUserID)
		}
		if recordedDocID != "doc-001" {
			t.Errorf("expected document ID 'doc-001', got '%s'", recordedDocID)
		}
	})

	t.Run("records history on cache hit", func(t *testing.T) {
		files := newTestFiles()
		cachedDoc := &specview.SpecDocument{
			ID:       "cached-doc-001",
			Language: "Korean",
		}

		var recordedUserID, recordedDocID string
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return cachedDoc, nil
			},
			recordUserHistoryFn: func(ctx context.Context, userID string, documentID string) error {
				recordedUserID = userID
				recordedDocID = documentID
				return nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, &mockAIProvider{}, "gemini-2.5-flash")

		userID := "user-002"
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &userID,
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if recordedUserID != "user-002" {
			t.Errorf("expected user ID 'user-002', got '%s'", recordedUserID)
		}
		if recordedDocID != "cached-doc-001" {
			t.Errorf("expected document ID 'cached-doc-001', got '%s'", recordedDocID)
		}
	})

	t.Run("skips history recording when userID is nil", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		historyRecorded := false
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUserHistoryFn: func(ctx context.Context, userID string, documentID string) error {
				historyRecorded = true
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     nil,
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if historyRecorded {
			t.Error("expected history NOT to be recorded when userID is nil")
		}
	})

	t.Run("history recording failure is non-blocking", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUserHistoryFn: func(ctx context.Context, userID string, documentID string) error {
				return errors.New("history recording failed")
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		userID := "user-001"
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &userID,
		}

		result, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if result.DocumentID != "doc-001" {
			t.Errorf("expected document ID 'doc-001', got '%s'", result.DocumentID)
		}
	})
}

func TestGenerateSpecViewUseCase_RecordUsageEvent(t *testing.T) {
	t.Run("records usage event on cache miss", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		var recordedUserID, recordedDocID string
		var recordedQuotaAmount int
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUsageEventFn: func(ctx context.Context, userID string, documentID string, quotaAmount int) error {
				recordedUserID = userID
				recordedDocID = documentID
				recordedQuotaAmount = quotaAmount
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		userID := "user-001"
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &userID,
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if recordedUserID != "user-001" {
			t.Errorf("expected user ID 'user-001', got '%s'", recordedUserID)
		}
		if recordedDocID != "doc-001" {
			t.Errorf("expected document ID 'doc-001', got '%s'", recordedDocID)
		}
		// files have 4 tests total (2 + 2)
		if recordedQuotaAmount != 4 {
			t.Errorf("expected quota amount 4, got %d", recordedQuotaAmount)
		}
	})

	t.Run("no usage event on cache hit", func(t *testing.T) {
		files := newTestFiles()
		cachedDoc := &specview.SpecDocument{
			ID:       "cached-doc-001",
			Language: "Korean",
		}

		usageEventRecorded := false
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return cachedDoc, nil
			},
			recordUsageEventFn: func(ctx context.Context, userID string, documentID string, quotaAmount int) error {
				usageEventRecorded = true
				return nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, &mockAIProvider{}, "gemini-2.5-flash")

		userID := "user-001"
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &userID,
		}

		result, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.CacheHit {
			t.Error("expected cache hit")
		}
		if usageEventRecorded {
			t.Error("expected usage event NOT to be recorded on cache hit")
		}
	})

	t.Run("skips usage event when userID is nil", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		usageEventRecorded := false
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUsageEventFn: func(ctx context.Context, userID string, documentID string, quotaAmount int) error {
				usageEventRecorded = true
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     nil,
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if usageEventRecorded {
			t.Error("expected usage event NOT to be recorded when userID is nil")
		}
	})

	t.Run("skips usage event when userID is empty string", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		usageEventRecorded := false
		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUsageEventFn: func(ctx context.Context, userID string, documentID string, quotaAmount int) error {
				usageEventRecorded = true
				return nil
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		emptyUserID := ""
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &emptyUserID,
		}

		_, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if usageEventRecorded {
			t.Error("expected usage event NOT to be recorded when userID is empty string")
		}
	})

	t.Run("usage event recording failure is non-blocking", func(t *testing.T) {
		files := newTestFiles()
		phase1Output := newPhase1Output()

		repo := &mockRepository{
			getTestDataByAnalysisIDFn: func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
				return files, nil
			},
			findDocumentByContentHashFn: func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
				return nil, nil
			},
			saveDocumentFn: func(ctx context.Context, doc *specview.SpecDocument) error {
				doc.ID = "doc-001"
				return nil
			},
			recordUsageEventFn: func(ctx context.Context, userID string, documentID string, quotaAmount int) error {
				return errors.New("usage event recording failed")
			},
		}

		aiProvider := &mockAIProvider{
			classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
				return phase1Output, nil, nil
			},
			convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
				return &specview.Phase2Output{Behaviors: []specview.BehaviorSpec{}}, nil, nil
			},
		}

		uc := NewGenerateSpecViewUseCase(repo, aiProvider, "gemini-2.5-flash")

		userID := "user-001"
		req := specview.SpecViewRequest{
			AnalysisID: "550e8400-e29b-41d4-a716-446655440000",
			Language:   "Korean",
			UserID:     &userID,
		}

		result, err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if result.DocumentID != "doc-001" {
			t.Errorf("expected document ID 'doc-001', got '%s'", result.DocumentID)
		}
	})
}

func TestCountTotalTestCases(t *testing.T) {
	t.Run("counts all test cases across files", func(t *testing.T) {
		files := []specview.FileInfo{
			{
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
					{Index: 1, Name: "Test2"},
				},
			},
			{
				Tests: []specview.TestInfo{
					{Index: 2, Name: "Test3"},
				},
			},
		}

		count := countTotalTestCases(files)

		if count != 3 {
			t.Errorf("expected 3 test cases, got %d", count)
		}
	})

	t.Run("returns zero for empty files", func(t *testing.T) {
		files := []specview.FileInfo{}

		count := countTotalTestCases(files)

		if count != 0 {
			t.Errorf("expected 0 test cases, got %d", count)
		}
	})

	t.Run("returns zero for files with no tests", func(t *testing.T) {
		files := []specview.FileInfo{
			{Path: "empty.go", Tests: []specview.TestInfo{}},
		}

		count := countTotalTestCases(files)

		if count != 0 {
			t.Errorf("expected 0 test cases, got %d", count)
		}
	})
}
