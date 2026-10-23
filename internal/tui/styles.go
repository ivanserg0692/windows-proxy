package tui

import (
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
	snapshot := screenSnapshot{config: m.config, proxyActive: m.running != nil}
	if m.active == m.runningView {
		if m.running != nil {
			snapshot.running = &runningSnapshot{config: m.running.Config, stats: m.running.Server.Snapshot()}
		}
		if m.height >= 32 {
			snapshot.recentEvents = m.logs.Lines()
		}
	}
	screen := m.active.view(snapshot)
	body, help := screen.body, screen.help
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
	body += screen.footer
	content += body + "\n\n" + muted.Render(help)
	frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4C617A")).Padding(1, 2).Width(width - 2).MaxWidth(width).MaxHeight(m.height - 1).Render(content)
	view := tea.NewView(frame)
	view.AltScreen = true
	view.WindowTitle = "WSL Proxy"
	return view
}
