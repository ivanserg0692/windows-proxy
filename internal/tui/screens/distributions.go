package screens

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type Distributions struct {
	receiver DistributionsReceiver
	choices  list.Model
	forForm  bool
}

func NewDistributions(receiver DistributionsReceiver) *Distributions {
	return &Distributions{choices: choiceList(nil), receiver: receiver}
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

func (distributions *Distributions) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "r":
		return distributions.receiver.RefreshDistributions()
	case "enter":
		selected, ok := distributions.choices.SelectedItem().(item)
		if !ok {
			return nil
		}
		return distributions.receiver.SelectDistribution(string(selected), distributions.forForm)
	}
	var cmd tea.Cmd
	distributions.choices, cmd = distributions.choices.Update(msg)
	return cmd
}

func (distributions *Distributions) View(Snapshot) Content {
	return Content{
		Body: "Select WSL distribution\n\n" + distributions.choices.View(),
		Help: "Up/Down: select   Enter: use   R: refresh   Esc: back",
	}
}
