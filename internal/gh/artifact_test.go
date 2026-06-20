package gh

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"
)

// makeZip builds an in-memory zip with one file.
func makeZip(t *testing.T, name, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// scriptRunner returns queued outputs in order, recording args.
type scriptRunner struct {
	outs    [][]byte
	i       int
	gotArgs [][]string
}

func (s *scriptRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	s.gotArgs = append(s.gotArgs, append([]string(nil), args...))
	out := s.outs[s.i]
	s.i++
	return out, nil
}

func TestFetchRunSummaryHappyPath(t *testing.T) {
	listJSON := `{"artifacts":[
	  {"id":11,"name":"qa-logs-100-1","created_at":"2026-06-20T16:00:00Z"},
	  {"id":22,"name":"qa-analysis-100","created_at":"2026-06-20T16:10:00Z"}
	]}`
	zipBytes := makeZip(t, "analysis.txt", "## QA Failure Analysis\nLikely cause: timeout")
	s := &scriptRunner{outs: [][]byte{[]byte(listJSON), zipBytes}}

	got, err := FetchRunSummary(context.Background(), s, "malbeclabs/infra", 100, "qa-analysis-*", "analysis.txt")
	if err != nil {
		t.Fatalf("FetchRunSummary: %v", err)
	}
	if !bytes.Contains(got, []byte("Likely cause")) {
		t.Errorf("body wrong: %q", got)
	}
	// second call must hit the matched artifact id (22), not 11
	if got := s.gotArgs[1]; got[len(got)-1] != "repos/malbeclabs/infra/actions/artifacts/22/zip" {
		t.Errorf("zip arg = %v", got)
	}
}

func TestFetchRunSummaryNoMatch(t *testing.T) {
	listJSON := `{"artifacts":[{"id":11,"name":"qa-logs-100-1","created_at":"2026-06-20T16:00:00Z"}]}`
	s := &scriptRunner{outs: [][]byte{[]byte(listJSON)}}
	_, err := FetchRunSummary(context.Background(), s, "a/b", 100, "qa-analysis-*", "analysis.txt")
	if !errors.Is(err, ErrNoArtifact) {
		t.Errorf("want ErrNoArtifact, got %v", err)
	}
}

func TestFetchRunSummaryMissingFile(t *testing.T) {
	listJSON := `{"artifacts":[{"id":22,"name":"qa-analysis-100","created_at":"2026-06-20T16:10:00Z"}]}`
	zipBytes := makeZip(t, "other.txt", "nope")
	s := &scriptRunner{outs: [][]byte{[]byte(listJSON), zipBytes}}
	_, err := FetchRunSummary(context.Background(), s, "a/b", 100, "qa-analysis-*", "analysis.txt")
	if err == nil || errors.Is(err, ErrNoArtifact) {
		t.Errorf("want file-missing error, got %v", err)
	}
}

// makeZipBytes builds an in-memory zip with one file whose body is the
// provided byte slice (allows large payloads without string allocation).
func makeZipBytes(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// Use Store (no compression) so the on-disk size == uncompressed size and
	// the central directory records the real UncompressedSize64.
	fh := &zip.FileHeader{Name: name, Method: zip.Store}
	w, err := zw.CreateHeader(fh)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestFetchRunSummaryTooLarge verifies that a zip entry whose uncompressed
// size exceeds maxSummaryBytes is rejected with an error (not silently
// loaded into memory).
func TestFetchRunSummaryTooLarge(t *testing.T) {
	listJSON := `{"artifacts":[{"id":22,"name":"qa-analysis-100","created_at":"2026-06-20T16:10:00Z"}]}`
	// Create a payload 1 byte over the cap so the check triggers.
	big := make([]byte, maxSummaryBytes+1)
	zipBytes := makeZipBytes(t, "analysis.txt", big)
	s := &scriptRunner{outs: [][]byte{[]byte(listJSON), zipBytes}}
	_, err := FetchRunSummary(context.Background(), s, "a/b", 100, "qa-analysis-*", "analysis.txt")
	if err == nil {
		t.Fatal("want error for oversized summary file, got nil")
	}
	if errors.Is(err, ErrNoArtifact) {
		t.Errorf("want size-limit error, got ErrNoArtifact")
	}
}
