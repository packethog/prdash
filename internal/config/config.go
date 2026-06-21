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
	PRDebug struct {
		Provider string   `yaml:"provider"`
		Args     []string `yaml:"args"`
		Prompt   string   `yaml:"prompt"`
	} `yaml:"prDebug"`
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

// compilePRPrompt validates the provider and compiles+dry-runs a PR prompt
// template. label prefixes error messages (e.g. "review", "prDebug").
func compilePRPrompt(label, provider, prompt string) (*template.Template, error) {
	if !validProvider(provider) {
		return nil, fmt.Errorf("%s.provider %q must be \"claude\" or \"codex\"", label, provider)
	}
	if prompt == "" {
		return nil, fmt.Errorf("%s.prompt is empty", label)
	}
	tmpl, err := template.New(label).Option("missingkey=error").Parse(prompt)
	if err != nil {
		return nil, fmt.Errorf("%s.prompt template: %w", label, err)
	}
	if err := tmpl.Execute(io.Discard, tmplData{}); err != nil {
		return nil, fmt.Errorf("%s.prompt template: %w", label, err)
	}
	return tmpl, nil
}

// renderPRPrompt executes a compiled PR prompt template against a PR.
func renderPRPrompt(tmpl *template.Template, p pr.PR) (string, error) {
	if tmpl == nil {
		return "", errors.New("not configured")
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, tmplData{
		URL: p.URL, Repo: p.Repo, Title: p.Title, Branch: p.HeadRefName, Number: p.Number,
	}); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Parse validates provider and compiles the review prompt template.
func Parse(provider, prompt string) (Review, error) {
	tmpl, err := compilePRPrompt("review", provider, prompt)
	if err != nil {
		return Review{}, err
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
// caller can print a one-line note; prdash still starts. All errors are joined
// via errors.Join so all nil → nil.
func Load() (Review, CI, PRDebug, error) {
	path, err := configPath()
	if err != nil {
		return Review{}, CI{}, PRDebug{}, nil // cannot resolve a config location → treat as no config
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check for the old TOML config; if present, prompt migration.
			tomlPath := filepath.Join(filepath.Dir(path), "config.toml")
			if _, terr := os.Stat(tomlPath); terr == nil {
				return Review{}, CI{}, PRDebug{}, fmt.Errorf("config: found config.toml but prdash now uses config.yaml — please migrate")
			}
			return Review{}, CI{}, PRDebug{}, nil
		}
		return Review{}, CI{}, PRDebug{}, fmt.Errorf("config: %w", err)
	}
	var f fileSchema
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) { // empty file: no keys at all
			return Review{}, CI{}, PRDebug{}, nil
		}
		return Review{}, CI{}, PRDebug{}, fmt.Errorf("config: %w", err)
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

	var prd PRDebug
	var prdErr error
	if f.PRDebug.Provider != "" || f.PRDebug.Prompt != "" {
		prd, prdErr = ParsePRDebug(f.PRDebug.Provider, f.PRDebug.Prompt)
		if prdErr == nil {
			prd.Args = f.PRDebug.Args
		} else {
			prd = PRDebug{}
			prdErr = fmt.Errorf("config: %w", prdErr)
		}
	}

	return rev, c, prd, errors.Join(revErr, ciErr, prdErr)
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

// Render executes the review prompt template against the selected PR.
func (r Review) Render(p pr.PR) (string, error) {
	return renderPRPrompt(r.tmpl, p)
}

// PRDebug is the parsed, validated PR CI-failure debug-launcher config. The zero
// value is disabled (Enabled reports false).
type PRDebug struct {
	Provider string
	Args     []string
	Prompt   string
	tmpl     *template.Template
}

// ParsePRDebug validates provider and compiles the prDebug prompt template.
func ParsePRDebug(provider, prompt string) (PRDebug, error) {
	tmpl, err := compilePRPrompt("prDebug", provider, prompt)
	if err != nil {
		return PRDebug{}, err
	}
	return PRDebug{Provider: provider, Prompt: prompt, tmpl: tmpl}, nil
}

func (d PRDebug) Enabled() bool { return d.tmpl != nil && validProvider(d.Provider) }

// Render executes the prDebug prompt template against the selected PR.
func (d PRDebug) Render(p pr.PR) (string, error) { return renderPRPrompt(d.tmpl, p) }

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

const defaultCILimit = 5

// ciInput is the raw, pre-validation CI config (decoded YAML or test input).
type ciInput struct {
	Limit     int
	Provider  string
	Args      []string
	Prompt    string
	Workflows []Workflow
}

// clampLimit normalizes a configured run limit: a non-positive value falls back
// to the default; any positive value is used as-is (no upper bound — `gh run
// list` paginates large limits).
func clampLimit(n int) int {
	if n <= 0 {
		return defaultCILimit
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
