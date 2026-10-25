// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/RhythmShahriar/rhythm/kit/cache"
	"github.com/RhythmShahriar/rhythm/kit/config"
	kitterm "github.com/RhythmShahriar/rhythm/kit/term"
)

// PromptMode is what a tool does at the end of a run when a newer
// release is out: ask to update, print the notice or say nothing.
type PromptMode string

// The prompt modes `rhythm config update-prompt` takes.
const (
	PromptAsk    PromptMode = "ask"    // ask "Update now? [y/N]" on a terminal, else the notice
	PromptNotify PromptMode = "notify" // only the notice
	PromptOff    PromptMode = "off"    // nothing at all
)

// PromptEnv overrides the saved prompt mode for one shell or one run.
const PromptEnv = "RHYTHM_UPDATE_PROMPT"

// PromptSetting is the key of the prompt mode in config.SettingsFile.
const PromptSetting = "update-prompt"

// ParsePromptMode reads "ask", "notify" or "off", in any case.
func ParsePromptMode(s string) (PromptMode, error) {
	switch m := PromptMode(strings.ToLower(strings.TrimSpace(s))); m {
	case PromptAsk, PromptNotify, PromptOff:
		return m, nil
	}
	return "", fmt.Errorf("%q isn't an update-prompt mode; use ask, notify or off", s)
}

// CurrentPromptMode returns the prompt mode in force and where it comes
// from: PromptEnv when that is set, "config" when it was saved with
// SetPromptMode, otherwise "default" (PromptAsk). A bad value is an
// error, returned with PromptNotify, the mode that never reads input.
func CurrentPromptMode() (mode PromptMode, source string, err error) {
	if v := os.Getenv(PromptEnv); v != "" {
		m, err := ParsePromptMode(v)
		if err != nil {
			return PromptNotify, PromptEnv, fmt.Errorf("%s: %w", PromptEnv, err)
		}
		return m, PromptEnv, nil
	}
	v, err := config.Setting(PromptSetting)
	switch {
	case err != nil:
		return PromptNotify, "config", err
	case v == "":
		return PromptAsk, "default", nil
	}
	m, err := ParsePromptMode(v)
	if err != nil {
		return PromptNotify, "config", err
	}
	return m, "config", nil
}

// SetPromptMode saves m in the config folder, for every tool.
func SetPromptMode(m PromptMode) error {
	if _, err := ParsePromptMode(string(m)); err != nil {
		return err
	}
	return config.SetSetting(PromptSetting, string(m))
}

// Pending is a release check running in the background, started with
// StartCheck at the beginning of a run and read by PromptAtExit at its
// end.
type Pending struct {
	u      *Updater
	done   chan struct{}
	latest string
	newer  bool
}

// StartCheck runs Check in the background, with at most five seconds
// for the request. It returns nil without sending anything when u is
// nil, u.Offline is set or CheckDisabled is true; PromptAtExit takes a nil
// Pending and does nothing.
func StartCheck(ctx context.Context, u *Updater) *Pending {
	if u == nil || u.checkDisabled() {
		return nil
	}
	p := &Pending{u: u, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		p.latest, p.newer = u.Check(cctx)
	}()
	return p
}

// result waits up to wait for the check and returns the newer release,
// or ok false when there is none or the check isn't done yet.
func (p *Pending) result(wait time.Duration) (latest string, ok bool) {
	if p == nil {
		return "", false
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-p.done:
		return p.latest, p.newer
	case <-t.C:
		return "", false
	}
}

// PromptOptions configures PromptAtExit. Only Check is needed; every
// other field has a working default.
type PromptOptions struct {
	Check *Pending // from StartCheck; nil means nothing to say

	// Name is the tool as people know it, "Tuner"; the default is the
	// binary without "rhythm-", capitalized ("rhythm" stays as it is).
	Name string
	// Tool is the name `rhythm update <Tool>` takes; the default is the
	// binary without "rhythm-".
	Tool string

	// Quiet is set for a run that asked for little output (--json, -q,
	// --quiet). It never asks then, though the notice still shows.
	Quiet bool

	// Wait is how long to wait for a check still running; default
	// 200ms. The answer is cached for a day, so a missed one shows on a
	// later run.
	Wait time.Duration

	In     io.Reader // where the answer is read; default os.Stdin
	Stdout io.Writer // the run's output, which must be a terminal to ask; default os.Stdout
	Out    io.Writer // where the notice and prompt go; default os.Stderr

	// Interactive, when set, replaces the terminal test: In, Stdout and
	// Out all terminals. Tests set it.
	Interactive func() bool

	// Install does the update after a yes, writing its progress to out.
	// The default runs `rhythm update <Tool>` from PATH, so the update
	// goes through rhythm's own verified path, signature check included.
	// When it is nil and rhythm isn't on PATH, the notice is shown
	// instead of the prompt.
	Install func(ctx context.Context, out io.Writer) error
}

// Outcome is what PromptAtExit did.
type Outcome int

// The outcomes of PromptAtExit.
const (
	OutcomeNone     Outcome = iota // nothing to say, mode off, or interrupted
	OutcomeNotice                  // printed the one-line notice
	OutcomeDeclined                // asked; the answer was no (and is remembered)
	OutcomeUpdated                 // asked; yes; the update worked
	OutcomeFailed                  // asked; yes; the update failed and the old binary stays
)

// lookPath finds rhythm for the default Install. Tests replace it.
var lookPath = exec.LookPath

