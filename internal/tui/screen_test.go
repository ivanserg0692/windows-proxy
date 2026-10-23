package tui

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
	"wslproxy/internal/wsl"
)

func TestFormsOwnIndependentDraftsAndReturnSaveRequests(t *testing.T) {
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	first, second := &formScreen{}, &formScreen{}
	first.open(cfg)
	second.open(cfg)
	first.fields[0].SetValue("127.0.0.2")
	first.update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd, intent := first.update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	request, ok := intent.(saveConfigIntent)
	if !ok || cmd != nil || request.config.ListenAddress != "127.0.0.2" {
		t.Fatalf("expected synchronous save request, got %T", intent)
	}
	if second.fields[0].Value() != cfg.ListenAddress || second.focus != 0 || cfg.ListenAddress == "127.0.0.2" {
		t.Fatal("editing one form affected another form or its input configuration")
	}
}

func TestInvalidFormDoesNotStartAnOperation(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.config.Distribution = "Ubuntu"
	uiModel.menu.choices.Select(1)
	press(uiModel, tea.KeyEnter)
	uiModel.form.fields[1].SetValue("invalid")
	cmd := press(uiModel, 's', tea.ModCtrl)
	if cmd != nil || uiModel.busy != "" || uiModel.operation != 0 || uiModel.active != uiModel.form {
		t.Fatal("invalid form began an operation or navigated away")
	}
	if uiModel.error != "Windows listen port must be an integer" {
		t.Fatalf("unexpected validation error: %q", uiModel.error)
	}
}

func TestDiagnosticResultsAreDeliveredOffscreenAndResetBySave(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.operation = 1
	uiModel.busy = "diagnosing"
	canceled := false
	uiModel.cancel = func() { canceled = true }
	selected := netip.MustParseAddr("172.28.1.4")
	uiModel.Update(operationMsg{id: 1, report: app.Report{Selected: selected}, err: errors.New("partial report")})
	if uiModel.active != uiModel.menu || uiModel.busy != "" || !canceled || uiModel.cancel != nil {
		t.Fatal("background result changed navigation or left its operation active")
	}
	if uiModel.diagnostics.report.Selected != selected || uiModel.error != "partial report" {
		t.Fatal("background diagnostic report was lost")
	}
	saved := uiModel.config
	saved.Distribution = "Debian"
	uiModel.Update(operationMsg{id: 1, saved: &saved})
	if uiModel.config != saved || uiModel.diagnostics.report.Selected.IsValid() || uiModel.active != uiModel.menu {
		t.Fatal("save result did not update configuration and clear the old report")
	}
}

func TestMenuAndEscapeKeepProxyActive(t *testing.T) {
	uiModel := newTestModel(t)
	running := &app.Running{Config: uiModel.config}
	uiModel.Update(operationMsg{running: running})
	if uiModel.active != uiModel.runningView {
		t.Fatal("start result did not open status")
	}
	if cmd := press(uiModel, tea.KeyEscape); cmd != nil || uiModel.running != running || uiModel.active != uiModel.menu {
		t.Fatal("Escape stopped the proxy")
	}
	if !strings.Contains(uiModel.View().Content, "Proxy remains active") {
		t.Fatal("menu lost the active proxy notice")
	}
	if cmd := press(uiModel, tea.KeyEnter); cmd != nil || uiModel.running != running || uiModel.active != uiModel.runningView || uiModel.busy != "" {
		t.Fatal("opening active proxy status started another operation")
	}
	if cmd := press(uiModel, 'S'); cmd == nil || uiModel.busy != "Stopping HTTP server..." {
		t.Fatal("status did not synchronously request stop")
	}
}

func TestDiagnosticScreenRendersLimitedSafeReport(t *testing.T) {
	diagnostics := &diagnosticsScreen{}
	report := app.Report{Selected: netip.MustParseAddr("172.28.1.4")}
	for index := 0; index < 5; index++ {
		report.Probes = append(report.Probes, app.Probe{
			Candidate: wsl.Candidate{IP: report.Selected, Interface: "eth0\x1b[2J"},
			Duration:  time.Millisecond, Error: "failure\x00",
		})
	}
	diagnostics.setReport(report)
	content := diagnostics.view(screenSnapshot{config: config.Default()})
	if strings.Count(content.body, "FAIL:") != 4 || !strings.Contains(content.body, "More addresses found") || !strings.Contains(content.body, "Selected: 172.28.1.4") {
		t.Fatalf("unexpected report: %q", content.body)
	}
	if strings.ContainsAny(content.body, "\x1b\x00") {
		t.Fatal("terminal controls leaked from the report")
	}
}

func TestRunningScreenUsesValueSnapshotAndLastThreeEvents(t *testing.T) {
	running := &runningScreen{}
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	content := running.view(screenSnapshot{
		running:      &runningSnapshot{config: cfg, stats: proxy.Snapshot{Upstream: "172.28.1.4", Requests: 12, Retries: 3, Failures: 2}},
		recentEvents: []string{"oldest event", "second event", "third event", "last event"},
	})
	for _, expected := range []string{"Ubuntu", "172.28.1.4", "Requests: 12", "Retries:  3", "Failures: 2"} {
		if !strings.Contains(content.body, expected) {
			t.Fatalf("status is missing %q: %q", expected, content.body)
		}
	}
	if strings.Contains(content.footer, "oldest event") {
		t.Fatalf("unexpected recent events: %q", content.footer)
	}
	previous := -1
	for _, event := range []string{"second event", "third event", "last event"} {
		position := strings.Index(content.footer, event)
		if position <= previous {
			t.Fatalf("missing or out-of-order event %q: %q", event, content.footer)
		}
		previous = position
	}
}
