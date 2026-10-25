package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"wslproxy/internal/app"
	"wslproxy/internal/tui/layout"
	"wslproxy/internal/tui/screens"
)

func TestTemplateOnlyOrdersPreparedBlocks(t *testing.T) {
	data := viewData{Header: "header", Body: "body {{.Status}}", Messages: "messages", Help: "help"}
	var out bytes.Buffer
	if err := viewTemplate.Execute(&out, data); err != nil {
		t.Fatal(err)
	}
	if out.String() != "header\nbody {{.Status}}\nmessages\nhelp\n" {
		t.Fatalf("template: %q", out.String())
	}
}

type snapshotScreen struct {
	recordingScreen
	snapshot screens.Snapshot
	area     layout.Size
	calls    int
}

func (s *snapshotScreen) Resize(area layout.Size, snapshot screens.Snapshot) {
	s.area, s.snapshot = area, snapshot
}
func (s *snapshotScreen) View(snapshot screens.Snapshot) screens.Content {
	s.snapshot = snapshot
	s.calls++
	return screens.Content{Body: "body {{.Status}}"}
}

func TestPresentationPreparedAfterEventsAndViewIsReadOnly(t *testing.T) {
	m := newTestModel(t)
	screen := &snapshotScreen{}
	m.setActive(screen)
	m.running = &app.Running{}
	m.busy, m.error, m.notice = "working", "bad\x00error", "new\x1bnotice"
	m.Update(struct{}{})
	if screen.calls != 1 || screen.snapshot.Config != m.config || !screen.snapshot.ProxyActive || screen.snapshot.Running != nil {
		t.Fatal("unexpected screen snapshot")
	}
	if screen.area.Width <= 0 || screen.area.Height <= 0 {
		t.Fatal("no screen allocation")
	}
	first := m.View()
	second := m.View()
	if screen.calls != 1 || first.Content != second.Content || !first.AltScreen || first.WindowTitle != "WSL Proxy" {
		t.Fatal("View changed presentation or options")
	}
	text := ansi.Strip(first.Content)
	for _, expected := range []string{"body {{.Status}}", "bad error", "new notice", "Ctrl+C: cancel and exit"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q: %s", expected, text)
		}
	}
}

func TestAllScreensFitTerminalAndKeepFrameAndHelp(t *testing.T) {
	for _, size := range []layout.Size{{Width: 40, Height: 20}, {Width: 41, Height: 21}, {Width: 60, Height: 24}, {Width: 80, Height: 30}, {Width: 120, Height: 60}} {
		m := newTestModel(t)
		m.config.Distribution = "Очень-длинное-имя-дистрибутива-世界"
		m.form.Open(m.config)
		m.distributions.SetNames([]string{"Ubuntu", "Debian", strings.Repeat("世界", 50)})
		m.logs.lines = []string{strings.Repeat("recent event ", 30), "last event"}
		for _, screen := range []screenComponent{m.menu, m.form, m.distributions, m.diagnostics, m.runningView} {
			m.setActive(screen)
			for _, messages := range []bool{false, true} {
				m.error, m.notice, m.busy = "", "", ""
				if messages {
					m.error = strings.Repeat("long error 世界 ", 50)
					m.notice = "notice"
					m.busy = "working"
				}
				m.Update(tea.WindowSizeMsg{Width: size.Width, Height: size.Height})
				got := m.View().Content
				if lipgloss.Width(got) > size.Width || lipgloss.Height(got) > size.Height {
					t.Fatalf("%T overflow %v: %dx%d\n%s", screen, size, lipgloss.Width(got), lipgloss.Height(got), got)
				}
				plain := ansi.Strip(got)
				if !strings.Contains(plain, "╰") || !strings.Contains(plain, "Esc:") && !strings.Contains(plain, "Ctrl+C:") {
					t.Fatalf("lost border/help %T %v:\n%s", screen, size, plain)
				}
			}
		}
	}
}

func TestMessagesReallocateScreenWithoutResize(t *testing.T) {
	m := newTestModel(t)
	screen := &snapshotScreen{}
	m.setActive(screen)
	m.Update(struct{}{})
	before := screen.area
	m.error = strings.Repeat("failure ", 30)
	m.Update(struct{}{})
	if screen.area.Height >= before.Height {
		t.Fatal("message did not reduce screen area")
	}
	m.error = ""
	m.Update(struct{}{})
	if screen.area != before {
		t.Fatal("screen did not regain available space")
	}
}

func TestSmallTerminalIsBounded(t *testing.T) {
	for _, size := range []layout.Size{{Width: 39, Height: 30}, {Width: 80, Height: 19}, {Width: 1, Height: 1}, {Width: 0, Height: 0}, {Width: 20, Height: 10}} {
		m := &model{width: size.Width, height: size.Height}
		view := m.View()
		if !view.AltScreen || lipgloss.Width(view.Content) > size.Width || view.Content != "" && lipgloss.Height(view.Content) > size.Height {
			t.Fatalf("small view: %+v", view)
		}
	}
}

func TestRootTemplateOmitsAbsentBlocks(t *testing.T) {
	for _, test := range []struct {
		data viewData
		want string
	}{
		{viewData{}, ""},
		{viewData{Header: "header", Body: "body", Help: "help"}, "header\nbody\nhelp"},
		{viewData{Messages: "message {{.Body}}", Help: "help"}, "message {{.Body}}\nhelp"},
		{viewData{Header: "compact", compact: true}, "compact"},
	} {
		var content bytes.Buffer
		if err := viewTemplate.Execute(&content, test.data); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSuffix(content.String(), "\n"); got != test.want {
			t.Fatalf("got %q, want %q", got, test.want)
		}
	}
}

func TestMenuNoteUpdatesAfterEventWithoutWindowResize(t *testing.T) {
	m := newTestModel(t)
	m.setActive(m.menu)
	m.Update(struct{}{})
	if strings.Contains(m.View().Content, "Proxy remains active") {
		t.Fatal("unexpected note")
	}
	m.running = &app.Running{}
	m.Update(struct{}{})
	if !strings.Contains(m.View().Content, "Proxy remains active") {
		t.Fatal("missing updated note")
	}
	m.running = nil
	m.Update(struct{}{})
	if strings.Contains(m.View().Content, "Proxy remains active") {
		t.Fatal("stale note")
	}
}
