package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/specvital/collector/internal/service"

	_ "github.com/specvital/core/pkg/parser/strategies/all"
)

const TypeAnalyze = "analysis:analyze"

type AnalyzePayload struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type AnalyzeHandler struct {
	analysisSvc service.AnalysisService
}

func NewAnalyzeHandler(svc service.AnalysisService) *AnalyzeHandler {
	return &AnalyzeHandler{
		analysisSvc: svc,
	}
}

func (h *AnalyzeHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload AnalyzePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	slog.InfoContext(ctx, "processing analyze task",
		"owner", payload.Owner,
		"repo", payload.Repo,
	)

	if err := h.analysisSvc.Analyze(ctx, service.AnalyzeRequest{
		Owner: payload.Owner,
		Repo:  payload.Repo,
	}); err != nil {
		return err
	}

	slog.InfoContext(ctx, "analyze task completed",
		"owner", payload.Owner,
		"repo", payload.Repo,
	)

	return nil
}
