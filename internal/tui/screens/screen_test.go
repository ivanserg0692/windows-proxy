package screens

import (
	"bytes"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"text/template"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
	"wslproxy/internal/tui/layout"
	"wslproxy/internal/wsl"
)

// recordingReceiver records synchronous screen requests without running services.
type recordingReceiver struct {
	calls   []string
	cmd     tea.Cmd
	config  config.Config
	name    string
	forForm bool
	err     error
}

func (r *recordingReceiver) record(action string) tea.Cmd {
	r.calls = append(r.calls, action)
	return r.cmd
}
func (r *recordingReceiver) ToggleProxy() tea.Cmd { return r.record("toggle") }
func (r *recordingReceiver) OpenForm() tea.Cmd    { return r.record("form") }
func (r *recordingReceiver) ChooseDistribution(forForm bool) tea.Cmd {
	r.forForm = forForm
	return r.record("choose")
}
func (r *recordingReceiver) Diagnose() tea.Cmd { return r.record("diagnose") }
func (r *recordingReceiver) Quit() tea.Cmd     { return r.record("quit") }
func (r *recordingReceiver) SaveConfig(cfg config.Config) tea.Cmd {
	r.config = cfg
	return r.record("save")
}
func (r *recordingReceiver) ShowError(err error) tea.Cmd {
	r.err = err
	return r.record("error")
}
func (r *recordingReceiver) RefreshDistributions() tea.Cmd { return r.record("refresh") }
func (r *recordingReceiver) SelectDistribution(name string, forForm bool) tea.Cmd {
	r.name, r.forForm = name, forForm
	return r.record("select")
}
func (r *recordingReceiver) StopProxy() tea.Cmd { return r.record("stop") }

func TestFormsOwnIndependentDraftsAndRequestSave(t *testing.T) {
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	receiver := &recordingReceiver{}
	first, second := NewForm(receiver), NewForm(&recordingReceiver{})
	first.Open(cfg)
	second.Open(cfg)
	first.fields[0].SetValue("127.0.0.2")
	first.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd := first.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if len(receiver.calls) != 1 || receiver.calls[0] != "save" || cmd != nil || receiver.config.ListenAddress != "127.0.0.2" {
		t.Fatal("expected synchronous save request with the edited configuration")
	}
	if second.fields[0].Value() != cfg.ListenAddress || second.focus != 0 || cfg.ListenAddress == "127.0.0.2" {
		t.Fatal("editing one form affected another form or its input configuration")
	}
}

func TestFormReportsInvalidPortWithoutSaving(t *testing.T) {
	receiver := &recordingReceiver{}
	form := NewForm(receiver)
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	form.Open(cfg)
	form.fields[1].SetValue("65536")
	if _, err := form.configuration(); err == nil {
		t.Fatal("invalid port accepted")
	}
	form.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if len(receiver.calls) != 1 || receiver.calls[0] != "error" || receiver.err == nil {
		t.Fatal("invalid configuration did not report an error without saving")
	}
	form.fields[1].SetValue("8080")
	form.fields[4].SetValue("172.28.1.4")
	cfg, err := form.configuration()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManualTargetIP != "172.28.1.4" {
		t.Fatalf("manual IP missing: %+v", cfg)
	}
}

func TestDiagnosticScreenRendersAllSafeReportWhenItFits(t *testing.T) {
	diagnostics := NewDiagnostics(nil)
	report := app.Report{Selected: netip.MustParseAddr("172.28.1.4")}
	for index := 0; index < 5; index++ {
		report.Probes = append(report.Probes, app.Probe{
			Candidate: wsl.Candidate{IP: report.Selected, Interface: "eth0\x1b[2J"},
			Duration:  time.Millisecond, Error: "failure\x00",
		})
	}
	diagnostics.SetReport(report)
	diagnostics.Resize(layout.Size{Width: 80, Height: 25}, Snapshot{Config: config.Default()})
	content := diagnostics.View(Snapshot{Config: config.Default()})
	if strings.Count(content.Body, "FAIL:") != 5 || !strings.Contains(content.Body, "Selected: 172.28.1.4") {
		t.Fatalf("unexpected report: %q", content.Body)
	}
	if strings.ContainsAny(content.Body, "\x1b\x00") {
		t.Fatal("terminal controls leaked from the report")
	}
}

