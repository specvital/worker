package fairness

import (
	"encoding/json"
	"log/slog"
)

const maxArgsByteSize = 64 * 1024 // 64KB - reasonable limit for job args

// UserJobExtractor extracts user identity from River job args.
type UserJobExtractor interface {
	ExtractUserID(encodedArgs []byte) string
}

// JSONArgsExtractor extracts user_id and tier from JSON-encoded job arguments.
// Compatible with both SpecView and Analysis worker args.
type JSONArgsExtractor struct{}

// ExtractUserID parses the user_id field from JSON-encoded args.
// Returns empty string if user_id is missing, invalid JSON, or args exceed size limit.
func (e *JSONArgsExtractor) ExtractUserID(encodedArgs []byte) string {
	if len(encodedArgs) > maxArgsByteSize {
		slog.Warn("job args exceed size limit, skipping fairness",
			"args_size", len(encodedArgs),
			"max_size", maxArgsByteSize,
		)
		return ""
	}

	var args struct {
		UserID *string `json:"user_id"` // pointer to detect null vs missing
	}

	if err := json.Unmarshal(encodedArgs, &args); err != nil {
		slog.Warn("failed to parse user_id from job args, skipping fairness",
			"error", err,
			"args_size", len(encodedArgs),
		)
		return ""
	}

	if args.UserID == nil {
		return ""
	}

	return *args.UserID
}
