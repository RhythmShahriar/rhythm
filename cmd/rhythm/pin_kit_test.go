// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

// kitTool builds testdata/kittool, a real kit/cli.Main tool, so the
// data pinning is checked against what the kit answers, not a stub.
func kitTool(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), exe("rhythm-tuner"))
	build := exec.Command("go", "build", "-o", bin, "./testdata/kittool")
	build.Env = startEnv
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building kittool: %v\n%s", err, out)
	}
	return bin
}

func TestRefreshDataPinsThroughKit(t *testing.T) {
	bin := kitTool(t)
	tgt := target{tool: "tuner", binary: "rhythm-tuner", path: bin, current: "0.3.1", kitCLI: true}
	pins := []dataPack{{Name: "checks", Version: "2026.10.20"}}
	refresh := func(pins []dataPack) ([]dataPack, string, string, error) {
		var out, errs bytes.Buffer
		packs, err := refreshData(context.Background(), tgt, pins, &out, &errs)
		return packs, out.String(), errs.String(), err
	}

	t.Run("kit without UpdatePinned", func(t *testing.T) {
		t.Setenv("KITTOOL_PIN", "")
		packs, out, errs, err := refresh(pins)
		if err != nil || len(packs) != 1 || packs[0].Version != "2026.10.24" {
			t.Fatalf("packs %v, err %v\n%s%s", packs, err, out, errs)
		}
		// Only the router's warning and the plain refresh's progress:
		// nothing of the kit's usage error.
		want := "rhythm-tuner can't pin its data; refreshing it to the latest instead\nfetching the latest checks\n"
		if errs != want {
			t.Errorf("stderr:\n%s\nwant:\n%s", errs, want)
		}
		if out != "rhythm-tuner 0.3.1  data: checks 2026.10.24  ok\n" {
			t.Errorf("stdout: %q", out)
		}
	})

	t.Run("kit with UpdatePinned", func(t *testing.T) {
		t.Setenv("KITTOOL_PIN", "1")
		packs, out, errs, err := refresh(pins)
		if err != nil || len(packs) != 1 || packs[0].Version != "2026.10.20" {
			t.Fatalf("packs %v, err %v\n%s%s", packs, err, out, errs)
		}
		if errs != "fetching checks 2026.10.20\n" || out != "rhythm-tuner 0.3.1  data: checks 2026.10.20  ok\n" {
			t.Errorf("output:\n%s%s", out, errs)
		}
	})

	t.Run("pinned version not available", func(t *testing.T) {
		t.Setenv("KITTOOL_PIN", "1")
		_, out, errs, err := refresh([]dataPack{{Name: "checks", Version: "missing"}})
		if !errors.Is(err, errDataFailed) || out != "" {
			t.Fatalf("err %v\n%s%s", err, out, errs)
		}
		want := "fetching checks missing\n" +
			"rhythm-tuner: update failed: checks missing isn't available (got 2026.10.24)\n" +
			"rhythm-tuner  data refresh failed (exit 3)\n"
		if errs != want {
			t.Errorf("stderr:\n%s\nwant:\n%s", errs, want)
		}
	})
}
