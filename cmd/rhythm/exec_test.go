// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build unix

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// cleanupBuilt removes the router rhythmBinary built, if it built one.
func cleanupBuilt() {
	if builtBin != "" {
		_ = os.RemoveAll(filepath.Dir(builtBin))
	}
}

// rhythmBinary builds the router once per test run, because exec only
// shows in a real process: one that replaces itself with the tool.
func rhythmBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rhythm-exec-test")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "rhythm")
		// Build with the environment the tests started with: TestMain
		// moves HOME and the config folders, which Go's caches follow.
		build := exec.Command("go", "build", "-o", builtBin, ".")
		build.Env = startEnv
		out, err := build.CombinedOutput()
		if err != nil {
			buildErr = errors.New(string(out))
		}
	})
	if buildErr != nil {
		t.Fatalf("building rhythm: %v", buildErr)
	}
	return builtBin
}

// startRhythm runs the built router on `rhythm <tool>` with dir first on
// PATH and waits until the stub has written its PID to pidFile.
func startRhythm(t *testing.T, dir, tool, pidFile string) (*exec.Cmd, int) {
	t.Helper()
	cmd := exec.Command(rhythmBinary(t), tool)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil && strings.HasSuffix(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			return cmd, pid
		}
	}
	t.Fatal("the stub never started")
	return nil, 0
}

func TestExecInterruptWaitsForCleanup(t *testing.T) {
	dir := t.TempDir()
	pidFile, outFile := filepath.Join(dir, "pid"), filepath.Join(dir, "out")
	stub(t, dir, "traps", `trap 'sleep 0.3; echo cleaned > "`+outFile+`"; exit 130' INT
echo $$ > "`+pidFile+`"
while :; do sleep 0.05; done`)

	cmd, pid := startRhythm(t, dir, "traps", pidFile)
	if pid != cmd.Process.Pid {
		t.Fatalf("tool PID %d, rhythm PID %d: rhythm didn't become the tool", pid, cmd.Process.Pid)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if code := cmd.ProcessState.ExitCode(); code != 130 {
		t.Errorf("exit %d, want 130", code)
	}
	// rhythm's process has ended, so the tool's cleanup must be done.
	if b, _ := os.ReadFile(outFile); string(b) != "cleaned\n" {
		t.Errorf("cleanup hadn't finished when rhythm exited: %q", b)
	}
}

func TestExecTermReachesTool(t *testing.T) {
	dir := t.TempDir()
	pidFile, outFile := filepath.Join(dir, "pid"), filepath.Join(dir, "out")
	stub(t, dir, "terms", `trap 'echo got TERM > "`+outFile+`"; exit 0' TERM
echo $$ > "`+pidFile+`"
while :; do sleep 0.05; done`)

	cmd, _ := startRhythm(t, dir, "terms", pidFile)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if b, _ := os.ReadFile(outFile); string(b) != "got TERM\n" {
		t.Errorf("SIGTERM to rhythm didn't reach the tool: %q", b)
	}
}

func TestToolKilledBySignal(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	stub(t, dir, "dies", `echo $$ > "`+pidFile+`"
kill -TERM $$`)

	// Exec: rhythm is the tool, so it dies of the same signal and the
	// shell above it reports 143.
	cmd, _ := startRhythm(t, dir, "dies", pidFile)
	_ = cmd.Wait()
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGTERM {
		t.Errorf("rhythm ended with %v, want killed by SIGTERM", cmd.ProcessState)
	}

	// Child runner: rhythm maps the signal to 128+15 itself.
	os.Remove(pidFile)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out, errb bytes.Buffer
	if code := run([]string{"dies"}, &out, &errb); code != 143 {
		t.Errorf("exit %d, want 143: %s", code, errb.String())
	}
}

func TestExecFailureFallsBack(t *testing.T) {
	dir := t.TempDir()
	// Executable but not a program: exec fails, then so does the child.
	os.WriteFile(filepath.Join(dir, "rhythm-tuner"), []byte("not a program\x00\x01"), 0o755)
	cmd := exec.Command(rhythmBinary(t), "tuner")
	cmd.Env = append(os.Environ(), "PATH="+dir)
	out, _ := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); code != 3 || !strings.Contains(string(out), "can't start tuner") {
		t.Errorf("exit %d: %s", code, out)
	}
}
