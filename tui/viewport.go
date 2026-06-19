package tui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"kblog/k8s"

	"github.com/charmbracelet/lipgloss"
)

type fieldFilter struct {
	key   string
	value string
}

// Viewport handles log buffers, filtering, searching, and rendering
type Viewport struct {
	AllLines      []k8s.LogLine // Global log buffer cache
	FilteredLines []k8s.LogLine // Filtered subset currently displayed
	ScrollY       int           // Top line index displayed on screen
	CursorY       int           // Interactive selected line index
	AutoScroll    bool          // Pin viewport to the bottom
	SearchQuery   string        // Search filter query
	FilterLevel   string        // "ALL", "INFO", "WARN", "ERROR", "DEBUG"
	Height        int
	Width         int
	Focused       bool

	// New feature state
	WrapLines       bool // toggle word-wrap on long lines
	SortDescending  bool // toggle newest-first sort
	SelectionStart  int  // anchor line for multiline copy
	SelectionActive bool // whether visual selection mode is on

	// Stern-style pod color mappings
	podColors   map[string]lipgloss.Color
	colorIndex  int
	colorScheme []lipgloss.Color

	// Parsed filter state (rebuilt by UpdateFilters / parseSearchQuery)
	searchRegex  *regexp.Regexp
	fieldFilters []fieldFilter
	textRegex    *regexp.Regexp
}

// NewViewport initializes a high-performance log viewport
func NewViewport() *Viewport {
	colorScheme := []lipgloss.Color{
		lipgloss.Color("#FF5733"),
		lipgloss.Color("#33FF57"),
		lipgloss.Color("#3357FF"),
		lipgloss.Color("#FF33A1"),
		lipgloss.Color("#FF8F33"),
		lipgloss.Color("#33FFF0"),
		lipgloss.Color("#D133FF"),
		lipgloss.Color("#FFD433"),
		lipgloss.Color("#85FF33"),
	}

	return &Viewport{
		AllLines:      make([]k8s.LogLine, 0, 50000),
		FilteredLines: []k8s.LogLine{},
		ScrollY:       0,
		CursorY:       0,
		AutoScroll:    true,
		SearchQuery:   "",
		FilterLevel:   "ALL",
		Focused:       true,
		podColors:     make(map[string]lipgloss.Color),
		colorIndex:    0,
		colorScheme:   colorScheme,
	}
}

// AddLine inserts a new log line into the global buffer
func (v *Viewport) AddLine(line k8s.LogLine) {
	v.AllLines = append(v.AllLines, line)
	if len(v.AllLines) > 50000 {
		v.AllLines = v.AllLines[10000:]
	}
}

// passesFilters evaluates whether a line should appear in FilteredLines given current state
func (v *Viewport) passesFilters(line k8s.LogLine, selectedContainers map[string]bool) bool {
	// 1. Container filter
	if len(selectedContainers) > 0 && line.Container != "" {
		if !selectedContainers[line.Container] {
			return false
		}
	}

	// 2. Log level filter
	if v.FilterLevel != "ALL" && v.FilterLevel != "DEBUG" {
		if line.IsEvent {
			if v.FilterLevel == "ERROR" && line.Level != "ERROR" {
				return false
			}
			if v.FilterLevel == "WARN" && line.Level != "WARN" && line.Level != "ERROR" {
				return false
			}
		} else {
			if v.FilterLevel == "ERROR" && line.Level != "ERROR" {
				return false
			}
			if v.FilterLevel == "WARN" && line.Level != "WARN" && line.Level != "ERROR" {
				return false
			}
			if v.FilterLevel == "INFO" && line.Level == "DEBUG" {
				return false
			}
		}
	}

	// 3. Structured field filters (key=value from JSON lines)
	for _, f := range v.fieldFilters {
		if line.Fields == nil {
			return false
		}
		val, ok := line.Fields[f.key]
		if !ok || !strings.Contains(strings.ToLower(val), strings.ToLower(f.value)) {
			return false
		}
	}

	// 4. Free-text regex (may coexist with field filters)
	if v.textRegex != nil {
		if !v.textRegex.MatchString(line.Content) && !v.textRegex.MatchString(line.Pod) && !v.textRegex.MatchString(line.Container) {
			return false
		}
	}

	return true
}

