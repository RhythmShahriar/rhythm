// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultAPI = "https://api.github.com"

// githubHosts may receive GITHUB_TOKEN: the API and the hosts release
// downloads start on. Anything else, including a mirror, gets
// RHYTHM_GITHUB_TOKEN or nothing.
var githubHosts = map[string]bool{
	"api.github.com":                true,
	"github.com":                    true,
	"objects.githubusercontent.com": true,
}

func (u *Updater) apiBase() string {
	if u.API == "" {
		return defaultAPI
	}
	return strings.TrimRight(u.API, "/")
}

// checkURL refuses anything but https, except plain http to a loopback
// host so tests can serve releases from 127.0.0.1. A release fetched over
// http could be swapped in transit together with its checksums.
func checkURL(raw string) error {
	p, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch {
	case p.Scheme == "https" && p.Host != "":
		return nil
	case p.Scheme == "http" && isLoopback(p.Hostname()):
		return nil
	case p.Scheme == "":
		return fmt.Errorf("not an https URL: %s", raw)
	}
	return fmt.Errorf("refusing to download over %s: %s", p.Scheme, raw)
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// token picks the credential for a request to target, or "" for none.
func (u *Updater) token(target *url.URL) string {
	api, err := url.Parse(u.apiBase())
	if err != nil {
		return ""
	}
	if tok := os.Getenv("RHYTHM_GITHUB_TOKEN"); tok != "" {
		if strings.EqualFold(target.Host, api.Host) {
			return tok
		}
		return ""
	}
	tok := os.Getenv("GITHUB_TOKEN")
	if tok == "" || target.Scheme != "https" || !strings.EqualFold(api.Host, "api.github.com") {
		return ""
	}
	if githubHosts[strings.ToLower(target.Host)] {
		return tok
	}
	return ""
}

// newRequest builds a GET for raw with the user agent and, when this host
// may have it, the token.
func (u *Updater) newRequest(ctx context.Context, raw string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	// Always the Updater's own agent: a tool's scan user agent would make
	// update checks look like part of a scan.
	ua := u.UserAgent
	if ua == "" {
		ua = "rhythm-update/" + u.Binary
	}
	req.Header.Set("User-Agent", ua)
	if tok := u.token(req.URL); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req, nil
}

// client returns a copy of u.HTTP (or a default client) whose redirects
// stay on https and drop the token when they leave the first host. Go
// keeps Authorization on a redirect to the same host name on another
// port, and a caller's client may not strip it at all, so this doesn't
// rely on either.
func (u *Updater) client() *http.Client {
	var c http.Client
	if u.HTTP != nil {
		c = *u.HTTP
	} else {
		c = http.Client{Timeout: 5 * time.Minute, Transport: defaultTransport()}
	}
	next := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := checkURL(req.URL.String()); err != nil {
			return err
		}
		if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			req.Header.Del("Authorization")
		}
		if next != nil {
			return next(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &c
}

// defaultTransport is Go's default transport with the proxy read from
// HTTPS_PROXY, HTTP_PROXY and NO_PROXY, stated here so it doesn't depend on
// http.DefaultTransport, which a program may have replaced.
func defaultTransport() http.RoundTripper {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	t = t.Clone()
	t.Proxy = http.ProxyFromEnvironment
	return t
}
