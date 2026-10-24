package tui

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/tui/screens"
)

func TestInvalidFormDoesNotStartAnOperation(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.config.Distribution = "Ubuntu"
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyEnter)
	press(uiModel, tea.KeyTab)
	replaceFocusedInput(uiModel, "invalid")
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
	uiModel.Update(diagnosticsCompletedMsg{id: 1, report: app.Report{Selected: selected}, err: errors.New("partial report")})
	if uiModel.active != uiModel.menu || uiModel.busy != "" || !canceled || uiModel.cancel != nil {
		t.Fatal("background result changed navigation or left its operation active")
	}
	if !strings.Contains(uiModel.diagnostics.View(screens.Snapshot{Config: uiModel.config}).Body, "Selected: "+selected.String()) || uiModel.error != "partial report" {
		t.Fatal("background diagnostic report was lost")
	}
	saved := uiModel.config
	saved.Distribution = "Debian"
	uiModel.Update(configSavedMsg{id: 1, config: saved})
	if uiModel.config != saved || strings.Contains(uiModel.diagnostics.View(screens.Snapshot{Config: uiModel.config}).Body, "Selected:") || uiModel.active != uiModel.menu {
		t.Fatal("save result did not update configuration and clear the old report")
	}
}

func TestMenuAndEscapeKeepProxyActive(t *testing.T) {
	uiModel := newTestModel(t)
	running := &app.Running{Config: uiModel.config}
	uiModel.Update(proxyStartedMsg{running: running})
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
