// Package config loads prdash's optional YAML config. It carries the
// review-launcher settings and CI workflow tracking configuration.
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
	CI struct {
		Limit     int        `yaml:"limit"`
		Provider  string     `yaml:"provider"`
		Args      []string   `yaml:"args"`
		Prompt    string     `yaml:"prompt"`
		Workflows []wfSchema `yaml:"workflows"`
	} `yaml:"ci"`
}

type wfSchema struct {
	Repo            string `yaml:"repo"`
	Workflow        string `yaml:"workflow"`
	Name            string `yaml:"name"`
	Branch          string `yaml:"branch"`
	Limit           int    `yaml:"limit"`
	SummaryArtifact string `yaml:"summaryArtifact"`
	SummaryFile     string `yaml:"summaryFile"`
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

// Load reads the config from the XDG config path. A missing file yields disabled
// features and no error. A present-but-invalid section (bad YAML, unknown
// provider, etc.) disables only that feature and returns a non-nil error so the
// caller can print a one-line note; prdash still starts. Both errors are joined
// via errors.Join so both nil → nil.
func Load() (Review, CI, error) {
	path, err := configPath()
	if err != nil {
		return Review{}, CI{}, nil // cannot resolve a config location → treat as no config
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Review{}, CI{}, nil
		}
		return Review{}, CI{}, fmt.Errorf("config: %w", err)
	}
	var f fileSchema
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Review{}, CI{}, fmt.Errorf("config: %w", err)
	}

	var rev Review
	var revErr error
	if f.Review.Provider != "" || f.Review.Prompt != "" {
		rev, revErr = Parse(f.Review.Provider, f.Review.Prompt)
		if revErr == nil {
			rev.Args = f.Review.Args
		} else {
			rev = Review{}
			revErr = fmt.Errorf("config: %w", revErr)
		}
	}

	var c CI
	var ciErr error
	if len(f.CI.Workflows) > 0 || f.CI.Provider != "" || f.CI.Prompt != "" {
		in := ciInput{Limit: f.CI.Limit, Provider: f.CI.Provider, Args: f.CI.Args, Prompt: f.CI.Prompt}
		for _, w := range f.CI.Workflows {
			in.Workflows = append(in.Workflows, Workflow(w))
		}
		c, ciErr = parseCI(in)
		if ciErr != nil {
			c = CI{}
			ciErr = fmt.Errorf("config: %w", ciErr)
		}
	}

	return rev, c, errors.Join(revErr, ciErr)
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

// CI is the parsed CI-workflows config. The zero value is disabled.
type CI struct {
	Limit     int
	Provider  string
	Args      []string
	Workflows []Workflow
	tmpl      *template.Template
}

// Workflow is one configured GitHub Actions workflow to track.
type Workflow struct {
	Repo            string
	Workflow        string // workflow file name
	Name            string // display label (defaults to Workflow)
	Branch          string // optional branch filter
	Limit           int    // optional per-workflow override (0 = use CI.Limit)
	SummaryArtifact string // optional artifact name/glob holding the analysis
	SummaryFile     string // file within the artifact (defaults to analysis.txt)
}

const (
	defaultCILimit = 5
	maxCILimit     = 20
)

// ciInput is the raw, pre-validation CI config (decoded YAML or test input).
type ciInput struct {
	Limit     int
	Provider  string
	Args      []string
	Prompt    string
	Workflows []Workflow
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultCILimit
	}
	if n > maxCILimit {
		return maxCILimit
	}
	return n
}

// parseCI validates and normalizes a CI config. A bad provider or prompt is an
// error (the caller disables CI and prints a note). Workflows are normalized
// (default Name, default SummaryFile).
func parseCI(in ciInput) (CI, error) {
	c := CI{
		Limit:     clampLimit(in.Limit),
		Provider:  in.Provider,
		Args:      in.Args,
		Workflows: make([]Workflow, 0, len(in.Workflows)),
	}
	for _, w := range in.Workflows {
		if w.Repo == "" || w.Workflow == "" {
			return CI{}, fmt.Errorf("ci.workflows: each entry needs repo and workflow")
		}
		if w.Name == "" {
			w.Name = w.Workflow
		}
		if w.SummaryArtifact != "" && w.SummaryFile == "" {
			w.SummaryFile = "analysis.txt"
		}
		c.Workflows = append(c.Workflows, w)
	}
	// provider+prompt are only required for debug dispatch; validate when present.
	if in.Provider != "" || in.Prompt != "" {
		if !validProvider(in.Provider) {
			return CI{}, fmt.Errorf("ci.provider %q must be \"claude\" or \"codex\"", in.Provider)
		}
		if in.Prompt == "" {
			return CI{}, errors.New("ci.prompt is empty")
		}
		tmpl, err := template.New("ci").Option("missingkey=error").Parse(in.Prompt)
		if err != nil {
			return CI{}, fmt.Errorf("ci.prompt template: %w", err)
		}
		if err := tmpl.Execute(io.Discard, ciTmplData{}); err != nil {
			return CI{}, fmt.Errorf("ci.prompt template: %w", err)
		}
		c.tmpl = tmpl
	}
	return c, nil
}

func (c CI) Enabled() bool      { return len(c.Workflows) > 0 }
func (c CI) DebugEnabled() bool { return c.tmpl != nil && validProvider(c.Provider) }

// LimitFor returns the per-workflow run limit, falling back to the CI default.
func (c CI) LimitFor(w Workflow) int {
	if w.Limit > 0 {
		return clampLimit(w.Limit)
	}
	return c.Limit
}

// RunInfo is the data the debug prompt template is rendered against. It is a
// config-local type so config does not depend on internal/ci; the UI builds it
// from a ci.Run at dispatch time.
type RunInfo struct {
	URL        string
	Repo       string
	Workflow   string
	Branch     string
	Conclusion string
	RunID      int64
	RunNumber  int
}

// ciTmplData is the template's field set (also used for the load-time dry run).
type ciTmplData = RunInfo

// Render executes the debug prompt template against the selected run.
func (c CI) Render(d RunInfo) (string, error) {
	if c.tmpl == nil {
		return "", errors.New("ci debug not configured")
	}
	var b bytes.Buffer
	if err := c.tmpl.Execute(&b, d); err != nil {
		return "", err
	}
	return b.String(), nil
}
