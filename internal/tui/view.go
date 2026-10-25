package tui

import (
	"bytes"
	"strings"
	"text/template"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"wslproxy/internal/tui/layout"
	"wslproxy/internal/tui/screens"
)

const (
	minTerminalWidth  = 40
	minTerminalHeight = 20
	maxFrameWidth     = 72
)

// Blocks contain their own padding, but no joining newline. The template owns
// separators; View removes only the final separator before adding the frame.
const viewTemplateText = `{{if .Header}}{{.Header}}
{{end}}{{if .Body}}{{.Body}}
{{end}}{{if .Messages}}{{.Messages}}
{{end}}{{if .Help}}{{.Help}}
{{end}}{{define "header"}}{{if .Horizontal}}{{.Horizontal}}{{else}}{{.Title}}
{{.Status}}{{end}}
{{.Subtitle}}{{end}}{{define "messages"}}{{if .Busy}}{{.Busy}}
{{end}}{{if .Error}}{{.Error}}
{{end}}{{if .Notice}}{{.Notice}}
{{end}}{{end}}{{define "small-terminal"}}WSL PROXY
Resize terminal to at least {{.Width}} x {{.Height}}.
Ctrl+C to exit.{{end}}`

var viewTemplate = template.Must(template.New("tui").Parse(viewTemplateText))

type headerViewData struct{ Title, Status, Subtitle, Horizontal string }
type messagesViewData struct{ Busy, Error, Notice string }
type viewData struct {
	Header, Body, Messages, Help string
	heading                      headerViewData
	width                        int
	compact                      bool
}
type viewLayout struct {
	outer                        layout.Size
	header, body, messages, help layout.Size
	headerText, messageText      string
	compact                      bool
}

// View returns the root presentation to Bubble Tea without changing model or
// widget state. It executes the root template with m.presentation, prepared by
// refreshView, removes the final template separator, and applies the frame
// unless the presentation is compact.
//
// For terminals below the minimum dimensions, View renders a bounded resize
// warning instead. The returned tea.View configures terminal presentation,
// including the alternate screen; Bubble Tea handles displaying the result.
func (m *model) View() tea.View {
	data := m.presentation
	if m.width < minTerminalWidth || m.height < minTerminalHeight {
		var content bytes.Buffer
		if err := viewTemplate.ExecuteTemplate(&content, "small-terminal", layout.Size{Width: minTerminalWidth, Height: minTerminalHeight}); err != nil {
			panic("render small terminal template: " + err.Error())
		}
		view := tea.NewView(layout.Fit(content.String(), layout.Size{Width: m.width, Height: m.height}))
		view.AltScreen = true
		return view
	}
	var content bytes.Buffer
	if err := viewTemplate.Execute(&content, data); err != nil {
		panic("render TUI template: " + err.Error())
	}
	text := strings.TrimSuffix(content.String(), "\n")
	if !data.compact {
		text = frameStyle.Width(data.width).Render(text)
	}
	view := tea.NewView(text)
	view.AltScreen, view.WindowTitle = true, "WSL Proxy"
	return view
}

// refreshView prepares and caches the presentation after state changes.
// New calls it once during initialization. Update defers it so preparation runs
// after each event handler, including early returns and events that change
// content without changing terminal dimensions.
//
// It prepares styled root data, calculates block areas, and builds a snapshot
// for the active screen. Resize applies the allocated body area and current
// snapshot before the screen's View produces its content. This ordering updates
// stateful widget geometry, such as pagination and input scrolling, before the
// next input event and keeps the root View free of widget mutations.
//
// The rendered screen body and root blocks are fitted to their allocated areas
// and saved together in m.presentation for the next root View call. Only the
// active screen is resized and rendered.
//
// If no screen is active or the terminal is below the minimum dimensions,
// refreshView leaves the cached presentation unchanged. If root layout reports
// insufficient space, it caches a compact warning without rendering the screen.
func (m *model) refreshView() {
	if m.active == nil || m.width < minTerminalWidth || m.height < minTerminalHeight {
		return
	}
	data := m.prepareViewData()
	areas := m.calculateLayout(data)
	if areas.compact {
		m.presentation = viewData{Header: layout.Fit("Resize terminal to show the interface. Ctrl+C: exit", areas.outer), compact: true}
		return
	}
	snapshot := screens.Snapshot{Config: m.config, ProxyActive: m.running != nil}
	if m.active == m.runningView {
		if m.running != nil {
			snapshot.Running = &screens.RunningSnapshot{Config: m.running.Config}
			if m.running.Server != nil {
				snapshot.Running.Stats = m.running.Server.Snapshot()
			}
		}
		snapshot.RecentEvents = m.logs.Lines()
	}
	m.active.Resize(areas.body, snapshot)
	screen := m.active.View(snapshot)
	data.Header = layout.Fit(areas.headerText, areas.header)
	data.Body = layout.Fit(screen.Body, areas.body)
	data.Messages = layout.Fit(areas.messageText, areas.messages)
	data.Help = layout.Fit(data.Help, areas.help)
	data.width = areas.outer.Width
	m.presentation = data
}

