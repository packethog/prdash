package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/pr"
)

type connState int

const (
	connLive connState = iota
	connStale
	connOffline
)

type modalState int

const (
	modalNone modalState = iota
	modalMerge
	modalClose
)

// Model is the Bubble Tea model for prdash.
type Model struct {
	runner  gh.Runner
	limit   int
	backoff *pr.Backoff
	now     func() time.Time // injectable clock for tests

	authored  []pr.PR
	reviewing []pr.PR
	bucket    pr.Bucket
	cursor    int

	conn        connState
	lastUpdated time.Time
	lastErr     error
	fetching    bool
	tickGen     int

	modal          modalState
	modalPR        pr.PR // PR captured when a modal opened (immune to refetch)
	method         pr.MergeMethod
	merging        bool
	closing        bool //nolint:unused // used in Task 3
	pendingRefresh bool
	toast          string

	width, height int
}

// New builds a Model. interval is the normal auto-refresh cadence; limit caps
// PRs fetched per bucket.
func New(r gh.Runner, interval time.Duration, limit int) *Model {
	return &Model{
		runner:   r,
		limit:    limit,
		backoff:  pr.NewBackoff(interval),
		now:      time.Now,
		bucket:   pr.Authored,
		conn:     connOffline, // until the first successful fetch
		fetching: true,
		method:   pr.MethodSquash,
	}
}

// Init kicks off the first fetch and starts the 1s UI heartbeat.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.runner, m.limit), uiTickCmd())
}

// rows returns the PR slice for the active bucket.
func (m *Model) rows() []pr.PR {
	if m.bucket == pr.Authored {
		return m.authored
	}
	return m.reviewing
}

// selected returns the PR under the cursor, if any.
func (m *Model) selected() (pr.PR, bool) {
	rows := m.rows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return pr.PR{}, false
	}
	return rows[m.cursor], true
}
