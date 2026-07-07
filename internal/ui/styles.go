package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorHealthy   = lipgloss.Color("#22c55e")
	colorStarting  = lipgloss.Color("#eab308")
	colorUnhealthy = lipgloss.Color("#ef4444")
	colorPending   = lipgloss.Color("#6b7280")
	colorSelected  = lipgloss.Color("#1e3a5f")

	styleSelected = lipgloss.NewStyle().Background(colorSelected)
	styleName     = lipgloss.NewStyle().Bold(true)
	styleDim      = lipgloss.NewStyle().Faint(true)

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#64748b"))

	stylePanelFocus = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#6366f1"))

	stylePanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e2e8f0"))

	styleLogHeader = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#1e293b")).
			Foreground(lipgloss.Color("#f1f5f9"))

	styleLogFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94a3b8")).
			Faint(true).
			Background(lipgloss.Color("#0f172a"))

	styleLogMarker = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6366f1")).
			Bold(true)

	styleScrollTrack = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#475569"))

	styleScrollThumb = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#cbd5e1"))

	styleSearchPrompt = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94a3b8")).
				Bold(true)

	styleSearchInput = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f8fafc"))

	styleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94a3b8")).
			Faint(true)

	styleSummaryBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e2e8f0")).
			Background(lipgloss.Color("#1e293b"))

	styleSectionHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#94a3b8"))

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f8fafc")).
			Background(lipgloss.Color("#6366f1"))
)

var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// Pre-rendered state glyphs, built once at init instead of allocating a new
// lipgloss.Style on every row on every render tick.
var (
	glyphHealthy   = lipgloss.NewStyle().Foreground(colorHealthy).Render("●")
	glyphPending   = lipgloss.NewStyle().Foreground(colorPending).Render("○")
	glyphCompleted = lipgloss.NewStyle().Foreground(colorHealthy).Render("✓") // success, not failure
	glyphFailed    = lipgloss.NewStyle().Foreground(colorUnhealthy).Render("✕")
	glyphUnhealthy = lipgloss.NewStyle().Foreground(colorUnhealthy).Render("●")
	glyphDegraded  = lipgloss.NewStyle().Foreground(colorStarting).Render("⚠") // wired up in Phase 2

	glyphStartingFrames = [4]string{
		lipgloss.NewStyle().Foreground(colorStarting).Render("◐"),
		lipgloss.NewStyle().Foreground(colorStarting).Render("◓"),
		lipgloss.NewStyle().Foreground(colorStarting).Render("◑"),
		lipgloss.NewStyle().Foreground(colorStarting).Render("◒"),
	}
)
