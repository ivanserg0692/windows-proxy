package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"wslproxy/internal/config"
)

var fieldLabels = []string{"Windows listen address", "Windows listen port", "WSL distribution (Enter to select)", "WSL target port", "Manual IPv4 (empty = auto)", "Connect timeout (ms)", "TCP retries", "Retry delay (ms)"}

func (m *model) openForm() {
	cfg := m.config
	values := []string{cfg.ListenAddress, strconv.Itoa(cfg.ListenPort), cfg.Distribution, strconv.Itoa(cfg.TargetPort), cfg.ManualTargetIP, strconv.Itoa(cfg.ConnectTimeoutMS), strconv.Itoa(cfg.ConnectRetries), strconv.Itoa(cfg.RetryDelayMS)}
	m.fields = make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.Prompt = "  "
		input.CharLimit = 256
		input.SetWidth(42)
		input.SetValue(value)
		m.fields[i] = input
	}
	m.focus = 0
	m.screen = formScreen
}

func (m *model) formConfig() (config.Config, error) {
	cfg := m.config
	cfg.ListenAddress = strings.TrimSpace(m.fields[0].Value())
	cfg.Distribution = m.fields[2].Value()
	cfg.ManualTargetIP = strings.TrimSpace(m.fields[4].Value())
	for _, field := range []struct {
		index  int
		target *int
	}{{1, &cfg.ListenPort}, {3, &cfg.TargetPort}, {5, &cfg.ConnectTimeoutMS}, {6, &cfg.ConnectRetries}, {7, &cfg.RetryDelayMS}} {
		value, err := strconv.Atoi(strings.TrimSpace(m.fields[field.index].Value()))
		if err != nil {
			return cfg, fmt.Errorf("%s must be an integer", fieldLabels[field.index])
		}
		*field.target = value
	}
	return cfg, cfg.Validate()
}
