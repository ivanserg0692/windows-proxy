package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
)

type diagnosticsScreen struct{ report app.Report }

func (diagnostics *diagnosticsScreen) setReport(report app.Report) {
	diagnostics.report = report
}

func (diagnostics *diagnosticsScreen) resize(width, height int) {}

func (diagnostics *diagnosticsScreen) update(msg tea.Msg) (tea.Cmd, screenIntent) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "r" {
		return nil, diagnoseIntent{}
	}
	return nil, nil
}

func (diagnostics *diagnosticsScreen) view(snapshot screenSnapshot) screenContent {
	cfg := snapshot.config
	body := fmt.Sprintf("Windows: %s\nWSL: %s, port %d\n\n", cfg.ListenEndpoint(), safe(cfg.Distribution), cfg.TargetPort)
	for index, probe := range diagnostics.report.Probes {
		if index >= 4 {
			body += "More addresses found; use manual IP to narrow selection.\n"
			break
		}
		result := fmt.Sprintf("OK (%s)", probe.Duration.Round(1000000))
		if probe.Error != "" {
			result = "FAIL: " + safe(probe.Error)
		}
		body += fmt.Sprintf("%s [%s]: %s\n", probe.Candidate.IP, safe(probe.Candidate.Interface), result)
	}
	if diagnostics.report.Selected.IsValid() {
		body += "\nSelected: " + diagnostics.report.Selected.String()
	}
	return screenContent{body: body, help: "R: run diagnostics   Esc: menu"}
}
