package gh

import (
	"context"
	"testing"
)

func TestExecRunnerSuccess(t *testing.T) {
	r := ExecRunner{Bin: "/usr/bin/true"}
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("true should succeed, got %v", err)
	}
}

func TestExecRunnerFailure(t *testing.T) {
	r := ExecRunner{Bin: "/usr/bin/false"}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("false should return an error")
	}
}

func TestNewExecRunnerDefaultsBin(t *testing.T) {
	if NewExecRunner().Bin != "gh" {
		t.Errorf("default Bin = %q, want gh", NewExecRunner().Bin)
	}
}
