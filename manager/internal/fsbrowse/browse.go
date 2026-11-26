package fsbrowse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultRoots are the directories users may browse and create under.
var DefaultRoots = []string{"/mnt"}

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	Mode    string `json:"mode,omitempty"`
}

type Listing struct {
	Path         string   `json:"path"`
	Parent       string   `json:"parent,omitempty"`
	Exists       bool     `json:"exists"`
	IsDir        bool     `json:"isDir"`
	Entries      []Entry  `json:"entries"`
	AllowedRoots []string `json:"allowedRoots"`
	Writable     bool     `json:"writable"`
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

// Resolve ensures path is absolute and under an allowed root.
func Resolve(path string, roots []string) (string, error) {
	clean, ok := underRoot(path, roots)
	if !ok {
		return "", fmt.Errorf("path must be under allowed roots: %s", strings.Join(roots, ", "))
	}
	return clean, nil
}

// IsRoot reports whether path equals one of the browse roots.
func IsRoot(path string, roots []string) bool {
	clean := filepath.Clean(path)
	for _, root := range roots {
		if clean == filepath.Clean(root) {
			return true
		}
	}
	return false
}

func List(path string, roots []string) (Listing, error) {
	clean, err := Resolve(path, roots)
	if err != nil {
		return Listing{}, err
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

	out.Writable = info.Mode().Perm()&0o200 != 0 || canWriteDir(clean)

	entries, err := os.ReadDir(clean)
	if err != nil {
		return Listing{}, err
	}
	items := make([]Entry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, Entry{
			Name:    name,
			Path:    filepath.Join(clean, name),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
			Mode:    info.Mode().String(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	out.Entries = items
	return out, nil
}

func Mkdir(path string, roots []string) (Listing, error) {
	clean, err := Resolve(path, roots)
	if err != nil {
		return Listing{}, err
	}
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return Listing{}, fmt.Errorf("mkdir: %w", err)
	}
	return List(clean, roots)
}

// Touch creates an empty file (or updates mtime if it exists).
func Touch(path string, roots []string) (Entry, error) {
	clean, err := Resolve(path, roots)
	if err != nil {
		return Entry{}, err
	}
	if IsRoot(clean, roots) {
		return Entry{}, fmt.Errorf("cannot create file at browse root")
	}
	f, err := os.OpenFile(clean, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return Entry{}, err
	}
	_ = f.Close()
	now := time.Now()
	_ = os.Chtimes(clean, now, now)
	return Stat(clean, roots)
}

func Stat(path string, roots []string) (Entry, error) {
	clean, err := Resolve(path, roots)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(clean)
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		Name:    filepath.Base(clean),
		Path:    clean,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime().UTC().Format(time.RFC3339),
		Mode:    info.Mode().String(),
	}, nil
}

func Rename(from, to string, roots []string) (Entry, error) {
	src, err := Resolve(from, roots)
	if err != nil {
		return Entry{}, err
	}
	dst, err := Resolve(to, roots)
	if err != nil {
		return Entry{}, err
	}
	if IsRoot(src, roots) {
		return Entry{}, fmt.Errorf("cannot rename a browse root")
	}
	if err := os.Rename(src, dst); err != nil {
		return Entry{}, err
	}
	return Stat(dst, roots)
}

func Delete(path string, roots []string) error {
	clean, err := Resolve(path, roots)
	if err != nil {
		return err
	}
	if IsRoot(clean, roots) {
		return fmt.Errorf("cannot delete a browse root")
	}
	return os.RemoveAll(clean)
}

func Move(from, to string, roots []string) (Entry, error) {
	return Rename(from, to, roots)
}

func Copy(from, to string, roots []string) (Entry, error) {
	src, err := Resolve(from, roots)
	if err != nil {
		return Entry{}, err
	}
	dst, err := Resolve(to, roots)
	if err != nil {
		return Entry{}, err
	}
	if IsRoot(src, roots) {
		return Entry{}, fmt.Errorf("cannot copy a browse root")
	}
	info, err := os.Stat(src)
	if err != nil {
		return Entry{}, err
	}
	if info.IsDir() {
		if err := copyDir(src, dst); err != nil {
			return Entry{}, err
		}
	} else {
		if err := copyFile(src, dst, info.Mode()); err != nil {
			return Entry{}, err
		}
	}
	return Stat(dst, roots)
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return err
		}
		if err := copyFile(s, d, fi.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// SaveUpload writes an uploaded file to destPath (full file path under roots).
func SaveUpload(destPath string, r io.Reader, roots []string) (Entry, error) {
	clean, err := Resolve(destPath, roots)
	if err != nil {
		return Entry{}, err
	}
	if IsRoot(clean, roots) {
		return Entry{}, fmt.Errorf("cannot upload onto a browse root path")
	}
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		return Entry{}, err
	}
	f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Entry{}, err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return Entry{}, err
	}
	if err := f.Close(); err != nil {
		return Entry{}, err
	}
	return Stat(clean, roots)
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
