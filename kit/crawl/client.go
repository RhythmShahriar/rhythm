// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/httpx"
)

// defaultTimeout bounds each request made by the default client. Tests
// shorten it.
var defaultTimeout = 30 * time.Second

// maxRedirects is the most redirect hops followed for one request.
const maxRedirects = httpx.MaxRedirects

// errRedirectRefused is wrapped by the errors sameSiteRedirect returns
// for its own refusals (off the site, too many hops), so robots can tell
// a redirect the crawler chose not to follow from a server that failed.
var errRedirectRefused = errors.New("redirect refused")

// crawlState rides on each request's context, so the client shared by
// every crawl knows which site a request (and its redirects) belongs to.
type crawlState struct {
	start *url.URL
	front *Frontier
	// guard decides which addresses the default client may dial for
	// this crawl, from what the start host resolves to.
	guard *httpx.Guard
}

// stateKey is the context key for crawlState. An unexported type keeps
// any other package from reading or replacing the state.
type stateKey struct{}

func stateFrom(ctx context.Context) *crawlState {
	st, _ := ctx.Value(stateKey{}).(*crawlState)
	return st
}

// newClient returns the client a crawl uses. A caller's client is copied
// (never changed) and gets the same-site redirect rule after its own.
// Without one, the crawler builds a client with a timeout that also
// refuses private addresses unless opts.AllowPrivate is set.
func (c *Crawler) newClient(opts Options) *http.Client {
	var cl http.Client
	if opts.HTTP != nil {
		cl = *opts.HTTP
	} else {
		tr := &http.Transport{}
		if dt, ok := http.DefaultTransport.(*http.Transport); ok {
			tr = dt.Clone()
		}
		if !opts.AllowPrivate {
			tr.Proxy = recordProxy
			tr.DialContext = dial
		}
		cl = http.Client{Transport: tr, Timeout: defaultTimeout}
	}
	cl.CheckRedirect = c.sameSiteRedirect(cl.CheckRedirect)
	return &cl
}

// sameSiteRedirect runs the caller's redirect policy, if any, then
// refuses hops off the start site and stops after maxRedirects. Each
// followed hop waits its turn on the host's limiter like any request.
func (c *Crawler) sameSiteRedirect(prev func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if prev != nil {
			if err := prev(req, via); err != nil {
				return err
			}
		}
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects: %w", maxRedirects, errRedirectRefused)
		}
		start := via[0].URL
		st := stateFrom(req.Context())
		if st != nil {
			start = st.start
		}
		if !SameSite(start, req.URL) {
			if st != nil && st.front != nil {
				st.front.skip("offsite-redirect")
			}
			return fmt.Errorf("not following a redirect off the site to %s: %w", req.URL.Redacted(), errRedirectRefused)
		}
		return c.hosts.For(req.URL.Host).Wait(req.Context())
	}
}

// recordProxy is the crawl's Guard.Proxy: http.ProxyFromEnvironment
// that also notes the proxy's address, so dial lets it through.
func recordProxy(req *http.Request) (*url.URL, error) {
	return stateFrom(req.Context()).guardOrNil().Proxy(req)
}

// dial connects through the crawl's Guard, so a loopback, link-local or
// private address is refused unless the start host is on the same kind.
// Without crawl state only public addresses are allowed.
func dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return stateFrom(ctx).guardOrNil().DialContext(ctx, network, addr)
}

// guardOrNil is the crawl's guard, or nil (public addresses only) when
// there is no crawl state.
func (st *crawlState) guardOrNil() *httpx.Guard {
	if st == nil {
		return nil
	}
	return st.guard
}
