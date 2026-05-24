package tui

import (
	"strings"
	"testing"
	"time"

	"kblog/k8s"
)

// ── wrapString ──────────────────────────────────────────────────────────────

func TestWrapString_ShortLine(t *testing.T) {
	got := wrapString("hello world", 40)
	if len(got) != 1 || got[0] != "hello world" {
		t.Errorf("short line should not wrap, got %v", got)
	}
}

func TestWrapString_ExactWidth(t *testing.T) {
	got := wrapString("12345", 5)
	if len(got) != 1 || got[0] != "12345" {
		t.Errorf("exact-width line should not wrap, got %v", got)
	}
}

func TestWrapString_BreaksAtSpace(t *testing.T) {
	got := wrapString("hello world foo bar", 12)
	// Should break at a space, not mid-word
	for _, line := range got {
		if len([]rune(line)) > 12 {
			t.Errorf("wrapped line %q exceeds width 12", line)
		}
	}
	// Rejoined content should equal original (spaces at breaks are consumed)
	rejoined := strings.Join(got, " ")
	if rejoined != "hello world foo bar" {
		t.Errorf("rejoined = %q, want %q", rejoined, "hello world foo bar")
	}
}

func TestWrapString_HardBreakNoSpace(t *testing.T) {
	// A single long word with no spaces must still break
	got := wrapString("abcdefghijklmnop", 5)
	for _, line := range got {
		if len([]rune(line)) > 5 {
			t.Errorf("hard-break line %q exceeds width 5", line)
		}
	}
	if strings.Join(got, "") != "abcdefghijklmnop" {
		t.Errorf("content lost during hard break: %v", got)
	}
}

func TestWrapString_EmptyString(t *testing.T) {
	got := wrapString("", 20)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("empty string wrap = %v, want [\"\"]", got)
	}
}

func TestWrapString_ZeroWidth(t *testing.T) {
	// Zero width — should not panic; returns the whole string
	got := wrapString("hello", 0)
	if len(got) == 0 {
		t.Error("zero-width wrap returned empty slice")
	}
}

// ── parseSearchQuery ─────────────────────────────────────────────────────────

func TestParseSearchQuery_Empty(t *testing.T) {
	filters, re := parseSearchQuery("")
	if filters != nil || re != nil {
		t.Error("empty query should return nil filters and nil regex")
	}
}

func TestParseSearchQuery_FieldFilterOnly(t *testing.T) {
	filters, re := parseSearchQuery("level=error")
	if len(filters) != 1 {
		t.Fatalf("expected 1 field filter, got %d", len(filters))
	}
	if filters[0].key != "level" || filters[0].value != "error" {
		t.Errorf("filter = %+v, want {level error}", filters[0])
	}
	if re != nil {
		t.Error("no text regex expected")
	}
}

func TestParseSearchQuery_MultipleFieldFilters(t *testing.T) {
	filters, re := parseSearchQuery("level=error request_id=abc-123")
	if len(filters) != 2 {
		t.Fatalf("expected 2 field filters, got %d", len(filters))
	}
	if re != nil {
		t.Error("no text regex expected")
	}
}

func TestParseSearchQuery_MixedFieldAndText(t *testing.T) {
	filters, re := parseSearchQuery("level=warn timeout")
	if len(filters) != 1 {
		t.Fatalf("expected 1 field filter, got %d", len(filters))
	}
	if re == nil {
		t.Fatal("expected text regex for 'timeout'")
	}
	if !re.MatchString("connection timeout") {
		t.Error("text regex should match 'connection timeout'")
	}
}

func TestParseSearchQuery_PlainTextOnly(t *testing.T) {
	filters, re := parseSearchQuery("nginx error")
	if len(filters) != 0 {
		t.Errorf("expected no field filters for plain text, got %v", filters)
	}
	if re == nil {
		t.Fatal("expected a text regex")
	}
}

// ── passesFilters ────────────────────────────────────────────────────────────

func newViewport() *Viewport {
	v := NewViewport()
	// Pre-initialize styles so lipgloss calls don't panic
	InitStyles(Themes[0])
	return v
}

func makeLogLine(content, level string, fields map[string]string) k8s.LogLine {
	return k8s.LogLine{
		Timestamp: time.Now(),
		Pod:       "test-pod",
		Container: "app",
		Content:   content,
		Level:     level,
		Fields:    fields,
	}
}

