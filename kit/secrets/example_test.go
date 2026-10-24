// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package secrets_test

import (
	"fmt"

	"github.com/RhythmShahriar/rhythm/kit/secrets"
)

func ExampleRedact() {
	fmt.Println(secrets.Redact("DB_PASSWORD=hunter2-but-longer"))
	fmt.Println(secrets.Redact("curl https://admin:s3cr3t@example.com/"))
	fmt.Println(secrets.Redact("nothing to hide here"))
	// Output:
	// DB_PASSWORD=hu•••••
	// curl https://admin:s3•••••@example.com/
	// nothing to hide here
}

func ExampleRedactHeader() {
	fmt.Println(secrets.RedactHeader("Authorization", "Bearer abc.def.ghi"))
	fmt.Println(secrets.RedactHeader("Cookie", "session=abc123; theme=dark"))
	fmt.Println(secrets.RedactHeader("Accept", "text/html"))
	// Output:
	// •••••
	// session=•••••; theme=•••••
	// text/html
}

func ExampleFindSecrets() {
	for _, s := range secrets.FindSecrets([]byte("API_KEY=sk_live_0123456789abcdefABCDEF\n")) {
		fmt.Println(s.Kind, s)
	}
	// Output:
	// stripe-key sk_l•••••
}