// AddLineWithFilter inserts a new log line and, if it passes current filters, adds it to FilteredLines
func (v *Viewport) AddLineWithFilter(line k8s.LogLine, selectedContainers map[string]bool) {
	v.AddLine(line)

	if !v.passesFilters(line, selectedContainers) {
		return
	}

	if v.SortDescending {
		// Newest lines go to the front in descending mode
		v.FilteredLines = append([]k8s.LogLine{line}, v.FilteredLines...)
	} else {
		v.FilteredLines = append(v.FilteredLines, line)
	}

	if len(v.FilteredLines) > 50000 {
		v.FilteredLines = v.FilteredLines[10000:]
	}

	if v.AutoScroll {
		v.ScrollToBottom()
	} else {
		v.ClampScroll()
	}
}

// fieldFilterRe matches tokens like "level=error" or "request_id=abc-123"
var fieldFilterRe = regexp.MustCompile(`^[\w][\w.\-]*=\S+$`)

// parseSearchQuery splits a search string into structured field filters and a free-text regex.
// Tokens matching key=value are field filters; the rest become a regex.
func parseSearchQuery(query string) ([]fieldFilter, *regexp.Regexp) {
	if query == "" {
		return nil, nil
	}
	var filters []fieldFilter
	var textParts []string
	for _, token := range strings.Fields(query) {
		if fieldFilterRe.MatchString(token) {
			idx := strings.Index(token, "=")
			filters = append(filters, fieldFilter{key: token[:idx], value: token[idx+1:]})
		} else {
			textParts = append(textParts, regexp.QuoteMeta(token))
		}
	}
	var re *regexp.Regexp
	if len(textParts) > 0 {
		re, _ = regexp.Compile("(?i)" + strings.Join(textParts, "|"))
	}
	return filters, re
}

// UpdateFilters re-applies all active filters and sort order to the full AllLines buffer
func (v *Viewport) UpdateFilters(selectedContainers map[string]bool) {
	v.fieldFilters, v.textRegex = parseSearchQuery(v.SearchQuery)
	// Keep searchRegex pointing to textRegex for formatLogLine highlighting
	v.searchRegex = v.textRegex

	var filtered []k8s.LogLine
	for _, line := range v.AllLines {
		if v.passesFilters(line, selectedContainers) {
			filtered = append(filtered, line)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if v.SortDescending {
			return filtered[i].Timestamp.After(filtered[j].Timestamp)
		}
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	v.FilteredLines = filtered

	if v.AutoScroll {
		v.ScrollToBottom()
	} else {
		v.ClampScroll()
	}
}

// GetPodColor gets or registers a stable color for a pod prefix
func (v *Viewport) GetPodColor(pod string) lipgloss.Color {
	if color, exists := v.podColors[pod]; exists {
		return color
	}
	color := v.colorScheme[v.colorIndex%len(v.colorScheme)]
	v.podColors[pod] = color
	v.colorIndex++
	return color
}

// ClampScroll ensures scroll index stays in valid range
func (v *Viewport) ClampScroll() {
	maxScroll := len(v.FilteredLines) - v.Height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.ScrollY > maxScroll {
		v.ScrollY = maxScroll
	}
	if v.ScrollY < 0 {
		v.ScrollY = 0
	}
	if v.CursorY >= len(v.FilteredLines) {
		v.CursorY = len(v.FilteredLines) - 1
	}
	if v.CursorY < 0 {
		v.CursorY = 0
	}
}

// ScrollToBottom pushes viewport view to the latest logs
func (v *Viewport) ScrollToBottom() {
	v.AutoScroll = true
	maxScroll := len(v.FilteredLines) - v.Height
	if maxScroll < 0 {
		maxScroll = 0
	}
	v.ScrollY = maxScroll
	v.CursorY = len(v.FilteredLines) - 1
}

// MoveUp scrolls logs up
func (v *Viewport) MoveUp() {
	v.AutoScroll = false
	if v.CursorY > 0 {
		v.CursorY--
	}
	if v.CursorY < v.ScrollY {
		v.ScrollY = v.CursorY
	}
	v.ClampScroll()
}

// MoveDown scrolls logs down
func (v *Viewport) MoveDown() {
	if v.CursorY < len(v.FilteredLines)-1 {
		v.CursorY++
		v.AutoScroll = (v.CursorY == len(v.FilteredLines)-1)
	}
	if v.CursorY >= v.ScrollY+v.Height {
		v.ScrollY = v.CursorY - v.Height + 1
	}
	v.ClampScroll()
}

// PageUp scrolls full page up
func (v *Viewport) PageUp() {
	v.AutoScroll = false
	v.ScrollY -= v.Height - 2
	v.CursorY -= v.Height - 2
	v.ClampScroll()
}

// PageDown scrolls full page down
func (v *Viewport) PageDown() {
	v.ScrollY += v.Height - 2
	v.CursorY += v.Height - 2
	if v.ScrollY+v.Height >= len(v.FilteredLines) {
		v.AutoScroll = true
	}
	v.ClampScroll()
}

// GetSelectedLine returns the log line currently under the cursor
func (v *Viewport) GetSelectedLine() (k8s.LogLine, bool) {
	if len(v.FilteredLines) == 0 || v.CursorY < 0 || v.CursorY >= len(v.FilteredLines) {
		return k8s.LogLine{}, false
	}
	return v.FilteredLines[v.CursorY], true
}

// GetSelectionLines returns all lines in the active visual selection range.
// Returns (nil, false) when no selection is active.
func (v *Viewport) GetSelectionLines() ([]k8s.LogLine, bool) {
	if !v.SelectionActive || len(v.FilteredLines) == 0 {
		return nil, false
	}
	lo, hi := v.SelectionStart, v.CursorY
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 {
		lo = 0
	}
	if hi >= len(v.FilteredLines) {
		hi = len(v.FilteredLines) - 1
	}
	return v.FilteredLines[lo : hi+1], true
}

// ClearSelection cancels visual selection mode
func (v *Viewport) ClearSelection() {
	v.SelectionActive = false
	v.SelectionStart = 0
}

// wrapString splits a plain-text string into chunks of at most width runes,
// preferring to break at spaces for readability.
func wrapString(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) <= width {
		return []string{s}
	}

	var lines []string
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		breakAt := width
		// Walk left to find a space for a nicer break
		for breakAt > width/2 && runes[breakAt-1] != ' ' {
			breakAt--
		}
		// No space found — hard break at width
		if breakAt <= width/2 {
			breakAt = width
		}
		lines = append(lines, strings.TrimRight(string(runes[:breakAt]), " "))
		// Skip the space at the break point if present
		if breakAt < len(runes) && runes[breakAt] == ' ' {
			runes = runes[breakAt+1:]
		} else {
			runes = runes[breakAt:]
		}
	}
	return lines
}