func TestPassesFilters_NoFilters(t *testing.T) {
	v := newViewport()
	line := makeLogLine("hello world", "INFO", nil)
	if !v.passesFilters(line, nil) {
		t.Error("line should pass with no filters active")
	}
}

func TestPassesFilters_LevelFilterError(t *testing.T) {
	v := newViewport()
	v.FilterLevel = "ERROR"

	info := makeLogLine("all good", "INFO", nil)
	if v.passesFilters(info, nil) {
		t.Error("INFO line should not pass ERROR-only filter")
	}

	errLine := makeLogLine("boom", "ERROR", nil)
	if !v.passesFilters(errLine, nil) {
		t.Error("ERROR line should pass ERROR-only filter")
	}
}

func TestPassesFilters_LevelFilterWarn(t *testing.T) {
	v := newViewport()
	v.FilterLevel = "WARN"

	debug := makeLogLine("trace", "DEBUG", nil)
	if v.passesFilters(debug, nil) {
		t.Error("DEBUG should not pass WARN filter")
	}
	info := makeLogLine("ok", "INFO", nil)
	if v.passesFilters(info, nil) {
		t.Error("INFO should not pass WARN filter")
	}
	warn := makeLogLine("low disk", "WARN", nil)
	if !v.passesFilters(warn, nil) {
		t.Error("WARN should pass WARN filter")
	}
	errLine := makeLogLine("boom", "ERROR", nil)
	if !v.passesFilters(errLine, nil) {
		t.Error("ERROR should pass WARN filter")
	}
}

func TestPassesFilters_FieldFilter(t *testing.T) {
	v := newViewport()
	v.fieldFilters = []fieldFilter{{key: "request_id", value: "abc-123"}}

	noFields := makeLogLine("plain", "INFO", nil)
	if v.passesFilters(noFields, nil) {
		t.Error("line with no fields should not pass field filter")
	}

	wrongField := makeLogLine("json", "INFO", map[string]string{"request_id": "xyz-999"})
	if v.passesFilters(wrongField, nil) {
		t.Error("wrong field value should not pass field filter")
	}

	match := makeLogLine("json", "INFO", map[string]string{"request_id": "abc-123", "level": "info"})
	if !v.passesFilters(match, nil) {
		t.Error("matching field should pass")
	}
}

func TestPassesFilters_FieldFilterCaseInsensitive(t *testing.T) {
	v := newViewport()
	v.fieldFilters = []fieldFilter{{key: "level", value: "ERROR"}}

	line := makeLogLine("json", "INFO", map[string]string{"level": "error"})
	if !v.passesFilters(line, nil) {
		t.Error("field filter should be case-insensitive")
	}
}

func TestPassesFilters_TextRegex(t *testing.T) {
	v := newViewport()
	_, re := parseSearchQuery("timeout")
	v.textRegex = re

	miss := makeLogLine("connection closed", "INFO", nil)
	if v.passesFilters(miss, nil) {
		t.Error("line without 'timeout' should not pass text filter")
	}

	hit := makeLogLine("read timeout after 30s", "WARN", nil)
	if !v.passesFilters(hit, nil) {
		t.Error("line with 'timeout' should pass text filter")
	}
}

func TestPassesFilters_ContainerFilter(t *testing.T) {
	v := newViewport()
	selected := map[string]bool{"sidecar": true}

	appLine := makeLogLine("log", "INFO", nil)
	appLine.Container = "app"
	if v.passesFilters(appLine, selected) {
		t.Error("app container should not pass when only sidecar is selected")
	}

	sidecarLine := makeLogLine("log", "INFO", nil)
	sidecarLine.Container = "sidecar"
	if !v.passesFilters(sidecarLine, selected) {
		t.Error("sidecar container should pass when sidecar is selected")
	}
}

// ── AddLineWithFilter sort direction ─────────────────────────────────────────

func makeTimedLine(content string, secondsAgo int) k8s.LogLine {
	return k8s.LogLine{
		Timestamp: time.Now().Add(-time.Duration(secondsAgo) * time.Second),
		Pod:       "pod",
		Container: "app",
		Content:   content,
		Level:     "INFO",
	}
}

func TestAddLineWithFilter_AscendingAppend(t *testing.T) {
	v := newViewport()
	v.AutoScroll = false

	older := makeTimedLine("older", 10)
	newer := makeTimedLine("newer", 1)
	v.AddLineWithFilter(older, nil)
	v.AddLineWithFilter(newer, nil)

	if len(v.FilteredLines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(v.FilteredLines))
	}
	if v.FilteredLines[0].Content != "older" || v.FilteredLines[1].Content != "newer" {
		t.Errorf("ascending: expected [older, newer], got [%s, %s]",
			v.FilteredLines[0].Content, v.FilteredLines[1].Content)
	}
}

