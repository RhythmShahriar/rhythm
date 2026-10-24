// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUnknownTool(t *testing.T) {
	var out, errb bytes.Buffer
	t.Setenv("PATH", t.TempDir())
	if code := run([]string{"tuner"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "rhythm install tuner") {
		t.Errorf("exit %d: %s", code, errb.String())
	}
}

func TestSoundcheckRegistered(t *testing.T) {
	sc, ok := lookupTool("sc")
	if !ok || sc.Name != "soundcheck" || sc.Repo != "RhythmShahriar/soundcheck" {
		t.Errorf("soundcheck isn't registered: %+v", sc)
	}
	var out bytes.Buffer
	usage(&out)
	if !strings.Contains(out.String(), "soundcheck (sc)") {
		t.Error("usage doesn't list soundcheck")
	}
}
