// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func scanFlags(stderr io.Writer) *flag.FlagSet {
	fs := NewFlagSet("scan", "Usage: rhythm-fake scan <what> [flags]", stderr)
	fs.Bool("offline", false, "stay off the network")
	return fs
}

// fakeTool is a tool whose scan command does what its first argument
// says, so one Tool covers every way a command can end.
func fakeTool() Tool {
	return Tool{
		Name: "rhythm-fake", Version: "1.2.3", Commit: "abc1234", Summary: "a tool for tests",
		Commands: []Command{
			{Name: "scan", Args: "<what>", Summary: "end the way <what> says", Flags: func() *flag.FlagSet { return scanFlags(io.Discard) }, Run: func(ctx context.Context, args []string, stdout, stderr io.Writer) error {
				pos, err := ParseArgs(scanFlags(stderr), args)
				if err != nil {
					return err
				}
				what := ""
				if len(pos) > 0 {
					what = pos[0]
				}
				switch what {
				case "ok":
					fmt.Fprintln(stdout, "scanned")
					return nil
				case "findings":
					return ErrFindings
				case "usage":
					return Usagef("scan: %q isn't a site", what)
				case "fail":
					return errors.New("site unreachable")
				case "exit42":
					return &ExitError{Code: 42, Err: errors.New("custom")}
				}
				return Usagef("scan: want a <what>")
			}},
			{Name: "hidden", Hidden: true, Run: func(context.Context, []string, io.Writer, io.Writer) error { return nil }},
		},
		Update: func(_ context.Context, w io.Writer) ([]DataPack, error) {
			fmt.Fprintln(w, "downloading...")
			return []DataPack{
				{Name: "wpvulnerability", Version: "2026-10-24", SHA256: strings.Repeat("a", 64)},
				{Name: "signatures", Version: "7", SHA256: strings.Repeat("b", 64)},
			}, nil
		},
	}
}

// runTool drives t through Main's body with fresh buffers.
func runTool(t *testing.T, tool Tool, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Setenv(InvokedAsEnv, "rhythm fake")
	t.Setenv(OfflineEnv, "1") // no release check unless a test turns it on
	var out, errb bytes.Buffer
	code = tool.main(args, &out, &errb)
	return code, out.String(), errb.String()
}

const fakeUsage = `♪ rhythm-fake  1.2.3
a tool for tests

Usage:
  rhythm fake <command> [args]

Commands:
  scan <what>        end the way <what> says
  update [--json]    download the latest data
  version [--json]   print the version

Run 'rhythm fake help <command>' for a command's flags.

Exit codes:
  0     success
  1     findings at or above --fail-on
  2     usage error
  3     the run failed, or a site, tool or service couldn't be reached
  130   interrupted
`

