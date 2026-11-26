package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/applog"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/fsbrowse"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/mcpoauth"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/metrics"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/nfs"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/quota"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/reports"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/settings"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// Deps wires NFS manager services into MCP tools.
type Deps struct {
	DB          *gorm.DB
	NFS         *nfs.Service
	Settings    *settings.Store
	Log         *applog.Ring
	Metrics     *metrics.Collector
	Quotas      *quota.Watcher
	BrowseRoots []string
}

type tools struct {
	Deps
}

func (t *tools) allowWrite() bool {
	if t.Settings == nil {
		return false
	}
	cfg, err := t.Settings.LoadMCPSettings()
	return err == nil && cfg.AllowWrite
}

func (t *tools) requireRead(ctx context.Context) error {
	tok := mcpoauth.TokenFromContext(ctx)
	if tok == nil {
		return fmt.Errorf("unauthorized: missing token")
	}
	// nfs:write implies read; empty scope treated as read for lenient clients.
	if strings.TrimSpace(tok.Scope) == "" {
		return nil
	}
	if mcpoauth.HasScope(ctx, mcpoauth.ScopeNFSRead) || mcpoauth.HasScope(ctx, mcpoauth.ScopeNFSWrite) {
		return nil
	}
	return fmt.Errorf("forbidden: missing scope %s", mcpoauth.ScopeNFSRead)
}

func (t *tools) requireWrite(ctx context.Context) error {
	if !t.allowWrite() {
		return fmt.Errorf("write tools are disabled (mcp_allow_write=false)")
	}
	if !mcpoauth.HasScope(ctx, mcpoauth.ScopeNFSWrite) {
		return fmt.Errorf("forbidden: missing scope %s", mcpoauth.ScopeNFSWrite)
	}
	return nil
}

func (t *tools) browseRoots() []string {
	if len(t.BrowseRoots) > 0 {
		return t.BrowseRoots
	}
	return fsbrowse.RootsFromEnv(os.Getenv("BROWSE_ROOTS"))
}

func textResult(v any) (*mcp.CallToolResult, any, error) {
	var text string
	switch x := v.(type) {
	case string:
		text = x
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, nil, err
		}
		text = string(b)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

func errResult(err error) (*mcp.CallToolResult, any, error) {
	return nil, nil, err
}

// NewServer builds an MCP server with full NFS manager tools (read + write).
// Write handlers require nfs:write scope and mcp allowWrite (default true).
func NewServer(deps Deps) *mcp.Server {
	t := &tools{Deps: deps}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "nfs-manager",
		Version: "1.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Full NFS Manager MCP. Prefer resources for context (nfs://guide/playbook, nfs://analysis, nfs://exports, nfs://status, nfs://option-catalog), then tools to act (create/update/clone/patch/delete/apply exports, quotas, mkdir, settings). Authorize with scope nfs:read nfs:write. Write tools need mcp allowWrite (on by default).",
	})

	// --- analyze / read ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "nfs_status",
		Description: "Return NFS daemon / exportfs status snapshot (mountd, last apply, live exports).",
	}, t.nfsStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nfs_dashboard",
		Description: "Dashboard summary: status, storage, connections, metrics, export counts.",
	}, t.nfsDashboard)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_nfs",
		Description: "Deep analysis: health, live vs DB exports, storage/quota pressure, client connections, recent errors, and suggested fixes.",
	}, t.analyzeNFS)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_exports",
		Description: "List all NFS exports from the manager database.",
	}, t.listExports)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_export",
		Description: "Get a single NFS export by id.",
	}, t.getExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "preview_export",
		Description: "Preview the /etc/exports line that would be generated for the given export fields (no write).",
	}, t.previewExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "option_catalog",
		Description: "Typed NFS export option catalog (access/commit/squash/security/etc).",
	}, t.optionCatalog)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "storage_report",
		Description: "Per-export directory usage and filesystem capacity report.",
	}, t.storageReport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "quota_report",
		Description: "Per-export soft quota usage report.",
	}, t.quotaReport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_usage",
		Description: "Usage + quota evaluation for one export id.",
	}, t.exportUsage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_logs",
		Description: "List recent manager log entries.",
	}, t.listLogs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_interfaces",
		Description: "List host network interfaces and suggested client CIDRs.",
	}, t.listInterfaces)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_clients",
		Description: "Validate/normalize an NFS clients ACL string (* / CIDRs / IPs).",
	}, t.validateClients)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browse_path",
		Description: "Browse directories under configured browse roots.",
	}, t.browsePath)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_settings",
		Description: "Read panel NFS defaults and related settings (no password).",
	}, t.getSettings)

	// --- manage / write (always registered; gated at call time) ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_export",
		Description: "Create a new NFS export. Optional apply=true writes live NFS. Requires nfs:write.",
	}, t.createExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_export",
		Description: "Update an existing NFS export by id. Optional apply=true. Requires nfs:write.",
	}, t.updateExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "patch_export",
		Description: "Quick patch enabled / read-only / description on an export; optionally apply live. Requires nfs:write.",
	}, t.patchExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "clone_export",
		Description: "Clone an export to a new path (new fsid). Optional apply=true. Requires nfs:write.",
	}, t.cloneExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_export",
		Description: "Delete an NFS export by id. Optional apply=true to refresh live NFS. Requires nfs:write.",
	}, t.deleteExport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_exports",
		Description: "Write /etc/exports and run exportfs -ra (make DB exports live). Requires nfs:write.",
	}, t.applyExports)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_quota",
		Description: "Set soft quota bytes / postback URL on an export. Requires nfs:write.",
	}, t.setQuota)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_quotas",
		Description: "Run quota check now (optional postback notify). Requires nfs:write.",
	}, t.checkQuotas)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mkdir",
		Description: "Create a directory under browse roots. Requires nfs:write.",
	}, t.mkdir)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_settings",
		Description: "Update NFS default clients/options / global extras (not password). Requires nfs:write.",
	}, t.updateSettings)

	t.registerResources(server)
	return server
}