// applyHighlight applies search highlighting to a plain-text string
func applyHighlight(content string, re *regexp.Regexp) string {
	if re == nil {
		return content
	}
	return re.ReplaceAllStringFunc(content, func(match string) string {
		return lipgloss.NewStyle().Background(PrimaryColor).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Render(match)
	})
}

// Render compiles the log screen display
func (v *Viewport) Render(width int, height int) string {
	v.Width = width
	v.Height = height

	box := ViewportStyle.
		Width(width - 4).
		Height(height - 2)

	if v.Focused {
		box = box.BorderForeground(ActiveBorder)
	} else {
		box = box.BorderForeground(BorderColor)
	}

	if len(v.FilteredLines) == 0 {
		return box.Render("\n\n   Waiting for logs... (Press Shift-L to toggle container filter sidebar)")
	}

	// Compute selection range for rendering
	selLo, selHi := -1, -1
	if v.SelectionActive {
		selLo, selHi = v.SelectionStart, v.CursorY
		if selLo > selHi {
			selLo, selHi = selHi, selLo
		}
	}

	var sb strings.Builder
	physicalLines := 0
	maxPhysical := height - 2

	for i := v.ScrollY; i < len(v.FilteredLines) && physicalLines < maxPhysical; i++ {
		line := v.FilteredLines[i]
		isHighlighted := (i == v.CursorY)
		inSelection := v.SelectionActive && i >= selLo && i <= selHi && !isHighlighted

		rendered := v.formatLogLine(line, width-6, isHighlighted, inSelection)
		lineCount := strings.Count(rendered, "\n") + 1

		// Truncate last wrapped line if it would overflow the viewport box
		if physicalLines+lineCount > maxPhysical {
			parts := strings.Split(rendered, "\n")
			remaining := maxPhysical - physicalLines
			for pi := 0; pi < remaining && pi < len(parts); pi++ {
				sb.WriteString(parts[pi])
				if pi < remaining-1 {
					sb.WriteString("\n")
				}
			}
			break
		}

		sb.WriteString(rendered)
		sb.WriteString("\n")
		physicalLines += lineCount
	}

	return box.Render(sb.String())
}

