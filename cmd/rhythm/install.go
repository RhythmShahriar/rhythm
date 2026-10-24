// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/cli"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

// Exit codes of the router's own commands. They match the ExitUsage and
// ExitInterrupted that kit/cli is getting, so rhythm and its tools agree.
const (
	exitUsage       = 2
	exitInterrupted = 130
)

// signalContext is the context install and update run under, ended
// by Ctrl-C or SIGTERM. Tests replace it to simulate an interrupt.
var signalContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// newFlagSet returns a flag set whose usage prints synopsis once and then
// every flag with a double dash, the way rhythm's help writes them.
func newFlagSet(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, synopsis)
		fmt.Fprintln(w, "\nFlags:")
		fs.VisitAll(func(f *flag.Flag) {
			arg, usage := flag.UnquoteUsage(f)
			name := "--" + f.Name
			if arg != "" {
				name += " " + arg
			}
			if f.DefValue != "" && f.DefValue != "false" {
				usage += fmt.Sprintf(" (default %s)", f.DefValue)
			}
			fmt.Fprintf(w, "  %-16s %s\n", name, usage)
		})
	}
	return fs
}

// parseArgs parses flags before or after positional arguments. When ok is
// false the caller returns code: 0 after --help (usage on stdout) or
// exitUsage after a bad flag (error and usage on stderr).
func parseArgs(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (pos []string, code int, ok bool) {
	// The flag package prints the usage itself; catch it so --help can go
	// to stdout and errors to stderr.
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	pos, err := parseInterspersed(fs, args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		_, _ = stdout.Write(buf.Bytes())
		return nil, 0, false
	case err != nil:
		_, _ = stderr.Write(buf.Bytes())
		return nil, exitUsage, false
	}
	return pos, 0, true
}

// defaultAPI is the release API rhythm asks unless RHYTHM_GITHUB_API
// names a mirror.
const defaultAPI = "https://api.github.com"

func updater(repo, binary, current string) *update.Updater {
	api := os.Getenv("RHYTHM_GITHUB_API") // a mirror or GitHub Enterprise; default api.github.com
	return &update.Updater{
		Repo:      repo,
		Binary:    binary,
		Current:   current,
		UserAgent: fmt.Sprintf("rhythm/%s (+https://rythm.sh)", version),
		API:       api,
		HTTP: &http.Client{
			Timeout:   5 * time.Minute, // kit/update's own default
			Transport: &explainTransport{base: http.DefaultTransport, api: apiOrDefault(api), repo: repo},
		},
	}
}

// apiOrDefault returns api, or defaultAPI when it is empty.
func apiOrDefault(api string) string {
	if api == "" {
		return defaultAPI
	}
	return strings.TrimSuffix(api, "/")
}

// explainTransport turns the release API's rate limit and a missing
// release into errors rhythm can explain. kit/update on its own reports
// both as a bare HTTP status, which says nothing about the fix.
type explainTransport struct {
	base http.RoundTripper
	api  string // the API base asked, without a trailing slash
	repo string

	// While archiveURL is set, its download body reports to progress.
	// kit/update has no progress callback of its own yet.
	archiveURL string
	progress   func(done, total int64)
}

func (t *explainTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	h := resp.Header
	switch code := resp.StatusCode; {
	case (code == http.StatusForbidden || code == http.StatusTooManyRequests) &&
		(h.Get("X-RateLimit-Remaining") == "0" || h.Get("Retry-After") != ""):
		_ = resp.Body.Close()
		e := &rateLimitedError{api: t.api, tokenSent: req.Header.Get("Authorization") != ""}
		if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil && n > 0 {
			e.reset = time.Unix(n, 0)
		} else if secs, err := strconv.Atoi(h.Get("Retry-After")); err == nil && secs >= 0 {
			e.reset = time.Now().Add(time.Duration(secs) * time.Second)
		}
		return nil, e
	case code == http.StatusNotFound && strings.HasPrefix(req.URL.String(), t.api+"/repos/") &&
		strings.HasSuffix(req.URL.Path, "/releases/latest"):
		_ = resp.Body.Close()
		return nil, &noReleaseError{api: t.api, repo: t.repo}
	case code == http.StatusOK && t.progress != nil && t.archiveURL != "" && requested(req, t.archiveURL):
		resp.Body = &countingBody{ReadCloser: resp.Body, total: resp.ContentLength, progress: t.progress}
	}
	return resp, nil
}

// requested reports whether req asks for rawURL, directly or as the end
// of a redirect: GitHub sends release downloads on to its CDN.
func requested(req *http.Request, rawURL string) bool {
	for r := req; r != nil; {
		if r.URL.String() == rawURL {
			return true
		}
		if r.Response == nil {
			break
		}
		r = r.Response.Request
	}
	return false
}

// rateLimitedError is a 403 or 429 that the release API marks as rate
// limiting.
type rateLimitedError struct {
	api       string
	tokenSent bool      // a token was sent, so suggesting one won't help
	reset     time.Time // when the limit lifts; zero when the API didn't say
}

func (e *rateLimitedError) Error() string { return e.explain("") }

// explain words the limit for the binaries it stopped, e.g. "GitHub rate
// limit reached for rhythm-tuner; set GITHUB_TOKEN (resets 14:05)".
func (e *rateLimitedError) explain(binaries string) string {
	var b strings.Builder
	tokenVar := "GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	if e.api == defaultAPI {
		b.WriteString("GitHub rate limit reached")
	} else {
		// GITHUB_TOKEN is kept for GitHub itself; a mirror gets its own.
		fmt.Fprintf(&b, "rate limit reached at %s", e.api)
		tokenVar = "RHYTHM_GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	}
	if binaries != "" {
		b.WriteString(" for " + binaries)
	}
	if !e.tokenSent {
		b.WriteString("; set " + tokenVar)
	}
	if !e.reset.IsZero() {
		layout := "15:04"
		if time.Until(e.reset) >= 24*time.Hour {
			layout = "Jan 2 15:04"
		}
		fmt.Fprintf(&b, " (resets %s)", e.reset.Local().Format(layout))
	}
	return b.String()
}

// noReleaseError is a 404 from the release API for a repository.
type noReleaseError struct {
	api, repo string
}

func (e *noReleaseError) Error() string {
	if e.api == defaultAPI {
		return e.repo + " has no published releases yet"
	}
	// A mirror that lacks the repository is usually the wrong mirror.
	return fmt.Sprintf("mirror %s has no release for %s; check RHYTHM_GITHUB_API", e.api, e.repo)
}

// describe returns err's message for the failure line. kit/update hands
// back the transport's errors inside Go's url.Error, whose `Get "<url>":`
// prefix would bury rhythm's explanation.
func describe(err error) string {
	var nr *noReleaseError
	if errors.As(err, &nr) {
		return nr.Error()
	}
	return err.Error()
}

// netProblem explains err when it is about the network or the release
// service, not the one binary that hit it, so it reads the same for every
// binary. key groups identical problems, host is the host they block
// ("" when they block nothing else) and text words the problem for the
// comma-separated binaries it hit.
func netProblem(err error) (key, host string, text func(binaries string) string, ok bool) {
	var rl *rateLimitedError
	if errors.As(err, &rl) {
		return "rate " + rl.api, hostOf(rl.api), rl.explain, true
	}
	var op *net.OpError
	var dns *net.DNSError
	var ne net.Error
	unreachable := errors.As(err, &op) || errors.As(err, &dns) || (errors.As(err, &ne) && ne.Timeout())
	if !unreachable {
		return "", "", nil, false
	}
	var ue *url.Error
	switch {
	case errors.As(err, &ue):
		host = hostOf(ue.URL)
	case dns != nil:
		host = dns.Name
	default:
		return "", "", nil, false
	}
	return "reach " + host, host, func(binaries string) string {
		return fmt.Sprintf("can't reach %s for %s; check network, proxy or RHYTHM_GITHUB_API", host, binaries)
	}, true
}

// hostOf returns the host[:port] of rawURL, or rawURL when it doesn't
// parse.
func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Host
	}
	return rawURL
}

