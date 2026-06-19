package tui

import (
	"strings"
	"testing"
)

func TestWindowRange(t *testing.T) {
	cases := []struct {
		name                   string
		total, cursor, visible int
		wantStart, wantEnd     int
	}{
		{"fits entirely", 3, 0, 10, 0, 3},
		{"cursor at top", 10, 0, 4, 0, 4},
		{"cursor centered", 10, 5, 4, 3, 7},
		{"cursor at bottom clamps", 10, 9, 4, 6, 10},
		{"zero visible treated as one", 5, 2, 0, 2, 3},
	}
	for _, c := range cases {
		start, end := windowRange(c.total, c.cursor, c.visible)
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("%s: windowRange(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.name, c.total, c.cursor, c.visible, start, end, c.wantStart, c.wantEnd)
		}
		// Cursor must always be inside the returned window.
		if c.cursor < start || c.cursor >= end {
			t.Errorf("%s: cursor %d not within window [%d,%d)", c.name, c.cursor, start, end)
		}
	}
}

func TestSidebar_CursorClampsWithinItems(t *testing.T) {
	s := NewSidebar()
	for _, c := range []string{"a", "b", "c"} {
		s.AddItem(c)
	}
	// Over-advance the cursor.
	for i := 0; i < 10; i++ {
		s.CursorDown()
	}
	if s.Cursor != 2 {
		t.Errorf("cursor should clamp to last index 2, got %d", s.Cursor)
	}
	for i := 0; i < 10; i++ {
		s.CursorUp()
	}
	if s.Cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", s.Cursor)
	}
}

func TestSidebar_RenderWindowsLongList(t *testing.T) {
	InitStyles(Themes[0])
	s := NewSidebar()
	for i := 0; i < 50; i++ {
		s.AddItem("container-" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
	}
	s.Focused = true
	s.Cursor = 0

	// Small sidebar: with 50 items the render must indicate more items below
	// rather than listing all 50 (which would overflow the panel).
	out := s.Render(26, 12)
	if !strings.Contains(out, "more") {
		t.Errorf("expected an overflow indicator for a long windowed list, got:\n%s", out)
	}

	// Moving the cursor to the end should keep it within the window and show a
	// "more above" indicator.
	s.Cursor = len(s.Items) - 1
	out = s.Render(26, 12)
	if !strings.Contains(out, "▲") {
		t.Errorf("expected a 'more above' indicator when cursor is at the bottom, got:\n%s", out)
	}
}