func TestRunningScreenUsesValueSnapshotAndAvailableEventSpace(t *testing.T) {
	running := NewRunning(nil)
	running.Resize(layout.Size{Width: 80, Height: 20}, Snapshot{})
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	content := running.View(Snapshot{
		Running:      &RunningSnapshot{Config: cfg, Stats: proxy.Snapshot{Upstream: "172.28.1.4", ActiveConnections: 5, Requests: 12, Retries: 3, Failures: 2}},
		RecentEvents: []string{"oldest event", "second event", "third event", "last event"},
	})
	for _, expected := range []string{"Ubuntu", "172.28.1.4", "Active connections: 5", "Requests: 12", "Retries:  3", "Failures: 2"} {
		if !strings.Contains(content.Body, expected) {
			t.Fatalf("status is missing %q: %q", expected, content.Body)
		}
	}
	if !strings.Contains(content.Body, "oldest event") {
		t.Fatalf("unexpected recent events: %q", content.Body)
	}
	previous := -1
	for _, event := range []string{"oldest event", "second event", "third event", "last event"} {
		position := strings.Index(content.Body, event)
		if position <= previous {
			t.Fatalf("missing or out-of-order event %q: %q", event, content.Body)
		}
		previous = position
	}
}

func TestRunningScreenRefreshesActiveConnections(t *testing.T) {
	running := NewRunning(nil)
	running.Resize(layout.Size{Width: 80, Height: 20}, Snapshot{})
	snapshot := Snapshot{Running: &RunningSnapshot{Config: config.Default()}}
	for _, count := range []int64{0, 8, 3, 0} {
		snapshot.Running.Stats.ActiveConnections = count
		body := running.View(snapshot).Body
		if want := fmt.Sprintf("Active connections: %d", count); !strings.Contains(body, want) {
			t.Fatalf("missing %q in %q", want, body)
		}
	}
}

func TestMenuProxyLabelPreservesSelectionAndNavigation(t *testing.T) {
	menu := NewMenu(&recordingReceiver{})
	area := layout.Size{Width: 80, Height: 20}
	for selected := range menuItems {
		menu.choices.Select(selected)
		for _, active := range []bool{false, true, false, true, false} {
			snapshot := Snapshot{ProxyActive: active}
			menu.Resize(area, snapshot)
			want := "Start proxy"
			if active {
				want = "Stop proxy"
			}
			if label := menu.choices.Items()[0].(item).Title(); label != want {
				t.Fatalf("active=%v: label=%q, want %q", active, label, want)
			}
			if menu.choices.Index() != selected || len(menu.choices.Items()) != len(menuItems) {
				t.Fatal("proxy state changed menu selection or item count")
			}
			if body := menu.View(snapshot).Body; !strings.Contains(body, want) {
				t.Fatalf("menu did not render %q: %q", want, body)
			}
		}
	}
	menu.choices.Select(0)
	menu.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if menu.choices.Index() != 1 {
		t.Fatal("Down did not select configuration")
	}
	menu.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if menu.choices.Index() != 0 {
		t.Fatal("Up did not return to the proxy action")
	}
}

func TestFormDistributionSelectionRetainsFocusAndDraft(t *testing.T) {
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	receiver := &recordingReceiver{}
	form := NewForm(receiver)
	form.Open(cfg)
	form.fields[0].SetValue("127.0.0.2")
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(receiver.calls) != 1 || receiver.calls[0] != "choose" || !receiver.forForm || cmd != nil {
		t.Fatal("distribution field did not request synchronous selection for the form")
	}
	form.SelectDistribution("Debian")
	if form.focus != 2 || !form.fields[2].Focused() || form.fields[0].Value() != "127.0.0.2" || form.fields[2].Value() != "Debian" {
		t.Fatal("distribution selection lost the draft or focused field")
	}
}

