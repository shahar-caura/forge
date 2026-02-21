package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/shahar-caura/forge/internal/state"
)

// Handlers implements the generated StrictServerInterface.
type Handlers struct {
	Version   string
	StartTime time.Time
	Logger    *slog.Logger
}

func (h *Handlers) GetHealth(_ context.Context, _ GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{
		Status:        "ok",
		Version:       h.Version,
		UptimeSeconds: int(time.Since(h.StartTime).Seconds()),
	}, nil
}

func (h *Handlers) ListRuns(_ context.Context, request ListRunsRequestObject) (ListRunsResponseObject, error) {
	runs, err := state.List()
	if err != nil {
		return nil, err
	}

	// Filter by status if provided.
	if request.Params.Status != nil {
		filtered := make([]*state.RunState, 0, len(runs))
		for _, r := range runs {
			if string(r.Status) == string(*request.Params.Status) {
				filtered = append(filtered, r)
			}
		}
		runs = filtered
	}

	total := len(runs)

	// Apply offset.
	offset := 0
	if request.Params.Offset != nil {
		offset = *request.Params.Offset
	}
	if offset > len(runs) {
		offset = len(runs)
	}
	runs = runs[offset:]

	// Apply limit.
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit > len(runs) {
		limit = len(runs)
	}
	runs = runs[:limit]

	apiRuns := make([]Run, len(runs))
	for i, r := range runs {
		apiRuns[i] = stateToRun(r)
	}

	return ListRuns200JSONResponse{
		Runs:  apiRuns,
		Total: total,
	}, nil
}

func (h *Handlers) GetRun(_ context.Context, request GetRunRequestObject) (GetRunResponseObject, error) {
	rs, err := state.Load(request.Id)
	if err != nil {
		return GetRun404JSONResponse{
			Code:    404,
			Message: "run not found",
		}, nil
	}

	return GetRun200JSONResponse(stateToRun(rs)), nil
}

// stateToRun converts a state.RunState to the API Run type.
func stateToRun(rs *state.RunState) Run {
	r := Run{
		Id:        rs.ID,
		PlanPath:  rs.PlanPath,
		Status:    RunStatus(rs.Status),
		CreatedAt: rs.CreatedAt,
		UpdatedAt: rs.UpdatedAt,
		Steps:     make([]StepState, len(rs.Steps)),
	}

	if rs.Mode != "" {
		r.Mode = &rs.Mode
	}
	if rs.Branch != "" {
		r.Branch = &rs.Branch
	}
	if rs.WorktreePath != "" {
		r.WorktreePath = &rs.WorktreePath
	}
	if rs.PRUrl != "" {
		r.PrUrl = &rs.PRUrl
	}
	if rs.PRNumber != 0 {
		r.PrNumber = &rs.PRNumber
	}
	if rs.IssueKey != "" {
		r.IssueKey = &rs.IssueKey
	}
	if rs.IssueURL != "" {
		r.IssueUrl = &rs.IssueURL
	}
	if rs.CRFeedback != "" {
		r.CrFeedback = &rs.CRFeedback
	}
	if rs.CRFixSummary != "" {
		r.CrFixSummary = &rs.CRFixSummary
	}
	if rs.PlanTitle != "" {
		r.PlanTitle = &rs.PlanTitle
	}
	if rs.SourceIssue != 0 {
		r.SourceIssue = &rs.SourceIssue
	}

	for i, step := range rs.Steps {
		s := StepState{
			Name:   step.Name,
			Status: StepStatus(step.Status),
		}
		if step.Error != "" {
			s.Error = &step.Error
		}
		r.Steps[i] = s
	}

	return r
}
