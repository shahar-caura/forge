package intent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shahar-caura/forge/internal/state"
)

// DynamicContext holds runtime information injected into the classification prompt.
type DynamicContext struct {
	PlanFiles []string
	RunIDs    []string
}

// GatherContext collects plan files and recent run IDs for prompt injection.
func GatherContext() DynamicContext {
	var dc DynamicContext

	plans, _ := filepath.Glob("plans/*.md")
	for _, p := range plans {
		dc.PlanFiles = append(dc.PlanFiles, filepath.Base(p))
	}

	runs, err := state.List()
	if err == nil {
		cap := min(10, len(runs))
		for _, r := range runs[:cap] {
			dc.RunIDs = append(dc.RunIDs, r.ID)
		}
	}

	return dc
}

// FormatForPrompt renders dynamic context as markdown for inclusion in the prompt.
func FormatForPrompt(dc DynamicContext) string {
	var sb strings.Builder

	if len(dc.PlanFiles) > 0 {
		sb.WriteString("## Available plan files\n")
		for _, f := range dc.PlanFiles {
			fmt.Fprintf(&sb, "- plans/%s\n", f)
		}
		sb.WriteString("\n")
	}

	if len(dc.RunIDs) > 0 {
		sb.WriteString("## Recent run IDs\n")
		for _, id := range dc.RunIDs {
			fmt.Fprintf(&sb, "- %s\n", id)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
