// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package secrets finds credentials in text and masks them, so tools can
// report what they saw without ever printing a key, token or password.
//
// Rules (listed by Rules, each with an ID and tags) cover these kinds:
//
//   - cloud and infrastructure: AWS, Google Cloud, Azure, DigitalOcean,
//     Heroku, Alibaba, Cloudflare, Netlify, Vault, Terraform Cloud,
//     Doppler, Pulumi, Databricks, Supabase, PlanetScale, age and
//     1Password keys
//   - payment and commerce: Stripe, Square, Braintree, Shopify,
//     Flutterwave and shipping API keys
//   - mail and messaging: SendGrid, Mailgun, Mailchimp, Brevo and Resend
//     keys, Slack, Discord, Teams and Telegram tokens and webhooks, Twilio
//   - code, packages and CI: GitHub, GitLab and Atlassian tokens, npm,
//     PyPI, RubyGems, Docker Hub, NuGet and JFrog tokens, CircleCI,
//     Buildkite and SonarQube tokens
//   - SaaS and monitoring: Grafana, Sentry, New Relic, Linear, Notion,
//     Figma, Airtable, HubSpot and similar API tokens
//   - AI: Anthropic, OpenAI, OpenRouter, Hugging Face, Replicate, Groq,
//     Perplexity and xAI keys
//   - generic shapes: private keys in PEM blocks, JWTs, URLs with a user
//     and password, Authorization and Cookie headers, token headers,
//     password reset links and codes, OAuth callback codes, /key/ and
//     /token/ path segments, curl -u and .npmrc credentials
//   - secret-looking names (password, api_key, AUTH_KEY...) in
//     wp-config.php, PHP serialized data, .env files, YAML, JSON, code and
//     form bodies
//
// Masking policy: names stay visible and values are masked. A variable,
// header or cookie name is kept as written, so a report still says which
// setting leaked. A value keeps at most 4 leading characters (never more
// than a third of it) followed by a fixed run of bullets, so neither the
// secret nor its length is printed. A value under 3 characters is left
// alone, except when its name looks like a secret's or it is a URL or
// curl password, which are masked at any length. FindSecrets returns what it found,
// Redact masks secrets inside text and RedactHeader masks one HTTP header.
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

// String formats the secret as name=masked, or just masked when it has
// no name. It is always safe to print.
func (s Secret) String() string {
	if s.Name != "" {
		return s.Name + "=" + s.Masked
	}
	return s.Masked
}

