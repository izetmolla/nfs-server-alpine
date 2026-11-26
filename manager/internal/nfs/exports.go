package nfs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
)

type Service struct {
	ExportsFile string

	mu         sync.RWMutex
	lastApply  time.Time
	lastOK     bool
	lastOutput string
	lastError  string
}

type Status struct {
	MountdRunning bool      `json:"mountdRunning"`
	ExportsLive   string    `json:"exportsLive"`
	LastApplyAt   time.Time `json:"lastApplyAt"`
	LastApplyOK   bool      `json:"lastApplyOK"`
	LastOutput    string    `json:"lastOutput"`
	LastError     string    `json:"lastError"`
}

type ApplyOptions struct {
	GlobalExtraOptions string
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Status{
		MountdRunning: mountdRunning(),
		ExportsLive:   liveExports(),
		LastApplyAt:   s.lastApply,
		LastApplyOK:   s.lastOK,
		LastOutput:    s.lastOutput,
		LastError:     s.lastError,
	}
}

func (s *Service) Apply(exports []models.Export, opts ApplyOptions) (string, error) {
	if err := ValidateExportSet(exports); err != nil {
		return "", err
	}

	enabled := make([]models.Export, 0, len(exports))
	for _, e := range exports {
		if e.Enabled {
			_ = NormalizeExport(&e)
			enabled = append(enabled, e)
		}
	}

	sort.SliceStable(enabled, func(i, j int) bool {
		return enabled[i].Fsid < enabled[j].Fsid
	})

	var b strings.Builder
	for _, e := range enabled {
		if err := os.MkdirAll(e.Path, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", e.Path, err)
		}
		line, err := BuildExportLine(e, opts.GlobalExtraOptions)
		if err != nil {
			return "", err
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	content := b.String()
	if content == "" {
		content = "# no enabled exports\n"
	}

	if err := os.MkdirAll(filepath.Dir(s.ExportsFile), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(s.ExportsFile, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write exports: %w", err)
	}

	out, err := runExportfs()
	s.mu.Lock()
	s.lastApply = time.Now().UTC()
	s.lastOutput = out
	if err != nil {
		s.lastOK = false
		s.lastError = err.Error()
		s.mu.Unlock()
		return out, err
	}
	s.lastOK = true
	s.lastError = ""
	s.mu.Unlock()
	return out, nil
}

func runExportfs() (string, error) {
	cmd := exec.Command("/usr/sbin/exportfs", "-rv")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		if out == "" {
			out = err.Error()
		}
		return out, fmt.Errorf("exportfs failed: %s", out)
	}

	list := exec.Command("/usr/sbin/exportfs", "-v")
	var listBuf bytes.Buffer
	list.Stdout = &listBuf
	list.Stderr = &listBuf
	_ = list.Run()
	listed := strings.TrimSpace(listBuf.String())
	if listed != "" {
		if out != "" {
			out += "\n" + listed
		} else {
			out = listed
		}
	}
	return out, nil
}

func mountdRunning() bool {
	cmd := exec.Command("pidof", "rpc.mountd")
	return cmd.Run() == nil
}

func liveExports() string {
	cmd := exec.Command("/usr/sbin/exportfs", "-v")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	_ = cmd.Run()
	return strings.TrimSpace(buf.String())
}
