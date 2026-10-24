// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"strconv"
	"strings"
)

// Severity is how serious a finding is, ordered so a higher value is
// worse. SeverityNone is the zero value and also means "--fail-on none".
type Severity int

// The severities, from none to critical.
const (
	SeverityNone Severity = iota
	SeverityInfo
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

var severityNames = [...]string{"none", "info", "low", "medium", "high", "critical"}

// String returns the lower-case name users type ("high").
func (s Severity) String() string {
	if s < 0 || int(s) >= len(severityNames) {
		return "Severity(" + strconv.Itoa(int(s)) + ")"
	}
	return severityNames[s]
}

// Fails reports whether a finding of severity found trips a --fail-on
// threshold of s: never for none, otherwise when found is at least s.
func (s Severity) Fails(found Severity) bool {
	return s != SeverityNone && found >= s
}

// ParseFailOn parses a --fail-on value in any case: none, info, low,
// medium, high or critical. An empty value means the flag wasn't given and
// parses as none, so a run fails only when the user asked it to. Anything
// else is a UsageError that names the accepted values.
func ParseFailOn(s string) (Severity, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return SeverityNone, nil
	}
	for i, name := range severityNames {
		if v == name {
			return Severity(i), nil
		}
	}
	return SeverityNone, Usagef("--fail-on %q: want none, info, low, medium, high or critical", s)
}
