package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type distributionsScreen struct {
	choices list.Model
	forForm bool
}

func newDistributionsScreen() *distributionsScreen {
	return &distributionsScreen{choices: choiceList(nil)}
}

func (distributions *distributionsScreen) open(forForm bool) {
	distributions.forForm = forForm
}

func (distributions *distributionsScreen) setNames(names []string, width, height int) {
	distributions.choices = choiceList(names)
	distributions.resize(width, height)
}

func (distributions *distributionsScreen) resize(width, height int) {
	distributions.choices.SetSize(max(20, min(64, width-8)), max(3, min(10, height-13)))
}

func (distributions *distributionsScreen) update(msg tea.Msg) (tea.Cmd, screenIntent) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil, nil
	}
	switch key.String() {
	case "r":
		return nil, refreshDistributionsIntent{}
	case "enter":
		selected, ok := distributions.choices.SelectedItem().(item)
		if !ok {
			return nil, nil
		}
		return nil, distributionSelectedIntent{name: string(selected), forForm: distributions.forForm}
	}
	var cmd tea.Cmd
	distributions.choices, cmd = distributions.choices.Update(msg)
	return cmd, nil
}

func (distributions *distributionsScreen) view(screenSnapshot) screenContent {
	return screenContent{
		body: "Select WSL distribution\n\n" + distributions.choices.View(),
		help: "Up/Down: select   Enter: use   R: refresh   Esc: back",
	}
}