func TestMainDispatch(t *testing.T) {
	scanHelp := "Usage: rhythm-fake scan <what> [flags]\n\nFlags:\n  --offline   stay off the network\n"
	cases := []struct {
		args           []string
		code           int
		stdout, stderr string
	}{
		{nil, 0, fakeUsage, ""},
		{[]string{"--help"}, 0, fakeUsage, ""},
		{[]string{"-h"}, 0, fakeUsage, ""},
		{[]string{"help"}, 0, fakeUsage, ""},
		{[]string{"help", "scan"}, 0, "", scanHelp},
		{[]string{"scan", "--help"}, 0, "", scanHelp},
		{[]string{"help", "version"}, 0, "Usage: rhythm fake version [--json]\n\nPrints the version.\n\nFlags:\n  --json   print the rhythm-version/v1 JSON document\n", ""},
		{[]string{"help", "scn"}, 2, "", "rhythm fake: unknown command \"scn\"\n  did you mean: rhythm fake help scan\n"},
		{[]string{"version"}, 0, "rhythm-fake 1.2.3 (commit abc1234)\n", ""},
		{[]string{"--version"}, 0, "rhythm-fake 1.2.3 (commit abc1234)\n", ""},
		{[]string{"version", "extra"}, 2, "", "unexpected argument \"extra\"\nUsage: rhythm fake version [--json]\n\nPrints the version.\n\nFlags:\n  --json   print the rhythm-version/v1 JSON document\n"},
		{[]string{"scan", "ok"}, 0, "scanned\n", ""},
		{[]string{"scan", "findings"}, 1, "", ""},
		{[]string{"scan", "exit42"}, 42, "", "rhythm fake: custom\n"},
		{[]string{"scan", "fail"}, 3, "", "rhythm fake: site unreachable\n"},
		{[]string{"scan", "usage"}, 2, "", "rhythm fake: scan: \"usage\" isn't a site\n  run 'rhythm fake help scan' for usage\n"},
		{[]string{"scan", "--nope"}, 2, "", "flag provided but not defined: -nope\n" + scanHelp},
		{[]string{"hidden"}, 0, "", ""},
		{[]string{"scna", "ok"}, 2, "", "rhythm fake: unknown command \"scna\"\n  did you mean: rhythm fake scan\n  run 'rhythm fake help' to see the commands\n"},
		{[]string{"update", "--json", "extra"}, 2, "", "unexpected argument \"extra\"\nUsage: rhythm fake update [--json]\n\nDownloads the latest data the tool works from.\n\nFlags:\n  --json   print the rhythm-data/v1 JSON document\n"},
		{[]string{"update"}, 0, "downloading...\nwpvulnerability 2026-10-24\nsignatures 7\n", ""},
	}
	for _, c := range cases {
		code, stdout, stderr := runTool(t, fakeTool(), c.args...)
		if code != c.code || stdout != c.stdout || stderr != c.stderr {
			t.Errorf("%q: exit %d (want %d)\nstdout:\n%s\nwant:\n%s\nstderr:\n%s\nwant:\n%s",
				c.args, code, c.code, stdout, c.stdout, stderr, c.stderr)
		}
	}
}

func TestMainVersionJSON(t *testing.T) {
	for _, tool := range []Tool{fakeTool(), {Name: "rhythm-bare"}} {
		code, stdout, stderr := runTool(t, tool, "version", "--json")
		if code != 0 || stderr != "" {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		// Every field is required and nothing else may appear.
		var doc struct {
			Schema     *string `json:"schema"`
			Name       *string `json:"name"`
			Version    *string `json:"version"`
			Commit     *string `json:"commit"`
			KitVersion *string `json:"kit_version"`
		}
		dec := json.NewDecoder(strings.NewReader(stdout))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&doc); err != nil {
			t.Fatalf("%s: %v", stdout, err)
		}
		if doc.Schema == nil || doc.Name == nil || doc.Version == nil || doc.Commit == nil || doc.KitVersion == nil {
			t.Fatalf("missing fields: %s", stdout)
		}
		wantVersion, wantCommit := tool.Version, tool.Commit
		if wantVersion == "" {
			wantVersion = "dev"
		}
		if *doc.Schema != VersionSchema || *doc.Name != tool.Name || *doc.Version != wantVersion ||
			*doc.Commit != wantCommit || *doc.KitVersion != "(devel)" {
			t.Errorf("version --json = %s", stdout)
		}
	}
	if _, stdout, _ := runTool(t, Tool{Name: "rhythm-bare"}, "version"); stdout != "rhythm-bare dev\n" {
		t.Errorf("bare version: %q", stdout)
	}
}

func TestMainUpdateJSON(t *testing.T) {
	code, stdout, stderr := runTool(t, fakeTool(), "update", "--json")
	want := `{"schema":"rhythm-data/v1","data":[` +
		`{"name":"wpvulnerability","version":"2026-10-24","sha256":"` + strings.Repeat("a", 64) + `"},` +
		`{"name":"signatures","version":"7","sha256":"` + strings.Repeat("b", 64) + `"}]}` + "\n"
	if code != 0 || stdout != want || stderr != "downloading...\n" {
		t.Errorf("exit %d\nstdout: %s\nwant:   %s\nstderr: %q", code, stdout, want, stderr)
	}

	tool := fakeTool()
	tool.Update = func(context.Context, io.Writer) ([]DataPack, error) { return nil, nil }
	if _, stdout, _ := runTool(t, tool, "update", "--json"); stdout != `{"schema":"rhythm-data/v1","data":[]}`+"\n" {
		t.Errorf("no packs: %s", stdout)
	}

	tool.Update = func(context.Context, io.Writer) ([]DataPack, error) { return nil, errors.New("HTTP 503") }
	for _, args := range [][]string{{"update"}, {"update", "--json"}} {
		code, stdout, stderr := runTool(t, tool, args...)
		if code != 3 || stdout != "" || stderr != "rhythm fake: update failed: HTTP 503\n" {
			t.Errorf("%q: exit %d %q %q", args, code, stdout, stderr)
		}
	}
}