// netReport collects the network problems of one run, so each is said
// once with every binary it hit instead of once per binary.
type netReport struct {
	keys    []string // in the order they first happened
	byKey   map[string]*netGroup
	blocked map[string]string // host -> key of the problem that blocks it
}

type netGroup struct {
	binaries []string
	text     func(binaries string) string
}

// add records err for binary when it is a network problem and reports
// whether it was one. The caller prints other errors itself.
func (r *netReport) add(binary string, err error) bool {
	key, host, text, ok := netProblem(err)
	if !ok {
		return false
	}
	if r.byKey == nil {
		r.byKey, r.blocked = map[string]*netGroup{}, map[string]string{}
	}
	g := r.byKey[key]
	if g == nil {
		g = &netGroup{text: text}
		r.byKey[key] = g
		r.keys = append(r.keys, key)
	}
	g.binaries = append(g.binaries, binary)
	if host != "" {
		r.blocked[host] = key
	}
	return true
}

// skip reports whether api is already known to be unreachable or rate
// limited in this run. If so it counts binary as hit too. Asking again
// would only repeat the wait or spend the limit.
func (r *netReport) skip(binary, api string) bool {
	key, ok := r.blocked[hostOf(apiOrDefault(api))]
	if ok {
		r.byKey[key].binaries = append(r.byKey[key].binaries, binary)
	}
	return ok
}

