package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/pr"
)

// Update is the Bubble Tea update function.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if msg.gen != m.tickGen {
			return m, nil
		}
		if m.fetching {
			return m, nil
		}
		m.fetching = true
		return m, fetchCmd(m.runner, m.limit)
	case uiTickMsg:
		return m, uiTickCmd()
	case prsFetchedMsg:
		return m.onFetched(msg)
	case fetchFailedMsg:
		return m.onFetchFailed(msg)
	case mergeDoneMsg:
		m.merging = false
		m.toast = "Merged " + msg.p.Ref()
		if !m.fetching {
			m.fetching = true
			m.tickGen++
			return m, fetchCmd(m.runner, m.limit)
		}
		m.pendingRefresh = true
		return m, nil
	case mergeFailedMsg:
		m.merging = false
		m.toast = "Merge failed: " + msg.err.Error()
		return m, nil
	case closeDoneMsg:
		m.closing = false
		m.toast = "Closed " + msg.p.Ref()
		if !m.fetching {
			m.fetching = true
			m.tickGen++
			return m, fetchCmd(m.runner, m.limit)
		}
		m.pendingRefresh = true
		return m, nil
	case closeFailedMsg:
		m.closing = false
		m.toast = "Close failed: " + msg.err.Error()
		return m, nil
	case openedMsg:
		if msg.err != nil {
			m.toast = "Open failed: " + msg.err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
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

func (m *Model) clampCursor() {
	n := len(m.rows())
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
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
		if m.cursor < len(m.rows())-1 {
			m.cursor++
		}
	case "tab":
		if m.bucket == pr.Authored {
			m.bucket = pr.AwaitingReview
		} else {
			m.bucket = pr.Authored
		}
		m.cursor = 0
	case "r":
		if !m.fetching {
			m.fetching = true
			m.tickGen++
			return m, fetchCmd(m.runner, m.limit)
		}
	case "m":
		if m.bucket == pr.Authored {
			if p, ok := m.selected(); ok {
				m.modal = modalMerge
				m.modalPR = p // capture so a refetch can't swap the row under us
				m.method = pr.MethodSquash
			}
		}
	case "c":
		if m.bucket == pr.Authored {
			if p, ok := m.selected(); ok {
				m.modal = modalClose
				m.modalPR = p
			}
		}
	case "o":
		if p, ok := m.selected(); ok {
			return m, openCmd(m.runner, p)
		}
	}
	return m, nil
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
