package screens

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/tui/layout"
)

type Diagnostics struct {
	report                  app.Report
	receiver                DiagnosticsReceiver
	area                    layout.Size
	headingArea, probesArea layout.Size
	heading                 string
	config                  config.Config
	probes                  viewport.Model
}

func NewDiagnostics(receiver DiagnosticsReceiver) *Diagnostics {
	return &Diagnostics{receiver: receiver, probes: viewport.New()}
}

func (diagnostics *Diagnostics) SetReport(report app.Report) {
	diagnostics.report = report
	diagnostics.probes.GotoTop()
	diagnostics.calculateLayout(diagnostics.prepareViewData(diagnostics.config))
}

func (diagnostics *Diagnostics) Help() string {
	return "R: run diagnostics   PgUp/PgDown: results   Esc: menu"
}

var diagnosticsTemplate = template.Must(template.New("diagnostics").Parse(`{{if .Heading}}{{.Heading}}
{{end}}{{if .Probes}}{{.Probes}}
{{end}}{{define "heading"}}Windows: {{.Endpoint}}
WSL: {{.Distribution}}, port {{.Port}}{{if .Selected}}
Selected: {{.Selected}}{{end}}{{end}}{{define "probes"}}{{range $index, $probe := .}}{{if $index}}
{{end}}{{$probe.IP}} [{{$probe.Interface}}]: {{$probe.Result}}{{end}}{{end}}`))

type diagnosticsHeadingData struct {
	Endpoint, Distribution, Selected string
	Port                             int
}
type diagnosticProbeData struct{ IP, Interface, Result string }
type diagnosticsViewData struct{ Heading, Probes string }

func (diagnostics *Diagnostics) prepareViewData(cfg config.Config) diagnosticsViewData {
	heading := diagnosticsHeadingData{Endpoint: cfg.ListenEndpoint(), Distribution: safe(cfg.Distribution), Port: cfg.TargetPort}
	if diagnostics.report.Selected.IsValid() {
		heading.Selected = diagnostics.report.Selected.String()
	}
	var title bytes.Buffer
	if err := diagnosticsTemplate.ExecuteTemplate(&title, "heading", heading); err != nil {
		panic("render diagnostics heading template: " + err.Error())
	}
	probes := make([]diagnosticProbeData, len(diagnostics.report.Probes))
	for index, probe := range diagnostics.report.Probes {
		result := fmt.Sprintf("OK (%s)", probe.Duration.Round(time.Millisecond))
		if probe.Error != "" {
			result = "FAIL: " + safe(probe.Error)
		}
		probes[index] = diagnosticProbeData{IP: probe.Candidate.IP.String(), Interface: safe(probe.Candidate.Interface), Result: result}
	}
	var results bytes.Buffer
	if err := diagnosticsTemplate.ExecuteTemplate(&results, "probes", probes); err != nil {
		panic("render diagnostic probes template: " + err.Error())
	}
	return diagnosticsViewData{Heading: lipgloss.NewStyle().MarginBottom(1).Render(title.String()), Probes: results.String()}
}

func (diagnostics *Diagnostics) Resize(area layout.Size, snapshot Snapshot) {
	diagnostics.area, diagnostics.config = area, snapshot.Config
	diagnostics.calculateLayout(diagnostics.prepareViewData(snapshot.Config))
}

// calculateLayout also refreshes viewport content after SetReport, independently
// of terminal resize events. SetContent preserves the current scroll offset.
func (diagnostics *Diagnostics) calculateLayout(data diagnosticsViewData) {
	area := diagnostics.area
	_, size := layout.Measure(data.Heading, area.Width)
	diagnostics.heading = layout.Fit(data.Heading, layout.Size{Width: area.Width, Height: min(size.Height, max(0, area.Height-1))})
	areas := layout.Column(area, layout.Block{Text: diagnostics.heading}, layout.Block{Grow: true})
	diagnostics.headingArea, diagnostics.probesArea = areas[0], areas[1]
	text, _ := layout.Measure(data.Probes, area.Width)
	diagnostics.probes.SetWidth(max(1, areas[1].Width))
	diagnostics.probes.SetHeight(max(1, areas[1].Height))
	diagnostics.probes.SetContent(text)
}

func (diagnostics *Diagnostics) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "r":
			return diagnostics.receiver.Diagnose()
		case "pgup":
			diagnostics.probes.PageUp()
		case "pgdown":
			diagnostics.probes.PageDown()
		}
	}
	return nil
}

func (diagnostics *Diagnostics) View(Snapshot) Content {
	data := diagnosticsViewData{
		Heading: layout.Fit(diagnostics.heading, diagnostics.headingArea),
		Probes:  layout.Fit(diagnostics.probes.View(), diagnostics.probesArea),
	}
	var content bytes.Buffer
	if err := diagnosticsTemplate.Execute(&content, data); err != nil {
		panic("render diagnostics template: " + err.Error())
	}
	return Content{Body: layout.Fit(strings.TrimSuffix(content.String(), "\n"), diagnostics.area)}
}
