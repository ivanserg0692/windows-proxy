package tui

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
)

func newTestModel(t *testing.T) *model {
	t.Helper()
	uiModel := New(context.Background(), &app.Service{Store: config.Store{Path: filepath.Join(t.TempDir(), "config.json")}}, &Logs{}).(*model)
	t.Cleanup(func() {
		if uiModel.cancel != nil {
			uiModel.cancel()
		}
	})
	return uiModel
}

func press(uiModel *model, code rune, modifiers ...tea.KeyMod) tea.Cmd {
	key := tea.KeyPressMsg{Code: code}
	for _, modifier := range modifiers {
		key.Mod |= modifier
	}
	_, cmd := uiModel.Update(key)
	return cmd
}

func TestDistributionSelectionPreservesFormDraftAndFocus(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.menu.choices.Select(1)
	press(uiModel, tea.KeyEnter)
	uiModel.form.fields[0].SetValue("127.0.0.2")
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyEnter)
	uiModel.Update(distributionsMsg{names: []string{"Ubuntu", "Debian"}})
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyEnter)
	if uiModel.active != uiModel.form || uiModel.form.focus != 2 || !uiModel.form.fields[2].Focused() {
		t.Fatal("selection did not return to the focused distribution field")
	}
	if uiModel.form.fields[0].Value() != "127.0.0.2" || uiModel.form.fields[2].Value() != "Debian" {
		t.Fatal("selection lost the form draft")
	}
}

func TestEscapeFromSelectorDiscardsFormOnReopen(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.menu.choices.Select(1)
	press(uiModel, tea.KeyEnter)
	uiModel.form.fields[0].SetValue("draft")
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyEnter)
	press(uiModel, tea.KeyEscape)
	if uiModel.active != uiModel.menu {
		t.Fatal("Escape from selector did not return to menu")
	}
	press(uiModel, tea.KeyEnter)
	if uiModel.form.fields[0].Value() != uiModel.config.ListenAddress || uiModel.form.focus != 0 {
		t.Fatal("reopened form retained a discarded draft")
	}
}

func TestBusyOperationBlocksInputButAllowsResizeAndCancel(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.menu.choices.Select(3)
	if press(uiModel, tea.KeyEnter) == nil || uiModel.busy == "" || uiModel.active != uiModel.diagnostics {
		t.Fatal("diagnostics did not start synchronously")
	}
	press(uiModel, tea.KeyEscape)
	if uiModel.active != uiModel.diagnostics {
		t.Fatal("busy operation allowed navigation")
	}
	uiModel.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if uiModel.width != 100 || uiModel.height != 40 {
		t.Fatal("busy operation blocked resize")
	}
	canceled := false
	uiModel.cancel = func() { canceled = true }
	cmd := press(uiModel, 'c', tea.ModCtrl)
	if !canceled || cmd == nil {
		t.Fatal("Ctrl+C did not cancel and quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl+C returned a non-quit command")
	}
}

func TestOperationResultsPreserveOrderingAndIgnoreStaleResults(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.operation = 2
	uiModel.busy = "stopping"
	uiModel.running = &app.Running{}
	uiModel.active = uiModel.runningView
	uiModel.Update(operationMsg{id: 1, stopped: true})
	if uiModel.busy == "" || uiModel.running == nil || uiModel.active != uiModel.runningView {
		t.Fatal("stale result changed current operation")
	}
	uiModel.Update(operationMsg{id: 2, stopped: true, err: errors.New("stop failed")})
	if uiModel.busy != "" || uiModel.running != nil || uiModel.active != uiModel.menu || uiModel.error != "stop failed" {
		t.Fatal("stop result did not release UI state before reporting its error")
	}
}

func TestDistributionResultsArriveWhileAnotherScreenIsActive(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.Update(distributionsMsg{names: []string{"Ubuntu", "Debian"}})
	uiModel.menu.choices.Select(2)
	press(uiModel, tea.KeyEnter)
	if len(uiModel.distributions.choices.Items()) != 2 {
		t.Fatal("background distribution result was lost")
	}
	if cmd := press(uiModel, tea.KeyEnter); cmd == nil || uiModel.busy == "" {
		t.Fatal("menu selection did not begin saving synchronously")
	}
}
