package tui

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/tui/layout"
	"wslproxy/internal/tui/screens"
	"wslproxy/internal/wsl"
)

func TestTypedOperationResults(t *testing.T) {
	selected := netip.MustParseAddr("172.28.1.4")
	previousReport := app.Report{Selected: netip.MustParseAddr("172.28.1.3")}
	report := app.Report{Selected: selected}
	cfg := config.Default()
	cfg.Distribution = "Debian"
	started := &app.Running{Config: cfg}
	failure := errors.New("operation failed")
	for _, operation := range []struct {
		name   string
		result func(int, error) tea.Msg
	}{
		{"save", func(id int, err error) tea.Msg { return configSavedMsg{id: id, config: cfg, err: err} }},
		{"start", func(id int, err error) tea.Msg {
			return proxyStartedMsg{id: id, running: started, report: report, err: err}
		}},
		{"stop", func(id int, err error) tea.Msg { return proxyStoppedMsg{id: id, err: err} }},
		{"diagnose", func(id int, err error) tea.Msg { return diagnosticsCompletedMsg{id: id, report: report, err: err} }},
	} {
		for _, outcome := range []struct {
			name string
			err  error
		}{{"success", nil}, {"error", failure}} {
			for _, stale := range []bool{false, true} {
				name := operation.name + "/" + outcome.name
				if stale {
					name += "/stale"
				}
				t.Run(name, func(t *testing.T) {
					m := newTestModel(t)
					originalConfig := m.config
					originalRunning := &app.Running{Config: originalConfig}
					m.running = originalRunning
					m.setActive(m.form) // Results must be processed independently of the active screen.
					m.diagnostics.SetReport(previousReport)
					m.notice = "previous notice"
					ctx, id := m.operationContext("working")
					if stale {
						id--
					}
					m.Update(operation.result(id, outcome.err))
					m.diagnostics.Resize(layout.Size{Width: 60, Height: 20}, screens.Snapshot{Config: m.config})
					body := m.diagnostics.View(screens.Snapshot{Config: m.config}).Body
					if stale {
						if m.busy != "working" || ctx.Err() != nil || m.cancel == nil || m.error != "" || m.notice != "previous notice" || m.config != originalConfig || m.running != originalRunning || m.active != m.form || !strings.Contains(body, previousReport.Selected.String()) {
							t.Fatal("stale result changed state or canceled the active operation")
						}
						return
					}
					if m.busy != "" || !errors.Is(ctx.Err(), context.Canceled) || m.cancel != nil {
						t.Fatal("completion did not release the operation context")
					}
					wantError := ""
					if outcome.err != nil {
						wantError = outcome.err.Error()
					}
					if m.error != wantError {
						t.Fatalf("error = %q, want %q", m.error, wantError)
					}
					if operation.name == "save" || operation.name == "stop" {
						if strings.Contains(body, "Selected:") {
							t.Fatal("completion retained an old diagnostic report")
						}
					} else if !strings.Contains(body, selected.String()) {
						t.Fatal("completion lost its diagnostic report")
					}
					wantConfig, wantRunning, wantActive, wantNotice := originalConfig, originalRunning, screenComponent(m.form), "previous notice"
					switch operation.name {
					case "save":
						if outcome.err == nil {
							wantConfig, wantActive = cfg, m.menu
							wantNotice = "Configuration saved. Changes apply on the next start."
						}
					case "start":
						if outcome.err == nil {
							wantConfig, wantRunning, wantActive, wantNotice = cfg, started, m.runningView, ""
						}
					case "stop":
						wantRunning, wantActive = nil, m.menu
						if outcome.err == nil {
							wantNotice = "Proxy stopped; Windows port released."
						}
					}
					if m.config != wantConfig || m.running != wantRunning || m.active != wantActive || m.notice != wantNotice {
						t.Fatal("completion changed configuration, running state, navigation, or notice unexpectedly")
					}
				})
			}
		}
	}
}

type operationRunner func(context.Context, ...string) ([]byte, error)

func (run operationRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	return run(ctx, args...)
}

func TestCommandsHonorCancellationAndDeliverTypedErrors(t *testing.T) {
	for _, name := range []string{"save", "start", "diagnose"} {
		t.Run(name, func(t *testing.T) {
			m := newTestModel(t)
			m.config.Distribution = "Ubuntu"
			if err := m.service.Store.Save(m.config); err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			m.service.WSL = wsl.Client{Runner: operationRunner(func(ctx context.Context, _ ...string) ([]byte, error) {
				close(entered)
				<-ctx.Done()
				return nil, ctx.Err()
			})}
			var cmd tea.Cmd
			switch name {
			case "save":
				cmd = m.SaveConfig(m.config)
			case "start":
				cmd = m.start()
			case "diagnose":
				cmd = m.diagnose()
			}
			id := m.operation
			results := make(chan tea.Msg, 1)
			go func() { results <- cmd() }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("command did not reach the background operation")
			}
			press(m, 'c', tea.ModCtrl)
			var result tea.Msg
			select {
			case result = <-results:
			case <-time.After(5 * time.Second):
				t.Fatal("command did not finish after cancellation")
			}
			var resultID int
			var err error
			switch result := result.(type) {
			case configSavedMsg:
				if name != "save" {
					t.Fatalf("unexpected result %T", result)
				}
				resultID, err = result.id, result.err
			case proxyStartedMsg:
				if name != "start" {
					t.Fatalf("unexpected result %T", result)
				}
				resultID, err = result.id, result.err
			case diagnosticsCompletedMsg:
				if name != "diagnose" {
					t.Fatalf("unexpected result %T", result)
				}
				resultID, err = result.id, result.err
			default:
				t.Fatalf("unexpected result %T", result)
			}
			if resultID != id || !errors.Is(err, context.Canceled) {
				t.Fatalf("wrong completion: %d, %v", resultID, err)
			}
			m.Update(result)
			if m.cancel != nil || m.busy != "" || m.error == "" {
				t.Fatal("canceled completion left an active operation or lost its error")
			}
		})
	}
}

