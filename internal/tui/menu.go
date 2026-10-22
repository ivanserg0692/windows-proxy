package tui

import "charm.land/bubbles/v2/list"

type item string

func (i item) Title() string       { return safe(string(i)) }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return string(i) }

func choiceList(names []string) list.Model {
	items := make([]list.Item, len(names))
	for i, name := range names {
		items[i] = item(name)
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	m := list.New(items, delegate, 60, 8)
	m.SetShowTitle(false)
	m.SetShowStatusBar(false)
	m.SetShowHelp(false)
	m.SetFilteringEnabled(false)
	m.DisableQuitKeybindings()
	return m
}

var menuItems = []string{"Start proxy", "Configure proxy", "Select WSL distribution", "Connection diagnostics", "Exit"}
