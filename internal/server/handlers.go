package server

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/shahar-caura/forge/internal/registry"
	"github.com/shahar-caura/forge/internal/state"
)

// Handlers implements the generated StrictServerInterface.
type Handlers struct {
	Version   string
	StartTime time.Time
	Logger    *slog.Logger
	MultiRepo bool // when true, aggregate runs from all registered repos
}

func (h *Handlers) GetHealth(_ context.Context, _ GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{
		Status:        "ok",
		Version:       h.Version,
		UptimeSeconds: int(time.Since(h.StartTime).Seconds()),
	}, nil
}

func (h *Handlers) ListRuns(_ context.Context, request ListRunsRequestObject) (ListRunsResponseObject, error) {
	runs, err := h.listAllRuns()
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
	rs, err := h.findRun(request.Id)
	if err != nil {
		return GetRun404JSONResponse{
			Code:    404,
			Message: "run not found",
		}, nil
	}

	return GetRun200JSONResponse(stateToRun(rs)), nil
}

// findRun looks up a run by ID, searching registered repos when in multi-repo mode.
func (h *Handlers) findRun(id string) (*state.RunState, error) {
	// Try local first (fast path).
	rs, err := state.Load(id)
	if err == nil {
		return rs, nil
	}
	if !h.MultiRepo {
		return nil, err
	}
	// Search registered repos.
	repoRuns, regErr := registry.ListRuns()
	if regErr != nil {
		return nil, err
	}
	for _, rr := range repoRuns {
		for _, r := range rr.Runs {
			if r.ID == id {
				return r, nil
			}
		}
	}
	return nil, err
}

// listAllRuns returns runs from the local directory or aggregated from all registered repos.
func (h *Handlers) listAllRuns() ([]*state.RunState, error) {
	if !h.MultiRepo {
		return state.List()
	}

	repoRuns, err := registry.ListRuns()
	if err != nil {
		return nil, err
	}

	// Flatten all runs from all repos, fall back to local if registry is empty.
	if len(repoRuns) == 0 {
		return state.List()
	}

	var all []*state.RunState
	for _, rr := range repoRuns {
		all = append(all, rr.Runs...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all, nil
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
