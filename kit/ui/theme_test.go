package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHelpers(t *testing.T) {
	if got := Truncate("abcdef", 4); got != "abc…" {
		t.Errorf("Truncate = %q", got)
	}
	if got := Pad("ab", 4); got != "ab  " {
		t.Errorf("Pad = %q", got)
	}
	if w := lipgloss.Width(Badge("BROKE", lipgloss.Color(Broke), true, 10)); w != 10 {
		t.Errorf("filled badge width = %d", w)
	}
	if w := lipgloss.Width(Badge("NEW", lipgloss.Color(New), false, 10)); w != 10 {
		t.Errorf("outline badge width = %d", w)
	}
	if s := Section("BROKE", 3, lipgloss.Color(Broke), 40); lipgloss.Width(s) != 40 || !strings.Contains(s, "BROKE  3") {
		t.Errorf("Section = %q (width %d)", s, lipgloss.Width(s))
	}
	if !strings.Contains(Panel("x", lipgloss.Color(Ok)), "╭") {
		t.Error("Panel should draw a rounded box")
	}
}
