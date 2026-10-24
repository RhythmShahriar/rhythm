// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package ui holds the shared terminal theme, from the Rhythm brand tokens
// (personal-portfolio/packages/brand/tokens.css): ink, paper and one dot.
package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

// Regression palette, for tools that compare a run against an earlier one.
const (
	Broke   = Danger // worked before, fails now
	New     = High   // a problem that wasn't there before
	Changed = Warn   // different, maybe fine
	Slower  = "#A78BFA"
	Fixed   = Ok
)

// Badge renders a label as a fixed-width tag: filled for confirmed
// results, outlined (colored text) for uncertain ones.
func Badge(label string, color lipgloss.TerminalColor, filled bool, width int) string {
	text := " " + Pad(label, width-2) + " "
	if !filled {
		return lipgloss.NewStyle().Foreground(color).Bold(true).Render(text)
	}
	return lipgloss.NewStyle().Background(color).Foreground(lipgloss.Color("#0B0B0C")).Bold(true).Render(text)
}

// Section renders "LABEL  n ───…" as a section heading width cells wide.
func Section(label string, n int, color lipgloss.TerminalColor, width int) string {
	head := lipgloss.NewStyle().Foreground(color).Bold(true).Render(label + "  " + strconv.Itoa(n))
	return head + " " + Dim.Render(strings.Repeat("─", max(2, width-lipgloss.Width(head)-1)))
}

// Panel draws content in a rounded box with a colored border.
func Panel(content string, color lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(color).
		Padding(0, 2).MarginLeft(1).Render(content)
}

// Truncate shortens s to n cells, ending in "…".
func Truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:max(0, n-1)]) + "…"
	}
	return s
}

// Pad right-pads s with spaces to n cells (ANSI-aware).
func Pad(s string, n int) string {
	if w := lipgloss.Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}
