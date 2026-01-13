package specview

import (
	"bytes"
	"testing"
)

func TestGenerateContentHash_Deterministic(t *testing.T) {
	files := []FileInfo{
		{
			Path: "src/auth/login_test.ts",
			Tests: []TestInfo{
				{Index: 0, Name: "should login with valid credentials"},
				{Index: 1, Name: "should reject invalid password"},
			},
		},
		{
			Path: "src/user/profile_test.ts",
			Tests: []TestInfo{
				{Index: 2, Name: "should update profile"},
			},
		},
	}

	hash1 := GenerateContentHash(files, LanguageEN)
	hash2 := GenerateContentHash(files, LanguageEN)

	if !bytes.Equal(hash1, hash2) {
		t.Errorf("hash should be deterministic: got different hashes for same input")
	}

	if len(hash1) != 32 {
		t.Errorf("expected SHA256 hash length 32, got %d", len(hash1))
	}
}

func TestGenerateContentHash_OrderIndependent(t *testing.T) {
	filesOrderA := []FileInfo{
		{Path: "src/a_test.ts", Tests: []TestInfo{{Index: 0, Name: "test a"}}},
		{Path: "src/b_test.ts", Tests: []TestInfo{{Index: 1, Name: "test b"}}},
	}

	filesOrderB := []FileInfo{
		{Path: "src/b_test.ts", Tests: []TestInfo{{Index: 1, Name: "test b"}}},
		{Path: "src/a_test.ts", Tests: []TestInfo{{Index: 0, Name: "test a"}}},
	}

	hashA := GenerateContentHash(filesOrderA, LanguageEN)
	hashB := GenerateContentHash(filesOrderB, LanguageEN)

	if !bytes.Equal(hashA, hashB) {
		t.Errorf("hash should be order-independent: different file order produced different hash")
	}
}

func TestGenerateContentHash_TestOrderIndependent(t *testing.T) {
	filesOrderA := []FileInfo{
		{
			Path: "src/test.ts",
			Tests: []TestInfo{
				{Index: 0, Name: "should do X"},
				{Index: 1, Name: "should do Y"},
			},
		},
	}

	filesOrderB := []FileInfo{
		{
			Path: "src/test.ts",
			Tests: []TestInfo{
				{Index: 1, Name: "should do Y"},
				{Index: 0, Name: "should do X"},
			},
		},
	}

	hashA := GenerateContentHash(filesOrderA, LanguageEN)
	hashB := GenerateContentHash(filesOrderB, LanguageEN)

	if !bytes.Equal(hashA, hashB) {
		t.Errorf("hash should be order-independent: different test order produced different hash")
	}
}

func TestGenerateContentHash_LanguageDifferent(t *testing.T) {
	files := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}

	hashEN := GenerateContentHash(files, LanguageEN)
	hashKO := GenerateContentHash(files, LanguageKO)
	hashJA := GenerateContentHash(files, LanguageJA)

	if bytes.Equal(hashEN, hashKO) {
		t.Error("different languages should produce different hashes: EN == KO")
	}
	if bytes.Equal(hashEN, hashJA) {
		t.Error("different languages should produce different hashes: EN == JA")
	}
	if bytes.Equal(hashKO, hashJA) {
		t.Error("different languages should produce different hashes: KO == JA")
	}
}

func TestGenerateContentHash_FileNameChangeDifferent(t *testing.T) {
	filesA := []FileInfo{
		{Path: "src/old_test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}
	filesB := []FileInfo{
		{Path: "src/new_test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}

	hashA := GenerateContentHash(filesA, LanguageEN)
	hashB := GenerateContentHash(filesB, LanguageEN)

	if bytes.Equal(hashA, hashB) {
		t.Error("different file paths should produce different hashes")
	}
}

func TestGenerateContentHash_TestNameChangeDifferent(t *testing.T) {
	filesA := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "should do X"}}},
	}
	filesB := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "should do Y"}}},
	}

	hashA := GenerateContentHash(filesA, LanguageEN)
	hashB := GenerateContentHash(filesB, LanguageEN)

	if bytes.Equal(hashA, hashB) {
		t.Error("different test names should produce different hashes")
	}
}

func TestGenerateContentHash_TestAddedDifferent(t *testing.T) {
	filesA := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}
	filesB := []FileInfo{
		{
			Path: "src/test.ts",
			Tests: []TestInfo{
				{Index: 0, Name: "test"},
				{Index: 1, Name: "new test"},
			},
		},
	}

	hashA := GenerateContentHash(filesA, LanguageEN)
	hashB := GenerateContentHash(filesB, LanguageEN)

	if bytes.Equal(hashA, hashB) {
		t.Error("adding a test should produce different hash")
	}
}

