package ui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/pr"
)

// inCmux reports whether prdash is running inside a cmux surface.
func inCmux() bool { return os.Getenv("CMUX_WORKSPACE_ID") != "" }

// Update is the Bubble Tea update function.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if msg.gen != m.tickGen || m.fetching {
			return m, nil
		}
		m.fetching = true
		if m.ciEnabled() {
			return m, tea.Batch(fetchCmd(m.runner, m.limit), m.ciFetch())
		}
		return m, fetchCmd(m.runner, m.limit)
	case uiTickMsg:
		m.expireToast()
		return m, uiTickCmd()
	case prsFetchedMsg:
		return m.onFetched(msg)
	case fetchFailedMsg:
		return m.onFetchFailed(msg)
	case ciFetchedMsg:
		if msg.gen != m.ciGen {
			return m, nil // stale result from a superseded fetch; discard
		}
		m.workflows = msg.workflows
		if m.section == secCI {
			m.clampCursor()
		}
		return m, nil
	case mergeDoneMsg:
		m.merging = false
		m.setToast("Merged " + msg.p.Ref())
		m.markActioned(msg.p.URL)
		if !m.fetching {
			m.fetching = true
			m.tickGen++
			return m, fetchCmd(m.runner, m.limit)
		}
		m.pendingRefresh = true
		return m, nil
	case mergeFailedMsg:
		m.merging = false
		m.setToast("Merge failed: " + msg.err.Error())
		return m, nil
	case closeDoneMsg:
		m.closing = false
		m.setToast("Closed " + msg.p.Ref())
		m.markActioned(msg.p.URL)
		if !m.fetching {
			m.fetching = true
			m.tickGen++
			return m, fetchCmd(m.runner, m.limit)
		}
		m.pendingRefresh = true
		return m, nil
	case closeFailedMsg:
		m.closing = false
		m.setToast("Close failed: " + msg.err.Error())
		return m, nil
	case openedMsg:
		if msg.err != nil {
			m.setToast("Open failed: " + msg.err.Error())
		}
		return m, nil
	case reviewLaunchedMsg:
		if msg.err != nil {
			m.setToast("Review failed: " + msg.err.Error())
		} else {
			m.setToast("Review started for " + msg.p.Ref())
		}
		return m, nil
	case runDetailMsg:
		if m.modal == modalDetails && msg.runID == m.detailRun.RunID {
			m.detail, m.detailErr = msg.d, msg.err
		}
		return m, nil
	case summaryMsg:
		if m.modal == modalDetails && msg.runID == m.detailRun.RunID {
			if msg.err != nil {
				m.summaryErr = msg.err
			} else {
				m.summary = string(msg.data)
			}
		}
		return m, nil
	case ciDebugLaunchedMsg:
		if msg.err != nil {
			m.setToast("Debug failed: " + msg.err.Error())
		} else {
			m.setToast("Debug started")
		}
		return m, nil
	case rerunDoneMsg:
		m.rerunning = false
		m.setToast(fmt.Sprintf("Rerun queued #%d", msg.run.RunNumber))
		// No immediate refetch: `gh run rerun` queues asynchronously, so a refetch
		// now would still show the old failure. The normal tick picks it up.
		return m, nil
	case rerunFailedMsg:
		m.rerunning = false
		m.setToast("Rerun failed: " + msg.err.Error())
		return m, nil
	case openedURLMsg:
		if msg.err != nil {
			m.setToast("Open failed: " + msg.err.Error())
		}
		return m, nil
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// markActioned flags a PR (by URL) as closed/merged this session so its row is
// struck through until a refetch drops it from the list.
func (m *Model) markActioned(url string) {
	if m.actioned == nil {
		m.actioned = map[string]bool{}
	}
	m.actioned[url] = true
}

// pruneActioned drops strike-through marks for PRs that the latest fetch no
// longer lists (they have fallen off the page), keeping marks for any that
// still appear (e.g. while the search index catches up).
func (m *Model) pruneActioned() {
	if len(m.actioned) == 0 {
		return
	}
	kept := map[string]bool{}
	for _, p := range m.authored {
		if m.actioned[p.URL] {
			kept[p.URL] = true
		}
	}
	m.actioned = kept
}

