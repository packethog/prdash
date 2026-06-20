package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/packethog/prdash/internal/ci"
)

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
