package tui

import "charm.land/lipgloss/v2"

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#67D9CF")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#929DB2"))
var danger = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8F9C"))
var frameStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#4C617A")).
	Padding(1, 2)

func styleViewData(data viewData) viewData {
	data.Title = accent.Render(data.Title)
	data.Status = accent.Render(data.Status)
	data.Subtitle = muted.Render(data.Subtitle)
	if data.Busy != "" {
		data.Busy = accent.Render(data.Busy)
	}
	if data.Error != "" {
		data.Error = danger.Render(data.Error)
	}
	if data.Notice != "" {
		data.Notice = muted.Render(data.Notice)
	}
	data.Help = muted.Render(data.Help)
	return data
}
