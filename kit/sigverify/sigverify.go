// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package sigverify checks the keyless Sigstore signature on a release's
// checksums.txt (the checksums.txt.sigstore.json asset), so an installer
// knows the checksums came from the release workflow and not just from
// whoever can upload to the release page.
//
// A bundle passes only when all of these hold:
//   - the certificate chains to the Sigstore public-good Fulcio CA and the
//     signature is in the Rekor transparency log;
//   - the OIDC issuer is GitHub Actions;
//   - the signer (SAN) is the shared reusable workflow
//     RhythmShahriar/.github/.github/workflows/release-go.yml at
//     refs/heads/master or a ref in Options.WorkflowRefs;
//   - the Source Repository URI extension names the calling repository and
//     the Source Repository Ref is the release tag. The SAN alone names a
//     public reusable workflow that any repository can call, so these two
//     checks are what tie the signature to one tool and one release;
//   - the signed digest is sha256 of the checksums bytes.
//
// It reads both the Sigstore protobuf bundle that cosign v3 writes and the
// older cosign v2 bundle that earlier releases carry.
//
// The trusted root is embedded at build time (go generate refreshes it).
// Unless Options.Offline is set, the first verification refreshes it
// through Sigstore's TUF repository into the rhythm cache and falls back to
// the embedded copy when that fails.
//
// Verifier satisfies kit/update.Verifier without importing it.
package sigverify

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

//go:generate go run ./internal/fetchroot -o trusted_root.json

const (
	// Issuer is the OIDC issuer of GitHub Actions tokens.
	Issuer = "https://token.actions.githubusercontent.com"
	// Workflow is the reusable release workflow every Rhythm tool calls.
	// Fulcio puts it in the certificate SAN followed by "@<ref>".
	Workflow = "https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml"
	// MasterRef is the workflow ref that is always accepted.
	MasterRef = "refs/heads/master"
)

// Options configures a Verifier. The zero value verifies against the
// public-good Sigstore instance, refreshing the trusted root when online.
type Options struct {
	// WorkflowRefs are extra refs of the release workflow to accept besides
	// refs/heads/master, for when callers move to a tagged workflow. A bare
	// name like "v1" means refs/tags/v1.
	WorkflowRefs []string
	// Offline skips the TUF refresh and uses the embedded trusted root.
	Offline bool
	// CacheDir holds the TUF metadata. Empty means
	// config.CacheDir("rhythm")/tuf.
	CacheDir string
	// Mirror is the TUF repository URL. Empty means Sigstore's public CDN.
	Mirror string
	// HTTPClient fetches TUF metadata. Nil means a client with a short
	// timeout, so a slow network delays an install by seconds, not minutes.
	HTTPClient *http.Client
}

// Verifier checks release signatures. It is safe for concurrent use.
type Verifier struct {
	opts Options
	san  *regexp.Regexp

	once     sync.Once
	material root.TrustedMaterial // set by tests to skip loading a root
	rootFrom string               // "tuf" or "embedded", for tests
	scts     int                  // SCTs required; tests lower it for a test CA without a CT log
	sv       *verify.Verifier
	err      error
}

// ErrSignature wraps every reason a bundle is rejected, so callers can tell
// a bad signature from a usage error with errors.Is.
var ErrSignature = errors.New("signature not valid")

// New returns a Verifier. It does no network I/O; the trusted root is
// loaded on the first VerifyChecksums call.
func New(opts Options) (*Verifier, error) {
	refs := []string{regexp.QuoteMeta(MasterRef)}
	for _, r := range opts.WorkflowRefs {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if !strings.HasPrefix(r, "refs/") {
			r = "refs/tags/" + r
		}
		refs = append(refs, regexp.QuoteMeta(r))
	}
	san, err := regexp.Compile("^" + regexp.QuoteMeta(Workflow+"@") + "(" + strings.Join(refs, "|") + ")$")
	if err != nil {
		return nil, err
	}
	return &Verifier{opts: opts, san: san, scts: 1}, nil
}

// VerifyChecksums checks bundle (the contents of checksums.txt.sigstore.json)
// against checksums (checksums.txt) for release version of repo, given as
// "owner/name" exactly as on GitHub. version may carry a leading "v". It
// returns the signer, the workflow identity from the certificate.
func (v *Verifier) VerifyChecksums(ctx context.Context, repo, version string, checksums, bundle []byte) (string, error) {
	if !validRepo(repo) {
		return "", fmt.Errorf("sigverify: repository %q is not owner/name", repo)
	}
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		return "", errors.New("sigverify: no version given")
	}
	entity, err := parseBundle(bundle)
	if err != nil {
		return "", fmt.Errorf("%w: %s %s: %w", ErrSignature, repo, version, err)
	}
	return v.verifyEntity(ctx, entity, repo, version, checksums)
}

func (v *Verifier) verifyEntity(ctx context.Context, entity verify.SignedEntity, repo, version string, checksums []byte) (string, error) {
	sv, err := v.verifier(ctx)
	if err != nil {
		return "", fmt.Errorf("sigverify: loading the Sigstore trusted root: %w", err)
	}
	id, err := v.identity(repo, version)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(checksums)
	res, err := sv.Verify(entity, verify.NewPolicy(
		verify.WithArtifactDigest("sha256", digest[:]),
		verify.WithCertificateIdentity(id),
	))
	if err != nil {
		return "", fmt.Errorf("%w: %s %s: %w", ErrSignature, repo, version, err)
	}
	if res.Signature == nil || res.Signature.Certificate == nil {
		return "", fmt.Errorf("%w: %s %s: no signing certificate", ErrSignature, repo, version)
	}
	return res.Signature.Certificate.SubjectAlternativeName, nil
}

// identity is the certificate policy for one release of one repository.
func (v *Verifier) identity(repo, version string) (verify.CertificateIdentity, error) {
	san, err := verify.NewSANMatcher("", v.san.String())
	if err != nil {
		return verify.CertificateIdentity{}, err
	}
	issuer, err := verify.NewIssuerMatcher(Issuer, "")
	if err != nil {
		return verify.CertificateIdentity{}, err
	}
	return verify.NewCertificateIdentity(san, issuer, certificate.Extensions{
		SourceRepositoryURI: "https://github.com/" + repo,
		SourceRepositoryRef: "refs/tags/v" + version,
	})
}

// verifier loads the trusted root once and builds the sigstore-go verifier.
// Like cosign it requires one transparency log entry, whose integrated time
// (or a signed timestamp) proves the short-lived certificate was valid when
// the release was signed. It also requires one certificate transparency proof.
func (v *Verifier) verifier(ctx context.Context) (*verify.Verifier, error) {
	v.once.Do(func() {
		if v.material == nil {
			v.material, v.rootFrom, v.err = loadRoot(ctx, v.opts)
			if v.err != nil {
				return
			}
		}
		opts := []verify.VerifierOption{
			verify.WithTransparencyLog(1),
			verify.WithObserverTimestamps(1),
		}
		if v.scts > 0 {
			opts = append(opts, verify.WithSignedCertificateTimestamps(v.scts))
		}
		v.sv, v.err = verify.NewVerifier(v.material, opts...)
	})
	return v.sv, v.err
}

var repoName = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

func validRepo(repo string) bool {
	return repoName.MatchString(repo)
}
