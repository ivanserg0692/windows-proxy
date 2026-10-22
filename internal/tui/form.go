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
	c := m.config
	values := []string{c.ListenAddress, strconv.Itoa(c.ListenPort), c.Distribution, strconv.Itoa(c.TargetPort), c.ManualTargetIP, strconv.Itoa(c.ConnectTimeoutMS), strconv.Itoa(c.ConnectRetries), strconv.Itoa(c.RetryDelayMS)}
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
	c := m.config
	c.ListenAddress = strings.TrimSpace(m.fields[0].Value())
	c.Distribution = m.fields[2].Value()
	c.ManualTargetIP = strings.TrimSpace(m.fields[4].Value())
	for _, field := range []struct {
		index  int
		target *int
	}{{1, &c.ListenPort}, {3, &c.TargetPort}, {5, &c.ConnectTimeoutMS}, {6, &c.ConnectRetries}, {7, &c.RetryDelayMS}} {
		value, err := strconv.Atoi(strings.TrimSpace(m.fields[field.index].Value()))
		if err != nil {
			return c, fmt.Errorf("%s must be an integer", fieldLabels[field.index])
		}
		*field.target = value
	}
	return c, c.Validate()
}
