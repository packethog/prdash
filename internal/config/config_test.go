package config

import (
	"os"
	"path/filepath"
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
	r, err := Load()
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
	r, err := Load()
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
	r, err := Load()
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
	r, err := Load()
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
	r, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if r.Enabled() {
		t.Error("malformed YAML should be disabled")
	}
}

func TestLoadReadsArgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	body := "review:\n  provider: claude\n  args: [\"--permission-mode\", \"auto\"]\n  prompt: \"go {{.URL}}\"\n"
	writeConfig(t, dir, body)
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Args) != 2 || r.Args[0] != "--permission-mode" || r.Args[1] != "auto" {
		t.Errorf("Args = %v", r.Args)
	}
}
