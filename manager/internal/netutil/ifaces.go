package netutil

import (
	"net"
	"sort"
	"strings"
)

type Address struct {
	IP        string `json:"ip"`
	CIDR      string `json:"cidr"`
	Family    string `json:"family"`
	Interface string `json:"interface"`
}

type InterfaceInfo struct {
	Name      string    `json:"name"`
	Up        bool      `json:"up"`
	Loopback  bool      `json:"loopback"`
	Addresses []Address `json:"addresses"`
}

func ListInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	out := make([]InterfaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		info := InterfaceInfo{
			Name:     iface.Name,
			Up:       iface.Flags&net.FlagUp != 0,
			Loopback: iface.Flags&net.FlagLoopback != 0,
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP
			family := "ipv4"
			if ip.To4() == nil {
				family = "ipv6"
			}
			info.Addresses = append(info.Addresses, Address{
				IP:        ip.String(),
				CIDR:      ipNet.String(),
				Family:    family,
				Interface: iface.Name,
			})
		}
		if len(info.Addresses) > 0 || info.Up {
			out = append(out, info)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func SuggestedClientCIDRs() ([]string, error) {
	ifaces, err := ListInterfaces()
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []string
	for _, iface := range ifaces {
		if iface.Loopback || !iface.Up {
			continue
		}
		for _, addr := range iface.Addresses {
			if addr.Family != "ipv4" {
				continue
			}
			if _, ok := seen[addr.CIDR]; ok {
				continue
			}
			seen[addr.CIDR] = struct{}{}
			out = append(out, addr.CIDR)
		}
	}
	sort.Strings(out)
	return out, nil
}
