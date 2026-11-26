package fsbrowse

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultRoots are the directories users may browse and create under.
var DefaultRoots = []string{"/mnt"}

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type Listing struct {
	Path        string   `json:"path"`
	Parent      string   `json:"parent,omitempty"`
	Exists      bool     `json:"exists"`
	IsDir       bool     `json:"isDir"`
	Entries     []Entry  `json:"entries"`
	AllowedRoots []string `json:"allowedRoots"`
	Writable    bool     `json:"writable"`
}

func RootsFromEnv(env string) []string {
	if strings.TrimSpace(env) == "" {
		return append([]string{}, DefaultRoots...)
	}
	parts := strings.FieldsFunc(env, func(r rune) bool {
		return r == ',' || r == ':' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" || p == "." {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return append([]string{}, DefaultRoots...)
	}
	return out
}

func underRoot(path string, roots []string) (string, bool) {
	clean := filepath.Clean(path)
	if clean == "/" {
		return "", false
	}
	for _, root := range roots {
		root = filepath.Clean(root)
		if clean == root || strings.HasPrefix(clean, root+string(os.PathSeparator)) {
			return clean, true
		}
	}
	return "", false
}

func List(path string, roots []string) (Listing, error) {
	clean, ok := underRoot(path, roots)
	if !ok {
		return Listing{}, fmt.Errorf("path must be under allowed roots: %s", strings.Join(roots, ", "))
	}

	out := Listing{
		Path:         clean,
		AllowedRoots: roots,
		Entries:      []Entry{},
	}
	parent := filepath.Dir(clean)
	if _, ok := underRoot(parent, roots); ok && parent != clean {
		out.Parent = parent
	}

	info, err := os.Stat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			out.Exists = false
			out.IsDir = false
			out.Writable = false
			return out, nil
		}
		return Listing{}, err
	}
	out.Exists = true
	out.IsDir = info.IsDir()
	if !info.IsDir() {
		return out, fmt.Errorf("%s is not a directory", clean)
	}

	// Heuristic: try open for write by checking permission bit / create probe.
	out.Writable = info.Mode().Perm()&0o200 != 0 || canWriteDir(clean)

	entries, err := os.ReadDir(clean)
	if err != nil {
		return Listing{}, err
	}
	dirs := make([]Entry, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, Entry{
			Name:  name,
			Path:  filepath.Join(clean, name),
			IsDir: true,
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	out.Entries = dirs
	return out, nil
}

func Mkdir(path string, roots []string) (Listing, error) {
	clean, ok := underRoot(path, roots)
	if !ok {
		return Listing{}, fmt.Errorf("path must be under allowed roots: %s", strings.Join(roots, ", "))
	}
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return Listing{}, fmt.Errorf("mkdir: %w", err)
	}
	return List(clean, roots)
}

func canWriteDir(path string) bool {
	f, err := os.CreateTemp(path, ".nfs-write-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
