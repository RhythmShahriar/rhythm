// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/cache"
	"github.com/RhythmShahriar/rhythm/kit/config"
)

// fakeRhythmEnv makes the test binary act as rhythm for the default
// Install: it prints its arguments and exits with the code given.
const fakeRhythmEnv = "RHYTHM_TEST_FAKE_RHYTHM"

// fakeRhythm runs the test binary as a fake rhythm when fakeRhythmEnv
// is set; ok is false otherwise.
func fakeRhythm() (code int, ok bool) {
	v := os.Getenv(fakeRhythmEnv)
	if v == "" {
		return 0, false
	}
	fmt.Println("fake rhythm", strings.Join(os.Args[1:], " "))
	if v == "fail" {
		return 1, true
	}
	return 0, true
}

// pending starts a check of rhythm-tuner 0.2.0 against a server that
// names 0.3.0 and waits for it, on an isolated config dir.
func pending(t *testing.T) *Pending {
	t.Helper()
	isolate(t)
	srv, _ := counting(t)
	p := StartCheck(context.Background(), &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL})
	if p == nil {
		t.Fatal("StartCheck returned nil")
	}
	<-p.done
	if !p.newer {
		t.Fatalf("check found no newer release: %q", p.latest)
	}
	return p
}

// recorder is an Install that counts its calls and returns err.
type recorder struct {
	calls int
	err   error
}

func (r *recorder) install(_ context.Context, out io.Writer) error {
	r.calls++
	fmt.Fprintln(out, "installing")
	return r.err
}

// ask returns options for an interactive run that answers answer.
func ask(p *Pending, answer string, out *bytes.Buffer, r *recorder) PromptOptions {
	return PromptOptions{
		Check:       p,
		In:          strings.NewReader(answer),
		Out:         out,
		Wait:        5 * time.Second,
		Interactive: func() bool { return true },
		Install:     r.install,
	}
}

const (
	wantPrompt = "A new release of Tuner is out: 0.3.0 (you have 0.2.0). Update now? [y/N] "
	wantNotice = "A new release of rhythm-tuner is available: 0.2.0 → 0.3.0\n  run: rhythm update\n"
)

func TestPromptYesInstalls(t *testing.T) {
	for _, answer := range []string{"y\n", "Y\n", "yes\n", " YES \r\n", "y"} {
		t.Run(answer, func(t *testing.T) {
			p := pending(t)
			var out bytes.Buffer
			r := &recorder{}
			if got := PromptAtExit(context.Background(), ask(p, answer, &out, r)); got != OutcomeUpdated {
				t.Fatalf("outcome %d, want updated; out %q", got, out.String())
			}
			if r.calls != 1 || out.String() != wantPrompt+"installing\n" {
				t.Errorf("%d installs, out %q", r.calls, out.String())
			}
			if declined("rhythm-tuner") != "" {
				t.Error("a yes was remembered as a no")
			}
		})
	}
}

func TestPromptFailedInstallSaysSo(t *testing.T) {
	p := pending(t)
	var out bytes.Buffer
	r := &recorder{err: errors.New("signature check failed")}
	if got := PromptAtExit(context.Background(), ask(p, "y\n", &out, r)); got != OutcomeFailed {
		t.Fatalf("outcome %d, want failed", got)
	}
	want := wantPrompt + "installing\nTuner wasn't updated and 0.2.0 is still installed: signature check failed\n  try again with: rhythm update tuner\n"
	if out.String() != want {
		t.Errorf("out %q\nwant %q", out.String(), want)
	}
}

func TestPromptNoIsRememberedPerVersion(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", "no\n", "maybe\n"} {
		t.Run(answer, func(t *testing.T) {
			p := pending(t)
			var out bytes.Buffer
			r := &recorder{}
			if got := PromptAtExit(context.Background(), ask(p, answer, &out, r)); got != OutcomeDeclined {
				t.Fatalf("outcome %d, want declined", got)
			}
			if r.calls != 0 || out.String() != wantPrompt {
				t.Errorf("%d installs, out %q", r.calls, out.String())
			}
			if v := declined("rhythm-tuner"); v != "0.3.0" {
				t.Fatalf("declined = %q, want 0.3.0", v)
			}

			// The same version again: the notice, no question.
			out.Reset()
			if got := PromptAtExit(context.Background(), ask(p, "y\n", &out, r)); got != OutcomeNotice || r.calls != 0 {
				t.Errorf("second run: outcome %d, %d installs", got, r.calls)
			}
			if out.String() != wantNotice {
				t.Errorf("second run printed %q", out.String())
			}

			// A newer version is asked about again.
			p.latest = "0.4.0"
			out.Reset()
			if got := PromptAtExit(context.Background(), ask(p, "n\n", &out, r)); got != OutcomeDeclined {
				t.Errorf("newer version: outcome %d, want declined", got)
			}
			if v := declined("rhythm-tuner"); v != "0.4.0" {
				t.Errorf("declined = %q, want 0.4.0", v)
			}
		})
	}
}

