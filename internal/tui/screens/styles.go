package screens

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#67D9CF")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#929DB2"))

func safe(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, text)
}
