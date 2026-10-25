package screens

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
	"wslproxy/internal/tui/layout"
	"wslproxy/internal/wsl"
)

func TestScreensRespectAllocatedArea(t *testing.T) {
	receiver := &recordingReceiver{}
	menu, form, distributions, diagnostics, running := NewMenu(receiver), NewForm(receiver), NewDistributions(receiver), NewDiagnostics(receiver), NewRunning(receiver)
	cfg := config.Default()
	cfg.Distribution = strings.Repeat("世界", 30)
	form.Open(cfg)
	form.fields[0].SetValue(strings.Repeat("input", 40))
	distributions.SetNames([]string{strings.Repeat("name", 30), "Ubuntu", "Debian"})
	report := app.Report{Selected: netip.MustParseAddr("172.28.1.4")}
	for i := 0; i < 20; i++ {
		report.Probes = append(report.Probes, app.Probe{Candidate: wsl.Candidate{IP: report.Selected, Interface: strings.Repeat("eth0", 30)}, Error: strings.Repeat("failure", 30)})
	}
	diagnostics.SetReport(report)
	snapshot := Snapshot{Config: cfg, ProxyActive: true, Running: &RunningSnapshot{Config: cfg, Stats: proxy.Snapshot{Upstream: "172.28.1.4"}}, RecentEvents: []string{strings.Repeat("event ", 50), "last event"}}
	screens := []interface {
		Resize(layout.Size, Snapshot)
		View(Snapshot) Content
	}{menu, form, distributions, diagnostics, running}
	for _, screen := range screens {
		for _, w := range []int{0, 1, 8, 20, 34, 66, 100} {
			for _, h := range []int{0, 1, 2, 4, 8, 20, 60} {
				area := layout.Size{Width: w, Height: h}
				screen.Resize(area, snapshot)
				got := screen.View(snapshot).Body
				if w == 0 || h == 0 {
					if got != "" {
						t.Fatalf("%T nonempty zero area", screen)
					}
					continue
				}
				if lipgloss.Width(got) > w || lipgloss.Height(got) > h {
					t.Fatalf("%T exceeded %v: %q", screen, area, got)
				}
			}
		}
	}
}

func TestFormKeepsFocusedInputAndDraftAcrossLayouts(t *testing.T) {
	form := NewForm(&recordingReceiver{})
	cfg := config.Default()
	form.Open(cfg)
	form.fields[0].SetValue("draft")
	for focus := range form.fields {
		if focus > 0 {
			form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		}
		form.fields[focus].SetValue("focused-value")
		for _, area := range []layout.Size{{Width: 28, Height: 5}, {Width: 66, Height: 30}, {Width: 28, Height: 1}, {Width: 40, Height: 8}} {
			form.Resize(area, Snapshot{})
			body := ansi.Strip(form.View(Snapshot{}).Body)
			if !strings.Contains(body, "focused-value") {
				t.Fatalf("focused input %d missing at %v: %q", focus, area, body)
			}
			if form.focus != focus || !form.fields[focus].Focused() {
				t.Fatal("layout changed focus")
			}
		}
	}
	if form.fields[0].Value() != "focused-value" {
		t.Fatal("layout modified draft")
	}
}

