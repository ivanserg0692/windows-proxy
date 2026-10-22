package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#67D9CF")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#929DB2"))
var danger = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8F9C"))

func (m *model) View() tea.View {
	if m.width < 40 || m.height < 20 {
		view := tea.NewView("WSL PROXY\nResize terminal to at least 40 x 20.\nCtrl+C to exit.")
		view.AltScreen = true
		return view
	}
	width := min(72, m.width-4)
	status := "STOPPED"
	if m.running != nil {
		status = "RUNNING"
	}
	content := accent.Render("WSL PROXY") + "                         " + accent.Render(status) + "\n" + muted.Render("Windows -> WSL2 HTTP Reverse Proxy") + "\n\n"
	var body, help string
	switch m.screen {
	case menuScreen:
		body = m.menu.View()
		help = "Up/Down: select   Enter: open   Ctrl+C: exit"
		if m.running != nil {
			body += "\n" + muted.Render("Proxy remains active. Start proxy opens its status.")
		}
	case formScreen:
		start := max(0, m.focus-2)
		end := min(len(m.fields), start+4)
		for i := start; i < end; i++ {
			label := fieldLabels[i]
			if i == m.focus {
				label = accent.Render(label)
			} else {
				label = muted.Render(label)
			}
			body += label + "\n" + m.fields[i].View() + "\n"
		}
		body += muted.Render(fmt.Sprintf("Field %d/%d", m.focus+1, len(m.fields)))
		help = "Tab/Up/Down: field   Ctrl+S: save   Esc: discard"
	case distributionsScreen:
		body = "Select WSL distribution\n\n" + m.distributions.View()
		help = "Up/Down: select   Enter: use   R: refresh   Esc: back"
	case diagnosticsScreen:
		body = fmt.Sprintf("Windows: %s\nWSL: %s, port %d\n\n", m.config.ListenEndpoint(), safe(m.config.Distribution), m.config.TargetPort)
		for i, probe := range m.report.Probes {
			if i >= 4 {
				body += "More addresses found; use manual IP to narrow selection.\n"
				break
			}
			result := fmt.Sprintf("OK (%s)", probe.Duration.Round(1000000))
			if probe.Error != "" {
				result = "FAIL: " + safe(probe.Error)
			}
			body += fmt.Sprintf("%s [%s]: %s\n", probe.Candidate.IP, safe(probe.Candidate.Interface), result)
		}
		if m.report.Selected.IsValid() {
			body += "\nSelected: " + m.report.Selected.String()
		}
		help = "R: run diagnostics   Esc: menu"
	case runningScreen:
		if m.running != nil {
			stats := m.running.Server.Snapshot()
			body = fmt.Sprintf("Windows:  %s\nWSL:      %s\nUpstream: %s:%d\n\nRequests: %d\nRetries:  %d\nFailures: %d", m.running.Config.ListenEndpoint(), safe(m.running.Config.Distribution), stats.Upstream, m.running.Config.TargetPort, stats.Requests, stats.Retries, stats.Failures)
		}
		help = "S: stop   Esc: menu (proxy stays active)"
	}
	if m.busy != "" {
		body += "\n\n" + accent.Render(m.busy)
		help = "Ctrl+C: cancel and exit"
	}
	if m.error != "" {
		body += "\n\n" + danger.Render(safe(m.error))
	}
	if m.notice != "" {
		body += "\n\n" + muted.Render(safe(m.notice))
	}
	if m.screen == runningScreen && m.height >= 32 {
		lines := m.logs.Lines()
		if len(lines) > 3 {
			lines = lines[len(lines)-3:]
		}
		if len(lines) > 0 {
			body += "\n\n" + muted.Render("Recent events\n"+strings.Join(lines, "\n"))
		}
	}
	content += body + "\n\n" + muted.Render(help)
	frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4C617A")).Padding(1, 2).Width(width - 2).MaxWidth(width).MaxHeight(m.height - 1).Render(content)
	view := tea.NewView(frame)
	view.AltScreen = true
	view.WindowTitle = "WSL Proxy"
	return view
}
