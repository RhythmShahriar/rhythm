// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package httpx_test

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/httpx"
)

func ExampleNew() {
	target, _, err := httpx.ParseTarget("example.com")
	if err != nil {
		fmt.Println(err)
		return
	}
	var log strings.Builder
	headers := []string{"Authorization: Bearer my-secret-token"}
	if _, err := httpx.ParseHeaders(headers); err != nil {
		fmt.Println(err) // reject a bad -H flag before sending anything
		return
	}
	c := httpx.New(httpx.Options{
		UserAgent: "rhythm-example/1.0",
		Rate:      2,
		Headers:   headers,
		Target:    target,
		Retries:   2,
		Debug:     &log,
		// A canned answer instead of the network, for this example.
		Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Status: "200 OK", Request: r,
				Header: http.Header{}, Body: io.NopCloser(strings.NewReader("hello"))}, nil
		}),
	})
	resp, err := c.Get("https://example.com/")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	// Durations vary, so the example hides them.
	fmt.Print(regexp.MustCompile(`\(\S+\)\n`).ReplaceAllString(log.String(), "(time)\n"))
	fmt.Println(string(body))
	// Output:
	// > GET https://example.com/
	// >   Authorization: •••••
	// >   User-Agent: rhythm-example/1.0
	// < 200 OK GET https://example.com/ (time)
	// hello
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
