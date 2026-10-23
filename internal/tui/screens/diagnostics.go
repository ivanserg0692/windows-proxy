package screens

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
)

type Diagnostics struct{ report app.Report }

func (diagnostics *Diagnostics) SetReport(report app.Report) {
	diagnostics.report = report
}

func (diagnostics *Diagnostics) Resize(width, height int) {}

func (diagnostics *Diagnostics) Update(msg tea.Msg) (tea.Cmd, Intent) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "r" {
		return nil, DiagnoseIntent{}
	}
	return nil, nil
}

func (diagnostics *Diagnostics) View(snapshot Snapshot) Content {
	cfg := snapshot.Config
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
	return Content{Body: body, Help: "R: run diagnostics   Esc: menu"}
}