// pinningTool is fakeTool with UpdatePinned, which reports the pinned
// versions it was given except "7x", which it can't get.
func pinningTool(got *map[string]string) Tool {
	tool := fakeTool()
	tool.UpdatePinned = func(_ context.Context, _ io.Writer, pins map[string]string) ([]DataPack, error) {
		*got = pins
		packs, _ := tool.Update(context.Background(), io.Discard)
		for i, p := range packs {
			if v, ok := pins[p.Name]; ok && v != "7x" {
				packs[i].Version = v
			}
		}
		return packs, nil
	}
	return tool
}

func TestMainUpdatePin(t *testing.T) {
	var got map[string]string
	tool := pinningTool(&got)
	code, stdout, stderr := runTool(t, tool, "update", "--json", "--pin", "wpvulnerability=2026-10-20, signatures=6")
	want := `{"schema":"rhythm-data/v1","data":[` +
		`{"name":"wpvulnerability","version":"2026-10-20","sha256":"` + strings.Repeat("a", 64) + `","pinned":true},` +
		`{"name":"signatures","version":"6","sha256":"` + strings.Repeat("b", 64) + `","pinned":true}]}` + "\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("exit %d\nstdout: %s\nwant:   %s\nstderr: %q", code, stdout, want, stderr)
	}
	if !maps.Equal(got, map[string]string{"wpvulnerability": "2026-10-20", "signatures": "6"}) {
		t.Errorf("pins passed: %v", got)
	}

	// Only the named pack is marked; plain update still runs Update.
	got = nil
	if _, stdout, _ := runTool(t, tool, "update", "--pin", "signatures=6"); stdout != "wpvulnerability 2026-10-24\nsignatures 6\n" {
		t.Errorf("one pin: %q", stdout)
	}
	got = nil
	if _, _, _ = runTool(t, tool, "update"); got != nil {
		t.Errorf("update without --pin called UpdatePinned with %v", got)
	}

	for spec, msg := range map[string]string{
		"signatures=7x":   "rhythm fake: update failed: signatures 7x isn't available (got 7)\n",
		"browser=131":     "rhythm fake: update failed: no browser data to pin at 131\n",
		"signatures":      "rhythm fake: --pin: \"signatures\" isn't name=version\n",
		"=6":              "rhythm fake: --pin: \"=6\" isn't name=version\n",
		"a=1,a=2":         "rhythm fake: --pin: a is pinned twice\n",
		"signatures=6,,b": "rhythm fake: --pin: \"\" isn't name=version\n",
	} {
		want := 3
		if strings.Contains(msg, "--pin:") {
			want = 2
		}
		code, stdout, stderr := runTool(t, tool, "update", "--json", "--pin", spec)
		if code != want || stdout != "" || stderr != msg {
			t.Errorf("--pin %q: exit %d %q %q", spec, code, stdout, stderr)
		}
	}

	_, help, _ := runTool(t, tool, "update", "--help")
	for _, want := range []string{"update [--json] [--pin name=version,...]", "--pin string   download these data versions, as name=version,..."} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
}

func TestMainUpdateWithoutPinning(t *testing.T) {
	code, stdout, stderr := runTool(t, fakeTool(), "update", "--json", "--pin", "signatures=6")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "flag provided but not defined: -pin") {
		t.Errorf("exit %d %q %q", code, stdout, stderr)
	}
	if _, help, _ := runTool(t, fakeTool(), "update", "--help"); strings.Contains(help, "pin") {
		t.Errorf("help offers --pin:\n%s", help)
	}
}

