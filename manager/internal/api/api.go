package api

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/applog"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/auth"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/fsbrowse"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/metrics"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/nfs"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/quota"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/reports"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/settings"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/tokens"
	"gorm.io/gorm"
)

type Handler struct {
	DB          *gorm.DB
	NFS         *nfs.Service
	Auth        *auth.Guard
	Settings    *settings.Store
	Tokens      *tokens.Store
	Log         *applog.Ring
	Metrics     *metrics.Collector
	Quotas      *quota.Watcher
	BrowseRoots []string
}

type exportInput struct {
	Path             string  `json:"path"`
	Description      *string `json:"description"`
	Clients          string  `json:"clients"`
	AccessMode       string  `json:"accessMode"`
	CommitMode       string  `json:"commitMode"`
	WriteDelay       string  `json:"writeDelay"`
	SubtreeCheck     string  `json:"subtreeCheck"`
	PortMode         string  `json:"portMode"`
	LockMode         string  `json:"lockMode"`
	HideMode         string  `json:"hideMode"`
	Crossmnt         bool    `json:"crossmnt"`
	RootSquash       string  `json:"rootSquash"`
	AllSquash        string  `json:"allSquash"`
	Security         string  `json:"security"`
	Fsid             uint    `json:"fsid"`
	AnonUID          *int    `json:"anonUid"`
	AnonGID          *int    `json:"anonGid"`
	Enabled          *bool   `json:"enabled"`
	QuotaBytes       *int64  `json:"quotaBytes"`
	QuotaPostbackURL *string `json:"quotaPostbackUrl"`
	// legacy aliases
	ReadOnly bool `json:"readOnly"`
	Sync     bool `json:"sync"`
}

func (h *Handler) Register(app *fiber.App) {
	app.Use(h.PanelFirewall())
	app.Use(h.Auth.MiddlewareSession())
	app.Use(h.TrackAPI())

	api := app.Group("/api")
	api.Get("/public-config", h.PublicConfig)
	api.Post("/login", h.Login)
	api.Post("/logout", h.Logout)

	api.Get("/dashboard", h.Auth.RequireAuth(models.ScopeExportsRead), h.Dashboard)
	api.Get("/status", h.Auth.RequireAuth(models.ScopeExportsRead), h.Status)
	api.Get("/exports/option-catalog", h.Auth.RequireAuth(models.ScopeExportsRead), h.OptionCatalog)
	api.Post("/exports/preview", h.Auth.RequireAuth(models.ScopeExportsRead), h.PreviewExport)
	api.Get("/exports", h.Auth.RequireAuth(models.ScopeExportsRead), h.ListExports)
	api.Get("/exports/:id", h.Auth.RequireAuth(models.ScopeExportsRead), h.GetExport)
	api.Post("/exports", h.Auth.RequireAuth(models.ScopeExportsWrite), h.CreateExport)
	api.Put("/exports/:id", h.Auth.RequireAuth(models.ScopeExportsWrite), h.UpdateExport)
	api.Patch("/exports/:id", h.Auth.RequireAuth(models.ScopeExportsWrite), h.PatchExport)
	api.Post("/exports/:id/clone", h.Auth.RequireAuth(models.ScopeExportsWrite), h.CloneExport)
	api.Delete("/exports/:id", h.Auth.RequireAuth(models.ScopeExportsDelete), h.DeleteExport)
	api.Post("/exports/apply", h.Auth.RequireAuth(models.ScopeExportsWrite), h.ApplyExports)
	api.Get("/exports/:id/usage", h.Auth.RequireAuth(models.ScopeExportsRead), h.ExportUsage)
	api.Put("/exports/:id/quota", h.Auth.RequireAuth(models.ScopeExportsWrite), h.UpdateExportQuota)

	api.Get("/network/interfaces", h.Auth.RequireAuth(models.ScopeExportsRead), h.NetworkInterfaces)
	api.Post("/network/validate-clients", h.Auth.RequireAuth(models.ScopeExportsRead), h.ValidateClients)

	api.Get("/logs", h.Auth.RequireAuth(models.ScopeLogsRead), h.ListLogs)
	api.Get("/logs/stream", h.Auth.RequireAuth(models.ScopeLogsRead), h.StreamLogs)
	api.Delete("/logs", h.Auth.RequirePanel(), h.ClearLogs)

	api.Get("/settings", h.Auth.RequireAuth(models.ScopeSettingsRead), h.GetSettings)
	api.Put("/settings", h.Auth.RequireAuth(models.ScopeSettingsWrite), h.SaveSettings)
	api.Post("/settings/apply", h.Auth.RequireAuth(models.ScopeSettingsWrite), h.ApplySettings)
	api.Post("/settings/password", h.Auth.RequirePanel(), h.ChangePassword)

	api.Get("/reports/storage", h.Auth.RequireAuth(models.ScopeExportsRead), h.StorageReport)
	api.Get("/reports/quotas", h.Auth.RequireAuth(models.ScopeExportsRead), h.QuotaReport)
	api.Post("/reports/quotas/check", h.Auth.RequireAuth(models.ScopeExportsWrite), h.QuotaCheck)

	api.Get("/filesystem", h.Auth.RequireAuth(models.ScopeExportsRead), h.ListFilesystem)
	api.Post("/filesystem/mkdir", h.Auth.RequireAuth(models.ScopeExportsWrite), h.MkdirFilesystem)

	api.Get("/tokens", h.Auth.RequirePanel(), h.ListTokens)
	api.Post("/tokens", h.Auth.RequirePanel(), h.CreateToken)
	api.Patch("/tokens/:id", h.Auth.RequirePanel(), h.UpdateToken)
	api.Delete("/tokens/:id", h.Auth.RequirePanel(), h.DeleteToken)
}

