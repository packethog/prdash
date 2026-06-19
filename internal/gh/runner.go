// Package gh is prdash's seam to the GitHub CLI. All network access flows
// through Runner so callers can be tested with a fake.
package gh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Runner executes a gh subcommand and returns its stdout.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// ExecRunner runs the real gh binary.
type ExecRunner struct {
	Bin string // defaults to "gh" when empty
}

// NewExecRunner returns an ExecRunner targeting "gh".
func NewExecRunner() ExecRunner { return ExecRunner{Bin: "gh"} }

// Run executes `<Bin> <args...>`, returning stdout. On failure the error wraps
// the exit error and includes captured stderr for diagnostics.
func (e ExecRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := e.Bin
	if bin == "" {
		bin = "gh"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("%s %v: %w: %s", bin, args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
