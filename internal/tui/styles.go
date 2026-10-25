package tui

import "charm.land/lipgloss/v2"

const (
	frameVerticalPadding   = 1
	frameHorizontalPadding = 2
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#67D9CF")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#929DB2"))
var danger = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8F9C"))
var frameStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#4C617A")).
	Padding(frameVerticalPadding, frameHorizontalPadding)

// Block spacing belongs to presentation styles and participates in measurement.
var blockStyle = lipgloss.NewStyle().MarginBottom(1)
var titleStyle = accent.MarginRight(2)
