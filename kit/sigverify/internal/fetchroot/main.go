// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Command fetchroot downloads the current public-good Sigstore trusted root
// through TUF, for go generate in kit/sigverify. TUF checks the metadata
// signatures from the root of trust shipped with sigstore-go, so the file
// it writes is the one Sigstore's root keys sign.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/tuf"
)

func main() {
	out := flag.String("o", "trusted_root.json", "file to write")
	flag.Parse()
	opts := tuf.DefaultOptions()
	opts.DisableLocalCache = true
	c, err := tuf.New(opts)
	if err == nil {
		var b []byte
		if b, err = c.GetTarget("trusted_root.json"); err == nil {
			err = os.WriteFile(*out, b, 0o600)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetchroot:", err)
		os.Exit(1)
	}
}