func TestAddLineWithFilter_DescendingPrepend(t *testing.T) {
	v := newViewport()
	v.SortDescending = true
	v.AutoScroll = false

	older := makeTimedLine("older", 10)
	newer := makeTimedLine("newer", 1)
	v.AddLineWithFilter(older, nil)
	v.AddLineWithFilter(newer, nil)

	if len(v.FilteredLines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(v.FilteredLines))
	}
	// Descending: newest arrives last, gets prepended → [newer, older]
	if v.FilteredLines[0].Content != "newer" || v.FilteredLines[1].Content != "older" {
		t.Errorf("descending: expected [newer, older], got [%s, %s]",
			v.FilteredLines[0].Content, v.FilteredLines[1].Content)
	}
}

// ── UpdateFilters sort direction ─────────────────────────────────────────────

func TestUpdateFilters_SortAscending(t *testing.T) {
	v := newViewport()
	v.AllLines = []k8s.LogLine{
		makeTimedLine("newest", 1),
		makeTimedLine("middle", 5),
		makeTimedLine("oldest", 10),
	}
	v.SortDescending = false
	v.UpdateFilters(nil)

	if v.FilteredLines[0].Content != "oldest" {
		t.Errorf("ascending sort: first should be oldest, got %q", v.FilteredLines[0].Content)
	}
	if v.FilteredLines[2].Content != "newest" {
		t.Errorf("ascending sort: last should be newest, got %q", v.FilteredLines[2].Content)
	}
}

func TestUpdateFilters_SortDescending(t *testing.T) {
	v := newViewport()
	v.AllLines = []k8s.LogLine{
		makeTimedLine("oldest", 10),
		makeTimedLine("middle", 5),
		makeTimedLine("newest", 1),
	}
	v.SortDescending = true
	v.UpdateFilters(nil)

	if v.FilteredLines[0].Content != "newest" {
		t.Errorf("descending sort: first should be newest, got %q", v.FilteredLines[0].Content)
	}
	if v.FilteredLines[2].Content != "oldest" {
		t.Errorf("descending sort: last should be oldest, got %q", v.FilteredLines[2].Content)
	}
}

func TestUpdateFilters_FieldFilter(t *testing.T) {
	v := newViewport()
	v.AllLines = []k8s.LogLine{
		makeLogLine("match", "INFO", map[string]string{"svc": "auth", "request_id": "req-1"}),
		makeLogLine("no-fields", "INFO", nil),
		makeLogLine("wrong-svc", "INFO", map[string]string{"svc": "billing"}),
	}
	v.SearchQuery = "svc=auth"
	v.UpdateFilters(nil)

	if len(v.FilteredLines) != 1 || v.FilteredLines[0].Content != "match" {
		t.Errorf("field filter: expected [match], got %v", v.FilteredLines)
	}
}

func TestUpdateFilters_MultipleFieldFilters(t *testing.T) {
	v := newViewport()
	v.AllLines = []k8s.LogLine{
		makeLogLine("both", "INFO", map[string]string{"svc": "auth", "env": "prod"}),
		makeLogLine("only-svc", "INFO", map[string]string{"svc": "auth", "env": "dev"}),
		makeLogLine("neither", "INFO", map[string]string{"svc": "billing"}),
	}
	v.SearchQuery = "svc=auth env=prod"
	v.UpdateFilters(nil)

	if len(v.FilteredLines) != 1 || v.FilteredLines[0].Content != "both" {
		t.Errorf("multi field filter: expected [both], got %v", v.FilteredLines)
	}
}

func TestUpdateFilters_MixedFieldAndTextFilter(t *testing.T) {
	v := newViewport()
	v.AllLines = []k8s.LogLine{
		makeLogLine("auth timeout", "WARN", map[string]string{"svc": "auth"}),
		makeLogLine("auth ok", "INFO", map[string]string{"svc": "auth"}),
		makeLogLine("billing timeout", "WARN", map[string]string{"svc": "billing"}),
	}
	v.SearchQuery = "svc=auth timeout"
	v.UpdateFilters(nil)

	if len(v.FilteredLines) != 1 || v.FilteredLines[0].Content != "auth timeout" {
		t.Errorf("mixed filter: expected [auth timeout], got %v", v.FilteredLines)
	}
}

