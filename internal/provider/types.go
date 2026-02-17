package provider

import "context"

// PR represents a pull request created by the VCS provider.
type PR struct {
	URL    string
	Number int
}

// Issue represents a tracker issue (Phase 2).
type Issue struct {
	Key   string
	URL   string
	Title string
}

// Comment represents a PR review comment (Phase 2).
type Comment struct {
	ID     string
	Author string
	Body   string
}

// VCS handles version control operations (commit, push, pull requests).
type VCS interface {
	CommitAndPush(ctx context.Context, dir, branch, message string) error
	CreatePR(ctx context.Context, branch, baseBranch, title, body string) (*PR, error)
}

// Agent runs an AI coding agent with a prompt in a working directory.
type Agent interface {
	Run(ctx context.Context, dir, prompt string) error
}

// Worktree manages isolated working directories for parallel development.
type Worktree interface {
	Create(ctx context.Context, branch, baseBranch string) (path string, err error)
	Remove(ctx context.Context, path string) error
}

// Tracker manages issue tracking (Phase 2).
type Tracker interface {
	CreateIssue(ctx context.Context, title, body string) (*Issue, error)
}

// Notifier sends notifications (Phase 2).
type Notifier interface {
	Notify(ctx context.Context, message string) error
}
