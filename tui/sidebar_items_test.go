package tui

import "testing"

func TestSidebar_AddItem_Dedup(t *testing.T) {
	s := NewSidebar()
	s.AddItem("auth")
	s.AddItem("auth") // duplicate
	s.AddItem("proxy")
	if len(s.Items) != 2 {
		t.Fatalf("expected 2 unique items, got %d: %v", len(s.Items), s.Items)
	}
	// Newly-added items default to selected (enabled).
	if !s.Selected["auth"] || !s.Selected["proxy"] {
		t.Error("new items should default to selected")
	}
}

func TestSidebar_AddItem_IgnoresEmpty(t *testing.T) {
	s := NewSidebar()
	s.AddItem("")
	if len(s.Items) != 0 {
		t.Errorf("empty item should be ignored, got %v", s.Items)
	}
}

func TestSidebar_ToggleSelected(t *testing.T) {
	s := NewSidebar()
	s.AddItem("a")
	s.AddItem("b")
	s.Cursor = 1 // "b"

	s.ToggleSelected()
	if s.Selected["b"] {
		t.Error("ToggleSelected should turn 'b' off")
	}
	if !s.Selected["a"] {
		t.Error("toggling 'b' must not affect 'a'")
	}

	s.ToggleSelected()
	if !s.Selected["b"] {
		t.Error("second toggle should turn 'b' back on")
	}
}

func TestSidebar_ToggleSelected_NoItemsIsSafe(t *testing.T) {
	s := NewSidebar()
	// Must not panic with an empty list.
	s.ToggleSelected()
}
