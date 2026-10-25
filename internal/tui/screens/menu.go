package screens

import (
	"bytes"
	"strings"
	"text/template"

	"wslproxy/internal/tui/layout"

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
	menu := list.New(items, delegate, 0, 0)
	menu.SetShowTitle(false)
	menu.SetShowStatusBar(false)
	menu.SetShowHelp(false)
	menu.SetFilteringEnabled(false)
	menu.DisableQuitKeybindings()
	return menu
}

var menuItems = []string{"Start proxy", "Configure proxy", "Select WSL distribution", "Connection diagnostics", "Exit"}

// Menu owns selection; the root decides how to perform each action.
type Menu struct {
	area     layout.Size
	listArea layout.Size
	noteArea layout.Size
	choices  list.Model
	receiver MenuReceiver
}

func NewMenu(receiver MenuReceiver) *Menu {
	return &Menu{choices: choiceList(menuItems), receiver: receiver}
}

func (menu *Menu) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "enter" {
			switch menu.choices.Index() {
			case 0:
				return menu.receiver.ToggleProxy()
			case 1:
				return menu.receiver.OpenForm()
			case 2:
				return menu.receiver.ChooseDistribution(false)
			case 3:
				return menu.receiver.Diagnose()
			case 4:
				return menu.receiver.Quit()
			}
		}
		var cmd tea.Cmd
		menu.choices, cmd = menu.choices.Update(msg)
		return cmd
	}
	return nil
}

func (menu *Menu) Help() string { return "Up/Down: select   Enter: open   Ctrl+C: exit" }

func menuNote(snapshot Snapshot) string {
	if snapshot.ProxyActive {
		return muted.Render("Proxy remains active. Stop proxy stops it and releases the port.")
	}
	return ""
}

func (menu *Menu) Resize(area layout.Size, snapshot Snapshot) {
	menu.area = area
	menu.calculateLayout(snapshot)
}

func (menu *Menu) calculateLayout(snapshot Snapshot) {
	area := menu.area
	label := "Start proxy"
	if snapshot.ProxyActive {
		label = "Stop proxy"
	}
	if menu.choices.Items()[0].(item) != item(label) {
		menu.choices.SetItem(0, item(label))
	}
	note := menuNote(snapshot)
	_, size := layout.Measure(note, area.Width)
	// Keep the selected item visible before admitting explanatory text.
	note = layout.Fit(note, layout.Size{Width: area.Width, Height: min(size.Height, max(0, area.Height-1))})
	areas := layout.Column(area, layout.Block{Grow: true}, layout.Block{Text: note})
	menu.listArea, menu.noteArea = areas[0], areas[1]
	menu.choices.SetSize(max(1, areas[0].Width), max(1, areas[0].Height))
}

// Each occupied block ends with a template-owned separator. View removes
// the final separator, preserving any padding inside the blocks themselves.
var menuTemplate = template.Must(template.New("menu").Parse(`{{if .List}}{{.List}}
{{end}}{{if .Note}}{{.Note}}
{{end}}`))

type menuViewData struct{ List, Note string }

func (menu *Menu) prepareViewData(snapshot Snapshot) menuViewData {
	return menuViewData{
		List: layout.Fit(menu.choices.View(), menu.listArea),
		Note: layout.Fit(menuNote(snapshot), menu.noteArea),
	}
}

func (menu *Menu) View(snapshot Snapshot) Content {
	var content bytes.Buffer
	if err := menuTemplate.Execute(&content, menu.prepareViewData(snapshot)); err != nil {
		panic("render menu template: " + err.Error())
	}
	return Content{Body: layout.Fit(strings.TrimSuffix(content.String(), "\n"), menu.area)}
}
