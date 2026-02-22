package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/shahar-caura/forge/internal/state"
	"gopkg.in/yaml.v3"
)

// RepoEntry represents a single registered repository.
type RepoEntry struct {
	Path     string    `yaml:"path"`
	Name     string    `yaml:"name"`
	LastUsed time.Time `yaml:"last_used"`
}

// RepoRuns holds runs for a single registered repo.
type RepoRuns struct {
	Repo RepoEntry
	Runs []*state.RunState
}

type registryFile struct {
	Repos []RepoEntry `yaml:"repos"`
}

var overridePath string

// SetPath overrides the registry file path (for testing).
func SetPath(path string) { overridePath = path }

// registryPath returns the path to the global registry file.
func registryPath() string {
	if overridePath != "" {
		return overridePath
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "forge", "repos.yaml")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "forge", "repos.yaml")
}

// Touch upserts a repo entry in the global registry.
// Best-effort: silently ignores errors so it never blocks the caller.
func Touch(repoPath string) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return
	}

	reg := load()

	found := false
	for i := range reg.Repos {
		if reg.Repos[i].Path == abs {
			reg.Repos[i].LastUsed = time.Now()
			found = true
			break
		}
	}
	if !found {
		reg.Repos = append(reg.Repos, RepoEntry{
			Path:     abs,
			Name:     filepath.Base(abs),
			LastUsed: time.Now(),
		})
	}

	_ = save(reg)
}

// List returns all registered repos sorted by last_used descending.
func List() ([]RepoEntry, error) {
	reg := load()
	sort.Slice(reg.Repos, func(i, j int) bool {
		return reg.Repos[i].LastUsed.After(reg.Repos[j].LastUsed)
	})
	return reg.Repos, nil
}

// ListRuns loads runs from all registered repos.
// Skips repos that no longer exist or have no runs.
func ListRuns() ([]RepoRuns, error) {
	repos, err := List()
	if err != nil {
		return nil, err
	}

	var result []RepoRuns
	for _, repo := range repos {
		runsDir := filepath.Join(repo.Path, ".forge", "runs")
		entries, err := filepath.Glob(filepath.Join(runsDir, "*.yaml"))
		if err != nil || len(entries) == 0 {
			continue
		}

		var runs []*state.RunState
		for _, path := range entries {
			rs, err := state.LoadFile(path)
			if err != nil {
				continue
			}
			runs = append(runs, rs)
		}
		if len(runs) > 0 {
			sort.Slice(runs, func(i, j int) bool {
				return runs[i].CreatedAt.After(runs[j].CreatedAt)
			})
			result = append(result, RepoRuns{Repo: repo, Runs: runs})
		}
	}
	return result, nil
}

// Remove unregisters a repo by path.
func Remove(repoPath string) error {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	reg := load()
	filtered := reg.Repos[:0]
	for _, r := range reg.Repos {
		if r.Path != abs {
			filtered = append(filtered, r)
		}
	}
	reg.Repos = filtered
	return save(reg)
}

func load() registryFile {
	var reg registryFile
	data, err := os.ReadFile(registryPath())
	if err != nil {
		return reg
	}
	_ = yaml.Unmarshal(data, &reg)
	return reg
}

func save(reg registryFile) error {
	path := registryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating registry dir: %w", err)
	}

	data, err := yaml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing registry temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming registry file: %w", err)
	}
	return nil
}
