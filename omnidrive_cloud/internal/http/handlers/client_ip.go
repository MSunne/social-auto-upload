package handlers

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

var nonPublicIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func resolveHeartbeatPublicIP(r *http.Request, payloadPublicIP *string) *string {
	if derived := extractRequestPublicIP(r); derived != nil {
		return derived
	}
	return normalizePublicIPPtr(payloadPublicIP)
}

func extractRequestPublicIP(r *http.Request) *string {
	if r == nil {
		return nil
	}
	for _, candidate := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if ip := normalizePublicIPString(candidate); ip != nil {
			return ip
		}
	}
	if ip := normalizePublicIPString(r.Header.Get("X-Real-IP")); ip != nil {
		return ip
	}
	return normalizePublicIPString(r.RemoteAddr)
}

func normalizePublicIPPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return normalizePublicIPString(*value)
}

func normalizePublicIPString(value string) *string {
	addr, ok := parseIPCandidate(value)
	if !ok || !isLikelyPublicIP(addr) {
		return nil
	}
	normalized := addr.String()
	return &normalized
}

func parseIPCandidate(value string) (netip.Addr, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" || strings.EqualFold(candidate, "unknown") {
		return netip.Addr{}, false
	}

	if host, _, err := net.SplitHostPort(candidate); err == nil {
		candidate = host
	} else {
		candidate = strings.TrimPrefix(strings.TrimSuffix(candidate, "]"), "[")
	}

	if zoneIndex := strings.Index(candidate, "%"); zoneIndex >= 0 {
		candidate = candidate[:zoneIndex]
	}

	addr, err := netip.ParseAddr(candidate)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func isLikelyPublicIP(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsMulticast() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicIPPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}
