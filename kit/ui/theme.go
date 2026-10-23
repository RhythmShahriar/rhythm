// Package ui holds the shared terminal theme, from the Rhythm brand tokens
// (personal-portfolio/packages/brand/tokens.css): ink, paper and one dot.
package ui

import "github.com/charmbracelet/lipgloss"

// Brand tokens.
const (
	Ink       = "#100F0D"
	Paper     = "#FFFFFF"
	Dot       = "#FF7C52" // the signal: Frans Hals orange
	DotStrong = "#E8603A" // the same hue, deeper, for legibility on light backgrounds
)

// Semantic colors. Severities stay conventional so findings read at a glance.
const (
	Danger = "#EF4444"
	High   = "#F97316"
	Warn   = "#EAB308"
	Low    = "#60A5FA"
	Ok     = "#10B981"
	Muted  = "#6B7280"
)

// Signal is the brand dot, adjusted for the terminal's background.
var Signal = lipgloss.AdaptiveColor{Light: DotStrong, Dark: Dot}

var (
	Brand = lipgloss.NewStyle().Foreground(Signal).Bold(true)
	Label = lipgloss.NewStyle().Bold(true)
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
		return lipgloss.Color(Low)
	}
	return lipgloss.Color(Muted)
}
