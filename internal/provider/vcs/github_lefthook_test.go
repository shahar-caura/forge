package vcs

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitAndPush_LefthookDisabled(t *testing.T) {
	// The run() closure sets cmd.Env = append(os.Environ(), "LEFTHOOK=0")
	// AFTER commandContext returns. Verify by using shell commands that
	// fail unless LEFTHOOK=0 is in their environment.
	var cmdsRun []string

	g := New("owner/repo", testLogger())
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmdsRun = append(cmdsRun, name+" "+strings.Join(args, " "))
		return exec.CommandContext(ctx, "sh", "-c", `[ "$LEFTHOOK" = "0" ]`)
	}

	err := g.CommitAndPush(context.Background(), t.TempDir(), "feat-branch", "msg")
	require.NoError(t, err, "all commands should see LEFTHOOK=0")
	assert.Len(t, cmdsRun, 3, "git add + git commit + git push")
}

func TestAmendAndForcePush_LefthookDisabled(t *testing.T) {
	var cmdsRun []string

	g := New("owner/repo", testLogger())
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmdsRun = append(cmdsRun, name+" "+strings.Join(args, " "))
		return exec.CommandContext(ctx, "sh", "-c", `[ "$LEFTHOOK" = "0" ]`)
	}

	err := g.AmendAndForcePush(context.Background(), t.TempDir(), "feat-branch")
	require.NoError(t, err, "all commands should see LEFTHOOK=0")
	assert.Len(t, cmdsRun, 3, "git add + git commit --amend + git push")
}