// formatLogLine converts a raw log model into a colored terminal string.
// Returns a string that may contain embedded newlines when WrapLines is true.
func (v *Viewport) formatLogLine(line k8s.LogLine, maxLen int, isSelected bool, inSelection bool) string {
	// 1. Timestamp
	timeStr := line.Timestamp.Format("15:04:05")
	styledTime := LogTimeStyle.Render(timeStr)

	// 2. Pod prefix
	podColor := v.GetPodColor(line.Pod)
	podStyle := LogPodStyle.Foreground(podColor)
	styledPod := podStyle.Render(fmt.Sprintf("[%s]", line.Pod))

	// 3. Level badge
	var levelBadge string
	switch line.Level {
	case "ERROR":
		levelBadge = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render("ERR")
	case "WARN":
		levelBadge = lipgloss.NewStyle().Foreground(WarnColor).Bold(true).Render("WRN")
	case "DEBUG":
		levelBadge = lipgloss.NewStyle().Foreground(DebugColor).Render("DBG")
	default:
		levelBadge = lipgloss.NewStyle().Foreground(InfoColor).Render("INF")
	}

	prefix := fmt.Sprintf("%s %s %s | ", styledTime, styledPod, levelBadge)
	prefixWidth := lipgloss.Width(prefix)

	// 4. Content rendering
	if line.IsEvent {
		eventStyle := LogEventBanner
		switch line.Level {
		case "ERROR":
			eventStyle = eventStyle.Background(lipgloss.Color("#3A1B1F")).Foreground(ErrorColor)
		case "WARN":
			eventStyle = eventStyle.Background(lipgloss.Color("#352A15")).Foreground(WarnColor)
		}
		lineText := prefix + eventStyle.Render(line.Content)
		return v.applyLineStyle(lineText, maxLen, isSelected, inSelection)
	}

	// Determine display content (extract JSON msg field for inline display)
	displayContent := line.Content
	rawContent := line.Content
	trimmed := strings.TrimSpace(rawContent)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			if msg, ok := parsed["msg"].(string); ok {
				displayContent = fmt.Sprintf("%s  %s", lipgloss.NewStyle().Foreground(SecondaryColor).Render("⚙"), msg)
				rawContent = msg
			} else if message, ok := parsed["message"].(string); ok {
				displayContent = fmt.Sprintf("%s  %s", lipgloss.NewStyle().Foreground(SecondaryColor).Render("⚙"), message)
				rawContent = message
			}
		}
	}

	if v.WrapLines {
		contentWidth := maxLen - prefixWidth
		if contentWidth < 20 {
			contentWidth = 20
		}
		chunks := wrapString(rawContent, contentWidth)
		var sb strings.Builder
		for i, chunk := range chunks {
			highlighted := applyHighlight(chunk, v.searchRegex)
			if i == 0 {
				lineText := prefix + highlighted
				sb.WriteString(v.applyLineStyle(lineText, maxLen, isSelected, inSelection))
			} else {
				indent := strings.Repeat(" ", prefixWidth)
				sb.WriteString("\n" + indent + highlighted)
			}
		}
		return sb.String()
	}

	// Non-wrap path: apply highlighting to the display content
	if v.searchRegex != nil {
		displayContent = applyHighlight(displayContent, v.searchRegex)
	}

	lineText := prefix + displayContent
	return v.applyLineStyle(lineText, maxLen, isSelected, inSelection)
}

// applyLineStyle applies cursor / selection / normal styling to a fully-composed line string
func (v *Viewport) applyLineStyle(lineText string, maxLen int, isSelected bool, inSelection bool) string {
	if isSelected {
		prefix := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render("➔ ")
		selectedLine := lipgloss.NewStyle().
			Background(lipgloss.Color("#1C1C24")).
			Width(maxLen - 2).
			Render(lineText)
		return prefix + selectedLine
	}
	if inSelection {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#2A2A3A")).
			Width(maxLen).
			Render("  " + lineText)
	}
	return "  " + lineText
}
