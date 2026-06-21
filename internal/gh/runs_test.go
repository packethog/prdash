package gh

import (
	"context"
	"errors"
	"testing"
)

type seqCall struct {
	out []byte
	err error
}

type seqRunner struct {
	calls   []seqCall
	i       int
	gotArgs [][]string
}

func (s *seqRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	s.gotArgs = append(s.gotArgs, append([]string(nil), args...))
	c := s.calls[s.i]
	s.i++
	return c.out, c.err
}

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

func TestFetchRunDetail(t *testing.T) {
	out := `{
	  "status":"completed","conclusion":"failure","headBranch":"main","number":4820,
	  "url":"https://x/4820","createdAt":"2026-06-20T16:00:00Z","updatedAt":"2026-06-20T16:12:00Z",
	  "jobs":[
	    {"name":"qa","status":"completed","conclusion":"success","steps":[{"name":"build","status":"completed","conclusion":"success"}]},
	    {"name":"analyze","status":"completed","conclusion":"failure","steps":[
	      {"name":"setup","status":"completed","conclusion":"success"},
	      {"name":"run suite","status":"completed","conclusion":"failure"}]}
	  ]
	}`
	f := &fakeRunner{out: []byte(out)}
	d, err := FetchRunDetail(context.Background(), f, "malbeclabs/infra", 4820)
	if err != nil {
		t.Fatal(err)
	}
	if d.Conclusion != "failure" || d.Number != 4820 || len(d.Jobs) != 2 {
		t.Errorf("decode wrong: %+v", d)
	}
	job, step, ok := d.FailedStep()
	if !ok || job != "analyze" || step != "run suite" {
		t.Errorf("FailedStep = %q %q %v", job, step, ok)
	}
	want := []string{"run", "view", "4820", "-R", "malbeclabs/infra", "--json",
		"status,conclusion,headBranch,number,url,createdAt,updatedAt,jobs"}
	if !equalArgs(f.gotArgs[0], want) {
		t.Errorf("args = %v want %v", f.gotArgs[0], want)
	}
}

func TestRerunFailedArgs(t *testing.T) {
	f := &fakeRunner{}
	if err := RerunFailed(context.Background(), f, "malbeclabs/infra", 4820); err != nil {
		t.Fatal(err)
	}
	want := []string{"run", "rerun", "4820", "-R", "malbeclabs/infra", "--failed"}
	if !equalArgs(f.gotArgs[0], want) {
		t.Errorf("args = %v want %v", f.gotArgs[0], want)
	}
}

func TestRerunPRFailedFiltersAndReruns(t *testing.T) {
	list := `[
	  {"databaseId":1,"headSha":"abc","status":"completed","conclusion":"failure"},
	  {"databaseId":2,"headSha":"abc","status":"completed","conclusion":"success"},
	  {"databaseId":3,"headSha":"abc","status":"completed","conclusion":"timed_out"},
	  {"databaseId":4,"headSha":"old","status":"completed","conclusion":"failure"},
	  {"databaseId":5,"headSha":"abc","status":"in_progress","conclusion":""},
	  {"databaseId":6,"headSha":"abc","status":"completed","conclusion":"cancelled"}
	]`
	f := &fakeRunner{out: []byte(list)}
	n, err := RerunPRFailed(context.Background(), f, "o/r", "feat", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 { // only runs 1 and 3 qualify
		t.Fatalf("count = %d, want 2", n)
	}
	// call 0 = run list; calls 1..2 = rerun of ids 1 and 3 (order preserved)
	if len(f.gotArgs) != 3 {
		t.Fatalf("calls = %d, want 3 (1 list + 2 rerun)", len(f.gotArgs))
	}
	wantList := []string{"run", "list", "-R", "o/r", "--branch", "feat", "--limit", "50",
		"--json", "databaseId,headSha,status,conclusion"}
	if !equalArgs(f.gotArgs[0], wantList) {
		t.Errorf("list args = %v", f.gotArgs[0])
	}
	for i, id := range []string{"1", "3"} {
		want := []string{"run", "rerun", id, "-R", "o/r", "--failed"}
		if !equalArgs(f.gotArgs[i+1], want) {
			t.Errorf("rerun %d args = %v, want %v", i, f.gotArgs[i+1], want)
		}
	}
}

func TestRerunPRFailedZeroMatches(t *testing.T) {
	list := `[{"databaseId":2,"headSha":"abc","status":"completed","conclusion":"success"}]`
	f := &fakeRunner{out: []byte(list)}
	n, err := RerunPRFailed(context.Background(), f, "o/r", "feat", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
	if len(f.gotArgs) != 1 {
		t.Errorf("should make only the list call, got %d calls", len(f.gotArgs))
	}
}

// A rerun failure mid-way returns the count reran so far plus the error. Needs a
// runner that returns per-call outputs/errors (list ok, rerun #1 ok, rerun #2 err).
func TestRerunPRFailedReturnsCountOnError(t *testing.T) {
	list := `[
	  {"databaseId":1,"headSha":"abc","status":"completed","conclusion":"failure"},
	  {"databaseId":2,"headSha":"abc","status":"completed","conclusion":"failure"}
	]`
	s := &seqRunner{calls: []seqCall{
		{out: []byte(list)},       // run list
		{},                        // rerun id 1 → ok
		{err: errors.New("boom")}, // rerun id 2 → fails
	}}
	n, err := RerunPRFailed(context.Background(), s, "o/r", "feat", "abc")
	if err == nil {
		t.Fatal("want error from the failing rerun")
	}
	if n != 1 {
		t.Errorf("count = %d, want 1 (only the first rerun succeeded)", n)
	}
}
