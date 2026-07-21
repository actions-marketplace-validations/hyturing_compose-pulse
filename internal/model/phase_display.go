package model

// DisplayState is the TUI glyph family retained from the status-viewer product.
// It maps onto ServicePhase so pstree rendering stays stable.
type DisplayState string

// Display states used by the pstree glyphs.
const (
	DisplayHealthy   DisplayState = "healthy"
	DisplayStarting  DisplayState = "starting"
	DisplayBlocked   DisplayState = "blocked"
	DisplayPending   DisplayState = "pending"
	DisplayMissing   DisplayState = "missing"
	DisplayCompleted DisplayState = "completed"
	DisplayFailed    DisplayState = "failed"
	DisplayUnhealthy DisplayState = "unhealthy"
	DisplayDegraded  DisplayState = "degraded"
)

// ToDisplayState maps a service phase onto the existing DisplayState glyphs.
// Blocked/missing are graph-derived and are not produced here.
func (p ServicePhase) ToDisplayState() DisplayState {
	switch p {
	case PhaseConfigured, PhasePulling, PhaseBuilding, PhaseCreated:
		return DisplayPending
	case PhaseStarted, PhaseProcessRunning, PhasePortListening:
		return DisplayStarting
	case PhaseHealthy, PhaseApplicationReady:
		return DisplayHealthy
	case PhaseExited:
		return DisplayCompleted
	case PhaseFailed:
		return DisplayFailed
	default:
		return DisplayPending
	}
}