func TestListSelectionAndPaginationSurviveResize(t *testing.T) {
	menu := NewMenu(&recordingReceiver{})
	for i := 0; i < len(menuItems)-1; i++ {
		menu.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	for _, height := range []int{1, 2, 5, 20, 3, 12, 0, 20} {
		menu.Resize(layout.Size{Width: 40, Height: height}, Snapshot{ProxyActive: true})
		if menu.choices.Index() != len(menuItems)-1 {
			t.Fatal("menu selection reset")
		}
		if height >= 1 && !strings.Contains(ansi.Strip(menu.View(Snapshot{ProxyActive: true}).Body), "Exit") {
			t.Fatalf("selected item hidden at height %d: %s", height, menu.View(Snapshot{ProxyActive: true}).Body)
		}
	}
	distributions := NewDistributions(&recordingReceiver{})
	names := make([]string, 20)
	for i := range names {
		names[i] = fmt.Sprintf("Distribution %02d", i)
	}
	distributions.SetNames(names)
	distributions.Resize(layout.Size{Width: 40, Height: 8}, Snapshot{})
	for i := 0; i < 15; i++ {
		distributions.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	for _, height := range []int{1, 3, 6, 20, 8} {
		distributions.Resize(layout.Size{Width: 40, Height: height}, Snapshot{})
		if distributions.choices.Index() != 15 || !strings.Contains(ansi.Strip(distributions.View(Snapshot{}).Body), names[15]) {
			t.Fatal("distribution selection/page lost")
		}
	}
	distributions.SetNames(names)
	if distributions.choices.Index() != 0 {
		t.Fatal("refresh did not reset selection as before")
	}
}

func TestDiagnosticsScrollsAllResultsAndPreservesOffset(t *testing.T) {
	diagnostics := NewDiagnostics(nil)
	report := app.Report{}
	for i := 0; i < 25; i++ {
		report.Probes = append(report.Probes, app.Probe{Candidate: wsl.Candidate{IP: netip.MustParseAddr("172.28.1.4"), Interface: fmt.Sprintf("probe-%02d", i)}})
	}
	diagnostics.SetReport(report)
	area := layout.Size{Width: 60, Height: 10}
	snapshot := Snapshot{Config: config.Default()}
	diagnostics.Resize(area, snapshot)
	if !strings.Contains(diagnostics.View(snapshot).Body, "probe-00") {
		t.Fatal("missing first result")
	}
	diagnostics.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	offset := diagnostics.probes.YOffset()
	diagnostics.Resize(area, snapshot)
	if offset == 0 || diagnostics.probes.YOffset() != offset {
		t.Fatal("layout reset scroll")
	}
	for i := 0; i < 25; i++ {
		diagnostics.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		diagnostics.Resize(area, snapshot)
	}
	if !strings.Contains(diagnostics.View(snapshot).Body, "probe-24") {
		t.Fatal("last result inaccessible")
	}
	diagnostics.SetReport(report)
	diagnostics.Resize(area, snapshot)
	if diagnostics.probes.YOffset() != 0 {
		t.Fatal("new report retained old scroll")
	}
}

func TestRunningUsesSpaceForNewestEvents(t *testing.T) {
	running := NewRunning(nil)
	cfg := config.Default()
	snapshot := Snapshot{Running: &RunningSnapshot{Config: cfg}, RecentEvents: []string{"oldest", "second", "third", "newest"}}
	running.Resize(layout.Size{Width: 60, Height: 11}, snapshot)
	small := running.View(snapshot).Body
	if !strings.Contains(small, "newest") || strings.Contains(small, "oldest") {
		t.Fatalf("wrong compact events: %q", small)
	}
	running.Resize(layout.Size{Width: 60, Height: 20}, snapshot)
	if !strings.Contains(running.View(snapshot).Body, "oldest") {
		t.Fatal("extra height did not reveal events")
	}
	snapshot.RecentEvents = []string{strings.Repeat("long event ", 40), "newest"}
	running.Resize(layout.Size{Width: 25, Height: 14}, snapshot)
	if !strings.Contains(running.View(snapshot).Body, "newest") {
		t.Fatal("wrapped event displaced newest event")
	}
}

func TestFormResizeKeepsLongInputCursorVisible(t *testing.T) {
	form := NewForm(&recordingReceiver{})
	form.Open(config.Default())
	value := strings.Repeat("0123456789", 20) + "TAIL"
	form.fields[0].SetValue(value)
	form.fields[0].CursorEnd()
	position := form.fields[0].Position()
	for _, width := range []int{60, 20, 40, 12, 60} {
		form.Resize(layout.Size{Width: width, Height: 1}, Snapshot{})
		input := form.fields[0].View()
		if lipgloss.Width(input) > width || lipgloss.Height(input) != 1 {
			t.Fatalf("input did not fit width %d: %q", width, input)
		}
		if !strings.Contains(ansi.Strip(form.View(Snapshot{}).Body), "TAIL") {
			t.Fatalf("cursor end hidden at width %d", width)
		}
		if form.fields[0].Position() != position || form.fields[0].Value() != value {
			t.Fatal("layout moved cursor or changed value")
		}
	}
}

func TestScreenContentChangesWithoutResize(t *testing.T) {
	receiver := &recordingReceiver{}
	area := layout.Size{Width: 60, Height: 8}
	snapshot := Snapshot{Config: config.Default()}

	distributions := NewDistributions(receiver)
	distributions.Resize(area, snapshot)
	distributions.SetNames([]string{"New distribution", "Second"})
	distributions.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if !strings.Contains(distributions.View(snapshot).Body, "Second") || distributions.choices.Index() != 1 {
		t.Fatal("new distributions were not sized for selection")
	}

	form := NewForm(receiver)
	form.Open(snapshot.Config)
	form.Resize(layout.Size{Width: 35, Height: 3}, snapshot)
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	form.SelectDistribution("New distribution")
	text := ansi.Strip(form.View(snapshot).Body)
	if !strings.Contains(text, "New distribution") || !strings.Contains(text, "Field 3/8") {
		t.Fatalf("form did not recalculate for its focused field: %q", text)
	}
	if form.focus != distributionFieldIndex || !form.fields[distributionFieldIndex].Focused() {
		t.Fatal("focus lost")
	}

	diagnostics := NewDiagnostics(receiver)
	diagnostics.Resize(area, snapshot)
	report := app.Report{Selected: netip.MustParseAddr("172.28.1.4")}
	for i := 0; i < 20; i++ {
		report.Probes = append(report.Probes, app.Probe{Candidate: wsl.Candidate{IP: report.Selected, Interface: fmt.Sprintf("new-%02d", i)}})
	}
	diagnostics.SetReport(report)
	if !strings.Contains(diagnostics.View(snapshot).Body, "Selected: 172.28.1.4") || !strings.Contains(diagnostics.View(snapshot).Body, "new-00") {
		t.Fatal("report content did not refresh without Resize")
	}
	for i := 0; i < 20; i++ {
		diagnostics.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if !strings.Contains(diagnostics.View(snapshot).Body, "new-19") {
		t.Fatal("new report is not scrollable")
	}
	diagnostics.SetReport(app.Report{})
	if diagnostics.probes.YOffset() != 0 || strings.Contains(diagnostics.View(snapshot).Body, "new-") || strings.Contains(diagnostics.View(snapshot).Body, "Selected:") {
		t.Fatal("clearing the report left stale content")
	}

	running := NewRunning(receiver)
	running.Resize(layout.Size{Width: 60, Height: 14}, Snapshot{})
	snapshot.Running = &RunningSnapshot{Config: snapshot.Config, Stats: proxy.Snapshot{Requests: 123}}
	snapshot.RecentEvents = []string{"first", "fresh event"}
	text = running.View(snapshot).Body
	if !strings.Contains(text, "Requests: 123") || !strings.Contains(text, "fresh event") {
		t.Fatal("running snapshot was not refreshed")
	}
	snapshot.RecentEvents = nil
	if strings.Contains(running.View(snapshot).Body, "Recent events") {
		t.Fatal("empty events retained their heading")
	}
}
