package screens

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

// Menu owns selection; the root decides how to perform each action.
type Menu struct{ choices list.Model }

func NewMenu() *Menu {
	return &Menu{choices: choiceList(menuItems)}
}

func (menu *Menu) Update(msg tea.Msg) (tea.Cmd, Intent) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "enter" {
			switch menu.choices.Index() {
			case 0:
				return nil, StartProxyIntent{}
			case 1:
				return nil, OpenFormIntent{}
			case 2:
				return nil, ChooseDistributionIntent{}
			case 3:
				return nil, DiagnoseIntent{}
			case 4:
				return nil, QuitIntent{}
			}
		}
		var cmd tea.Cmd
		menu.choices, cmd = menu.choices.Update(msg)
		return cmd, nil
	}
	return nil, nil
}

func (menu *Menu) Resize(width, height int) {
	menu.choices.SetSize(max(20, min(64, width-8)), 7)
}

func (menu *Menu) View(snapshot Snapshot) Content {
	body := menu.choices.View()
	if snapshot.ProxyActive {
		body += "\n" + muted.Render("Proxy remains active. Start proxy opens its status.")
	}
	return Content{Body: body, Help: "Up/Down: select   Enter: open   Ctrl+C: exit"}
}
