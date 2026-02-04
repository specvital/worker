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
			name: "valid request with English",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "English",
				UserID:     "user-123",
			},
			wantErr: nil,
		},
		{
			name: "valid request with Korean",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "Korean",
				ModelID:    "gemini-2.5-flash",
				UserID:     "user-123",
			},
			wantErr: nil,
		},
		{
			name: "valid request with any language",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "Chinese",
				UserID:     "user-123",
			},
			wantErr: nil,
		},
		{
			name: "empty analysis ID",
			req: SpecViewRequest{
				AnalysisID: "",
				Language:   "English",
				UserID:     "user-123",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty user ID",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "English",
				UserID:     "",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty language",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "",
				UserID:     "user-123",
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
		{name: "English valid", language: "English", want: true},
		{name: "Korean valid", language: "Korean", want: true},
		{name: "Japanese valid", language: "Japanese", want: true},
		{name: "Chinese valid", language: "Chinese", want: true},
		{name: "Spanish valid", language: "Spanish", want: true},
		{name: "any string valid", language: "AnyLanguage", want: true},
		{name: "empty invalid", language: "", want: false},
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
