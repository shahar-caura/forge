package intent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shahar-caura/forge/internal/state"
)

func TestGatherContext_WithPlanFiles(t *testing.T) {
	// Create temp dir with plan files.
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "auth.md"), []byte("# Auth"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "deploy.md"), []byte("# Deploy"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Point state to empty dir so List() returns nothing.
	state.SetRunsDir(filepath.Join(dir, "no-runs"))

	dc := GatherContext(dir)
	if len(dc.PlanFiles) != 2 {
		t.Fatalf("expected 2 plan files, got %d", len(dc.PlanFiles))
	}
	if len(dc.RunIDs) != 0 {
		t.Fatalf("expected 0 run IDs, got %d", len(dc.RunIDs))
	}
}

func TestGatherContext_NoPlanFiles(t *testing.T) {
	dir := t.TempDir()

	state.SetRunsDir(filepath.Join(dir, "no-runs"))

	dc := GatherContext(dir)
	if len(dc.PlanFiles) != 0 {
		t.Fatalf("expected 0 plan files, got %d", len(dc.PlanFiles))
	}
}

func TestFormatForPrompt_WithContent(t *testing.T) {
	dc := DynamicContext{
		PlanFiles: []string{"auth.md", "deploy.md"},
		RunIDs:    []string{"abc123", "def456"},
	}
	out := FormatForPrompt(dc)

	if !strings.Contains(out, "plans/auth.md") {
		t.Fatal("expected plan file in output")
	}
	if !strings.Contains(out, "plans/deploy.md") {
		t.Fatal("expected plan file in output")
	}
	if !strings.Contains(out, "abc123") {
		t.Fatal("expected run ID in output")
	}
	if !strings.Contains(out, "def456") {
		t.Fatal("expected run ID in output")
	}
}

func TestFormatForPrompt_Empty(t *testing.T) {
	dc := DynamicContext{}
	out := FormatForPrompt(dc)
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}
