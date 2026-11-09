// Package clientip resolves the real client address of a request.
//
// X-Forwarded-For is only honoured when the direct peer is a trusted proxy
// (TRUSTED_PROXIES). The chain is walked right to left and the first address
// that is not a trusted proxy is the client; anything to its left was supplied
// by the client and is ignored. Requests from untrusted peers use RemoteAddr,
// so a spoofed header cannot bypass IP rate limits.
package clientip

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// DefaultTrustedProxies covers loopback and private ranges: the BFF, Traefik and
// other containers on the internal Docker network.
const DefaultTrustedProxies = "127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,fc00::/7"

type ctxKey struct{}

// Resolver picks the client address using a set of trusted proxy networks.
type Resolver struct {
	trusted []netip.Prefix
}

// ParseTrusted parses a comma-separated list of CIDRs or single addresses.
func ParseTrusted(csv string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", part, err)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", part, err)
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

// New builds a resolver. An empty list trusts nobody: headers are always ignored.
func New(trusted []netip.Prefix) *Resolver {
	return &Resolver{trusted: trusted}
}

func (rs *Resolver) isTrusted(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range rs.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Resolve returns the client address for r.
func (rs *Resolver) Resolve(r *http.Request) netip.Addr {
	peer, ok := remoteAddr(r.RemoteAddr)
	if !ok || !rs.isTrusted(peer) {
		return peer
	}
	hops := forwardedChain(r.Header.Values("X-Forwarded-For"))
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(hops[i])
		if err != nil {
			// A malformed entry breaks the chain; stop at the last good hop.
			break
		}
		client = hop.Unmap()
		if !rs.isTrusted(client) {
			break
		}
	}
	return client
}

// Middleware stores the resolved address in the context and rewrites
// RemoteAddr, so code that reads RemoteAddr directly sees the client.
func (rs *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr := rs.Resolve(r)
		if addr.IsValid() {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, addr))
			r.RemoteAddr = net.JoinHostPort(addr.String(), "0")
		}
		next.ServeHTTP(w, r)
	})
}

// From returns the client address resolved by Middleware, falling back to RemoteAddr.
func From(r *http.Request) netip.Addr {
	if a, ok := r.Context().Value(ctxKey{}).(netip.Addr); ok {
		return a
	}
	a, _ := remoteAddr(r.RemoteAddr)
	return a
}

// String returns the client address as text, or "" when unknown.
func String(r *http.Request) string {
	a := From(r)
	if !a.IsValid() {
		return ""
	}
	return a.String()
}

// IP returns the client address as net.IP, or nil when unknown.
func IP(r *http.Request) net.IP {
	a := From(r)
	if !a.IsValid() {
		return nil
	}
	return net.IP(a.AsSlice())
}

func remoteAddr(s string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap(), true
	}
	return netip.Addr{}, false
}

func forwardedChain(values []string) []string {
	var hops []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				hops = append(hops, part)
			}
		}
	}
	return hops
}
