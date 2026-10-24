// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package httpx

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

// ErrPrivate is returned (wrapped) when a Guard refuses to connect to a
// loopback, link-local or private address.
var ErrPrivate = errors.New("private address")

// Guard decides which addresses a client may connect to. Public
// addresses are always allowed. A loopback, link-local or private one is
// allowed only when the target host itself resolves to the same kind, so
// checking a site on localhost or on the office network keeps working
// while a public site can't steer a tool at the cloud metadata service
// (169.254.169.254) or the local network through a link or redirect.
//
// The check runs on the resolved address just before connecting, so a
// hostname that resolves to a private address is caught too. A proxy
// from the environment (HTTPS_PROXY and friends) is dialed as is: the
// user configured it and what it connects to is its own decision.
//
// A nil *Guard allows public addresses only. A Guard is safe for
// concurrent use.
type Guard struct {
	host     string
	mu       sync.Mutex // held while resolving, so the lookup runs once
	resolved bool
	ips      []netip.Addr
	proxies  sync.Map // proxy host:port → true, dialed without the check
}

// NewGuard returns a guard for a target on host. host is resolved on the
// first dial, not here, so building a client never waits on DNS. An
// empty host, or one that doesn't resolve, allows public addresses only.
func NewGuard(host string) *Guard { return &Guard{host: host} }

// targetIPs resolves the target host once. A lookup cut short by a
// canceled or expired ctx isn't kept, so one slow first answer doesn't
// leave the guard treating a localhost or LAN target as public for the
// life of the client; the next dial looks it up again.
func (g *Guard) targetIPs(ctx context.Context) []netip.Addr {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.resolved || g.host == "" {
		return g.ips
	}
	ips := Resolve(ctx, g.host)
	if len(ips) > 0 || ctx.Err() == nil {
		g.ips, g.resolved = ips, true
	}
	return ips
}

// Allow reports whether the guard lets a connection to ip through.
func (g *Guard) Allow(ctx context.Context, ip netip.Addr) bool {
	return AllowAddr(g.targetIPs(ctx), ip)
}

// Proxy is http.ProxyFromEnvironment that also notes the proxy's
// address, so DialContext lets it through. Use it as an
// http.Transport's Proxy together with DialContext.
func (g *Guard) Proxy(req *http.Request) (*url.URL, error) {
	p, err := http.ProxyFromEnvironment(req)
	if p != nil && g != nil {
		g.proxies.Store(canonicalAddr(p), true)
	}
	return p, err
}

// DialContext connects like the default transport but refuses addresses
// Allow rejects, before any packet is sent. Use it as an
// http.Transport's DialContext.
func (g *Guard) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	proxied := false
	if g != nil {
		_, proxied = g.proxies.Load(addr)
	}
	if !proxied {
		ips := g.targetIPs(ctx)
		d.ControlContext = func(_ context.Context, _, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if !AllowAddr(ips, ip) {
				return fmt.Errorf("refusing to connect to %s: %w (set AllowPrivate to allow it)", ip, ErrPrivate)
			}
			return nil
		}
	}
	return d.DialContext(ctx, network, addr)
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

// AddrClass names the kind of non-public address ip is ("loopback",
// "link-local" or "private"), or "" for a public one.
func AddrClass(ip netip.Addr) string {
	ip = ip.Unmap()
	// A NAT64 address carries an IPv4 one in its last 32 bits and
	// reaches it through the gateway, so it is classed by that address.
	if ip.Is6() && nat64.Contains(ip) {
		b := ip.As16()
		ip = netip.AddrFrom4([4]byte(b[12:]))
	}
	switch {
	case ip.IsLoopback() || ip.IsUnspecified():
		return "loopback"
	case ip.IsLinkLocalUnicast():
		return "link-local" // 169.254.0.0/16 (cloud metadata), fe80::/10
	case ip.IsPrivate() || cgnat.Contains(ip):
		return "private" // RFC 1918, fc00::/7, RFC 6598 shared space
	}
	return ""
}

var (
	// nat64 is the RFC 6052 well-known NAT64 prefix.
	nat64 = netip.MustParsePrefix("64:ff9b::/96")
	// cgnat is the RFC 6598 carrier-grade NAT range. Some clouds put
	// their metadata service there (Alibaba Cloud at 100.100.100.200).
	cgnat = netip.MustParsePrefix("100.64.0.0/10")
)

// AllowAddr reports whether a connection to ip is allowed for a target
// that resolves to targetIPs. A public ip is always allowed. A non-public
// one is allowed only when a target address is of the same class.
func AllowAddr(targetIPs []netip.Addr, ip netip.Addr) bool {
	class := AddrClass(ip)
	if class == "" {
		return true
	}
	for _, s := range targetIPs {
		if AddrClass(s) == class {
			return true
		}
	}
	return false
}

// Resolve returns the addresses host resolves to. An IP literal is
// returned as is. A lookup failure gives none, so only public addresses
// are allowed.
func Resolve(ctx context.Context, host string) []netip.Addr {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil
	}
	return addrs
}
