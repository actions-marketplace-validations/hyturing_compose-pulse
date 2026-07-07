package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	Home          key.Binding
	Enter         key.Binding
	Search        key.Binding
	LoadMore      key.Binding
	Back          key.Binding
	Quit          key.Binding
	FollowEnd     key.Binding
	Tab           key.Binding
	Left          key.Binding
	Right         key.Binding
	Action        key.Binding
	Tab1          key.Binding
	Tab2          key.Binding
	Tab3          key.Binding
	FilterFailed  key.Binding
	FilterBlocked key.Binding
	Help          key.Binding
}

var keys = keyMap{
	Up:            key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "line up")),
	Down:          key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "line down")),
	PageUp:        key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("PgUp/^U", "page up")),
	PageDown:      key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("PgDn/^D", "page down")),
	Home:          key.NewBinding(key.WithKeys("home"), key.WithHelp("Home", "top / load more")),
	Enter:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "fullscreen logs")),
	Search:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter logs")),
	LoadMore:      key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "load older logs")),
	Back:          key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/Esc", "back to graph")),
	Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	FollowEnd:     key.NewBinding(key.WithKeys("g", "end"), key.WithHelp("g/End", "follow logs")),
	Tab:           key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
	Left:          key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "services panel")),
	Right:         key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "logs panel")),
	Action:        key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "actions")),
	Tab1:          key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "overview")),
	Tab2:          key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "logs")),
	Tab3:          key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "deps")),
	FilterFailed:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "failed filter")),
	FilterBlocked: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "blocked filter")),
	Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}
