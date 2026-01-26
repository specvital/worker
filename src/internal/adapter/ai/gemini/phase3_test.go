package gemini

import (
	"testing"
)

func TestParsePhase3Response(t *testing.T) {
	t.Run("should parse valid JSON response", func(t *testing.T) {
		text := `{"summary": "This project covers authentication and payment domains."}`

		output, err := parsePhase3Response(text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output.Summary != "This project covers authentication and payment domains." {
			t.Errorf("got %q, want %q", output.Summary, "This project covers authentication and payment domains.")
		}
	})

	t.Run("should return error for empty summary", func(t *testing.T) {
		text := `{"summary": ""}`

		_, err := parsePhase3Response(text)
		if err == nil {
			t.Fatal("expected error for empty summary")
		}
	})

	t.Run("should return error for invalid JSON", func(t *testing.T) {
		text := `not json at all`

		_, err := parsePhase3Response(text)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("should return error for missing summary field", func(t *testing.T) {
		text := `{"other": "field"}`

		_, err := parsePhase3Response(text)
		if err == nil {
			t.Fatal("expected error for missing summary field")
		}
	})
}