type emptyParams struct{}

type rangeParams struct {
	Range string `json:"range,omitempty" jsonschema:"Metrics range preset, e.g. last_7_days, last_24_hours, last_30_days."`
	From  string `json:"from,omitempty" jsonschema:"Optional RFC3339 start when range is custom."`
	To    string `json:"to,omitempty" jsonschema:"Optional RFC3339 end when range is custom."`
}

type idParams struct {
	ID uint `json:"id" jsonschema:"Export database id."`
}

type limitParams struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max log entries to return (default 100)."`
}

type pathParams struct {
	Path string `json:"path" jsonschema:"Absolute path under allowed browse roots."`
}

type exportParams struct {
	ID               uint    `json:"id,omitempty" jsonschema:"Export id (required for update)."`
	Path             string  `json:"path,omitempty" jsonschema:"Absolute export path."`
	Description      *string `json:"description,omitempty"`
	Clients          string  `json:"clients,omitempty" jsonschema:"Client list, e.g. * or CIDRs."`
	AccessMode       string  `json:"accessMode,omitempty" jsonschema:"rw or ro."`
	CommitMode       string  `json:"commitMode,omitempty"`
	WriteDelay       string  `json:"writeDelay,omitempty"`
	SubtreeCheck     string  `json:"subtreeCheck,omitempty"`
	PortMode         string  `json:"portMode,omitempty"`
	LockMode         string  `json:"lockMode,omitempty"`
	HideMode         string  `json:"hideMode,omitempty"`
	Crossmnt         *bool   `json:"crossmnt,omitempty"`
	RootSquash       string  `json:"rootSquash,omitempty"`
	AllSquash        string  `json:"allSquash,omitempty"`
	Security         string  `json:"security,omitempty"`
	Fsid             *uint   `json:"fsid,omitempty"`
	AnonUID          *int    `json:"anonUid,omitempty"`
	AnonGID          *int    `json:"anonGid,omitempty"`
	Enabled          *bool   `json:"enabled,omitempty"`
	QuotaBytes       *int64  `json:"quotaBytes,omitempty"`
	QuotaPostbackURL *string `json:"quotaPostbackUrl,omitempty"`
	ReadOnly         *bool   `json:"readOnly,omitempty"`
	Sync             *bool   `json:"sync,omitempty"`
	Apply            *bool   `json:"apply,omitempty" jsonschema:"If true, apply exports to live NFS after the change."`
}

