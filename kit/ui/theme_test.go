// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func TestTruncate(t *testing.T) {
	styled := "\x1b[1mstyled text here\x1b[0m" // lipgloss renders no styling without a terminal
	for _, tc := range []struct {
		in   string
		n    int
		want string // "" skips the exact check
	}{
		{"abcdef", 4, "abc…"},
		{"abc", 3, "abc"},
		{"abc", 10, "abc"},
		{"abc", 0, ""},
		{"abc", -1, ""},
		{"日本語のタイトル", 6, "日本…"},
		{"日本語", 6, "日本語"},
		{"🎉🎉🎉🎉", 5, "🎉🎉…"},
		{styled, 8, ""},
	} {
		got := Truncate(tc.in, tc.n)
		if w := ansi.StringWidth(got); w > max(tc.n, 0) {
			t.Errorf("Truncate(%q, %d) = %q is %d cells wide", tc.in, tc.n, got, w)
		}
		if tc.want != "" && got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
	if got := Truncate(styled, 8); ansi.Strip(got) != "styled …" || !strings.Contains(got, "\x1b[") {
		t.Errorf("styling is kept and not counted: %q", got)
	}
}

func TestPad(t *testing.T) {
	styled := "\x1b[1mok\x1b[0m"
	for _, tc := range []struct {
		in    string
		n     int
		width int
	}{
		{"ab", 4, 4},
		{"abcdef", 4, 6}, // never cut
		{"日本", 6, 6},
		{"🎉", 4, 4},
		{styled, 5, 5},
	} {
		if w := ansi.StringWidth(Pad(tc.in, tc.n)); w != tc.width {
			t.Errorf("Pad(%q, %d) is %d cells, want %d", tc.in, tc.n, w, tc.width)
		}
	}
}

func TestSeverityColor(t *testing.T) {
	for sev, want := range map[string]string{
		"critical": Danger, "high": High, "medium": Warn, "low": Low, "info": Muted, "": Muted, "HIGH": Muted,
	} {
		if got := SeverityColor(sev); string(got) != want {
			t.Errorf("SeverityColor(%q) = %s, want %s", sev, got, want)
		}
	}
}