// PromptAtExit is what a tool calls once its work is done, never before:
// when the check found a newer release it asks on Out, in the manner of
// oh-my-zsh,
//
//	A new release of Tuner is out: 0.4.0 (you have 0.3.2). Update now? [y/N]
//
// and updates on y. The answer defaults to no. It asks only when the
// prompt mode is PromptAsk, the run is interactive (In, Stdout and Out
// are terminals), CI is unset, the run isn't Quiet, the user hasn't
// already said no to this version and Install can run; otherwise it
// prints the notice, or nothing in PromptOff mode. A no is remembered in
// the update-check state, so each version is asked about once. Ctrl-C
// (ctx canceled) at the prompt ends it without remembering anything.
func PromptAtExit(ctx context.Context, opts PromptOptions) Outcome {
	opts.defaults()
	latest, ok := opts.Check.result(opts.Wait)
	if !ok {
		return OutcomeNone
	}
	u := opts.Check.u
	latest = kitterm.SafeText(latest)
	mode, _, err := CurrentPromptMode()
	if mode == PromptOff {
		return OutcomeNone
	}
	notice := func() Outcome {
		fmt.Fprintln(opts.Out, noticeText(u, latest))
		return OutcomeNotice
	}
	if err != nil || mode != PromptAsk || opts.Quiet || os.Getenv("CI") != "" || !opts.Interactive() {
		return notice()
	}
	if declined(u.Binary) == latest {
		return notice()
	}
	install := opts.Install
	if install == nil {
		if install = rhythmUpdate(opts.Tool); install == nil {
			return notice()
		}
	}

	fmt.Fprintf(opts.Out, "A new release of %s is out: %s (you have %s). Update now? [y/N] ", opts.Name, latest, u.Current)
	answer, ok := readAnswer(ctx, opts.In)
	if !ok {
		fmt.Fprintln(opts.Out) // end the prompt line
		return OutcomeNone
	}
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		remember(u.Binary, latest)
		return OutcomeDeclined
	}
	if err := install(ctx, opts.Out); err != nil {
		fmt.Fprintf(opts.Out, "%s wasn't updated and %s is still installed: %v\n  try again with: rhythm update %s\n",
			opts.Name, u.Current, err, opts.Tool)
		return OutcomeFailed
	}
	return OutcomeUpdated
}

func (o *PromptOptions) defaults() {
	if o.Check == nil {
		return
	}
	bin := o.Check.u.Binary
	if o.Tool == "" {
		o.Tool = strings.TrimPrefix(bin, "rhythm-")
	}
	if o.Name == "" {
		o.Name = displayName(bin)
	}
	if o.Wait <= 0 {
		o.Wait = 200 * time.Millisecond
	}
	if o.In == nil {
		o.In = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Out == nil {
		o.Out = os.Stderr
	}
	if o.Interactive == nil {
		in, stdout, out := o.In, o.Stdout, o.Out
		o.Interactive = func() bool { return isTerminal(in) && isTerminal(stdout) && isTerminal(out) }
	}
}

// displayName turns rhythm-soundcheck into Soundcheck.
func displayName(binary string) string {
	name, ok := strings.CutPrefix(binary, "rhythm-")
	if !ok || name == "" {
		return binary
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// isTerminal reports whether v is an *os.File open on a terminal.
func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

// noticeText is the one-line notice that a newer release is out.
func noticeText(u *Updater, latest string) string {
	return fmt.Sprintf("A new release of %s is available: %s → %s\n  run: rhythm update", u.Binary, u.Current, latest)
}

// readAnswer reads one line from in. ok is false when ctx ends first or
// in is closed before anything was typed.
func readAnswer(ctx context.Context, in io.Reader) (string, bool) {
	ch := make(chan string, 1)
	go func() {
		// The goroutine outlives a canceled prompt only until the process
		// exits, which is what follows a Ctrl-C.
		line, err := bufio.NewReader(io.LimitReader(in, 256)).ReadString('\n')
		if err != nil && line == "" {
			close(ch)
			return
		}
		ch <- line
	}()
	select {
	case <-ctx.Done():
		return "", false
	case line, ok := <-ch:
		return line, ok
	}
}

// declined returns the version the user last said no to for binary.
func declined(binary string) string {
	path, err := checkStatePath(binary)
	if err != nil {
		return ""
	}
	var st checkState
	if cache.ReadJSON(path, &st) != nil {
		return ""
	}
	return st.Declined
}

// remember saves version as declined for binary, keeping the rest of the
// state. Without a state file there is nothing to keep: the check that
// found version wrote one.
func remember(binary, version string) {
	path, err := checkStatePath(binary)
	if err != nil {
		return
	}
	var st checkState
	if err := cache.ReadJSON(path, &st); err != nil && !errors.Is(err, os.ErrNotExist) {
		return // a broken file is the next check's to rewrite
	}
	st.Declined = version
	_ = cache.WriteJSON(path, st)
}

// rhythmUpdate returns an Install that runs `rhythm update <tool>`, or
// nil when rhythm isn't on PATH. The child shares the terminal, so it
// prints its own progress and verdict; its stdout goes to out with the
// rest of the prompt. It is the same command a person would type, so
// the signature check can't be skipped from here.
func rhythmUpdate(tool string) func(ctx context.Context, out io.Writer) error {
	bin, err := lookPath("rhythm")
	if err != nil {
		return nil
	}
	return func(ctx context.Context, out io.Writer) error {
		cmd := exec.CommandContext(ctx, bin, "update", tool) //nolint:gosec // rhythm from PATH, as the user would run it
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, out
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return fmt.Errorf("rhythm update %s exited %d", tool, ee.ExitCode())
			}
			return err
		}
		return nil
	}
}
