// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package kit

import (
	"fmt"
	"runtime/debug"
	"testing"
)

func TestBanner(t *testing.T) {
	if got := Banner("rhythm-tuner", "0.3.0"); got != "♪ rhythm-tuner  0.3.0" {
		t.Errorf("Banner = %q", got)
	}
}

func ExampleBanner() {
	fmt.Println(Banner("rhythm-tuner", "0.3.0"))
	// Output: ♪ rhythm-tuner  0.3.0
}

func TestVersion(t *testing.T) {
	// A test binary is built from the working tree, so it has no version.
	if got := Version(); got != devel {
		t.Errorf("Version() in a test = %q, want %q", got, devel)
	}
}

func TestVersionFrom(t *testing.T) {
	kitDep := func(v string) *debug.Module { return &debug.Module{Path: Module, Version: v} }
	cases := []struct {
		name string
		bi   debug.BuildInfo
		want string
	}{
		{"tool importing a released kit",
			debug.BuildInfo{Main: debug.Module{Path: "github.com/RhythmShahriar/tuner", Version: "v0.3.0"},
				Deps: []*debug.Module{{Path: "golang.org/x/net", Version: "v0.61.0"}, kitDep("v0.6.0")}},
			"v0.6.0"},
		{"tool built against a local kit",
			debug.BuildInfo{Main: debug.Module{Path: "github.com/RhythmShahriar/tuner"},
				Deps: []*debug.Module{{Path: Module, Version: "v0.6.0", Replace: &debug.Module{Path: "../rhythm"}}}},
			devel},
		{"tool with a kit replaced by another version",
			debug.BuildInfo{Deps: []*debug.Module{{Path: Module, Version: "v0.5.0", Replace: kitDep("v0.6.1")}}},
			"v0.6.1"},
		{"rhythm installed with go install",
			debug.BuildInfo{Main: debug.Module{Path: Module, Version: "v0.6.0"}},
			"v0.6.0"},
		{"rhythm built from a checkout",
			debug.BuildInfo{Main: debug.Module{Path: Module, Version: devel}},
			devel},
		{"another program, no kit",
			debug.BuildInfo{Main: debug.Module{Path: "example.com/other", Version: "v1.0.0"}},
			devel},
		{"empty build info", debug.BuildInfo{}, devel},
	}
	for _, c := range cases {
		if got := versionFrom(&c.bi); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