type patchParams struct {
	ID          uint    `json:"id" jsonschema:"Export id."`
	Enabled     *bool   `json:"enabled,omitempty"`
	AccessMode  *string `json:"accessMode,omitempty" jsonschema:"rw or ro."`
	ReadOnly    *bool   `json:"readOnly,omitempty"`
	Description *string `json:"description,omitempty"`
	Apply       *bool   `json:"apply,omitempty" jsonschema:"Defaults to true when enabled/access change."`
}

type cloneParams struct {
	ID          uint    `json:"id" jsonschema:"Source export id."`
	Path        string  `json:"path" jsonschema:"New absolute path for the clone."`
	Description *string `json:"description,omitempty"`
	Apply       *bool   `json:"apply,omitempty"`
}

type clientsParams struct {
	Clients string `json:"clients" jsonschema:"Clients ACL to validate (* / CIDR / IP)."`
}

type checkQuotasParams struct {
	Notify *bool `json:"notify,omitempty" jsonschema:"If true, fire quota postbacks when full (default true)."`
}

type settingsParams struct {
	DefaultClients     *string `json:"defaultClients,omitempty"`
	DefaultReadOnly    *bool   `json:"defaultReadOnly,omitempty"`
	DefaultSync        *bool   `json:"defaultSync,omitempty"`
	DefaultExtraOpts   *string `json:"defaultExtraOptions,omitempty"`
	GlobalExtraOptions *string `json:"globalExtraOptions,omitempty"`
	SwaggerEnabled     *bool   `json:"swaggerEnabled,omitempty"`
	PanelAllowedCIDRs  *string `json:"panelAllowedCidrs,omitempty"`
}

type quotaParams struct {
	ID               uint    `json:"id" jsonschema:"Export id."`
	QuotaBytes       *int64  `json:"quotaBytes,omitempty" jsonschema:"Soft quota in bytes; 0 = unlimited."`
	QuotaPostbackURL *string `json:"quotaPostbackUrl,omitempty"`
	Unlimited        *bool   `json:"unlimited,omitempty" jsonschema:"If true, clear quota (quotaBytes=0)."`
}

func (t *tools) nfsStatus(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	return textResult(t.NFS.Status())
}

func (t *tools) nfsDashboard(ctx context.Context, _ *mcp.CallToolRequest, params rangeParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return errResult(err)
	}
	enabled := 0
	for _, e := range exports {
		if e.Enabled {
			enabled++
		}
	}
	storage := reports.Collect(exports)
	rangeKey := params.Range
	if rangeKey == "" {
		rangeKey = "last_7_days"
	}
	rangeInfo := metrics.ResolveRange(rangeKey, params.From, params.To, time.Now().UTC())
	var metricsSnap metrics.Snapshot
	if t.Metrics != nil {
		metricsSnap = t.Metrics.SnapshotRange(rangeInfo)
	} else {
		metricsSnap = metrics.Snapshot{Range: rangeInfo, Series: []metrics.Point{}}
	}
	return textResult(map[string]any{
		"status":         t.NFS.Status(),
		"storage":        storage,
		"connections":    nfs.Connections(),
		"metrics":        metricsSnap,
		"logLevels":      t.Log.LevelCounts(),
		"exportsTotal":   len(exports),
		"exportsEnabled": enabled,
		"uptimeSeconds":  int(time.Since(metricsSnap.StartedAt).Seconds()),
	})
}

func (t *tools) listExports(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return errResult(err)
	}
	cfg, _ := t.Settings.LoadAppSettings()
	out := make([]models.ExportDTO, 0, len(exports))
	for _, e := range exports {
		out = append(out, nfs.ToDTO(e, cfg.GlobalExtraOptions))
	}
	return textResult(out)
}

