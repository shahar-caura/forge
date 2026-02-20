package plan

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Plan holds parsed frontmatter and body from a plan file.
type Plan struct {
	Title string `yaml:"title"`
	Body  string // everything after closing ---
}

// Parse extracts YAML frontmatter from a plan file.
// If the content starts with "---\n", it looks for a closing "---" delimiter,
// unmarshals the YAML between them, and sets Body to the remainder.
// No frontmatter returns Plan{Body: content}. Unclosed frontmatter returns an error.
func Parse(content string) (*Plan, error) {
	if !strings.HasPrefix(content, "---\n") {
		return &Plan{Body: content}, nil
	}

	rest := content[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, fmt.Errorf("unclosed frontmatter: missing closing ---")
	}

	frontmatter := rest[:idx]
	// Skip "\n---" (4 chars) then optional newline.
	after := rest[idx+4:]
	after = strings.TrimPrefix(after, "\n")

	var p Plan
	if err := yaml.Unmarshal([]byte(frontmatter), &p); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	p.Body = after

	return &p, nil
}
