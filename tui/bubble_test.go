package tui

import (
	"strings"
	"testing"
	"time"

	"kblog/k8s"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a minimal Model wired to a dummy channel, suitable for
// key-event testing. K8s streamer/watcher/cancel are all nil — Update() only
// touches them inside Shutdown(), which is not called by these tests.
func newTestModel() *Model {
	InitStyles(Themes[0])
	ch := make(chan k8s.LogLine, 1)
	m := NewModel("ctx", "ns", "pod", "", "v9.9.9-test", ch, nil, nil, nil)
	m.width = 120
	m.height = 40
	return m
}

func pressKey(m *Model, key string) *Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	result := updated.(Model)
	return &result
}

func pressSpecialKey(m *Model, t tea.KeyType) *Model {
	updated, _ := m.Update(tea.KeyMsg{Type: t})
	result := updated.(Model)
	return &result
}

// ── Level filter keybindings ──────────────────────────────────────────────────

func TestLevelFilter_ZeroKey_ClearsFilter(t *testing.T) {
	m := newTestModel()
	m.viewport.FilterLevel = "ERROR"
	m = pressKey(m, "0")
	if m.viewport.FilterLevel != "ALL" {
		t.Errorf("0 key should set FilterLevel=ALL, got %q", m.viewport.FilterLevel)
	}
	if !strings.Contains(m.statusMsg, "ALL") {
		t.Errorf("status message should mention ALL, got %q", m.statusMsg)
	}
}

func TestLevelFilter_OneKey_ErrorOnly(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "1")
	if m.viewport.FilterLevel != "ERROR" {
		t.Errorf("1 key should set FilterLevel=ERROR, got %q", m.viewport.FilterLevel)
	}
}

func TestLevelFilter_TwoKey_WarnAndAbove(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "2")
	if m.viewport.FilterLevel != "WARN" {
		t.Errorf("2 key should set FilterLevel=WARN, got %q", m.viewport.FilterLevel)
	}
}

func TestLevelFilter_ThreeKey_DebugAndAbove(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "3")
	if m.viewport.FilterLevel != "DEBUG" {
		t.Errorf("3 key should set FilterLevel=DEBUG, got %q", m.viewport.FilterLevel)
	}
}

func TestOldLevelKeys_NotHandledAsFilter(t *testing.T) {
	// 'e', 'w', 'd', 'a' must no longer act as level filters
	m := newTestModel()
	original := m.viewport.FilterLevel

	m2 := pressKey(m, "e")
	if m2.viewport.FilterLevel != original {
		t.Errorf("'e' key should not change FilterLevel (was %q, got %q)", original, m2.viewport.FilterLevel)
	}

	m3 := pressKey(m, "d")
	if m3.viewport.FilterLevel != original {
		t.Errorf("'d' key should not change FilterLevel (was %q, got %q)", original, m3.viewport.FilterLevel)
	}

	m4 := pressKey(m, "a")
	if m4.viewport.FilterLevel != original {
		t.Errorf("'a' key should not change FilterLevel (was %q, got %q)", original, m4.viewport.FilterLevel)
	}
}

// ── Word wrap keybinding ──────────────────────────────────────────────────────

func TestWrapToggle(t *testing.T) {
	m := newTestModel()
	if m.viewport.WrapLines {
		t.Fatal("wrap should be off by default")
	}

	m = pressKey(m, "w")
	if !m.viewport.WrapLines {
		t.Error("'w' should enable wrap")
	}
	if !strings.Contains(m.statusMsg, "ON") {
		t.Errorf("status should say ON, got %q", m.statusMsg)
	}

	m = pressKey(m, "w")
	if m.viewport.WrapLines {
		t.Error("second 'w' should disable wrap")
	}
	if !strings.Contains(m.statusMsg, "OFF") {
		t.Errorf("status should say OFF, got %q", m.statusMsg)
	}
}

func TestWrapUppercaseW(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "W")
	if !m.viewport.WrapLines {
		t.Error("'W' should also toggle wrap")
	}
}