func (t *tools) getExport(ctx context.Context, _ *mcp.CallToolRequest, params idParams) (*mcp.CallToolResult, any, error) {
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
	cfg, _ := t.Settings.LoadAppSettings()
	return textResult(nfs.ToDTO(exp, cfg.GlobalExtraOptions))
}

func (t *tools) storageReport(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return errResult(err)
	}
	return textResult(reports.Collect(exports))
}

func (t *tools) quotaReport(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return errResult(err)
	}
	return textResult(reports.CollectQuotas(exports))
}

func (t *tools) listLogs(ctx context.Context, _ *mcp.CallToolRequest, params limitParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	return textResult(map[string]any{"entries": t.Log.Since(0, limit)})
}

func (t *tools) listInterfaces(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	ifaces, err := netutil.ListInterfaces()
	if err != nil {
		return errResult(err)
	}
	suggestions, _ := netutil.SuggestedClientCIDRs()
	return textResult(map[string]any{"interfaces": ifaces, "suggestions": suggestions})
}

func (t *tools) browsePath(ctx context.Context, _ *mcp.CallToolRequest, params pathParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireRead(ctx); err != nil {
		return errResult(err)
	}
	path := strings.TrimSpace(params.Path)
	if path == "" {
		roots := t.browseRoots()
		path = roots[0]
	}
	listing, err := fsbrowse.List(path, t.browseRoots())
	if err != nil {
		return errResult(err)
	}
	return textResult(listing)
}

func (t *tools) createExport(ctx context.Context, _ *mcp.CallToolRequest, params exportParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	exp, err := mapExportParams(params, true)
	if err != nil {
		return errResult(err)
	}
	if err := os.MkdirAll(exp.Path, 0o755); err != nil {
		return errResult(fmt.Errorf("create path: %w", err))
	}
	if err := t.DB.Create(&exp).Error; err != nil {
		return errResult(err)
	}
	t.Log.Info("mcp", "created export", fmt.Sprintf("id=%d path=%s", exp.ID, exp.Path))
	cfg, _ := t.Settings.LoadAppSettings()
	out := map[string]any{"export": nfs.ToDTO(exp, cfg.GlobalExtraOptions)}
	if params.Apply != nil && *params.Apply {
		if err := t.applyInto(out); err != nil {
			return errResult(err)
		}
	}
	return textResult(out)
}

func (t *tools) updateExport(ctx context.Context, _ *mcp.CallToolRequest, params exportParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if params.ID == 0 {
		return errResult(fmt.Errorf("id is required"))
	}
	var exp models.Export
	if err := t.DB.First(&exp, params.ID).Error; err != nil {
		return errResult(err)
	}
	mapped, err := mapExportParams(params, false)
	if err != nil {
		return errResult(err)
	}
	mapped.ID = exp.ID
	mapped.CreatedAt = exp.CreatedAt
	if params.Description == nil {
		mapped.Description = exp.Description
	}
	if params.Path == "" {
		mapped.Path = exp.Path
	}
	if params.Clients == "" {
		mapped.Clients = exp.Clients
	}
	if params.QuotaBytes == nil {
		mapped.QuotaBytes = exp.QuotaBytes
	}
	if params.QuotaPostbackURL == nil {
		mapped.QuotaPostbackURL = exp.QuotaPostbackURL
	}
	mapped.QuotaNotifiedAt = exp.QuotaNotifiedAt
	if params.QuotaBytes != nil && *params.QuotaBytes != exp.QuotaBytes {
		mapped.QuotaNotifiedAt = nil
	}
	// Preserve fields not provided by applying defaults from existing when empty enums omitted.
	if params.AccessMode == "" && (params.ReadOnly == nil) {
		mapped.AccessMode = exp.AccessMode
	}
	if params.CommitMode == "" && params.Sync == nil {
		mapped.CommitMode = exp.CommitMode
	}
	if params.WriteDelay == "" {
		mapped.WriteDelay = exp.WriteDelay
	}
	if params.SubtreeCheck == "" {
		mapped.SubtreeCheck = exp.SubtreeCheck
	}
	if params.PortMode == "" {
		mapped.PortMode = exp.PortMode
	}
	if params.LockMode == "" {
		mapped.LockMode = exp.LockMode
	}
	if params.HideMode == "" {
		mapped.HideMode = exp.HideMode
	}
	if params.Crossmnt == nil {
		mapped.Crossmnt = exp.Crossmnt
	}
	if params.RootSquash == "" {
		mapped.RootSquash = exp.RootSquash
	}
	if params.AllSquash == "" {
		mapped.AllSquash = exp.AllSquash
	}
	if params.Security == "" {
		mapped.Security = exp.Security
	}
	if params.Fsid == nil {
		mapped.Fsid = exp.Fsid
	}
	if params.AnonUID == nil {
		mapped.AnonUID = exp.AnonUID
	}
	if params.AnonGID == nil {
		mapped.AnonGID = exp.AnonGID
	}
	if params.Enabled == nil {
		mapped.Enabled = exp.Enabled
	}
	if err := nfs.NormalizeExport(&mapped); err != nil {
		return errResult(err)
	}
	if err := t.DB.Save(&mapped).Error; err != nil {
		return errResult(err)
	}
	t.Log.Info("mcp", "updated export", fmt.Sprintf("id=%d path=%s", mapped.ID, mapped.Path))
	cfg, _ := t.Settings.LoadAppSettings()
	out := map[string]any{"export": nfs.ToDTO(mapped, cfg.GlobalExtraOptions)}
	if params.Apply != nil && *params.Apply {
		if err := t.applyInto(out); err != nil {
			return errResult(err)
		}
	}
	return textResult(out)
}