func TestDistributionScreensOwnSelectionAndReportItsPurpose(t *testing.T) {
	receiver := &recordingReceiver{}
	formSelector, menuSelector := NewDistributions(receiver), NewDistributions(receiver)
	formSelector.SetNames([]string{"Ubuntu", "Debian"})
	menuSelector.SetNames([]string{"Ubuntu", "Debian"})
	formSelector.Resize(layout.Size{Width: 60, Height: 20}, Snapshot{})
	menuSelector.Resize(layout.Size{Width: 60, Height: 20}, Snapshot{})
	formSelector.Open(true)
	menuSelector.Open(false)
	formSelector.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	for _, tc := range []struct {
		screen  *Distributions
		name    string
		forForm bool
	}{{formSelector, "Debian", true}, {menuSelector, "Ubuntu", false}} {
		receiver.calls = nil
		cmd := tc.screen.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if len(receiver.calls) != 1 || receiver.calls[0] != "select" || cmd != nil || receiver.name != tc.name || receiver.forForm != tc.forForm {
			t.Fatal("selection state or purpose leaked between screens")
		}
	}
}

func TestScreensReturnReceiverCommandsWithoutExecutingThem(t *testing.T) {
	for _, action := range []string{"toggle", "form", "choose", "diagnose", "quit", "refresh", "stop", "stop uppercase", "rerun"} {
		t.Run(action, func(t *testing.T) {
			executed := false
			receiver := &recordingReceiver{cmd: func() tea.Msg { executed = true; return action }}
			var cmd tea.Cmd
			want := action
			switch action {
			case "refresh":
				cmd = NewDistributions(receiver).Update(tea.KeyPressMsg{Code: 'r'})
			case "stop", "stop uppercase":
				key := 's'
				if action == "stop uppercase" {
					key = 'S'
					want = "stop"
				}
				cmd = NewRunning(receiver).Update(tea.KeyPressMsg{Code: key})
			case "rerun":
				want = "diagnose"
				cmd = NewDiagnostics(receiver).Update(tea.KeyPressMsg{Code: 'r'})
			default:
				menu := NewMenu(receiver)
				for index, name := range []string{"toggle", "form", "choose", "diagnose", "quit"} {
					if name == action {
						menu.choices.Select(index)
						break
					}
				}
				cmd = menu.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
			if executed || len(receiver.calls) != 1 || receiver.calls[0] != want || cmd == nil {
				t.Fatal("screen did not synchronously request the action and return its deferred command")
			}
			if cmd() != action || !executed {
				t.Fatal("receiver command was lost")
			}
		})
	}
}

// Template separators connect occupied blocks without adding empty rows. Data,
// including ANSI and template delimiters, must be emitted literally.
func TestScreenTemplatesOrderOptionalBlocks(t *testing.T) {
	for _, test := range []struct {
		name string
		tmpl *template.Template
		data any
		want string
	}{
		{"menu empty", menuTemplate, menuViewData{}, ""},
		{"menu list", menuTemplate, menuViewData{List: "世界 {{.Note}}"}, "世界 {{.Note}}"},
		{"menu note", menuTemplate, menuViewData{Note: "note"}, "note"},
		{"menu both", menuTemplate, menuViewData{List: "list\nsecond", Note: "note"}, "list\nsecond\nnote"},
		{"distributions list", distributionsTemplate, distributionsViewData{List: "Ubuntu"}, "Ubuntu"},
		{"distributions both", distributionsTemplate, distributionsViewData{Title: "title", List: "Ubuntu"}, "title\nUbuntu"},
		{"form input", formTemplate, formViewData{Fields: "field"}, "field"},
		{"form counter", formTemplate, formViewData{Counter: "counter"}, "counter"},
		{"form both", formTemplate, formViewData{Fields: "fields", Counter: "counter"}, "fields\ncounter"},
		{"diagnostics results", diagnosticsTemplate, diagnosticsViewData{Probes: "result"}, "result"},
		{"diagnostics both", diagnosticsTemplate, diagnosticsViewData{Heading: "heading", Probes: "result"}, "heading\nresult"},
		{"running empty", runningTemplate, runningViewData{}, ""},
		{"running stats", runningTemplate, runningViewData{Stats: "stats"}, "stats"},
		{"running all", runningTemplate, runningViewData{Stats: "stats", EventHeading: "events", Events: "\x1b[31m世界\x1b[m"}, "stats\nevents\n\x1b[31m世界\x1b[m"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var content bytes.Buffer
			if err := test.tmpl.Execute(&content, test.data); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSuffix(content.String(), "\n"); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
