package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
)

type configSavedMsg struct {
	id     int
	config config.Config
	err    error
}

type proxyStartedMsg struct {
	id      int
	running *app.Running
	report  app.Report
	err     error
}

type proxyStoppedMsg struct {
	id  int
	err error
}

type diagnosticsCompletedMsg struct {
	id     int
	report app.Report
	err    error
}

func (m *model) handleConfigSaved(msg configSavedMsg) {
	if !m.finishOperation(msg.id) {
		return
	}
	m.diagnostics.SetReport(app.Report{})
	if msg.err != nil {
		m.error = msg.err.Error()
		return
	}
	m.config = msg.config
	m.notice = "Configuration saved. Changes apply on the next start."
	m.setActive(m.menu)
}

func (m *model) handleProxyStarted(msg proxyStartedMsg) {
	if !m.finishOperation(msg.id) {
		return
	}
	m.diagnostics.SetReport(msg.report)
	if msg.err != nil {
		m.error = msg.err.Error()
		return
	}
	if msg.running != nil {
		m.running = msg.running
		m.config = msg.running.Config
		m.setActive(m.runningView)
		m.notice = ""
	}
}

func (m *model) handleProxyStopped(msg proxyStoppedMsg) {
	if !m.finishOperation(msg.id) {
		return
	}
	m.diagnostics.SetReport(app.Report{})
	m.running = nil
	m.setActive(m.menu)
	if msg.err != nil {
		m.error = msg.err.Error()
		return
	}
	m.notice = "Proxy stopped; Windows port released."
}

func (m *model) handleDiagnosticsCompleted(msg diagnosticsCompletedMsg) {
	if !m.finishOperation(msg.id) {
		return
	}
	m.diagnostics.SetReport(msg.report)
	if msg.err != nil {
		m.error = msg.err.Error()
	}
}

// finishOperation releases the current operation context; stale results have no effect.
func (m *model) finishOperation(id int) bool {
	if id != m.operation {
		return false
	}
	m.busy = ""
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	return true
}

func (m *model) operationContext(label string) (context.Context, int) {
	m.error = ""
	m.busy = label
	m.operation++
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	return ctx, m.operation
}
func (m *model) start() tea.Cmd {
	ctx, operationID := m.operationContext("Resolving WSL, checking TCP and opening Windows listener...")
	service := m.service
	return func() tea.Msg {
		running, report, err := service.Start(ctx)
		return proxyStartedMsg{id: operationID, running: running, report: report, err: err}
	}
}
func (m *model) StopProxy() tea.Cmd {
	_, operationID := m.operationContext("Stopping HTTP server...")
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := service.Stop(ctx)
		return proxyStoppedMsg{id: operationID, err: err}
	}
}
func (m *model) diagnose() tea.Cmd {
	ctx, operationID := m.operationContext("Checking WSL addresses and TCP connectivity...")
	service, cfg := m.service, m.config
	return func() tea.Msg {
		report, err := service.Diagnose(ctx, cfg)
		return diagnosticsCompletedMsg{id: operationID, report: report, err: err}
	}
}
func (m *model) SaveConfig(cfg config.Config) tea.Cmd {
	ctx, operationID := m.operationContext("Validating and saving configuration...")
	service := m.service
	return func() tea.Msg {
		err := cfg.Validate()
		if err == nil {
			err = service.WSL.CheckDistribution(ctx, cfg.Distribution)
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			err = service.Store.Save(cfg)
		}
		return configSavedMsg{id: operationID, config: cfg, err: err}
	}
}
