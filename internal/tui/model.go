package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
)

type screen int

const (
	menuScreen screen = iota
	formScreen
	distributionsScreen
	diagnosticsScreen
	runningScreen
)

type model struct {
	ctx           context.Context
	service       *app.Service
	logs          *Logs
	config        config.Config
	menu          list.Model
	distributions list.Model
	names         []string
	screen        screen
	returnTo      screen
	fields        []textinput.Model
	focus         int
	width, height int
	busy          string
	cancel        context.CancelFunc
	operation     int
	error         string
	notice        string
	report        app.Report
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
	uiModel := &model{ctx: ctx, service: service, logs: logs, config: cfg, menu: choiceList(menuItems), distributions: choiceList(nil), width: 80, height: 30}
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
		m.menu.SetSize(max(20, min(64, msg.Width-8)), 7)
		m.distributions.SetSize(max(20, min(64, msg.Width-8)), max(3, min(10, msg.Height-13)))
		for i := range m.fields {
			m.fields[i].SetWidth(max(12, min(46, msg.Width-12)))
		}
		return m, nil
	case distributionsMsg:
		if msg.err != nil {
			m.error = msg.err.Error()
			return m, nil
		}
		m.names = msg.names
		m.distributions = choiceList(msg.names)
		m.distributions.SetSize(max(20, min(64, m.width-8)), max(3, min(10, m.height-13)))
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
		m.report = msg.report
		if msg.stopped {
			m.running = nil
			m.screen = menuScreen
		}
		if msg.err != nil {
			m.error = msg.err.Error()
			return m, nil
		}
		if msg.saved != nil {
			m.config = *msg.saved
			m.notice = "Configuration saved. Changes apply on the next start."
			m.screen = menuScreen
		}
		if msg.running != nil {
			m.running = msg.running
			m.config = msg.running.Config
			m.screen = runningScreen
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
			m.screen = menuScreen
			m.error = ""
			return m, nil
		}
		switch m.screen {
		case menuScreen:
			if key == "enter" {
				m.error = ""
				m.notice = ""
				switch m.menu.Index() {
				case 0:
					if m.running != nil {
						m.screen = runningScreen
						return m, nil
					}
					return m, m.start()
				case 1:
					m.openForm()
					return m, m.fields[0].Focus()
				case 2:
					m.returnTo = menuScreen
					m.screen = distributionsScreen
					return m, m.loadDistributions()
				case 3:
					m.screen = diagnosticsScreen
					return m, m.diagnose()
				case 4:
					return m, tea.Quit
				}
			}
			var cmd tea.Cmd
			m.menu, cmd = m.menu.Update(msg)
			return m, cmd
		case formScreen:
			return m, m.updateForm(msg)
		case distributionsScreen:
			if key == "r" {
				m.error = ""
				return m, m.loadDistributions()
			}
			if key == "enter" {
				selected, ok := m.distributions.SelectedItem().(item)
				if !ok {
					return m, nil
				}
				if m.returnTo == formScreen {
					m.fields[2].SetValue(string(selected))
					m.screen = formScreen
					return m, m.fields[m.focus].Focus()
				}
				cfg := m.config
				cfg.Distribution = string(selected)
				return m, m.save(cfg)
			}
			var cmd tea.Cmd
			m.distributions, cmd = m.distributions.Update(msg)
			return m, cmd
		case diagnosticsScreen:
			if key == "r" {
				return m, m.diagnose()
			}
		case runningScreen:
			if key == "s" || key == "S" {
				return m, m.stop()
			}
		}
	}
	if m.screen == formScreen && m.busy == "" && len(m.fields) > 0 {
		var cmd tea.Cmd
		m.fields[m.focus], cmd = m.fields[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) updateForm(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+s":
		cfg, err := m.formConfig()
		if err != nil {
			m.error = err.Error()
			return nil
		}
		return m.save(cfg)
	case "enter":
		if m.focus == 2 {
			m.returnTo = formScreen
			m.screen = distributionsScreen
			return m.loadDistributions()
		}
		fallthrough
	case "tab", "down":
		m.fields[m.focus].Blur()
		m.focus = (m.focus + 1) % len(m.fields)
		return m.fields[m.focus].Focus()
	case "shift+tab", "up":
		m.fields[m.focus].Blur()
		m.focus = (m.focus + len(m.fields) - 1) % len(m.fields)
		return m.fields[m.focus].Focus()
	}
	var cmd tea.Cmd
	m.fields[m.focus], cmd = m.fields[m.focus].Update(msg)
	return cmd
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
