// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package term

import (
	"strings"
	"testing"
)

// hostile has every kind of byte a hostile page could send: OSC 52
// (clipboard write), a CSI erase, BEL, a C1 CSI, DEL, a bidi override, a
// broken UTF-8 byte and a carriage return.
const hostile = "a\x1b]52;c;aGk=\x07b\x1b[2Kc\u009bd\x7fe\u202ef\xffg\rh"

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		hostile:                 `a\x1b]52;c;aGk=\x07b\x1b[2Kc\u009bd\x7fef\xffg\x0dh`,
		"tab\tand\nnewline":     "tab\tand\nnewline",
		"plain text, ünïcode ✓": "plain text, ünïcode ✓",
		"\u2066isolate\u2069":   "isolate",
		"":                      "",
	} {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeText(t *testing.T) {
	if got, want := SafeText(hostile), "a\uFFFD]52;c;aGk=\uFFFDb\uFFFD[2Kc\uFFFDd\uFFFDef\uFFFDg\uFFFDh"; got != want {
		t.Errorf("SafeText = %q, want %q", got, want)
	}
	if got := SafeText("one\ntwo\tthree"); got != "one\uFFFDtwo\tthree" {
		t.Errorf("SafeText newline = %q", got)
	}
	if got := SafeLines("one\ntwo\x1b"); got != "one\ntwo\uFFFD" {
		t.Errorf("SafeLines = %q", got)
	}
	if s := "already safe"; SafeText(s) != s {
		t.Error("clean text must come back unchanged")
	}
}

func TestEscapeMarkdown(t *testing.T) {
	for in, want := range map[string]string{
		`<img src=https://attacker.invalid/px.gif>`: `\<img src=https://attacker.invalid/px.gif\>`,
		`[click](javascript:alert(1))`:              `\[click\]\(javascript:alert\(1\)\)`,
		"a | b":                                     `a \| b`,
		"*bold* _em_ `code` ~del~":                  "\\*bold\\* \\_em\\_ \\`code\\` \\~del\\~",
		"# heading":                                 `\# heading`,
		"- item":                                    `\- item`,
		"> quote":                                   `\> quote`,
		"1. first":                                  `1\. first`,
		"2026":                                      "2026",
		"&lt;b&gt;":                                 `\&lt;b\&gt;`,
		"two\r\nlines\nhere":                        "two lines here",
		"back\\slash":                               `back\\slash`,
		"esc\x1b[2K":                                "esc\uFFFD\\[2K",
		"a-b+c":                                     "a-b+c",
	} {
		if got := EscapeMarkdown(in); got != want {
			t.Errorf("EscapeMarkdown(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkdownCode(t *testing.T) {
	for in, want := range map[string]string{
		"x := 1":          "`x := 1`",
		"a `b` c":         "``a `b` c``",
		"``double``":      "``` ``double`` ```",
		"":                "`  `",
		"line\nbreak\x07": "`line break\uFFFD`",
	} {
		if got := MarkdownCode(in); got != want {
			t.Errorf("MarkdownCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkdownLink(t *testing.T) {
	for _, c := range []struct{ text, url, want string }{
		{"Advisory", "https://wpscan.example/v/1", "[Advisory](https://wpscan.example/v/1)"},
		{"", "https://x.example/a", `[https://x.example/a](https://x.example/a)`},
		{"[x](y)", "HTTP://x.example/a(b) c", `[\[x\]\(y\)](http://x.example/a%28b%29%20c)`},
		{"click", "javascript:alert(1)", "click"},
		{"", "javascript:alert(1)", `javascript:alert\(1\)`},
		{"img", "data:text/html,<b>", "img"},
		{"rel", "/relative/path", "rel"},
		{"bad", "http://x.example/\x7f", "bad"},
	} {
		if got := MarkdownLink(c.text, c.url); got != c.want {
			t.Errorf("MarkdownLink(%q, %q) = %q, want %q", c.text, c.url, got, c.want)
		}
	}
}

// TestNoControlBytesSurvive is the contract every writer relies on.
func TestNoControlBytesSurvive(t *testing.T) {
	for name, f := range map[string]func(string) string{
		"Sanitize": Sanitize, "SafeText": SafeText, "SafeLines": SafeLines, "EscapeMarkdown": EscapeMarkdown,
		"MarkdownCode": MarkdownCode, "MarkdownLink": func(s string) string { return MarkdownLink(s, "https://x.example/"+s) },
	} {
		out := f(hostile)
		if strings.ContainsAny(out, "\x1b\x07\x7f\u009b\u202e\r") || strings.Contains(out, "\xff") {
			t.Errorf("%s(%q) = %q keeps a control byte", name, hostile, out)
		}
	}
}
