package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
)

type model struct {
	ctx           context.Context
	service       *app.Service
	logs          *Logs
	config        config.Config
	menu          *menuScreen
	form          *formScreen
	distributions *distributionsScreen
	diagnostics   *diagnosticsScreen
	runningView   *runningScreen
	active        screenComponent
	width, height int
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
type operationMsg struct {
	id      int
	running *app.Running
	report  app.Report
	saved   *config.Config
	stopped bool
	err     error
}
type tickMsg struct{}

func New(ctx context.Context, service *app.Service, logs *Logs) tea.Model {
	cfg, err := service.Store.Load()
	uiModel := &model{
		ctx: ctx, service: service, logs: logs, config: cfg,
		menu: newMenuScreen(), form: &formScreen{},
		distributions: newDistributionsScreen(),
		diagnostics:   &diagnosticsScreen{}, runningView: &runningScreen{},
		width: 80, height: 30,
	}
	uiModel.active = uiModel.menu
	if err != nil {
		uiModel.config = config.Default()
		if errors.Is(err, os.ErrNotExist) {
			uiModel.notice = "Select a WSL distribution, then configure and save."
		} else {
			uiModel.error = err.Error()
			uiModel.notice = "Saved config is invalid. Review all values before saving."
		}
	}
	return uiModel
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.loadDistributions(), tick()) }
func tick() tea.Cmd            { return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m *model) loadDistributions() tea.Cmd {
	return func() tea.Msg { names, err := m.service.WSL.Distributions(m.ctx); return distributionsMsg{names, err} }
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for _, component := range []screenComponent{m.menu, m.form, m.distributions, m.diagnostics, m.runningView} {
			component.resize(msg.Width, msg.Height)
		}
		return m, nil
	case distributionsMsg:
		if msg.err != nil {
			m.error = msg.err.Error()
			return m, nil
		}
		m.distributions.setNames(msg.names, m.width, m.height)
		if len(msg.names) == 0 {
			m.error = "No WSL distributions found. Install a distribution and refresh with R."
		}
		return m, nil
	case operationMsg:
		if msg.id != m.operation {
			return m, nil
		}
		m.busy = ""
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.diagnostics.setReport(msg.report)
		if msg.stopped {
			m.running = nil
			m.active = m.menu
		}
		if msg.err != nil {
			m.error = msg.err.Error()
			return m, nil
		}
		if msg.saved != nil {
			m.config = *msg.saved
			m.notice = "Configuration saved. Changes apply on the next start."
			m.active = m.menu
		}
		if msg.running != nil {
			m.running = msg.running
			m.config = msg.running.Config
			m.active = m.runningView
			m.notice = ""
		}
		if msg.stopped {
			m.notice = "Proxy stopped; Windows port released."
		}
		return m, nil
	case tickMsg:
		if m.running != nil && m.busy == "" {
			select {
			case <-m.running.Server.Done():
				message := fmt.Sprintf("HTTP server stopped: %v", m.running.Server.Err())
				cmd := m.stop()
				m.error = message
				return m, tea.Batch(cmd, tick())
			default:
			}
		}
		return m, tick()
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.busy != "" {
			return m, nil
		}
		if key == "esc" {
			m.active = m.menu
			m.error = ""
			return m, nil
		}
	}
	if m.busy != "" {
		return m, nil
	}
	cmd, intent := m.active.update(msg)
	intentCmd := m.handleIntent(intent)
	if cmd == nil {
		return m, intentCmd
	}
	if intentCmd == nil {
		return m, cmd
	}
	return m, tea.Batch(cmd, intentCmd)
}

// handleIntent coordinates navigation and application operations. Screens never
// receive the root model or invoke services themselves.
func (m *model) handleIntent(intent screenIntent) tea.Cmd {
	switch intent := intent.(type) {
	case startProxyIntent:
		m.error, m.notice = "", ""
		if m.running != nil {
			m.active = m.runningView
			return nil
		}
		return m.start()
	case openFormIntent:
		m.error, m.notice = "", ""
		m.active = m.form
		return m.form.open(m.config)
	case chooseDistributionIntent:
		if !intent.forForm {
			m.error, m.notice = "", ""
		}
		m.distributions.open(intent.forForm)
		m.active = m.distributions
		return m.loadDistributions()
	case diagnoseIntent:
		if m.active == m.menu {
			m.error, m.notice = "", ""
		}
		m.active = m.diagnostics
		return m.diagnose()
	case quitIntent:
		m.error, m.notice = "", ""
		return tea.Quit
	case refreshDistributionsIntent:
		m.error = ""
		return m.loadDistributions()
	case distributionSelectedIntent:
		if intent.forForm {
			m.active = m.form
			return m.form.selectDistribution(intent.name)
		}
		cfg := m.config
		cfg.Distribution = intent.name
		return m.save(cfg)
	case saveConfigIntent:
		return m.save(intent.config)
	case showErrorIntent:
		m.error = intent.err.Error()
	case stopProxyIntent:
		return m.stop()
	}
	return nil
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
		return operationMsg{id: operationID, running: running, report: report, err: err}
	}
}
func (m *model) stop() tea.Cmd {
	_, operationID := m.operationContext("Stopping HTTP server...")
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := service.Stop(ctx)
		return operationMsg{id: operationID, stopped: true, err: err}
	}
}
func (m *model) diagnose() tea.Cmd {
	ctx, operationID := m.operationContext("Checking WSL addresses and TCP connectivity...")
	service, cfg := m.service, m.config
	return func() tea.Msg {
		report, err := service.Diagnose(ctx, cfg)
		return operationMsg{id: operationID, report: report, err: err}
	}
}
func (m *model) save(cfg config.Config) tea.Cmd {
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
		return operationMsg{id: operationID, saved: &cfg, err: err}
	}
}
