// Package config loads prdash's optional YAML config. Today it carries only the
// review-launcher settings: which agent provider to spawn and the prompt
// template to seed it with.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/packethog/prdash/internal/pr"
)

// Review is the parsed, validated review-launcher config. The zero value is a
// disabled feature (Enabled reports false).
type Review struct {
	Provider string
	Args     []string // extra flags passed to the provider before the prompt (e.g. --permission-mode auto)
	Prompt   string
	tmpl     *template.Template
}

type fileSchema struct {
	Review struct {
		Provider string   `yaml:"provider"`
		Args     []string `yaml:"args"`
		Prompt   string   `yaml:"prompt"`
	} `yaml:"review"`
}

func validProvider(p string) bool { return p == "claude" || p == "codex" }

// Parse validates provider and compiles the prompt template.
func Parse(provider, prompt string) (Review, error) {
	if !validProvider(provider) {
		return Review{}, fmt.Errorf("review.provider %q must be \"claude\" or \"codex\"", provider)
	}
	if prompt == "" {
		return Review{}, errors.New("review.prompt is empty")
	}
	tmpl, err := template.New("review").Option("missingkey=error").Parse(prompt)
	if err != nil {
		return Review{}, fmt.Errorf("review.prompt template: %w", err)
	}
	// Reject prompts that reference unknown fields now (executed against zero
	// data), so an invalid prompt is disabled at load time, not at first keypress.
	if err := tmpl.Execute(io.Discard, tmplData{}); err != nil {
		return Review{}, fmt.Errorf("review.prompt template: %w", err)
	}
	return Review{Provider: provider, Prompt: prompt, tmpl: tmpl}, nil
}

func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "prdash", "config.yaml"), nil
}

// Load reads the review config from the XDG config path. A missing file yields a
// disabled Review and no error. A present-but-invalid file (bad YAML, unknown
// provider, empty/unparseable prompt) yields a disabled Review and a non-nil
// error so the caller can print a one-line note; prdash still starts.
func Load() (Review, error) {
	path, err := configPath()
	if err != nil {
		return Review{}, nil // cannot resolve a config location → treat as no config
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Review{}, nil
		}
		return Review{}, fmt.Errorf("config: %w", err)
	}
	var f fileSchema
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Review{}, fmt.Errorf("config: %w", err)
	}
	if f.Review.Provider == "" && f.Review.Prompt == "" {
		return Review{}, nil // no review section
	}
	r, err := Parse(f.Review.Provider, f.Review.Prompt)
	if err != nil {
		return Review{}, fmt.Errorf("config: %w", err)
	}
	r.Args = f.Review.Args
	return r, nil
}

// Enabled reports whether a valid review config was loaded.
func (r Review) Enabled() bool { return r.tmpl != nil && validProvider(r.Provider) }

type tmplData struct {
	URL    string
	Repo   string
	Title  string
	Branch string
	Number int
}

// Render executes the prompt template against the selected PR.
func (r Review) Render(p pr.PR) (string, error) {
	if r.tmpl == nil {
		return "", errors.New("review not configured")
	}
	var b bytes.Buffer
	if err := r.tmpl.Execute(&b, tmplData{
		URL: p.URL, Repo: p.Repo, Title: p.Title, Branch: p.HeadRefName, Number: p.Number,
	}); err != nil {
		return "", err
	}
	return b.String(), nil
}
