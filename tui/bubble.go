package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"kblog/k8s"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// logMsg wraps incoming stream log lines for the Elm loop
type logMsg k8s.LogLine

// streamClosedMsg indicates the log channel was shut down
type streamClosedMsg struct{}

// Model holds the entire TUI application state
type Model struct {
	// K8s integration
	namespace   string
	podName     string
	deployment  string
	contextName string
	version     string
	logChan     <-chan k8s.LogLine
	streamer    *k8s.LogStreamer
	watcher     *k8s.EventWatcher
	cancelFn    context.CancelFunc

	// UI layout & sizing
	width  int
	height int

	// TUI Components
	viewport      *Viewport
	sidebar       *Sidebar
	jsonInspector *JSONInspector
	searchInput   textinput.Model

	// State toggles
	showSidebar bool
	showModal   bool
	searching   bool
	statusMsg   string
	statusTime  time.Time
	themeIdx    int
}

// NewModel initializes the main Bubbletea controller
func NewModel(
	ctxName string,
	ns string,
	pod string,
	deploy string,
	version string,
	ch <-chan k8s.LogLine,
	streamer *k8s.LogStreamer,
	watcher *k8s.EventWatcher,
	cancel context.CancelFunc,
) *Model {
	ti := textinput.New()
	ti.Placeholder = "Type search term... (Supports instant regex)"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40
	ti.Prompt = "🔍 / "

	return &Model{
		namespace:     ns,
		podName:       pod,
		deployment:    deploy,
		contextName:   ctxName,
		version:       version,
		logChan:       ch,
		streamer:      streamer,
		watcher:       watcher,
		cancelFn:      cancel,
		viewport:      NewViewport(),
		sidebar:       NewSidebar(),
		jsonInspector: NewJSONInspector(),
		searchInput:   ti,
		showSidebar:   false,
		showModal:     false,
		searching:     false,
		themeIdx:      0,
	}
}

// Init starts the Elm update stream receiver
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.waitForLogs(),
	)
}

// waitForLogs listens asynchronously to the Go channels
func (m *Model) waitForLogs() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-m.logChan
		if !ok {
			return streamClosedMsg{}
		}
		return logMsg(line)
	}
}

// copyToClipboard uses platform native tools to copy log details
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard")
		}
	default:
		return fmt.Errorf("unsupported platform")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write([]byte(text)); err != nil {
		return err
	}
	_ = stdin.Close()

	return cmd.Wait()
}

