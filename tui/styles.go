package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds color tokens for the app appearance
type Theme struct {
	Name         string
	Primary      lipgloss.TerminalColor
	Secondary    lipgloss.TerminalColor
	Accent       lipgloss.TerminalColor
	Info         lipgloss.TerminalColor
	Warn         lipgloss.TerminalColor
	Error        lipgloss.TerminalColor
	Debug        lipgloss.TerminalColor
	Bg           lipgloss.TerminalColor
	HeaderBg     lipgloss.TerminalColor
	Border       lipgloss.TerminalColor
	ActiveBorder lipgloss.TerminalColor
}

// Built-in Themes list
var Themes = []Theme{
	{
		Name:         "Terminal (Default)",
		Primary:      lipgloss.NoColor{},
		Secondary:    lipgloss.NoColor{},
		Accent:       lipgloss.NoColor{},
		Info:         lipgloss.NoColor{},
		Warn:         lipgloss.NoColor{},
		Error:        lipgloss.NoColor{},
		Debug:        lipgloss.NoColor{},
		Bg:           lipgloss.NoColor{},
		HeaderBg:     lipgloss.NoColor{},
		Border:       lipgloss.NoColor{},
		ActiveBorder: lipgloss.NoColor{},
	},
	{
		Name:         "Midnight",
		Primary:      lipgloss.Color("#8A2BFF"), // Purple Accent
		Secondary:    lipgloss.Color("#00E5FF"), // Neon Cyan
		Accent:       lipgloss.Color("#FF007F"), // Pink Red
		Info:         lipgloss.Color("#00FF87"), // Mint Green
		Warn:         lipgloss.Color("#FFD700"), // Gold
		Error:        lipgloss.Color("#FF2A5F"), // Crimson Red
		Debug:        lipgloss.Color("#A0A0A0"), // Muted Gray
		Bg:           lipgloss.Color("#121214"), // Dark Gray
		HeaderBg:     lipgloss.Color("#1E1E24"), // Blue Slate
		Border:       lipgloss.Color("#2E2E38"), // Border
		ActiveBorder: lipgloss.Color("#8A2BFF"),
	},
	{
		Name:         "Dracula",
		Primary:      lipgloss.Color("#BD93F9"), // Purple
		Secondary:    lipgloss.Color("#8BE9FD"), // Cyan
		Accent:       lipgloss.Color("#FF79C6"), // Pink
		Info:         lipgloss.Color("#50FA7B"), // Green
		Warn:         lipgloss.Color("#F1FA8C"), // Yellow
		Error:        lipgloss.Color("#FF5555"), // Red
		Debug:        lipgloss.Color("#6272A4"), // Muted Blue
		Bg:           lipgloss.Color("#282A36"), // Dark
		HeaderBg:     lipgloss.Color("#1E1F29"), // Deepest
		Border:       lipgloss.Color("#44475A"),
		ActiveBorder: lipgloss.Color("#BD93F9"),
	},
	{
		Name:         "Catppuccin Macchiato",
		Primary:      lipgloss.Color("#C6A0F6"), // Mauve
		Secondary:    lipgloss.Color("#8BD5CA"), // Havre
		Accent:       lipgloss.Color("#F5BDE6"), // Pink
		Info:         lipgloss.Color("#A6DA95"), // Green
		Warn:         lipgloss.Color("#EED49F"), // Yellow
		Error:        lipgloss.Color("#ED8796"), // Red
		Debug:        lipgloss.Color("#5B6078"), // Muted Blue Gray
		Bg:           lipgloss.Color("#24273A"), // Blue Slate Dark
		HeaderBg:     lipgloss.Color("#1E2030"),
		Border:       lipgloss.Color("#36394F"),
		ActiveBorder: lipgloss.Color("#C6A0F6"),
	},
	{
		Name:         "Nord",
		Primary:      lipgloss.Color("#88C0D0"), // Frost Ice
		Secondary:    lipgloss.Color("#8FBCBB"), // Teal
		Accent:       lipgloss.Color("#B48EAD"), // Purple Accent
		Info:         lipgloss.Color("#A3BE8C"), // Aurora Green
		Warn:         lipgloss.Color("#EBCB8B"), // Aurora Yellow
		Error:        lipgloss.Color("#BF616A"), // Aurora Red
		Debug:        lipgloss.Color("#4C566A"), // Slate Gray
		Bg:           lipgloss.Color("#2E3440"), // Polar Night
		HeaderBg:     lipgloss.Color("#232731"),
		Border:       lipgloss.Color("#3B4252"),
		ActiveBorder: lipgloss.Color("#88C0D0"),
	},
	{
		Name:         "Monokai",
		Primary:      lipgloss.Color("#AE81FF"), // Purple
		Secondary:    lipgloss.Color("#66D9EF"), // Cyan
		Accent:       lipgloss.Color("#F92672"), // Pink
		Info:         lipgloss.Color("#A6E22E"), // Lime Green
		Warn:         lipgloss.Color("#FD971F"), // Orange
		Error:        lipgloss.Color("#F92672"), // Red
		Debug:        lipgloss.Color("#75715E"), // Muted Olive
		Bg:           lipgloss.Color("#272822"), // Classic Monokai Dark
		HeaderBg:     lipgloss.Color("#1E1F1C"),
		Border:       lipgloss.Color("#3E3D32"),
		ActiveBorder: lipgloss.Color("#A6E22E"),
	},
}

