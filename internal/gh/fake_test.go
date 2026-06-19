package gh

import "context"

// fakeRunner records the args it was called with and returns canned output.
type fakeRunner struct {
	out     []byte
	err     error
	gotArgs [][]string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.gotArgs = append(f.gotArgs, append([]string(nil), args...))
	return f.out, f.err
}
