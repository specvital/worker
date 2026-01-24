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

	hash1 := GenerateContentHash(files, "English")
	hash2 := GenerateContentHash(files, "English")

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

	hashA := GenerateContentHash(filesOrderA, "English")
	hashB := GenerateContentHash(filesOrderB, "English")

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

	hashA := GenerateContentHash(filesOrderA, "English")
	hashB := GenerateContentHash(filesOrderB, "English")

	if !bytes.Equal(hashA, hashB) {
		t.Errorf("hash should be order-independent: different test order produced different hash")
	}
}

func TestGenerateContentHash_LanguageDifferent(t *testing.T) {
	files := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{{Index: 0, Name: "test"}}},
	}

	hashEN := GenerateContentHash(files, "English")
	hashKO := GenerateContentHash(files, "Korean")
	hashJA := GenerateContentHash(files, "Japanese")

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

	hashA := GenerateContentHash(filesA, "English")
	hashB := GenerateContentHash(filesB, "English")

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

	hashA := GenerateContentHash(filesA, "English")
	hashB := GenerateContentHash(filesB, "English")

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

	hashA := GenerateContentHash(filesA, "English")
	hashB := GenerateContentHash(filesB, "English")

	if bytes.Equal(hashA, hashB) {
		t.Error("adding a test should produce different hash")
	}
}

func TestGenerateContentHash_EmptyFiles(t *testing.T) {
	var emptyFiles []FileInfo

	hash := GenerateContentHash(emptyFiles, "English")

	if len(hash) != 32 {
		t.Errorf("expected SHA256 hash length 32 for empty input, got %d", len(hash))
	}
}

