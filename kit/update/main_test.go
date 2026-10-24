// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"os"
	"testing"
)

// TestMain clears the variables users are told to set, so the tests give
// the same results with RHYTHM_OFFLINE, a mirror, a token or a minimum
// release age in the environment.
func TestMain(m *testing.M) {
	for _, v := range []string{"RHYTHM_OFFLINE", "RHYTHM_GITHUB_API", "RHYTHM_GITHUB_TOKEN", "GITHUB_TOKEN",
		"RHYTHM_MIN_RELEASE_AGE", "RHYTHM_DOWNLOAD_CACHE"} {
		_ = os.Unsetenv(v)
	}
	os.Exit(m.Run())
}
