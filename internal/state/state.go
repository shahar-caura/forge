package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var runsDir = ".forge/runs"

// SetRunsDir overrides the default runs directory path.
func SetRunsDir(dir string) { runsDir = dir }

// StepStatus represents the execution state of an individual step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
)

// RunStatus represents the overall execution state of a run.
type RunStatus string

const (
	RunActive    RunStatus = "active"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
)

// StepNames defines the ordered pipeline steps.
var StepNames = []string{
	"read plan",
	"create issue",
	"generate branch",
	"create worktree",
	"run agent",
	"commit and push",
	"create pr",
	"poll cr",
	"fix cr",
	"push cr fix",
	"notify",
}

// StepState tracks status and error for a single pipeline step.
type StepState struct {
	Name   string     `yaml:"name"`
	Status StepStatus `yaml:"status"`
	Error  string     `yaml:"error,omitempty"`
}

// RunState is the persistent state for a single pipeline run.
type RunState struct {
	ID        string    `yaml:"id"`
	PlanPath  string    `yaml:"plan_path"`
	Mode      string    `yaml:"mode,omitempty"` // "" = "run", "push" = push
	Status    RunStatus `yaml:"status"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`

	// Artifacts accumulated across steps.
	Branch       string `yaml:"branch,omitempty"`
	WorktreePath string `yaml:"worktree_path,omitempty"`
	PRUrl        string `yaml:"pr_url,omitempty"`
	PRNumber     int    `yaml:"pr_number,omitempty"`
	IssueKey     string `yaml:"issue_key,omitempty"`
	IssueURL     string `yaml:"issue_url,omitempty"`
	CRFeedback   string `yaml:"cr_feedback,omitempty"`
	CRFixSummary string `yaml:"cr_fix_summary,omitempty"`
	PlanTitle    string `yaml:"plan_title,omitempty"`
	SourceIssue  int    `yaml:"source_issue,omitempty"`

	Steps []StepState `yaml:"steps"`
}

// New creates a RunState with all steps pending.
func New(id, planPath string) *RunState {
	now := time.Now()
	steps := make([]StepState, len(StepNames))
	for i, name := range StepNames {
		steps[i] = StepState{Name: name, Status: StepPending}
	}
	return &RunState{
		ID:        id,
		PlanPath:  planPath,
		Status:    RunActive,
		CreatedAt: now,
		UpdatedAt: now,
		Steps:     steps,
	}
}

// Load reads a RunState from .forge/runs/<id>.yaml.
func Load(id string) (*RunState, error) {
	path := filepath.Join(runsDir, id+".yaml")
	return LoadFile(path)
}

// LoadFile reads a RunState from an arbitrary file path.
func LoadFile(path string) (*RunState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading run state %q: %w", path, err)
	}

	var rs RunState
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("parsing run state %q: %w", path, err)
	}
	return &rs, nil
}

// Save writes the RunState atomically to .forge/runs/<id>.yaml.
func (s *RunState) Save() error {
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return fmt.Errorf("creating runs dir: %w", err)
	}

	s.UpdatedAt = time.Now()

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshaling run state: %w", err)
	}

	dest := filepath.Join(runsDir, s.ID+".yaml")
	tmp := dest + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

// List returns all run states sorted by created_at descending.
func List() ([]*RunState, error) {
	entries, err := filepath.Glob(filepath.Join(runsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("listing run states: %w", err)
	}

	var runs []*RunState
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // skip unreadable files
		}
		var rs RunState
		if err := yaml.Unmarshal(data, &rs); err != nil {
			continue // skip corrupt files
		}
		runs = append(runs, &rs)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	return runs, nil
}

// StepIndex returns the index of the named step, or -1 and false if not found.
// Accepts exact names ("commit and push") or hyphenated ("commit-and-push").
func StepIndex(name string) (int, bool) {
	normalized := strings.ReplaceAll(strings.ToLower(name), "-", " ")
	for i, s := range StepNames {
		if strings.ToLower(s) == normalized {
			return i, true
		}
	}
	return -1, false
}

// ResetFrom marks all steps before idx as completed and idx onward as pending.
// Sets run status to active.
func (s *RunState) ResetFrom(idx int) {
	for i := range s.Steps {
		if i < idx {
			s.Steps[i].Status = StepCompleted
			s.Steps[i].Error = ""
		} else {
			s.Steps[i].Status = StepPending
			s.Steps[i].Error = ""
		}
	}
	s.Status = RunActive
}

// Cleanup deletes completed run state files older than the given retention duration.
// Returns the number of files deleted.
func Cleanup(retention time.Duration) (int, error) {
	entries, err := filepath.Glob(filepath.Join(runsDir, "*.yaml"))
	if err != nil {
		return 0, fmt.Errorf("listing run states for cleanup: %w", err)
	}

	cutoff := time.Now().Add(-retention)
	deleted := 0

	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var rs RunState
		if err := yaml.Unmarshal(data, &rs); err != nil {
			continue
		}

		if rs.Status != RunCompleted {
			continue // only clean up completed runs
		}
		if rs.UpdatedAt.After(cutoff) {
			continue // not old enough
		}

		if err := os.Remove(path); err == nil {
			deleted++
		}
	}

	return deleted, nil
}
