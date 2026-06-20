package gh

import (
	"context"
	"testing"
)

func TestListRunsArgsAndDecode(t *testing.T) {
	out := `[
	  {"databaseId":4821,"number":4821,"headBranch":"main","status":"completed","conclusion":"success","url":"https://x/4821","createdAt":"2026-06-20T18:00:00Z","updatedAt":"2026-06-20T18:12:00Z"},
	  {"databaseId":4820,"number":4820,"headBranch":"main","status":"completed","conclusion":"failure","url":"https://x/4820","createdAt":"2026-06-20T16:00:00Z","updatedAt":"2026-06-20T16:12:00Z"}
	]`
	f := &fakeRunner{out: []byte(out)}
	wr, err := ListRuns(context.Background(), f, "malbeclabs/infra", "qa.mainnet-beta.yml", "QA mainnet-beta", "main", 5)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if wr.Name != "QA mainnet-beta" || wr.Repo != "malbeclabs/infra" || wr.Key != "qa.mainnet-beta.yml" {
		t.Errorf("metadata wrong: %+v", wr)
	}
	if len(wr.Runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(wr.Runs))
	}
	if wr.Runs[0].RunID != 4821 || wr.Runs[1].Conclusion != "failure" {
		t.Errorf("decode wrong: %+v", wr.Runs)
	}
	if wr.Runs[0].WorkflowName != "QA mainnet-beta" {
		t.Errorf("run WorkflowName not set: %q", wr.Runs[0].WorkflowName)
	}
	want := []string{
		"run", "list", "-R", "malbeclabs/infra",
		"--workflow", "qa.mainnet-beta.yml",
		"--branch", "main",
		"--limit", "5",
		"--json", "databaseId,number,headBranch,status,conclusion,url,createdAt,updatedAt",
	}
	if len(f.gotArgs) != 1 || !equalArgs(f.gotArgs[0], want) {
		t.Errorf("args = %v, want %v", f.gotArgs, want)
	}
}

func TestListRunsNoBranch(t *testing.T) {
	f := &fakeRunner{out: []byte(`[]`)}
	if _, err := ListRuns(context.Background(), f, "a/b", "w.yml", "w.yml", "", 3); err != nil {
		t.Fatal(err)
	}
	for _, a := range f.gotArgs[0] {
		if a == "--branch" {
			t.Error("no --branch flag expected when branch is empty")
		}
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
