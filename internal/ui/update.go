package ui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

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
			return m, tea.Batch(fetchCmd(m.runner, m.limit), ciFetchCmd(m.runner, m.ci))
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
	// If a modal was armed on a PR the refetch dropped (closed/merged
	// elsewhere), dismiss the now-orphaned inline prompt.
	if m.modal != modalNone && indexByURL(m.authored, m.modalPR.URL) < 0 {
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
	var n int
	if m.section == secCI {
		n = len(m.ciItems())
	} else {
		n = len(m.rows())
	}
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
				return m, tea.Batch(fetchCmd(m.runner, m.limit), ciFetchCmd(m.runner, m.ci))
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
	case "o":
		if p, ok := m.selected(); ok {
			return m, openCmd(m.runner, p)
		}
	case "v":
		if m.reviewEligible() {
			if p, ok := m.selected(); ok {
				return m, reviewCmd(m.cmux, m.review, p)
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
	if m.modal == modalClose {
		return m.onCloseModalKey(msg)
	}
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
