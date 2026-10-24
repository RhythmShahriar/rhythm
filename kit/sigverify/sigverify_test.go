// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package sigverify

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"google.golang.org/protobuf/encoding/protojson"
)

// The fixtures are the real checksums.txt and checksums.txt.sigstore.json
// published with RhythmShahriar/rhythm v0.1.0 (a cosign v2 bundle). The
// cosign command that accepts them, with the same pins as this package:
//
//	cosign verify-blob --bundle checksums.txt.sigstore.json \
//	  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
//	  --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
//	  --certificate-github-workflow-repository RhythmShahriar/rhythm \
//	  --certificate-github-workflow-ref refs/tags/v0.1.0 \
//	  checksums.txt
const (
	fixRepo    = "RhythmShahriar/rhythm"
	fixVersion = "0.1.0"
	fixSigner  = Workflow + "@" + MasterRef
)

func fixtures(t testing.TB) (checksums, bundle []byte) {
	t.Helper()
	var err error
	if checksums, err = os.ReadFile("testdata/rhythm_v0.1.0_checksums.txt"); err != nil {
		t.Fatal(err)
	}
	if bundle, err = os.ReadFile("testdata/rhythm_v0.1.0_checksums.txt.sigstore.json"); err != nil {
		t.Fatal(err)
	}
	return checksums, bundle
}

func offline(t *testing.T, opts Options) *Verifier {
	t.Helper()
	opts.Offline = true
	v, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRealBundleVerifies(t *testing.T) {
	sums, b := fixtures(t)
	v := offline(t, Options{OnFallback: func(err error) {
		t.Errorf("OnFallback(%v) called with Offline set", err)
	}})
	if got := v.RootSource(); got != "" {
		t.Errorf("RootSource before the first verify = %q, want empty", got)
	}
	for _, version := range []string{fixVersion, "v" + fixVersion} {
		signer, err := v.VerifyChecksums(context.Background(), fixRepo, version, sums, b)
		if err != nil {
			t.Fatalf("version %s: %v", version, err)
		}
		if signer != fixSigner {
			t.Errorf("signer = %q, want %q", signer, fixSigner)
		}
	}
	if got := v.RootSource(); got != RootEmbedded {
		t.Errorf("RootSource = %q, want %q", got, RootEmbedded)
	}
}

// The same signature as a v0.1 protobuf bundle, the shape cosign v3 and
// sigstore-go read natively, so both parsing paths see real data.
func TestRealBundleAsProtobuf(t *testing.T) {
	sums, legacy := fixtures(t)
	pb, err := convertLegacy(legacy)
	if err != nil {
		t.Fatal(err)
	}
	b, err := protojson.Marshal(pb)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := offline(t, Options{}).VerifyChecksums(context.Background(), fixRepo, fixVersion, sums, b)
	if err != nil || signer != fixSigner {
		t.Fatalf("signer %q, err %v", signer, err)
	}
}

func TestRealBundleRejected(t *testing.T) {
	sums, b := fixtures(t)
	tampered := append([]byte(nil), sums...)
	tampered[0] ^= 1
	cases := []struct {
		name, repo, version string
		sums                []byte
	}{
		{"other repository", "RhythmShahriar/tuner", fixVersion, sums},
		{"other version", fixRepo, "9.9.9", sums},
		{"modified checksums", fixRepo, fixVersion, tampered},
		{"case of repository", "rhythmshahriar/rhythm", fixVersion, sums},
	}
	v := offline(t, Options{})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			signer, err := v.VerifyChecksums(context.Background(), c.repo, c.version, c.sums, b)
			if !errors.Is(err, ErrSignature) {
				t.Fatalf("err = %v, want ErrSignature", err)
			}
			if signer != "" {
				t.Errorf("signer = %q on failure", signer)
			}
		})
	}
}

