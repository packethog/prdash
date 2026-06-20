package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packethog/prdash/internal/pr"
)

func TestParseValid(t *testing.T) {
	for _, prov := range []string{"claude", "codex"} {
		r, err := Parse(prov, "review {{.URL}}")
		if err != nil {
			t.Fatalf("provider %q: %v", prov, err)
		}
		if !r.Enabled() {
			t.Errorf("provider %q: want enabled", prov)
		}
	}
}

func TestParseRejectsBadProvider(t *testing.T) {
	if _, err := Parse("gpt", "x {{.URL}}"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestParseRejectsEmptyPrompt(t *testing.T) {
	if _, err := Parse("claude", ""); err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestParseRejectsBadTemplate(t *testing.T) {
	if _, err := Parse("claude", "review {{.URL"); err == nil {
		t.Fatal("expected error for unparseable template")
	}
}

func TestParseRejectsUnknownTemplateField(t *testing.T) {
	if _, err := Parse("claude", "review {{.Missing}}"); err == nil {
		t.Fatal("expected error for unknown template field")
	}
}

func TestRenderSubstitutesFields(t *testing.T) {
	r, err := Parse("claude", "{{.Repo}}#{{.Number}} {{.URL}} {{.Branch}} {{.Title}}")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(pr.PR{
		URL: "https://u", Repo: "o/r", Number: 7, Title: "T", HeadRefName: "feat",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "o/r#7 https://u feat T" {
		t.Errorf("render = %q", got)
	}
}

func TestZeroReviewDisabled(t *testing.T) {
	var r Review
	if r.Enabled() {
		t.Error("zero Review must be disabled")
	}
}

func TestLoadMissingFileIsDisabledNoError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // empty dir: no config.yaml
	r, _, err := Load()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if r.Enabled() {
		t.Error("missing file should be disabled")
	}
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	pdir := filepath.Join(dir, "prdash")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	body := "review:\n  provider: claude\n  prompt: \"go {{.URL}}\"\n"
	writeConfig(t, dir, body)
	r, _, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !r.Enabled() || r.Provider != "claude" {
		t.Errorf("loaded review = %+v enabled=%v", r, r.Enabled())
	}
}

func TestLoadInvalidFileIsDisabledWithError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	body := "review:\n  provider: nope\n  prompt: x\n"
	writeConfig(t, dir, body)
	r, _, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
	if r.Enabled() {
		t.Error("invalid file should be disabled")
	}
}

func TestLoadFileWithoutReviewTableIsDisabledNoError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Present file, but no review section at all.
	writeConfig(t, dir, "# empty\n")
	r, _, err := Load()
	if err != nil {
		t.Fatalf("absent review section should not error: %v", err)
	}
	if r.Enabled() {
		t.Error("absent review section should be disabled")
	}
}

func TestLoadMalformedYAMLIsDisabledWithError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeConfig(t, dir, "review:\n  provider: [unclosed\n")
	r, _, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if r.Enabled() {
		t.Error("malformed YAML should be disabled")
	}
}

// TestLoadUnknownKeyIsError verifies that a misspelled key (which KnownFields
// would silently ignore with plain yaml.Unmarshal) is now rejected.
func TestLoadUnknownKeyIsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// "boguskey" is not in fileSchema — strict decoding must reject it.
	writeConfig(t, dir, "boguskey: oops\n")
	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown YAML key")
	}
}

// TestLoadUnknownWorkflowKeyIsError verifies that a misspelled field inside a
// workflow entry (e.g. "summaryartifact" instead of "summaryArtifact") is
// caught by strict decoding.
func TestLoadUnknownWorkflowKeyIsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	body := "ci:\n  workflows:\n    - repo: a/b\n      workflow: w.yml\n      summaryartifact: oops\n"
	writeConfig(t, dir, body)
	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error for misspelled workflow key summaryartifact")
	}
}

// TestLoadEmptyFileIsDisabledNoError verifies that an empty file (io.EOF from
// the decoder) is treated as disabled+no-error, not as a decode failure.
func TestLoadEmptyFileIsDisabledNoError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeConfig(t, dir, "")
	r, c, err := Load()
	if err != nil {
		t.Fatalf("empty file should not error: %v", err)
	}
	if r.Enabled() || c.Enabled() {
		t.Error("empty file should yield disabled features")
	}
}

// TestLoadTOMLWarning verifies that when config.yaml is absent but config.toml
// exists in the same directory, Load returns an actionable migration error.
func TestLoadTOMLWarning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Create the prdash subdir and a config.toml (no config.yaml).
	pdir := filepath.Join(dir, "prdash")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "config.toml"), []byte("[review]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load()
	if err == nil {
		t.Fatal("expected migration error when config.toml exists but config.yaml does not")
	}
	if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("error should mention config.toml, got: %v", err)
	}
}

func TestLoadReadsArgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	body := "review:\n  provider: claude\n  args: [\"--permission-mode\", \"auto\"]\n  prompt: \"go {{.URL}}\"\n"
	writeConfig(t, dir, body)
	r, _, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Args) != 2 || r.Args[0] != "--permission-mode" || r.Args[1] != "auto" {
		t.Errorf("Args = %v", r.Args)
	}
}

func TestLoadCI(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prdash"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
ci:
  limit: 5
  provider: claude
  prompt: "Debug {{.URL}} run {{.RunID}}"
  workflows:
    - repo: malbeclabs/infra
      workflow: qa.mainnet-beta.yml
      name: QA mainnet-beta
      branch: main
      summaryArtifact: qa-analysis-*
    - repo: malbeclabs/infra
      workflow: qa.testnet.yml
      limit: 10
`
	if err := os.WriteFile(filepath.Join(dir, "prdash", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.Enabled() {
		t.Fatal("CI should be enabled")
	}
	if !c.DebugEnabled() {
		t.Fatal("debug should be enabled")
	}
	if len(c.Workflows) != 2 {
		t.Fatalf("want 2 workflows, got %d", len(c.Workflows))
	}
	w0 := c.Workflows[0]
	if w0.Name != "QA mainnet-beta" || w0.SummaryFile != "analysis.txt" {
		t.Errorf("w0 defaults wrong: %+v", w0)
	}
	if c.LimitFor(c.Workflows[0]) != 5 || c.LimitFor(c.Workflows[1]) != 10 {
		t.Errorf("LimitFor wrong: %d %d", c.LimitFor(c.Workflows[0]), c.LimitFor(c.Workflows[1]))
	}
	// w1 has no name -> defaults to the workflow file
	if c.Workflows[1].Name != "qa.testnet.yml" {
		t.Errorf("w1 name default wrong: %q", c.Workflows[1].Name)
	}
}

func TestLoadCIRender(t *testing.T) {
	c, err := parseCI(ciInput{Limit: 5, Provider: "claude", Prompt: "Debug {{.URL}} {{.Workflow}} {{.RunID}}"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(RunInfo{URL: "u", Workflow: "QA", RunID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Debug u QA 42" {
		t.Errorf("render = %q", got)
	}
}

func TestLoadCIInvalidProviderDisablesDebug(t *testing.T) {
	c, err := parseCI(ciInput{Limit: 5, Provider: "bogus", Prompt: "x", Workflows: []Workflow{{Repo: "a/b", Workflow: "w.yml"}}})
	if err == nil {
		t.Fatal("want error for bad provider")
	}
	_ = c
}