func (t *tools) deleteExport(ctx context.Context, _ *mcp.CallToolRequest, params idParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if params.ID == 0 {
		return errResult(fmt.Errorf("id is required"))
	}
	res := t.DB.Delete(&models.Export{}, params.ID)
	if res.Error != nil {
		return errResult(res.Error)
	}
	if res.RowsAffected == 0 {
		return errResult(fmt.Errorf("export not found"))
	}
	t.Log.Info("mcp", "deleted export", "id="+strconv.FormatUint(uint64(params.ID), 10))
	out := map[string]any{"ok": true, "id": params.ID}
	return textResult(out)
}

func (t *tools) applyInto(out map[string]any) error {
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return err
	}
	cfg, err := t.Settings.LoadAppSettings()
	if err != nil {
		return err
	}
	applied, err := t.NFS.Apply(exports, nfs.ApplyOptions{GlobalExtraOptions: cfg.GlobalExtraOptions})
	status := t.NFS.Status()
	out["applied"] = err == nil
	out["output"] = applied
	out["status"] = status
	if err != nil {
		if t.Metrics != nil {
			t.Metrics.IncApplyFail()
		}
		t.Log.Error("mcp", "exportfs apply failed", err.Error()+"\n"+applied)
		return fmt.Errorf("%w: %s", err, applied)
	}
	if t.Metrics != nil {
		t.Metrics.IncApplyOK()
	}
	t.Log.Success("mcp", "exports applied", applied)
	return nil
}

func (t *tools) applyExports(ctx context.Context, _ *mcp.CallToolRequest, _ emptyParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return errResult(err)
	}
	cfg, err := t.Settings.LoadAppSettings()
	if err != nil {
		return errResult(err)
	}
	out, err := t.NFS.Apply(exports, nfs.ApplyOptions{GlobalExtraOptions: cfg.GlobalExtraOptions})
	status := t.NFS.Status()
	if err != nil {
		if t.Metrics != nil {
			t.Metrics.IncApplyFail()
		}
		t.Log.Error("mcp", "exportfs apply failed", err.Error()+"\n"+out)
		return errResult(fmt.Errorf("%w: %s", err, out))
	}
	if t.Metrics != nil {
		t.Metrics.IncApplyOK()
	}
	t.Log.Success("mcp", "exports applied", out)
	return textResult(map[string]any{"ok": true, "output": out, "status": status})
}