func TestMalformedInput(t *testing.T) {
	sums, b := fixtures(t)
	v := offline(t, Options{})
	ctx := context.Background()
	for _, repo := range []string{"", "rhythm", "https://github.com/RhythmShahriar/rhythm", "a/b/c"} {
		if _, err := v.VerifyChecksums(ctx, repo, fixVersion, sums, b); err == nil || errors.Is(err, ErrSignature) {
			t.Errorf("repo %q: err = %v, want a usage error", repo, err)
		}
	}
	if _, err := v.VerifyChecksums(ctx, fixRepo, "v", sums, b); err == nil {
		t.Error("empty version accepted")
	}
	var legacy map[string]any
	if err := json.Unmarshal(b, &legacy); err != nil {
		t.Fatal(err)
	}
	delete(legacy, "cert")
	noCert, _ := json.Marshal(legacy)
	for name, bad := range map[string][]byte{
		"empty":     nil,
		"not JSON":  []byte("<html>"),
		"unknown":   []byte(`{"hello":"world"}`),
		"no cert":   noCert,
		"bad proto": []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`),
	} {
		if _, err := v.VerifyChecksums(ctx, fixRepo, fixVersion, sums, bad); !errors.Is(err, ErrSignature) {
			t.Errorf("%s: err = %v, want ErrSignature", name, err)
		}
	}
}

// A bundle from a different Sigstore instance does not chain to the
// public-good root.
func TestUntrustedRoot(t *testing.T) {
	sums, b := fixtures(t)
	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatal(err)
	}
	v := offline(t, Options{})
	v.material = vs
	if _, err := v.VerifyChecksums(context.Background(), fixRepo, fixVersion, sums, b); !errors.Is(err, ErrSignature) {
		t.Fatalf("err = %v, want ErrSignature", err)
	}
}

// A bundle that is valid for its own trusted root but was signed by some
// other workflow identity is rejected on identity, not on trust.
func TestDifferentIdentity(t *testing.T) {
	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatal(err)
	}
	sums := []byte("abc  rhythm_linux_amd64.tar.gz\n")
	for _, signer := range []string{
		"https://github.com/someone/.github/.github/workflows/release-go.yml@refs/heads/master",
		Workflow + "@refs/heads/dev",
		"me@example.com",
	} {
		entity, err := vs.Sign(signer, Issuer, sums)
		if err != nil {
			t.Fatal(err)
		}
		v := offline(t, Options{})
		v.material = vs
		v.scts = 0 // the virtual CA embeds no SCTs
		// Control: the entity is valid for the virtual root when no
		// identity is required, so the rejection below is the policy's.
		sv, err := v.verifier(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sv.Verify(entity, verify.NewPolicy(verify.WithArtifact(strings.NewReader(string(sums))), verify.WithoutIdentitiesUnsafe())); err != nil {
			t.Fatalf("control: %v", err)
		}
		_, err = v.verifyEntity(context.Background(), entity, fixRepo, fixVersion, sums)
		var mismatch *verify.ErrNoMatchingCertificateIdentity
		if !errors.Is(err, ErrSignature) || !errors.As(err, &mismatch) {
			t.Errorf("%s: err = %v, want an identity mismatch", signer, err)
		}
	}
}

// The identity policy, checked against certificate summaries: master is
// always allowed, other workflow refs only when listed and the repository
// and tag extensions must match exactly.
func TestIdentityPolicy(t *testing.T) {
	cert := func(ref, repo, tag string) certificate.Summary {
		return certificate.Summary{
			SubjectAlternativeName: Workflow + "@" + ref,
			Extensions: certificate.Extensions{
				Issuer:              Issuer,
				SourceRepositoryURI: "https://github.com/" + repo,
				SourceRepositoryRef: tag,
			},
		}
	}
	good := cert(MasterRef, fixRepo, "refs/tags/v0.1.0")
	other := cert("refs/tags/v2", fixRepo, "refs/tags/v0.1.0")
	cases := []struct {
		name string
		refs []string
		c    certificate.Summary
		ok   bool
	}{
		{"master", nil, good, true},
		{"unlisted tag", nil, other, false},
		{"listed bare tag", []string{"v2"}, other, true},
		{"listed full ref", []string{"refs/tags/v2"}, other, true},
		{"listed other tag", []string{"v3"}, other, false},
		{"suffix after master", nil, cert(MasterRef+"x", fixRepo, "refs/tags/v0.1.0"), false},
		{"regex in WorkflowRefs", []string{"v.*"}, cert("refs/tags/v2", fixRepo, "refs/tags/v0.1.0"), false},
		{"other repository", nil, cert(MasterRef, "RhythmShahriar/tuner", "refs/tags/v0.1.0"), false},
		{"other tag", nil, cert(MasterRef, fixRepo, "refs/tags/v0.2.0"), false},
		{"branch build", nil, cert(MasterRef, fixRepo, "refs/heads/master"), false},
		{"other issuer", nil, func() certificate.Summary {
			c := good
			c.Issuer = "https://accounts.google.com"
			return c
		}(), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := New(Options{WorkflowRefs: c.refs})
			if err != nil {
				t.Fatal(err)
			}
			id, err := v.identity(fixRepo, fixVersion)
			if err != nil {
				t.Fatal(err)
			}
			if err := id.Verify(c.c); (err == nil) != c.ok {
				t.Errorf("Verify = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

// When the TUF mirror can't be reached the embedded root is used and
// verification still works.
func TestTUFFallback(t *testing.T) {
	sums, b := fixtures(t)
	ln, err := net.Listen("tcp", "127.0.0.1:20040")
	if err != nil {
		t.Skip("port 20040 busy:", err)
	}
	var hits atomic.Int32
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			http.NotFound(w, nil)
		}),
		ReadHeaderTimeout: time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	cache := filepath.Join(t.TempDir(), "tuf")
	var fallbacks []error
	v, err := New(Options{
		Mirror:     "http://127.0.0.1:20040",
		CacheDir:   cache,
		OnFallback: func(err error) { fallbacks = append(fallbacks, err) },
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := v.VerifyChecksums(context.Background(), fixRepo, fixVersion, sums, b); err != nil {
			t.Fatal(err)
		}
	}
	if got := v.RootSource(); got != RootEmbedded {
		t.Errorf("RootSource = %q, want %q", got, RootEmbedded)
	}
	if len(fallbacks) != 1 || fallbacks[0] == nil {
		t.Errorf("OnFallback calls = %v, want one with the TUF error", fallbacks)
	}
	if hits.Load() == 0 {
		t.Error("the mirror was never asked")
	}
}

func TestEmbeddedRootParses(t *testing.T) {
	tm, from, err := loadRoot(context.Background(), Options{Offline: true})
	if err != nil || from != RootEmbedded {
		t.Fatalf("from %q, err %v", from, err)
	}
	if len(tm.FulcioCertificateAuthorities()) == 0 || len(tm.RekorLogs()) == 0 {
		t.Fatal("embedded root has no Fulcio CA or Rekor log")
	}
}
