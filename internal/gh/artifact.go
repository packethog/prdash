package gh

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"
)

// ErrNoArtifact is returned when no artifact name matches the configured glob.
var ErrNoArtifact = errors.New("no matching artifact")

type artifactNode struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type artifactList struct {
	Artifacts []artifactNode `json:"artifacts"`
}

// FetchRunSummary downloads the newest artifact whose name matches artifactGlob
// for the given run and returns the bytes of fileName inside it. The artifact zip
// is read entirely in memory (no temp files). Returns ErrNoArtifact when nothing
// matches the glob.
func FetchRunSummary(ctx context.Context, r Runner, repo string, runID int64, artifactGlob, fileName string) ([]byte, error) {
	// per_page=100 returns all artifacts for any realistic run in one page (a run
	// with >100 artifacts is implausible), avoiding multi-page JSON handling.
	out, err := r.Run(ctx, "api", fmt.Sprintf("repos/%s/actions/runs/%d/artifacts?per_page=100", repo, runID))
	if err != nil {
		return nil, err
	}
	var list artifactList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("decode artifacts: %w", err)
	}

	var best *artifactNode
	var bestT time.Time
	for i := range list.Artifacts {
		a := &list.Artifacts[i]
		// Match an exact name first; otherwise treat artifactGlob as a glob.
		// A malformed pattern is surfaced, not silently treated as no-match.
		matched := a.Name == artifactGlob
		if !matched {
			ok, err := path.Match(artifactGlob, a.Name)
			if err != nil {
				return nil, fmt.Errorf("invalid summaryArtifact pattern %q: %w", artifactGlob, err)
			}
			matched = ok
		}
		if !matched {
			continue
		}
		t, _ := time.Parse(time.RFC3339, a.CreatedAt)
		if best == nil || t.After(bestT) {
			best, bestT = a, t
		}
	}
	if best == nil {
		return nil, ErrNoArtifact
	}

	// IMPORTANT: this must stay a bare `gh api <path>` (GET). gh follows the 302 to
	// the signed blob and writes the raw zip to stdout (verified empirically). Do
	// NOT add JSON-oriented flags (e.g. --jq, -H 'Accept: application/json') here —
	// they would corrupt the binary body.
	zipBytes, err := r.Run(ctx, "api", fmt.Sprintf("repos/%s/actions/artifacts/%s/zip", repo, strconv.FormatInt(best.ID, 10)))
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open artifact zip: %w", err)
	}
	for _, fh := range zr.File {
		if fh.Name != fileName {
			continue
		}
		rc, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s in artifact: %w", fileName, err)
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("%s not found in artifact %s", fileName, best.Name)
}
