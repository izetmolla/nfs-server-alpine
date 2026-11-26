package nfs

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ConnectionInfo struct {
	ActiveClients int      `json:"activeClients"`
	Addresses     []string `json:"addresses"`
	Source        string   `json:"source"`
	Detail        string   `json:"detail,omitempty"`
}

// Connections best-effort snapshot of NFS client connections.
func Connections() ConnectionInfo {
	if info, ok := fromNFSDClients(); ok {
		return info
	}
	if info, ok := fromSS(); ok {
		return info
	}
	return ConnectionInfo{Source: "unavailable", Detail: "could not read nfsd clients or ss"}
}

func fromNFSDClients() (ConnectionInfo, bool) {
	root := "/proc/fs/nfsd/clients"
	entries, err := os.ReadDir(root)
	if err != nil {
		return ConnectionInfo{}, false
	}
	addrs := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		infoPath := filepath.Join(root, e.Name(), "info")
		data, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}
		addr := parseClientInfoAddr(string(data))
		if addr != "" {
			addrs = append(addrs, addr)
		} else {
			addrs = append(addrs, "client-"+e.Name())
		}
	}
	return ConnectionInfo{
		ActiveClients: len(addrs),
		Addresses:     addrs,
		Source:        "proc-nfsd",
	}, true
}

func parseClientInfoAddr(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "address:") {
			return strings.TrimSpace(line[len("address:"):])
		}
		if strings.HasPrefix(lower, "clientaddr:") {
			return strings.TrimSpace(line[len("clientaddr:"):])
		}
	}
	return ""
}

func fromSS() (ConnectionInfo, bool) {
	cmd := exec.Command("ss", "-H", "-tn", "sport", "=", ":2049")
	out, err := cmd.Output()
	if err != nil {
		return ConnectionInfo{}, false
	}
	seen := map[string]struct{}{}
	addrs := make([]string, 0)
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		peer := fields[len(fields)-1]
		host, _, ok := splitHostPort(peer)
		if !ok {
			continue
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		addrs = append(addrs, host)
	}
	return ConnectionInfo{
		ActiveClients: len(addrs),
		Addresses:     addrs,
		Source:        "ss",
		Detail:        "established TCP peers on :2049",
	}, true
}

func splitHostPort(s string) (string, int, bool) {
	// ss may show [ipv6]:port or ip:port
	if strings.HasPrefix(s, "[") {
		end := strings.LastIndex(s, "]")
		if end <= 1 {
			return "", 0, false
		}
		host := s[1:end]
		portPart := strings.TrimPrefix(s[end+1:], ":")
		port, _ := strconv.Atoi(portPart)
		return host, port, true
	}
	idx := strings.LastIndex(s, ":")
	if idx <= 0 {
		return "", 0, false
	}
	host := s[:idx]
	port, _ := strconv.Atoi(s[idx+1:])
	return host, port, true
}