func TestGenerateContentHash_EmptyTests(t *testing.T) {
	files := []FileInfo{
		{Path: "src/test.ts", Tests: []TestInfo{}},
	}

	hash := GenerateContentHash(files, "English")

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

	hashUnix := GenerateContentHash(filesUnix, "English")
	hashWindows := GenerateContentHash(filesWindows, "English")

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

	hashNormal := GenerateContentHash(filesNormal, "English")
	hashWhitespace := GenerateContentHash(filesWhitespace, "English")

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

	hashWithout := GenerateContentHash(filesWithoutSuite, "English")
	hashWith := GenerateContentHash(filesWithSuite, "English")

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

	hashA := GenerateContentHash(filesA, "English")
	hashB := GenerateContentHash(filesB, "English")

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

func TestGenerateCacheKeyHash_Deterministic(t *testing.T) {
	key := BehaviorCacheKey{
		TestName:  "should login with valid credentials",
		SuitePath: "Authentication > Login",
		FilePath:  "src/auth/login_test.ts",
		Language:  "Korean",
		ModelID:   "gemini-2.5-flash",
	}

	hash1 := GenerateCacheKeyHash(key)
	hash2 := GenerateCacheKeyHash(key)

	if !bytes.Equal(hash1, hash2) {
		t.Error("hash should be deterministic: same input produced different hashes")
	}

	if len(hash1) != 32 {
		t.Errorf("expected SHA256 hash length 32, got %d", len(hash1))
	}
}

func TestGenerateCacheKeyHash_DifferentTestName(t *testing.T) {
	key1 := BehaviorCacheKey{
		TestName:  "should login",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	key2 := BehaviorCacheKey{
		TestName:  "should logout",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hash1 := GenerateCacheKeyHash(key1)
	hash2 := GenerateCacheKeyHash(key2)

	if bytes.Equal(hash1, hash2) {
		t.Error("different test names should produce different hashes")
	}
}

func TestGenerateCacheKeyHash_DifferentFilePath(t *testing.T) {
	key1 := BehaviorCacheKey{
		TestName:  "test_success",
		SuitePath: "",
		FilePath:  "auth/login.test.ts",
		Language:  "English",
		ModelID:   "gemini-2.5-flash",
	}
	key2 := BehaviorCacheKey{
		TestName:  "test_success",
		SuitePath: "",
		FilePath:  "payment/checkout.test.ts",
		Language:  "English",
		ModelID:   "gemini-2.5-flash",
	}

	hash1 := GenerateCacheKeyHash(key1)
	hash2 := GenerateCacheKeyHash(key2)

	if bytes.Equal(hash1, hash2) {
		t.Error("different file paths should produce different hashes")
	}
}

func TestGenerateCacheKeyHash_UnicodeNFCNormalization(t *testing.T) {
	// é as single codepoint (U+00E9) vs composed form (U+0065 + U+0301)
	keyNFC := BehaviorCacheKey{
		TestName:  "test caf\u00e9", // é as single codepoint
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	keyNFD := BehaviorCacheKey{
		TestName:  "test cafe\u0301", // e + combining acute accent
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hashNFC := GenerateCacheKeyHash(keyNFC)
	hashNFD := GenerateCacheKeyHash(keyNFD)

	if !bytes.Equal(hashNFC, hashNFD) {
		t.Error("NFC normalization should produce same hash for equivalent Unicode sequences")
	}
}

func TestGenerateCacheKeyHash_DifferentLanguage(t *testing.T) {
	key1 := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	key2 := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "Korean",
		ModelID:   "model",
	}

	hash1 := GenerateCacheKeyHash(key1)
	hash2 := GenerateCacheKeyHash(key2)

	if bytes.Equal(hash1, hash2) {
		t.Error("different languages should produce different hashes")
	}
}

func TestGenerateCacheKeyHash_DifferentModelID(t *testing.T) {
	key1 := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "gemini-2.5-flash",
	}
	key2 := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "gemini-2.5-flash-lite",
	}

	hash1 := GenerateCacheKeyHash(key1)
	hash2 := GenerateCacheKeyHash(key2)

	if bytes.Equal(hash1, hash2) {
		t.Error("different model IDs should produce different hashes")
	}
}

func TestGenerateCacheKeyHash_DifferentSuitePath(t *testing.T) {
	key1 := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "SuiteA",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	key2 := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "SuiteB",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hash1 := GenerateCacheKeyHash(key1)
	hash2 := GenerateCacheKeyHash(key2)

	if bytes.Equal(hash1, hash2) {
		t.Error("different suite paths should produce different hashes")
	}
}

func TestGenerateCacheKeyHash_EmptySuitePath(t *testing.T) {
	keyWithSuite := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "Suite",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	keyWithoutSuite := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hashWith := GenerateCacheKeyHash(keyWithSuite)
	hashWithout := GenerateCacheKeyHash(keyWithoutSuite)

	if bytes.Equal(hashWith, hashWithout) {
		t.Error("different suite paths (empty vs non-empty) should produce different hashes")
	}
}

func TestGenerateCacheKeyHash_SuitePathNormalization(t *testing.T) {
	keyNormal := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "SuiteA > SuiteB",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	keyWhitespace := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "  SuiteA > SuiteB  ",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hashNormal := GenerateCacheKeyHash(keyNormal)
	hashWhitespace := GenerateCacheKeyHash(keyWhitespace)

	if !bytes.Equal(hashNormal, hashWhitespace) {
		t.Error("suite path normalization should produce same hash for equivalent paths")
	}
}

func TestGenerateCacheKeyHash_TestNameNormalization(t *testing.T) {
	keyNormal := BehaviorCacheKey{
		TestName:  "should login",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	keyWhitespace := BehaviorCacheKey{
		TestName:  "  should  login  ",
		SuitePath: "",
		FilePath:  "test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hashNormal := GenerateCacheKeyHash(keyNormal)
	hashWhitespace := GenerateCacheKeyHash(keyWhitespace)

	if !bytes.Equal(hashNormal, hashWhitespace) {
		t.Error("test name normalization should produce same hash for equivalent names")
	}
}

func TestGenerateCacheKeyHash_FilePathNormalization(t *testing.T) {
	keyUnix := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "src/auth/login_test.ts",
		Language:  "English",
		ModelID:   "model",
	}
	keyWindows := BehaviorCacheKey{
		TestName:  "test",
		SuitePath: "",
		FilePath:  "src\\auth\\login_test.ts",
		Language:  "English",
		ModelID:   "model",
	}

	hashUnix := GenerateCacheKeyHash(keyUnix)
	hashWindows := GenerateCacheKeyHash(keyWindows)

	if !bytes.Equal(hashUnix, hashWindows) {
		t.Error("file path normalization should produce same hash for unix and windows paths")
	}
}
