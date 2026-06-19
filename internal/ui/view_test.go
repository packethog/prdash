package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/packethog/prdash/internal/pr"
)

// TestMain forces a 256-color profile so lipgloss emits ANSI escape sequences
// even when tests run outside a TTY (e.g., CI, go test pipe). It also clears
// CMUX_WORKSPACE_ID so no test in this package ever execs the real `cmux`
// binary (which would spawn a browser pane) when the suite runs inside cmux.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	_ = os.Unsetenv("CMUX_WORKSPACE_ID")
	os.Exit(m.Run())
}

func TestViewListsBucketsAndPRs(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.now = fixedClock(time.Date(2026, 6, 18, 15, 0, 12, 0, time.UTC))
	m.lastUpdated = time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	m.conn = connLive
	m.authored = []pr.PR{{Repo: "malbeclabs/doublezero", Number: 1234, Title: "fix withdraw race", ReviewDecision: "APPROVED", RollupState: "SUCCESS"}}
	m.reviewing = []pr.PR{{Repo: "malbeclabs/infra", Number: 88, Title: "bump", ReviewDecision: "REVIEW_REQUIRED", RollupState: "PENDING"}}

	out := m.View()
	for _, want := range []string{"AUTHORED", "AWAITING MY REVIEW", "malbeclabs/doublezero#1234", "Approved", "● Live", "PR", "TITLE", "REVIEW", "CI"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q", want)
		}
	}
}

func TestTitleWidthFlexes(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	if got := m.titleWidth(); got != colTitleW {
		t.Errorf("width 0 (unsized): titleWidth = %d, want default %d", got, colTitleW)
	}
	fixed := colRefW + colGap + colGap + colReviewW + colGap + colCIW
	m.width = 120
	if got, want := m.titleWidth(), 120-fixed; got != want {
		t.Errorf("width 120: titleWidth = %d, want %d", got, want)
	}
	m.width = 40 // narrower than the fixed columns → title floors at 1, never negative
	if got := m.titleWidth(); got != 1 {
		t.Errorf("width 40: titleWidth = %d, want floor 1", got)
	}
}

func TestTableFitsTerminalWidth(t *testing.T) {
	rows := []pr.PR{
		{Repo: "malbeclabs/doublezero-shreds", Number: 501, Title: strings.Repeat("long ", 20), ReviewDecision: "CHANGES_REQUESTED", RollupState: "SUCCESS"},
		{Repo: "o/r", Number: 1, Title: "メトリクスを追加 🎉🚀", ReviewDecision: "APPROVED", RollupState: "SUCCESS"},
	}
	for _, w := range []int{40, 58, 59, 60, 80, 120} { // incl. the narrow clamp path
		m := New(stubRunner{}, 45*time.Second, 50)
		m.width = w
		for _, ln := range strings.Split(m.renderTable(rows, 0), "\n") {
			if got := lipgloss.Width(ln); got > w {
				t.Errorf("width %d: table line width %d exceeds terminal: %q", w, got, stripANSI(ln))
			}
		}
	}
}

func TestTableShowsHeadersAndBadges(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width = 100
	rows := []pr.PR{{Repo: "malbeclabs/doublezero", Number: 1234, Title: "fix race", ReviewDecision: "APPROVED", RollupState: "SUCCESS", LatestReviews: []string{"APPROVED"}}}
	out := stripANSI(m.renderTable(rows, -1))
	for _, want := range []string{"PR", "TITLE", "REVIEW", "CI", "malbeclabs/doublezero#1234", "Approved"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q", want)
		}
	}
}

func TestTableSelectedRowHasBackground(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width = 80
	rows := []pr.PR{{Repo: "o/r", Number: 1, Title: "t", ReviewDecision: "APPROVED", RollupState: "SUCCESS"}}
	out := m.renderTable(rows, 0)
	// selectedStyle sets a background; its ANSI (48;5;237) must appear on the selected row.
	if !strings.Contains(out, "48;5;237") {
		t.Error("selected row should carry the selection background")
	}
}

func TestCommentedBadgeRenders(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width = 100
	rows := []pr.PR{{Repo: "o/r", Number: 2, Title: "t", LatestReviews: []string{"COMMENTED"}}}
	if !strings.Contains(stripANSI(m.renderTable(rows, -1)), "Commented") {
		t.Error("a COMMENTED PR should show the Commented badge")
	}
}