func TestDeclinedSurvivesTheDailyCheck(t *testing.T) {
	p := pending(t)
	remember("rhythm-tuner", "0.3.0")
	path, err := checkStatePath("rhythm-tuner")
	if err != nil {
		t.Fatal(err)
	}
	var st checkState
	if err := cache.ReadJSON(path, &st); err != nil {
		t.Fatal(err)
	}
	st.Checked = time.Now().Add(-48 * time.Hour) // stale: Check asks again
	if err := cache.WriteJSON(path, st); err != nil {
		t.Fatal(err)
	}
	if _, newer := p.u.Check(context.Background()); !newer {
		t.Fatal("no newer release")
	}
	if err := cache.ReadJSON(path, &st); err != nil || st.Declined != "0.3.0" || time.Since(st.Checked) > time.Hour {
		t.Errorf("state after the check: %+v, %v", st, err)
	}
}

func TestPromptFallsBackToTheNotice(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, o *PromptOptions)
	}{
		{"not a terminal", func(_ *testing.T, o *PromptOptions) { o.Interactive = func() bool { return false } }},
		{"default terminal test on buffers", func(_ *testing.T, o *PromptOptions) { o.Interactive = nil; o.Stdout = io.Discard }},
		{"CI", func(t *testing.T, _ *PromptOptions) { t.Setenv("CI", "true") }},
		{"quiet", func(_ *testing.T, o *PromptOptions) { o.Quiet = true }},
		{"notify in config", func(t *testing.T, _ *PromptOptions) { must(t, SetPromptMode(PromptNotify)) }},
		{"notify from env", func(t *testing.T, _ *PromptOptions) { t.Setenv(PromptEnv, "notify") }},
		{"bad env value", func(t *testing.T, _ *PromptOptions) { t.Setenv(PromptEnv, "sometimes") }},
		{"rhythm not installed", func(t *testing.T, o *PromptOptions) {
			o.Install = nil
			saved := lookPath
			t.Cleanup(func() { lookPath = saved })
			lookPath = func(string) (string, error) { return "", errors.New("not found") }
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pending(t)
			var out bytes.Buffer
			r := &recorder{}
			o := ask(p, "y\n", &out, r)
			tc.setup(t, &o)
			if got := PromptAtExit(context.Background(), o); got != OutcomeNotice {
				t.Errorf("outcome %d, want the notice", got)
			}
			if r.calls != 0 || out.String() != wantNotice {
				t.Errorf("%d installs, out %q", r.calls, out.String())
			}
		})
	}
}

func TestPromptSaysNothing(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, o *PromptOptions)
	}{
		{"off in config", func(t *testing.T, _ *PromptOptions) { must(t, SetPromptMode(PromptOff)) }},
		{"off from env over ask in config", func(t *testing.T, _ *PromptOptions) {
			must(t, SetPromptMode(PromptAsk))
			t.Setenv(PromptEnv, "OFF")
		}},
		{"no check", func(_ *testing.T, o *PromptOptions) { o.Check = nil }},
		{"up to date", func(_ *testing.T, o *PromptOptions) { o.Check.newer = false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pending(t)
			var out bytes.Buffer
			r := &recorder{}
			o := ask(p, "y\n", &out, r)
			tc.setup(t, &o)
			if got := PromptAtExit(context.Background(), o); got != OutcomeNone || r.calls != 0 || out.Len() != 0 {
				t.Errorf("outcome %d, %d installs, out %q", got, r.calls, out.String())
			}
		})
	}
}

func TestPromptEndsQuietlyOnEOFOrCtrlC(t *testing.T) {
	t.Run("EOF", func(t *testing.T) {
		p := pending(t)
		var out bytes.Buffer
		r := &recorder{}
		if got := PromptAtExit(context.Background(), ask(p, "", &out, r)); got != OutcomeNone || r.calls != 0 {
			t.Errorf("outcome %d, %d installs", got, r.calls)
		}
		if out.String() != wantPrompt+"\n" || declined("rhythm-tuner") != "" {
			t.Errorf("out %q, declined %q", out.String(), declined("rhythm-tuner"))
		}
	})
	t.Run("Ctrl-C", func(t *testing.T) {
		p := pending(t)
		var out bytes.Buffer
		r := &recorder{}
		in, w := io.Pipe() // never written: the prompt waits until ctx ends
		t.Cleanup(func() { _ = w.Close() })
		o := ask(p, "", &out, r)
		o.In = in
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)
		if got := PromptAtExit(ctx, o); got != OutcomeNone || r.calls != 0 || declined("rhythm-tuner") != "" {
			t.Errorf("outcome %d, %d installs, declined %q", got, r.calls, declined("rhythm-tuner"))
		}
	})
}

