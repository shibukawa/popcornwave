// Package requestorigin answers three questions for every caller that asks
// them: what scheme did this request arrive over, what origin does this
// deployment serve it as, and which client sent it.
//
// It exists because the first question was being answered three times. The
// CSRF middleware compared a whole origin from r.TLS alone, the security-header
// middleware read X-Forwarded-Proto behind a trusted-proxy gate, and the logout
// endpoint read the same header with no gate at all. Three answers to one
// question is one answer that drifts, so all of them now come from here.
package requestorigin

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Proxies is the set of peer networks whose forwarded headers this deployment
// reads. The zero value trusts nothing, which is the correct answer for a
// listener with no proxy in front of it.
//
// A header arriving from outside the set is treated as absent rather than as
// false, so a deployment that is behind a proxy but has declared none degrades
// to the answer it would have given before the proxy existed.
type Proxies struct {
	networks []*net.IPNet
}

// Compile turns configured addresses and CIDR blocks into the trust set. A
// bare address is taken as a single-host network.
//
// Errors name the offending value and leave the configuration key to the
// caller, since this package does not know which binding it came from.
func Compile(values []string) (Proxies, error) {
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return Proxies{}, fmt.Errorf("contains an empty value")
		}
		if ip := net.ParseIP(value); ip != nil {
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return Proxies{}, fmt.Errorf("%q: %w", value, err)
		}
		networks = append(networks, network)
	}
	return Proxies{networks: networks}, nil
}

// FromNetworks adopts networks a caller compiled elsewhere.
//
// It exists because the public middleware surfaces speak []*net.IPNet, a
// standard type an application outside this module can construct, while the
// resolution itself lives on this type.
func FromNetworks(networks []*net.IPNet) Proxies { return Proxies{networks: networks} }

// Networks returns the trust set in the form those public surfaces take.
func (p Proxies) Networks() []*net.IPNet { return p.networks }

// Empty reports whether this deployment declared no proxy at all.
func (p Proxies) Empty() bool { return len(p.networks) == 0 }

