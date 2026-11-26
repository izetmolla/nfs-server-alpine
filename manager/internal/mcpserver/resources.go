package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/fsbrowse"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/nfs"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/reports"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const playbookMarkdown = `# NFS Manager — agent playbook

Use **resources** first for context, then **tools** to act.

## Typical workflows

### Inspect health
1. Read resource ` + "`nfs://guide/playbook`" + ` (this doc) and ` + "`nfs://status`" + `
2. Or call tool ` + "`analyze_nfs`" + ` for warnings + suggested fixes

### List / inspect exports
1. Read ` + "`nfs://exports`" + ` or call ` + "`list_exports`" + `
2. For one export: ` + "`nfs://exports/{id}`" + ` or tool ` + "`get_export`" + `
3. Usage: ` + "`export_usage`" + ` / resource ` + "`nfs://quotas`" + `

### Create a share
1. Read ` + "`nfs://option-catalog`" + ` and ` + "`nfs://interfaces`" + ` for clients CIDRs
2. Optionally ` + "`browse_path`" + ` / ` + "`mkdir`" + ` under browse roots
3. ` + "`preview_export`" + ` to check the exports line
4. ` + "`create_export`" + ` with ` + "`apply: true`" + ` to make it live
5. Confirm with ` + "`nfs_status`" + ` / ` + "`nfs://status`" + `

### Change access quickly
- ` + "`patch_export`" + ` with enabled / readOnly / accessMode and apply
- Or ` + "`update_export`" + ` for full fields

### Clone a share
- ` + "`clone_export`" + ` with new path + optional apply

### Apply pending DB changes
- ` + "`apply_exports`" + ` writes /etc/exports and runs exportfs

### Quotas
- Read ` + "`nfs://quotas`" + ` / ` + "`nfs://storage`" + `
- ` + "`set_quota`" + ` then ` + "`check_quotas`" + `

