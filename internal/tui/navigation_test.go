package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/tui/screens"
	"wslproxy/internal/wsl"
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
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyEnter)
	replaceFocusedInput(uiModel, "127.0.0.2")
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyEnter)
	uiModel.Update(distributionsMsg{names: []string{"Ubuntu", "Debian"}})
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyEnter)
	if uiModel.active != uiModel.form || !strings.Contains(uiModel.form.View(screens.Snapshot{}).Body, "Field 3/8") {
		t.Fatal("selection did not return to the focused distribution field")
	}
	uiModel.service.WSL = wsl.Client{Runner: operationRunner(func(context.Context, ...string) ([]byte, error) {
		return []byte("Ubuntu\nDebian"), nil
	})}
	cmd := press(uiModel, 's', tea.ModCtrl)
	if cmd == nil || uiModel.busy == "" {
		t.Fatal("save did not start synchronously")
	}
	saved, ok := cmd().(configSavedMsg)
	if !ok || saved.err != nil || saved.config.ListenAddress != "127.0.0.2" || saved.config.Distribution != "Debian" {
		t.Fatal("selection lost the form draft")
	}
}

func TestEscapeFromSelectorDiscardsFormOnReopen(t *testing.T) {
	uiModel := newTestModel(t)
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyEnter)
	replaceFocusedInput(uiModel, "draft")
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyTab)
	press(uiModel, tea.KeyEnter)
	press(uiModel, tea.KeyEscape)
	if uiModel.active != uiModel.menu {
		t.Fatal("Escape from selector did not return to menu")
	}
	press(uiModel, tea.KeyEnter)
	if body := uiModel.form.View(screens.Snapshot{}).Body; strings.Contains(body, "draft") || !strings.Contains(body, uiModel.config.ListenAddress) || !strings.Contains(body, "Field 1/8") {
		t.Fatal("reopened form retained a discarded draft")
	}
}

func TestBusyOperationBlocksInputButAllowsResizeAndCancel(t *testing.T) {
	uiModel := newTestModel(t)
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyDown)
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
	uiModel.setActive(uiModel.runningView)
	uiModel.Update(proxyStoppedMsg{id: 1})
	if uiModel.busy == "" || uiModel.running == nil || uiModel.active != uiModel.runningView {
		t.Fatal("stale result changed current operation")
	}
	uiModel.Update(proxyStoppedMsg{id: 2, err: errors.New("stop failed")})
	if uiModel.busy != "" || uiModel.running != nil || uiModel.active != uiModel.menu || uiModel.error != "stop failed" {
		t.Fatal("stop result did not release UI state before reporting its error")
	}
}

func TestDistributionResultsArriveWhileAnotherScreenIsActive(t *testing.T) {
	uiModel := newTestModel(t)
	uiModel.Update(distributionsMsg{names: []string{"Ubuntu", "Debian"}})
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyDown)
	press(uiModel, tea.KeyEnter)
	if body := uiModel.distributions.View(screens.Snapshot{}).Body; !strings.Contains(body, "Ubuntu") || !strings.Contains(body, "Debian") {
		t.Fatal("background distribution result was lost")
	}
	if cmd := press(uiModel, tea.KeyEnter); cmd == nil || uiModel.busy == "" {
		t.Fatal("menu selection did not begin saving synchronously")
	}
}

// replaceFocusedInput edits through the same key events as the interactive UI.
func replaceFocusedInput(uiModel *model, value string) {
	press(uiModel, tea.KeyEnd)
	for i := 0; i < 256; i++ {
		press(uiModel, tea.KeyBackspace)
	}
	for _, r := range value {
		uiModel.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}
