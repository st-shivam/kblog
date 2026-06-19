package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Sidebar Model
type Sidebar struct {
	Items    []string
	Selected map[string]bool
	Cursor   int
	Focused  bool
	Height   int
}

// NewSidebar initializes a sidebar component
func NewSidebar() *Sidebar {
	return &Sidebar{
		Items:    []string{},
		Selected: make(map[string]bool),
		Cursor:   0,
		Focused:  false,
		Height:   10,
	}
}

// AddItem adds an item to the list if it doesn't already exist
func (s *Sidebar) AddItem(item string) {
	if item == "" {
		return
	}
	for _, it := range s.Items {
		if it == item {
			return
		}
	}
	s.Items = append(s.Items, item)
	s.Selected[item] = true // Default to enabled
}

// ToggleSelected toggles the currently highlighted container/pod
func (s *Sidebar) ToggleSelected() {
	if len(s.Items) == 0 {
		return
	}
	item := s.Items[s.Cursor]
	s.Selected[item] = !s.Selected[item]
}

// CursorUp moves selection cursor up
func (s *Sidebar) CursorUp() {
	if s.Cursor > 0 {
		s.Cursor--
	}
}

// CursorDown moves selection cursor down
func (s *Sidebar) CursorDown() {
	if s.Cursor < len(s.Items)-1 {
		s.Cursor++
	}
}

// Render returns the string representation of the sidebar
func (s *Sidebar) Render(width int, height int) string {
	s.Height = height

	// Create sidebar container style with dynamic height
	sidebarContainer := SidebarStyle.
		Width(width - 4).
		Height(height - 2)

	if s.Focused {
		sidebarContainer = sidebarContainer.BorderForeground(ActiveBorder)
	} else {
		sidebarContainer = sidebarContainer.BorderForeground(BorderColor)
	}

	var sb strings.Builder
	sb.WriteString(SidebarTitle.Render("Containers"))
	sb.WriteString("\n\n")

	if len(s.Items) == 0 {
		sb.WriteString(HelpDescStyle.Render("No containers found"))
		return sidebarContainer.Render(sb.String())
	}

	for i, item := range s.Items {
		// Shorten long container names to fit
		displayName := item
		if len(displayName) > width-10 {
			displayName = displayName[:width-12] + ".."
		}

		checkbox := "[ ]"
		if s.Selected[item] {
			checkbox = "[x]"
		}

		lineContent := fmt.Sprintf("%s %s", checkbox, displayName)

		var line string
		if s.Cursor == i && s.Focused {
			// Highlight current cursor item
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(PrimaryColor).
				Bold(true).
				Render(lineContent)
		} else {
			if s.Selected[item] {
				line = ContainerActive.Render(lineContent)
			} else {
				line = ContainerInactive.Render(lineContent)
			}
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sidebarContainer.Render(sb.String())
}
