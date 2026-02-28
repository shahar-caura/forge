package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/shahar-caura/forge/internal/state"
)

const defaultRunsDir = ".forge/runs"

var stateLoadMu sync.Mutex

// RepoRuns contains all run states discovered for a repository.
type RepoRuns struct {
	RepoPath string
	RepoName string
	Runs     []state.RunState
}

// ScanRepos walks root directories and returns run states for each discovered repo.
func ScanRepos(roots []string) ([]RepoRuns, error) {
	repos := make(map[string]RepoRuns)

	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}

		resolvedRoot, err := resolveRoot(root)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
				continue
			}
			return nil, fmt.Errorf("resolve root %q: %w", root, err)
		}

		err = filepath.WalkDir(resolvedRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrPermission) {
					return filepath.SkipDir
				}
				return nil
			}

			if d.Type()&os.ModeSymlink != 0 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			if path != resolvedRoot && isHiddenDir(d.Name()) {
				return filepath.SkipDir
			}

			if d.Name() == "runs" && filepath.Base(filepath.Dir(path)) == ".forge" {
				repoPath := filepath.Clean(filepath.Dir(filepath.Dir(path)))
				if _, exists := repos[repoPath]; exists {
					return filepath.SkipDir
				}

				runs, err := loadRuns(path)
				if err != nil {
					if errors.Is(err, fs.ErrPermission) {
						return filepath.SkipDir
					}
					return nil
				}

				repos[repoPath] = RepoRuns{
					RepoPath: repoPath,
					RepoName: filepath.Base(repoPath),
					Runs:     runs,
				}
				return filepath.SkipDir
			}

			return nil
		})
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				continue
			}
			return nil, fmt.Errorf("walk root %q: %w", resolvedRoot, err)
		}
	}

	out := make([]RepoRuns, 0, len(repos))
	for _, repo := range repos {
		out = append(out, repo)
	}
	sortRepoRuns(out)
	return out, nil
}

func sortRepoRuns(repos []RepoRuns) {
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].RepoPath < repos[j].RepoPath
	})
}

func loadRuns(runsDir string) ([]state.RunState, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	runs := make([]state.RunState, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}

		id := strings.TrimSuffix(name, ".yaml")
		rs, err := loadRun(runsDir, id)
		if err != nil {
			continue
		}
		runs = append(runs, *rs)
	}

	return runs, nil
}

func loadRun(runsDir, id string) (*state.RunState, error) {
	stateLoadMu.Lock()
	defer stateLoadMu.Unlock()

	state.SetRunsDir(runsDir)
	defer state.SetRunsDir(defaultRunsDir)

	return state.Load(id)
}

func resolveRoot(root string) (string, error) {
	path := root
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	} else if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
		return "", err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".") && name != ".forge"
}
