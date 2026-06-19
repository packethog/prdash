package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/packethog/prdash/internal/pr"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	// selectedStyle is the full-row highlight bar for the row under the cursor.
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("231")).Bold(true)

	liveStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	staleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	offlineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	commentedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // cyan

	modalBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 2)
)

func reviewStyle(r pr.ReviewState) lipgloss.Style {
	switch r {
	case pr.ReviewApproved:
		return liveStyle
	case pr.ReviewChangesRequested:
		return offlineStyle
	case pr.ReviewDraft:
		return dimStyle
	case pr.ReviewCommented:
		return commentedStyle
	default:
		return staleStyle
	}
}

func ciStyle(c pr.CIState) lipgloss.Style {
	switch c {
	case pr.CISuccess:
		return liveStyle
	case pr.CIFailure:
		return offlineStyle
	case pr.CIPending:
		return staleStyle
	default:
		return dimStyle
	}
}

// truncate shortens s to at most w terminal cells (not runes), adding an
// ellipsis when it cuts. Cell-aware so CJK/emoji titles don't overflow their
// column, and ANSI-aware so it won't split a color escape.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	if w == 1 {
		return ansi.Truncate(s, 1, "")
	}
	return ansi.Truncate(s, w, "…")
}

// Column widths (in cells). TITLE flexes; the rest are fixed. colGap is the
// 1-cell space rendered between columns (as right padding).
const (
	colRefW    = 34 // owner/name#number
	colTitleW  = 34 // default TITLE width before the terminal size is known
	colReviewW = 17 // widest review label: "Changes requested"
	colCIW     = 2  // CI column: "CI" header + 1-cell badge
	colGap     = 1
)

