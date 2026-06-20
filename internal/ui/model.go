package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/config"
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

// toastTTL is how long a transient status toast stays on screen before the 1s
// UI tick clears it.
const toastTTL = 6 * time.Second

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
	closing        bool
	pendingRefresh bool
	toast          string
	toastAt        time.Time       // when the current toast was set; it expires after toastTTL
	actioned       map[string]bool // URLs of PRs closed/merged this session, struck until the refetch drops them

	review config.Review // review-launcher config (disabled when unset)
	cmux   gh.Runner     // runner targeting the cmux binary

	width, height int
}

// Option configures a Model at construction.
type Option func(*Model)

// WithReview sets the review-launcher config.
func WithReview(r config.Review) Option { return func(m *Model) { m.review = r } }

// New builds a Model. interval is the normal auto-refresh cadence; limit caps
// PRs fetched per bucket.
func New(r gh.Runner, interval time.Duration, limit int, opts ...Option) *Model {
	m := &Model{
		runner:   r,
		limit:    limit,
		backoff:  pr.NewBackoff(interval),
		now:      time.Now,
		bucket:   pr.Authored,
		conn:     connOffline, // until the first successful fetch
		fetching: true,
		method:   pr.MethodSquash,
		cmux:     gh.NewCmuxRunner(),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Init kicks off the first fetch and starts the 1s UI heartbeat.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.runner, m.limit), uiTickCmd())
}

// setToast shows a transient status message, stamping it so the UI tick can
// expire it after toastTTL (otherwise a toast would linger indefinitely).
func (m *Model) setToast(s string) {
	m.toast = s
	m.toastAt = m.now()
}

// expireToast clears the toast once it has been on screen for toastTTL.
func (m *Model) expireToast() {
	if m.toast != "" && m.now().Sub(m.toastAt) >= toastTTL {
		m.toast = ""
	}
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
