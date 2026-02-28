package intent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// testHelper is a test binary approach: when invoked as a subprocess, it
// writes the value of TEST_CLAUDE_OUTPUT to stdout and exits with TEST_CLAUDE_EXIT.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	out := os.Getenv("TEST_CLAUDE_OUTPUT")
	_, _ = os.Stdout.WriteString(out)
	if os.Getenv("TEST_CLAUDE_EXIT") == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

func fakeCommandContext(output string, exitErr bool) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--"}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"TEST_CLAUDE_OUTPUT="+output,
		)
		if exitErr {
			cmd.Env = append(cmd.Env, "TEST_CLAUDE_EXIT=1")
		}
		return cmd
	}
}

func TestClassify_Success(t *testing.T) {
	// Envelope wrapping actual JSON result.
	envelope := `{"result":"{\"argv\":[\"run\",\"plans/auth.md\"],\"confidence\":0.95,\"reasoning\":\"user wants to run auth plan\"}"}`

	orig := CommandContext
	CommandContext = fakeCommandContext(envelope, false)
	defer func() { CommandContext = orig }()

	r, err := Classify(context.Background(), "run the auth plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Argv) != 2 || r.Argv[0] != "run" || r.Argv[1] != "plans/auth.md" {
		t.Fatalf("unexpected argv: %v", r.Argv)
	}
	if r.Confidence < 0.9 {
		t.Fatalf("unexpected confidence: %f", r.Confidence)
	}
}

func TestClassify_NoClaude(t *testing.T) {
	// Override PATH so claude is not found.
	t.Setenv("PATH", "")

	_, err := Classify(context.Background(), "anything")
	if !errors.Is(err, ErrNoClaude) {
		t.Fatalf("expected ErrNoClaude, got: %v", err)
	}
}

func TestClassify_ExitError(t *testing.T) {
	orig := CommandContext
	CommandContext = fakeCommandContext("something went wrong", true)
	defer func() { CommandContext = orig }()

	_, err := Classify(context.Background(), "do something")
	if !errors.Is(err, ErrClassificationFailed) {
		t.Fatalf("expected ErrClassificationFailed, got: %v", err)
	}
}

func TestClassify_MalformedJSON(t *testing.T) {
	orig := CommandContext
	CommandContext = fakeCommandContext(`{"result":"not json at all"}`, false)
	defer func() { CommandContext = orig }()

	_, err := Classify(context.Background(), "do something")
	if !errors.Is(err, ErrClassificationFailed) {
		t.Fatalf("expected ErrClassificationFailed, got: %v", err)
	}
}

func TestClassify_EmptyArgv(t *testing.T) {
	orig := CommandContext
	CommandContext = fakeCommandContext(`{"result":"{\"argv\":[],\"confidence\":0.1,\"reasoning\":\"unclear\"}"}`, false)
	defer func() { CommandContext = orig }()

	_, err := Classify(context.Background(), "do something")
	if !errors.Is(err, ErrClassificationFailed) {
		t.Fatalf("expected ErrClassificationFailed, got: %v", err)
	}
}

func TestClassify_CodeFencedJSON(t *testing.T) {
	// Claude sometimes wraps JSON in code fences.
	fenced := "{\"result\":\"```json\\n{\\\"argv\\\":[\\\"runs\\\"],\\\"confidence\\\":0.9,\\\"reasoning\\\":\\\"list runs\\\"}\\n```\"}"

	orig := CommandContext
	CommandContext = fakeCommandContext(fenced, false)
	defer func() { CommandContext = orig }()

	r, err := Classify(context.Background(), "show my runs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Argv) != 1 || r.Argv[0] != "runs" {
		t.Fatalf("unexpected argv: %v", r.Argv)
	}
}

func TestParseResponse_DirectJSON(t *testing.T) {
	// No envelope, direct JSON.
	r, err := parseResponse(`{"argv":["status","abc123"],"confidence":0.85,"reasoning":"check status"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Argv) != 2 || r.Argv[0] != "status" {
		t.Fatalf("unexpected argv: %v", r.Argv)
	}
}

func TestClassify_LowConfidence(t *testing.T) {
	envelope := `{"result":"{\"argv\":[\"run\",\"plans/auth.md\"],\"confidence\":0.2,\"reasoning\":\"not sure\"}"}`

	orig := CommandContext
	CommandContext = fakeCommandContext(envelope, false)
	defer func() { CommandContext = orig }()

	_, err := Classify(context.Background(), "maybe run something")
	if !errors.Is(err, ErrClassificationFailed) {
		t.Fatalf("expected ErrClassificationFailed, got: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "confidence") {
		t.Fatalf("expected confidence-related error, got: %v", err)
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no fences", `{"argv":["runs"]}`, `{"argv":["runs"]}`},
		{"with fences", "```json\n{\"argv\":[\"runs\"]}\n```", `{"argv":["runs"]}`},
		{"with bare fences", "```\n{\"argv\":[\"runs\"]}\n```", `{"argv":["runs"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.in)
			if got != tt.want {
				t.Fatalf("stripCodeFences(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
