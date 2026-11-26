package mcpserver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/nfs"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/reports"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

func (t *tools) analyzeNFS(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	data, err := t.buildAnalysis()
	if err != nil {
		return errResult(err)
	}
	return textResult(data)
}

func (t *tools) buildAnalysis() (map[string]any, error) {
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return nil, err
	}
	status := t.NFS.Status()
	storage := reports.Collect(exports)
	quotas := reports.CollectQuotas(exports)
	conns := nfs.Connections()
	logs := t.Log.Since(0, 50)

	enabled, disabled, fsid0 := 0, 0, 0
	var warnings []string
	for _, e := range exports {
		if e.Enabled {
			enabled++
		} else {
			disabled++
		}
		if e.Fsid == 0 {
			fsid0++
		}
		if e.Enabled && e.Clients == "*" {
			warnings = append(warnings, fmt.Sprintf("export id=%d path=%s allows all clients (*)", e.ID, e.Path))
		}
		if e.Enabled {
			if _, err := os.Stat(e.Path); err != nil {
				warnings = append(warnings, fmt.Sprintf("export id=%d path=%s missing on disk: %v", e.ID, e.Path, err))
			}
		}
	}
	if fsid0 == 0 && enabled > 0 {
		warnings = append(warnings, "no enabled export uses fsid=0 (NFSv4 root); clients may fail to mount /")
	}
	if fsid0 > 1 {
		warnings = append(warnings, fmt.Sprintf("%d exports claim fsid=0; only one should", fsid0))
	}
	if !status.LastApplyOK && status.LastError != "" {
		warnings = append(warnings, "last exportfs apply failed: "+status.LastError)
	}

	fullQuotas := 0
	for _, q := range quotas.Exports {
		if q.Full {
			fullQuotas++
			warnings = append(warnings, fmt.Sprintf("quota full: export id=%d path=%s used=%d/%d", q.ExportID, q.Path, q.UsedBytes, q.QuotaBytes))
		}
	}

	errorLogs := 0
	for _, e := range logs {
		if e.Level == "error" {
			errorLogs++
		}
	}

	healthy := len(warnings) == 0 && status.LastApplyOK
	return map[string]any{
		"healthy":        healthy,
		"summary":        fmt.Sprintf("%d exports (%d enabled, %d disabled), %d active clients, %d full quotas, %d recent error logs", len(exports), enabled, disabled, conns.ActiveClients, fullQuotas, errorLogs),
		"warnings":       warnings,
		"status":         status,
		"connections":    conns,
		"storage":        storage,
		"quotas":         quotas,
		"exportsTotal":   len(exports),
		"exportsEnabled": enabled,
		"recentErrors":   errorLogs,
		"checkedAt":      time.Now().UTC(),
		"suggestedNext": []string{
			"Read nfs://guide/playbook for workflows",
			"Use patch_export / create_export / apply_exports to act on warnings",
			"Use nfs://logs/recent if recentErrors > 0",
		},
	}, nil
}

func (t *tools) previewExport(ctx context.Context, _ *mcp.CallToolRequest, params exportParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	exp, err := mapExportParams(params, true)
	if err != nil {
		// allow preview of partial update-shaped payloads
		exp, err = mapExportParams(params, false)
		if err != nil {
			return errResult(err)
		}
		if exp.Path == "" {
			return errResult(fmt.Errorf("path is required for preview"))
		}
	}
	cfg, _ := t.Settings.LoadAppSettings()
	line, err := nfs.BuildExportLine(exp, cfg.GlobalExtraOptions)
	if err != nil {
		return errResult(err)
	}
	return textResult(map[string]any{
		"exportLine": line,
		"export":     nfs.ToDTO(exp, cfg.GlobalExtraOptions),
	})
}

func (t *tools) optionCatalog(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	return textResult(nfs.Catalog())
}

func (t *tools) exportUsage(ctx context.Context, _ *mcp.CallToolRequest, params idParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	if params.ID == 0 {
		return errResult(fmt.Errorf("id is required"))
	}
	var exp models.Export
	if err := t.DB.First(&exp, params.ID).Error; err != nil {
		return errResult(err)
	}
	return textResult(reports.EvaluateExport(exp))
}

func (t *tools) validateClients(ctx context.Context, _ *mcp.CallToolRequest, params clientsParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	normalized, err := netutil.NormalizeAndValidateClients(params.Clients)
	if err != nil {
		return textResult(map[string]any{"valid": false, "error": err.Error(), "clients": params.Clients})
	}
	return textResult(map[string]any{"valid": true, "clients": normalized})
}

func (t *tools) getSettings(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	cfg, err := t.Settings.LoadAppSettings()
	if err != nil {
		return errResult(err)
	}
	return textResult(cfg)
}

