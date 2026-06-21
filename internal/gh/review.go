package gh

import (
	"context"
	"errors"
	"strings"
)

// NewCmuxRunner returns an ExecRunner targeting the cmux binary.
func NewCmuxRunner() ExecRunner { return ExecRunner{Bin: "cmux"} }

// paneArgs opens a new terminal pane (split below the dashboard). A terminal
// pane — not an agent-session surface — is used so `cmux send`/`send-key` can
// type the review command into it; those only work on terminal surfaces.
func paneArgs() []string {
	return []string{"new-pane", "--type", "terminal", "--direction", "down", "--focus", "true"}
}

func sendArgs(ref, text string) []string { return []string{"send", "--surface", ref, "--", text} }

func enterArgs(ref string) []string { return []string{"send-key", "--surface", ref, "enter"} }

// shellQuote wraps s in single quotes so it survives the new pane's shell as a
// single argument, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// parseSurfaceRef pulls the surface ref from `cmux new-pane` stdout (e.g.
// "OK surface:32 pane:28 workspace:2"); scan for the surface token so the pane
// and workspace tokens, or any warning line, are not mistaken for it.
func parseSurfaceRef(out []byte) (string, error) {
	for _, f := range strings.Fields(string(out)) {
		if strings.HasPrefix(f, "surface:") {
			return f, nil
		}
	}
	return "", errors.New("cmux new-pane returned no surface ref")
}

// reviewCommand builds the shell command run in the new pane:
// `<provider> <args...> '<prompt>'`. Every part — including the provider — is
// shell-quoted so it survives the pane's shell as a single argument.
func reviewCommand(provider string, args []string, prompt string) string {
	parts := make([]string, 0, len(args)+2)
	parts = append(parts, shellQuote(provider))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	parts = append(parts, shellQuote(prompt))
	return strings.Join(parts, " ")
}

// startInPane opens a new terminal pane and types+runs command in it via the
// cmux new-pane → send → send-key enter dance.
func startInPane(ctx context.Context, cmux Runner, command string) error {
	out, err := cmux.Run(ctx, paneArgs()...)
	if err != nil {
		return err
	}
	ref, err := parseSurfaceRef(out)
	if err != nil {
		return err
	}
	if _, err := cmux.Run(ctx, sendArgs(ref, command)...); err != nil {
		return err
	}
	if _, err := cmux.Run(ctx, enterArgs(ref)...); err != nil {
		return err
	}
	return nil
}

// StartAgent opens a new terminal pane and runs `<provider> <args...> '<prompt>'`
// in it. It is the generic dispatcher behind StartReview and StartCIDebug.
func StartAgent(ctx context.Context, cmux Runner, provider string, args []string, prompt string) error {
	return startInPane(ctx, cmux, reviewCommand(provider, args, prompt))
}

// StartReview opens a new terminal pane and runs `<provider> <args...> '<prompt>'`
// in it: new-pane (capture the terminal's surface ref) -> send the command ->
// send-key enter to run it. args are provider flags (e.g. --permission-mode auto)
// inserted before the prompt. prdash does no cloning, review, or GitHub posting
// itself — the spawned command does all of that.
func StartReview(ctx context.Context, cmux Runner, provider string, args []string, prompt string) error {
	return StartAgent(ctx, cmux, provider, args, prompt)
}

// StartCIDebug spawns the configured provider in a new cmux pane to debug a
// failed CI run. Identical mechanism to StartReview.
func StartCIDebug(ctx context.Context, cmux Runner, provider string, args []string, prompt string) error {
	return StartAgent(ctx, cmux, provider, args, prompt)
}