func (h *Handler) PanelFirewall() fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := h.Settings.LoadAppSettings()
		if err != nil {
			return c.Next()
		}
		ip := clientIP(c)
		if netutil.IPAllowed(ip, cfg.PanelAllowedCIDRs) {
			return c.Next()
		}
		if h.Metrics != nil {
			h.Metrics.IncFirewallDeny()
		}
		h.Log.Warn("firewall", "panel access denied", ip+" not in "+cfg.PanelAllowedCIDRs)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "your IP is not allowed to access this panel",
			"ip":    ip,
		})
	}
}

func (h *Handler) TrackAPI() fiber.Handler {
	return func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api") && h.Metrics != nil {
			h.Metrics.IncAPIRequest()
		}
		return c.Next()
	}
}

func (h *Handler) PublicConfig(c fiber.Ctx) error {
	cfg, err := h.Settings.PublicConfig()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(cfg)
}

func (h *Handler) Login(c fiber.Ctx) error {
	var body struct {
		Password string `json:"password"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if !h.Auth.CheckPassword(body.Password) {
		if h.Metrics != nil {
			h.Metrics.IncAuthFail()
		}
		h.Log.Warn("auth", "login failed", clientIP(c))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
	}
	if err := h.Auth.Login(c); err != nil {
		return err
	}
	if h.Metrics != nil {
		h.Metrics.IncAuthOK()
	}
	h.Log.Success("auth", "login success", clientIP(c))
	return c.JSON(fiber.Map{"ok": true, "token": body.Password})
}

func (h *Handler) Logout(c fiber.Ctx) error {
	_ = h.Auth.Logout(c)
	h.Log.Info("auth", "logout", clientIP(c))
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) Status(c fiber.Ctx) error {
	return c.JSON(h.NFS.Status())
}

func (h *Handler) Dashboard(c fiber.Ctx) error {
	var exports []models.Export
	if err := h.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	enabled := 0
	for _, e := range exports {
		if e.Enabled {
			enabled++
		}
	}
	storage := reports.Collect(exports)
	rangeInfo := metrics.ResolveRange(
		c.Query("range", "last_7_days"),
		c.Query("from"),
		c.Query("to"),
		time.Now().UTC(),
	)
	var metricsSnap metrics.Snapshot
	if h.Metrics != nil {
		metricsSnap = h.Metrics.SnapshotRange(rangeInfo)
	} else {
		metricsSnap = metrics.Snapshot{Range: rangeInfo, Series: []metrics.Point{}}
	}
	tokenCount := 0
	if list, err := h.Tokens.List(); err == nil {
		tokenCount = len(list)
	}
	return c.JSON(fiber.Map{
		"status":         h.NFS.Status(),
		"storage":        storage,
		"connections":    nfs.Connections(),
		"metrics":        metricsSnap,
		"logLevels":      h.Log.LevelCounts(),
		"exportsTotal":   len(exports),
		"exportsEnabled": enabled,
		"tokens":         tokenCount,
		"uptimeSeconds":  int(time.Since(metricsSnap.StartedAt).Seconds()),
	})
}

func (h *Handler) OptionCatalog(c fiber.Ctx) error {
	return c.JSON(nfs.Catalog())
}

func (h *Handler) PreviewExport(c fiber.Ctx) error {
	var in exportInput
	if err := c.Bind().JSON(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	exp, err := h.mapInput(in)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	cfg, _ := h.Settings.LoadAppSettings()
	line, err := nfs.BuildExportLine(exp, cfg.GlobalExtraOptions)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"exportLine": line, "export": exp})
}

func (h *Handler) ListExports(c fiber.Ctx) error {
	var exports []models.Export
	if err := h.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	cfg, _ := h.Settings.LoadAppSettings()
	out := make([]models.ExportDTO, 0, len(exports))
	for _, e := range exports {
		out = append(out, nfs.ToDTO(e, cfg.GlobalExtraOptions))
	}
	return c.JSON(out)
}

func (h *Handler) GetExport(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var exp models.Export
	if err := h.DB.First(&exp, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	cfg, _ := h.Settings.LoadAppSettings()
	return c.JSON(nfs.ToDTO(exp, cfg.GlobalExtraOptions))
}

func (h *Handler) CreateExport(c fiber.Ctx) error {
	var in exportInput
	if err := c.Bind().JSON(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	exp, err := h.mapInput(in)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := h.DB.Create(&exp).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("exports", "created export", exportLogRef(exp))
	cfg, _ := h.Settings.LoadAppSettings()
	return c.Status(fiber.StatusCreated).JSON(nfs.ToDTO(exp, cfg.GlobalExtraOptions))
}

func (h *Handler) UpdateExport(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var exp models.Export
	if err := h.DB.First(&exp, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var in exportInput
	if err := c.Bind().JSON(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	mapped, err := h.mapInput(in)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	mapped.ID = exp.ID
	mapped.CreatedAt = exp.CreatedAt
	if in.Description == nil {
		mapped.Description = exp.Description
	}
	// Preserve quota unless explicitly provided in this request.
	if in.QuotaBytes == nil {
		mapped.QuotaBytes = exp.QuotaBytes
	}
	if in.QuotaPostbackURL == nil {
		mapped.QuotaPostbackURL = exp.QuotaPostbackURL
	}
	mapped.QuotaNotifiedAt = exp.QuotaNotifiedAt
	if in.QuotaBytes != nil && *in.QuotaBytes != exp.QuotaBytes {
		mapped.QuotaNotifiedAt = nil
	}
	if err := h.DB.Save(&mapped).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("exports", "updated export", exportLogRef(mapped))
	cfg, _ := h.Settings.LoadAppSettings()
	return c.JSON(nfs.ToDTO(mapped, cfg.GlobalExtraOptions))
}

// PatchExport updates enabled / accessMode / description and optionally applies live NFS exports.
func (h *Handler) PatchExport(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var exp models.Export
	if err := h.DB.First(&exp, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var body struct {
		Enabled     *bool   `json:"enabled"`
		AccessMode  *string `json:"accessMode"`
		ReadOnly    *bool   `json:"readOnly"`
		Description *string `json:"description"`
		Apply       *bool   `json:"apply"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Enabled == nil && body.AccessMode == nil && body.ReadOnly == nil && body.Description == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no fields to patch"})
	}

	changedLive := false
	if body.Description != nil {
		exp.Description = strings.TrimSpace(*body.Description)
	}
	if body.Enabled != nil && *body.Enabled != exp.Enabled {
		exp.Enabled = *body.Enabled
		changedLive = true
	}
	if body.ReadOnly != nil {
		next := "rw"
		if *body.ReadOnly {
			next = "ro"
		}
		if next != exp.AccessMode {
			exp.AccessMode = next
			changedLive = true
		}
	}
	if body.AccessMode != nil {
		mode := strings.TrimSpace(*body.AccessMode)
		if mode != "rw" && mode != "ro" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "accessMode must be rw or ro"})
		}
		if mode != exp.AccessMode {
			exp.AccessMode = mode
			changedLive = true
		}
	}
	if err := nfs.NormalizeExport(&exp); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := h.DB.Save(&exp).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	apply := changedLive
	if body.Apply != nil {
		apply = *body.Apply
	}

	cfg, _ := h.Settings.LoadAppSettings()
	resp := fiber.Map{"export": nfs.ToDTO(exp, cfg.GlobalExtraOptions)}

	if body.Enabled != nil {
		state := "disabled"
		if exp.Enabled {
			state = "enabled"
		}
		h.Log.Info("exports", "storage "+state, exportLogRef(exp))
	}
	if body.ReadOnly != nil || body.AccessMode != nil {
		h.Log.Info("exports", "storage access "+exp.AccessMode, exportLogRef(exp))
	}
	if body.Description != nil && !changedLive {
		h.Log.Info("exports", "storage description updated", exportLogRef(exp))
	}

	if apply {
		out, status, err := h.applyAllExports()
		resp["applied"] = err == nil
		resp["output"] = out
		resp["status"] = status
		if err != nil {
			h.Log.Error("nfs", "exportfs apply failed after patch", err.Error()+"\n"+out)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":  err.Error(),
				"output": out,
				"status": status,
				"export": nfs.ToDTO(exp, cfg.GlobalExtraOptions),
			})
		}
		h.Log.Success("nfs", "exports applied after storage patch", exportLogRef(exp))
	}

	return c.JSON(resp)
}

