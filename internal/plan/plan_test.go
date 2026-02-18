package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidFrontmatter(t *testing.T) {
	content := "---\ntitle: Deploy Server\n---\nCreate a deployment server"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "Deploy Server", p.Title)
	assert.Equal(t, "Create a deployment server", p.Body)
}

func TestParse_NoFrontmatter(t *testing.T) {
	content := "Just a plain plan body"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Empty(t, p.Title)
	assert.Equal(t, content, p.Body)
}

func TestParse_UnclosedFrontmatter(t *testing.T) {
	content := "---\ntitle: Broken\nNo closing delimiter"
	_, err := Parse(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed frontmatter")
}

func TestParse_EmptyTitle(t *testing.T) {
	content := "---\ntitle: \"\"\n---\nBody here"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Empty(t, p.Title)
	assert.Equal(t, "Body here", p.Body)
}

func TestParse_ExtraFieldsIgnored(t *testing.T) {
	content := "---\ntitle: Auth\npriority: high\ntags: [go, api]\n---\nImplement auth"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "Auth", p.Title)
	assert.Equal(t, "Implement auth", p.Body)
}

func TestParse_TripleDashInBodyAfterFrontmatter(t *testing.T) {
	content := "---\ntitle: Test\n---\nBody with --- in it\nand more ---"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "Test", p.Title)
	assert.Equal(t, "Body with --- in it\nand more ---", p.Body)
}

func TestParse_EmptyBody(t *testing.T) {
	content := "---\ntitle: No Body\n---\n"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "No Body", p.Title)
	assert.Empty(t, p.Body)
}

func TestParse_MultilineBody(t *testing.T) {
	content := "---\ntitle: Multi\n---\nLine 1\nLine 2\nLine 3"
	p, err := Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "Multi", p.Title)
	assert.Equal(t, "Line 1\nLine 2\nLine 3", p.Body)
}
