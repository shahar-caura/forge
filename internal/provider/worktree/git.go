package worktree

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type templateData struct {
	Branch     string
	BaseBranch string
	Path       string
}

// Git implements provider.Worktree using template commands.
type Git struct {
	CreateCmd string
	RemoveCmd string
	Cleanup   bool
	RepoRoot  string
	Logger    *slog.Logger

	// commandContext is overridable for testing.
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// New creates a new Git worktree provider.
func New(createCmd, removeCmd string, cleanup bool, repoRoot string, logger *slog.Logger) *Git {
	return &Git{
		CreateCmd:      createCmd,
		RemoveCmd:      removeCmd,
		Cleanup:        cleanup,
		RepoRoot:       repoRoot,
		Logger:         logger,
		commandContext: exec.CommandContext,
	}
}

func (g *Git) Create(ctx context.Context, branch, baseBranch string) (string, error) {
	wtPath := filepath.Join(g.RepoRoot, ".worktrees", branch)

	data := templateData{Branch: branch, BaseBranch: baseBranch, Path: wtPath}
	args, err := renderTemplate(g.CreateCmd, data)
	if err != nil {
		return "", fmt.Errorf("worktree create: rendering template: %w", err)
	}

	g.Logger.Info("creating worktree", "cmd", args, "path", wtPath)

	cmd := g.commandContext(ctx, args[0], args[1:]...)
	cmd.Dir = g.RepoRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("worktree create: %w: %s", err, strings.TrimSpace(string(out)))
	}

	g.Logger.Info("worktree created", "path", wtPath)
	return wtPath, nil
}

func (g *Git) Remove(ctx context.Context, path string) error {
	if !g.Cleanup {
		g.Logger.Info("worktree cleanup disabled, skipping remove", "path", path)
		return nil
	}

	args, err := renderTemplate(g.RemoveCmd, templateData{Path: path})
	if err != nil {
		return fmt.Errorf("worktree remove: rendering template: %w", err)
	}

	g.Logger.Info("removing worktree", "cmd", args)

	cmd := g.commandContext(ctx, args[0], args[1:]...)
	cmd.Dir = g.RepoRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}

	g.Logger.Info("worktree removed", "path", path)
	return nil
}

func renderTemplate(tmplStr string, data templateData) ([]string, error) {
	tmpl, err := template.New("cmd").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	fields := strings.Fields(buf.String())
	if len(fields) == 0 {
		return nil, fmt.Errorf("template produced empty command")
	}

	return fields, nil
}
