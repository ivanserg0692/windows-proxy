package screens

import (
	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/config"
)

// MenuReceiver coordinates application actions requested by the menu.
type MenuReceiver interface {
	StartProxy() tea.Cmd
	OpenForm() tea.Cmd
	ChooseDistribution(forForm bool) tea.Cmd
	Diagnose() tea.Cmd
	Quit() tea.Cmd
}

// FormReceiver handles form navigation, validated saves, and validation errors.
type FormReceiver interface {
	ChooseDistribution(forForm bool) tea.Cmd
	SaveConfig(config.Config) tea.Cmd
	ShowError(error) tea.Cmd
}

// DistributionsReceiver reloads distributions and applies a confirmed selection.
type DistributionsReceiver interface {
	RefreshDistributions() tea.Cmd
	// SelectDistribution returns to the form draft when forForm is true;
	// otherwise, it saves name as the application distribution.
	SelectDistribution(name string, forForm bool) tea.Cmd
}

// DiagnosticsReceiver starts diagnostics requested by the diagnostics screen.
type DiagnosticsReceiver interface {
	Diagnose() tea.Cmd
}

// RunningReceiver stops the proxy when requested by its status screen.
type RunningReceiver interface {
	StopProxy() tea.Cmd
}