// ── Sort keybinding ───────────────────────────────────────────────────────────

func TestSortToggle(t *testing.T) {
	m := newTestModel()
	if m.viewport.SortDescending {
		t.Fatal("sort should be ascending by default")
	}

	m = pressKey(m, "s")
	if !m.viewport.SortDescending {
		t.Error("'s' should switch to descending sort")
	}
	if !strings.Contains(m.statusMsg, "newest") {
		t.Errorf("status should mention newest-first, got %q", m.statusMsg)
	}

	m = pressKey(m, "s")
	if m.viewport.SortDescending {
		t.Error("second 's' should return to ascending sort")
	}
	if !strings.Contains(m.statusMsg, "oldest") {
		t.Errorf("status should mention oldest-first, got %q", m.statusMsg)
	}
}

// ── Visual selection keybinding ───────────────────────────────────────────────

func TestVisualSelect_ActivatesOnV(t *testing.T) {
	m := newTestModel()
	m.viewport.FilteredLines = []k8s.LogLine{
		{Timestamp: time.Now(), Pod: "p", Content: "a", Level: "INFO"},
		{Timestamp: time.Now(), Pod: "p", Content: "b", Level: "INFO"},
	}
	m.viewport.CursorY = 1

	m = pressKey(m, "v")
	if !m.viewport.SelectionActive {
		t.Error("'v' should activate selection")
	}
	if m.viewport.SelectionStart != 1 {
		t.Errorf("SelectionStart should be CursorY=1, got %d", m.viewport.SelectionStart)
	}
}

func TestVisualSelect_CancelsOnSecondV(t *testing.T) {
	m := newTestModel()
	m.viewport.FilteredLines = []k8s.LogLine{
		{Timestamp: time.Now(), Pod: "p", Content: "a", Level: "INFO"},
	}
	m = pressKey(m, "v") // activate
	m = pressKey(m, "v") // cancel
	if m.viewport.SelectionActive {
		t.Error("second 'v' should cancel selection")
	}
}

func TestVisualSelect_EscCancels(t *testing.T) {
	m := newTestModel()
	m.viewport.FilteredLines = []k8s.LogLine{
		{Timestamp: time.Now(), Pod: "p", Content: "a", Level: "INFO"},
	}
	m = pressKey(m, "v")
	if !m.viewport.SelectionActive {
		t.Fatal("setup: selection should be active")
	}
	m = pressSpecialKey(m, tea.KeyEsc)
	if m.viewport.SelectionActive {
		t.Error("Esc should cancel selection")
	}
}

// ── Copy keybinding ───────────────────────────────────────────────────────────

func TestCopy_SingleLine_UsesContent(t *testing.T) {
	m := newTestModel()
	m.viewport.FilteredLines = []k8s.LogLine{
		{Timestamp: time.Now(), Pod: "p", Container: "app", Content: "raw log content here", Level: "INFO"},
	}
	m.viewport.CursorY = 0

	// We can't easily verify clipboard contents in a test, but we can verify
	// the status message is set correctly (copy attempted).
	m = pressKey(m, "c")
	// Status should mention "copied" or "failed" (failed is OK in CI without pbcopy)
	if m.statusMsg == "" {
		t.Error("status message should be set after copy attempt")
	}
	// Must NOT leave selection active
	if m.viewport.SelectionActive {
		t.Error("selection should remain inactive after single-line copy")
	}
}

func TestCopy_Multiline_ClearsSelection(t *testing.T) {
	m := newTestModel()
	m.viewport.FilteredLines = []k8s.LogLine{
		{Timestamp: time.Now(), Pod: "p", Content: "line0", Level: "INFO"},
		{Timestamp: time.Now(), Pod: "p", Content: "line1", Level: "INFO"},
		{Timestamp: time.Now(), Pod: "p", Content: "line2", Level: "INFO"},
	}
	m.viewport.SelectionActive = true
	m.viewport.SelectionStart = 0
	m.viewport.CursorY = 2

	m = pressKey(m, "c")
	// Selection must be cleared after copy
	if m.viewport.SelectionActive {
		t.Error("selection should be cleared after multiline copy")
	}
	// Status must mention line count or copy
	if m.statusMsg == "" {
		t.Error("status message should be set after multiline copy attempt")
	}
}