func TestOperationCommandsSaveDiagnoseStartAndStop(t *testing.T) {
	m := newTestModel(t)
	m.service.Logger = m.logs.Logger()
	m.service.WSL = wsl.Client{Runner: operationRunner(func(context.Context, ...string) ([]byte, error) {
		return []byte("Ubuntu"), nil
	})}
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer target.Close()
	_, targetPort, err := net.SplitHostPort(target.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	// Reserve an available loopback port, then release it for Service.Start.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, listenPort, err := net.SplitHostPort(listener.Addr().String())
	listener.Close()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Distribution, cfg.ListenAddress, cfg.ManualTargetIP = "Ubuntu", "127.0.0.1", "127.0.0.1"
	cfg.ListenPort, _ = strconv.Atoi(listenPort)
	cfg.TargetPort, _ = strconv.Atoi(targetPort)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.service.Stop(ctx)
	})
	save := m.SaveConfig(cfg)
	saved, ok := save().(configSavedMsg)
	if !ok || saved.id != m.operation || saved.err != nil {
		t.Fatalf("save result: %+v", saved)
	}
	m.Update(saved)
	stored, err := m.service.Store.Load()
	if err != nil || stored != cfg || m.config != cfg {
		t.Fatalf("configuration was not persisted and applied: %v", err)
	}
	diagnose := m.diagnose()
	diagnosed, ok := diagnose().(diagnosticsCompletedMsg)
	if !ok || diagnosed.id != m.operation || diagnosed.err != nil || diagnosed.report.Selected.String() != "127.0.0.1" {
		t.Fatalf("diagnostic result: %+v", diagnosed)
	}
	m.Update(diagnosed)
	var started proxyStartedMsg
	for cycle := 0; cycle < 3; cycle++ {
		if body := m.View().Content; !strings.Contains(body, "Start proxy") || strings.Contains(body, "Stop proxy") {
			t.Fatalf("cycle %d: menu does not offer Start: %q", cycle, body)
		}
		start := press(m, tea.KeyEnter)
		if start == nil {
			t.Fatal("menu did not request start")
		}
		started, ok = start().(proxyStartedMsg)
		if !ok || started.id != m.operation || started.err != nil || started.running == nil {
			t.Fatalf("start result: %+v", started)
		}
		m.Update(started)
		if m.running != started.running || m.active != m.runningView {
			t.Fatal("start result did not activate the proxy status")
		}
		if body := m.View().Content; !strings.Contains(body, "RUNNING") {
			t.Fatal("running proxy is not rendered")
		}
		press(m, tea.KeyEscape)
		if body := m.View().Content; !strings.Contains(body, "Stop proxy") || strings.Contains(body, "Start proxy") || m.active != m.menu {
			t.Fatalf("cycle %d: menu does not offer Stop: %q", cycle, body)
		}
		stop := press(m, tea.KeyEnter)
		if stop == nil {
			t.Fatal("menu did not request stop")
		}
		// Graceful shutdown has its own context and must finish even if the UI is canceled.
		m.cancel()
		stopped, ok := stop().(proxyStoppedMsg)
		if !ok || stopped.id != m.operation || stopped.err != nil {
			t.Fatalf("stop result: %+v", stopped)
		}
		m.Update(stopped)
		if m.running != nil || m.active != m.menu || m.cancel != nil || m.busy != "" {
			t.Fatal("stop did not release the proxy and operation state")
		}
		select {
		case <-started.running.Server.Done():
		default:
			t.Fatal("stop left the server running")
		}
		released, err := net.Listen("tcp4", cfg.ListenEndpoint())
		if err != nil {
			t.Fatalf("stop did not release the listener: %v", err)
		}
		released.Close()
	}

	// Model a server that finished before the UI observed its termination.
	m.running = started.running
	previousOperation := m.operation
	_, cmd := m.Update(tickMsg{})
	if cmd == nil || m.operation != previousOperation+1 || m.busy != "Stopping HTTP server..." || m.error != "HTTP server stopped: <nil>" {
		t.Fatal("tick did not begin cleanup or preserve the server termination error")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatal("cleanup and the next tick were not batched")
	}
	result, ok := batch[0]().(proxyStoppedMsg)
	if !ok || result.id != m.operation || result.err != nil {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	m.Update(result)
	if _, ok := batch[1]().(tickMsg); !ok || m.running != nil || m.busy != "" || m.error != "HTTP server stopped: <nil>" {
		t.Fatal("cleanup lost the next tick or server termination error")
	}
}
