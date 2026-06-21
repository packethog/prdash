package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/packethog/prdash/internal/ci"
	"github.com/packethog/prdash/internal/gh"
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

	// promptStyle is the accent for the inline confirm line shown under the
	// selected row when a merge/close is armed.
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
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

func ciRunStyle(s ci.RunStatus) lipgloss.Style {
	switch s {
	case ci.RunSuccess:
		return liveStyle
	case ci.RunFailure:
		return offlineStyle
	case ci.RunQueued, ci.RunInProgress:
		return staleStyle
	default:
		return dimStyle
	}
}

// sparkline renders up to the configured run glyphs, colored by status.
func sparkline(runs []ci.Run) string {
	parts := make([]string, 0, len(runs))
	for _, r := range runs {
		st := ci.Status(r)
		parts = append(parts, ciRunStyle(st).Render(st.Symbol()))
	}
	return strings.Join(parts, " ")
}

// sparklinePlain is the uncolored glyph run used for measuring/selection.
func sparklinePlain(runs []ci.Run) string {
	parts := make([]string, 0, len(runs))
	for _, r := range runs {
		parts = append(parts, ci.Status(r).Symbol())
	}
	return strings.Join(parts, " ")
}

// padToWidth right-pads an already-styled string (whose visible width is known)
// to w cells, so ANSI escapes aren't counted by ansi.StringWidth twice.
func padToWidth(styled string, visibleW, w int) string {
	if pad := w - visibleW; pad > 0 {
		return styled + strings.Repeat(" ", pad)
	}
	return styled
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

// CI-section fixed column widths. The LAST column width is computed dynamically
// in renderCI so the sparkline fits every run (see ciSparkWidth).
const (
	ciNameW    = 26 // WORKFLOW
	ciBranchW  = 8  // BRANCH
	ciSparkGap = 3  // spaces between the LAST column and UPDATED (wider than colGap)
)

// ciSparkWidth returns the LAST-column width (cells) needed to show every run's
// status glyph across all workflows, so the column scales with the run count and
// never truncates the sparkline. N glyphs joined by single spaces = 2N-1 cells;
// floored at len("LAST") so the header always fits.
func (m *Model) ciSparkWidth() int {
	w := ansi.StringWidth("LAST")
	for wi := range m.workflows {
		if n := len(m.workflows[wi].Runs); n > 0 {
			if cells := 2*n - 1; cells > w {
				w = cells
			}
		}
	}
	return w
}

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
		if row == table.HeaderRow {
			return st.Inherit(dimStyle)
		}
		if m.actioned[rows[row].URL] {
			return st.Strikethrough(true).Inherit(dimStyle) // closed/merged, pending removal
		}
		switch col {
		case 2:
			return st.Inherit(reviewStyle(pr.Review(rows[row])))
		case 3:
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
		s := selectedStyle
		if m.actioned[p.URL] {
			s = s.Strikethrough(true) // closed/merged + selected: struck highlight bar
		}
		selLine = s.Render(line)
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
		// When a modal is armed, anchor BOTH the highlight and the inline confirm
		// prompt to the captured PR's row (matched by URL), so a background
		// refetch that reorders rows can't show the prompt under a different PR
		// than Enter will act on.
		sel := selIdx(active, m.cursor)
		promptRow := -1
		var prompt []string
		if active && m.modal != modalNone {
			if idx := indexByURL(rows, m.modalPR.URL); idx >= 0 {
				prompt = m.promptLines()
				promptRow = idx
				sel = idx
			}
		}
		blockStart := len(lines) // the table's header line lands here
		tableLines := strings.Split(m.renderTable(rows, sel), "\n")
		for i, ln := range tableLines {
			lines = append(lines, ln)
			if len(prompt) > 0 && i == 1+promptRow { // 1 for the table header row
				lines = append(lines, prompt...)
			}
		}
		if active {
			anchor := m.cursor
			if promptRow >= 0 {
				anchor = promptRow
			}
			cursorLine = blockStart + 1 + anchor + len(prompt) // keep the row (and prompt) visible
		}
	}
	appendBucket("AUTHORED", m.authored, m.section == secAuthored)
	lines = append(lines, "") // separator between buckets
	appendBucket("AWAITING MY REVIEW", m.reviewing, m.section == secReviewing)

	if m.ciEnabled() {
		lines = append(lines, "") // separator before CI
		ciStart := len(lines)
		ciLines, ciCursor := m.renderCI()
		lines = append(lines, ciLines...)
		if m.section == secCI && ciCursor >= 0 {
			cursorLine = ciStart + ciCursor
		}
	}
	return lines, cursorLine
}

