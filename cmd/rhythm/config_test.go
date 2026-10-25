// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigUpdatePrompt(t *testing.T) {
	dir := freshConfig(t)
	steps := []struct {
		args      []string
		code      int
		out, errs string
	}{
		{[]string{"config"}, 0, "update-prompt  ask  (default)\n", ""},
		{[]string{"config", "update-prompt"}, 0, "ask  (default)\n", ""},
		{[]string{"config", "update-prompt", "notify"}, 0, "update-prompt set to notify\n", ""},
		{[]string{"config", "update-prompt"}, 0, "notify\n", ""},
		{[]string{"config", "update-prompt", "OFF"}, 0, "update-prompt set to off\n", ""},
		{[]string{"config"}, 0, "update-prompt  off\n", ""},
		{[]string{"config", "update-prompt", "loud"}, 2, "", "rhythm: \"loud\" isn't an update-prompt mode; use ask, notify or off\n"},
		{[]string{"config", "update-promt"}, 2, "", "rhythm: unknown setting \"update-promt\"\n  did you mean: rhythm config update-prompt\n  known: update-prompt\n"},
		{[]string{"config", "update-prompt", "ask", "extra"}, 2, "", "usage: rhythm config [update-prompt [ask|notify|off]]\n"},
	}
	for _, s := range steps {
		var out, errb bytes.Buffer
		code := run(s.args, &out, &errb)
		if code != s.code || out.String() != s.out || errb.String() != s.errs {
			t.Errorf("rhythm %s: exit %d, stdout %q, stderr %q", strings.Join(s.args, " "), code, out.String(), errb.String())
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "config", "rhythm", "settings.json"))
	if err != nil || string(data) != `{"update-prompt":"off"}` {
		t.Errorf("settings.json = %s, %v", data, err)
	}

	t.Setenv("RHYTHM_UPDATE_PROMPT", "ask")
	var out, errb bytes.Buffer
	if code := run([]string{"config", "update-prompt"}, &out, &errb); code != 0 || out.String() != "ask  (from RHYTHM_UPDATE_PROMPT)\n" {
		t.Errorf("env override: exit %d, %q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"config", "update-prompt", "notify"}, &out, &errb); code != 0 ||
		errb.String() != "Note: RHYTHM_UPDATE_PROMPT=ask is set and wins over this setting.\n" {
		t.Errorf("set under env: exit %d, stderr %q", code, errb.String())
	}
	t.Setenv("RHYTHM_UPDATE_PROMPT", "sometimes")
	errb.Reset()
	if code := run([]string{"config"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "RHYTHM_UPDATE_PROMPT") {
		t.Errorf("bad env: exit %d, stderr %q", code, errb.String())
	}
}

func TestConfigHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"config", "--help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "Usage: rhythm config") {
		t.Errorf("exit %d, %q", code, out.String())
	}
	if problems := layoutProblems(out.String()); len(problems) > 0 {
		t.Errorf("layout: %v", problems)
	}
}

// selfCheck never starts a check for test buffers, for the commands in
// noSelfPrompt, hidden commands or --offline, so no test reaches the
// network for rhythm's own release.
func TestSelfCheckOnlyForRealTerminals(t *testing.T) {
	var buf bytes.Buffer
	saved := version
	t.Cleanup(func() { version = saved })
	version = "0.7.0"
	for _, tc := range []struct {
		c    command
		args []string
	}{
		{commands["list"], nil},
		{commands["update"], nil},
		{commands["list"], []string{"--offline"}},
		{command{name: "__complete", hidden: true}, nil},
	} {
		if p := selfCheck(tc.c, tc.args, &buf, &buf); p != nil {
			t.Errorf("%s %v: check started", tc.c.name, tc.args)
		}
		// On the process's own streams only the commands that never
		// prompt are tried, so a test run in a terminal sends nothing.
		if noSelfPrompt[tc.c.name] || tc.c.hidden || len(tc.args) > 0 {
			if p := selfCheck(tc.c, tc.args, os.Stdout, os.Stderr); p != nil {
				t.Errorf("%s %v: check started on the real streams", tc.c.name, tc.args)
			}
		}
	}
	for _, name := range []string{"update", "uninstall", "version", "completion", "config"} {
		if !noSelfPrompt[name] {
			t.Errorf("%s may offer a self update", name)
		}
		if _, ok := commands[name]; !ok {
			t.Errorf("noSelfPrompt names %q, which isn't a command", name)
		}
	}
}
