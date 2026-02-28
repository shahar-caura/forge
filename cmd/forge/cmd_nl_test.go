package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestRunNaturalLanguage_EmptyArgs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := newRootCmd(logger)

	// Empty args should print help, not error.
	err := runNaturalLanguage(root, logger, []string{})
	if err != nil {
		t.Fatalf("expected no error for empty args, got: %v", err)
	}
}

func TestRunNaturalLanguage_RecursionGuard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := newRootCmd(logger)

	nlClassifying = true
	defer func() { nlClassifying = false }()

	err := runNaturalLanguage(root, logger, []string{"something"})
	if err == nil {
		t.Fatal("expected error from recursion guard")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected 'unknown command' error, got: %v", err)
	}
}

func TestRunNaturalLanguage_NoClaude(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := newRootCmd(logger)

	// Override PATH so claude is not found.
	t.Setenv("PATH", "")

	err := runNaturalLanguage(root, logger, []string{"run", "the", "auth", "plan"})
	if err == nil {
		t.Fatal("expected error when claude CLI is not available")
	}
	if !strings.Contains(err.Error(), "install claude CLI") {
		t.Fatalf("expected 'install claude CLI' hint, got: %v", err)
	}
}

func TestRootCmd_UnknownArgsTriggersNL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := newRootCmd(logger)

	// Without claude in PATH, unknown args should trigger NL which falls back to error.
	t.Setenv("PATH", "")
	root.SetArgs([]string{"do", "something", "cool"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown args without claude CLI")
	}
	if !strings.Contains(err.Error(), "install claude CLI") {
		t.Fatalf("expected NL classification error, got: %v", err)
	}
}