func TestSelectedRowSpansFullWidth(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width = 80
	rows := []pr.PR{
		{Repo: "o/r", Number: 1, Title: "a", ReviewDecision: "APPROVED", RollupState: "SUCCESS"},
		{Repo: "o/r", Number: 2, Title: "b", ReviewDecision: "APPROVED", RollupState: "SUCCESS"},
	}
	lines := strings.Split(m.renderTable(rows, 1), "\n")
	// find the selected data line (the one carrying the selection background)
	var sel string
	for _, ln := range lines {
		if strings.Contains(ln, "48;5;237") {
			sel = ln
		}
	}
	if sel == "" {
		t.Fatal("no selected line found")
	}
	if got := lipgloss.Width(sel); got != 80 {
		t.Errorf("selected highlight width = %d, want exactly 80 (full width)", got)
	}
}

// stripANSI removes lipgloss color escape sequences for width assertions.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestViewShowsOfflineRetry(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.conn = connOffline
	m.backoff.RecordFailure() // delay now 5s
	if !strings.Contains(m.View(), "✕ Offline") {
		t.Error("expected offline indicator")
	}
}

func TestViewMergeModalShowsBlockers(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	blocked := pr.PR{Repo: "o/r", Number: 2, Title: "x", ReviewDecision: "", RollupState: "PENDING"}
	m.authored = []pr.PR{blocked}
	m.modal = modalMerge
	m.modalPR = blocked
	out := m.View()
	if !strings.Contains(out, "can't merge") {
		t.Error("blocked merge prompt missing")
	}
	if !strings.Contains(out, "review not approved") {
		t.Error("blocked merge prompt should list blockers")
	}
}

func TestViewMergeModalArmedShowsConfirm(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.conn = connLive
	m.authored = []pr.PR{mergeablePR()}
	m.modal = modalMerge
	m.modalPR = mergeablePR()
	out := m.View()
	if !strings.Contains(out, "squash") {
		t.Error("armed modal should show the method")
	}
}

func TestViewOfflineWithDataShowsCachedPhrase(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.conn = connOffline
	m.authored = []pr.PR{mergeablePR()} // retained data
	if !strings.Contains(m.View(), "showing last-known data") {
		t.Error("offline view with cached data should say so")
	}
}

func TestViewEmptyDoesNotPanic(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	_ = m.View() // no PRs, offline; must not panic
}

