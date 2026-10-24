// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package secrets finds credentials in text and masks them, so tools can
// report what they saw without ever printing a key, token or password.
// Rules cover common cloud, payment and messaging providers, HTTP
// authorization headers, URLs with credentials, and secret-looking keys in
// config files, JSON and form bodies.
package secrets

import (
	"regexp"
	"sort"
	"strings"
)

// Secret is a detected credential. The raw value never leaves this package.
type Secret struct {
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Masked string `json:"masked"`
}

func (s Secret) String() string {
	if s.Name != "" {
		return s.Name + "=" + s.Masked
	}
	return s.Masked
}

type secretRule struct {
	kind  string
	re    *regexp.Regexp
	name  int // capture group holding the variable name, 0 = none
	value int // capture group holding the secret
	keep  int // leading characters left visible
}

var secretRules = []secretRule{
	// Specific formats first: on overlap the earlier rule wins.
	{"aws-access-key", regexp.MustCompile(`\b((?:AKIA|ASIA)[0-9A-Z]{16})\b`), 0, 1, 4},
	{"stripe-key", regexp.MustCompile(`\b((?:sk|rk)_(?:live|test)_[0-9a-zA-Z]{16,})`), 0, 1, 8},
	{"stripe-webhook-secret", regexp.MustCompile(`\b(whsec_[0-9a-zA-Z]{16,})`), 0, 1, 6},
	{"google-api-key", regexp.MustCompile(`\b(AIza[0-9A-Za-z_\-]{35})`), 0, 1, 4},
	{"github-token", regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{36,})`), 0, 1, 4},
	{"slack-token", regexp.MustCompile(`\b(xox[abposr]-[0-9A-Za-z-]{10,})`), 0, 1, 5},
	{"slack-webhook", regexp.MustCompile(`hooks\.slack\.com/(?:services|workflows|triggers)/[A-Z0-9]+/[A-Z0-9]+/([0-9A-Za-z]{8,})`), 0, 1, 2},
	{"discord-webhook", regexp.MustCompile(`discord(?:app)?\.com/api/webhooks/\d+/([0-9A-Za-z_\-]{20,})`), 0, 1, 2},
	{"sendgrid-key", regexp.MustCompile(`\b(SG\.[0-9A-Za-z_\-]{16,}\.[0-9A-Za-z_\-]{16,})`), 0, 1, 3},
	{"mailgun-key", regexp.MustCompile(`\b(key-[0-9a-f]{32})\b`), 0, 1, 4},
	{"resend-key", regexp.MustCompile(`\b(re_[0-9A-Za-z_]{20,})`), 0, 1, 3},
	{"twilio-api-key", regexp.MustCompile(`\b(SK[0-9a-f]{32})\b`), 0, 1, 2},
	{"private-key", regexp.MustCompile(`-----BEGIN ([A-Z ]*PRIVATE KEY)-----\s*([A-Za-z0-9+/=\s]{16,})`), 1, 2, 0},
	{"url-credentials", regexp.MustCompile(`\b[a-z][a-z0-9+]*://[^:\s/@"']+:([^@\s/"']+)@`), 0, 1, 2},
	{"authorization", regexp.MustCompile(`(?i)\b(authorization|x-postmark-server-token|x-api-key|api-key)["']?\s*[:=]\s*["']?(?:(?:bearer|basic|token)\s+)?([^\s"',;]{6,})`), 1, 2, 0},
	{"wp-config", regexp.MustCompile(`define\(\s*['"]((?:DB_PASSWORD|DB_USER)|(?:SECURE_)?AUTH_(?:KEY|SALT)|LOGGED_IN_(?:KEY|SALT)|NONCE_(?:KEY|SALT))['"]\s*,\s*['"]([^'"]+)['"]`), 1, 2, 2},
	{"env", regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?([A-Z0-9_]*(?:PASS|PASSWORD|PASSWD|SECRET|_KEY|TOKEN|AUTH)[A-Z0-9_]*)[ \t]*=[ \t]*["']?([^\s"'#]+)`), 1, 2, 2},
	{"json-field", regexp.MustCompile(`(?i)"([a-z0-9_\-]*(?:password|passwd|secret|token|api_?key|auth_?key|access_?key)[a-z0-9_\-]*)"\s*:\s*"([^"]+)"`), 1, 2, 2},
	{"form-field", regexp.MustCompile(`(?i)(?:^|[?&\s])([a-z0-9_\-]*(?:password|passwd|secret|token|api_?key|auth_?key|access_?key)[a-z0-9_\-]*)=([^&\s"']+)`), 1, 2, 2},
}

// Values that look like secrets but are only placeholders.
var placeholders = map[string]bool{"null": true, "true": true, "false": true, "none": true, "empty": true}

type span struct {
	start, end int
	secret     Secret
}

func findSpans(s string) []span {
	var spans []span
	for _, r := range secretRules {
		for _, m := range r.re.FindAllStringSubmatchIndex(s, -1) {
			vs, ve := m[2*r.value], m[2*r.value+1]
			v := s[vs:ve]
			if len(v) < 3 || placeholders[strings.ToLower(v)] || strings.HasPrefix(v, "${") {
				continue
			}
			sec := Secret{Kind: r.kind, Masked: Mask(v, r.keep)}
			if r.name > 0 {
				sec.Name = s[m[2*r.name]:m[2*r.name+1]]
			}
			spans = append(spans, span{vs, ve, sec})
		}
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	// Drop overlaps; earlier rules are more specific and were appended first.
	out := spans[:0]
	end := -1
	for _, sp := range spans {
		if sp.start >= end {
			out = append(out, sp)
			end = sp.end
		}
	}
	return out
}

// FindSecrets returns every secret in b, masked and de-duplicated.
func FindSecrets(b []byte) []Secret {
	var out []Secret
	seen := map[Secret]bool{}
	for _, sp := range findSpans(string(b)) {
		if !seen[sp.secret] {
			seen[sp.secret] = true
			out = append(out, sp.secret)
		}
	}
	return out
}

// Redact replaces every secret value in s with its masked form.
func Redact(s string) string {
	spans := findSpans(s)
	if len(spans) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		b.WriteString(s[last:sp.start])
		b.WriteString(sp.secret.Masked)
		last = sp.end
	}
	b.WriteString(s[last:])
	return b.String()
}

// Mask keeps at most keep leading characters and hides the rest. The
// number of bullets is fixed so the length of the secret does not leak.
func Mask(v string, keep int) string {
	r := []rune(v)
	if keep > len(r)/3 {
		keep = len(r) / 3
	}
	return string(r[:keep]) + "•••••"
}
