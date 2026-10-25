// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Command kittool is a tool built on kit/cli.Main for the router's
// tests. Its data is checks 2026.10.24; with KITTOOL_PIN set it also
// takes update --pin and can get any checks version but "missing".
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/cli"
)

// packs is the data report for the checks pack at version.
func packs(version string) []cli.DataPack {
	return []cli.DataPack{{Name: "checks", Version: version, SHA256: strings.Repeat("b", 64)}}
}

func main() {
	tool := cli.Tool{
		Name:    "rhythm-tuner",
		Version: "0.3.1",
		Update: func(_ context.Context, w io.Writer) ([]cli.DataPack, error) {
			fmt.Fprintln(w, "fetching the latest checks")
			return packs("2026.10.24"), nil
		},
	}
	if os.Getenv("KITTOOL_PIN") != "" {
		tool.UpdatePinned = func(_ context.Context, w io.Writer, pins map[string]string) ([]cli.DataPack, error) {
			v := pins["checks"]
			fmt.Fprintln(w, "fetching checks", v)
			if v == "missing" {
				return packs("2026.10.24"), nil
			}
			return packs(v), nil
		}
	}
	os.Exit(cli.Main(tool))
}
