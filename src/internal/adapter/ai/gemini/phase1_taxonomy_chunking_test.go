package gemini

import (
	"context"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestSplitTaxonomyInput(t *testing.T) {
	tests := []struct {
		name           string
		fileCount      int
		chunkSize      int
		wantChunkCount int
		wantLastSize   int
	}{
		{
			name:           "single chunk when below limit",
			fileCount:      100,
			chunkSize:      400,
			wantChunkCount: 1,
			wantLastSize:   100,
		},
		{
			name:           "exact split",
			fileCount:      800,
			chunkSize:      400,
			wantChunkCount: 2,
			wantLastSize:   400,
		},
		{
			name:           "uneven split",
			fileCount:      500,
			chunkSize:      400,
			wantChunkCount: 2,
			wantLastSize:   100,
		},
		{
			name:           "large file set",
			fileCount:      1801,
			chunkSize:      400,
			wantChunkCount: 5,
			wantLastSize:   201, // 1801 - 400*4 = 201
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := make([]specview.TaxonomyFileInfo, tt.fileCount)
			for i := range files {
				files[i] = specview.TaxonomyFileInfo{
					Index:     i,
					Path:      "test.go",
					TestCount: 5,
				}
			}

			input := specview.TaxonomyInput{
				AnalysisID: "test",
				Files:      files,
				Language:   "Korean",
			}

			chunks := splitTaxonomyInput(input, tt.chunkSize)

			if len(chunks) != tt.wantChunkCount {
				t.Errorf("chunk count = %d, want %d", len(chunks), tt.wantChunkCount)
			}

			if len(chunks) > 0 {
				lastChunk := chunks[len(chunks)-1]
				if len(lastChunk.Files) != tt.wantLastSize {
					t.Errorf("last chunk size = %d, want %d", len(lastChunk.Files), tt.wantLastSize)
				}
			}

			// Verify indices are remapped to local
			for _, chunk := range chunks {
				for i, file := range chunk.Files {
					if file.Index != i {
						t.Errorf("chunk[%d] file[%d].Index = %d, want %d", chunk.ChunkIndex, i, file.Index, i)
					}
				}
			}

			// Verify global offsets
			expectedOffset := 0
			for _, chunk := range chunks {
				if chunk.GlobalOffset != expectedOffset {
					t.Errorf("chunk[%d].GlobalOffset = %d, want %d", chunk.ChunkIndex, chunk.GlobalOffset, expectedOffset)
				}
				expectedOffset += len(chunk.Files)
			}
		})
	}
}

func TestRemapIndices(t *testing.T) {
	tests := []struct {
		name    string
		indices []int
		offset  int
		want    []int
	}{
		{
			name:    "zero offset",
			indices: []int{0, 1, 2},
			offset:  0,
			want:    []int{0, 1, 2},
		},
		{
			name:    "positive offset",
			indices: []int{0, 1, 2},
			offset:  400,
			want:    []int{400, 401, 402},
		},
		{
			name:    "empty indices",
			indices: []int{},
			offset:  100,
			want:    []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remapIndices(tt.indices, tt.offset)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeDomainName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "authentication", "authentication"},
		{"uppercase", "AUTHENTICATION", "authentication"},
		{"mixed case", "Authentication", "authentication"},
		{"with spaces", "  Authentication  ", "authentication"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDomainName(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDomainName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMergeTaxonomyChunks(t *testing.T) {
	results := []ChunkedTaxonomyResult{
		{
			ChunkIndex:   0,
			FileCount:    400,
			GlobalOffset: 0,
			Taxonomy: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name:        "Authentication",
						Description: "Auth tests",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0, 1}},
						},
					},
				},
			},
			Usage: &specview.TokenUsage{PromptTokens: 100, CandidatesTokens: 50},
		},
		{
			ChunkIndex:   1,
			FileCount:    400,
			GlobalOffset: 400,
			Taxonomy: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name:        "Authentication", // Same domain name - should merge
						Description: "More auth tests",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0, 1}},  // Should become 400, 401
							{Name: "Logout", FileIndices: []int{2, 3}}, // Should become 402, 403
						},
					},
					{
						Name:        "Payment",
						Description: "Payment tests",
						Features: []specview.TaxonomyFeature{
							{Name: "Checkout", FileIndices: []int{4}}, // Should become 404
						},
					},
				},
			},
			Usage: &specview.TokenUsage{PromptTokens: 150, CandidatesTokens: 75},
		},
	}

	taxonomy, usage := mergeTaxonomyChunks(context.Background(), results, 800)

	// Verify usage aggregation
	if usage.PromptTokens != 250 {
		t.Errorf("PromptTokens = %d, want 250", usage.PromptTokens)
	}
	if usage.CandidatesTokens != 125 {
		t.Errorf("CandidatesTokens = %d, want 125", usage.CandidatesTokens)
	}

	// Verify domain merging (Authentication should be merged, Payment added)
	if len(taxonomy.Domains) != 2 {
		t.Fatalf("domain count = %d, want 2", len(taxonomy.Domains))
	}

	// Find Authentication domain
	var authDomain *specview.TaxonomyDomain
	for i := range taxonomy.Domains {
		if normalizeDomainName(taxonomy.Domains[i].Name) == "authentication" {
			authDomain = &taxonomy.Domains[i]
			break
		}
	}

	if authDomain == nil {
		t.Fatal("Authentication domain not found")
	}

	// Verify Login feature was merged with remapped indices
	var loginFeature *specview.TaxonomyFeature
	for i := range authDomain.Features {
		if normalizeDomainName(authDomain.Features[i].Name) == "login" {
			loginFeature = &authDomain.Features[i]
			break
		}
	}

	if loginFeature == nil {
		t.Fatal("Login feature not found")
	}

	// Should have 4 indices: 0, 1 from chunk 0 + 400, 401 from chunk 1
	if len(loginFeature.FileIndices) != 4 {
		t.Errorf("Login feature indices count = %d, want 4", len(loginFeature.FileIndices))
	}
}

