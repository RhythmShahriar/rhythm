// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/config"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

func init() {
	addRouterCommand(routerCommand{name: "doctor", args: "[--offline] [--json]", summary: "check the install: PATH, versions, data, Chrome and the release API", run: cmdDoctor})
}

const doctorSynopsis = `Usage: rhythm doctor [flags]

Checks that rhythm and its tools are set up well: which copies run first
on PATH, whether the install folder is writable, development builds, the
saved update checks, each tool's data, Chrome for Soundcheck, the GitHub
token in use (never its value) and whether the release API answers.
Each check says ok, warn or fail.

Checking the data runs each tool's own data refresh. --offline skips it
and the release API.

Exit status: 0 when no check failed, 1 when any failed, 2 for a usage
error and 130 when interrupted.`

// Doctor check statuses.
const (
	statusOK   = "ok"
	statusWarn = "warn"
	statusFail = "fail"
)

// check is one finding of rhythm doctor.
type check struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// doctorReport is rhythm doctor --json's answer.
type doctorReport struct {
	Schema string  `json:"schema"` // rhythm-doctor/v1
	Checks []check `json:"checks"`
}

// doctorEnv is what doctor looks at besides PATH and the binaries on it.
// Tests replace the parts that would reach the network, Chrome or the
// real home folder.
type doctorEnv struct {
	getenv    func(string) string
	now       func() time.Time
	http      *http.Client
	writable  func(dir string) bool
	lookPath  func(string) (string, error)
	exists    func(string) bool
	configDir func() (string, error)
	cacheDir  func(tool string) (string, error)
	// runTool runs a tool binary and returns its stdout and exit status.
	runTool func(ctx context.Context, timeout time.Duration, bin string, args ...string) ([]byte, int, error)
}

// dataTimeout bounds one tool's data refresh in doctor. Tuner's first
// vulnerability download can take a while on a slow line.
var dataTimeout = 2 * time.Minute

// newDoctorEnv is the real environment. It is a variable so tests can
// swap in fakes.
var newDoctorEnv = func() doctorEnv {
	return doctorEnv{
		getenv:   os.Getenv,
		now:      time.Now,
		http:     &http.Client{Timeout: 10 * time.Second},
		writable: writable,
		lookPath: exec.LookPath,
		exists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
		configDir: config.Dir,
		cacheDir:  config.CacheDir,
		runTool:   runTool,
	}
}

// runTool runs bin with args for at most timeout and returns what it
// printed on stdout. A non-zero exit is not an error: exit says it.
func runTool(ctx context.Context, timeout time.Duration, bin string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := probeOutput(ctx, bin, args...)
	var ee *exec.ExitError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return out, -1, fmt.Errorf("no answer within %s", timeout)
	case errors.As(err, &ee):
		return out, ee.ExitCode(), nil
	case err != nil:
		return out, -1, err
	}
	return out, 0, nil
}

func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("doctor", doctorSynopsis)
	offline := flags.Bool("offline", false, "skip the data refresh and the release API check")
	asJSON := flags.Bool("json", false, "print the checks as rhythm-doctor/v1 JSON")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) > 0 {
		fmt.Fprintf(stderr, "rhythm: doctor takes no names (got %q)\n", pos[0])
		return exitUsage
	}
	ctx, stop := signalContext()
	defer stop()
	env := newDoctorEnv()
	checks := runDoctor(ctx, env, *offline || offlineOn(env.getenv(offlineEnv)))
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: doctor interrupted")
		return exitInterrupted
	}
	if *asJSON {
		if err := writeJSON(stdout, doctorReport{Schema: "rhythm-doctor/v1", Checks: checks}); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
	} else {
		printChecks(stdout, checks)
	}
	if anyFailed(checks) {
		return 1
	}
	return 0
}

// runDoctor runs every check, in the order they are shown.
func runDoctor(ctx context.Context, env doctorEnv, offline bool) []check {
	targets, _ := findTargets(ctx, nil)
	var checks []check
	checks = append(checks, checkPath(targets)...)
	checks = append(checks, checkInstallDir(env, targets)...)
	checks = append(checks, checkBuilds(targets)...)
	checks = append(checks, checkUpdateState(env, targets)...)
	if !offline {
		checks = append(checks, checkData(ctx, env, targets)...)
	}
	checks = append(checks, checkChrome(ctx, env, targets)...)
	checks = append(checks, checkToken(env))
	if !offline {
		checks = append(checks, checkAPI(ctx, env))
	}
	return checks
}