func (m *Model) onFetched(msg prsFetchedMsg) (tea.Model, tea.Cmd) {
	m.authored = msg.res.Authored
	m.reviewing = msg.res.Reviewing
	m.conn = connLive
	m.lastUpdated = m.now()
	m.lastErr = nil
	m.fetching = false
	m.backoff.RecordSuccess()
	m.clampCursor()
	// Only the merge/close modals are anchored to a PR row; a refetch that drops
	// that PR dismisses them. CI modals (details/rerun) are not PR-anchored.
	if (m.modal == modalMerge || m.modal == modalClose) && indexByURL(m.authored, m.modalPR.URL) < 0 {
		m.modal = modalNone
	}
	m.pruneActioned()
	if m.pendingRefresh {
		m.pendingRefresh = false
		m.fetching = true
		m.tickGen++
		return m, fetchCmd(m.runner, m.limit)
	}
	return m, m.scheduleTick()
}

func (m *Model) onFetchFailed(msg fetchFailedMsg) (tea.Model, tea.Cmd) {
	m.lastErr = msg.err
	m.fetching = false
	m.backoff.RecordFailure()
	switch {
	case len(m.authored)+len(m.reviewing) == 0:
		m.conn = connOffline
	case m.backoff.Failures() == 1:
		m.conn = connStale
	default:
		m.conn = connOffline
	}
	if m.pendingRefresh {
		m.pendingRefresh = false
		m.fetching = true
		m.tickGen++
		return m, fetchCmd(m.runner, m.limit)
	}
	return m, m.scheduleTick()
}

func (m *Model) scheduleTick() tea.Cmd {
	m.tickGen++
	return tickCmd(m.backoff.Delay(), m.tickGen)
}

// itemCount returns the number of navigable items in the current section.
func (m *Model) itemCount() int {
	if m.section == secCI {
		return len(m.ciItems())
	}
	return len(m.rows())
}

