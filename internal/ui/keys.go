package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the complete keybinding contract from TUI-DESIGN.md §7. Adding a
// key here means removing one — keep help.go and the x menu in sync.
type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Home         key.Binding
	Enter        key.Binding
	Search       key.Binding
	LoadMore     key.Binding
	Quit         key.Binding
	FollowEnd    key.Binding
	Tab          key.Binding
	Left         key.Binding
	Right        key.Binding
	Action       key.Binding
	Tab1         key.Binding
	Tab2         key.Binding
	Tab3         key.Binding
	Tab4         key.Binding
	TabPrev      key.Binding
	TabNext      key.Binding
	Filter       key.Binding
	Help         key.Binding
	JumpDoctor   key.Binding
	JumpTimeline key.Binding
	NextMatch    key.Binding
	PrevMatch    key.Binding
}

var keys = keyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move / scroll")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move / scroll")),
	PageUp:       key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("PgUp/^U", "page up")),
	PageDown:     key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("PgDn/^D", "page down")),
	Home:         key.NewBinding(key.WithKeys("home"), key.WithHelp("Home", "top / load more")),
	Enter:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "zoom / jump / run probe")),
	Search:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	LoadMore:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "load older logs")),
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	FollowEnd:    key.NewBinding(key.WithKeys("g", "end"), key.WithHelp("g/End", "follow logs")),
	Tab:          key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab/←→", "switch panel")),
	Left:         key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "left column")),
	Right:        key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "main panel")),
	Action:       key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "menu")),
	Tab1:         key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "logs / doctor")),
	Tab2:         key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "stats / timeline")),
	Tab3:         key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "deps / graph")),
	Tab4:         key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "health")),
	TabPrev:      key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev tab")),
	TabNext:      key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next tab")),
	Filter:       key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter all→failed→waiting")),
	Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	JumpDoctor:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "jump doctor")),
	JumpTimeline: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "jump timeline")),
	NextMatch:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
	PrevMatch:    key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev match")),
}
