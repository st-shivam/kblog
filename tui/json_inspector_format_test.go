package tui

import (
	"strings"
	"testing"
)

func TestJSONInspector_SetContent_DetectsAndRejects(t *testing.T) {
	j := NewJSONInspector()

	j.SetContent(`{"a":1,"b":"two"}`)
	if !j.IsJSON {
		t.Error("a JSON object should be detected as JSON")
	}
	if j.ParsedJSON["b"] != "two" {
		t.Errorf("parsed b = %v, want two", j.ParsedJSON["b"])
	}

	j.SetContent("not json at all")
	if j.IsJSON {
		t.Error("plain text should not be detected as JSON")
	}
	if j.ParsedJSON != nil {
		t.Error("ParsedJSON should be reset to nil for non-JSON content")
	}

	// Malformed JSON should not be treated as JSON.
	j.SetContent(`{"a":1`)
	if j.IsJSON {
		t.Error("malformed JSON should not be detected as JSON")
	}
}

func TestJSONInspector_FormatRawText_WrapsLongInput(t *testing.T) {
	InitStyles(Themes[0])
	j := NewJSONInspector()
	j.RawContent = strings.Repeat("word ", 50) // ~250 chars

	out, total := j.formatRawText(40)
	if total < 2 {
		t.Errorf("long input should wrap to multiple lines, got total=%d", total)
	}
	if !strings.Contains(out, "\n") {
		t.Error("wrapped output should contain newlines")
	}
}

func TestJSONInspector_FormatRawText_EmptyInput(t *testing.T) {
	InitStyles(Themes[0])
	j := NewJSONInspector()
	j.RawContent = ""
	_, total := j.formatRawText(40)
	if total != 1 {
		t.Errorf("empty input should be a single line, got total=%d", total)
	}
}

func TestJSONInspector_FormatJSONTree_SortsAndCountsKeys(t *testing.T) {
	InitStyles(Themes[0])
	j := NewJSONInspector()
	j.SetContent(`{"zebra":"z","alpha":"a","mid":"m"}`)
	if !j.IsJSON {
		t.Fatal("setup: expected JSON")
	}

	out, total := j.formatJSONTree(80)
	// Three keys + opening and closing braces => at least 5 lines.
	if total < 5 {
		t.Errorf("expected >=5 rendered lines for 3 keys, got %d", total)
	}
	// Keys are rendered in sorted order: alpha before mid before zebra.
	ai, mi, zi := strings.Index(out, "alpha"), strings.Index(out, "mid"), strings.Index(out, "zebra")
	if ai < 0 || ai >= mi || mi >= zi {
		t.Errorf("keys should be sorted alpha<mid<zebra, got positions %d,%d,%d", ai, mi, zi)
	}
}
