package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/packethog/prdash/internal/pr"
)

// FetchResult holds the two PR buckets. Reviewing has already been deduped
// against Authored.
type FetchResult struct {
	Authored  []pr.PR
	Reviewing []pr.PR
}

type ghResponse struct {
	Data struct {
		Authored  searchBlock `json:"authored"`
		Reviewing searchBlock `json:"reviewing"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type searchBlock struct {
	Nodes []prNode `json:"nodes"`
}

type prNode struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	IsDraft    bool   `json:"isDraft"`
	UpdatedAt  string `json:"updatedAt"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	ReviewDecision   string `json:"reviewDecision"`
	Mergeable        string `json:"mergeable"`
	MergeStateStatus string `json:"mergeStateStatus"`
	HeadRefName      string `json:"headRefName"`
	Commits          struct {
		Nodes []struct {
			Commit struct {
				Oid               string `json:"oid"`
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	LatestReviews struct {
		Nodes []struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestReviews"`
	LatestOpinionatedReviews struct {
		Nodes []struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
}

func (n prNode) toPR() pr.PR {
	var rollup, headSHA string
	if len(n.Commits.Nodes) > 0 {
		headSHA = n.Commits.Nodes[0].Commit.Oid
		if n.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
			rollup = n.Commits.Nodes[0].Commit.StatusCheckRollup.State
		}
	}
	t, _ := time.Parse(time.RFC3339, n.UpdatedAt)
	var reviews []string
	for _, r := range n.LatestReviews.Nodes {
		reviews = append(reviews, r.State)
	}
	var opinionated []string
	for _, r := range n.LatestOpinionatedReviews.Nodes {
		opinionated = append(opinionated, r.State)
	}
	return pr.PR{
		Repo:               n.Repository.NameWithOwner,
		Number:             n.Number,
		Title:              n.Title,
		URL:                n.URL,
		HeadRefName:        n.HeadRefName,
		HeadSHA:            headSHA,
		IsDraft:            n.IsDraft,
		UpdatedAt:          t,
		ReviewDecision:     n.ReviewDecision,
		Mergeable:          n.Mergeable,
		MergeStateStatus:   n.MergeStateStatus,
		RollupState:        rollup,
		LatestReviews:      reviews,
		OpinionatedReviews: opinionated,
	}
}

// Fetch runs the combined GraphQL search and returns both buckets. limit caps
// the number of PRs fetched per bucket.
func Fetch(ctx context.Context, r Runner, limit int) (FetchResult, error) {
	args := []string{
		"api", "graphql",
		"-f", "query=" + searchQuery,
		"-f", "authored=" + authoredFilter,
		"-f", "reviewing=" + reviewingFilter,
		"-F", "first=" + strconv.Itoa(limit),
	}
	out, err := r.Run(ctx, args...)
	if err != nil {
		return FetchResult{}, err
	}
	var resp ghResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return FetchResult{}, fmt.Errorf("decode graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		return FetchResult{}, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	var res FetchResult
	for _, n := range resp.Data.Authored.Nodes {
		res.Authored = append(res.Authored, n.toPR())
	}
	for _, n := range resp.Data.Reviewing.Nodes {
		res.Reviewing = append(res.Reviewing, n.toPR())
	}
	res.Reviewing = pr.DedupeReviewing(res.Authored, res.Reviewing)
	return res, nil
}
