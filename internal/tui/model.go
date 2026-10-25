package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/tui/screens"

	tea "charm.land/bubbletea/v2"
)

const (
	defaultTerminalWidth  = 80
	defaultTerminalHeight = 30
)

type model struct {
	ctx           context.Context
	service       *app.Service
	logs          *Logs
	config        config.Config
	menu          *screens.Menu
	form          *screens.Form
	distributions *screens.Distributions
	diagnostics   *screens.Diagnostics
	runningView   *screens.Running
	active        screenComponent
	width, height int
	presentation  viewData
	busy          string
	cancel        context.CancelFunc
	operation     int
	error         string
	notice        string
	running       *app.Running
}

type distributionsMsg struct {
	names []string
	err   error
}
type tickMsg struct{}

func New(ctx context.Context, service *app.Service, logs *Logs) tea.Model {
	cfg, err := service.Store.Load()
	uiModel := &model{
		ctx: ctx, service: service, logs: logs, config: cfg,
		width: defaultTerminalWidth, height: defaultTerminalHeight,
	}
	uiModel.menu = screens.NewMenu(uiModel)
	uiModel.form = screens.NewForm(uiModel)
	uiModel.distributions = screens.NewDistributions(uiModel)
	uiModel.diagnostics = screens.NewDiagnostics(uiModel)
	uiModel.runningView = screens.NewRunning(uiModel)
	uiModel.setActive(uiModel.menu)
	if err != nil {
		uiModel.config = config.Default()
		if errors.Is(err, os.ErrNotExist) {
			uiModel.notice = "Select a WSL distribution, then configure and save."
		} else {
			uiModel.error = err.Error()
			uiModel.notice = "Saved config is invalid. Review all values before saving."
		}
	}
	uiModel.refreshView()
	return uiModel
}

func (m *model) setActive(screen screenComponent) {
	m.active = screen
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.loadDistributions(), tick()) }
func tick() tea.Cmd            { return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m *model) loadDistributions() tea.Cmd {
	return func() tea.Msg { names, err := m.service.WSL.Distributions(m.ctx); return distributionsMsg{names, err} }
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer m.refreshView()
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case distributionsMsg:
		m.handleDistributionsLoaded(msg)
		return m, nil
	case configSavedMsg:
		m.handleConfigSaved(msg)
		return m, nil
	case proxyStartedMsg:
		m.handleProxyStarted(msg)
		return m, nil
	case proxyStoppedMsg:
		m.handleProxyStopped(msg)
		return m, nil
	case diagnosticsCompletedMsg:
		m.handleDiagnosticsCompleted(msg)
		return m, nil
	case tickMsg:
		return m, m.handleTick()
	case tea.KeyPressMsg:
		if cmd, handled := m.handleGlobalKey(msg); handled {
			return m, cmd
		}
	}
	if m.busy != "" {
		return m, nil
	}
	return m, m.active.Update(msg)
}

func (m *model) handleDistributionsLoaded(msg distributionsMsg) {
	if msg.err != nil {
		m.error = msg.err.Error()
		return
	}
	m.distributions.SetNames(msg.names)
	if len(msg.names) == 0 {
		m.error = "No WSL distributions found. Install a distribution and refresh with R."
	}
}

func (m *model) handleTick() tea.Cmd {
	if m.running != nil && m.busy == "" {
		select {
		case <-m.running.Server.Done():
			message := fmt.Sprintf("HTTP server stopped: %v", m.running.Server.Err())
			cmd := m.StopProxy()
			m.error = message
			return tea.Batch(cmd, tick())
		default:
		}
	}
	return tick()
}

func (m *model) handleGlobalKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()
	if key == "ctrl+c" {
		if m.cancel != nil {
			m.cancel()
		}
		return tea.Quit, true
	}
	if m.busy != "" {
		return nil, true
	}
	if key == "esc" {
		m.setActive(m.menu)
		m.error = ""
		return nil, true
	}
	return nil, false
}

func (m *model) StartProxy() tea.Cmd {
	m.error, m.notice = "", ""
	if m.running != nil {
		m.setActive(m.runningView)
		return nil
	}
	return m.start()
}

func (m *model) OpenForm() tea.Cmd {
	m.error, m.notice = "", ""
	m.setActive(m.form)
	return m.form.Open(m.config)
}

func (m *model) ChooseDistribution(forForm bool) tea.Cmd {
	if !forForm {
		m.error, m.notice = "", ""
	}
	m.distributions.Open(forForm)
	m.setActive(m.distributions)
	return m.loadDistributions()
}

func (m *model) Diagnose() tea.Cmd {
	if m.active == m.menu {
		m.error, m.notice = "", ""
	}
	m.setActive(m.diagnostics)
	return m.diagnose()
}

func (m *model) Quit() tea.Cmd {
	m.error, m.notice = "", ""
	return tea.Quit
}

func (m *model) RefreshDistributions() tea.Cmd {
	m.error = ""
	return m.loadDistributions()
}

func (m *model) SelectDistribution(name string, forForm bool) tea.Cmd {
	if forForm {
		m.setActive(m.form)
		return m.form.SelectDistribution(name)
	}
	cfg := m.config
	cfg.Distribution = name
	return m.SaveConfig(cfg)
}

func (m *model) ShowError(err error) tea.Cmd {
	m.error = err.Error()
	return nil
}
