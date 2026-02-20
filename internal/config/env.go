package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFiles loads .forge.env files into the process environment.
// Load order (later wins): global (~/.config/forge/env), then project (.forge.env).
// Actual environment variables always win — keys already set before loading are never overwritten.
func LoadEnvFiles() {
	// Snapshot keys present in the actual environment before we touch anything.
	origKeys := make(map[string]bool)
	for _, entry := range os.Environ() {
		if k, _, ok := strings.Cut(entry, "="); ok {
			origKeys[k] = true
		}
	}

	// Merge both files: global first, project overwrites.
	merged := make(map[string]string)
	mergeEnvFile(merged, GlobalEnvPath())
	mergeEnvFile(merged, ".forge.env")

	// Set only keys that weren't in the original environment.
	for k, v := range merged {
		if !origKeys[k] {
			_ = os.Setenv(k, v)
		}
	}
}

// mergeEnvFile reads a KEY=VALUE file and merges into dst (later call overwrites earlier).
// Silently skips missing or unreadable files.
func mergeEnvFile(dst map[string]string, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	envs, err := ParseEnvFile(data)
	if err != nil {
		return
	}
	for k, v := range envs {
		dst[k] = v
	}
}

// ParseEnvFile parses KEY=VALUE lines from data.
// Blank lines and lines starting with # are skipped.
func ParseEnvFile(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: missing '=' in %q", lineNum, line)
		}
		result[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return result, scanner.Err()
}

// GlobalEnvPath returns the path to the global forge env file.
func GlobalEnvPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "forge", "env")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "forge", "env")
}
