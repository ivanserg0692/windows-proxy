package screens

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type Distributions struct {
	choices list.Model
	forForm bool
}

func NewDistributions() *Distributions {
	return &Distributions{choices: choiceList(nil)}
}

func (distributions *Distributions) Open(forForm bool) {
	distributions.forForm = forForm
}

func (distributions *Distributions) SetNames(names []string, width, height int) {
	distributions.choices = choiceList(names)
	distributions.Resize(width, height)
}

func (distributions *Distributions) Resize(width, height int) {
	distributions.choices.SetSize(max(20, min(64, width-8)), max(3, min(10, height-13)))
}

func (distributions *Distributions) Update(msg tea.Msg) (tea.Cmd, Intent) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil, nil
	}
	switch key.String() {
	case "r":
		return nil, RefreshDistributionsIntent{}
	case "enter":
		selected, ok := distributions.choices.SelectedItem().(item)
		if !ok {
			return nil, nil
		}
		return nil, DistributionSelectedIntent{Name: string(selected), ForForm: distributions.forForm}
	}
	var cmd tea.Cmd
	distributions.choices, cmd = distributions.choices.Update(msg)
	return cmd, nil
}

func (distributions *Distributions) View(Snapshot) Content {
	return Content{
		Body: "Select WSL distribution\n\n" + distributions.choices.View(),
		Help: "Up/Down: select   Enter: use   R: refresh   Esc: back",
	}
}
