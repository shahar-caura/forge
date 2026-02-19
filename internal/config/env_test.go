package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvFile_Basic(t *testing.T) {
	data := []byte("KEY=value\nOTHER=stuff\n")
	m, err := ParseEnvFile(data)
	require.NoError(t, err)
	assert.Equal(t, "value", m["KEY"])
	assert.Equal(t, "stuff", m["OTHER"])
}

func TestParseEnvFile_CommentsAndBlanks(t *testing.T) {
	data := []byte("# comment\n\nKEY=value\n  # indented comment\n\nOTHER=stuff\n")
	m, err := ParseEnvFile(data)
	require.NoError(t, err)
	assert.Len(t, m, 2)
	assert.Equal(t, "value", m["KEY"])
	assert.Equal(t, "stuff", m["OTHER"])
}

func TestParseEnvFile_ValueWithEquals(t *testing.T) {
	data := []byte("URL=https://example.com?foo=bar&baz=qux\n")
	m, err := ParseEnvFile(data)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com?foo=bar&baz=qux", m["URL"])
}

func TestParseEnvFile_TrimSpaces(t *testing.T) {
	data := []byte("  KEY  =  value  \n")
	m, err := ParseEnvFile(data)
	require.NoError(t, err)
	assert.Equal(t, "value", m["KEY"])
}

func TestParseEnvFile_MissingEquals(t *testing.T) {
	data := []byte("BADLINE\n")
	_, err := ParseEnvFile(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing '='")
	assert.Contains(t, err.Error(), "line 1")
}

func TestParseEnvFile_EmptyValue(t *testing.T) {
	data := []byte("KEY=\n")
	m, err := ParseEnvFile(data)
	require.NoError(t, err)
	assert.Equal(t, "", m["KEY"])
}

func TestLoadEnvFiles_ProjectOverridesGlobal(t *testing.T) {
	// Set up global env file.
	globalDir := filepath.Join(t.TempDir(), "forge")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	globalFile := filepath.Join(globalDir, "env")
	require.NoError(t, os.WriteFile(globalFile, []byte("GLOBAL_ONLY=from_global\nSHARED=from_global\n"), 0o644))

	// Set up project env file in a temp working dir.
	projectDir := t.TempDir()
	projectFile := filepath.Join(projectDir, ".forge.env")
	require.NoError(t, os.WriteFile(projectFile, []byte("PROJECT_ONLY=from_project\nSHARED=from_project\n"), 0o644))

	// Clear any pre-existing values.
	os.Unsetenv("GLOBAL_ONLY")
	os.Unsetenv("PROJECT_ONLY")
	os.Unsetenv("SHARED")
	t.Cleanup(func() {
		os.Unsetenv("GLOBAL_ONLY")
		os.Unsetenv("PROJECT_ONLY")
		os.Unsetenv("SHARED")
	})

	// Merge: global first, project overwrites.
	merged := make(map[string]string)
	mergeEnvFile(merged, globalFile)
	mergeEnvFile(merged, projectFile)

	for k, v := range merged {
		os.Setenv(k, v)
	}

	assert.Equal(t, "from_global", os.Getenv("GLOBAL_ONLY"))
	assert.Equal(t, "from_project", os.Getenv("PROJECT_ONLY"))
	assert.Equal(t, "from_project", os.Getenv("SHARED"), "project should override global")
}

func TestLoadEnvFiles_ActualEnvWins(t *testing.T) {
	// Set up a project env file.
	projectDir := t.TempDir()
	projectFile := filepath.Join(projectDir, ".forge.env")
	require.NoError(t, os.WriteFile(projectFile, []byte("MY_VAR=from_file\n"), 0o644))

	// Simulate: actual env is set before loading.
	t.Setenv("MY_VAR", "from_actual_env")

	// Snapshot original keys (MY_VAR is present).
	origKeys := map[string]bool{"MY_VAR": true}

	merged := make(map[string]string)
	mergeEnvFile(merged, projectFile)

	for k, v := range merged {
		if !origKeys[k] {
			os.Setenv(k, v)
		}
	}

	assert.Equal(t, "from_actual_env", os.Getenv("MY_VAR"), "actual env should win over file")
}

func TestMergeEnvFile_MissingFile(t *testing.T) {
	// Should not panic — silently skipped.
	merged := make(map[string]string)
	mergeEnvFile(merged, "/nonexistent/path/.forge.env")
	assert.Empty(t, merged)
}

func TestGlobalEnvPath_ReturnsPath(t *testing.T) {
	p := GlobalEnvPath()
	assert.Contains(t, p, "forge")
	assert.True(t, filepath.IsAbs(p))
}
