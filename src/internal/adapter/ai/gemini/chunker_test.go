package gemini

import (
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		files    []specview.FileInfo
		wantMin  int
		wantMax  int
	}{
		{
			name:    "empty files",
			files:   nil,
			wantMin: basePromptTokens,
			wantMax: basePromptTokens,
		},
		{
			name: "single file with 10 tests",
			files: []specview.FileInfo{
				{Path: "test.go", Tests: makeTests(10, 0)},
			},
			wantMin: basePromptTokens + tokensPerFile + 10*tokensPerTest,
			wantMax: basePromptTokens + tokensPerFile + 10*tokensPerTest,
		},
		{
			name: "multiple files",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(100, 0)},
				{Path: "b.go", Tests: makeTests(100, 100)},
			},
			wantMin: basePromptTokens + 2*tokensPerFile + 200*tokensPerTest,
			wantMax: basePromptTokens + 2*tokensPerFile + 200*tokensPerTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.files)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EstimateTokens() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestNeedsChunking(t *testing.T) {
	config := ChunkConfig{
		MaxTestsPerChunk:  100,
		MaxTokensPerChunk: 10000,
	}

	tests := []struct {
		name   string
		files  []specview.FileInfo
		want   bool
	}{
		{
			name:  "empty files",
			files: nil,
			want:  false,
		},
		{
			name: "below threshold",
			files: []specview.FileInfo{
				{Path: "test.go", Tests: makeTests(50, 0)},
			},
			want: false,
		},
		{
			name: "above test count threshold",
			files: []specview.FileInfo{
				{Path: "test.go", Tests: makeTests(150, 0)},
			},
			want: true,
		},
		{
			name: "above token threshold",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(50, 0)},
				{Path: "b.go", Tests: makeTests(50, 50)},
				{Path: "c.go", Tests: makeTests(50, 100)},
				{Path: "d.go", Tests: makeTests(50, 150)},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsChunking(tt.files, config); got != tt.want {
				t.Errorf("NeedsChunking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitIntoChunks(t *testing.T) {
	config := ChunkConfig{
		MaxTestsPerChunk:  100,
		MaxTokensPerChunk: 100000, // high enough to not trigger
	}

	tests := []struct {
		name           string
		files          []specview.FileInfo
		wantChunkCount int
		wantFileCounts []int // number of files per chunk
	}{
		{
			name:           "no chunking needed",
			files:          []specview.FileInfo{{Path: "a.go", Tests: makeTests(50, 0)}},
			wantChunkCount: 1,
			wantFileCounts: []int{1},
		},
		{
			name: "split into 2 chunks",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(60, 0)},
				{Path: "b.go", Tests: makeTests(60, 60)},
			},
			wantChunkCount: 2,
			wantFileCounts: []int{1, 1},
		},
		{
			name: "split into 3 chunks",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(80, 0)},
				{Path: "b.go", Tests: makeTests(80, 80)},
				{Path: "c.go", Tests: makeTests(80, 160)},
			},
			wantChunkCount: 3,
			wantFileCounts: []int{1, 1, 1},
		},
		{
			name: "single oversized file stays in one chunk",
			files: []specview.FileInfo{
				{Path: "huge.go", Tests: makeTests(200, 0)},
			},
			wantChunkCount: 1,
			wantFileCounts: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := SplitIntoChunks(tt.files, config)

			if len(chunks) != tt.wantChunkCount {
				t.Errorf("SplitIntoChunks() chunk count = %v, want %v", len(chunks), tt.wantChunkCount)
			}

			for i, chunk := range chunks {
				if len(chunk.Files) != tt.wantFileCounts[i] {
					t.Errorf("chunk[%d] file count = %v, want %v", i, len(chunk.Files), tt.wantFileCounts[i])
				}
			}
		})
	}
}

func TestReindexTests(t *testing.T) {
	files := []specview.FileInfo{
		{
			Path: "a.go",
			Tests: []specview.TestInfo{
				{Index: 100, Name: "test1"},
				{Index: 101, Name: "test2"},
			},
		},
		{
			Path: "b.go",
			Tests: []specview.TestInfo{
				{Index: 200, Name: "test3"},
			},
		},
	}

	reindexed, indexMap := ReindexTests(files)

	// Check reindexed files have sequential indices starting from 0
	expectedNewIndices := []int{0, 1, 2}
	idx := 0
	for _, file := range reindexed {
		for _, test := range file.Tests {
			if test.Index != expectedNewIndices[idx] {
				t.Errorf("reindexed test index = %v, want %v", test.Index, expectedNewIndices[idx])
			}
			idx++
		}
	}

	// Check index map
	expectedMap := map[int]int{
		0: 100,
		1: 101,
		2: 200,
	}
	for newIdx, wantOriginal := range expectedMap {
		if gotOriginal, ok := indexMap[newIdx]; !ok || gotOriginal != wantOriginal {
			t.Errorf("indexMap[%v] = %v, want %v", newIdx, gotOriginal, wantOriginal)
		}
	}
}