// Trusts reports whether an address, with or without a port, falls inside the
// set. An unparseable address is never trusted.
func (p Proxies) Trusts(address string) bool {
	ip := parseAddress(address)
	if ip == nil {
		return false
	}
	for _, network := range p.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Scheme is the scheme the client actually used.
//
// A direct TLS connection outranks every header, since it needs no assertion.
// Otherwise X-Forwarded-Proto is read, and only when the peer is one of this
// deployment's declared proxies.
func (p Proxies) Scheme(r *http.Request) string {
	return p.SchemeOf(r.TLS != nil, r.RemoteAddr, r.Header.Get("X-Forwarded-Proto"))
}

// SchemeOf is Scheme over the three facts it actually reads, for a caller whose
// request is not a *http.Request.
//
// The rule is the reason this is one function rather than two: a forwarded
// header is only evidence, and only from a declared peer, because anybody can
// send one. A second transport reimplementing that would be a fourth answer to
// the question this package exists to answer once.
func (p Proxies) SchemeOf(tls bool, remoteAddress, forwardedProto string) string {
	if tls {
		return "https"
	}
	if !p.Trusts(remoteAddress) {
		return "http"
	}
	proto, _, _ := strings.Cut(forwardedProto, ",")
	if strings.EqualFold(strings.TrimSpace(proto), "https") {
		return "https"
	}
	return "http"
}

// IsHTTPS reports whether the client reached this deployment over TLS.
func (p Proxies) IsHTTPS(r *http.Request) bool { return p.Scheme(r) == "https" }

// Of reconstructs the origin of the request itself, as scheme and host.
//
// It returns the empty string for a request carrying no Host, which never
// matches anything.
func (p Proxies) Of(r *http.Request) string {
	host := r.Host
	if host == "" {
		return ""
	}
	return p.Scheme(r) + "://" + strings.TrimSuffix(host, ":")
}

// ClientAddress is the address of the caller rather than of the relay in
// front of it, without a port.
//
// X-Forwarded-For is walked from the right while each hop is one of this
// deployment's own proxies; the first address outside the set is the client.
// A chain that is entirely trusted yields its leftmost entry, which is the
// closest thing to a client it records. A malformed entry abandons the header
// and returns the peer, because a chain that cannot be parsed cannot be
// trusted partway.
func (p Proxies) ClientAddress(r *http.Request) string {
	return p.ClientAddressOf(r.RemoteAddr, r.Header.Values("X-Forwarded-For"))
}

// ClientAddressOf is ClientAddress over the two facts it reads, for a caller
// whose request is not a *http.Request.
//
// The walk backwards through the forwarded chain is the part worth having once:
// it stops at the first hop this deployment does not trust, which is the last
// address a trusted peer vouched for, and it gives up entirely on an
// unparseable hop rather than accepting a later one. A second transport
// reimplementing that would be a second answer to who the caller is, and every
// rate limit and live bound counts against it.
func (p Proxies) ClientAddressOf(remoteAddress string, forwarded []string) string {
	peer := remoteHost(remoteAddress)
	if !p.Trusts(remoteAddress) {
		return peer
	}
	if len(forwarded) == 0 {
		return peer
	}
	hops := make([]string, 0, len(forwarded))
	for _, line := range forwarded {
		for _, hop := range strings.Split(line, ",") {
			hop = strings.TrimSpace(hop)
			if hop == "" {
				continue
			}
			hops = append(hops, hop)
		}
	}
	if len(hops) == 0 {
		return peer
	}
	for index := len(hops) - 1; index >= 0; index-- {
		if parseAddress(hops[index]) == nil {
			return peer
		}
		if !p.Trusts(hops[index]) {
			return remoteHost(hops[index])
		}
	}
	return remoteHost(hops[0])
}

// Matches reports whether the request came from this deployment's own origin
// or one the caller named in trusted, which may be nil.
//
// Origin is preferred, because a browser sets it on exactly the state-changing
// requests this protects. A literal null Origin is not one: it is what an
// opaque origin sends, and treating it as absent would fall through to the
// weaker check below.
//
// Referer is the fallback for a proxy that stripped Origin, and it is read
// strictly. A missing one is a refusal rather than a pass, since treating
// absence as trust would make the whole check optional for anything able to
// omit a header.
//
// The declared origins stay the stronger half of this comparison: a proxy set
// resolves what this deployment calls itself, and never makes an origin nobody
// declared acceptable.
func (p Proxies) Matches(r *http.Request, trusted map[string]bool) bool {
	self := p.Of(r)
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		return origin == self || trusted[origin]
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		return false
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	origin := parsed.Scheme + "://" + parsed.Host
	return origin == self || trusted[origin]
}

// Of resolves the origin for a deployment that declared no proxy.
func Of(r *http.Request) string { return Proxies{}.Of(r) }

// Matches compares origins for a deployment that declared no proxy.
func Matches(r *http.Request, trusted map[string]bool) bool {
	return Proxies{}.Matches(r, trusted)
}

// Set turns configured origin strings into the map Matches takes.
//
// Each value is normalized to scheme and host, so a trailing slash or a path
// someone pasted from a browser bar does not silently fail to match. A value
// that names no scheme or no host is dropped rather than stored, because it
// cannot match an origin and keeping it would suggest it could.
func Set(origins ...string) map[string]bool {
	trusted := make(map[string]bool, len(origins))
	for _, value := range origins {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		trusted[parsed.Scheme+"://"+parsed.Host] = true
	}
	return trusted
}

// parseAddress reads an address that may carry a port or square brackets.
func parseAddress(address string) net.IP {
	return net.ParseIP(remoteHost(address))
}

// remoteHost strips a port and IPv6 brackets, leaving the address alone when
// it carries neither.
func remoteHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	return strings.Trim(host, "[]")
}
