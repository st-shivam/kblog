package tui

import "testing"

func TestJSONInspector_ScrollBoundedByContent(t *testing.T) {
	j := NewJSONInspector()
	j.Height = 10
	j.TotalLines = 25 // maxScroll = 25 - 10 = 15

	for i := 0; i < 100; i++ {
		j.ScrollDown()
	}
	if j.ScrollY != 15 {
		t.Errorf("ScrollY should cap at maxScroll=15, got %d", j.ScrollY)
	}

	for i := 0; i < 100; i++ {
		j.ScrollUp()
	}
	if j.ScrollY != 0 {
		t.Errorf("ScrollY should floor at 0, got %d", j.ScrollY)
	}
}

func TestJSONInspector_ShortContentDoesNotScroll(t *testing.T) {
	j := NewJSONInspector()
	j.Height = 10
	j.TotalLines = 5 // content shorter than the viewport: maxScroll = 0

	j.ScrollDown()
	if j.ScrollY != 0 {
		t.Errorf("short content should not scroll into blank space, got ScrollY=%d", j.ScrollY)
	}
}

func TestJSONInspector_RenderSetsTotalLinesAndClampsScroll(t *testing.T) {
	InitStyles(Themes[0])
	j := NewJSONInspector()
	j.SetContent(`{"a":"1","b":"2","c":"3"}`)
	if !j.IsJSON {
		t.Fatal("SetContent should detect JSON object")
	}

	// An over-large scroll position must be clamped by Render.
	j.ScrollY = 9999
	_ = j.Render(80, 20)

	if j.TotalLines == 0 {
		t.Error("Render should record TotalLines for the content")
	}
	if j.ScrollY > j.maxScroll() {
		t.Errorf("Render should clamp ScrollY (%d) to maxScroll (%d)", j.ScrollY, j.maxScroll())
	}
}

func TestJSONInspector_SetContentResetsScroll(t *testing.T) {
	j := NewJSONInspector()
	j.ScrollY = 5
	j.SetContent("plain text, not json")
	if j.ScrollY != 0 {
		t.Errorf("SetContent should reset ScrollY to 0, got %d", j.ScrollY)
	}
	if j.IsJSON {
		t.Error("plain text should not be detected as JSON")
	}
}
