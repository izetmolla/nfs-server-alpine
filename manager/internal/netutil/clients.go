package netutil

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

// NormalizeAndValidateClients validates NFS client specs.
// Allowed: "*", IPv4/IPv6, CIDR, hostname/FQDN, space/comma-separated lists.
func NormalizeAndValidateClients(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "*", nil
	}

	parts := splitClients(raw)
	if len(parts) == 0 {
		return "*", nil
	}

	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "*" {
			normalized = append(normalized, "*")
			continue
		}
		if ip := net.ParseIP(part); ip != nil {
			normalized = append(normalized, ip.String())
			continue
		}
		if _, network, err := net.ParseCIDR(part); err == nil {
			normalized = append(normalized, network.String())
			continue
		}
		if looksLikeBadIP(part) {
			return "", fmt.Errorf("invalid IP or CIDR: %q", part)
		}
		if !isValidHostname(part) {
			return "", fmt.Errorf("invalid client host: %q (use *, IP, CIDR, or hostname)", part)
		}
		normalized = append(normalized, strings.ToLower(part))
	}

	if len(normalized) == 0 {
		return "*", nil
	}
	return strings.Join(normalized, " "), nil
}

func splitClients(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	return strings.Fields(raw)
}

func looksLikeBadIP(s string) bool {
	if strings.Contains(s, "/") {
		return true
	}
	if strings.Count(s, ".") == 3 {
		parts := strings.Split(s, ".")
		numeric := true
		for _, p := range parts {
			if p == "" {
				numeric = false
				break
			}
			for _, r := range p {
				if r < '0' || r > '9' {
					numeric = false
					break
				}
			}
			if !numeric {
				break
			}
		}
		if numeric {
			return true
		}
	}
	if strings.Contains(s, ":") {
		hexish := true
		for _, r := range s {
			switch {
			case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F', r == ':', r == '.':
			default:
				hexish = false
			}
		}
		if hexish {
			return true
		}
	}
	return false
}

func isValidHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	if host[0] == '.' || host[len(host)-1] == '.' {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}
