package screens

import (
	"bytes"
	"strings"
	"text/template"

	"wslproxy/internal/tui/layout"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The live server remains owned by the root. This screen only renders snapshots.
type Running struct {
	receiver RunningReceiver
	area     layout.Size
}

func NewRunning(receiver RunningReceiver) *Running           { return &Running{receiver: receiver} }
func (running *Running) Help() string                        { return "S: stop   Esc: menu (proxy stays active)" }
func (running *Running) Resize(area layout.Size, _ Snapshot) { running.area = area }

func (running *Running) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "s" || key.String() == "S") {
		return running.receiver.StopProxy()
	}
	return nil
}

var runningTemplate = template.Must(template.New("running").Parse(`{{if .Stats}}{{.Stats}}
{{end}}{{if .EventHeading}}{{.EventHeading}}
{{end}}{{if .Events}}{{.Events}}
{{end}}{{define "stats"}}Windows:  {{.Endpoint}}
WSL:      {{.Distribution}}
Upstream: {{.Upstream}}:{{.Port}}

Active connections: {{.ActiveConnections}}
Requests: {{.Requests}}
Retries:  {{.Retries}}
Failures: {{.Failures}}{{end}}{{define "events"}}{{range $index, $event := .}}{{if $index}}
{{end}}{{$event}}{{end}}{{end}}`))

type runningStatsData struct {
	Endpoint, Distribution, Upstream string
	Port                             int
	Requests, Retries, Failures      uint64
	ActiveConnections                int64
}
type runningViewData struct{ Stats, EventHeading, Events string }
type runningLayout struct {
	statsArea, headingArea, eventsArea layout.Size
	firstEvent                         int
}

func (running *Running) prepareViewData(snapshot Snapshot) runningViewData {
	data := runningViewData{}
	if snapshot.Running != nil {
		cfg, stats := snapshot.Running.Config, snapshot.Running.Stats
		values := runningStatsData{Endpoint: cfg.ListenEndpoint(), Distribution: safe(cfg.Distribution),
			Upstream: stats.Upstream, Port: cfg.TargetPort, ActiveConnections: stats.ActiveConnections,
			Requests: stats.Requests, Retries: stats.Retries, Failures: stats.Failures}
		var content bytes.Buffer
		if err := runningTemplate.ExecuteTemplate(&content, "stats", values); err != nil {
			panic("render running stats template: " + err.Error())
		}
		data.Stats = content.String()
	}
	if len(snapshot.RecentEvents) > 0 {
		data.EventHeading = lipgloss.NewStyle().MarginTop(1).Render(muted.Render("Recent events"))
	}
	return data
}

func (running *Running) calculateLayout(data runningViewData, events []string) runningLayout {
	areas := layout.Column(running.area, layout.Block{Text: data.Stats}, layout.Block{Grow: true})
	result := runningLayout{statsArea: areas[0], firstEvent: len(events)}
	if len(events) == 0 || areas[1].Height <= 0 {
		return result
	}
	eventAreas := layout.Column(areas[1], layout.Block{Text: data.EventHeading}, layout.Block{Grow: true})
	if eventAreas[1].Height <= 0 {
		return result
	}
	result.headingArea, result.eventsArea = eventAreas[0], eventAreas[1]
	result.firstEvent = len(events) - 1
	_, lastSize := layout.Measure(safe(events[result.firstEvent]), result.eventsArea.Width)
	usedHeight := lastSize.Height
	for result.firstEvent > 0 {
		_, size := layout.Measure(safe(events[result.firstEvent-1]), result.eventsArea.Width)
		if usedHeight+size.Height > result.eventsArea.Height {
			break
		}
		usedHeight += size.Height
		result.firstEvent--
	}
	return result
}

func (running *Running) View(snapshot Snapshot) Content {
	data := running.prepareViewData(snapshot)
	areas := running.calculateLayout(data, snapshot.RecentEvents)
	var events bytes.Buffer
	if err := runningTemplate.ExecuteTemplate(&events, "events", snapshot.RecentEvents[areas.firstEvent:]); err != nil {
		panic("render running events template: " + err.Error())
	}
	data.Stats = layout.Fit(data.Stats, areas.statsArea)
	data.EventHeading = layout.Fit(data.EventHeading, areas.headingArea)
	data.Events = layout.Fit(muted.Render(safe(events.String())), areas.eventsArea)
	var content bytes.Buffer
	if err := runningTemplate.Execute(&content, data); err != nil {
		panic("render running template: " + err.Error())
	}
	return Content{Body: layout.Fit(strings.TrimSuffix(content.String(), "\n"), running.area)}
}