// Update processes incoming keystrokes and background messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		return m, nil

	case streamClosedMsg:
		// Sticky indicator: the merged stream has closed, no more logs will arrive.
		m.viewport.StreamError = "Log stream disconnected. No further logs will arrive."
		return m, nil

	case logMsg:
		// A line arrived: the stream is live, clear any sticky disconnect notice.
		m.viewport.StreamError = ""

		// 1. Ingest new log line with fast inline filtering
		m.viewport.AddLineWithFilter(k8s.LogLine(msg), m.sidebar.Selected)

		// 2. Discover new containers/pods dynamically for sidebar lists
		if msg.Container != "" {
			m.sidebar.AddItem(msg.Container)
		}

		// 3. Trigger next stream read batch
		return m, m.waitForLogs()

	case tea.KeyMsg:
		// Active alert notification duration limit (3 seconds)
		if m.statusMsg != "" && time.Since(m.statusTime) > 3*time.Second {
			m.statusMsg = ""
		}

		// Keystrokes while JSON Modal is open
		if m.showModal {
			switch msg.String() {
			case "esc", "enter", "q":
				m.showModal = false
			case "ctrl+c":
				m.Shutdown()
				return m, tea.Quit
			case "up", "k":
				m.jsonInspector.ScrollUp()
			case "down", "j":
				m.jsonInspector.ScrollDown()
			}
			return m, nil
		}

		// Keystrokes while in text search mode
		if m.searching {
			switch msg.String() {
			case "enter":
				m.searching = false
				m.viewport.SearchQuery = m.searchInput.Value()
				m.viewport.UpdateFilters(m.sidebar.Selected)
				m.statusMsg = fmt.Sprintf("🔍 Filtering query: '%s'", m.viewport.SearchQuery)
				m.statusTime = time.Now()
			case "esc":
				m.searching = false
				m.searchInput.SetValue("")
				m.viewport.SearchQuery = ""
				m.viewport.UpdateFilters(m.sidebar.Selected)
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				// Live-filtering search as they type!
				m.viewport.SearchQuery = m.searchInput.Value()
				m.viewport.UpdateFilters(m.sidebar.Selected)
				return m, cmd
			}
			return m, nil
		}

		// General navigation & state toggles
		switch msg.String() {
		case "ctrl+c":
			m.Shutdown()
			return m, tea.Quit

		case "q":
			m.Shutdown()
			return m, tea.Quit

		case "esc":
			// Cancel visual selection if active, otherwise quit
			if m.viewport.SelectionActive {
				m.viewport.ClearSelection()
				m.statusMsg = "Selection cancelled"
				m.statusTime = time.Now()
			} else {
				m.Shutdown()
				return m, tea.Quit
			}

		// Focus toggling between log viewport and sidebar container filters
		case "tab":
			if m.showSidebar {
				m.viewport.Focused = !m.viewport.Focused
				m.sidebar.Focused = !m.sidebar.Focused
			} else {
				m.viewport.Focused = true
				m.sidebar.Focused = false
			}

		// Instantly toggle container sidebar open/close
		case "L", "l":
			m.showSidebar = !m.showSidebar
			if m.showSidebar {
				m.sidebar.Focused = true
				m.viewport.Focused = false
			} else {
				m.sidebar.Focused = false
				m.viewport.Focused = true
			}

		// Start search mode (supports plain text, regex, and key=value field filters)
		case "/":
			m.searching = true
			m.searchInput.Focus()
			m.searchInput.SetValue("")
			m.viewport.AutoScroll = false
			return m, textinput.Blink

		// Pretty-print JSON details modal
		case "enter":
			if line, ok := m.viewport.GetSelectedLine(); ok {
				m.jsonInspector.SetContent(line.Content)
				m.showModal = true
			}

		// Toggle Auto-Scroll / follow mode
		case "f", "F":
			m.viewport.ScrollToBottom()
			m.statusMsg = "🔒 Auto-Scroll locked to bottom"
			m.statusTime = time.Now()

		// Copy: single line or multiline selection
		case "c", "y":
			if lines, ok := m.viewport.GetSelectionLines(); ok {
				// Multiline selection active
				parts := make([]string, len(lines))
				for i, l := range lines {
					parts[i] = l.Content
				}
				copyText := strings.Join(parts, "\n")
				if err := copyToClipboard(copyText); err == nil {
					m.statusMsg = fmt.Sprintf("📋 %d lines copied to clipboard!", len(lines))
				} else {
					m.statusMsg = fmt.Sprintf("❌ Copy failed: %v", err)
				}
				m.viewport.ClearSelection()
			} else if line, ok := m.viewport.GetSelectedLine(); ok {
				if err := copyToClipboard(line.Content); err == nil {
					m.statusMsg = "📋 Line copied to clipboard!"
				} else {
					m.statusMsg = fmt.Sprintf("❌ Copy failed: %v", err)
				}
			}
			m.statusTime = time.Now()

		// Visual multiline selection
		case "v":
			if m.viewport.SelectionActive {
				m.viewport.ClearSelection()
				m.statusMsg = "Selection cancelled"
			} else {
				m.viewport.SelectionActive = true
				m.viewport.SelectionStart = m.viewport.CursorY
				m.statusMsg = "Visual select — move cursor to extend, c/y to copy, v/Esc to cancel"
			}
			m.statusTime = time.Now()

		// Word-wrap toggle
		case "w", "W":
			m.viewport.WrapLines = !m.viewport.WrapLines
			if m.viewport.WrapLines {
				m.statusMsg = "↩ Word-wrap ON"
			} else {
				m.statusMsg = "→ Word-wrap OFF"
			}
			m.statusTime = time.Now()

		// Sort order toggle (ascending / descending by timestamp)
		case "s", "S":
			m.viewport.SortDescending = !m.viewport.SortDescending
			m.viewport.UpdateFilters(m.sidebar.Selected)
			if m.viewport.SortDescending {
				m.statusMsg = "↓ Sort: newest first"
			} else {
				m.statusMsg = "↑ Sort: oldest first"
			}
			m.statusTime = time.Now()

		// Severity level filters (number keys — 0=all, 1=error, 2=warn+, 3=debug+)
		case "0":
			m.viewport.FilterLevel = "ALL"
			m.viewport.UpdateFilters(m.sidebar.Selected)
			m.statusMsg = "🟢 Level filter: ALL"
			m.statusTime = time.Now()

		case "1":
			m.viewport.FilterLevel = "ERROR"
			m.viewport.UpdateFilters(m.sidebar.Selected)
			m.statusMsg = "🔴 Level filter: ERROR only"
			m.statusTime = time.Now()

		case "2":
			m.viewport.FilterLevel = "WARN"
			m.viewport.UpdateFilters(m.sidebar.Selected)
			m.statusMsg = "🟡 Level filter: WARN and above"
			m.statusTime = time.Now()

		case "3":
			m.viewport.FilterLevel = "DEBUG"
			m.viewport.UpdateFilters(m.sidebar.Selected)
			m.statusMsg = "⚪ Level filter: DEBUG and above (all)"
			m.statusTime = time.Now()

		// Cycle themes dynamically
		case "t", "T":
			m.themeIdx = (m.themeIdx + 1) % len(Themes)
			InitStyles(Themes[m.themeIdx])
			m.statusMsg = fmt.Sprintf("🎨 Theme: %s", Themes[m.themeIdx].Name)
			m.statusTime = time.Now()

		// Cursor / scroll movements
		case "up", "k":
			if m.sidebar.Focused {
				m.sidebar.CursorUp()
			} else {
				m.viewport.MoveUp()
			}

		case "down", "j":
			if m.sidebar.Focused {
				m.sidebar.CursorDown()
			} else {
				m.viewport.MoveDown()
			}

		case "space":
			if m.sidebar.Focused {
				m.sidebar.ToggleSelected()
				m.viewport.UpdateFilters(m.sidebar.Selected)
			}

		case "pgup":
			m.viewport.PageUp()

		case "pgdown":
			m.viewport.PageDown()
		}
	}

	return m, nil
}

