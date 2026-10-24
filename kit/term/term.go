// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package term makes untrusted text safe to show: in a terminal, where
// control bytes can move the cursor, retitle the window or write the
// clipboard (OSC 52) and in Markdown, where a page's text could become
// an image, a link or a broken table. Response bodies, headers, file
// names and plugin slugs all come from the site being checked, so every
// tool runs them through here before printing or writing a report.
package term

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// bidi reports whether r is a bidirectional embedding, override or
// isolate (U+202A-U+202E, U+2066-U+2069). They can make text read
// differently from what it is (Trojan Source), so they are dropped.
func bidi(r rune) bool { return r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 }

// control reports whether r is a C0 control other than tab and newline,
// DEL or a C1 control.
func control(r rune) bool { return r < 0x20 && r != '\t' && r != '\n' || r >= 0x7F && r <= 0x9F }

// Sanitize makes s safe for a terminal and shows what was removed: C0
// controls other than tab and newline, DEL and C1 controls become
// escapes such as \x1b and \u009b, bytes that are not UTF-8 become \xff,
// and bidirectional overrides are dropped. Use it where a person may need
// to see the raw bytes, such as a request log.
func Sanitize(s string) string {
	return clean(s, func(b *strings.Builder, r rune, invalid byte) {
		switch {
		case invalid != 0:
			fmt.Fprintf(b, `\x%02x`, invalid)
		case r < 0x80:
			fmt.Fprintf(b, `\x%02x`, r)
		default:
			fmt.Fprintf(b, `\u%04x`, r)
		}
	}, true)
}

// SafeText makes s safe for any output by replacing control characters
// (C0 including newline, DEL, C1) and bytes that are not UTF-8 with
// U+FFFD and dropping bidirectional overrides. Tabs stay. Use
// SafeLines where line breaks are wanted.
func SafeText(s string) string { return clean(s, replace, false) }

// SafeLines is SafeText that keeps newlines.
func SafeLines(s string) string { return clean(s, replace, true) }

func replace(b *strings.Builder, _ rune, _ byte) { b.WriteRune(utf8.RuneError) }

// clean copies s, passing each unsafe rune (or invalid byte) to bad.
func clean(s string, bad func(b *strings.Builder, r rune, invalid byte), newlines bool) string {
	if isClean(s, newlines) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			bad(&b, r, s[i])
		case bidi(r):
		case control(r) || r == '\n' && !newlines:
			bad(&b, r, 0)
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// isClean is the fast path: most text has nothing to change.
func isClean(s string, newlines bool) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 || bidi(r) || control(r) || r == '\n' && !newlines {
			return false
		}
		i += size
	}
	return true
}

// mdSpecial are the characters that start Markdown or HTML syntax
// anywhere in a line. CommonMark lets any ASCII punctuation be escaped
// with a backslash.
const mdSpecial = "\\`*_[]()!<>|#~&{}"

// EscapeMarkdown makes s plain text in Markdown, safe in a paragraph or a
// table cell: SafeText first, then a backslash before every character
// that starts emphasis, code, links, images, HTML, entities, headings or
// table cells and before a list or block marker at the start. Line breaks
// become spaces, so one value can't end a table row.
//
// GitHub Flavored Markdown also turns bare URLs (https://x.example),
// www. names and e-mail addresses into links. The ':' of "://", the '.'
// after "www" and every '@' are escaped too, which keeps them as text
// there while CommonMark renders the same characters.
func EscapeMarkdown(s string) string {
	s = SafeText(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
	var b strings.Builder
	b.Grow(len(s) + len(s)/8)
	for i, r := range s {
		if r < utf8.RuneSelf && strings.ContainsRune(mdSpecial, r) || i == 0 && strings.ContainsRune("-+=>", r) || autolinkChar(s, i) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	out := b.String()
	// "1. text" at the start would become an ordered list.
	if n := len(out) - len(strings.TrimLeft(out, "0123456789")); n > 0 && n < len(out) && (out[n] == '.' || out[n] == ')') {
		out = out[:n] + `\` + out[n:]
	}
	return out
}

// autolinkChar reports whether s[i] is a character GFM needs to see
// unescaped to start an extended autolink: '@', the ':' of "://" or the
// '.' of "www.".
func autolinkChar(s string, i int) bool {
	switch s[i] {
	case '@':
		return true
	case ':':
		return strings.HasPrefix(s[i+1:], "//")
	case '.':
		return i >= 3 && strings.EqualFold(s[i-3:i], "www")
	}
	return false
}

// MarkdownCode returns s as inline code: SafeText, line breaks as spaces,
// fenced by more backticks than the longest run inside it, with spaces
// added when s starts or ends with a backtick so the fence still reads
// as one.
func MarkdownCode(s string) string {
	s = SafeText(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || s == "" {
		s = " " + s + " "
	}
	return fence + s + fence
}

// MarkdownLink returns a Markdown link to rawURL with text as its label,
// both escaped. Only absolute http and https URLs become links: anything
// else (javascript:, data:, relative paths) is returned as escaped text,
// so a page can't plant a link that runs code or points somewhere
// unexpected.
func MarkdownLink(text, rawURL string) string {
	label := EscapeMarkdown(text)
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		if text == "" {
			return EscapeMarkdown(rawURL)
		}
		return label
	}
	if label == "" {
		label = EscapeMarkdown(u.String())
	}
	dest := strings.NewReplacer("(", "%28", ")", "%29", "<", "%3C", ">", "%3E", " ", "%20").Replace(u.String())
	return "[" + label + "](" + dest + ")"
}