func (h *Handler) CloneExport(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var src models.Export
	if err := h.DB.First(&src, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var body struct {
		Path        string  `json:"path"`
		Description *string `json:"description"`
		Apply       *bool   `json:"apply"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	path := strings.TrimSpace(body.Path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path must be an absolute directory"})
	}
	if path == src.Path {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clone path must differ from the source"})
	}

	var existing int64
	if err := h.DB.Model(&models.Export{}).Where("path = ?", path).Count(&existing).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if existing > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "an export with this path already exists"})
	}

	clone := src
	clone.ID = 0
	clone.Path = path
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.QuotaNotifiedAt = nil
	if body.Description != nil {
		clone.Description = strings.TrimSpace(*body.Description)
	} else if strings.TrimSpace(src.Description) != "" {
		clone.Description = "Clone of " + src.Description
	} else {
		clone.Description = "Clone of " + src.Path
	}
	// Clones must not reuse fsid=0; pick the next free positive fsid.
	clone.Fsid = nextExportFsid(h.DB, src.Fsid)
	if err := nfs.NormalizeExport(&clone); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := h.DB.Create(&clone).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("exports", "cloned export", fmt.Sprintf("from=%d %s → id=%d %s", src.ID, exportLogRef(src), clone.ID, exportLogRef(clone)))

	cfg, _ := h.Settings.LoadAppSettings()
	resp := fiber.Map{"export": nfs.ToDTO(clone, cfg.GlobalExtraOptions)}
	apply := false
	if body.Apply != nil {
		apply = *body.Apply
	}
	if apply {
		out, status, err := h.applyAllExports()
		resp["applied"] = err == nil
		resp["output"] = out
		resp["status"] = status
		if err != nil {
			h.Log.Error("nfs", "exportfs apply failed after clone", err.Error()+"\n"+out)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":  err.Error(),
				"output": out,
				"status": status,
				"export": nfs.ToDTO(clone, cfg.GlobalExtraOptions),
			})
		}
		h.Log.Success("nfs", "exports applied after clone", exportLogRef(clone))
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
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

func (h *Handler) DeleteExport(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	res := h.DB.Delete(&models.Export{}, id)
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	h.Log.Info("exports", "deleted export", fmt.Sprintf("id=%d", id))
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ApplyExports(c fiber.Ctx) error {
	out, status, err := h.applyAllExports()
	if err != nil {
		h.Log.Error("nfs", "exportfs apply failed", err.Error()+"\n"+out)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "output": out, "status": status})
	}
	h.Log.Success("nfs", "exports applied", out)
	return c.JSON(fiber.Map{"ok": true, "output": out, "status": status})
}

func (h *Handler) applyAllExports() (string, nfs.Status, error) {
	var exports []models.Export
	if err := h.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return "", h.NFS.Status(), err
	}
	cfg, err := h.Settings.LoadAppSettings()
	if err != nil {
		return "", h.NFS.Status(), err
	}
	out, err := h.NFS.Apply(exports, nfs.ApplyOptions{GlobalExtraOptions: cfg.GlobalExtraOptions})
	if err != nil {
		if h.Metrics != nil {
			h.Metrics.IncApplyFail()
		}
		return out, h.NFS.Status(), err
	}
	if h.Metrics != nil {
		h.Metrics.IncApplyOK()
	}
	return out, h.NFS.Status(), nil
}

func exportLogRef(exp models.Export) string {
	ref := fmt.Sprintf("id=%d path=%s", exp.ID, exp.Path)
	if strings.TrimSpace(exp.Description) != "" {
		ref += " note=" + strings.TrimSpace(exp.Description)
	}
	return ref
}

func (h *Handler) NetworkInterfaces(c fiber.Ctx) error {
	ifaces, err := netutil.ListInterfaces()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	suggestions, _ := netutil.SuggestedClientCIDRs()
	return c.JSON(fiber.Map{"interfaces": ifaces, "suggestions": suggestions})
}

func (h *Handler) ValidateClients(c fiber.Ctx) error {
	var body struct {
		Clients string `json:"clients"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	normalized, err := netutil.NormalizeAndValidateClients(body.Clients)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "valid": false})
	}
	return c.JSON(fiber.Map{"valid": true, "clients": normalized})
}