func TestRestoreIndices(t *testing.T) {
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name: "Auth",
				Features: []specview.FeatureGroup{
					{Name: "Login", TestIndices: []int{0, 1}},
					{Name: "Logout", TestIndices: []int{2}},
				},
			},
		},
	}

	indexMap := map[int]int{
		0: 100,
		1: 101,
		2: 200,
	}

	RestoreIndices(output, indexMap)

	wantIndices := [][]int{{100, 101}, {200}}
	for i, feature := range output.Domains[0].Features {
		for j, idx := range feature.TestIndices {
			if idx != wantIndices[i][j] {
				t.Errorf("feature[%d].TestIndices[%d] = %v, want %v", i, j, idx, wantIndices[i][j])
			}
		}
	}
}

func TestMergePhase1Outputs(t *testing.T) {
	outputs := []*specview.Phase1Output{
		{
			Domains: []specview.DomainGroup{
				{
					Name:        "Auth",
					Description: "Authentication",
					Confidence:  0.9,
					Features: []specview.FeatureGroup{
						{Name: "Login", TestIndices: []int{0, 1}},
					},
				},
				{
					Name:        "Dashboard",
					Description: "Dashboard features",
					Confidence:  0.8,
					Features: []specview.FeatureGroup{
						{Name: "Charts", TestIndices: []int{2}},
					},
				},
			},
		},
		{
			Domains: []specview.DomainGroup{
				{
					Name:        "Auth",
					Description: "Authentication domain",
					Confidence:  0.85,
					Features: []specview.FeatureGroup{
						{Name: "Password Reset", TestIndices: []int{100, 101}},
					},
				},
				{
					Name:        "Billing",
					Description: "Billing features",
					Confidence:  0.9,
					Features: []specview.FeatureGroup{
						{Name: "Invoices", TestIndices: []int{102}},
					},
				},
			},
		},
	}

	merged := MergePhase1Outputs(outputs)

	// Should have 3 domains: Auth, Dashboard, Billing
	if len(merged.Domains) != 3 {
		t.Errorf("merged domains count = %v, want 3", len(merged.Domains))
	}

	// Auth should have merged features
	var authDomain *specview.DomainGroup
	for i := range merged.Domains {
		if merged.Domains[i].Name == "Auth" {
			authDomain = &merged.Domains[i]
			break
		}
	}

	if authDomain == nil {
		t.Fatal("Auth domain not found in merged output")
	}

	if len(authDomain.Features) != 2 {
		t.Errorf("Auth domain features count = %v, want 2", len(authDomain.Features))
	}
}

func TestDefaultChunkConfig(t *testing.T) {
	config := DefaultChunkConfig()

	if config.MaxTestsPerChunk != 250 {
		t.Errorf("MaxTestsPerChunk = %v, want 250", config.MaxTestsPerChunk)
	}

	if config.MaxTokensPerChunk != 25_000 {
		t.Errorf("MaxTokensPerChunk = %v, want 25000", config.MaxTokensPerChunk)
	}
}

func TestSplitIntoChunks_With250Threshold(t *testing.T) {
	config := DefaultChunkConfig()

	tests := []struct {
		name           string
		files          []specview.FileInfo
		wantChunkCount int
	}{
		{
			name: "under 250 - single chunk",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(100, 0)},
				{Path: "b.go", Tests: makeTests(100, 100)},
			},
			wantChunkCount: 1,
		},
		{
			name: "exactly 250 - single chunk",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(125, 0)},
				{Path: "b.go", Tests: makeTests(125, 125)},
			},
			wantChunkCount: 1,
		},
		{
			name: "over 250 - two chunks",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(150, 0)},
				{Path: "b.go", Tests: makeTests(150, 150)},
			},
			wantChunkCount: 2,
		},
		{
			name: "500 tests - two chunks",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(250, 0)},
				{Path: "b.go", Tests: makeTests(250, 250)},
			},
			wantChunkCount: 2,
		},
		{
			name: "750 tests - three chunks",
			files: []specview.FileInfo{
				{Path: "a.go", Tests: makeTests(250, 0)},
				{Path: "b.go", Tests: makeTests(250, 250)},
				{Path: "c.go", Tests: makeTests(250, 500)},
			},
			wantChunkCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := SplitIntoChunks(tt.files, config)

			if len(chunks) != tt.wantChunkCount {
				t.Errorf("SplitIntoChunks() chunk count = %v, want %v", len(chunks), tt.wantChunkCount)
			}
		})
	}
}

// makeTests creates test info with sequential indices starting from offset.
func makeTests(count, offset int) []specview.TestInfo {
	tests := make([]specview.TestInfo, count)
	for i := 0; i < count; i++ {
		tests[i] = specview.TestInfo{
			Index: offset + i,
			Name:  "test",
		}
	}
	return tests
}
