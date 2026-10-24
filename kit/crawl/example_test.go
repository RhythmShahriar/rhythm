// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/crawl"
)

// exampleSite serves a tiny site from memory, so the example needs no
// network: a robots.txt that blocks /private/, a home page and two posts.
type exampleSite map[string]string

func (s exampleSite) RoundTrip(r *http.Request) (*http.Response, error) {
	body, ok := s[r.URL.Path]
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Request: r,
		Body: io.NopCloser(strings.NewReader(body))}
	switch {
	case !ok:
		resp.StatusCode = http.StatusNotFound
	case r.URL.Path == "/robots.txt":
		resp.Header.Set("Content-Type", "text/plain")
	default:
		resp.Header.Set("Content-Type", "text/html")
	}
	return resp, nil
}

func ExampleNew() {
	site := exampleSite{
		"/robots.txt": "User-agent: *\nDisallow: /private/\n",
		"/": `<a href="/blog/hello/">Hello</a> <a href="/private/">Private</a>
			<a href="/wp-login.php">Log in</a> <a href="https://other.example/">Elsewhere</a>`,
		"/blog/hello/":  `<a href="/blog/second/">Next</a>`,
		"/blog/second/": `<a href="/">Home</a>`,
	}
	c := crawl.New(crawl.Options{
		UserAgent:  "rhythm-example/1.0",
		NoSitemaps: true,
		HTTP:       &http.Client{Transport: site},
	})
	start, _ := url.Parse("https://example.com/")
	f, err := c.Crawl(context.Background(), start)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, p := range f.Pages() {
		fmt.Println(p.Source, p.URL)
	}
	fmt.Println("skipped:", f.Skipped())
	// Output:
	// start https://example.com/
	// link https://example.com/blog/hello/
	// link https://example.com/blog/second/
	// skipped: map[excluded:1 other site:1 robots.txt:1]
}
