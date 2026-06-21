package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/packethog/prdash/internal/ci"
)

// RunDetail is structured run metadata for the details modal.
type RunDetail struct {
	Status     string
	Conclusion string
	HeadBranch string
	Number     int
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Jobs       []JobDetail
}

// JobDetail holds per-job metadata within a RunDetail.
type JobDetail struct {
	Name       string
	Status     string
	Conclusion string
	Steps      []StepDetail
}

// StepDetail holds per-step metadata within a JobDetail.
type StepDetail struct {
	Name       string
	Status     string
	Conclusion string
}

type runViewJSON struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadBranch string `json:"headBranch"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Jobs       []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Steps      []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"steps"`
	} `json:"jobs"`
}

const runViewFields = "status,conclusion,headBranch,number,url,createdAt,updatedAt,jobs"

// FetchRunDetail fetches structured run metadata via `gh run view --json`.
func FetchRunDetail(ctx context.Context, r Runner, repo string, runID int64) (RunDetail, error) {
	out, err := r.Run(ctx, "run", "view", strconv.FormatInt(runID, 10), "-R", repo, "--json", runViewFields)
	if err != nil {
		return RunDetail{}, err
	}
	var v runViewJSON
	if err := json.Unmarshal(out, &v); err != nil {
		return RunDetail{}, fmt.Errorf("decode run view: %w", err)
	}
	created, _ := time.Parse(time.RFC3339, v.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, v.UpdatedAt)
	d := RunDetail{
		Status: v.Status, Conclusion: v.Conclusion, HeadBranch: v.HeadBranch,
		Number: v.Number, URL: v.URL, CreatedAt: created, UpdatedAt: updated,
	}
	for _, j := range v.Jobs {
		jd := JobDetail{Name: j.Name, Status: j.Status, Conclusion: j.Conclusion}
		for _, s := range j.Steps {
			jd.Steps = append(jd.Steps, StepDetail{Name: s.Name, Status: s.Status, Conclusion: s.Conclusion})
		}
		d.Jobs = append(d.Jobs, jd)
	}
	return d, nil
}

// FailedStep returns the first failed job and step, if any.
func (d RunDetail) FailedStep() (job, step string, ok bool) {
	for _, j := range d.Jobs {
		if j.Conclusion != "failure" {
			continue
		}
		for _, s := range j.Steps {
			if s.Conclusion == "failure" {
				return j.Name, s.Name, true
			}
		}
		return j.Name, "", true
	}
	return "", "", false
}

// RerunFailed re-runs only the failed jobs of a run via `gh run rerun --failed`.
func RerunFailed(ctx context.Context, r Runner, repo string, runID int64) error {
	_, err := r.Run(ctx, "run", "rerun", strconv.FormatInt(runID, 10), "-R", repo, "--failed")
	return err
}

// prRunListCap bounds how many recent branch runs we scan for the head commit.
// A single head commit's Actions runs won't realistically exceed this.
const prRunListCap = 50

type prRunNode struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSha    string `json:"headSha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// RerunPRFailed reruns the failed jobs of every Actions run on the PR's head
// commit. It lists runs for the branch, keeps completed runs whose headSha
// matches and whose conclusion is failure/timed_out/startup_failure, and calls
// `gh run rerun <id> --failed` on each. Returns the number of runs reran.
func RerunPRFailed(ctx context.Context, r Runner, repo, branch, headSHA string) (int, error) {
	out, err := r.Run(ctx, "run", "list", "-R", repo, "--branch", branch,
		"--limit", strconv.Itoa(prRunListCap), "--json", "databaseId,headSha,status,conclusion")
	if err != nil {
		return 0, err
	}
	var nodes []prRunNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		return 0, fmt.Errorf("decode run list: %w", err)
	}
	n := 0
	for _, node := range nodes {
		if node.Status != "completed" || node.HeadSha != headSHA {
			continue
		}
		switch node.Conclusion {
		case "failure", "timed_out", "startup_failure":
		default:
			continue
		}
		if err := RerunFailed(ctx, r, repo, node.DatabaseID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

type runNode struct {
	DatabaseID int64  `json:"databaseId"`
	Number     int    `json:"number"`
	HeadBranch string `json:"headBranch"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

const runListFields = "databaseId,number,headBranch,status,conclusion,url,createdAt,updatedAt"

// ListRuns fetches the last `limit` runs of one workflow via `gh run list`.
// name is the display label applied to the returned WorkflowRuns and each Run.
func ListRuns(ctx context.Context, r Runner, repo, workflowFile, name, branch string, limit int) (ci.WorkflowRuns, error) {
	args := []string{"run", "list", "-R", repo, "--workflow", workflowFile}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, "--limit", strconv.Itoa(limit), "--json", runListFields)

	out, err := r.Run(ctx, args...)
	if err != nil {
		return ci.WorkflowRuns{}, err
	}
	var nodes []runNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		return ci.WorkflowRuns{}, fmt.Errorf("decode run list: %w", err)
	}
	wr := ci.WorkflowRuns{Name: name, Branch: branch, Repo: repo, Key: workflowFile}
	for _, n := range nodes {
		created, _ := time.Parse(time.RFC3339, n.CreatedAt)
		updated, _ := time.Parse(time.RFC3339, n.UpdatedAt)
		wr.Runs = append(wr.Runs, ci.Run{
			Repo:         repo,
			WorkflowKey:  workflowFile,
			WorkflowName: name,
			Branch:       n.HeadBranch,
			Status:       n.Status,
			Conclusion:   n.Conclusion,
			URL:          n.URL,
			RunID:        n.DatabaseID,
			RunNumber:    n.Number,
			CreatedAt:    created,
			UpdatedAt:    updated,
		})
	}
	return wr, nil
}