// printChecks writes one line per check and a closing count.
func printChecks(w io.Writer, checks []check) {
	warns, fails := 0, 0
	for _, c := range checks {
		fmt.Fprintf(w, "%-4s  %s\n", c.Status, c.Message)
		switch c.Status {
		case statusWarn:
			warns++
		case statusFail:
			fails++
		}
	}
	if warns+fails == 0 {
		fmt.Fprintln(w, "No problems found.")
		return
	}
	fmt.Fprintf(w, "%s, %s.\n", plural(fails, "failure"), plural(warns, "warning"))
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

func anyFailed(checks []check) bool {
	for _, c := range checks {
		if c.Status == statusFail {
			return true
		}
	}
	return false
}

// checkPath reports which copy of each binary runs and the copies it
// hides further down PATH. A hidden copy is usually the one rhythm
// updates, so updates seem to do nothing.
func checkPath(targets []target) []check {
	var checks []check
	for _, t := range targets {
		id := "path." + t.binary
		copies := pathCopies(t.binary)
		if t.tool == selfName {
			switch {
			case len(copies) == 0:
				checks = append(checks, check{id, statusWarn, fmt.Sprintf("rhythm isn't on your PATH; add %s to it", filepath.Dir(t.real))})
				continue
			case resolve(copies[0]) != t.real:
				checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s comes first on your PATH, not this rhythm (%s)", copies[0], t.real)})
				continue
			}
		}
		var hidden []string
		for _, c := range copies[1:] {
			if resolve(c) != resolve(copies[0]) {
				hidden = append(hidden, c)
			}
		}
		if len(hidden) > 0 {
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s: %s comes first on your PATH and hides %s; remove the copy you don't use",
				t.binary, copies[0], strings.Join(hidden, ", "))})
			continue
		}
		checks = append(checks, check{id, statusOK, fmt.Sprintf("%s runs from %s", t.binary, copies[0])})
	}
	return checks
}

// checkInstallDir reports whether rhythm can write next to itself, where
// rhythm install puts tools and rhythm update replaces binaries.
func checkInstallDir(env doctorEnv, targets []target) []check {
	for _, t := range targets {
		if t.tool != selfName {
			continue
		}
		dir := filepath.Dir(t.real)
		switch {
		case t.managed != nil:
			return []check{{"install-dir", statusOK, fmt.Sprintf("rhythm is managed by %s; update it with: %s", t.managed.name, t.managed.upgrade)}}
		case env.writable(dir):
			return []check{{"install-dir", statusOK, dir + " is writable"}}
		default:
			return []check{{"install-dir", statusWarn, fmt.Sprintf("can't write to %s; rhythm install and rhythm update need sudo there, or install to a folder you own with --dir", dir)}}
		}
	}
	return nil
}

// checkBuilds flags development builds and binaries whose version can't
// be read: rhythm update leaves both alone.
func checkBuilds(targets []target) []check {
	var checks []check
	for _, t := range targets {
		id := "version." + t.binary
		switch {
		case t.current == unknownVersion:
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s version unknown (%s); rhythm update leaves it alone", t.binary, t.probe)})
		case !update.IsRelease(t.current):
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s is a development build (%s); rhythm update leaves it alone", t.binary, t.current)})
		}
	}
	if len(checks) == 0 {
		checks = append(checks, check{"version", statusOK, "every binary is a release build"})
	}
	return checks
}

// updateState is the saved answer of a daily update check, as kit/update
// writes it to update-check/<binary>.json.
type updateState struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

// checkUpdateState reads the saved update checks. A broken file, one
// dated in the future, one for a binary that is gone or one saying a
// newer release is out is worth knowing; the age alone is not, since a
// tool checks again the next day it runs.
func checkUpdateState(env doctorEnv, targets []target) []check {
	if v := firstSet(env, "RHYTHM_NO_UPDATE_CHECK", "CI"); v != "" {
		return []check{{"update-check", statusOK, "update checks are off (" + v + " is set)"}}
	}
	dir, err := env.configDir()
	if err != nil {
		return []check{{"update-check", statusWarn, "can't find the config folder: " + err.Error()}}
	}
	dir = filepath.Join(dir, "update-check")
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) == 0 {
		return []check{{"update-check", statusOK, "no update checks saved yet"}}
	}
	current := map[string]string{}
	for _, tp := range thirdPartyTools() {
		current["rhythm-"+tp.name] = "" // installed; its version isn't compared
	}
	for _, t := range targets {
		current[t.binary] = t.current
	}
	var checks []check
	for _, f := range files {
		binary := strings.TrimSuffix(filepath.Base(f), ".json")
		id := "update-check." + binary
		var st updateState
		data, err := os.ReadFile(f) //nolint:gosec // a file in rhythm's own config folder
		if err == nil {
			err = json.Unmarshal(data, &st)
		}
		v, installed := current[binary]
		switch {
		case err != nil:
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s can't be read; delete it and the next check rewrites it", f)})
		case st.Checked.After(env.now().Add(time.Hour)):
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s is dated in the future (%s), so no check runs until then; delete it", f, st.Checked.Format(time.DateOnly))})
		case !installed:
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s is left from %s, which isn't installed; delete it", f, binary)})
		case update.Newer(v, st.Latest):
			checks = append(checks, check{id, statusWarn, fmt.Sprintf("%s %s is out (%s installed); run: rhythm update", binary, st.Latest, v)})
		}
	}
	if len(checks) == 0 {
		checks = append(checks, check{"update-check", statusOK, fmt.Sprintf("%d saved update checks, none stale", len(files))})
	}
	return checks
}