func TestMergeTaxonomyChunks_PartialFailure(t *testing.T) {
	results := []ChunkedTaxonomyResult{
		{
			ChunkIndex:   0,
			FileCount:    400,
			GlobalOffset: 0,
			Taxonomy: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name: "API",
						Features: []specview.TaxonomyFeature{
							{Name: "Endpoints", FileIndices: []int{0, 1}},
						},
					},
				},
			},
		},
		{
			ChunkIndex:   1,
			FileCount:    400,
			GlobalOffset: 400,
			Taxonomy:     nil, // Failed chunk
			Err:          specview.ErrOutputTruncated,
		},
	}

	taxonomy, _ := mergeTaxonomyChunks(context.Background(), results, 800)

	// Should have at least one domain (from successful chunk + heuristic for failed)
	if len(taxonomy.Domains) == 0 {
		t.Error("expected at least one domain")
	}

	// Verify successful chunk data is preserved
	found := false
	for _, domain := range taxonomy.Domains {
		if normalizeDomainName(domain.Name) == "api" {
			found = true
			break
		}
	}
	if !found {
		t.Error("API domain from successful chunk not found")
	}
}

func TestMergeFeatures(t *testing.T) {
	target := &specview.TaxonomyDomain{
		Name: "Auth",
		Features: []specview.TaxonomyFeature{
			{Name: "Login", FileIndices: []int{0, 1}},
		},
	}

	source := specview.TaxonomyDomain{
		Name: "Auth",
		Features: []specview.TaxonomyFeature{
			{Name: "Login", FileIndices: []int{0, 1}},  // Same name - should merge
			{Name: "Logout", FileIndices: []int{2, 3}}, // New feature - should add
		},
	}

	mergeFeatures(target, source, 400)

	if len(target.Features) != 2 {
		t.Fatalf("feature count = %d, want 2", len(target.Features))
	}

	// Login should have 4 indices now
	var loginFeature *specview.TaxonomyFeature
	for i := range target.Features {
		if normalizeDomainName(target.Features[i].Name) == "login" {
			loginFeature = &target.Features[i]
			break
		}
	}

	if loginFeature == nil {
		t.Fatal("Login feature not found")
	}

	if len(loginFeature.FileIndices) != 4 {
		t.Errorf("Login indices count = %d, want 4", len(loginFeature.FileIndices))
	}
}

func TestMergeTaxonomyChunks_FileInMultipleFeatures(t *testing.T) {
	// Validates that same file can appear in multiple features across chunks
	results := []ChunkedTaxonomyResult{
		{
			ChunkIndex:   0,
			FileCount:    400,
			GlobalOffset: 0,
			Taxonomy: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name: "Authentication",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0, 1, 2}},
							{Name: "Session", FileIndices: []int{0, 1}}, // Same files as Login
						},
					},
				},
			},
		},
		{
			ChunkIndex:   1,
			FileCount:    400,
			GlobalOffset: 400,
			Taxonomy: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name: "Authentication",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0, 1}},   // Will become 400, 401
							{Name: "Session", FileIndices: []int{0, 2}}, // Will become 400, 402
						},
					},
				},
			},
		},
	}

	taxonomy, _ := mergeTaxonomyChunks(context.Background(), results, 800)

	// Find Authentication domain
	var authDomain *specview.TaxonomyDomain
	for i := range taxonomy.Domains {
		if normalizeDomainName(taxonomy.Domains[i].Name) == "authentication" {
			authDomain = &taxonomy.Domains[i]
			break
		}
	}

	if authDomain == nil {
		t.Fatal("Authentication domain not found")
	}

	// Should have 2 features
	if len(authDomain.Features) != 2 {
		t.Errorf("feature count = %d, want 2", len(authDomain.Features))
	}

	// Build feature index map
	featureIndices := make(map[string][]int)
	for _, f := range authDomain.Features {
		featureIndices[normalizeDomainName(f.Name)] = f.FileIndices
	}

	// Login: [0, 1, 2] from chunk 0 + [400, 401] from chunk 1 = 5 indices
	loginIndices := featureIndices["login"]
	if len(loginIndices) != 5 {
		t.Errorf("Login indices count = %d, want 5, got %v", len(loginIndices), loginIndices)
	}

	// Session: [0, 1] from chunk 0 + [400, 402] from chunk 1 = 4 indices
	sessionIndices := featureIndices["session"]
	if len(sessionIndices) != 4 {
		t.Errorf("Session indices count = %d, want 4, got %v", len(sessionIndices), sessionIndices)
	}
}
