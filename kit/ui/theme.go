// Package ui holds the shared terminal theme.
// Replace Primary and Accent with the colors from the brand assets.
package ui

import "github.com/charmbracelet/lipgloss"

const (
	Primary = "#7C3AED" // TODO: brand primary
	Accent  = "#22D3EE" // TODO: brand accent
	Danger  = "#EF4444"
	High    = "#F97316"
	Warn    = "#F59E0B"
	Ok      = "#10B981"
	Muted   = "#6B7280"
)

var (
	Brand = lipgloss.NewStyle().Foreground(lipgloss.Color(Primary)).Bold(true)
	Label = lipgloss.NewStyle().Foreground(lipgloss.Color(Accent))
	Dim   = lipgloss.NewStyle().Foreground(lipgloss.Color(Muted))
	Good  = lipgloss.NewStyle().Foreground(lipgloss.Color(Ok))
)

// SeverityColor maps a finding severity to its theme color.
func SeverityColor(sev string) lipgloss.Color {
	switch sev {
	case "critical":
		return lipgloss.Color(Danger)
	case "high":
		return lipgloss.Color(High)
	case "medium":
		return lipgloss.Color(Warn)
	case "low":
		return lipgloss.Color(Accent)
	}
	return lipgloss.Color(Muted)
}
