// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// doctorNow is the fixed clock of the doctor tests.
var doctorNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

// fakeDoctorEnv finds nothing, reaches nothing and runs no tool.
func fakeDoctorEnv(t *testing.T, vars map[string]string) doctorEnv {
	t.Helper()
	cfg, cache := t.TempDir(), t.TempDir()
	return doctorEnv{
		getenv:    func(k string) string { return vars[k] },
		now:       func() time.Time { return doctorNow },
		http:      &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("no network in tests") })},
		writable:  func(string) bool { return true },
		lookPath:  func(string) (string, error) { return "", exec.ErrNotFound },
		exists:    func(string) bool { return false },
		configDir: func() (string, error) { return cfg, nil },
		cacheDir:  func(tool string) (string, error) { return filepath.Join(cache, tool), nil },
		runTool: func(context.Context, time.Duration, string, ...string) ([]byte, int, error) {
			return nil, 2, nil // a tool without the JSON answers
		},
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// statuses returns "id=status" for each check, for compact comparisons.
func statuses(checks []check) string {
	var s []string
	for _, c := range checks {
		s = append(s, c.ID+"="+c.Status)
	}
	return strings.Join(s, " ")
}

func TestCheckPath(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("shell script stand-ins")
	}
	first, second := t.TempDir(), t.TempDir()
	self := script(t, first, "rhythm", "echo rhythm 0.1.0")
	tuner := script(t, first, "rhythm-tuner", "echo rhythm-tuner 0.1.0")
	hidden := script(t, second, "rhythm-tuner", "echo rhythm-tuner 0.0.9")
	echo := script(t, second, "rhythm-echo", "echo rhythm-echo 0.1.0")
	// A link to the copy that runs is the same copy, not a shadow.
	if err := os.Symlink(echo, filepath.Join(first, "rhythm-echo")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", first+string(os.PathListSeparator)+second)
	targets := []target{
		{tool: selfName, binary: selfName, real: self},
		{tool: "tuner", binary: "rhythm-tuner", path: tuner},
		{tool: "echo", binary: "rhythm-echo", path: filepath.Join(first, "rhythm-echo")},
	}
	got := checkPath(targets)
	if s := statuses(got); s != "path.rhythm=ok path.rhythm-tuner=warn path.rhythm-echo=ok" {
		t.Fatalf("%s\n%+v", s, got)
	}
	if want := "rhythm-tuner: " + tuner + " comes first on your PATH and hides " + hidden + "; remove the copy you don't use"; got[1].Message != want {
		t.Errorf("got %q\nwant %q", got[1].Message, want)
	}

	// rhythm off PATH, or another rhythm first.
	other := script(t, t.TempDir(), "rhythm", "echo rhythm 0.0.1")
	t.Setenv("PATH", filepath.Dir(other))
	got = checkPath(targets[:1])
	if got[0].Status != statusWarn || got[0].Message != other+" comes first on your PATH, not this rhythm ("+self+")" {
		t.Errorf("%+v", got)
	}
	t.Setenv("PATH", t.TempDir())
	got = checkPath(targets[:1])
	if got[0].Status != statusWarn || got[0].Message != "rhythm isn't on your PATH; add "+first+" to it" {
		t.Errorf("%+v", got)
	}
}

func TestCheckInstallDir(t *testing.T) {
	self := []target{{tool: selfName, binary: selfName, real: "/home/me/.local/bin/rhythm"}}
	env := fakeDoctorEnv(t, nil)
	if c := checkInstallDir(env, self); c[0].Status != statusOK || c[0].Message != "/home/me/.local/bin is writable" {
		t.Errorf("%+v", c)
	}
	env.writable = func(string) bool { return false }
	if c := checkInstallDir(env, self); c[0].Status != statusWarn || !strings.HasPrefix(c[0].Message, "can't write to /home/me/.local/bin; rhythm install and rhythm update need sudo") {
		t.Errorf("%+v", c)
	}
	self[0].managed = &manager{"Homebrew", "brew upgrade rhythm", "brew uninstall rhythm"}
	if c := checkInstallDir(env, self); c[0].Status != statusOK || c[0].Message != "rhythm is managed by Homebrew; update it with: brew upgrade rhythm" {
		t.Errorf("%+v", c)
	}
}