func TestMainUpdateIsACommandWithoutUpdate(t *testing.T) {
	ran := false
	tool := Tool{Name: "rhythm-fake", Commands: []Command{{Name: "update", Run: func(context.Context, []string, io.Writer, io.Writer) error {
		ran = true
		return nil
	}}}}
	if code, _, _ := runTool(t, tool, "update"); code != 0 || !ran {
		t.Errorf("exit %d, ran %v", code, ran)
	}
}

func TestMainDefaultCommand(t *testing.T) {
	tool := fakeTool()
	tool.Default = "scan"
	if code, stdout, _ := runTool(t, tool, "ok"); code != 0 || stdout != "scanned\n" {
		t.Errorf("exit %d: %q", code, stdout)
	}
}

func TestMainInterrupted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a process can't send itself Ctrl-C on Windows")
	}
	for _, ret := range []error{nil, context.Canceled, ErrFindings} {
		tool := Tool{Name: "rhythm-fake", Commands: []Command{{Name: "wait", Run: func(ctx context.Context, _ []string, _, _ io.Writer) error {
			p, _ := os.FindProcess(os.Getpid())
			if err := p.Signal(os.Interrupt); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
			case <-time.After(10 * time.Second):
				return errors.New("ctx wasn't canceled")
			}
			return ret
		}}}}
		code, _, stderr := runTool(t, tool, "wait")
		if code != 130 || stderr != "rhythm fake: interrupted\n" {
			t.Errorf("returning %v: exit %d: %q", ret, code, stderr)
		}
	}
}

func TestMainFirstRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	tool := fakeTool()
	tool.FirstRun = func(w io.Writer) { fmt.Fprintln(w, "Note: shown once.") }
	for i, want := range []string{"Note: shown once.\n", ""} {
		if code, _, stderr := runTool(t, tool, "scan", "ok"); code != 0 || stderr != want {
			t.Errorf("run %d: exit %d: %q", i+1, code, stderr)
		}
	}
	// version and help never count as the first run.
	tool.FirstRunKey = "fake-other"
	runTool(t, tool, "version")
	if _, _, stderr := runTool(t, tool, "scan", "ok"); stderr != "Note: shown once.\n" {
		t.Errorf("version used up the first run: %q", stderr)
	}
}

// fixtureServer starts srv on the first free 127.0.0.1 port in
// 20080-20099, the range these tests may use.
func fixtureServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	for port := 20080; port <= 20099; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		srv := httptest.NewUnstartedServer(h)
		srv.Listener = l
		srv.Start()
		t.Cleanup(srv.Close)
		return srv
	}
	t.Skip("no free port in 20080-20099")
	return nil
}