// ── Footer reflects current state ─────────────────────────────────────────────

func TestView_FooterShowsSortIndicator(t *testing.T) {
	m := newTestModel()

	view := m.View()
	if !strings.Contains(view, "↑") {
		t.Error("footer should show ↑ indicator for ascending sort")
	}

	m = pressKey(m, "s")
	view = m.View()
	if !strings.Contains(view, "↓") {
		t.Error("footer should show ↓ indicator for descending sort")
	}
}

func TestView_HeaderShowsBuildVersion(t *testing.T) {
	m := newTestModel() // version "v9.9.9-test"
	view := m.View()
	if !strings.Contains(view, "v9.9.9-test") {
		t.Errorf("header should show the build version v9.9.9-test, got %q", view)
	}
	if strings.Contains(view, "v1.0.0") {
		t.Errorf("header must not show the old hardcoded v1.0.0, got %q", view)
	}
}

func TestView_HeaderVersionFallsBackToDev(t *testing.T) {
	InitStyles(Themes[0])
	ch := make(chan k8s.LogLine, 1)
	m := NewModel("ctx", "ns", "pod", "", "", ch, nil, nil, nil)
	m.width = 120
	m.height = 40
	if !strings.Contains(m.View(), "dev") {
		t.Errorf("empty version should fall back to 'dev', got %q", m.View())
	}
}

func TestView_FooterNoFormatErrors(t *testing.T) {
	m := newTestModel()

	// Default state: sidebar closed.
	view := m.View()
	if strings.Contains(view, "%!(EXTRA") {
		t.Errorf("footer (sidebar closed) must not contain a fmt EXTRA error: %q", view)
	}
	if strings.Contains(view, "%!") {
		t.Errorf("footer (sidebar closed) must not contain any fmt verb error: %q", view)
	}

	// Sidebar open state.
	m.showSidebar = true
	view = m.View()
	if strings.Contains(view, "%!(EXTRA") {
		t.Errorf("footer (sidebar open) must not contain a fmt EXTRA error: %q", view)
	}
	if strings.Contains(view, "%!") {
		t.Errorf("footer (sidebar open) must not contain any fmt verb error: %q", view)
	}
}

func TestView_FooterLabelsMatchState(t *testing.T) {
	m := newTestModel()

	// Sidebar closed: the toggle hint should read "l=sidebar" and search should
	// be bound to "/", not shifted onto another key.
	view := m.View()
	if !strings.Contains(view, "=sidebar") {
		t.Errorf("footer (sidebar closed) should advertise the sidebar toggle, got %q", view)
	}
	if !strings.Contains(view, "=search") {
		t.Errorf("footer should advertise search, got %q", view)
	}

	// Sidebar open: close/focus/toggle hints should appear.
	m.showSidebar = true
	view = m.View()
	if !strings.Contains(view, "=close") || !strings.Contains(view, "=focus") || !strings.Contains(view, "=toggle") {
		t.Errorf("footer (sidebar open) should advertise close/focus/toggle, got %q", view)
	}
}

func TestView_StickyStreamErrorInFooter(t *testing.T) {
	m := newTestModel()
	m.viewport.StreamError = "Log stream disconnected. No further logs will arrive."
	view := m.View()
	if !strings.Contains(view, "Log stream disconnected.") {
		t.Errorf("footer should show the sticky stream error, got %q", view)
	}
}

func TestView_FooterShowsWrapCheckmark(t *testing.T) {
	m := newTestModel()

	// Before enabling wrap: no checkmark
	view := m.View()
	if strings.Contains(view, "w✓") {
		t.Error("footer should not show w✓ before wrap is enabled")
	}

	m = pressKey(m, "w")
	view = m.View()
	if !strings.Contains(view, "w✓") {
		t.Error("footer should show w✓ when wrap is enabled")
	}
}