// RuleInfo describes one detection rule, for listings such as
// `rhythm-tuner checks --tag secrets`.
type RuleInfo struct {
	ID          string   `json:"id"` // the Kind of the secrets it finds
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// Rules returns every detection rule in the order they are applied.
func Rules() []RuleInfo {
	out := make([]RuleInfo, len(secretRules))
	for i, r := range secretRules {
		out[i] = RuleInfo{ID: r.id, Description: r.desc, Tags: append([]string(nil), r.tags...)}
	}
	return out
}

type secretRule struct {
	id   string
	desc string
	tags []string
	// hint lists lower-case literals one of which must appear in the
	// text for the rule to run, so most rules skip their regex.
	hint []string
	re   *regexp.Regexp
	// name is the capture group holding the variable name, 0 for none.
	name int
	// value is the capture group holding the secret. When a regex has
	// alternatives (quoted or bare), the first group from value on that
	// took part in the match holds it.
	value int
	keep  int // leading characters left visible
	// secretName requires the name to look like a secret's (see
	// isSecretName), for rules that match any key = value shape.
	secretName bool
	// valid, when set, decides whether a match counts.
	valid func(s string, m []int) bool
	// split, when set, finds several secrets inside the value, such as
	// each value in a Cookie header.
	split func(v string) []part
	// multiline rules run over the whole text instead of line by line.
	multiline bool
	// anyLength masks values shorter than 3 characters too, for rules
	// whose shape alone says the value is a secret.
	anyLength bool
}

// part is one secret inside a larger value, at v[start:end].
type part struct {
	name       string
	start, end int
}

type span struct {
	start, end int
	secret     Secret
}

// findSpans runs every rule over s. Rules run line by line, which keeps
// Go's regexp engine on its fast path for short inputs, unless they are
// marked multiline. A rule's hint is checked on the whole text first and
// then on each line, so most lines skip most rules.
func findSpans(s string) []span {
	var spans []span
	lower := asciiLower(s)
	add := func(r *secretRule, name string, vs, ve int) {
		if r.secretName {
			// "password: Bearer abc" keeps the scheme word readable.
			if m := schemePrefix.FindStringIndex(s[vs:ve]); m != nil {
				vs += m[1] - 1 // up to the token, not into it
			}
		}
		v := s[vs:ve]
		// A value named as a secret (password=ab) or written as a URL
		// password is masked however short. Elsewhere one or two
		// characters (a=1 in a cookie) are too short to hold a secret.
		if v == "" || len(v) < 3 && !r.secretName && !r.anyLength || placeholder(v) {
			return
		}
		spans = append(spans, span{vs, ve, Secret{Kind: r.id, Name: name, Masked: Mask(v, r.keep)}})
	}
	apply := func(r *secretRule, text string, off int) {
		for _, m := range r.re.FindAllStringSubmatchIndex(text, -1) {
			if r.valid != nil && !r.valid(text, m) {
				continue
			}
			name := ""
			if r.name > 0 {
				name = text[m[2*r.name]:m[2*r.name+1]]
				if r.secretName && !isSecretName(name) {
					continue
				}
			}
			vs, ve := group(m, r.value)
			if r.split == nil {
				add(r, name, off+vs, off+ve)
				continue
			}
			for _, p := range r.split(text[vs:ve]) {
				add(r, p.name, off+vs+p.start, off+vs+p.end)
			}
		}
	}
	for i := range secretRules {
		r := &secretRules[i]
		if len(r.hint) > 0 && !containsAny(lower, r.hint) {
			continue
		}
		if r.multiline {
			apply(r, s, 0)
			continue
		}
		for ls := 0; ls < len(s); {
			le := strings.IndexByte(s[ls:], '\n')
			if le < 0 {
				le = len(s)
			} else {
				le += ls
			}
			if len(r.hint) == 0 || containsAny(lower[ls:le], r.hint) {
				apply(r, s[ls:le], ls)
			}
			ls = le + 1
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

// schemePrefix is an HTTP auth scheme word and the space after it, at the
// start of a value with more after it.
var schemePrefix = regexp.MustCompile(`^` + scheme + `[ \t]+\S`)

// group returns the bounds of the first capture group from g on that took
// part in match m. Rules put the value's alternatives last, so the last
// group is the fallback.
func group(m []int, g int) (int, int) {
	for 2*g+3 < len(m) && m[2*g] < 0 {
		g++
	}
	return m[2*g], m[2*g+1]
}

// asciiLower lower-cases ASCII letters only, so offsets into the result
// match offsets into s.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
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

// Redact replaces every secret value in s with its masked form. Names
// (the variable, header or cookie name) stay readable.
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

// secretHeader matches header names that carry a credential whole.
var secretHeader = regexp.MustCompile(`(?i)^(?:(?:proxy-)?authorization|x-[a-z0-9-]*(?:token|key|secret|auth|nonce|signature|password)[a-z0-9-]*|api-?key|private-token)$`)

// RedactHeader masks one HTTP header. A credential header (Authorization,
// X-Auth-Token, X-Api-Key, X-WP-Nonce, or a secret-looking name) is
// masked whole. Cookie and Set-Cookie keep their cookie names and
// attributes and lose every value. Any other header goes through Redact.
func RedactHeader(name, value string) string {
	switch {
	case strings.TrimSpace(value) == "":
		return value
	case strings.EqualFold(name, "cookie") || strings.EqualFold(name, "set-cookie"):
		var b strings.Builder
		last := 0
		for _, p := range cookieValues(value) {
			if p.end-p.start == 0 {
				continue
			}
			b.WriteString(value[last:p.start])
			b.WriteString(Mask(value[p.start:p.end], 0))
			last = p.end
		}
		b.WriteString(value[last:])
		return b.String()
	case secretHeader.MatchString(name) || isSecretName(name):
		return Mask(value, 0)
	}
	return Redact(value)
}

// maxKeep is the most leading characters Mask ever leaves visible.
const maxKeep = 4

// Mask keeps at most keep leading characters (never more than 4, nor more
// than a third of the value) and hides the rest. The number of bullets is
// fixed so the length of the secret does not leak.
func Mask(v string, keep int) string {
	r := []rune(v)
	keep = max(0, min(keep, maxKeep, len(r)/3))
	return string(r[:keep]) + "•••••"
}