func (m *Model) clampCursor() {
	n := m.itemCount()
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// nextSection returns the section that follows the current one, respecting
// whether CI is enabled.
func (m *Model) nextSection() section {
	switch m.section {
	case secAuthored:
		return secReviewing
	case secReviewing:
		if m.ciEnabled() {
			return secCI
		}
		return secAuthored
	default:
		return secAuthored
	}
}

func (m *Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal != modalNone {
		return m.onModalKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < m.itemCount()-1 {
			m.cursor++
		}
	case "tab":
		m.section = m.nextSection()
		m.cursor = 0
	case "r":
		if !m.fetching {
			m.fetching = true
			m.tickGen++
			if m.ciEnabled() {
				return m, tea.Batch(fetchCmd(m.runner, m.limit), m.ciFetch())
			}
			return m, fetchCmd(m.runner, m.limit)
		}
	case "m":
		if m.section == secAuthored {
			if p, ok := m.selected(); ok {
				m.modal = modalMerge
				m.modalPR = p // capture so a refetch can't swap the row under us
				m.method = pr.MethodSquash
			}
		}
	case "c":
		if m.section == secAuthored {
			if p, ok := m.selected(); ok {
				m.modal = modalClose
				m.modalPR = p
			}
		}
	case "d":
		if m.section == secCI && m.ciDebugEligible() {
			if r, ok := m.selectedRun(); ok {
				return m, ciDebugCmd(m.cmux, m.ci, r)
			}
		}
	case "R":
		if m.section == secCI {
			if r, ok := m.selectedRun(); ok && ci.IsFailed(r) {
				m.modal = modalRerun
				m.rerunRun = r
			}
		}
	case "o":
		if m.section == secCI {
			if r, ok := m.selectedRun(); ok {
				return m, openURLCmd(m.runner, r.URL)
			}
			return m, nil
		}
		if p, ok := m.selected(); ok {
			return m, openCmd(m.runner, p)
		}
	case "v":
		if m.reviewEligible() {
			if p, ok := m.selected(); ok {
				return m, reviewCmd(m.cmux, m.review, p)
			}
		}
	case "enter":
		if m.section == secCI {
			if it, ok := m.selectedCIItem(); ok {
				switch it.kind {
				case ciHeader:
					key := wfKey(m.workflows[it.wf])
					m.expanded[key] = !m.expanded[key]
				case ciRun:
					r := m.workflows[it.wf].Runs[it.run]
					m.modal = modalDetails
					m.detailRun = r
					m.summary, m.summaryErr, m.detailErr = "", nil, nil
					m.detail = gh.RunDetail{}
					glob, file := m.summaryConfigFor(r)
					m.detailGlob, m.detailFile = glob, file
					cmds := []tea.Cmd{runDetailCmd(m.runner, r.Repo, r.RunID)}
					if glob != "" {
						cmds = append(cmds, summaryCmd(m.runner, r.Repo, r.RunID, glob, file))
					}
					return m, tea.Batch(cmds...)
				}
			}
		}
	}
	return m, nil
}

// reviewEligible reports whether the review launcher should act/show: under
// cmux, in the Reviewing section, with a configured review.
func (m *Model) reviewEligible() bool {
	return inCmux() && m.section == secReviewing && m.review.Enabled()
}

func (m *Model) ciDebugEligible() bool {
	return inCmux() && m.ci.DebugEnabled()
}

// summaryConfigFor returns the summaryArtifact glob and file configured for the
// run's workflow. An exact branch match wins; an unbranched entry is the
// fallback. This is order-independent (a branch-specific entry is preferred even
// if an unbranched entry for the same workflow appears earlier in config).
func (m *Model) summaryConfigFor(r ci.Run) (glob, file string) {
	fbIdx := -1
	for i := range m.ci.Workflows {
		w := m.ci.Workflows[i]
		if w.Repo != r.Repo || w.Workflow != r.WorkflowKey {
			continue
		}
		if w.Branch == r.Branch {
			return w.SummaryArtifact, w.SummaryFile
		}
		if w.Branch == "" && fbIdx < 0 {
			fbIdx = i
		}
	}
	if fbIdx >= 0 {
		return m.ci.Workflows[fbIdx].SummaryArtifact, m.ci.Workflows[fbIdx].SummaryFile
	}
	return "", ""
}

func (m *Model) mergeBlockers() []string {
	b := pr.MergeBlockers(m.modalPR)
	if m.conn != connLive {
		b = append(b, "connection not live — refresh first")
	}
	return b
}

// closeBlockers returns the reasons the captured PR cannot be closed. Closing
// your own open PR only requires a live connection (close is reversible).
func (m *Model) closeBlockers() []string {
	if m.conn != connLive {
		return []string{"connection not live — refresh first"}
	}
	return nil
}

func (m *Model) onCloseModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
	case "enter":
		if len(m.closeBlockers()) == 0 && !m.closing {
			m.closing = true
			m.modal = modalNone
			return m, closeCmd(m.runner, m.modalPR)
		}
	}
	return m, nil
}

func (m *Model) onModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalDetails:
		return m.onDetailsModalKey(msg)
	case modalRerun:
		return m.onRerunModalKey(msg)
	case modalClose:
		return m.onCloseModalKey(msg)
	default:
		return m.onMergeModalKey(msg)
	}
}

// onMergeModalKey is the former onModalKey body (esc / left / right / s / enter).
func (m *Model) onMergeModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
	case "right", "s":
		m.method = m.method.Next()
	case "left":
		m.method = m.method.Prev()
	case "enter":
		if len(m.mergeBlockers()) == 0 && !m.merging {
			m.merging = true
			m.modal = modalNone
			return m, mergeCmd(m.runner, m.modalPR, m.method)
		}
	}
	return m, nil
}

func (m *Model) onDetailsModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q":
		m.modal = modalNone
	case "o":
		return m, openURLCmd(m.runner, m.detailRun.URL)
	case "d":
		if m.ciDebugEligible() {
			return m, ciDebugCmd(m.cmux, m.ci, m.detailRun)
		}
	case "R":
		if ci.IsFailed(m.detailRun) {
			m.modal = modalRerun
			m.rerunRun = m.detailRun
		}
	}
	return m, nil
}

func (m *Model) onRerunModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.modal = modalNone
	case "enter":
		if !m.rerunning {
			m.rerunning = true
			m.modal = modalNone
			return m, rerunCmd(m.runner, m.rerunRun)
		}
	}
	return m, nil
}
