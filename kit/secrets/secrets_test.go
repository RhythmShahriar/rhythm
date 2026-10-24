// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package secrets

import (
	"strings"
	"testing"
)

func TestFindSecretsMasks(t *testing.T) {
	raw := map[string]string{
		"DB_PASSWORD=supersecret\n":                                     "supersecret",
		"STRIPE_KEY=sk_live_abcdefghijklmnop1234\n":                     "sk_live_abcdefghijklmnop1234",
		"key AKIAABCDEFGHIJKLMNOP here":                                 "AKIAABCDEFGHIJKLMNOP",
		"define( 'DB_PASSWORD', 'wp-pass-123' );":                       "wp-pass-123",
		"DATABASE_URL=mysql://root:hunter22@db:3306/app":                "hunter22",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAx1234567\n":   "MIIEowIBAAKCAQEAx1234567",
		"Authorization: Bearer re_live_abcdefghijklmnopqrstu":           "abcdefghijklmnopqrstu",
		"Authorization: Basic QUMxMjM6c2VjcmV0dG9rZW4=":                 "QUMxMjM6c2VjcmV0dG9rZW4=",
		`{"api_key":"abc123def456ghi","to":"+15550001"}`:                "abc123def456ghi",
		"From=%2B1555&Body=hi&access_token=EAAGm0PX4ZCpsBAF0q":          "EAAGm0PX4ZCpsBAF0q",
		"x-postmark-server-token: 1b2c3d4e-0000-4f00-8000-123456789abc": "1b2c3d4e-0000-4f00-8000-123456789abc",
	}
	// Provider-format fixtures are assembled at run time, so the source
	// never contains anything a secret scanner would flag.
	fake := func(parts ...string) string { return strings.Join(parts, "") }
	for in, secret := range map[string]string{
		fake("https://hooks.slack.com/services/T0001/B0002/", "XXXXabcd", "YYYYefgh"): "XXXXabcdYYYYefgh",
		fake("whs", "ec_", "1234567890abcdefghijklmn"):                                "1234567890abcdefghijklmn",
		fake("S", "G.", strings.Repeat("a", 22), ".", strings.Repeat("B", 43)):        strings.Repeat("B", 43),
		fake("ke", "y-", strings.Repeat("0f", 16)):                                    strings.Repeat("0f", 16),
	} {
		raw[in] = secret
	}
	for in, secret := range raw {
		got := FindSecrets([]byte(in))
		if len(got) == 0 {
			t.Errorf("%q: no secret found", in)
			continue
		}
		for _, s := range got {
			if strings.Contains(s.String(), secret) {
				t.Errorf("%q: masked output %q leaks the secret", in, s)
			}
		}
		if red := Redact(in); strings.Contains(red, secret) {
			t.Errorf("Redact(%q) = %q leaks the secret", in, red)
		}
	}
}

func TestFindSecretsSkipsPlaceholders(t *testing.T) {
	for _, in := range []string{"DB_PASSWORD=\n", "DB_PASSWORD=null\n", "API_KEY=${API_KEY}\n", "APP_NAME=Laravel\n"} {
		if got := FindSecrets([]byte(in)); len(got) != 0 {
			t.Errorf("%q: unexpected secrets %v", in, got)
		}
	}
}

func TestMask(t *testing.T) {
	if got := Mask("supersecret", 2); got != "su•••••" {
		t.Errorf("Mask = %q", got)
	}
	if got := Mask("abc", 8); got != "a•••••" {
		t.Errorf("short Mask = %q", got)
	}
}

func TestRedactKeepsOrdinaryText(t *testing.T) {
	in := `{"to":"+15550001111","body":"Your order 1234 has shipped","subject":"Thanks"}`
	if got := Redact(in); got != in {
		t.Errorf("Redact changed text without secrets: %q", got)
	}
}