// explain returns the line the network problem that hit binary prints,
// or "" when none did.
func (r *netReport) explain(binary string) string {
	for _, k := range r.keys {
		g := r.byKey[k]
		for _, b := range g.binaries {
			if b == binary {
				return g.text(binary)
			}
		}
	}
	return ""
}

// print writes one line per problem.
func (r *netReport) print(w io.Writer) {
	for _, k := range r.keys {
		g := r.byKey[k]
		fmt.Fprintln(w, g.text(strings.Join(g.binaries, ", ")))
	}
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

const installSynopsis = `Usage: rhythm install <tool>... [flags]

Downloads the latest release of each tool and installs it next to rhythm.
Separate names with spaces or commas; "none" installs nothing. A tool
that fails doesn't stop the others.

Exit status: 0 when every tool was installed, 1 when any failed, 2 for a
usage error and 130 when interrupted.`

// installOptions are the rhythm install flags that shape each tool's
// install.
type installOptions struct {
	dir      string // absolute install folder
	quiet    bool
	force    bool      // reinstall a binary already on the latest release
	progress io.Writer // where the download progress line goes; nil for none
}

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("install", installSynopsis)
	dir := flags.String("dir", "", "install into `dir` (default: next to rhythm)")
	quiet := flags.Bool("quiet", false, "print only the Installed lines and errors")
	dryRun := flags.Bool("dry-run", false, "check the names and print the tools, one per line, without installing")
	defaults := flags.Bool("defaults", false, "also install the tools a fresh setup gets")
	force := flags.Bool("force", false, "reinstall tools that are already on the latest release")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	// Every name is checked before anything is downloaded, so a typo in
	// the third name doesn't leave the first two installed.
	list, code := installScope(pos, *defaults, stderr)
	if code != 0 {
		return code
	}
	if *dryRun {
		for _, tool := range list {
			fmt.Fprintln(stdout, tool)
		}
		return 0
	}
	if len(list) == 0 {
		if !*quiet {
			fmt.Fprintln(stdout, "Nothing to install.")
		}
		return 0
	}

	if *dir == "" {
		self, err := selfPath()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		*dir = filepath.Dir(self)
	}
	// An absolute path, so the PATH note compares like with like:
	// LookPath answers with absolute paths.
	abs, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	// A folder that can't take the binaries should fail now, not after a
	// 30 MB download.
	if err := prepareDir(abs, abs); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	opt := installOptions{dir: abs, quiet: *quiet, force: *force}
	if !*quiet && isTerminal(stderr) {
		opt.progress = stderr
	}

	ctx, stop := signalContext()
	defer stop()
	var installed, already, failed []string
	var nets netReport
	offPath := false
	for _, tool := range list {
		binary := "rhythm-" + tool
		if nets.skip(binary, os.Getenv("RHYTHM_GITHUB_API")) {
			failed = append(failed, binary)
			continue
		}
		res, err := installTool(ctx, tool, opt, stdout)
		if ctx.Err() != nil {
			fmt.Fprintln(stderr, "rhythm: install interrupted")
			return exitInterrupted
		}
		var me *managedError
		switch {
		case err == nil:
		case errors.As(err, &me):
			fmt.Fprintln(stderr, err)
		case nets.add(binary, err):
			// said once for every binary it hit, below
		default:
			fmt.Fprintf(stderr, "%-18s %s\n", binary, describe(err))
		}
		if err != nil {
			failed = append(failed, binary)
			continue
		}
		if res.already {
			already = append(already, binary)
			continue
		}
		installed = append(installed, binary)
		switch {
		case opt.quiet || res.note == "":
		case res.note == noteOffPath:
			offPath = true // said once below, not once per tool
		default:
			fmt.Fprintln(stdout, res.note)
		}
	}
	if offPath {
		fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", opt.dir)
	}
	nets.print(stderr)
	if len(failed) > 0 {
		if len(list) > 1 {
			fmt.Fprintln(stderr, installSummary(len(installed), len(already), failed))
		}
		return 1
	}
	return 0
}