func (h *Handler) ListLogs(c fiber.Ctx) error {
	afterID, _ := strconv.ParseUint(c.Query("after", "0"), 10, 64)
	// Legacy live-tail mode when only after/limit provided without page.
	if c.Query("page") == "" && c.Query("q") == "" && c.Query("level") == "" && c.Query("sort") == "" {
		limit, _ := strconv.Atoi(c.Query("limit", "200"))
		return c.JSON(fiber.Map{"entries": h.Log.Since(afterID, limit)})
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "25"))
	return c.JSON(h.Log.Query(applog.QueryParams{
		AfterID:  afterID,
		Level:    c.Query("level"),
		Source:   c.Query("source"),
		Search:   c.Query("q"),
		Sort:     c.Query("sort", "time"),
		Order:    c.Query("order", "desc"),
		Page:     page,
		PageSize: pageSize,
	}))
}

func (h *Handler) StreamLogs(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	afterID, _ := strconv.ParseUint(c.Query("after", "0"), 10, 64)
	entries := h.Log.Since(afterID, 200)
	for _, e := range entries {
		if _, err := fmt.Fprintf(c, "data: {\"id\":%d,\"time\":%q,\"level\":%q,\"source\":%q,\"message\":%q,\"detail\":%q}\n\n",
			e.ID, e.Time.Format(time.RFC3339Nano), e.Level, e.Source, e.Message, e.Detail); err != nil {
			return nil
		}
		afterID = e.ID
	}
	return nil
}

