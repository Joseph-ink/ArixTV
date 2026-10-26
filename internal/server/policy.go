package server

import (
	"net/http"
	"net/netip"
)

type ListenerKind string

const (
	Loopback ListenerKind = "loopback"
	LAN      ListenerKind = "lan"
)

func peerIP(r *http.Request) (netip.Addr, bool) {
	addr, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || addr.Addr().Zone() != "" {
		return netip.Addr{}, false
	}
	return addr.Addr().Unmap(), true
}
func contains(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func policyPrefix(value string) netip.Prefix {
	p, _ := netip.ParsePrefix(value) // Config validation precedes policy construction.
	if p.Addr().Is4In6() && p.Bits() >= 96 {
		return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
	}
	return p
}
func forwarded(r *http.Request) bool {
	for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Proto", "Forwarded", "X-Real-IP"} {
		if _, ok := r.Header[http.CanonicalHeaderKey(name)]; ok {
			return true
		}
	}
	return false
}

// effectiveIP accepts only the documented single-hop Nginx convention. Headers
// from LAN/untrusted peers are ignored, never used for ACL or limiter identity.
func (a *App) effectiveIP(kind ListenerKind, r *http.Request) (netip.Addr, bool) {
	ip, ok := peerIP(r)
	if !ok {
		return ip, false
	}
	if kind == LAN {
		return ip, a.cfg.LAN.Enabled && contains(a.lanNetworks, ip)
	}
	if kind != Loopback || !ip.IsLoopback() {
		return ip, false
	}
	if !contains(a.trustedProxies, ip) {
		return ip, true
	}
	xff, proto := r.Header.Values("X-Forwarded-For"), r.Header.Values("X-Forwarded-Proto")
	if len(xff) == 0 && len(proto) == 0 {
		return ip, true
	}
	if len(xff) != 1 || len(proto) != 1 || proto[0] != "https" {
		return ip, false
	}
	client, err := netip.ParseAddr(xff[0])
	if err != nil || client.Zone() != "" || client.IsUnspecified() || client.IsMulticast() {
		return ip, false
	}
	return client.Unmap(), true
}