func TestCheckBuilds(t *testing.T) {
	for _, tc := range []struct {
		targets []target
		want    string
		msg     string
	}{
		{[]target{{binary: "rhythm", current: "0.5.0"}, {binary: "rhythm-tuner", current: "0.3.1"}}, "version=ok", "every binary is a release build"},
		{[]target{{binary: "rhythm", current: "dev"}}, "version.rhythm=warn", "rhythm is a development build (dev); rhythm update leaves it alone"},
		{[]target{{binary: "rhythm-echo", current: unknownVersion, probe: "no answer within 5s"}}, "version.rhythm-echo=warn", "rhythm-echo version unknown (no answer within 5s); rhythm update leaves it alone"},
	} {
		got := checkBuilds(tc.targets)
		if statuses(got) != tc.want || got[0].Message != tc.msg {
			t.Errorf("%v: %+v", tc.targets, got)
		}
	}
}

func TestCheckUpdateState(t *testing.T) {
	targets := []target{{binary: "rhythm", current: "0.5.0"}, {binary: "rhythm-tuner", current: "0.3.1"}}
	write := func(t *testing.T, env doctorEnv, binary, body string) string {
		dir, _ := env.configDir()
		p := filepath.Join(dir, "update-check", binary+".json")
		touch(t, p)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	t.Run("none", func(t *testing.T) {
		if c := checkUpdateState(fakeDoctorEnv(t, nil), targets); statuses(c) != "update-check=ok" || c[0].Message != "no update checks saved yet" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("off", func(t *testing.T) {
		c := checkUpdateState(fakeDoctorEnv(t, map[string]string{"CI": "true"}), targets)
		if statuses(c) != "update-check=ok" || c[0].Message != "update checks are off (CI is set)" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("fine", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		write(t, env, "rhythm", `{"checked":"2026-10-01T00:00:00Z","latest":"0.5.0"}`)
		if c := checkUpdateState(env, targets); statuses(c) != "update-check=ok" || c[0].Message != "1 saved update checks, none stale" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("problems", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		broken := write(t, env, "rhythm", `{not json`)
		future := write(t, env, "rhythm-tuner", `{"checked":"2027-01-01T00:00:00Z","latest":"0.3.1"}`)
		orphan := write(t, env, "rhythm-echo", `{"checked":"2026-10-20T00:00:00Z","latest":"0.1.0"}`)
		c := checkUpdateState(env, targets)
		if statuses(c) != "update-check.rhythm-echo=warn update-check.rhythm-tuner=warn update-check.rhythm=warn" {
			t.Fatalf("%+v", c)
		}
		for i, want := range []string{
			orphan + " is left from rhythm-echo, which isn't installed; delete it",
			future + " is dated in the future (2027-01-01), so no check runs until then; delete it",
			broken + " can't be read; delete it and the next check rewrites it",
		} {
			if c[i].Message != want {
				t.Errorf("got %q\nwant %q", c[i].Message, want)
			}
		}
	})
	t.Run("newer release", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		write(t, env, "rhythm-tuner", `{"checked":"2026-10-23T00:00:00Z","latest":"0.3.2"}`)
		c := checkUpdateState(env, targets)
		if statuses(c) != "update-check.rhythm-tuner=warn" || c[0].Message != "rhythm-tuner 0.3.2 is out (0.3.1 installed); run: rhythm update" {
			t.Errorf("%+v", c)
		}
	})
}

func TestCheckData(t *testing.T) {
	// soundcheck isn't on kit/cli.Main, so it is never asked to refresh.
	targets := []target{{tool: selfName, binary: "rhythm"}, {tool: "tuner", binary: "rhythm-tuner", path: "/bin/rhythm-tuner", kitCLI: true},
		{tool: "soundcheck", binary: "rhythm-soundcheck", path: "/bin/rhythm-soundcheck"}}
	for _, tc := range []struct {
		name string
		out  string
		exit int
		err  error
		want string // "" for no check
		msg  string
	}{
		{"fresh", `{"schema":"rhythm-data/v1","data":[{"name":"vulns","version":"2026-10-23"},{"name":"checks","version":"1.4.0"}]}`, 0, nil,
			"ok", "rhythm-tuner data: vulns 2026-10-23, checks 1.4.0"},
		{"old", `{"schema":"rhythm-data/v1","data":[{"name":"vulns","version":"20260901.2"}]}`, 0, nil,
			"warn", "rhythm-tuner data: vulns is 53 days old, even after a refresh"},
		{"empty", `{"schema":"rhythm-data/v1","data":[]}`, 0, nil, "warn", "rhythm-tuner reported no data"},
		{"failed", "", 3, nil, "fail", "rhythm-tuner data refresh failed (exit 3); run: rhythm-tuner update"},
		{"failed with report", `{"schema":"rhythm-data/v1","data":[]}`, 1, nil, "fail", "rhythm-tuner data refresh failed (exit 1); run: rhythm-tuner update"},
		{"hung with report", `{"schema":"rhythm-data/v1"`, 0, errors.New("no answer within 2m0s"), "", ""},
		{"no JSON support", "Updated the vulnerability database\n", 0, nil, "", ""},
		{"rejects --json", "", 2, nil, "", ""},
		{"other schema", `{"schema":"rhythm-tuner/v0"}`, 0, nil, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := fakeDoctorEnv(t, nil)
			var ran []string
			env.runTool = func(_ context.Context, _ time.Duration, bin string, args ...string) ([]byte, int, error) {
				ran = append(ran, bin+" "+strings.Join(args, " "))
				return []byte(tc.out), tc.exit, tc.err
			}
			got := checkData(context.Background(), env, targets)
			if strings.Join(ran, ";") != "/bin/rhythm-tuner update --json" {
				t.Errorf("ran %v", ran)
			}
			switch {
			case tc.want == "" && len(got) != 0:
				t.Errorf("want no check, got %+v", got)
			case tc.want != "" && (len(got) != 1 || got[0].Status != tc.want || got[0].Message != tc.msg || got[0].ID != "data.rhythm-tuner"):
				t.Errorf("got %+v\nwant %s %q", got, tc.want, tc.msg)
			}
		})
	}
}

func TestCheckChrome(t *testing.T) {
	sc := []target{{tool: "soundcheck", binary: "rhythm-soundcheck", path: "/bin/rhythm-soundcheck"}}
	t.Run("no soundcheck", func(t *testing.T) {
		if c := checkChrome(context.Background(), fakeDoctorEnv(t, nil), nil); c != nil {
			t.Errorf("%+v", c)
		}
	})
	t.Run("delegates", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		env.runTool = func(_ context.Context, _ time.Duration, bin string, args ...string) ([]byte, int, error) {
			if strings.Join(args, " ") != "doctor --json" {
				t.Errorf("ran %v", args)
			}
			return []byte(`{"schema":"rhythm-doctor/v1","checks":[{"id":"chrome","status":"ok","message":"Chrome 130 at /usr/bin/google-chrome"},{"id":"sandbox","status":"odd","message":"x"}]}`), 0, nil
		}
		c := checkChrome(context.Background(), env, sc)
		if statuses(c) != "soundcheck.chrome=ok soundcheck.sandbox=warn" || c[0].Message != "soundcheck: Chrome 130 at /usr/bin/google-chrome" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("on PATH", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		env.lookPath = func(name string) (string, error) {
			if name == "chromium" {
				return "/usr/local/bin/chromium", nil
			}
			return "", exec.ErrNotFound
		}
		if c := checkChrome(context.Background(), env, sc); statuses(c) != "chrome=ok" || c[0].Message != "Soundcheck can use /usr/local/bin/chromium" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("known path", func(t *testing.T) {
		env := fakeDoctorEnv(t, map[string]string{"ProgramFiles": `C:\Program Files`})
		want := chromePaths(env)[0]
		env.exists = func(p string) bool { return p == want }
		if c := checkChrome(context.Background(), env, sc); statuses(c) != "chrome=ok" || c[0].Message != "Soundcheck can use "+want {
			t.Errorf("%+v", c)
		}
	})
	t.Run("downloaded", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		d, _ := env.cacheDir("soundcheck")
		env.exists = func(p string) bool { return p == filepath.Join(d, "browser") }
		if c := checkChrome(context.Background(), env, sc); statuses(c) != "chrome=ok" || !strings.Contains(c[0].Message, "downloaded") {
			t.Errorf("%+v", c)
		}
	})
	t.Run("none", func(t *testing.T) {
		c := checkChrome(context.Background(), fakeDoctorEnv(t, nil), sc)
		if statuses(c) != "chrome=warn" || c[0].Message != "no Chrome or Chromium found; Soundcheck downloads Chromium on its first run, or pass it --browser /path/to/chrome" {
			t.Errorf("%+v", c)
		}
	})
}

func TestCheckTokenNeverShowsValues(t *testing.T) {
	for _, tc := range []struct {
		vars map[string]string
		want string
	}{
		{nil, "no GitHub token set; GitHub allows 60 release lookups an hour without one"},
		{map[string]string{"GITHUB_TOKEN": "ghp_secret1"}, "token in use: GITHUB_TOKEN (value not shown)"},
		{map[string]string{"RHYTHM_GITHUB_TOKEN": "secret2"}, "token in use: RHYTHM_GITHUB_TOKEN (value not shown)"},
		{map[string]string{"GITHUB_TOKEN": "ghp_secret1", "RHYTHM_GITHUB_TOKEN": "secret2"},
			"token in use: RHYTHM_GITHUB_TOKEN (value not shown); GITHUB_TOKEN is not used while RHYTHM_GITHUB_TOKEN is set"},
		{map[string]string{"GITHUB_TOKEN": "ghp_secret1", "RHYTHM_GITHUB_TOKEN": "secret2", "RHYTHM_GITHUB_API": "https://bot:secret3@mirror.example/api/"},
			"token in use: RHYTHM_GITHUB_TOKEN (value not shown); release API https://mirror.example/api (from RHYTHM_GITHUB_API)"},
		{map[string]string{"GITHUB_TOKEN": "ghp_secret1", "RHYTHM_GITHUB_API": "http://127.0.0.1:21002"},
			"no token for the mirror; GITHUB_TOKEN goes to GitHub only (set RHYTHM_GITHUB_TOKEN for a mirror); release API http://127.0.0.1:21002 (from RHYTHM_GITHUB_API)"},
		{map[string]string{"RHYTHM_GITHUB_API": "https://mirror.example/api?access_token=secret4#secret5"},
			"no token for the mirror; release API https://mirror.example/api (from RHYTHM_GITHUB_API)"},
		{map[string]string{"RHYTHM_GITHUB_API": "https://api.github.com/"},
			"no GitHub token set; GitHub allows 60 release lookups an hour without one; release API https://api.github.com (from RHYTHM_GITHUB_API)"},
	} {
		c := checkToken(fakeDoctorEnv(t, tc.vars))
		if c.Status != statusOK || c.Message != tc.want || strings.Contains(c.Message, "secret") {
			t.Errorf("%v: %+v", tc.vars, c)
		}
	}
}

func TestCheckAPI(t *testing.T) {
	serve := func(t *testing.T, h http.HandlerFunc) *httptest.Server {
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		return srv
	}
	t.Run("answers", func(t *testing.T) {
		srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rate_limit" || r.Header.Get("Authorization") != "Bearer mirror-token" {
				t.Errorf("%s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
			}
			w.Header().Set("X-RateLimit-Remaining", "57")
			w.Header().Set("X-RateLimit-Limit", "60")
		})
		// A mirror gets RHYTHM_GITHUB_TOKEN, never GITHUB_TOKEN.
		env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": srv.URL, "GITHUB_TOKEN": "github-token", "RHYTHM_GITHUB_TOKEN": "mirror-token"})
		env.http = srv.Client()
		c := checkAPI(context.Background(), env)
		if c.Status != statusOK || c.Message != hostOf(srv.URL)+" answers; 57 of 60 requests left this hour" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("mirror without rate_limit", func(t *testing.T) {
		srv := serve(t, http.NotFound)
		env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": srv.URL})
		env.http = srv.Client()
		if c := checkAPI(context.Background(), env); c.Status != statusOK || c.Message != hostOf(srv.URL)+" answers" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		env := fakeDoctorEnv(t, nil)
		env.http = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != defaultAPI+"/rate_limit" || r.Header.Get("Authorization") != "" {
				t.Errorf("%s auth=%q", r.URL, r.Header.Get("Authorization"))
			}
			h := http.Header{}
			h.Set("X-RateLimit-Remaining", "0")
			return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
		})}
		if c := checkAPI(context.Background(), env); c.Status != statusWarn || c.Message != "GitHub rate limit reached; set GITHUB_TOKEN" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("github token to github", func(t *testing.T) {
		env := fakeDoctorEnv(t, map[string]string{"GITHUB_TOKEN": "github-token"})
		env.http = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "Bearer github-token" {
				t.Errorf("auth=%q", r.Header.Get("Authorization"))
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
		})}
		if c := checkAPI(context.Background(), env); c.Status != statusOK || c.Message != "api.github.com answers" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("rhythm token wins on github, as in downloads", func(t *testing.T) {
		env := fakeDoctorEnv(t, map[string]string{"GITHUB_TOKEN": "github-token", "RHYTHM_GITHUB_TOKEN": "rhythm-token"})
		env.http = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "Bearer rhythm-token" {
				t.Errorf("auth=%q", r.Header.Get("Authorization"))
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
		})}
		if c := checkAPI(context.Background(), env); c.Status != statusOK || c.Message != "api.github.com answers" {
			t.Errorf("%+v", c)
		}
	})
	t.Run("server error", func(t *testing.T) {
		srv := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) })
		env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": srv.URL})
		env.http = srv.Client()
		if c := checkAPI(context.Background(), env); c.Status != statusWarn || !strings.Contains(c.Message, "answered HTTP 502") {
			t.Errorf("%+v", c)
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		api := closedPort(t)
		env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": api})
		env.http = &http.Client{}
		c := checkAPI(context.Background(), env)
		if c.Status != statusFail || c.Message != "can't reach "+hostOf(api)+"; check network, proxy or RHYTHM_GITHUB_API" {
			t.Errorf("%+v", c)
		}
	})
}

