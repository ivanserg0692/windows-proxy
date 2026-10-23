package screens

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/config"
)

var fieldLabels = []string{"Windows listen address", "Windows listen port", "WSL distribution (Enter to select)", "WSL target port", "Manual IPv4 (empty = auto)", "Connect timeout (ms)", "TCP retries", "Retry delay (ms)"}

type Form struct {
	base   config.Config
	fields []textinput.Model
	focus  int
}

func (form *Form) Open(cfg config.Config) tea.Cmd {
	form.base = cfg
	values := []string{cfg.ListenAddress, strconv.Itoa(cfg.ListenPort), cfg.Distribution, strconv.Itoa(cfg.TargetPort), cfg.ManualTargetIP, strconv.Itoa(cfg.ConnectTimeoutMS), strconv.Itoa(cfg.ConnectRetries), strconv.Itoa(cfg.RetryDelayMS)}
	form.fields = make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.Prompt = "  "
		input.CharLimit = 256
		input.SetWidth(42)
		input.SetValue(value)
		form.fields[i] = input
	}
	form.focus = 0
	return form.fields[0].Focus()
}

func (form *Form) configuration() (config.Config, error) {
	cfg := form.base
	cfg.ListenAddress = strings.TrimSpace(form.fields[0].Value())
	cfg.Distribution = form.fields[2].Value()
	cfg.ManualTargetIP = strings.TrimSpace(form.fields[4].Value())
	for _, field := range []struct {
		index  int
		target *int
	}{{1, &cfg.ListenPort}, {3, &cfg.TargetPort}, {5, &cfg.ConnectTimeoutMS}, {6, &cfg.ConnectRetries}, {7, &cfg.RetryDelayMS}} {
		value, err := strconv.Atoi(strings.TrimSpace(form.fields[field.index].Value()))
		if err != nil {
			return cfg, fmt.Errorf("%s must be an integer", fieldLabels[field.index])
		}
		*field.target = value
	}
	return cfg, cfg.Validate()
}

func (form *Form) SelectDistribution(name string) tea.Cmd {
	form.fields[2].SetValue(name)
	return form.fields[form.focus].Focus()
}

func (form *Form) Resize(width, height int) {
	for index := range form.fields {
		form.fields[index].SetWidth(max(12, min(46, width-12)))
	}
}

func (form *Form) Update(msg tea.Msg) (tea.Cmd, Intent) {
	if len(form.fields) == 0 {
		return nil, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+s":
			cfg, err := form.configuration()
			if err != nil {
				return nil, ShowErrorIntent{Err: err}
			}
			return nil, SaveConfigIntent{Config: cfg}
		case "enter":
			if form.focus == 2 {
				return nil, ChooseDistributionIntent{ForForm: true}
			}
			fallthrough
		case "tab", "down":
			form.fields[form.focus].Blur()
			form.focus = (form.focus + 1) % len(form.fields)
			return form.fields[form.focus].Focus(), nil
		case "shift+tab", "up":
			form.fields[form.focus].Blur()
			form.focus = (form.focus + len(form.fields) - 1) % len(form.fields)
			return form.fields[form.focus].Focus(), nil
		}
	}
	var cmd tea.Cmd
	form.fields[form.focus], cmd = form.fields[form.focus].Update(msg)
	return cmd, nil
}

func (form *Form) View(Snapshot) Content {
	body := ""
	start := max(0, form.focus-2)
	end := min(len(form.fields), start+4)
	for index := start; index < end; index++ {
		label := fieldLabels[index]
		if index == form.focus {
			label = accent.Render(label)
		} else {
			label = muted.Render(label)
		}
		body += label + "\n" + form.fields[index].View() + "\n"
	}
	body += muted.Render(fmt.Sprintf("Field %d/%d", form.focus+1, len(form.fields)))
	return Content{Body: body, Help: "Tab/Up/Down: field   Ctrl+S: save   Esc: discard"}
}