// installSummary is the closing line of an install where something
// failed, e.g. "Installed 2, already installed 1, failed 1: rhythm-echo".
func installSummary(installed, already int, failed []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Installed %d", installed)
	if already > 0 {
		fmt.Fprintf(&b, ", already installed %d", already)
	}
	fmt.Fprintf(&b, ", failed %d: %s", len(failed), strings.Join(failed, ", "))
	return b.String()
}

// installScope turns the names given to rhythm install into the tools to
// install, in the order given and without repeats; --defaults appends the
// tools a fresh setup gets. A non-zero code means a name was wrong.
func installScope(names []string, defaults bool, stderr io.Writer) ([]string, int) {
	list := splitNames(names)
	none := false
	var tools []string
	seen := map[string]bool{}
	add := func(tool string) {
		if !seen[tool] {
			seen[tool] = true
			tools = append(tools, tool)
		}
	}
	bad := false
	for _, n := range list {
		if strings.EqualFold(n, "none") {
			none = true // the installers pass RHYTHM_TOOLS=none through as is
			continue
		}
		tool, ok := resolveName(n)
		switch {
		case ok && tool == selfName:
			fmt.Fprintln(stderr, "rhythm: rhythm installs tools, not itself; update it with: rhythm update --self-only")
			bad = true
		case !ok:
			fmt.Fprintf(stderr, "rhythm: unknown tool %q%s\n", n, suggest(n))
			bad = true
		default:
			add(tool)
		}
	}
	if bad {
		fmt.Fprintf(stderr, "  known: %s\n", strings.Join(toolNames(), ", "))
		return nil, exitUsage
	}
	if defaults {
		for _, tool := range defaultInstallTools() {
			add(tool)
		}
	}
	if len(tools) == 0 && !none && !defaults {
		fmt.Fprintln(stderr, "usage: rhythm install <tool>... [--dir <dir>]; name a tool, or use --defaults")
		return nil, exitUsage
	}
	return tools, 0
}

// defaultInstallTools returns the tools a fresh setup gets. Every tool
// in the registry is one today; router-core's registry marks them with
// DefaultInstall; this then asks it.
func defaultInstallTools() []string { return toolNames() }

// managedError refuses to install over a package manager's binary.
type managedError struct {
	binary string
	m      *manager
}

func (e *managedError) Error() string {
	return fmt.Sprintf("%s is managed by %s; update it with: %s", e.binary, e.m.name, e.m.upgrade)
}

// noteOffPath is installTool's note for a folder that isn't on PATH.
// cmdInstall prints it once for all tools.
const noteOffPath = "off-path"

// installResult is what installTool did with one tool.
type installResult struct {
	note    string // the PATH note for the new binary, if any
	already bool   // the latest release was there already; nothing was downloaded
}

// installTool downloads tool's latest release into opt.dir and prints the
// Installed line, or says the release is there already.
func installTool(ctx context.Context, tool string, opt installOptions, stdout io.Writer) (installResult, error) {
	// An existing symlink is followed, so a link into a tool folder stays
	// a link; a package manager's install is left to the manager.
	binary := "rhythm-" + tool
	dest := filepath.Join(opt.dir, exe(binary))
	writeTo := resolve(dest)
	if m := managedBy(writeTo, binary); m != nil {
		return installResult{}, &managedError{binary, m}
	}
	if target := filepath.Dir(writeTo); target != opt.dir {
		if err := prepareDir(opt.dir, target); err != nil {
			return installResult{}, err
		}
	}
	u := updater(tools[tool], binary, "")
	rel, err := u.Latest(ctx)
	if err != nil {
		return installResult{}, err
	}
	if !opt.force {
		if _, err := os.Stat(writeTo); err == nil {
			if v, _, _ := probeVersion(ctx, dest); v == rel.Version {
				fmt.Fprintf(stdout, "%s %s is already installed at %s (use --force to reinstall)\n", binary, v, dest)
				return installResult{already: true}, nil
			}
		}
	}
	if err := installWithProgress(ctx, u, rel, writeTo, opt.progress); err != nil {
		return installResult{}, err
	}
	fmt.Fprintf(stdout, "Installed %s %s to %s\n", binary, rel.Version, dest)
	return installResult{note: pathNote(binary, dest)}, nil
}

