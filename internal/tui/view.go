package tui

import (
	"bytes"
	"text/template"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/tui/screens"
)

const viewTemplateText = `{{.Title}}                         {{.Status}}
{{.Subtitle}}

{{.Body}}{{if .Busy}}

{{.Busy}}{{end}}{{if .Error}}

{{.Error}}{{end}}{{if .Notice}}

{{.Notice}}{{end}}{{.Footer}}

{{.Help}}`

var viewTemplate = template.Must(template.New("tui").Parse(viewTemplateText))

type viewData struct {
	Title, Status, Subtitle string
	Body, Footer, Help      string
	Busy, Error, Notice     string
	width, height           int
}

func (m *model) View() tea.View {
	if m.width < 40 || m.height < 20 {
		view := tea.NewView("WSL PROXY\nResize terminal to at least 40 x 20.\nCtrl+C to exit.")
		view.AltScreen = true
		return view
	}

	data := styleViewData(m.prepareViewData())
	var content bytes.Buffer
	if err := viewTemplate.Execute(&content, data); err != nil {
		// The static template and concrete data type must agree; failure is a programming error.
		panic("render TUI template: " + err.Error())
	}
	frame := frameStyle.Width(data.width - 2).MaxWidth(data.width).MaxHeight(data.height - 1).Render(content.String())
	view := tea.NewView(frame)
	view.AltScreen = true
	view.WindowTitle = "WSL Proxy"
	return view
}

func (m *model) prepareViewData() viewData {
	data := viewData{
		Title: "WSL PROXY", Status: "STOPPED", Subtitle: "Windows -> WSL2 HTTP Reverse Proxy",
		Busy: m.busy, Error: safe(m.error), Notice: safe(m.notice),
		width: min(72, m.width-4), height: m.height,
	}
	if m.running != nil {
		data.Status = "RUNNING"
	}

	snapshot := screens.Snapshot{Config: m.config, ProxyActive: m.running != nil}
	if m.active == m.runningView {
		if m.running != nil {
			snapshot.Running = &screens.RunningSnapshot{Config: m.running.Config, Stats: m.running.Server.Snapshot()}
		}
		if m.height >= 32 {
			snapshot.RecentEvents = m.logs.Lines()
		}
	}

	screen := m.active.View(snapshot)
	data.Body, data.Footer, data.Help = screen.Body, screen.Footer, screen.Help
	if m.busy != "" {
		data.Help = "Ctrl+C: cancel and exit"
	}
	return data
}
