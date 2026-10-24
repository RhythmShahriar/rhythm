// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"fmt"
)

// bundleAsset is the Sigstore bundle GoReleaser publishes next to
// checksums.txt.
const bundleAsset = "checksums.txt.sigstore.json"

// Verifier checks a Sigstore bundle over a release's checksums.txt and
// returns the identity that signed it, such as the release workflow's
// URL. repo is the Updater's Repo and version the release without the
// leading v. It uses only standard types, so an implementation (kit's
// sigverify) need not import this package.
type Verifier interface {
	VerifyChecksums(ctx context.Context, repo, version string, checksums, bundle []byte) (signer string, err error)
}

// ErrSignature means the signature over checksums.txt is missing or
// doesn't verify, so nothing was installed. Cause says why.
type ErrSignature struct {
	Repo, Version string
	Cause         error
}

// Error says what was refused and why.
func (e *ErrSignature) Error() string {
	return fmt.Sprintf("refusing to install %s %s: signature check failed: %v", e.Repo, e.Version, e.Cause)
}

// Unwrap returns the cause, for errors.Is and errors.As.
func (e *ErrSignature) Unwrap() error { return e.Cause }

// verifySignature runs u.Verifier over sums and the release's bundle and
// returns the signer. With no Verifier, or SkipSignature, it does nothing.
// Every failure, a missing bundle included, is an ErrSignature: the check
// fails closed.
func (u *Updater) verifySignature(ctx context.Context, rel *Release, sums []byte) (string, error) {
	if u.Verifier == nil || u.SkipSignature {
		return "", nil
	}
	fail := func(err error) (string, error) {
		return "", &ErrSignature{Repo: u.Repo, Version: rel.Version, Cause: err}
	}
	if _, ok := rel.Assets[bundleAsset]; !ok {
		return fail(errors.New("the release has no " + bundleAsset))
	}
	bundle, err := u.download(ctx, rel, bundleAsset, 1<<20)
	if err != nil {
		return fail(fmt.Errorf("downloading %s: %w", bundleAsset, err))
	}
	signer, err := u.Verifier.VerifyChecksums(ctx, u.Repo, rel.Version, sums, bundle)
	if err != nil {
		return fail(err)
	}
	return signer, nil
}