## Rules
- Paths must be absolute and under browse roots for mkdir/browse
- Exactly one enabled export should use ` + "`fsid=0`" + ` (NFSv4 root)
- Prefer LAN CIDRs over ` + "`*`" + ` for clients in production
- After structural changes, use ` + "`apply: true`" + ` or ` + "`apply_exports`" + `
- Auth scopes: ` + "`nfs:read`" + ` for resources/read tools; ` + "`nfs:write`" + ` for manage tools
`

func (t *tools) registerResources(server *mcp.Server) {
	add := func(uri, name, title, desc, mime string, h mcp.ResourceHandler) {
		server.AddResource(&mcp.Resource{
			URI:         uri,
			Name:        name,
			Title:       title,
			Description: desc,
			MIMEType:    mime,
		}, h)
	}

	add("nfs://guide/playbook", "playbook", "NFS playbook",
		"How to inspect, create, update, clone, apply, and quota NFS exports with this MCP.",
		"text/markdown", t.resourcePlaybook)

	add("nfs://guide/tool-map", "tool-map", "Tool map",
		"Maps common goals to the best MCP tool/resource.",
		"application/json", t.resourceToolMap)

	add("nfs://status", "status", "NFS status",
		"Live nfsd/exportfs status snapshot.",
		"application/json", t.resourceStatus)

	add("nfs://exports", "exports", "All exports",
		"All configured NFS exports from the manager DB.",
		"application/json", t.resourceExports)

	add("nfs://storage", "storage", "Storage report",
		"Per-export folder usage and filesystem capacity.",
		"application/json", t.resourceStorage)

	add("nfs://quotas", "quotas", "Quota report",
		"Per-export soft quota usage.",
		"application/json", t.resourceQuotas)

	add("nfs://interfaces", "interfaces", "Network interfaces",
		"Host interfaces and suggested client CIDRs.",
		"application/json", t.resourceInterfaces)

	add("nfs://settings", "settings", "App settings",
		"NFS defaults and panel settings (no password).",
		"application/json", t.resourceSettings)

	add("nfs://option-catalog", "option-catalog", "Export option catalog",
		"Typed NFS export options for forms/tools.",
		"application/json", t.resourceOptionCatalog)

	add("nfs://logs/recent", "recent-logs", "Recent logs",
		"Last manager log events.",
		"application/json", t.resourceLogs)

	add("nfs://analysis", "analysis", "NFS analysis",
		"Health summary with warnings and suggested actions.",
		"application/json", t.resourceAnalysis)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "nfs://exports/{id}",
		Name:        "export-by-id",
		Title:       "Export by id",
		Description: "Single export JSON by database id.",
		MIMEType:    "application/json",
	}, t.resourceExportByID)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "nfs://fs{+path}",
		Name:        "filesystem-browse",
		Title:       "Browse filesystem",
		Description: "Directory listing for an absolute path under browse roots, e.g. nfs://fs/mnt.",
		MIMEType:    "application/json",
	}, t.resourceBrowse)
}

func (t *tools) textResource(uri, mime, text string) *mcp.ReadResourceResult {
	if mime == "" {
		mime = "text/plain"
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: mime,
			Text:     text,
		}},
	}
}

func (t *tools) jsonResource(uri string, v any) (*mcp.ReadResourceResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return t.textResource(uri, "application/json", string(b)), nil
}

func (t *tools) requireResourceRead(ctx context.Context) error {
	return t.requireRead(ctx)
}

func (t *tools) resourcePlaybook(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	return t.textResource(req.Params.URI, "text/markdown", playbookMarkdown), nil
}

func (t *tools) resourceToolMap(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, map[string]any{
		"inspect":   []string{"nfs://analysis", "analyze_nfs", "nfs://status", "nfs_status"},
		"list":      []string{"nfs://exports", "list_exports", "nfs://exports/{id}", "get_export"},
		"create":    []string{"nfs://option-catalog", "nfs://interfaces", "browse_path", "mkdir", "preview_export", "create_export"},
		"update":    []string{"update_export", "patch_export", "apply_exports"},
		"clone":     []string{"clone_export"},
		"delete":    []string{"delete_export", "apply_exports"},
		"storage":   []string{"nfs://storage", "storage_report", "nfs://quotas", "set_quota", "check_quotas"},
		"network":   []string{"nfs://interfaces", "validate_clients", "list_interfaces"},
		"settings":  []string{"nfs://settings", "get_settings", "update_settings"},
		"troubleshoot": []string{"nfs://logs/recent", "list_logs", "nfs://analysis", "analyze_nfs"},
	})
}

func (t *tools) resourceStatus(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, t.NFS.Status())
}

func (t *tools) resourceExports(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return nil, err
	}
	cfg, _ := t.Settings.LoadAppSettings()
	out := make([]models.ExportDTO, 0, len(exports))
	for _, e := range exports {
		out = append(out, nfs.ToDTO(e, cfg.GlobalExtraOptions))
	}
	return t.jsonResource(req.Params.URI, out)
}

func (t *tools) resourceStorage(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, reports.Collect(exports))
}

func (t *tools) resourceQuotas(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	var exports []models.Export
	if err := t.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, reports.CollectQuotas(exports))
}

func (t *tools) resourceInterfaces(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	ifaces, err := netutil.ListInterfaces()
	if err != nil {
		return nil, err
	}
	suggestions, _ := netutil.SuggestedClientCIDRs()
	return t.jsonResource(req.Params.URI, map[string]any{"interfaces": ifaces, "suggestions": suggestions})
}

func (t *tools) resourceSettings(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	cfg, err := t.Settings.LoadAppSettings()
	if err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, cfg)
}

func (t *tools) resourceOptionCatalog(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, nfs.Catalog())
}

func (t *tools) resourceLogs(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, map[string]any{
		"entries":   t.Log.Since(0, 100),
		"fetchedAt": time.Now().UTC(),
	})
}

func (t *tools) resourceAnalysis(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	data, err := t.buildAnalysis()
	if err != nil {
		return nil, err
	}
	return t.jsonResource(req.Params.URI, data)
}

func (t *tools) resourceExportByID(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	uri := req.Params.URI
	idStr := strings.TrimPrefix(uri, "nfs://exports/")
	if idStr == uri || idStr == "" {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	var exp models.Export
	if err := t.DB.First(&exp, uint(id64)).Error; err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	cfg, _ := t.Settings.LoadAppSettings()
	return t.jsonResource(uri, nfs.ToDTO(exp, cfg.GlobalExtraOptions))
}

func (t *tools) resourceBrowse(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if err := t.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	uri := req.Params.URI
	path := strings.TrimPrefix(uri, "nfs://fs")
	if path == uri {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("path must be absolute")
	}
	listing, err := fsbrowse.List(path, t.browseRoots())
	if err != nil {
		return nil, err
	}
	return t.jsonResource(uri, listing)
}