func TestMainReleaseNotice(t *testing.T) {
	var hits atomic.Int32
	srv := fixtureServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/repos/RhythmShahriar/fake/releases/latest" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"tag_name": "v9.0.0", "assets": []}`)
	}))
	saved := noticeWait
	t.Cleanup(func() { noticeWait = saved })
	noticeWait = 5 * time.Second

	// waitForHit is a command that gives the release check time to reach
	// the server, or not, before it returns.
	waitForHit := func(ctx context.Context, _ []string, _, _ io.Writer) error {
		deadline := time.Now().Add(500 * time.Millisecond)
		for hits.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	}
	setup := func(t *testing.T) Tool {
		t.Setenv("RHYTHM_GITHUB_API", srv.URL)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no cached answer
		t.Setenv("HOME", t.TempDir())
		t.Setenv("CI", "")
		t.Setenv("RHYTHM_NO_UPDATE_CHECK", "")
		hits.Store(0)
		return Tool{Name: "rhythm-fake", Version: "1.0.0", Commands: []Command{{Name: "work", Run: waitForHit}}}
	}

	t.Run("online", func(t *testing.T) {
		tool := setup(t)
		var out, errb bytes.Buffer
		t.Setenv(OfflineEnv, "")
		code := tool.main([]string{"work"}, &out, &errb)
		want := "A new release of rhythm-fake is available: 1.0.0 → 9.0.0\n  run: rhythm update\n"
		if code != 0 || hits.Load() != 1 || errb.String() != want {
			t.Errorf("exit %d, %d requests, stderr %q", code, hits.Load(), errb.String())
		}
	})
	t.Run("RHYTHM_OFFLINE", func(t *testing.T) {
		tool := setup(t)
		var out, errb bytes.Buffer
		t.Setenv(OfflineEnv, "1")
		if code := tool.main([]string{"work"}, &out, &errb); code != 0 || hits.Load() != 0 || errb.Len() != 0 {
			t.Errorf("exit %d, %d requests, stderr %q", code, hits.Load(), errb.String())
		}
	})
	t.Run("Offline", func(t *testing.T) {
		tool := setup(t)
		tool.Offline = func() bool { return HasFlag([]string{"work", "--offline"}, "offline") }
		var out, errb bytes.Buffer
		t.Setenv(OfflineEnv, "")
		if code := tool.main([]string{"work", "--offline"}, &out, &errb); code != 0 || hits.Load() != 0 || errb.Len() != 0 {
			t.Errorf("exit %d, %d requests, stderr %q", code, hits.Load(), errb.String())
		}
	})
	t.Run("not for version", func(t *testing.T) {
		tool := setup(t)
		var out, errb bytes.Buffer
		t.Setenv(OfflineEnv, "")
		tool.main([]string{"version"}, &out, &errb)
		time.Sleep(100 * time.Millisecond)
		if hits.Load() != 0 {
			t.Errorf("version checked for a release")
		}
	})
}

func TestToolCheck(t *testing.T) {
	run := func(context.Context, []string, io.Writer, io.Writer) error { return nil }
	for name, tool := range map[string]Tool{
		"reserved help":    {Name: "x", Commands: []Command{{Name: "help", Run: run}}},
		"reserved version": {Name: "x", Commands: []Command{{Name: "version", Run: run}}},
		"reserved update":  {Name: "x", Commands: []Command{{Name: "update", Run: run}}, Update: fakeTool().Update},
		"repeated":         {Name: "x", Commands: []Command{{Name: "a", Run: run}, {Name: "a", Run: run}}},
		"no Run":           {Name: "x", Commands: []Command{{Name: "a"}}},
		"no name":          {Name: "x", Commands: []Command{{Run: run}}},
		"missing default":  {Name: "x", Default: "scan"},
		"pinned no update": {Name: "x", UpdatePinned: func(context.Context, io.Writer, map[string]string) ([]DataPack, error) { return nil, nil }},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			tool.check()
		}()
	}
}

func TestHasFlag(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"scan", "--offline"}, true},
		{[]string{"-offline", "scan"}, true},
		{[]string{"--offline=true"}, true},
		{[]string{"--offline=false"}, false},
		{[]string{"--offline", "--offline=0"}, false},
		{[]string{"--offline=nope"}, false},
		{[]string{"--", "--offline"}, false},
		{[]string{"offline"}, false},
		{[]string{"--offline-mode"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := HasFlag(c.args, "offline"); got != c.want {
			t.Errorf("HasFlag(%q) = %v", c.args, got)
		}
	}
}

func TestExitError(t *testing.T) {
	if got := ErrFindings.Error(); got != "exit status 1" {
		t.Errorf("ErrFindings: %q", got)
	}
	base := errors.New("base")
	ee := &ExitError{Code: 4, Err: base}
	if ee.Error() != "base" || !errors.Is(ee, base) {
		t.Errorf("ExitError doesn't wrap: %v", ee)
	}
}

func TestQuietRun(t *testing.T) {
	for args, want := range map[string]bool{
		"scan x":             false,
		"scan x --json":      true,
		"scan -q x":          true,
		"scan --quiet x":     true,
		"scan --json=false":  false,
		"scan -- --json":     false,
		"scan --format json": false,
	} {
		if got := QuietRun(strings.Fields(args)); got != want {
			t.Errorf("QuietRun(%q) = %v, want %v", args, got, want)
		}
	}
}
