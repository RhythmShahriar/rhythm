// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package kit is the shared base every Rhythm tool imports. This package
// holds the banner and the kit's own version; the rest lives in
// subpackages:
//
//   - ui: the theme, severity colors and the regression palette
//   - config: the per-user config and cache folders and first-run markers
//   - update: release lookup, verified self-update and the update notice
//   - sigverify: signature checks for release checksums
//   - cache: a disk cache with a TTL and a stale fallback
//   - crawl: same-site crawling with robots.txt, sitemaps and page groups
//   - ratelimit: per-host request pacing that slows down on 429 and 503
//   - httpx: HTTP clients that keep headers and cookies on the scanned site
//   - secrets: finding and masking credentials in anything a tool prints
//   - cli: flags, help, exit codes, did-you-mean and the tool skeleton
//
// Release order: tag the kit first, then bump the kit in each tool and
// release them, then release rhythm. Tools pin a released kit in go.mod,
// so a tool can only use kit code that has been tagged. rhythm ships
// last so its installers never point at tool releases that don't exist.
package kit

import (
	"fmt"
	"runtime/debug"
)

// Module is the module path of the kit (and of rhythm, which shares it).
const Module = "github.com/RhythmShahriar/rhythm"

// Banner returns the standard header line for a tool.
func Banner(tool, version string) string {
	return fmt.Sprintf("♪ %s  %s", tool, version)
}

// Version returns the version of the kit the running program was built
// with, such as "v0.6.0": the kit dependency's version in a tool, the
// main module's version in rhythm itself and "(devel)" when the build
// carries no version (go run, a local build, or a test).
func Version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return devel
	}
	return versionFrom(bi)
}

// devel is what Go records as the version of a module built from a
// working tree.
const devel = "(devel)"

// versionFrom finds the kit's version in bi. A replace directive (a tool
// built against a local kit checkout) wins, because that is the code that
// was compiled in.
func versionFrom(bi *debug.BuildInfo) string {
	for _, m := range bi.Deps {
		if m.Path != Module {
			continue
		}
		if m.Replace != nil {
			m = m.Replace
		}
		if m.Version != "" {
			return m.Version
		}
		return devel
	}
	if bi.Main.Path == Module && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return devel
}