// ── GetSelectionLines ─────────────────────────────────────────────────────────

func TestGetSelectionLines_NoSelection(t *testing.T) {
	v := newViewport()
	v.FilteredLines = []k8s.LogLine{makeLogLine("a", "INFO", nil)}

	lines, ok := v.GetSelectionLines()
	if ok || lines != nil {
		t.Error("GetSelectionLines should return (nil, false) when not active")
	}
}

func TestGetSelectionLines_ForwardSelection(t *testing.T) {
	v := newViewport()
	v.FilteredLines = []k8s.LogLine{
		makeLogLine("line0", "INFO", nil),
		makeLogLine("line1", "INFO", nil),
		makeLogLine("line2", "INFO", nil),
		makeLogLine("line3", "INFO", nil),
	}
	v.SelectionActive = true
	v.SelectionStart = 1
	v.CursorY = 3

	lines, ok := v.GetSelectionLines()
	if !ok {
		t.Fatal("expected selection to be active")
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	contents := []string{lines[0].Content, lines[1].Content, lines[2].Content}
	expected := []string{"line1", "line2", "line3"}
	for i, c := range contents {
		if c != expected[i] {
			t.Errorf("lines[%d].Content = %q, want %q", i, c, expected[i])
		}
	}
}

func TestGetSelectionLines_ReverseSelection(t *testing.T) {
	v := newViewport()
	v.FilteredLines = []k8s.LogLine{
		makeLogLine("line0", "INFO", nil),
		makeLogLine("line1", "INFO", nil),
		makeLogLine("line2", "INFO", nil),
	}
	// Cursor moved up past anchor — selection should normalise lo/hi
	v.SelectionActive = true
	v.SelectionStart = 2
	v.CursorY = 0

	lines, ok := v.GetSelectionLines()
	if !ok {
		t.Fatal("expected selection to be active")
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (full range), got %d", len(lines))
	}
}

func TestGetSelectionLines_SingleLine(t *testing.T) {
	v := newViewport()
	v.FilteredLines = []k8s.LogLine{
		makeLogLine("only", "INFO", nil),
	}
	v.SelectionActive = true
	v.SelectionStart = 0
	v.CursorY = 0

	lines, ok := v.GetSelectionLines()
	if !ok || len(lines) != 1 || lines[0].Content != "only" {
		t.Errorf("single-line selection failed: ok=%v, lines=%v", ok, lines)
	}
}

func TestClearSelection(t *testing.T) {
	v := newViewport()
	v.SelectionActive = true
	v.SelectionStart = 5
	v.ClearSelection()

	if v.SelectionActive {
		t.Error("SelectionActive should be false after ClearSelection")
	}
	if v.SelectionStart != 0 {
		t.Error("SelectionStart should be reset to 0")
	}
}

// ── WrapLines integration ─────────────────────────────────────────────────────

func TestFormatLogLine_WrapProducesMultipleLines(t *testing.T) {
	v := newViewport()
	v.WrapLines = true
	// force-register a pod color so GetPodColor doesn't index-out
	_ = v.GetPodColor("test-pod")

	longContent := strings.Repeat("word ", 30) // 150 chars
	line := makeLogLine(longContent, "INFO", nil)

	rendered := v.formatLogLine(line, 60, false, false)
	if !strings.Contains(rendered, "\n") {
		t.Error("wrapped line should contain newlines for long content")
	}
}

func TestFormatLogLine_NoWrapSingleLine(t *testing.T) {
	v := newViewport()
	v.WrapLines = false
	_ = v.GetPodColor("test-pod")

	line := makeLogLine("short content", "INFO", nil)
	rendered := v.formatLogLine(line, 80, false, false)
	if strings.Contains(rendered, "\n") {
		t.Error("non-wrapped short line should not contain newlines")
	}
}

func TestFormatLogLine_EventNotWrapped(t *testing.T) {
	v := newViewport()
	v.WrapLines = true
	_ = v.GetPodColor("test-pod")

	eventLine := k8s.LogLine{
		Timestamp: time.Now(),
		Pod:       "test-pod",
		Container: "app",
		Content:   strings.Repeat("event-word ", 30),
		IsEvent:   true,
		Level:     "WARN",
	}
	rendered := v.formatLogLine(eventLine, 60, false, false)
	// Events bypass wrap — rendered as a single styled line (no embedded \n)
	if strings.Contains(rendered, "\n") {
		t.Error("event lines should not be word-wrapped")
	}
}
