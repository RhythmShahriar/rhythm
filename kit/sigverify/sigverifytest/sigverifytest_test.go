// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package sigverifytest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/kit/sigverify"
)

const sums = "0123  rhythm-zap_linux_amd64.tar.gz\n"

func newCA(t *testing.T) *CA {
	t.Helper()
	ca, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return ca
}

func verifier(t *testing.T, opts sigverify.Options) *sigverify.Verifier {
	t.Helper()
	v, err := sigverify.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSharedWorkflowVerifies(t *testing.T) {
	ca := newCA(t)
	b, err := ca.Sign(Shared("RhythmShahriar/tuner", "0.3.2"), []byte(sums))
	if err != nil {
		t.Fatal(err)
	}
	v := verifier(t, ca.Options())
	signer, err := v.VerifyChecksums(context.Background(), "RhythmShahriar/tuner", "0.3.2", []byte(sums), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := sigverify.Workflow + "@" + sigverify.MasterRef; signer != want {
		t.Errorf("signer %q, want %q", signer, want)
	}
	if v.RootSource() != sigverify.RootCustom {
		t.Errorf("RootSource %q", v.RootSource())
	}
	// The same bundle fails for anything it wasn't made for.
	for name, c := range map[string]struct{ repo, version, sums string }{
		"other repository": {"RhythmShahriar/echo", "0.3.2", sums},
		"other version":    {"RhythmShahriar/tuner", "0.3.3", sums},
		"other checksums":  {"RhythmShahriar/tuner", "0.3.2", sums + "x"},
	} {
		if _, err := v.VerifyChecksums(context.Background(), c.repo, c.version, []byte(c.sums), b); !errors.Is(err, sigverify.ErrSignature) {
			t.Errorf("%s: err = %v, want ErrSignature", name, err)
		}
	}
}

// Another CA's bundle doesn't chain to this one. The public-good root
// trusts neither.
func TestOtherRootsRefuse(t *testing.T) {
	ca, other := newCA(t), newCA(t)
	b, err := other.Sign(Shared("RhythmShahriar/tuner", "0.3.2"), []byte(sums))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier(t, ca.Options()).VerifyChecksums(context.Background(), "RhythmShahriar/tuner", "0.3.2", []byte(sums), b); !errors.Is(err, sigverify.ErrSignature) {
		t.Errorf("other CA: err = %v", err)
	}
	if _, err := verifier(t, sigverify.Options{Offline: true}).VerifyChecksums(context.Background(), "RhythmShahriar/tuner", "0.3.2", []byte(sums), b); !errors.Is(err, sigverify.ErrSignature) {
		t.Errorf("public-good root: err = %v", err)
	}
}

func TestOwnWorkflow(t *testing.T) {
	ca := newCA(t)
	const repo = "someone/rhythm-zap"
	own := Identity{SAN: "https://github.com/someone/rhythm-zap/.github/workflows/release.yml@refs/tags/v1.0.0", Repo: repo, Ref: "refs/tags/v1.0.0"}
	cases := []struct {
		name string
		id   Identity
		own  bool // OwnWorkflow set
		ok   bool
	}{
		{"own workflow, allowed", own, true, true},
		{"own workflow, not allowed", own, false, false},
		{"shared workflow", Shared(repo, "1.0.0"), true, true},
		{"another repository's workflow", Identity{SAN: "https://github.com/mallory/rhythm-zap/.github/workflows/release.yml@refs/tags/v1.0.0", Repo: repo, Ref: "refs/tags/v1.0.0"}, true, false},
		{"own workflow for another source", Identity{SAN: own.SAN, Repo: "mallory/rhythm-zap", Ref: "refs/tags/v1.0.0"}, true, false},
		{"prefix of another repository", Identity{SAN: "https://github.com/someone/rhythm-zapper/.github/workflows/release.yml@refs/tags/v1.0.0", Repo: repo, Ref: "refs/tags/v1.0.0"}, true, false},
		{"branch build", Identity{SAN: own.SAN, Repo: repo, Ref: "refs/heads/main"}, true, false},
		{"other issuer", Identity{SAN: own.SAN, Issuer: "https://accounts.google.com", Repo: repo, Ref: "refs/tags/v1.0.0"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := ca.Sign(c.id, []byte(sums))
			if err != nil {
				t.Fatal(err)
			}
			opts := ca.Options()
			opts.OwnWorkflow = c.own
			signer, err := verifier(t, opts).VerifyChecksums(context.Background(), repo, "1.0.0", []byte(sums), b)
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok=%v", err, c.ok)
			}
			if c.ok && signer != c.id.SAN {
				t.Errorf("signer %q, want %q", signer, c.id.SAN)
			}
			if !c.ok && !errors.Is(err, sigverify.ErrSignature) {
				t.Errorf("err = %v, want ErrSignature", err)
			}
		})
	}
}

func TestBundleIsProtobufJSON(t *testing.T) {
	b, err := newCA(t).Sign(Shared("RhythmShahriar/tuner", "0.3.2"), []byte(sums))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"mediaType":"application/vnd.dev.sigstore.bundle+json;version=0.1"`) {
		t.Errorf("bundle: %s", b)
	}
}
