package netutil

import (
	"net"
	"strings"
)

// IPAllowed reports whether clientIP matches any of the allowed CIDRs/IPs.
// Empty, "*", or "0.0.0.0/0" (and optionally "::/0") allow everything for that family.
func IPAllowed(clientIP, allowed string) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" || allowed == "*" {
		return true
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	parts := strings.Fields(strings.ReplaceAll(allowed, ",", " "))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "*" {
			return true
		}
		if part == "0.0.0.0/0" || part == "::/0" {
			return true
		}
		if partIP := net.ParseIP(part); partIP != nil {
			if partIP.Equal(ip) {
				return true
			}
			continue
		}
		if _, network, err := net.ParseCIDR(part); err == nil {
			if network.Contains(ip) {
				return true
			}
		}
	}
	return false
}