// renderCI builds the CI Workflows section lines and the index (within those
// lines) of the active cursor item, or -1 when CI is not focused.
func (m *Model) renderCI() (lines []string, cursorLine int) {
	cursorLine = -1
	sparkW := m.ciSparkWidth() // LAST column scales to the run count
	lines = append(lines, sectionStyle.Render("CI Workflows"))
	// column header, dimmed, aligned to the collapsed columns
	hdr := padTo("WORKFLOW", ciNameW+colGap) + padTo("BRANCH", ciBranchW+colGap) +
		padTo("LAST", sparkW+ciSparkGap) + "UPDATED"
	lines = append(lines, dimStyle.Render(hdr))
	if len(m.workflows) == 0 {
		lines = append(lines, dimStyle.Render("  (none)"))
		return lines, cursorLine
	}
	items := m.ciItems()
	itemLine := make([]int, len(items))
	ii := 0

	appendRow := func(plain, colored string) {
		idx := ii
		itemLine[idx] = len(lines)
		if m.section == secCI && m.cursor == idx {
			line := plain
			if m.width > 0 {
				line = padTo(plain, m.width)
			}
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, colored)
		}
		ii++
	}

	for wi := range m.workflows {
		w := m.workflows[wi]
		expanded := m.expanded[wfKey(w)]
		marker := "▸"
		if expanded {
			marker = "▾"
		}
		name := padTo(marker+" "+w.Name, ciNameW+colGap)
		// Branch is shown per run row, not on the workflow header; keep an empty
		// BRANCH cell on the header so the LAST/UPDATED columns stay aligned.
		blankBranch := padTo("", ciBranchW+colGap)
		switch {
		case expanded:
			// expanded header: just the workflow name
			appendRow(name, name)
		case w.Err != nil:
			appendRow(name+blankBranch+"error", name+blankBranch+offlineStyle.Render("error"))
		default:
			updated := ""
			if len(w.Runs) > 0 {
				updated = humanizeSince(m.now().Sub(w.Runs[0].UpdatedAt)) + " ago"
			}
			runs := w.Runs // newest-first; the LAST column scales to fit them all
			plainSpark := padTo(sparklinePlain(runs), sparkW+ciSparkGap)
			colorSpark := padToWidth(sparkline(runs), ansi.StringWidth(sparklinePlain(runs)), sparkW+ciSparkGap)
			appendRow(name+blankBranch+plainSpark+updated,
				name+blankBranch+colorSpark+dimStyle.Render(updated))
		}
		if expanded {
			for ri := range w.Runs {
				r := w.Runs[ri]
				st := ci.Status(r)
				// Columns aligned under the header: #number (WORKFLOW), branch
				// (BRANCH), status glyph (LAST), "<updated> ago (<runtime>)" (UPDATED).
				num := padTo(fmt.Sprintf("    #%d", r.RunNumber), ciNameW+colGap)
				rbranch := padTo(r.Branch, ciBranchW+colGap)
				glyphPlain := padTo(st.Symbol(), sparkW+ciSparkGap)
				glyphColored := padToWidth(ciRunStyle(st).Render(st.Symbol()), ansi.StringWidth(st.Symbol()), sparkW+ciSparkGap)
				upd := ""
				if !r.UpdatedAt.IsZero() {
					upd = humanizeSince(m.now().Sub(r.UpdatedAt)) + " ago"
				}
				if !r.CreatedAt.IsZero() && r.UpdatedAt.After(r.CreatedAt) {
					upd += " (" + humanizeSince(r.UpdatedAt.Sub(r.CreatedAt)) + ")"
				}
				appendRow(num+rbranch+glyphPlain+upd,
					num+dimStyle.Render(rbranch)+glyphColored+dimStyle.Render(upd))
				// Inline details panel under the selected run while it's open.
				if m.modal == modalDetails && r.RunID == m.detailRun.RunID {
					lines = append(lines, m.ciDetailLines()...)
				}
			}
		}
	}
	if m.section == secCI && m.cursor >= 0 && m.cursor < len(items) {
		cursorLine = itemLine[m.cursor]
	}
	if m.modal == modalRerun {
		lines = append(lines, promptStyle.Render(fmt.Sprintf("   ↳ rerun failed jobs of #%d?   ⏎ rerun   esc cancel", m.rerunRun.RunNumber)))
	}
	return lines, cursorLine
}