// Shutdown cancels the root context, which cascades to all background goroutines.
func (m *Model) Shutdown() {
	if m.cancelFn != nil {
		m.cancelFn()
	}
}

// View compiles all panels into the terminal window layout
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading kblog viewport..."
	}

	// 1. Build Header
	versionLabel := m.version
	if versionLabel == "" {
		versionLabel = "dev"
	}
	headerLeft := TitleStyle.Render(fmt.Sprintf("⚡ KBLOG %s", versionLabel))
	var resourceInfo string
	if m.podName != "" {
		resourceInfo = fmt.Sprintf("Pod: %s", m.podName)
	} else {
		resourceInfo = fmt.Sprintf("Deployment selector: %s", m.deployment)
	}
	headerRight := fmt.Sprintf("Context: %s | NS: %s | %s | Level: %s", m.contextName, m.namespace, resourceInfo, m.viewport.FilterLevel)
	headerText := lipgloss.JoinHorizontal(lipgloss.Top, headerLeft, strings.Repeat(" ", m.width-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight)-4), headerRight)
	headerPanel := HeaderStyle.Width(m.width - 2).Render(headerText)

	// 2. Build Body panels
	bodyHeight := m.height - 5
	var bodyView string

	if m.showSidebar {
		sidebarWidth := 26
		viewportWidth := m.width - sidebarWidth
		logView := m.viewport.Render(viewportWidth, bodyHeight)
		sideView := m.sidebar.Render(sidebarWidth, bodyHeight)
		bodyView = lipgloss.JoinHorizontal(lipgloss.Top, logView, sideView)
	} else {
		bodyView = m.viewport.Render(m.width, bodyHeight)
	}

	// 3. Build Footer/Input/Modal Overlay
	var footerText string
	if m.searching {
		footerText = m.searchInput.View()
	} else {
		var statusText string
		if m.viewport.StreamError != "" {
			// Sticky stream-down indicator takes precedence over transient toasts.
			statusText = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render("⚠️ " + m.viewport.StreamError)
		} else if m.statusMsg != "" && time.Since(m.statusTime) < 3*time.Second {
			statusText = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(m.statusMsg)
		} else {
			bufInfo := fmt.Sprintf("%d", len(m.viewport.AllLines))
			if m.viewport.Truncated {
				// Signal that the rolling buffer has dropped older history.
				bufInfo = fmt.Sprintf("%d kept of %d rcvd ⚠", len(m.viewport.AllLines), m.viewport.TotalReceived)
			}
			statusText = fmt.Sprintf("Matches: %d/%s | AutoScroll: %t", len(m.viewport.FilteredLines), bufInfo, m.viewport.AutoScroll)
		}

		sortIndicator := "↑"
		if m.viewport.SortDescending {
			sortIndicator = "↓"
		}
		wrapStr := "w"
		if m.viewport.WrapLines {
			wrapStr = "w✓"
		}
		// Build the sidebar hint independently so its verb/arg count matches the
		// current sidebar state. The remaining help string has a fixed verb count.
		var sidebarHint string
		if m.showSidebar {
			sidebarHint = fmt.Sprintf("%s=close %s=focus %s=toggle",
				HelpKeyStyle.Render("l"),
				HelpKeyStyle.Render("tab"),
				HelpKeyStyle.Render("space"),
			)
		} else {
			sidebarHint = fmt.Sprintf("%s=sidebar", HelpKeyStyle.Render("l"))
		}
		helpText := sidebarHint + fmt.Sprintf(
			" %s=search %s=JSON %s/%s=copy/select %s=wrap %s=sort(%s) %s=level %s=theme %s=follow %s=quit",
			HelpKeyStyle.Render("/"),
			HelpKeyStyle.Render("↵"),
			HelpKeyStyle.Render("c"),
			HelpKeyStyle.Render("v"),
			HelpKeyStyle.Render(wrapStr),
			HelpKeyStyle.Render("s"),
			sortIndicator,
			HelpKeyStyle.Render("0-3"),
			HelpKeyStyle.Render("t"),
			HelpKeyStyle.Render("f"),
			HelpKeyStyle.Render("q"),
		)
		gap := m.width - lipgloss.Width(statusText) - lipgloss.Width(helpText) - 4
		if gap < 0 {
			gap = 0
		}
		footerText = lipgloss.JoinHorizontal(lipgloss.Top, statusText, strings.Repeat(" ", gap), helpText)
	}
	footerPanel := FooterStyle.Width(m.width - 2).Render(footerText)

	// 4. Join panels vertically
	fullLayout := lipgloss.JoinVertical(lipgloss.Left, headerPanel, bodyView, footerPanel)

	// Full-screen JSON inspector (like k9s drill-down)
	if m.showModal {
		return m.jsonInspector.Render(m.width, m.height)
	}

	return MainContainer.Render(fullLayout)
}
