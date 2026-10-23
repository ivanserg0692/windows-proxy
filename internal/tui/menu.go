package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

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
	menu := list.New(items, delegate, 60, 8)
	menu.SetShowTitle(false)
	menu.SetShowStatusBar(false)
	menu.SetShowHelp(false)
	menu.SetFilteringEnabled(false)
	menu.DisableQuitKeybindings()
	return menu
}

var menuItems = []string{"Start proxy", "Configure proxy", "Select WSL distribution", "Connection diagnostics", "Exit"}

// menuScreen owns selection; the root decides how to perform each action.
type menuScreen struct{ choices list.Model }

func newMenuScreen() *menuScreen {
	return &menuScreen{choices: choiceList(menuItems)}
}

func (menu *menuScreen) update(msg tea.Msg) (tea.Cmd, screenIntent) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "enter" {
			switch menu.choices.Index() {
			case 0:
				return nil, startProxyIntent{}
			case 1:
				return nil, openFormIntent{}
			case 2:
				return nil, chooseDistributionIntent{}
			case 3:
				return nil, diagnoseIntent{}
			case 4:
				return nil, quitIntent{}
			}
		}
		var cmd tea.Cmd
		menu.choices, cmd = menu.choices.Update(msg)
		return cmd, nil
	}
	return nil, nil
}

func (menu *menuScreen) resize(width, height int) {
	menu.choices.SetSize(max(20, min(64, width-8)), 7)
}

func (menu *menuScreen) view(snapshot screenSnapshot) screenContent {
	body := menu.choices.View()
	if snapshot.proxyActive {
		body += "\n" + muted.Render("Proxy remains active. Start proxy opens its status.")
	}
	return screenContent{body: body, help: "Up/Down: select   Enter: open   Ctrl+C: exit"}
}