func (t *tools) setQuota(ctx context.Context, _ *mcp.CallToolRequest, params quotaParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if params.ID == 0 {
		return errResult(fmt.Errorf("id is required"))
	}
	var exp models.Export
	if err := t.DB.First(&exp, params.ID).Error; err != nil {
		return errResult(err)
	}
	prevQuota := exp.QuotaBytes
	if params.Unlimited != nil && *params.Unlimited {
		exp.QuotaBytes = 0
	}
	if params.QuotaBytes != nil {
		if *params.QuotaBytes < 0 {
			return errResult(fmt.Errorf("quotaBytes must be >= 0"))
		}
		exp.QuotaBytes = *params.QuotaBytes
	}
	if params.QuotaPostbackURL != nil {
		url := strings.TrimSpace(*params.QuotaPostbackURL)
		if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return errResult(fmt.Errorf("quotaPostbackUrl must be http(s)"))
		}
		exp.QuotaPostbackURL = url
	}
	if exp.QuotaBytes != prevQuota {
		exp.QuotaNotifiedAt = nil
	}
	if err := t.DB.Save(&exp).Error; err != nil {
		return errResult(err)
	}
	t.Log.Info("mcp", "quota updated", fmt.Sprintf("id=%d path=%s quota=%d", exp.ID, exp.Path, exp.QuotaBytes))
	return textResult(map[string]any{"export": exp, "usage": reports.EvaluateExport(exp)})
}

func (t *tools) mkdir(ctx context.Context, _ *mcp.CallToolRequest, params pathParams) (*mcp.CallToolResult, any, error) {
	if err := t.requireWrite(ctx); err != nil {
		return errResult(err)
	}
	if strings.TrimSpace(params.Path) == "" {
		return errResult(fmt.Errorf("path is required"))
	}
	listing, err := fsbrowse.Mkdir(params.Path, t.browseRoots())
	if err != nil {
		return errResult(err)
	}
	t.Log.Info("mcp", "mkdir", listing.Path)
	return textResult(listing)
}

func mapExportParams(in exportParams, create bool) (models.Export, error) {
	exp := models.DefaultExport()
	exp.Path = strings.TrimSpace(in.Path)
	if create && (exp.Path == "" || !strings.HasPrefix(exp.Path, "/")) {
		return exp, fmt.Errorf("path must be an absolute directory")
	}
	if in.Description != nil {
		exp.Description = strings.TrimSpace(*in.Description)
	}
	if in.Clients != "" {
		exp.Clients = in.Clients
	}
	exp.AccessMode = in.AccessMode
	exp.CommitMode = in.CommitMode
	exp.WriteDelay = in.WriteDelay
	exp.SubtreeCheck = in.SubtreeCheck
	exp.PortMode = in.PortMode
	exp.LockMode = in.LockMode
	exp.HideMode = in.HideMode
	if in.Crossmnt != nil {
		exp.Crossmnt = *in.Crossmnt
	}
	exp.RootSquash = in.RootSquash
	exp.AllSquash = in.AllSquash
	exp.Security = in.Security
	if in.Fsid != nil {
		exp.Fsid = *in.Fsid
	}
	exp.AnonUID = in.AnonUID
	exp.AnonGID = in.AnonGID
	if in.QuotaBytes != nil {
		if *in.QuotaBytes < 0 {
			return exp, fmt.Errorf("quotaBytes must be >= 0")
		}
		exp.QuotaBytes = *in.QuotaBytes
	}
	if in.QuotaPostbackURL != nil {
		url := strings.TrimSpace(*in.QuotaPostbackURL)
		if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return exp, fmt.Errorf("quotaPostbackUrl must be http(s)")
		}
		exp.QuotaPostbackURL = url
	}
	if in.AccessMode == "" && in.ReadOnly != nil && *in.ReadOnly {
		exp.AccessMode = "ro"
	}
	if in.CommitMode == "" && in.Sync != nil && *in.Sync {
		exp.CommitMode = "sync"
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	exp.Enabled = enabled
	if err := nfs.NormalizeExport(&exp); err != nil {
		return exp, err
	}
	return exp, nil
}