// firstSet returns the name of the first variable that is set.
func firstSet(env doctorEnv, names ...string) string {
	for _, n := range names {
		if env.getenv(n) != "" {
			return n
		}
	}
	return ""
}

// dataReport is a tool's answer to `update --json`.
type dataReport struct {
	Schema string `json:"schema"`
	Data   []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"data"`
}

// staleData is how old dated data may be before doctor warns.
const staleData = 14 * 24 * time.Hour

// datedVersion matches data versions that start with a date, such as
// 2026-10-20 or 20261020.1.
var datedVersion = regexp.MustCompile(`^(\d{4})-?(\d{2})-?(\d{2})`)

// checkData refreshes each tool's data through `update --json`, the
// rhythm-data/v1 answer of tools built on kit/cli. It reports what they
// hold. A tool that doesn't answer in that form isn't reported: its data
// can't be judged from outside.
func checkData(ctx context.Context, env doctorEnv, targets []target) []check {
	var checks []check
	for _, t := range targets {
		if t.tool == selfName || t.hung || ctx.Err() != nil {
			continue
		}
		id := "data." + t.binary
		out, exit, err := env.runTool(ctx, dataTimeout, t.path, "update", "--json")
		var rep dataReport
		known := json.Unmarshal(out, &rep) == nil && rep.Schema == "rhythm-data/v1"
		switch {
		case ctx.Err() != nil:
			return checks
		case err != nil && known:
			checks = append(checks, check{id, statusFail, fmt.Sprintf("%s data refresh failed (%v)", t.binary, err)})
		case err != nil:
			continue
		case exit == 3 || (known && exit != 0):
			// 3 is kit/cli's "the data couldn't be refreshed".
			checks = append(checks, check{id, statusFail, fmt.Sprintf("%s data refresh failed (exit %d); run: rhythm-%s update", t.binary, exit, t.tool)})
		case !known:
			continue
		case len(rep.Data) == 0:
			checks = append(checks, check{id, statusWarn, t.binary + " reported no data"})
		default:
			checks = append(checks, dataCheck(id, t.binary, rep, env.now()))
		}
	}
	return checks
}

// dataCheck words one tool's data report, warning about dated data that
// is older than staleData even right after a refresh.
func dataCheck(id, binary string, rep dataReport, now time.Time) check {
	var parts, old []string
	for _, d := range rep.Data {
		parts = append(parts, d.Name+" "+d.Version)
		if m := datedVersion.FindStringSubmatch(d.Version); m != nil {
			if day, err := time.Parse("20060102", m[1]+m[2]+m[3]); err == nil && now.Sub(day) > staleData {
				old = append(old, fmt.Sprintf("%s is %d days old", d.Name, int(now.Sub(day).Hours()/24)))
			}
		}
	}
	if len(old) > 0 {
		return check{id, statusWarn, fmt.Sprintf("%s data: %s, even after a refresh", binary, strings.Join(old, ", "))}
	}
	return check{id, statusOK, fmt.Sprintf("%s data: %s", binary, strings.Join(parts, ", "))}
}

// checkChrome looks for the browser Soundcheck drives, when Soundcheck is
// installed. Soundcheck's own `doctor --json` is asked first, since it
// knows its settings; otherwise doctor looks where Chrome and Chromium
// usually are.
func checkChrome(ctx context.Context, env doctorEnv, targets []target) []check {
	var sc *target
	for i := range targets {
		if targets[i].tool == "soundcheck" {
			sc = &targets[i]
		}
	}
	if sc == nil || sc.hung {
		return nil
	}
	if out, exit, err := env.runTool(ctx, 10*time.Second, sc.path, "doctor", "--json"); err == nil && exit <= 1 {
		var rep struct {
			Checks []check `json:"checks"`
		}
		if json.Unmarshal(out, &rep) == nil && len(rep.Checks) > 0 {
			checks := make([]check, 0, len(rep.Checks))
			for _, c := range rep.Checks {
				if c.Status != statusOK && c.Status != statusFail {
					c.Status = statusWarn
				}
				checks = append(checks, check{"soundcheck." + c.ID, c.Status, "soundcheck: " + c.Message})
			}
			return checks
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "microsoft-edge", "msedge"} {
		if p, err := env.lookPath(name); err == nil {
			return []check{{"chrome", statusOK, "Soundcheck can use " + p}}
		}
	}
	for _, p := range chromePaths(env) {
		if env.exists(p) {
			return []check{{"chrome", statusOK, "Soundcheck can use " + p}}
		}
	}
	if d, err := env.cacheDir("soundcheck"); err == nil && env.exists(filepath.Join(d, "browser")) {
		return []check{{"chrome", statusOK, "Soundcheck uses the Chromium it downloaded to " + filepath.Join(d, "browser")}}
	}
	return []check{{"chrome", statusWarn, "no Chrome or Chromium found; Soundcheck downloads Chromium on its first run, or pass it --browser /path/to/chrome"}}
}

// chromePaths lists where Chrome, Chromium and Edge are installed outside
// PATH on this OS.
func chromePaths(env doctorEnv) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "windows":
		var paths []string
		for _, root := range []string{env.getenv("ProgramFiles"), env.getenv("ProgramFiles(x86)"), env.getenv("LOCALAPPDATA")} {
			if root == "" {
				continue
			}
			paths = append(paths,
				filepath.Join(root, `Google\Chrome\Application\chrome.exe`),
				filepath.Join(root, `Chromium\Application\chrome.exe`),
				filepath.Join(root, `Microsoft\Edge\Application\msedge.exe`))
		}
		return paths
	}
	return []string{"/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/chromium-browser", "/snap/bin/chromium"}
}

// checkToken says which GitHub token variables are set and which release
// API rhythm asks. Token values are never shown.
func checkToken(env doctorEnv) check {
	var set []string
	for _, v := range []string{"GITHUB_TOKEN", "RHYTHM_GITHUB_TOKEN"} {
		if env.getenv(v) != "" {
			set = append(set, v)
		}
	}
	msg := "no GitHub token set; GitHub allows 60 release lookups an hour without one"
	if len(set) > 0 {
		msg = "token in use: " + strings.Join(set, ", ") + " (value not shown)"
	}
	if api := env.getenv("RHYTHM_GITHUB_API"); api != "" {
		msg += "; release API " + redactURL(api) + " (from RHYTHM_GITHUB_API)"
	}
	return check{"token", statusOK, msg}
}

// checkAPI asks the release API's rate_limit endpoint, which GitHub
// doesn't count against the limit. Any HTTP answer means it is
// reachable; a mirror without that endpoint answers 404, which is fine.
func checkAPI(ctx context.Context, env doctorEnv) check {
	api := apiOrDefault(env.getenv("RHYTHM_GITHUB_API"))
	host := hostOf(api)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/rate_limit", nil)
	if err != nil {
		return check{"api", statusFail, fmt.Sprintf("RHYTHM_GITHUB_API isn't a usable URL: %s", redactURL(api))}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("rhythm/%s (+https://rythm.sh)", version))
	// GitHub's token goes to GitHub only; a mirror gets its own.
	tokenVar := "RHYTHM_GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	if api == defaultAPI {
		tokenVar = "GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	}
	if tok := env.getenv(tokenVar); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := env.http.Do(req)
	if err != nil {
		return check{"api", statusFail, fmt.Sprintf("can't reach %s; check network, proxy or RHYTHM_GITHUB_API", host)}
	}
	defer func() { _ = resp.Body.Close() }()
	h := resp.Header
	if h.Get("X-RateLimit-Remaining") == "0" {
		e := &rateLimitedError{api: api, tokenSent: req.Header.Get("Authorization") != ""}
		if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil && n > 0 {
			e.reset = time.Unix(n, 0)
		}
		return check{"api", statusWarn, e.explain("")}
	}
	if resp.StatusCode >= 500 {
		return check{"api", statusWarn, fmt.Sprintf("%s answered HTTP %d; try again later", host, resp.StatusCode)}
	}
	msg := host + " answers"
	if left, limit := h.Get("X-RateLimit-Remaining"), h.Get("X-RateLimit-Limit"); left != "" && limit != "" {
		msg += fmt.Sprintf("; %s of %s requests left this hour", left, limit)
	}
	return check{"api", statusOK, msg}
}
