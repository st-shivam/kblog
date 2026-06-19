package tui

import (
	"testing"

	"kblog/k8s"
)

func viewportWithLines(n, height int) *Viewport {
	v := NewViewport()
	v.Height = height
	lines := make([]k8s.LogLine, n)
	v.FilteredLines = lines
	v.AllLines = lines
	return v
}

func TestViewport_ScrollToBottom(t *testing.T) {
	v := viewportWithLines(100, 10)
	v.ScrollToBottom()
	if !v.AutoScroll {
		t.Error("ScrollToBottom should enable AutoScroll")
	}
	if v.CursorY != 99 {
		t.Errorf("CursorY = %d, want 99 (last line)", v.CursorY)
	}
	if v.ScrollY != 90 { // 100 - height(10)
		t.Errorf("ScrollY = %d, want 90", v.ScrollY)
	}
}

func TestViewport_MoveUpDown(t *testing.T) {
	v := viewportWithLines(50, 10)
	v.ScrollToBottom() // CursorY=49
	v.MoveUp()
	if v.AutoScroll {
		t.Error("MoveUp should disable AutoScroll")
	}
	if v.CursorY != 48 {
		t.Errorf("after MoveUp CursorY = %d, want 48", v.CursorY)
	}
	v.MoveDown()
	if v.CursorY != 49 {
		t.Errorf("after MoveDown CursorY = %d, want 49", v.CursorY)
	}
	if !v.AutoScroll {
		t.Error("MoveDown onto the last line should re-enable AutoScroll")
	}
}

func TestViewport_MoveUp_StopsAtTop(t *testing.T) {
	v := viewportWithLines(5, 10)
	v.CursorY = 0
	v.MoveUp()
	if v.CursorY != 0 {
		t.Errorf("MoveUp at top should stay at 0, got %d", v.CursorY)
	}
}

func TestViewport_PageUpDown(t *testing.T) {
	v := viewportWithLines(100, 10)
	v.ScrollToBottom()
	startCursor := v.CursorY
	v.PageUp()
	if v.CursorY >= startCursor {
		t.Errorf("PageUp should move the cursor up, got %d (was %d)", v.CursorY, startCursor)
	}
	if v.AutoScroll {
		t.Error("PageUp should disable AutoScroll")
	}
	up := v.CursorY
	v.PageDown()
	if v.CursorY <= up {
		t.Errorf("PageDown should move the cursor down, got %d (was %d)", v.CursorY, up)
	}
}

func TestViewport_ClampScroll(t *testing.T) {
	v := viewportWithLines(5, 10)
	// Out-of-range values must be clamped back into bounds.
	v.ScrollY = 999
	v.CursorY = 999
	v.ClampScroll()
	if v.ScrollY != 0 { // fewer lines than height => maxScroll 0
		t.Errorf("ScrollY = %d, want 0", v.ScrollY)
	}
	if v.CursorY != 4 {
		t.Errorf("CursorY = %d, want 4 (last index)", v.CursorY)
	}

	v.ScrollY = -5
	v.CursorY = -5
	v.ClampScroll()
	if v.ScrollY != 0 || v.CursorY != 0 {
		t.Errorf("negative values should clamp to 0, got ScrollY=%d CursorY=%d", v.ScrollY, v.CursorY)
	}
}
