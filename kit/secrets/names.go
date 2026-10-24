// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package secrets

import (
	"strings"
	"unicode"
)

// Words that make a name secret wherever they appear in it, so
// DB_PASSWORD, clientSecret and STRIPE_APIKEY all count.
var secretWords = []string{
	"password", "passwd", "passphrase", "secret", "token", "apikey", "api_key", "api-key",
	"authkey", "auth_key", "accesskey", "access_key", "privatekey", "private_key",
	"credential", "authorization",
}

// Words that make a name secret only as a whole part of it: PWD and
// DB_PW count while PWA_ENABLED does not.
var secretParts = map[string]bool{"pass": true, "pwd": true, "pw": true, "auth": true, "dsn": true, "otp": true, "key": true}

// Last parts that describe a secret rather than hold one, such as
// PASSWORD_MIN_LENGTH, TOKEN_TTL or DB_PASSWORD_FILE.
var describesSecret = map[string]bool{
	"url": true, "uri": true, "endpoint": true, "path": true, "file": true, "dir": true, "host": true,
	"port": true, "user": true, "username": true, "name": true, "type": true, "expiry": true,
	"expires": true, "ttl": true, "length": true, "len": true, "min": true, "max": true, "id": true,
	"enabled": true, "required": true, "field": true, "label": true, "policy": true, "reset": true,
	"hint": true, "mode": true, "method": true, "lifetime": true, "timeout": true,
}

// Shell variables whose names hold a secret word but whose values are
// ordinary context: PWD and OLDPWD are folders. Only the upper-case
// names count, so WordPress's pwd login field stays masked.
var shellVars = map[string]bool{"PWD": true, "OLDPWD": true}

// Words ending in "key" or "pass" that are not secrets.
var notSecret = map[string]bool{"monkey": true, "turkey": true, "hockey": true, "jockey": true, "whiskey": true, "donkey": true, "bypass": true, "compass": true, "overpass": true}

// Words before "key" that make it an ordinary lookup key: sort_key,
// cacheKey, primary_key.
var lookupKey = map[string]bool{
	"cache": true, "sort": true, "primary": true, "foreign": true, "partition": true, "routing": true,
	"idempotency": true, "hot": true, "shortcut": true, "lookup": true, "map": true, "object": true,
	"row": true, "unique": true, "group": true, "index": true, "meta": true, "translation": true,
	"i18n": true, "react": true, "storage": true, "dedupe": true,
}

// isSecretName reports whether a variable, field or parameter name looks
// like it holds a credential. Matching ignores case and splits the name
// at _, -, ., brackets and camelCase boundaries. A bare "key" is not
// enough on its own: it is too often an ordinary lookup key.
func isSecretName(name string) bool {
	if shellVars[name] {
		return false
	}
	parts := nameParts(name)
	if len(parts) == 0 || describesSecret[parts[len(parts)-1]] {
		return false
	}
	lower := strings.ToLower(name)
	for _, w := range secretWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	if len(parts) == 1 && parts[0] == "key" {
		return false
	}
	for i, p := range parts {
		if notSecret[p] || lookupKey[strings.TrimSuffix(p, "key")] || p == "key" && i > 0 && lookupKey[parts[i-1]] {
			continue
		}
		base := strings.TrimRight(p, "0123456789") // pass1, pass2
		if secretParts[base] || strings.HasSuffix(p, "key") || strings.HasSuffix(p, "pass") || strings.HasSuffix(p, "pwd") || strings.HasSuffix(p, "dsn") {
			return true
		}
	}
	return false
}

// nameParts splits name into lower-case parts at punctuation and at
// camelCase boundaries: "dbUserPwd" gives db, user, pwd.
func nameParts(name string) []string {
	var parts []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			parts = append(parts, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	var prev rune
	for _, r := range name {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && unicode.IsLower(prev):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
		prev = r
	}
	flush()
	return parts
}

// Values that only stand in for a secret.
var placeholderWords = map[string]bool{
	"null": true, "nil": true, "none": true, "true": true, "false": true, "empty": true,
	"changeme": true, "change-me": true, "change_me": true, "example": true, "placeholder": true,
	"redacted": true, "undefined": true,
}

// placeholder reports whether v is a stand-in rather than a secret:
// ${VAR}, {{ var }}, <your-key>, %VAR%, changeme, xxxx or ****, or a
// value that is already masked.
func placeholder(v string) bool {
	l := strings.ToLower(v)
	switch {
	case placeholderWords[l],
		strings.HasPrefix(v, "${"), strings.HasPrefix(v, "{{"),
		strings.HasPrefix(v, "<") && strings.HasSuffix(v, ">"),
		len(v) > 2 && strings.HasPrefix(v, "%") && strings.HasSuffix(v, "%"),
		strings.Contains(v, "•"),
		strings.Trim(l, "x*.-_ ") == "":
		return true
	}
	return false
}
