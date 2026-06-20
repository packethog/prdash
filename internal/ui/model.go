package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
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

type section int

const (
	secAuthored section = iota
	secReviewing
	secCI
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
	section   section
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

	ci        config.CI
	workflows []ci.WorkflowRuns
	expanded  map[string]bool

	width, height int
}

// Option configures a Model at construction.
type Option func(*Model)

// WithReview sets the review-launcher config.
func WithReview(r config.Review) Option { return func(m *Model) { m.review = r } }

// WithCI sets the CI-workflows config.
func WithCI(c config.CI) Option { return func(m *Model) { m.ci = c } }

// New builds a Model. interval is the normal auto-refresh cadence; limit caps
// PRs fetched per bucket.
func New(r gh.Runner, interval time.Duration, limit int, opts ...Option) *Model {
	m := &Model{
		runner:   r,
		limit:    limit,
		backoff:  pr.NewBackoff(interval),
		now:      time.Now,
		section:  secAuthored,
		conn:     connOffline, // until the first successful fetch
		fetching: true,
		method:   pr.MethodSquash,
		cmux:     gh.NewCmuxRunner(),
		expanded: map[string]bool{},
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Init kicks off the first fetch and starts the 1s UI heartbeat.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{fetchCmd(m.runner, m.limit), uiTickCmd()}
	if m.ciEnabled() {
		cmds = append(cmds, ciFetchCmd(m.runner, m.ci))
	}
	return tea.Batch(cmds...)
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

// activeBucket returns the PR bucket corresponding to the current section.
func (m *Model) activeBucket() pr.Bucket {
	if m.section == secReviewing {
		return pr.AwaitingReview
	}
	return pr.Authored
}

// rows returns the PR slice for the active bucket.
func (m *Model) rows() []pr.PR {
	if m.activeBucket() == pr.AwaitingReview {
		return m.reviewing
	}
	return m.authored
}

// selected returns the PR under the cursor, if any.
func (m *Model) selected() (pr.PR, bool) {
	rows := m.rows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return pr.PR{}, false
	}
	return rows[m.cursor], true
}

// ciEnabled reports whether CI workflows are configured.
func (m *Model) ciEnabled() bool { return m.ci.Enabled() }

// wfKey is the stable expand-state key for a workflow. Branch is included so two
// entries for the same repo+workflow on different branches don't collide; it is
// appended only when set, so unbranched entries key as "repo file".
func wfKey(w ci.WorkflowRuns) string {
	k := w.Repo + " " + w.Key
	if w.Branch != "" {
		k += " " + w.Branch
	}
	return k
}

type ciItemKind int

const (
	ciHeader ciItemKind = iota
	ciRun
)

type ciItem struct {
	kind ciItemKind
	wf   int // index into m.workflows
	run  int // index into m.workflows[wf].Runs (only for ciRun)
}

// ciItems is the flattened navigable list: each workflow header, followed by its
// runs when expanded.
func (m *Model) ciItems() []ciItem {
	var items []ciItem
	for wi := range m.workflows {
		items = append(items, ciItem{kind: ciHeader, wf: wi})
		if m.expanded[wfKey(m.workflows[wi])] {
			for ri := range m.workflows[wi].Runs {
				items = append(items, ciItem{kind: ciRun, wf: wi, run: ri})
			}
		}
	}
	return items
}

// selectedCIItem returns the ci item under the cursor when CI is focused.
// Used by Task 9 (CI section rendering and key handling).
//
//nolint:unused
func (m *Model) selectedCIItem() (ciItem, bool) {
	if m.section != secCI {
		return ciItem{}, false
	}
	items := m.ciItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return ciItem{}, false
	}
	return items[m.cursor], true
}

// selectedRun returns the run under the cursor, if the cursor is on a run row.
// Used by Task 9 (CI section key handling).
//
//nolint:unused
func (m *Model) selectedRun() (ci.Run, bool) {
	it, ok := m.selectedCIItem()
	if !ok || it.kind != ciRun {
		return ci.Run{}, false
	}
	return m.workflows[it.wf].Runs[it.run], true
}