// isTerminal reports whether w is an interactive terminal. Tests replace
// it to see the progress line without a terminal.
var isTerminal = cli.IsTerminal

// installWithProgress runs u.Install and, when w is not nil, keeps one
// line on w updated with how much of the archive has arrived. The line
// is cleared when the download ends, so the Installed line takes its
// place.
func installWithProgress(ctx context.Context, u *update.Updater, rel *update.Release, dest string, w io.Writer) error {
	t, ok := u.HTTP.Transport.(*explainTransport)
	if w == nil || !ok {
		return u.Install(ctx, rel, dest)
	}
	line := &progressLine{w: w, label: fmt.Sprintf("Downloading %s %s", u.Binary, rel.Version)}
	t.archiveURL, t.progress = rel.Assets[u.ArchiveName()], line.update
	defer func() { t.archiveURL, t.progress = "", nil }()
	defer line.clear()
	return u.Install(ctx, rel, dest)
}

// progressLine draws "Downloading rhythm-tuner 0.3.2  12.4/27.1 MB" over
// itself with a carriage return, at most ten times a second.
type progressLine struct {
	w     io.Writer
	label string
	last  time.Time
	drawn bool
}

func (p *progressLine) update(done, total int64) {
	if time.Since(p.last) < 100*time.Millisecond && done != total {
		return
	}
	p.last = time.Now()
	const mb = 1 << 20
	if total > 0 {
		fmt.Fprintf(p.w, "\r%s  %.1f/%.1f MB", p.label, float64(done)/mb, float64(total)/mb)
	} else {
		fmt.Fprintf(p.w, "\r%s  %.1f MB", p.label, float64(done)/mb)
	}
	p.drawn = true
}

// clear erases the line, if it was drawn.
func (p *progressLine) clear() {
	if p.drawn {
		fmt.Fprint(p.w, "\r\x1b[K")
	}
}

// countingBody reports each read of a download body to progress.
type countingBody struct {
	io.ReadCloser
	done, total int64
	progress    func(done, total int64)
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.done += int64(n)
	if n > 0 || err == io.EOF {
		c.progress(c.done, c.total)
	}
	return n, err
}

// prepareDir makes sure the install folder exists and that the binary
// can be written into target, the folder it really lands in (they differ
// when the destination is a symlink). A missing folder is created.
func prepareDir(dir, target string) error {
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		return fmt.Errorf("%s is a file, not a folder; pick a folder with --dir", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // a bin folder others may run from
		return fmt.Errorf("can't create %s: %w", dir, err)
	}
	if !writable(target) {
		return fmt.Errorf("can't write to %s; rerun with sudo, or install to a folder you own with --dir", target)
	}
	return nil
}

// writable reports whether a file can be created in dir. Creating one is
// the only check that holds everywhere: mode bits say nothing about ACLs,
// read-only mounts or Windows.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".rhythm-write-test-*")
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(f.Name()) //nolint:gosec // the probe file CreateTemp just made
	return true
}

// pathNote explains why running binary may not reach the copy just
// installed at dest, or returns "" when it will. A binary found nowhere
// on PATH gives noteOffPath. Both sides have their
// symlinks resolved, so a link on PATH that points at dest counts as dest.
func pathNote(binary, dest string) string {
	found, err := exec.LookPath(binary)
	if errors.Is(err, exec.ErrDot) {
		err = nil // found in the current folder through "." on PATH; still a real shadow
	}
	if err != nil {
		return noteOffPath
	}
	if abs, aerr := filepath.Abs(found); aerr == nil {
		found = abs
	}
	if found == dest || resolve(found) == resolve(dest) {
		return ""
	}
	return fmt.Sprintf("Note: %s comes first on your PATH; remove it to use this install.", found)
}