// padTo truncates s to w cells, then right-pads with spaces to exactly w cells,
// so every column occupies a fixed visible width regardless of wide characters.
func padTo(s string, w int) string {
	s = truncate(s, w)
	if pad := w - ansi.StringWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func humanizeSince(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}

func (m *Model) connLine() string {
	switch m.conn {
	case connLive:
		return liveStyle.Render("● Live") + " · updated " + humanizeSince(m.now().Sub(m.lastUpdated)) + " ago"
	case connStale:
		return staleStyle.Render("◐ Stale") + " · updated " + humanizeSince(m.now().Sub(m.lastUpdated)) + " ago"
	default:
		line := offlineStyle.Render("✕ Offline") + " · retrying in " + humanizeSince(m.backoff.Delay())
		if len(m.authored)+len(m.reviewing) > 0 {
			line += "   " + dimStyle.Render("(showing last-known data)")
		}
		return line
	}
}

// titleWidth is the flexible TITLE width: terminal width minus the fixed columns
// and the inter-column gaps. Falls back to colTitleW before the first
// WindowSizeMsg; floors at 1 on very narrow terminals.
func (m *Model) titleWidth() int {
	if m.width <= 0 {
		return colTitleW
	}
	// PR + gap + [title] + gap + REVIEW + gap + CI
	fixed := colRefW + colGap + colGap + colReviewW + colGap + colCIW
	if w := m.width - fixed; w > 1 {
		return w
	}
	return 1
}

// renderTable lays out one bucket's rows with lipgloss/table. sel is the index
// of the selected row, or -1 for none.
//
// Each cell's content is PRE-PADDED to its exact column width (content width +
// the inter-column gap, the last column gets no gap), so the table never has to
// distribute width and the columns sum to exactly m.width. lipgloss/table then
// only does structure + per-cell styling.
//
// FALLBACK: lipgloss/table renders each cell independently, so background styles
// applied via StyleFunc do not span the full row width. The selected row is
// therefore rendered as one pre-padded plain string wrapped in selectedStyle and
// spliced into the table output at its line index (header is line 0, data rows
// start at line 1), producing a gap-free full-width highlight bar.
func (m *Model) renderTable(rows []pr.PR, sel int) string {
	tw := m.titleWidth()
	// content widths, and the rendered cell widths (content + gap; CI has none).
	cellW := [4]int{colRefW + colGap, tw + colGap, colReviewW + colGap, colCIW}

	style := func(row, col int) lipgloss.Style {
		st := lipgloss.NewStyle().Padding(0) // no table padding; cells are pre-sized
		switch {
		case row == table.HeaderRow:
			return st.Inherit(dimStyle)
		case col == 2:
			return st.Inherit(reviewStyle(pr.Review(rows[row])))
		case col == 3:
			return st.Inherit(ciStyle(pr.CI(rows[row])))
		}
		return st
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderHeader(false).
		Wrap(false).
		Headers("PR", "TITLE", "REVIEW", "CI").
		StyleFunc(style)
	for _, p := range rows {
		t.Row(
			padTo(p.Ref(), cellW[0]),
			padTo(truncate(p.Title, tw), cellW[1]),
			padTo(pr.Review(p).String(), cellW[2]),
			padTo(pr.CI(p).Symbol(), cellW[3]),
		)
	}
	out := strings.TrimRight(t.String(), "\n") // table may emit a trailing newline

	// Build the fallback selected-row line: one pre-padded string at m.width.
	var selLine string
	if sel >= 0 && sel < len(rows) {
		p := rows[sel]
		line := padTo(p.Ref(), cellW[0]) +
			padTo(truncate(p.Title, tw), cellW[1]) +
			padTo(pr.Review(p).String(), cellW[2]) +
			padTo(pr.CI(p).Symbol(), cellW[3])
		if m.width > 0 {
			line = padTo(line, m.width)
		}
		selLine = selectedStyle.Render(line)
	}

	if m.width <= 0 {
		return out
	}
	// Hard clamp each line as a safety net (e.g. a terminal that draws a glyph
	// double-width). ANSI-aware so it won't split a color escape. Splice in the
	// selected-row fallback at line index sel+1 (line 0 is the header).
	var b strings.Builder
	for i, ln := range strings.Split(out, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		// line 0 = header; data row `sel` is at line index sel+1
		if sel >= 0 && i == sel+1 {
			b.WriteString(selLine)
		} else {
			b.WriteString(ansi.Truncate(ln, m.width, ""))
		}
	}
	return b.String()
}

// chromeLines is the fixed UI outside the scrollable body: title + blank (2),
// blank separator before footer (1), status (1), keys (1).
const chromeLines = 5

// window returns the visible slice of lines for a viewport of vpHeight lines,
// scrolled so cursorLine stays visible, plus first (the index of the first
// visible line, for the scroll hint). cursorLine < 0 means "no cursor" (keeps
// the top). vpHeight <= 0 shows nothing; content that fits returns all lines.
func window(lines []string, cursorLine, vpHeight int) (visible []string, first int) {
	if vpHeight <= 0 {
		return nil, 0
	}
	if len(lines) <= vpHeight {
		return lines, 0
	}
	offset := 0
	if cursorLine >= vpHeight {
		offset = cursorLine - vpHeight + 1
	}
	if max := len(lines) - vpHeight; offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}
	return lines[offset : offset+vpHeight], offset
}

// renderBody builds the scrollable region (both buckets) as a flat line slice,
// and the absolute line index of the active cursor's row (-1 if none).
func (m *Model) renderBody() (lines []string, cursorLine int) {
	cursorLine = -1
	appendBucket := func(label string, rows []pr.PR, active bool) {
		lines = append(lines, sectionStyle.Render(fmt.Sprintf("%s (%d)", label, len(rows))))
		if len(rows) == 0 {
			lines = append(lines, dimStyle.Render("  (none)"))
			return
		}
		blockStart := len(lines) // the table's header line lands here
		lines = append(lines, strings.Split(m.renderTable(rows, selIdx(active, m.cursor)), "\n")...)
		if active {
			cursorLine = blockStart + 1 + m.cursor // +1 for the table header row
		}
	}
	appendBucket("AUTHORED", m.authored, m.bucket == pr.Authored)
	lines = append(lines, "") // separator between buckets
	appendBucket("AWAITING MY REVIEW", m.reviewing, m.bucket == pr.AwaitingReview)
	return lines, cursorLine
}

func selIdx(active bool, cursor int) int {
	if active {
		return cursor
	}
	return -1
}

// clampLine truncates a rendered line to the terminal width (ANSI-aware).
func (m *Model) clampLine(s string) string {
	if m.width <= 0 {
		return s
	}
	return ansi.Truncate(s, m.width, "")
}

func (m *Model) renderModal() string {
	p := m.modalPR // captured when the modal opened
	var b strings.Builder
	b.WriteString(titleStyle.Render("Merge pull request") + "\n\n")
	b.WriteString(p.Ref() + "\n")
	b.WriteString(truncate(p.Title, 46) + "\n\n")
	blockers := m.mergeBlockers()
	if len(blockers) == 0 {
		fmt.Fprintf(&b, "Review: %s   CI: %s\n", pr.Review(p), pr.CI(p).Symbol())
		b.WriteString("Method: ‹ " + m.method.String() + " ›   (←/→ to change)\n\n")
		b.WriteString(dimStyle.Render("enter Merge    esc Cancel"))
	} else {
		b.WriteString(offlineStyle.Render("✗ Blocked: "+strings.Join(blockers, "; ")) + "\n\n")
		b.WriteString(dimStyle.Render("esc Close"))
	}
	return modalBox.Render(b.String())
}

// View renders the whole UI. It assembles all lines and joins them WITHOUT a
// trailing newline, so the total line count is exactly len(top)+len(visible)+
// len(footer) and never exceeds m.height.
func (m *Model) View() string {
	body, cursorLine := m.renderBody()

	var visible []string
	hint := ""
	if m.height <= 0 {
		visible = body // unwindowed before the first resize (and in tests)
	} else {
		vp := m.height - chromeLines
		if vp < 0 {
			vp = 0
		}
		var first int
		visible, first = window(body, cursorLine, vp)
		if vp > 0 && len(body) > vp {
			hint = dimStyle.Render(fmt.Sprintf("  %d–%d/%d", first+1, first+len(visible), len(body)))
		}
	}

	status := m.connLine()
	if m.toast != "" {
		status += "   " + m.toast
	}
	keys := dimStyle.Render("↑↓ move  tab switch  r refresh  m merge  o open  q quit") + hint

	all := []string{titleStyle.Render("prdash"), ""}
	all = append(all, visible...)
	all = append(all, "", status, keys)
	for i := range all {
		all[i] = m.clampLine(all[i]) // every line fits the terminal width
	}
	if m.height > 0 && len(all) > m.height {
		all = all[:m.height] // tiny-height guard: never exceed the terminal height
	}
	base := strings.Join(all, "\n")

	if m.modal != modalMerge {
		return base
	}
	modal := m.renderModal()
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	}
	return base + "\n\n" + modal
}