func TestHumanizeSince(t *testing.T) {
	cases := map[time.Duration]string{
		12 * time.Second: "12s",
		2 * time.Minute:  "2m",
		3 * time.Hour:    "3h",
	}
	for d, want := range cases {
		if got := humanizeSince(d); got != want {
			t.Errorf("humanizeSince(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestWindowKeepsCursorVisible(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	cases := []int{0, 5, 20, 39}
	for _, cur := range cases {
		got, _ := window(lines, cur, 10)
		if len(got) != 10 {
			t.Fatalf("cursor %d: window len = %d, want 10", cur, len(got))
		}
		if !containsLine(got, fmt.Sprintf("line-%d", cur)) {
			t.Errorf("cursor %d: window does not contain the cursor line", cur)
		}
	}
}

func TestWindowShorterThanViewportReturnsAll(t *testing.T) {
	lines := []string{"a", "b", "c"}
	got, first := window(lines, 1, 10)
	if len(got) != 3 || first != 0 {
		t.Errorf("got %d lines first=%d, want all 3 at 0", len(got), first)
	}
}

func TestWindowClampsAtEnd(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("l%d", i)
	}
	got, first := window(lines, 19, 5)
	if got[len(got)-1] != "l19" {
		t.Errorf("last visible = %q, want l19", got[len(got)-1])
	}
	if len(got) != 5 || first != 15 {
		t.Errorf("len=%d first=%d, want 5 at 15", len(got), first)
	}
}

func containsLine(lines []string, s string) bool {
	for _, ln := range lines {
		if ln == s {
			return true
		}
	}
	return false
}

func TestViewWindowsLongListWithinHeight(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width, m.height = 80, 14
	m.conn = connLive
	for i := 0; i < 30; i++ {
		m.authored = append(m.authored, pr.PR{Repo: "o/r", Number: i, Title: "t", ReviewDecision: "APPROVED", RollupState: "SUCCESS"})
	}
	m.bucket = pr.Authored
	m.cursor = 27 // deep in the list
	out := m.View()
	if got := strings.Count(out, "\n") + 1; got > m.height {
		t.Errorf("View produced %d lines, exceeds height %d", got, m.height)
	}
	if !strings.Contains(stripANSI(out), "o/r#27") {
		t.Error("the selected PR (cursor=27) must remain visible after scrolling")
	}
}

func TestViewNeverExceedsHeight(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width = 80
	m.conn = connLive
	for i := 0; i < 20; i++ {
		m.authored = append(m.authored, pr.PR{Repo: "o/r", Number: i, Title: "t"})
	}
	for _, h := range []int{1, 3, 5, 8, 14, 100} { // incl. tiny heights below chrome
		m.height = h
		if got := strings.Count(m.View(), "\n") + 1; got > h {
			t.Errorf("height %d: View produced %d lines", h, got)
		}
	}
}

func TestViewRoutesCloseModal(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width, m.height = 80, 24
	m.conn = connLive
	m.authored = []pr.PR{{Repo: "o/r", Number: 7, Title: "t"}}
	m.modal = modalClose
	m.modalPR = m.authored[0]
	out := stripANSI(m.View())
	if !strings.Contains(out, "close this PR?") {
		t.Error("View should render the inline close prompt")
	}
	if !strings.Contains(out, "esc cancel") {
		t.Error("armed close prompt should show the confirm keys")
	}
}

func TestCloseModalShowsBlockerWhenNotLive(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width, m.height = 80, 24
	m.conn = connOffline
	m.authored = []pr.PR{{Repo: "o/r", Number: 7, Title: "t"}}
	m.modal = modalClose
	m.modalPR = m.authored[0]
	if !strings.Contains(stripANSI(m.View()), "connection not live") {
		t.Error("blocked close modal should show the connection blocker")
	}
}

func TestInlinePromptSitsUnderSelectedRow(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width, m.height = 90, 24
	m.conn = connLive
	m.authored = []pr.PR{
		{Repo: "o/r", Number: 1, URL: "u1", Title: "first"},
		{Repo: "o/r", Number: 2, URL: "u2", Title: "second"},
	}
	m.cursor = 1
	m.modal = modalClose
	m.modalPR = m.authored[1]
	out := stripANSI(m.View())
	// list and footer stay visible (inline, not a full-screen overlay)
	if !strings.Contains(out, "o/r#1") {
		t.Error("non-selected rows must stay visible")
	}
	if !strings.Contains(out, "tab switch") {
		t.Error("footer must remain visible")
	}
	// the prompt sits on the line directly below the selected row (#2)
	lines := strings.Split(out, "\n")
	sel := -1
	for i, ln := range lines {
		if strings.Contains(ln, "o/r#2") {
			sel = i
		}
	}
	if sel < 0 || sel+1 >= len(lines) || !strings.Contains(lines[sel+1], "close this PR?") {
		t.Fatalf("prompt should sit directly below the selected row (sel=%d)", sel)
	}
}

func TestActionedRowRendersStruck(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width = 90
	rows := []pr.PR{{Repo: "o/r", Number: 1, URL: "u1", Title: "t", ReviewDecision: "APPROVED", RollupState: "SUCCESS"}}
	plain := m.renderTable(rows, -1)
	m.markActioned("u1")
	struck := m.renderTable(rows, -1)
	if plain == struck {
		t.Error("an actioned (closed/merged) row should render struck-through, not identical")
	}
}

func TestInlinePromptFollowsCapturedPRAfterReorder(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width, m.height = 90, 24
	m.conn = connLive
	a := pr.PR{Repo: "o/r", Number: 1, URL: "u1", Title: "one"}
	b := pr.PR{Repo: "o/r", Number: 2, URL: "u2", Title: "two"}
	m.cursor = 0
	m.modal = modalClose
	m.modalPR = a // captured #1
	// a background refetch reordered the rows; cursor index now points at #2.
	m.authored = []pr.PR{b, a}
	lines := strings.Split(stripANSI(m.View()), "\n")
	sel := -1
	for i, ln := range lines {
		if strings.Contains(ln, "o/r#1") { // the captured PR, now at index 1
			sel = i
		}
	}
	if sel < 0 || sel+1 >= len(lines) || !strings.Contains(lines[sel+1], "close this PR?") {
		t.Fatal("prompt must follow the captured PR (#1) after a reorder, not the cursor row")
	}
}

func TestFooterShowsCloseKey(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.width, m.height = 80, 24
	m.conn = connLive
	if !strings.Contains(stripANSI(m.View()), "c close") {
		t.Error("footer should advertise the close key")
	}
}
