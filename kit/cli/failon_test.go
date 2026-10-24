// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"errors"
	"testing"
)

func TestParseFailOn(t *testing.T) {
	cases := []struct {
		in   string
		want Severity
		ok   bool
	}{
		{"", SeverityNone, true},
		{"none", SeverityNone, true},
		{"NONE", SeverityNone, true},
		{"info", SeverityInfo, true},
		{"low", SeverityLow, true},
		{"Medium", SeverityMedium, true},
		{"high", SeverityHigh, true},
		{"HIGH", SeverityHigh, true},
		{" critical ", SeverityCritical, true},
		{"crit", SeverityNone, false},
		{"warning", SeverityNone, false},
		{"1", SeverityNone, false},
		{"high,low", SeverityNone, false},
	}
	for _, c := range cases {
		got, err := ParseFailOn(c.in)
		if got != c.want || (err == nil) != c.ok {
			t.Errorf("ParseFailOn(%q) = %v, %v", c.in, got, err)
		}
		var ue UsageError
		if err != nil && !errors.As(err, &ue) {
			t.Errorf("ParseFailOn(%q): %v isn't a UsageError", c.in, err)
		}
	}
	_, err := ParseFailOn("Crit")
	if err == nil || err.Error() != `--fail-on "Crit": want none, info, low, medium, high or critical` {
		t.Errorf("message: %v", err)
	}
}

func TestSeverity(t *testing.T) {
	for s := SeverityNone; s <= SeverityCritical; s++ {
		if back, err := ParseFailOn(s.String()); err != nil || back != s {
			t.Errorf("%v doesn't round-trip: %v %v", s, back, err)
		}
	}
	if got := Severity(9).String(); got != "Severity(9)" {
		t.Errorf("out of range: %q", got)
	}
	cases := []struct {
		threshold, found Severity
		fails            bool
	}{
		{SeverityNone, SeverityCritical, false},
		{SeverityHigh, SeverityCritical, true},
		{SeverityHigh, SeverityHigh, true},
		{SeverityHigh, SeverityMedium, false},
		{SeverityInfo, SeverityInfo, true},
		{SeverityLow, SeverityNone, false},
	}
	for _, c := range cases {
		if got := c.threshold.Fails(c.found); got != c.fails {
			t.Errorf("%v.Fails(%v) = %v", c.threshold, c.found, got)
		}
	}
}
