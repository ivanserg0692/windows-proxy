package screens

import (
	"bytes"
	"strings"
	"text/template"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"wslproxy/internal/tui/layout"
)

type Distributions struct {
	area      layout.Size
	titleArea layout.Size
	listArea  layout.Size
	receiver  DistributionsReceiver
	choices   list.Model
	forForm   bool
}

func NewDistributions(receiver DistributionsReceiver) *Distributions {
	return &Distributions{choices: choiceList(nil), receiver: receiver}
}

func (distributions *Distributions) Open(forForm bool) {
	distributions.forForm = forForm
}

func (distributions *Distributions) SetNames(names []string) {
	distributions.choices = choiceList(names)
	distributions.calculateLayout()
}

func (distributions *Distributions) Help() string {
	return "Up/Down: select   Enter: use   R: refresh   Esc: back"
}

func (distributions *Distributions) Resize(area layout.Size, _ Snapshot) {
	distributions.area = area
	distributions.calculateLayout()
}

func (distributions *Distributions) calculateLayout() {
	area := distributions.area
	title := lipgloss.NewStyle().MarginBottom(1).Render("Select WSL distribution")
	_, size := layout.Measure(title, area.Width)
	title = layout.Fit(title, layout.Size{Width: area.Width, Height: min(size.Height, max(0, area.Height-1))})
	areas := layout.Column(area, layout.Block{Text: title}, layout.Block{Grow: true})
	distributions.titleArea, distributions.listArea = areas[0], areas[1]
	distributions.choices.SetSize(max(1, areas[1].Width), max(1, areas[1].Height))
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

var distributionsTemplate = template.Must(template.New("distributions").Parse(`{{if .Title}}{{.Title}}
{{end}}{{if .List}}{{.List}}
{{end}}`))

type distributionsViewData struct{ Title, List string }

func (distributions *Distributions) prepareViewData() distributionsViewData {
	title := lipgloss.NewStyle().MarginBottom(1).Render("Select WSL distribution")
	return distributionsViewData{
		Title: layout.Fit(title, distributions.titleArea),
		List:  layout.Fit(distributions.choices.View(), distributions.listArea),
	}
}

func (distributions *Distributions) View(Snapshot) Content {
	var content bytes.Buffer
	if err := distributionsTemplate.Execute(&content, distributions.prepareViewData()); err != nil {
		panic("render distributions template: " + err.Error())
	}
	return Content{Body: layout.Fit(strings.TrimSuffix(content.String(), "\n"), distributions.area)}
}
