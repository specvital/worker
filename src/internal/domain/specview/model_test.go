package specview

import (
	"errors"
	"testing"
)

func TestSpecViewRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     SpecViewRequest
		wantErr error
	}{
		{
			name: "valid request",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   LanguageEN,
			},
			wantErr: nil,
		},
		{
			name: "valid request with model ID",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   LanguageKO,
				ModelID:    "gemini-2.5-flash",
			},
			wantErr: nil,
		},
		{
			name: "empty analysis ID",
			req: SpecViewRequest{
				AnalysisID: "",
				Language:   LanguageEN,
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty language",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "unsupported language",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "fr",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "language case sensitive - uppercase invalid",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "EN",
			},
			wantErr: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLanguage_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		language Language
		want     bool
	}{
		{name: "english valid", language: LanguageEN, want: true},
		{name: "korean valid", language: LanguageKO, want: true},
		{name: "japanese valid", language: LanguageJA, want: true},
		{name: "empty invalid", language: "", want: false},
		{name: "french invalid", language: "fr", want: false},
		{name: "uppercase EN invalid", language: "EN", want: false},
		{name: "mixed case invalid", language: "En", want: false},
		{name: "chinese invalid", language: "zh", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.language.IsValid()
			if got != tt.want {
				t.Errorf("Language(%q).IsValid() = %v, want %v", tt.language, got, tt.want)
			}
		})
	}
}
