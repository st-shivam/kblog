package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// JSONInspector manages rendering of formatted log data
type JSONInspector struct {
	RawContent string
	ParsedJSON map[string]interface{}
	IsJSON     bool
	ScrollY    int
	Height     int
	TotalLines int // total rendered lines of the current content (set by Render)
}

// NewJSONInspector initializes an empty inspector
func NewJSONInspector() *JSONInspector {
	return &JSONInspector{
		ScrollY: 0,
		Height:  12,
	}
}

// SetContent updates content, checking if it is JSON
func (j *JSONInspector) SetContent(content string) {
	j.RawContent = content
	j.ScrollY = 0

	var parsed map[string]interface{}
	// Trim spaces and try unmarshalling
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		if err := json.Unmarshal([]byte(content), &parsed); err == nil {
			j.ParsedJSON = parsed
			j.IsJSON = true
			return
		}
	}

	j.IsJSON = false
	j.ParsedJSON = nil
}

// ScrollUp scrolls the inspector content
func (j *JSONInspector) ScrollUp() {
	if j.ScrollY > 0 {
		j.ScrollY--
	}
}

// ScrollDown scrolls the inspector content down, bounded by the actual rendered
// content length (TotalLines) so the bottom is reachable and we never scroll
// into blank space below the content.
func (j *JSONInspector) ScrollDown() {
	maxScroll := j.maxScroll()
	if j.ScrollY < maxScroll {
		j.ScrollY++
	}
}

// maxScroll is the largest valid ScrollY for the current content/height.
func (j *JSONInspector) maxScroll() int {
	max := j.TotalLines - j.Height
	if max < 0 {
		max = 0
	}
	return max
}

// Render returns the styled string representation of the inspector modal
func (j *JSONInspector) Render(width int, height int) string {
	j.Height = height - 4

	modal := ModalStyle.Copy().
		Width(width - 8).
		Height(height - 4)

	var renderedContent string
	var totalLines int

	if j.IsJSON {
		renderedContent, totalLines = j.formatJSONTree(width - 12)
	} else {
		renderedContent, totalLines = j.formatRawText(width - 12)
	}

	// Record content length and clamp scroll so the keybinding handler can bound
	// against the real content rather than a hardcoded value.
	j.TotalLines = totalLines
	if j.ScrollY > j.maxScroll() {
		j.ScrollY = j.maxScroll()
	}

	// Dynamic scroll slicing
	lines := strings.Split(renderedContent, "\n")
	if len(lines) > j.Height {
		start := j.ScrollY
		end := start + j.Height
		if end > len(lines) {
			end = len(lines)
		}
		renderedContent = strings.Join(lines[start:end], "\n")
	}

	// Add top header / control tip
	headerText := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render("⚙ Structured Log Inspector")
	closeTip := HelpDescStyle.Render(" (Esc / Enter to close | j/k or ↑/↓ to scroll)")
	title := fmt.Sprintf("%s%s\n\n", headerText, closeTip)

	// Add scrollbar status indicator
	scrollIndicator := ""
	if totalLines > j.Height {
		scrollIndicator = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Render(fmt.Sprintf("\n\n-- Page %d/%d (Use j/k or ↑/↓ to scroll) --", j.ScrollY+1, totalLines-j.Height+1))
	}

	return modal.Render(title + renderedContent + scrollIndicator)
}

// formatRawText handles wrapping and formatting non-JSON logs
func (j *JSONInspector) formatRawText(maxLen int) (string, int) {
	words := strings.Fields(j.RawContent)
	if len(words) == 0 {
		return j.RawContent, 1
	}

	var sb strings.Builder
	var lineLen int
	var totalLines int = 1

	for _, word := range words {
		if lineLen+len(word)+1 > maxLen {
			sb.WriteString("\n")
			lineLen = 0
			totalLines++
		}
		sb.WriteString(word)
		sb.WriteString(" ")
		lineLen += len(word) + 1
	}

	return LogContentStyle.Render(sb.String()), totalLines
}

// formatJSONTree renders a beautifully colorized key-value syntax view
func (j *JSONInspector) formatJSONTree(maxLen int) (string, int) {
	// Custom colorful syntax highlighter for JSON keys and values
	var sb strings.Builder
	totalLines := 0

	// Sort keys to maintain a stable sorted output
	keys := make([]string, 0, len(j.ParsedJSON))
	for k := range j.ParsedJSON {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render("{"))
	sb.WriteString("\n")
	totalLines++

	for i, key := range keys {
		val := j.ParsedJSON[key]

		// Format Key
		styledKey := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render(fmt.Sprintf("  \"%s\"", key))

		// Format Value based on type
		var styledVal string
		switch v := val.(type) {
		case string:
			// Pretty format nested string JSONs if they exist
			if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
				styledVal = lipgloss.NewStyle().Foreground(SecondaryColor).Render("{...}")
			} else {
				styledVal = lipgloss.NewStyle().Foreground(InfoColor).Render(fmt.Sprintf("\"%v\"", v))
			}
		case float64:
			styledVal = lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("%v", v))
		case bool:
			styledVal = lipgloss.NewStyle().Foreground(AccentColor).Bold(true).Render(fmt.Sprintf("%t", v))
		case nil:
			styledVal = lipgloss.NewStyle().Foreground(DebugColor).Render("null")
		default:
			// Try pretty printing arrays / nested maps
			marshalled, _ := json.Marshal(v)
			// Compact representation if short
			if len(marshalled) < 40 {
				styledVal = lipgloss.NewStyle().Foreground(SecondaryColor).Render(string(marshalled))
			} else {
				// indent multiline
				var indented bytes.Buffer
				if err := json.Indent(&indented, marshalled, "    ", "  "); err == nil {
					styledVal = lipgloss.NewStyle().Foreground(SecondaryColor).Render(indented.String())
				} else {
					styledVal = lipgloss.NewStyle().Foreground(SecondaryColor).Render(string(marshalled))
				}
			}
		}

		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}

		line := fmt.Sprintf("%s: %s%s", styledKey, styledVal, comma)

		// Split on explicit newlines within value (for indented objects) and wrap accordingly
		subLines := strings.Split(line, "\n")
		for _, sl := range subLines {
			sb.WriteString(sl)
			sb.WriteString("\n")
			totalLines++
		}
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render("}"))
	totalLines++

	return sb.String(), totalLines
}
