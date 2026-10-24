package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/tui/screens"
)

func TestLogsAreBoundedAndEscapeTerminalControls(t *testing.T) {
	logs := &Logs{}
	for i := 0; i < 20; i++ {
		logs.Logger().Error("failure\x1b[2J", "error", "bad\x00input")
	}
	lines := logs.Lines()
	if len(lines) != 8 {
		t.Fatalf("unbounded events: %d", len(lines))
	}
	if strings.ContainsAny(strings.Join(lines, ""), "\x1b\x00") {
		t.Fatal("terminal controls leaked")
	}
}

func TestSmallTerminalView(t *testing.T) {
	uiModel := &model{width: 20, height: 10}
	view := uiModel.View()
	if !view.AltScreen || !strings.Contains(view.Content, "Resize") {
		t.Fatalf("unexpected view: %+v", view)
	}
}

type recordingScreen struct {
	messages []tea.Msg
	cmd      tea.Cmd
}

func (s *recordingScreen) Update(msg tea.Msg) tea.Cmd {
	s.messages = append(s.messages, msg)
	return s.cmd
}

func (*recordingScreen) View(screens.Snapshot) screens.Content { return screens.Content{} }
func (*recordingScreen) Resize(int, int)                       {}

func TestUpdateRoutesOnlyUnhandledMessagesToActiveScreen(t *testing.T) {
	for _, busy := range []string{"", "working"} {
		t.Run("busy="+busy, func(t *testing.T) {
			m := newTestModel(t)
			screen := &recordingScreen{}
			m.setActive(screen)
			m.busy = busy
			m.operation = 1
			for _, msg := range []tea.Msg{
				tea.WindowSizeMsg{Width: 100, Height: 40},
				distributionsMsg{names: []string{"Ubuntu"}},
				configSavedMsg{id: 0},
				proxyStartedMsg{id: 0},
				proxyStoppedMsg{id: 0},
				diagnosticsCompletedMsg{id: 0},
				tickMsg{},
			} {
				m.Update(msg)
			}
			if len(screen.messages) != 0 || m.active != screen || m.busy != busy {
				t.Fatal("root message reached the screen or changed operation state")
			}

			key := tea.KeyPressMsg{Code: 'x'}
			other := struct{ value string }{"screen event"}
			m.Update(key)
			m.Update(other)
			if busy != "" {
				if len(screen.messages) != 0 {
					t.Fatal("busy model forwarded a screen event")
				}
			} else if len(screen.messages) != 2 || screen.messages[0] != key || screen.messages[1] != other {
				t.Fatal("idle model did not forward messages unchanged")
			}

			m.error = "previous error"
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if busy == "" {
				if m.active != m.menu || m.error != "" {
					t.Fatal("Escape did not return to the menu and clear the error")
				}
			} else if m.active != screen || m.error != "previous error" {
				t.Fatal("Escape changed state while busy")
			}
		})
	}
}

func TestUpdateReturnsActiveScreenCommand(t *testing.T) {
	for _, withCommand := range []bool{false, true} {
		m := newTestModel(t)
		screen := &recordingScreen{}
		m.setActive(screen)
		executed := false
		if withCommand {
			screen.cmd = func() tea.Msg { executed = true; return "screen command" }
		}
		_, cmd := m.Update(struct{}{})
		if executed {
			t.Fatal("root executed the command during Update")
		}
		if withCommand {
			if cmd == nil || cmd() != "screen command" {
				t.Fatal("screen command was lost")
			}
		} else if cmd != nil {
			t.Fatal("unexpected command")
		}
	}
}

func TestMenuQuitUpdatesStateBeforeReturningCommand(t *testing.T) {
	m := newTestModel(t)
	for i := 0; i < 4; i++ {
		press(m, tea.KeyDown)
	}
	m.error, m.notice = "previous error", "previous notice"
	cmd := press(m, tea.KeyEnter)
	if m.error != "" || m.notice != "" || cmd == nil {
		t.Fatal("menu request did not clear messages synchronously")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit command was lost")
	}
}

func TestTickKeepsSchedulingWhileIdleOrBusy(t *testing.T) {
	for _, busy := range []string{"", "working"} {
		m := newTestModel(t)
		m.busy = busy
		if busy != "" {
			// A busy model must not inspect the server while an operation owns it.
			m.running = &app.Running{}
		}
		_, cmd := m.Update(tickMsg{})
		if cmd == nil {
			t.Fatal("next tick was not scheduled")
		}
		if _, ok := cmd().(tickMsg); !ok || m.busy != busy || m.operation != 0 {
			t.Fatal("tick changed operation state or returned the wrong message")
		}
	}
}

func TestDistributionErrorsPreserveExistingChoices(t *testing.T) {
	m := newTestModel(t)
	m.Update(distributionsMsg{names: []string{"Ubuntu"}})
	m.busy = "working"
	m.Update(distributionsMsg{err: errors.New("WSL unavailable")})
	if m.error != "WSL unavailable" || !strings.Contains(m.distributions.View(screens.Snapshot{}).Body, "Ubuntu") {
		t.Fatal("distribution error replaced choices or was lost while busy")
	}
	m.Update(distributionsMsg{})
	if m.error != "No WSL distributions found. Install a distribution and refresh with R." {
		t.Fatal("empty distribution list did not report its error")
	}
}
