package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/provider/agent"
	"github.com/shahar-caura/forge/internal/provider/notifier"
	"github.com/shahar-caura/forge/internal/provider/tracker"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
	"github.com/shahar-caura/forge/internal/state"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error("forge failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: forge <run|resume|runs>")
	}

	switch os.Args[1] {
	case "run":
		return cmdRun(logger)
	case "resume":
		return cmdResume(logger)
	case "runs":
		return cmdRuns(logger)
	case "steps":
		return cmdSteps()
	case "completion":
		return cmdCompletion()
	default:
		return fmt.Errorf("usage: forge <run|resume|runs|steps|completion>")
	}
}

func cmdRun(logger *slog.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge run <plan.md>")
	}

	planPath := os.Args[2]

	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("plan file: %w", err)
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Generate run ID: YYYYMMDD-HHMMSS-<slug>
	slug := pipeline.SlugFromTitle(filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
	runID := time.Now().Format("20060102-150405") + "-" + slug

	rs := state.New(runID, planPath)
	if err := rs.Save(); err != nil {
		return fmt.Errorf("saving initial run state: %w", err)
	}

	logger.Info("starting run", "id", runID, "plan", planPath)

	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipelineErr := pipeline.Run(ctx, cfg, providers, planPath, rs, logger)

	// Best-effort cleanup of old completed runs.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

func cmdResume(logger *slog.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge resume <run-id> [--from <step-name>]")
	}

	runID := os.Args[2]

	// Parse optional --from flag.
	var fromStep string
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--from" {
			if i+1 >= len(os.Args) {
				return fmt.Errorf("--from requires a step name")
			}
			fromStep = os.Args[i+1]
			break
		}
	}

	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	if _, err := os.Stat(rs.PlanPath); err != nil {
		return fmt.Errorf("plan file %q: %w", rs.PlanPath, err)
	}

	if fromStep != "" {
		idx, ok := state.StepIndex(fromStep)
		if !ok {
			return fmt.Errorf("unknown step %q; valid steps: %s", fromStep, strings.Join(state.StepNames, ", "))
		}
		rs.ResetFrom(idx)
		logger.Info("resuming from step", "step", state.StepNames[idx])
	} else {
		if rs.Status == state.RunCompleted {
			return fmt.Errorf("run %q already completed; use --from <step> to re-run from a specific step", runID)
		}
		// Reset failed status to active for re-run.
		rs.Status = state.RunActive
		// Reset any failed steps to pending so they get re-run.
		for i := range rs.Steps {
			if rs.Steps[i].Status == state.StepFailed {
				rs.Steps[i].Status = state.StepPending
				rs.Steps[i].Error = ""
			}
		}
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("resuming run", "id", runID, "plan", rs.PlanPath)

	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipelineErr := pipeline.Run(ctx, cfg, providers, rs.PlanPath, rs, logger)

	// Best-effort cleanup.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

func cmdRuns(logger *slog.Logger) error {
	runs, err := state.List()
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	fmt.Printf("%-30s  %-10s  %-20s  %s\n", "ID", "STATUS", "CREATED", "PLAN")
	for _, r := range runs {
		fmt.Printf("%-30s  %-10s  %-20s  %s\n",
			r.ID,
			r.Status,
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.PlanPath,
		)
	}

	_ = logger // unused but kept for consistency
	return nil
}

func cmdSteps() error {
	for i, name := range state.StepNames {
		fmt.Printf("%2d  %s\n", i, name)
	}
	return nil
}

func cmdCompletion() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge completion <zsh|bash>")
	}
	switch os.Args[2] {
	case "zsh":
		fmt.Print(zshCompletion())
	case "bash":
		fmt.Print(bashCompletion())
	default:
		return fmt.Errorf("unsupported shell %q; supported: zsh, bash", os.Args[2])
	}
	return nil
}

func stepNamesHyphenated() []string {
	out := make([]string, len(state.StepNames))
	for i, name := range state.StepNames {
		out[i] = strings.ReplaceAll(name, " ", "-")
	}
	return out
}

func zshCompletion() string {
	steps := stepNamesHyphenated()
	return `#compdef forge

_forge() {
  local -a commands
  commands=(run resume runs steps completion)

  local -a steps
  steps=(` + strings.Join(steps, " ") + `)

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi

  case "${words[2]}" in
    run)
      _files -g '*.md'
      ;;
    resume)
      if [[ "${words[CURRENT-1]}" == "--from" || "${words[CURRENT-1]}" == "--f" ]]; then
        _describe 'step' steps
      elif (( CURRENT == 3 )); then
        local -a run_ids
        run_ids=(${(f)"$(ls .forge/runs/*.yaml 2>/dev/null | xargs -I{} basename {} .yaml)"})
        _describe 'run-id' run_ids
      else
        compadd -- --from
      fi
      ;;
    completion)
      _describe 'shell' '(zsh bash)'
      ;;
  esac
}

compdef _forge forge
`
}

func bashCompletion() string {
	steps := stepNamesHyphenated()
	return `_forge() {
  local cur prev commands steps
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  commands="run resume runs steps completion"
  steps="` + strings.Join(steps, " ") + `"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    return
  fi

  case "${COMP_WORDS[1]}" in
    run)
      COMPREPLY=( $(compgen -f -X '!*.md' -- "${cur}") )
      ;;
    resume)
      if [[ "${prev}" == "--from" ]]; then
        COMPREPLY=( $(compgen -W "${steps}" -- "${cur}") )
      elif [[ ${COMP_CWORD} -eq 2 ]]; then
        local run_ids
        run_ids=$(ls .forge/runs/*.yaml 2>/dev/null | xargs -I{} basename {} .yaml)
        COMPREPLY=( $(compgen -W "${run_ids}" -- "${cur}") )
      else
        COMPREPLY=( $(compgen -W "--from" -- "${cur}") )
      fi
      ;;
    completion)
      COMPREPLY=( $(compgen -W "zsh bash" -- "${cur}") )
      ;;
  esac
}

complete -F _forge forge
`
}

func wireProviders(cfg *config.Config, logger *slog.Logger) (pipeline.Providers, error) {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return pipeline.Providers{}, fmt.Errorf("resolving repo root: %w", err)
	}

	p := pipeline.Providers{
		Worktree: worktree.New(
			cfg.Worktree.CreateCmd,
			cfg.Worktree.RemoveCmd,
			cfg.Worktree.Cleanup,
			repoRoot,
			logger,
		),
		Agent: agent.New(cfg.Agent.Timeout.Duration, logger),
		VCS:   vcs.New(cfg.VCS.Repo, logger),
	}

	if cfg.Tracker.Provider != "" {
		p.Tracker = tracker.New(cfg.Tracker.BaseURL, cfg.Tracker.Project, cfg.Tracker.Email, cfg.Tracker.Token, cfg.Tracker.BoardID)
	}

	if cfg.Notifier.Provider != "" {
		p.Notifier = notifier.New(cfg.Notifier.WebhookURL)
	}

	return p, nil
}