// Current theme variables (accessed dynamically)
var (
	PrimaryColor   lipgloss.TerminalColor
	SecondaryColor lipgloss.TerminalColor
	AccentColor    lipgloss.TerminalColor
	InfoColor      lipgloss.TerminalColor
	WarnColor      lipgloss.TerminalColor
	ErrorColor     lipgloss.TerminalColor
	DebugColor     lipgloss.TerminalColor
	BgColor        lipgloss.TerminalColor
	HeaderBgColor  lipgloss.TerminalColor
	BorderColor    lipgloss.TerminalColor
	ActiveBorder   lipgloss.TerminalColor
)

// UI Component Styles
var (
	MainContainer     lipgloss.Style
	HeaderStyle       lipgloss.Style
	TitleStyle        lipgloss.Style
	FooterStyle       lipgloss.Style
	SidebarStyle      lipgloss.Style
	SidebarTitle      lipgloss.Style
	ContainerActive   lipgloss.Style
	ContainerInactive lipgloss.Style
	ViewportStyle     lipgloss.Style
	LogTimeStyle      lipgloss.Style
	LogPodStyle       lipgloss.Style
	LogContentStyle   lipgloss.Style
	LogEventBanner    lipgloss.Style
	HelpKeyStyle      lipgloss.Style
	HelpDescStyle     lipgloss.Style
	ModalStyle        lipgloss.Style
)

// InitStyles loads a specific theme and compiles all Lipgloss styles
func InitStyles(theme Theme) {
	// Set active colors
	PrimaryColor = theme.Primary
	SecondaryColor = theme.Secondary
	AccentColor = theme.Accent
	InfoColor = theme.Info
	WarnColor = theme.Warn
	ErrorColor = theme.Error
	DebugColor = theme.Debug
	BgColor = theme.Bg
	HeaderBgColor = theme.HeaderBg
	BorderColor = theme.Border
	ActiveBorder = theme.ActiveBorder

	// Recompile styles
	MainContainer = lipgloss.NewStyle().
		Background(BgColor).
		Padding(0)

	HeaderStyle = lipgloss.NewStyle().
		Background(HeaderBgColor).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	FooterStyle = lipgloss.NewStyle().
		Background(HeaderBgColor).
		Foreground(lipgloss.Color("#888899")).
		Padding(0, 1)

	SidebarStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1).
		Width(22)

	SidebarTitle = lipgloss.NewStyle().
		Foreground(SecondaryColor).
		Bold(true).
		Underline(true).
		PaddingBottom(1)

	ContainerActive = lipgloss.NewStyle().
		Foreground(InfoColor).
		Bold(true)

	ContainerInactive = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666677"))

	ViewportStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveBorder).
		Padding(0, 1)

	LogTimeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555566"))

	LogPodStyle = lipgloss.NewStyle().
		Bold(true)

	LogContentStyle = lipgloss.NewStyle().
		Foreground(InfoColor)

	LogEventBanner = lipgloss.NewStyle().
		Background(lipgloss.Color("#2A1F30")).
		Foreground(WarnColor).
		Bold(true).
		Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(SecondaryColor).
		Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#777788"))

	ModalStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(PrimaryColor).
		Background(lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#1E1E24"}).
		Padding(1, 2).
		Width(65).
		Height(18)
}

func init() {
	// Initialize default theme on startup
	InitStyles(Themes[0])
}