func (t *tools) patchExport(ctx context.Context, _ *mcp.CallToolRequest, params patchParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if params.ID == 0 {
		return errResult(fmt.Errorf("id is required"))
	}
	if params.Enabled == nil && params.AccessMode == nil && params.ReadOnly == nil && params.Description == nil {
		return errResult(fmt.Errorf("no fields to patch"))
	}
	var exp models.Export
	if err := t.DB.First(&exp, params.ID).Error; err != nil {
		return errResult(err)
	}
	changedLive := false
	if params.Description != nil {
		exp.Description = strings.TrimSpace(*params.Description)
	}
	if params.Enabled != nil && *params.Enabled != exp.Enabled {
		exp.Enabled = *params.Enabled
		changedLive = true
	}
	if params.ReadOnly != nil {
		next := "rw"
		if *params.ReadOnly {
			next = "ro"
		}
		if next != exp.AccessMode {
			exp.AccessMode = next
			changedLive = true
		}
	}
	if params.AccessMode != nil {
		mode := strings.TrimSpace(*params.AccessMode)
		if mode != "rw" && mode != "ro" {
			return errResult(fmt.Errorf("accessMode must be rw or ro"))
		}
		if mode != exp.AccessMode {
			exp.AccessMode = mode
			changedLive = true
		}
	}
	if err := nfs.NormalizeExport(&exp); err != nil {
		return errResult(err)
	}
	if err := t.DB.Save(&exp).Error; err != nil {
		return errResult(err)
	}
	cfg, _ := t.Settings.LoadAppSettings()
	out := map[string]any{"export": nfs.ToDTO(exp, cfg.GlobalExtraOptions)}
	apply := changedLive
	if params.Apply != nil {
		apply = *params.Apply
	}
	if apply {
		if err := t.applyInto(out); err != nil {
			return errResult(err)
		}
	}
	return textResult(out)
}

func (t *tools) cloneExport(ctx context.Context, _ *mcp.CallToolRequest, params cloneParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if params.ID == 0 {
		return errResult(fmt.Errorf("id is required"))
	}
	path := strings.TrimSpace(params.Path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return errResult(fmt.Errorf("path must be an absolute directory"))
	}
	var src models.Export
	if err := t.DB.First(&src, params.ID).Error; err != nil {
		return errResult(err)
	}
	if path == src.Path {
		return errResult(fmt.Errorf("clone path must differ from the source"))
	}
	var existing int64
	if err := t.DB.Model(&models.Export{}).Where("path = ?", path).Count(&existing).Error; err != nil {
		return errResult(err)
	}
	if existing > 0 {
		return errResult(fmt.Errorf("an export with this path already exists"))
	}
	clone := src
	clone.ID = 0
	clone.Path = path
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.QuotaNotifiedAt = nil
	if params.Description != nil {
		clone.Description = strings.TrimSpace(*params.Description)
	} else if strings.TrimSpace(src.Description) != "" {
		clone.Description = "Clone of " + src.Description
	} else {
		clone.Description = "Clone of " + src.Path
	}
	clone.Fsid = nextExportFsid(t.DB, src.Fsid)
	if err := nfs.NormalizeExport(&clone); err != nil {
		return errResult(err)
	}
	if err := os.MkdirAll(clone.Path, 0o755); err != nil {
		return errResult(fmt.Errorf("create path: %w", err))
	}
	if err := t.DB.Create(&clone).Error; err != nil {
		return errResult(err)
	}
	t.Log.Info("mcp", "cloned export", fmt.Sprintf("from=%d → id=%d path=%s", src.ID, clone.ID, clone.Path))
	cfg, _ := t.Settings.LoadAppSettings()
	out := map[string]any{"export": nfs.ToDTO(clone, cfg.GlobalExtraOptions)}
	if params.Apply != nil && *params.Apply {
		if err := t.applyInto(out); err != nil {
			return errResult(err)
		}
	}
	return textResult(out)
}

func (t *tools) checkQuotas(ctx context.Context, _ *mcp.CallToolRequest, params checkQuotasParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if t.Quotas == nil {
		return errResult(fmt.Errorf("quota watcher unavailable"))
	}
	notify := true
	if params.Notify != nil {
		notify = *params.Notify
	}
	report, err := t.Quotas.Check(notify)
	if err != nil {
		return errResult(err)
	}
	return textResult(map[string]any{"ok": true, "report": report})
}

func (t *tools) updateSettings(ctx context.Context, _ *mcp.CallToolRequest, params settingsParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	cfg, err := t.Settings.LoadAppSettings()
	if err != nil {
		return errResult(err)
	}
	if params.DefaultClients != nil {
		cfg.DefaultClients = *params.DefaultClients
	}
	if params.DefaultReadOnly != nil {
		cfg.DefaultReadOnly = *params.DefaultReadOnly
	}
	if params.DefaultSync != nil {
		cfg.DefaultSync = *params.DefaultSync
	}
	if params.DefaultExtraOpts != nil {
		cfg.DefaultExtraOpts = *params.DefaultExtraOpts
	}
	if params.GlobalExtraOptions != nil {
		cfg.GlobalExtraOptions = *params.GlobalExtraOptions
	}
	if params.SwaggerEnabled != nil {
		cfg.SwaggerEnabled = *params.SwaggerEnabled
	}
	if params.PanelAllowedCIDRs != nil {
		cfg.PanelAllowedCIDRs = *params.PanelAllowedCIDRs
	}
	saved, err := t.Settings.SaveAppSettings(cfg)
	if err != nil {
		return errResult(err)
	}
	t.Log.Info("mcp", "settings updated", "")
	return textResult(saved)
}

func nextExportFsid(db *gorm.DB, prefer uint) uint {
	var exports []models.Export
	_ = db.Select("fsid").Find(&exports).Error
	used := map[uint]struct{}{}
	var max uint
	for _, e := range exports {
		used[e.Fsid] = struct{}{}
		if e.Fsid > max {
			max = e.Fsid
		}
	}
	if prefer > 0 {
		if _, ok := used[prefer]; !ok {
			return prefer
		}
	}
	for candidate := uint(1); candidate < 1<<20; candidate++ {
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
	return max + 1
}
