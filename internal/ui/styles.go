package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorHealthy   = lipgloss.Color("#34d399")
	colorStarting  = lipgloss.Color("#fbbf24")
	colorUnhealthy = lipgloss.Color("#f87171")
	colorPending   = lipgloss.Color("#64748b")
	colorAccent    = lipgloss.Color("#38bdf8")
	colorSelected  = lipgloss.Color("#1e3a5f")

	styleSelected = lipgloss.NewStyle().
			Background(colorSelected).
			Foreground(lipgloss.Color("#f8fafc")).
			Bold(true)
	styleName = lipgloss.NewStyle().Bold(true)
	styleDim  = lipgloss.NewStyle().Faint(true)

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#64748b"))

	stylePanelFocus = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorAccent)

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
			Foreground(colorAccent).
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
			Foreground(lipgloss.Color("#e2e8f0"))

	styleSectionHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#94a3b8"))

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f8fafc")).
			Background(colorAccent)

	// styleCritical/Warn/Info color finding/status text by severity, matching
	// the glyph colors in TUI-DESIGN.md §8.
	styleCritical = lipgloss.NewStyle().Foreground(colorUnhealthy)
	styleWarn     = lipgloss.NewStyle().Foreground(colorStarting)
	styleInfo     = lipgloss.NewStyle().Faint(true)

	// Dependency-graph row colors (state / detail columns).
	styleStateHealthy   = lipgloss.NewStyle().Foreground(colorHealthy)
	styleStateStarting  = lipgloss.NewStyle().Foreground(colorStarting).Bold(true)
	styleStateBlocked   = lipgloss.NewStyle().Foreground(colorStarting)
	styleStateFailed    = lipgloss.NewStyle().Foreground(colorUnhealthy).Bold(true)
	styleStatePending   = lipgloss.NewStyle().Foreground(colorPending)
	styleDetailNeeds    = lipgloss.NewStyle().Foreground(colorStarting)
	styleDetailExitOK   = lipgloss.NewStyle().Foreground(colorHealthy).Faint(true)
	styleDetailExitFail = lipgloss.NewStyle().Foreground(colorUnhealthy)
	styleProjectName    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleTreePrefix     = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))

	styleAxis = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))

	// Shared timeline palette — bar fills and legend chips use the same hex.
	colorTLPending   = lipgloss.Color("#475569")
	colorTLBlocked   = lipgloss.Color("#52525b")
	colorTLStarting  = lipgloss.Color("#a16207")
	colorTLHealthy   = lipgloss.Color("#047857")
	colorTLFailed    = lipgloss.Color("#991b1b")
	colorTLUnhealthy = lipgloss.Color("#9a3412")

	// Timeline bar fills (muted foreground on mid-cell bars — thin track).
	styleTLSegPending   = lipgloss.NewStyle().Foreground(colorTLPending)
	styleTLSegBlocked   = lipgloss.NewStyle().Foreground(colorTLBlocked)
	styleTLSegStarting  = lipgloss.NewStyle().Foreground(colorTLStarting)
	styleTLSegHealthy   = lipgloss.NewStyle().Foreground(colorTLHealthy)
	styleTLSegFailed    = lipgloss.NewStyle().Foreground(colorTLFailed)
	styleTLSegUnhealthy = lipgloss.NewStyle().Foreground(colorTLUnhealthy)

	// Legend chips — same colors as the bars, via background so they read as swatches.
	styleTLLegendPending  = lipgloss.NewStyle().Background(colorTLPending)
	styleTLLegendBlocked  = lipgloss.NewStyle().Background(colorTLBlocked)
	styleTLLegendStarting = lipgloss.NewStyle().Background(colorTLStarting)
	styleTLLegendHealthy  = lipgloss.NewStyle().Background(colorTLHealthy)
	styleTLLegendFailed   = lipgloss.NewStyle().Background(colorTLFailed)
	styleTLNow            = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	// Sparkline coloring per TUI-DESIGN.md §8: green under 70%, amber to 90%, red above.
	styleSparklineGreen = lipgloss.NewStyle().Foreground(colorHealthy)
	styleSparklineAmber = lipgloss.NewStyle().Foreground(colorStarting)
	styleSparklineRed   = lipgloss.NewStyle().Foreground(colorUnhealthy)

	styleMenuKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)
)

var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// Pre-rendered state glyphs, built once at init instead of allocating a new
// lipgloss.Style on every row on every render tick.
var (
	glyphHealthy   = lipgloss.NewStyle().Foreground(colorHealthy).Render("●")
	glyphPending   = lipgloss.NewStyle().Foreground(colorPending).Render("┄")
	glyphCompleted = lipgloss.NewStyle().Foreground(colorHealthy).Render("✓") // success, not failure
	glyphFailed    = lipgloss.NewStyle().Foreground(colorUnhealthy).Render("✕")
	glyphUnhealthy = lipgloss.NewStyle().Foreground(colorUnhealthy).Render("✕")
	glyphDegraded  = lipgloss.NewStyle().Foreground(colorStarting).Render("⚠")

	glyphStartingFrames = [4]string{
		lipgloss.NewStyle().Foreground(colorStarting).Render("◐"),
		lipgloss.NewStyle().Foreground(colorStarting).Render("◓"),
		lipgloss.NewStyle().Foreground(colorStarting).Render("◑"),
		lipgloss.NewStyle().Foreground(colorStarting).Render("◒"),
	}
)

// sparklineTicks are the 8 block glyphs used to render CPU/MEM sparklines,
// from lowest to highest.
var sparklineTicks = []rune("▁▂▃▄▅▆▇█")
