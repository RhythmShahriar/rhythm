// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/RhythmShahriar/rhythm/kit/cli"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

func init() {
	register(command{
		name:    "config",
		args:    "[update-prompt [ask|notify|off]]",
		summary: "show or change rhythm's settings",
		run:     cmdConfig,
	})
}

const configSynopsis = `Usage: rhythm config [update-prompt [ask|notify|off]]

Shows rhythm's settings, or one of them, or changes one. They are kept
in settings.json in rhythm's config folder and apply to every tool.

  update-prompt   what rhythm and the tools do at the end of a run when a
                  newer release is out: ask "Update now? [y/N]" (ask, the
                  default), print a notice (notify) or nothing (off).
                  RHYTHM_UPDATE_PROMPT overrides it.`

// cmdConfig is rhythm config: with no argument it lists the settings,
// with a key it prints that one and with a key and a value it saves it.
func cmdConfig(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("config", configSynopsis)
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) > 0 && pos[0] != update.PromptSetting {
		fmt.Fprintf(stderr, "rhythm: unknown setting %q\n", pos[0])
		cli.DidYouMean(stderr, "rhythm config", pos[0], []string{update.PromptSetting})
		fmt.Fprintf(stderr, "  known: %s\n", update.PromptSetting)
		return exitUsage
	}
	switch len(pos) {
	case 0, 1:
		mode, source, err := update.CurrentPromptMode()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		note := ""
		switch source {
		case "default":
			note = "  (default)"
		case update.PromptEnv:
			note = "  (from " + update.PromptEnv + ")"
		}
		if len(pos) == 1 {
			fmt.Fprintf(stdout, "%s%s\n", mode, note)
		} else {
			fmt.Fprintf(stdout, "%s  %s%s\n", update.PromptSetting, mode, note)
		}
		return 0
	case 2:
		mode, err := update.ParsePromptMode(pos[1])
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return exitUsage
		}
		if err := update.SetPromptMode(mode); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s set to %s\n", update.PromptSetting, mode)
		if v := os.Getenv(update.PromptEnv); v != "" {
			fmt.Fprintf(stderr, "Note: %s=%s is set and wins over this setting.\n", update.PromptEnv, v)
		}
		return 0
	}
	fmt.Fprintln(stderr, "usage: rhythm config [update-prompt [ask|notify|off]]")
	return exitUsage
}

// noSelfPrompt are the commands after which rhythm never offers to
// update itself: update does it already, uninstall may have removed
// rhythm, version and completion are read by scripts and shells and
// config may be turning the prompt off.
var noSelfPrompt = map[string]bool{"update": true, "uninstall": true, "version": true, "completion": true, "config": true}

// selfCheck starts the background release check for rhythm itself
// before command c runs. It returns nil when rhythm shouldn't offer an
// update after c: a hidden command, one in noSelfPrompt, --offline, or
// streams that aren't the process's own (tests) or aren't a terminal.
func selfCheck(c command, args []string, stdout, stderr io.Writer) *update.Pending {
	if c.hidden || noSelfPrompt[c.name] || cli.HasFlag(args, "offline") ||
		stdout != os.Stdout || stderr != os.Stderr || !isTerminal(stderr) {
		return nil
	}
	return update.StartCheck(context.Background(), updater(selfRepo, selfName, version))
}

// selfInstall is what a yes at rhythm's own prompt runs: rhythm update
// --self-only, the same verified path as typing it. Tests replace it.
var selfInstall = func(_ context.Context, out io.Writer) error {
	if code := cmdUpdate([]string{"--self-only"}, out, out); code != 0 {
		return errors.New("rhythm update --self-only failed")
	}
	return nil
}

// selfPrompt offers to update rhythm at the end of one of its own
// commands, when selfCheck found a newer release.
func selfPrompt(check *update.Pending, args []string, stdout, stderr io.Writer) {
	if check == nil {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	update.PromptAtExit(ctx, update.PromptOptions{
		Check:   check,
		Name:    "rhythm",
		Tool:    selfName,
		Quiet:   cli.QuietRun(args),
		Stdout:  stdout,
		Out:     stderr,
		Install: selfInstall,
	})
}
