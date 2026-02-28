package intent

import (
	"strings"
	"testing"
)

func TestBuildPrompt_ContainsQuery(t *testing.T) {
	dc := DynamicContext{}
	prompt := BuildPrompt("run the auth plan", dc)

	if !strings.Contains(prompt, "run the auth plan") {
		t.Fatal("expected query in prompt")
	}
}

func TestBuildPrompt_ContainsSubcommands(t *testing.T) {
	dc := DynamicContext{}
	prompt := BuildPrompt("anything", dc)

	subcommands := []string{"forge run", "forge push", "forge resume", "forge runs",
		"forge status", "forge logs", "forge steps", "forge edit",
		"forge cleanup", "forge init", "forge completion", "forge serve"}

	for _, sub := range subcommands {
		if !strings.Contains(prompt, sub) {
			t.Fatalf("expected %q in prompt", sub)
		}
	}
}

func TestBuildPrompt_ContainsDynamicContext(t *testing.T) {
	dc := DynamicContext{
		PlanFiles: []string{"auth.md"},
		RunIDs:    []string{"run-xyz"},
	}
	prompt := BuildPrompt("do something", dc)

	if !strings.Contains(prompt, "plans/auth.md") {
		t.Fatal("expected plan file in prompt")
	}
	if !strings.Contains(prompt, "run-xyz") {
		t.Fatal("expected run ID in prompt")
	}
}

func TestBuildPrompt_NoDynamicContext(t *testing.T) {
	dc := DynamicContext{}
	prompt := BuildPrompt("do something", dc)

	// Should not contain section headers for empty context.
	if strings.Contains(prompt, "Available plan files") {
		t.Fatal("unexpected plan files section in prompt")
	}
	if strings.Contains(prompt, "Recent run IDs") {
		t.Fatal("unexpected run IDs section in prompt")
	}
}
