// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package sigverify_test

import (
	"context"
	"fmt"
	"os"

	"github.com/RhythmShahriar/rhythm/kit/sigverify"
)

// Verify the signed checksums of RhythmShahriar/rhythm v0.1.0. The cosign
// CLI equivalent is:
//
//	cosign verify-blob --bundle checksums.txt.sigstore.json \
//	  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
//	  --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
//	  --certificate-github-workflow-repository RhythmShahriar/rhythm \
//	  --certificate-github-workflow-ref refs/tags/v0.1.0 \
//	  checksums.txt
func ExampleVerifier_VerifyChecksums() {
	checksums, _ := os.ReadFile("testdata/rhythm_v0.1.0_checksums.txt")
	bundle, _ := os.ReadFile("testdata/rhythm_v0.1.0_checksums.txt.sigstore.json")

	v, err := sigverify.New(sigverify.Options{Offline: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	signer, err := v.VerifyChecksums(context.Background(), "RhythmShahriar/rhythm", "0.1.0", checksums, bundle)
	if err != nil {
		fmt.Println("refusing to install:", err)
		return
	}
	fmt.Println("signed by", signer)
	// Output: signed by https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master
}