func (h *Handler) ClearLogs(c fiber.Ctx) error {
	h.Log.Clear()
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) GetSettings(c fiber.Ctx) error {
	cfg, err := h.Settings.LoadAppSettings()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(cfg)
}

func (h *Handler) SaveSettings(c fiber.Ctx) error {
	var in models.AppSettings
	if err := c.Bind().JSON(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	cfg, err := h.Settings.SaveAppSettings(in)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("settings", "settings saved", cfg.DefaultClients)
	return c.JSON(cfg)
}

func (h *Handler) ApplySettings(c fiber.Ctx) error {
	cfg, err := h.Settings.LoadAppSettings()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var exports []models.Export
	if err := h.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	out, err := h.NFS.Apply(exports, nfs.ApplyOptions{GlobalExtraOptions: cfg.GlobalExtraOptions})
	if err != nil {
		if h.Metrics != nil {
			h.Metrics.IncApplyFail()
		}
		h.Log.Error("settings", "apply failed", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "output": out, "settings": cfg, "status": h.NFS.Status()})
	}
	if h.Metrics != nil {
		h.Metrics.IncApplyOK()
	}
	h.Log.Success("settings", "settings applied", out)
	return c.JSON(fiber.Map{"ok": true, "output": out, "settings": cfg, "status": h.NFS.Status()})
}

