// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"syscall"
	"time"
)

// defaultTimeout bounds each request made by the default client. Tests
// shorten it.
var defaultTimeout = 30 * time.Second

// maxRedirects is the most redirect hops followed for one request.
const maxRedirects = 10

// errPrivate is returned when the default client refuses to connect to a
// private address.
var errPrivate = errors.New("private address")

// crawlState rides on each request's context, so the client shared by
// every crawl knows which site a request (and its redirects) belongs to.
type crawlState struct {
	start    *url.URL
	front    *Frontier
	startIPs []netip.Addr
	proxies  sync.Map // proxy host:port → true, dialed without the address check
}

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
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
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
			return fmt.Errorf("not following a redirect off the site to %s", req.URL.Redacted())
		}
		return c.hosts.For(req.URL.Host).Wait(req.Context())
	}
}

// recordProxy is http.ProxyFromEnvironment that also notes the proxy's
// address, so dial lets it through: the user configured that proxy, and
// what it connects to is its own decision.
func recordProxy(req *http.Request) (*url.URL, error) {
	p, err := http.ProxyFromEnvironment(req)
	if p != nil {
		if st := stateFrom(req.Context()); st != nil {
			st.proxies.Store(canonicalAddr(p), true)
		}
	}
	return p, err
}

// canonicalAddr is host:port for u, with the scheme's default port, the
// form the transport dials.
func canonicalAddr(u *url.URL) string {
	p := u.Port()
	if p == "" {
		p = map[string]string{"http": "80", "https": "443", "socks5": "1080", "socks5h": "1080"}[u.Scheme]
	}
	return net.JoinHostPort(u.Hostname(), p)
}

// dial connects like the default transport but checks the resolved
// address before connecting, so a hostname that resolves to a private
// address is caught too.
func dial(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	st := stateFrom(ctx)
	var startIPs []netip.Addr
	proxied := false
	if st != nil {
		startIPs = st.startIPs
		_, proxied = st.proxies.Load(addr)
	}
	if !proxied {
		d.ControlContext = func(_ context.Context, _, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if !allowAddr(startIPs, ip) {
				return fmt.Errorf("refusing to connect to %s: %w (set AllowPrivate to allow it)", ip, errPrivate)
			}
			return nil
		}
	}
	return d.DialContext(ctx, network, addr)
}

// addrClass names the kind of non-public address ip is, or "" for a
// public one.
func addrClass(ip netip.Addr) string {
	ip = ip.Unmap()
	switch {
	case ip.IsLoopback() || ip.IsUnspecified():
		return "loopback"
	case ip.IsLinkLocalUnicast():
		return "link-local" // 169.254.0.0/16 (cloud metadata), fe80::/10
	case ip.IsPrivate():
		return "private" // RFC 1918, fc00::/7
	}
	return ""
}

// allowAddr reports whether the crawl may connect to ip. Public addresses
// are always allowed. A non-public one is allowed only when the start
// host itself resolves to the same kind, so checking a site on localhost
// or on the office network keeps working while a public site can't
// steer the crawler at the cloud metadata service or the local network.
func allowAddr(startIPs []netip.Addr, ip netip.Addr) bool {
	class := addrClass(ip)
	if class == "" {
		return true
	}
	for _, s := range startIPs {
		if addrClass(s) == class {
			return true
		}
	}
	return false
}

// resolve returns the addresses host resolves to. An IP literal is
// returned as is. A lookup failure gives none, so only public addresses
// are allowed.
func resolve(ctx context.Context, host string) []netip.Addr {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil
	}
	return addrs
}