// ciDetailLines builds the inline details panel rendered beneath the selected
// run row (status/branch/runtime already live on the row itself, so this adds the
// job breakdown, the failed step, and the analysis summary or its fallback).
func (m *Model) ciDetailLines() []string {
	const ind = "       " // align under the run row's content
	var b []string
	if m.detailErr != nil {
		return append(b, ind+offlineStyle.Render("failed to load details: "+m.detailErr.Error()))
	}
	if m.detail.Status == "" { // FetchRunDetail not back yet
		b = append(b, ind+dimStyle.Render("loading details…"))
	}
	if len(m.detail.Jobs) > 0 {
		b = append(b, ind+dimStyle.Render("jobs:"))
		for _, j := range m.detail.Jobs {
			b = append(b, ind+"  "+jobGlyph(j)+" "+j.Name)
		}
	}
	if job, step, ok := m.detail.FailedStep(); ok {
		label := job
		if step != "" {
			label += " · " + step
		}
		b = append(b, ind+dimStyle.Render("failed step: ")+offlineStyle.Render(label))
	}
	switch {
	case m.summary != "":
		for _, ln := range strings.Split(strings.TrimRight(m.summary, "\n"), "\n") {
			b = append(b, ind+ln)
		}
	case m.summaryErr != nil && errors.Is(m.summaryErr, gh.ErrNoArtifact):
		b = append(b, ind+dimStyle.Render("no analysis artifact — press o to open the run page"))
	case m.summaryErr != nil:
		b = append(b, ind+dimStyle.Render("could not load analysis: "+m.summaryErr.Error()))
	case m.detailGlob != "":
		b = append(b, ind+dimStyle.Render("loading analysis…"))
	default:
		b = append(b, ind+dimStyle.Render("press o to open the run page"))
	}
	return b
}

// jobGlyph maps a job's status/conclusion to a colored status glyph (reusing the
// run status mapping).
func jobGlyph(j gh.JobDetail) string {
	st := ci.Status(ci.Run{Status: j.Status, Conclusion: j.Conclusion})
	return ciRunStyle(st).Render(st.Symbol())
}

// promptLines is the inline confirm shown beneath the selected row while a
// merge or close is armed. Indented with a "↳" so it reads as belonging to the
// row above it.
func (m *Model) promptLines() []string {
	switch m.modal {
	case modalClose:
		if blk := m.closeBlockers(); len(blk) > 0 {
			return []string{promptStyle.Render("   ↳ ✗ can't close: " + strings.Join(blk, ", ") + "   esc")}
		}
		return []string{promptStyle.Render("   ↳ close this PR?   ⏎ close   esc cancel")}
	case modalMerge:
		if blk := m.mergeBlockers(); len(blk) > 0 {
			return []string{promptStyle.Render("   ↳ ✗ can't merge: " + strings.Join(blk, ", ") + "   esc")}
		}
		return []string{promptStyle.Render("   ↳ merge ‹ " + m.method.String() + " ›   ←/→ method   ⏎ merge   esc cancel")}
	}
	return nil
}

func selIdx(active bool, cursor int) int {
	if active {
		return cursor
	}
	return -1
}

// indexByURL returns the index of the PR with the given URL, or -1.
func indexByURL(rows []pr.PR, url string) int {
	for i := range rows {
		if rows[i].URL == url {
			return i
		}
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

// View renders the whole UI. It assembles all lines and joins them WITHOUT a
// trailing newline, so the total line count is exactly len(top)+len(visible)+
// len(footer) and never exceeds m.height.
func (m *Model) View() string {
	// The details panel renders inline beneath its run (see renderCI), so the
	// rest of the dashboard stays visible — View renders the body normally.
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
	keyText := "↑↓ move  tab switch  r refresh"
	switch {
	case m.modal == modalDetails:
		keyText = "↵/esc close  o open run page"
		if m.ciDebugEligible() {
			keyText += "  d debug"
		}
		if ci.IsFailed(m.detailRun) {
			keyText += "  R rerun"
		}
	case m.section == secCI:
		keyText += "  ↵ details/expand  o open"
		if m.ciDebugEligible() {
			keyText += "  d debug"
		}
		if r, ok := m.selectedRun(); ok && ci.IsFailed(r) {
			keyText += "  R rerun"
		}
		keyText += "  q quit"
	default:
		keyText += "  m merge  c close  o open"
		if m.reviewEligible() {
			keyText += "  v review"
		}
		keyText += "  q quit"
	}
	keys := dimStyle.Render(keyText) + hint

	all := []string{titleStyle.Render("prdash"), ""}
	all = append(all, visible...)
	all = append(all, "", status, keys)
	for i := range all {
		all[i] = m.clampLine(all[i]) // every line fits the terminal width
	}
	if m.height > 0 && len(all) > m.height {
		all = all[:m.height] // tiny-height guard: never exceed the terminal height
	}
	return strings.Join(all, "\n")
}