func TestGenerateContentHash_EmptyFiles(t *testing.T) {
	var emptyFiles []FileInfo

	hash := GenerateContentHash(emptyFiles, LanguageEN)

	if len(hash) != 32 {
		t.Errorf("expected SHA256 hash length 32 for empty input, got %d", len(hash))
	}
}

func TestGenerateContentHash_EmptyTests(t *testing.T) {
	files := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{}},
	}

	hash := GenerateContentHash(files, LanguageEN)

	if len(hash) != 32 {
		t.Errorf("expected SHA256 hash length 32 for empty tests, got %d", len(hash))
	}
}

func TestNormalizeFilePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "unix path unchanged",
			path:     "src/auth/login_test.ts",
			expected: "src/auth/login_test.ts",
		},
		{
			name:     "windows backslash converted",
			path:     "src\\auth\\login_test.ts",
			expected: "src/auth/login_test.ts",
		},
		{
			name:     "leading slash removed",
			path:     "/src/auth/login_test.ts",
			expected: "src/auth/login_test.ts",
		},
		{
			name:     "redundant separators cleaned",
			path:     "src//auth///login_test.ts",
			expected: "src/auth/login_test.ts",
		},
		{
			name:     "dot segments cleaned",
			path:     "src/./auth/../auth/login_test.ts",
			expected: "src/auth/login_test.ts",
		},
		{
			name:     "empty path",
			path:     "",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeFilePath(tt.path)
			if got != tt.expected {
				t.Errorf("normalizeFilePath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestNormalizeTestName(t *testing.T) {
	tests := []struct {
		name     string
		testName string
		expected string
	}{
		{
			name:     "normal name unchanged",
			testName: "should login with valid credentials",
			expected: "should login with valid credentials",
		},
		{
			name:     "leading whitespace trimmed",
			testName: "  should login",
			expected: "should login",
		},
		{
			name:     "trailing whitespace trimmed",
			testName: "should login  ",
			expected: "should login",
		},
		{
			name:     "multiple internal spaces normalized",
			testName: "should  login   with    valid",
			expected: "should login with valid",
		},
		{
			name:     "tabs and newlines normalized",
			testName: "should\tlogin\nwith\r\nvalid",
			expected: "should login with valid",
		},
		{
			name:     "empty string",
			testName: "",
			expected: "",
		},
		{
			name:     "whitespace only",
			testName: "   \t\n   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTestName(tt.testName)
			if got != tt.expected {
				t.Errorf("normalizeTestName(%q) = %q, want %q", tt.testName, got, tt.expected)
			}
		})
	}
}

func TestGenerateContentHash_PathNormalization(t *testing.T) {
	filesUnix := []FileInfo{
		{Path: "src/auth/login_test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}
	filesWindows := []FileInfo{
		{Path: "src\\auth\\login_test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}

	hashUnix := GenerateContentHash(filesUnix, LanguageEN)
	hashWindows := GenerateContentHash(filesWindows, LanguageEN)

	if !bytes.Equal(hashUnix, hashWindows) {
		t.Error("path normalization should produce same hash for unix and windows paths")
	}
}

func TestGenerateContentHash_TestNameNormalization(t *testing.T) {
	filesNormal := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "should login"}}},
	}
	filesWhitespace := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "  should  login  "}}},
	}

	hashNormal := GenerateContentHash(filesNormal, LanguageEN)
	hashWhitespace := GenerateContentHash(filesWhitespace, LanguageEN)

	if !bytes.Equal(hashNormal, hashWhitespace) {
		t.Error("test name normalization should produce same hash for equivalent names")
	}
}

func TestGenerateContentHash_SuitePathAffectsHash(t *testing.T) {
	filesWithoutSuite := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "should do X"}}},
	}
	filesWithSuite := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "should do X", SuitePath: "SuiteA"}}},
	}

	hashWithout := GenerateContentHash(filesWithoutSuite, LanguageEN)
	hashWith := GenerateContentHash(filesWithSuite, LanguageEN)

	if bytes.Equal(hashWithout, hashWith) {
		t.Error("different suite paths should produce different hashes")
	}
}

func TestGenerateContentHash_DifferentSuitePathsDifferent(t *testing.T) {
	filesA := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "test", SuitePath: "SuiteA"}}},
	}
	filesB := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "test", SuitePath: "SuiteB"}}},
	}

	hashA := GenerateContentHash(filesA, LanguageEN)
	hashB := GenerateContentHash(filesB, LanguageEN)

	if bytes.Equal(hashA, hashB) {
		t.Error("different suite paths should produce different hashes")
	}
}

func TestGenerateContentHash_EmptyLanguage(t *testing.T) {
	files := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}

	hash := GenerateContentHash(files, "")

	if len(hash) != 32 {
		t.Errorf("expected SHA256 hash length 32 for empty language, got %d", len(hash))
	}
}