func (h *Handler) ChangePassword(c fiber.Ctx) error {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if len(strings.TrimSpace(body.NewPassword)) < 4 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "new password must be at least 4 characters"})
	}
	if !h.Auth.CheckPassword(body.CurrentPassword) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "current password is incorrect"})
	}
	if err := h.Settings.SavePassword(body.NewPassword); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Auth.SetPassword(body.NewPassword)
	h.Log.Info("auth", "panel password changed", clientIP(c))
	return c.JSON(fiber.Map{"ok": true, "token": body.NewPassword})
}

func (h *Handler) StorageReport(c fiber.Ctx) error {
	var exports []models.Export
	if err := h.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(reports.Collect(exports))
}

func (h *Handler) QuotaReport(c fiber.Ctx) error {
	var exports []models.Export
	if err := h.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(reports.CollectQuotas(exports))
}

func (h *Handler) QuotaCheck(c fiber.Ctx) error {
	if h.Quotas == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "quota watcher unavailable"})
	}
	report, err := h.Quotas.Check(true)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "report": report})
}

func (h *Handler) ExportUsage(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var exp models.Export
	if err := h.DB.First(&exp, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(reports.EvaluateExport(exp))
}

func (h *Handler) UpdateExportQuota(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var exp models.Export
	if err := h.DB.First(&exp, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var body struct {
		QuotaBytes       *int64  `json:"quotaBytes"`
		QuotaPostbackURL *string `json:"quotaPostbackUrl"`
		Unlimited        *bool   `json:"unlimited"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	prevQuota := exp.QuotaBytes
	if body.Unlimited != nil && *body.Unlimited {
		exp.QuotaBytes = 0
	}
	if body.QuotaBytes != nil {
		if *body.QuotaBytes < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "quotaBytes must be >= 0 (0 = unlimited)"})
		}
		exp.QuotaBytes = *body.QuotaBytes
	}
	if body.QuotaPostbackURL != nil {
		url := strings.TrimSpace(*body.QuotaPostbackURL)
		if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "quotaPostbackUrl must be http(s)"})
		}
		exp.QuotaPostbackURL = url
	}
	if exp.QuotaBytes != prevQuota {
		exp.QuotaNotifiedAt = nil
	}
	if err := h.DB.Save(&exp).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("quota", "quota updated", fmt.Sprintf("id=%d path=%s quota=%d", exp.ID, exp.Path, exp.QuotaBytes))
	usage := reports.EvaluateExport(exp)
	return c.JSON(fiber.Map{"export": exp, "usage": usage})
}

func (h *Handler) browseRoots() []string {
	if len(h.BrowseRoots) > 0 {
		return h.BrowseRoots
	}
	return fsbrowse.RootsFromEnv(os.Getenv("BROWSE_ROOTS"))
}

func (h *Handler) ListFilesystem(c fiber.Ctx) error {
	path := c.Query("path")
	if strings.TrimSpace(path) == "" {
		path = h.browseRoots()[0]
	}
	listing, err := fsbrowse.List(path, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(listing)
}

func (h *Handler) MkdirFilesystem(c fiber.Ctx) error {
	var body struct {
		Path string `json:"path"`
	}
	if err := c.Bind().JSON(&body); err != nil || strings.TrimSpace(body.Path) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path is required"})
	}
	listing, err := fsbrowse.Mkdir(body.Path, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("filesystem", "mkdir", listing.Path)
	return c.Status(fiber.StatusCreated).JSON(listing)
}

func (h *Handler) ListTokens(c fiber.Ctx) error {
	items, err := h.Tokens.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(items)
}

func (h *Handler) CreateToken(c fiber.Ctx) error {
	var in tokens.CreateInput
	if err := c.Bind().JSON(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	created, err := h.Tokens.Create(in)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("tokens", "token created", created.Token.Name)
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *Handler) UpdateToken(c fiber.Ctx) error {
	id64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var body struct {
		Name         string     `json:"name"`
		AllowedCIDRs string     `json:"allowedCidrs"`
		Scopes       []string   `json:"scopes"`
		Enabled      *bool      `json:"enabled"`
		ExpiresAt    *time.Time `json:"expiresAt"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	item, err := h.Tokens.Update(uint(id64), body.Enabled, body.Name, body.AllowedCIDRs, body.Scopes, body.ExpiresAt)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(item)
}

func (h *Handler) DeleteToken(c fiber.Ctx) error {
	id64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.Tokens.Delete(uint(id64)); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("tokens", "token deleted", fmt.Sprintf("id=%d", id64))
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) mapInput(in exportInput) (models.Export, error) {
	exp := models.DefaultExport()
	exp.Path = strings.TrimSpace(in.Path)
	if in.Description != nil {
		exp.Description = strings.TrimSpace(*in.Description)
	}
	exp.Clients = in.Clients
	exp.AccessMode = in.AccessMode
	exp.CommitMode = in.CommitMode
	exp.WriteDelay = in.WriteDelay
	exp.SubtreeCheck = in.SubtreeCheck
	exp.PortMode = in.PortMode
	exp.LockMode = in.LockMode
	exp.HideMode = in.HideMode
	exp.Crossmnt = in.Crossmnt
	exp.RootSquash = in.RootSquash
	exp.AllSquash = in.AllSquash
	exp.Security = in.Security
	exp.Fsid = in.Fsid
	exp.AnonUID = in.AnonUID
	exp.AnonGID = in.AnonGID
	if in.QuotaBytes != nil {
		if *in.QuotaBytes < 0 {
			return exp, fmt.Errorf("quotaBytes must be >= 0 (0 = unlimited)")
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
	if in.AccessMode == "" && in.ReadOnly {
		exp.AccessMode = "ro"
	}
	if in.CommitMode == "" && in.Sync {
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

func clientIP(c fiber.Ctx) string {
	ip := c.IP()
	if ip == "" {
		return "unknown"
	}
	return ip
}