// prepareViewData converts current application state into styled root view data.
// It prepares proxy status, the active screen's help, and optional busy, error,
// and notice messages, using the named messages template for their structure.
//
// The result is input to calculateLayout: block dimensions and the active
// screen's body are not assigned here. This method does not resize widgets or
// update m.presentation.
func (m *model) prepareViewData() viewData {
	status := "STOPPED"
	if m.running != nil {
		status = "RUNNING"
	}
	data := viewData{heading: headerViewData{Title: titleStyle.Render("WSL PROXY"), Status: accent.Render(status), Subtitle: muted.Render("Windows -> WSL2 HTTP Reverse Proxy")}}
	help := m.active.Help()
	if m.busy != "" {
		help = "Ctrl+C: cancel and exit"
	}
	data.Help = muted.Render(help)
	messages := messagesViewData{}
	if m.busy != "" {
		messages.Busy = blockStyle.Render(accent.Render(m.busy))
	}
	if m.error != "" {
		messages.Error = blockStyle.Render(danger.Render(safe(m.error)))
	}
	if m.notice != "" {
		messages.Notice = blockStyle.Render(muted.Render(safe(m.notice)))
	}
	var content bytes.Buffer
	if err := viewTemplate.ExecuteTemplate(&content, "messages", messages); err != nil {
		panic("render messages template: " + err.Error())
	}
	data.Messages = strings.TrimSuffix(content.String(), "\n")
	return data
}

// calculateLayout allocates root block areas from terminal dimensions and
// prepared view data without changing model or widget state. It caps the frame
// width and subtracts the frame's borders and padding to obtain the inner area.
//
// The title and status share a row when they fit; otherwise the header template
// stacks them. The rendered header and help are measured at the available width.
// Messages are limited to preserve at least one body row, then layout.Column
// assigns fixed block heights and gives the remaining height to the body.
//
// The result includes the allocated areas and the header and message text used
// during measurement, allowing refreshView to use the same content. A compact
// result signals that the header and help would leave no room for the body.
// This method does not resize or render the active screen.
func (m *model) calculateLayout(data viewData) viewLayout {
	outer := layout.Size{Width: min(maxFrameWidth, m.width), Height: m.height}
	frameWidth, frameHeight := frameStyle.GetFrameSize()
	inner := layout.Size{Width: max(0, outer.Width-frameWidth), Height: max(0, outer.Height-frameHeight)}
	heading := data.heading
	if lipgloss.Width(heading.Title)+lipgloss.Width(heading.Status) <= inner.Width {
		columns := layout.Row(layout.Size{Width: inner.Width, Height: 1}, layout.Block{Text: heading.Title}, layout.Block{Grow: true}, layout.Block{Text: heading.Status})
		heading.Horizontal = lipgloss.JoinHorizontal(lipgloss.Top, layout.Fit(heading.Title, columns[0]), layout.Fit("", columns[1]), layout.Fit(heading.Status, columns[2]))
	}
	var content bytes.Buffer
	if err := viewTemplate.ExecuteTemplate(&content, "header", heading); err != nil {
		panic("render header template: " + err.Error())
	}
	header := blockStyle.Render(content.String())
	_, headerSize := layout.Measure(header, inner.Width)
	_, helpSize := layout.Measure(data.Help, inner.Width)
	if headerSize.Height+helpSize.Height >= inner.Height {
		return viewLayout{outer: outer, compact: true}
	}
	// Keep at least one content row, shortening messages before navigation.
	messageArea := layout.Size{Width: inner.Width, Height: max(0, inner.Height-headerSize.Height-helpSize.Height-1)}
	_, messageSize := layout.Measure(data.Messages, inner.Width)
	messageArea.Height = min(messageArea.Height, messageSize.Height)
	messages := layout.Fit(data.Messages, messageArea)
	areas := layout.Column(inner, layout.Block{Text: header}, layout.Block{Grow: true}, layout.Block{Text: messages}, layout.Block{Text: data.Help})
	return viewLayout{outer: outer, header: areas[0], body: areas[1], messages: areas[2], help: areas[3], headerText: header, messageText: messages}
}
