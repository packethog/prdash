package gh

import (
	"context"
	"errors"
	"strings"
)

// NewCmuxRunner returns an ExecRunner targeting the cmux binary.
func NewCmuxRunner() ExecRunner { return ExecRunner{Bin: "cmux"} }

func surfaceArgs(provider string) []string {
	return []string{"new-surface", "--type", "agent-session", "--provider", provider, "--focus", "true"}
}

func sendArgs(ref, text string) []string { return []string{"send", "--surface", ref, "--", text} }

func enterArgs(ref string) []string { return []string{"send-key", "--surface", ref, "enter"} }

// parseSurfaceRef pulls the surface ref from `cmux new-surface` stdout. cmux
// prints refs in its default format (e.g. "surface:4"); scan for that token so
// a stray warning or trailing line does not get mistaken for the ref.
func parseSurfaceRef(out []byte) (string, error) {
	for _, f := range strings.Fields(string(out)) {
		if strings.HasPrefix(f, "surface:") {
			return f, nil
		}
	}
	return "", errors.New("cmux new-surface returned no surface ref")
}

// StartReview spawns a cmux agent surface for the given provider and injects the
// prompt: new-surface (capture ref) -> send prompt -> send-key enter. It does no
// cloning, review, or GitHub posting — the spawned agent does all of that.
func StartReview(ctx context.Context, cmux Runner, provider, prompt string) error {
	out, err := cmux.Run(ctx, surfaceArgs(provider)...)
	if err != nil {
		return err
	}
	ref, err := parseSurfaceRef(out)
	if err != nil {
		return err
	}
	if _, err := cmux.Run(ctx, sendArgs(ref, prompt)...); err != nil {
		return err
	}
	if _, err := cmux.Run(ctx, enterArgs(ref)...); err != nil {
		return err
	}
	return nil
}