func TestPromptWaitsOnlyBriefly(t *testing.T) {
	isolate(t)
	p := &Pending{u: &Updater{Binary: "rhythm-tuner", Current: "0.2.0"}, done: make(chan struct{})} // never finishes
	var out bytes.Buffer
	o := ask(p, "y\n", &out, &recorder{})
	o.Wait = 20 * time.Millisecond
	start := time.Now()
	if got := PromptAtExit(context.Background(), o); got != OutcomeNone || out.Len() != 0 {
		t.Errorf("outcome %d, out %q", got, out.String())
	}
	if time.Since(start) > 2*time.Second {
		t.Error("waited too long for the check")
	}
}

func TestStartCheckOffInCIAndOffline(t *testing.T) {
	for name, set := range map[string]func(t *testing.T, u *Updater){
		"CI":                     func(t *testing.T, _ *Updater) { t.Setenv("CI", "1") },
		"RHYTHM_NO_UPDATE_CHECK": func(t *testing.T, _ *Updater) { t.Setenv("RHYTHM_NO_UPDATE_CHECK", "1") },
		"Offline":                func(_ *testing.T, u *Updater) { u.Offline = true },
		"dev build":              func(_ *testing.T, u *Updater) { u.Current = "dev" },
	} {
		t.Run(name, func(t *testing.T) {
			isolate(t)
			srv, hits := counting(t)
			u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL}
			set(t, u)
			if p := StartCheck(context.Background(), u); p != nil {
				t.Error("StartCheck started a check")
			}
			if hits.Load() != 0 {
				t.Errorf("%d requests", hits.Load())
			}
		})
	}
	if StartCheck(context.Background(), nil) != nil {
		t.Error("StartCheck(nil) isn't nil")
	}
}

func TestPromptDefaultInstallRunsRhythmUpdate(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	saved := lookPath
	t.Cleanup(func() { lookPath = saved })
	lookPath = func(name string) (string, error) {
		if name != "rhythm" {
			return "", errors.New("unexpected lookup " + name)
		}
		return self, nil
	}
	for _, tc := range []struct {
		fake string
		want Outcome
	}{{"ok", OutcomeUpdated}, {"fail", OutcomeFailed}} {
		t.Run(tc.fake, func(t *testing.T) {
			p := pending(t)
			t.Setenv(fakeRhythmEnv, tc.fake)
			var out bytes.Buffer
			o := ask(p, "y\n", &out, nil)
			o.Install = nil
			if got := PromptAtExit(context.Background(), o); got != tc.want {
				t.Fatalf("outcome %d, want %d; out %q", got, tc.want, out.String())
			}
			if !strings.Contains(out.String(), "fake rhythm update tuner\n") {
				t.Errorf("rhythm wasn't run as rhythm update tuner: %q", out.String())
			}
			if tc.want == OutcomeFailed && !strings.Contains(out.String(), "rhythm update tuner exited 1") {
				t.Errorf("failure not reported: %q", out.String())
			}
		})
	}
}

func TestPromptModeSetting(t *testing.T) {
	isolate(t)
	if m, src, err := CurrentPromptMode(); m != PromptAsk || src != "default" || err != nil {
		t.Errorf("default = %s, %s, %v", m, src, err)
	}
	for _, m := range []PromptMode{PromptNotify, PromptOff, PromptAsk} {
		must(t, SetPromptMode(m))
		if got, src, err := CurrentPromptMode(); got != m || src != "config" || err != nil {
			t.Errorf("after setting %s: %s, %s, %v", m, got, src, err)
		}
	}
	t.Setenv(PromptEnv, "Notify")
	if m, src, err := CurrentPromptMode(); m != PromptNotify || src != PromptEnv || err != nil {
		t.Errorf("env = %s, %s, %v", m, src, err)
	}
	t.Setenv(PromptEnv, "loud")
	if m, _, err := CurrentPromptMode(); m != PromptNotify || err == nil || !strings.Contains(err.Error(), PromptEnv) {
		t.Errorf("bad env = %s, %v", m, err)
	}
	if err := SetPromptMode("loud"); err == nil {
		t.Error("SetPromptMode took a bad mode")
	}

	// A settings file that can't be parsed is an error, not the default.
	t.Setenv(PromptEnv, "")
	dir, _ := config.Dir()
	must(t, os.WriteFile(filepath.Join(dir, config.SettingsFile), []byte("{"), 0o600))
	if m, _, err := CurrentPromptMode(); m != PromptNotify || err == nil {
		t.Errorf("broken settings = %s, %v", m, err)
	}
}

func TestDisplayName(t *testing.T) {
	for in, want := range map[string]string{"rhythm-tuner": "Tuner", "rhythm-soundcheck": "Soundcheck", "rhythm": "rhythm", "rhythm-": "rhythm-", "other": "other"} {
		if got := displayName(in); got != want {
			t.Errorf("displayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
