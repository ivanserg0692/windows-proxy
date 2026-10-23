package tui

import (
	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
)

// screenComponent owns local interaction state. Intents are handled by the root
// synchronously; commands are reserved for Bubble Tea effects.
type screenComponent interface {
	update(tea.Msg) (tea.Cmd, screenIntent)
	view(screenSnapshot) screenContent
	resize(width, height int)
}

// screenSnapshot exposes values for presentation, without access to the root,
// application services, or the live server.
type screenSnapshot struct {
	config       config.Config
	proxyActive  bool
	running      *runningSnapshot
	recentEvents []string
}

type runningSnapshot struct {
	config config.Config
	stats  proxy.Snapshot
}

type screenContent struct {
	body   string
	help   string
	footer string
}

type screenIntent interface{ screenIntent() }

type startProxyIntent struct{}
type openFormIntent struct{}
type chooseDistributionIntent struct{ forForm bool }
type diagnoseIntent struct{}
type quitIntent struct{}
type refreshDistributionsIntent struct{}
type distributionSelectedIntent struct {
	name    string
	forForm bool
}
type saveConfigIntent struct{ config config.Config }
type showErrorIntent struct{ err error }
type stopProxyIntent struct{}

func (startProxyIntent) screenIntent()           {}
func (openFormIntent) screenIntent()             {}
func (chooseDistributionIntent) screenIntent()   {}
func (diagnoseIntent) screenIntent()             {}
func (quitIntent) screenIntent()                 {}
func (refreshDistributionsIntent) screenIntent() {}
func (distributionSelectedIntent) screenIntent() {}
func (saveConfigIntent) screenIntent()           {}
func (showErrorIntent) screenIntent()            {}
func (stopProxyIntent) screenIntent()            {}