// useDoctorEnv makes cmdDoctor run with env.
func useDoctorEnv(t *testing.T, env doctorEnv) {
	t.Helper()
	old := newDoctorEnv
	newDoctorEnv = func() doctorEnv { return env }
	t.Cleanup(func() { newDoctorEnv = old })
}

func TestDoctorCommand(t *testing.T) {
	s := newSandbox(t, nil, nil)
	s.kitTool(t, "rhythm-tuner", "0.3.1", "")
	fakeSelf(t, s.stub(t, "rhythm", "0.5.0", ""), "0.5.0")
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": srv.URL})
	env.http = srv.Client()
	ranData := false
	env.runTool = func(context.Context, time.Duration, string, ...string) ([]byte, int, error) {
		ranData = true
		return []byte(`{"schema":"rhythm-data/v1","data":[{"name":"vulns","version":"2026-10-24"}]}`), 0, nil
	}
	useDoctorEnv(t, env)

	code, out, errs := runCmd("doctor")
	want := "ok    rhythm runs from " + filepath.Join(s.bin, "rhythm") + "\n" +
		"ok    rhythm-tuner runs from " + filepath.Join(s.bin, "rhythm-tuner") + "\n" +
		"ok    " + s.bin + " is writable\n" +
		"ok    every binary is a release build\n" +
		"ok    no update checks saved yet\n" +
		"ok    rhythm-tuner data: vulns 2026-10-24\n" +
		"ok    no token for the mirror; release API " + srv.URL + " (from RHYTHM_GITHUB_API)\n" +
		"ok    " + hostOf(srv.URL) + " answers\n" +
		"No problems found.\n"
	if code != 0 || out != want || errs != "" {
		t.Errorf("exit %d\n%s\nwant\n%s%s", code, out, want, errs)
	}

	// --offline leaves out the data refresh and the API.
	ranData = false
	code, out, _ = runCmd("doctor", "--offline")
	if code != 0 || ranData || strings.Contains(out, "answers") || strings.Contains(out, "data:") {
		t.Errorf("--offline: exit %d, ran data %v\n%s", code, ranData, out)
	}

	// Any failed check makes the exit status 1.
	env.runTool = func(context.Context, time.Duration, string, ...string) ([]byte, int, error) { return nil, 3, nil }
	env.writable = func(string) bool { return false }
	useDoctorEnv(t, env)
	code, out, _ = runCmd("doctor")
	if code != 1 || !strings.Contains(out, "fail  rhythm-tuner data refresh failed (exit 3)") || !strings.HasSuffix(out, "1 failure, 1 warning.\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestDoctorUsage(t *testing.T) {
	if code, out, _ := runCmd("doctor", "--help"); code != 0 || !strings.Contains(out, "Usage: rhythm doctor [flags]") || !strings.Contains(out, "--offline") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if code, _, errs := runCmd("doctor", "extra"); code != 2 || !strings.Contains(errs, "takes no names") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestRunTool(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("shell script stand-ins")
	}
	dir := t.TempDir()
	ok := script(t, dir, "ok", `echo "$*"; exit 4`)
	out, exit, err := runTool(context.Background(), time.Second, ok, "update", "--json")
	if err != nil || exit != 4 || string(out) != "update --json\n" {
		t.Errorf("%q %d %v", out, exit, err)
	}
	slow := script(t, dir, "slow", "sleep 5")
	if _, _, err := runTool(context.Background(), 100*time.Millisecond, slow); err == nil || !strings.Contains(err.Error(), "no answer within") {
		t.Errorf("err = %v", err)
	}
}

func TestDoctorJSON(t *testing.T) {
	s := newSandbox(t, map[string]string{"rhythm-tuner": "0.3.1"}, nil)
	fakeSelf(t, s.stub(t, "rhythm", "dev", ""), "dev")
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": srv.URL, "GITHUB_TOKEN": "ghp_never_shown"})
	env.http = srv.Client()
	useDoctorEnv(t, env)

	code, out, errs := runCmd("doctor", "--json")
	if code != 0 || errs != "" {
		t.Errorf("exit %d\n%s", code, errs)
	}
	golden(t, "doctor.golden.json", out, s.dir, "$SANDBOX", srv.URL, "$API", hostOf(srv.URL), "$HOST")
	var rep doctorReport
	requireKeys(t, decodeStrict(t, out, &rep, "checks"), "id", "status", "message")
	if rep.Schema != "rhythm-doctor/v1" || len(rep.Checks) == 0 || strings.Contains(out, "ghp_") {
		t.Errorf("%+v", rep)
	}

	// A failed check exits 1, with the JSON still whole.
	env.http = &http.Client{}
	env.getenv = func(k string) string {
		if k == "RHYTHM_GITHUB_API" {
			return closedPort(t)
		}
		return ""
	}
	useDoctorEnv(t, env)
	code, out, _ = runCmd("doctor", "--json")
	if err := json.Unmarshal([]byte(out), &rep); code != 1 || err != nil || rep.Checks[len(rep.Checks)-1].Status != statusFail {
		t.Errorf("exit %d, %v\n%s", code, err, out)
	}
}
