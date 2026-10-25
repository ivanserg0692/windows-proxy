package tui

import (
	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/tui/layout"
	"wslproxy/internal/tui/screens"
)

// screenComponent is the root's view of a screen. Screens own local state;
// the root coordinates navigation, services, and asynchronous operations.
type screenComponent interface {
	Update(tea.Msg) tea.Cmd
	View(screens.Snapshot) screens.Content
	Help() string
	Resize(layout.Size, screens.Snapshot)
}
