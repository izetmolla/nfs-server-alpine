package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/api"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/applog"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/auth"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/db"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/mcpoauth"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/mcpserver"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/metrics"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/nfs"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/quota"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/settings"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/tokens"
	"github.com/izetmolla/nfs-server-alpine/manager/web"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// @title NFS Manager API
// @version 1.0
// @description Management API for NFSv4 exports, reports, settings, and tokens.
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	password := os.Getenv("PANEL_PASSWORD")
	if strings.TrimSpace(password) == "" {
		log.Fatal("PANEL_PASSWORD is required")
	}

	addr := envOr("PANEL_ADDR", ":3000")
	dbPath := envOr("MANAGER_DB", "/var/lib/nfs-manager/manager.db")
	exportsFile := envOr("EXPORTS_FILE", "/etc/exports")

	gdb, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := db.SeedFromEnv(gdb); err != nil {
		log.Fatalf("seed: %v", err)
	}

	settingsStore := &settings.Store{DB: gdb}
	if err := settingsStore.ApplyMCPEnv(); err != nil {
		log.Fatalf("mcp env: %v", err)
	}
	tokenStore := &tokens.Store{DB: gdb}
	effectivePassword, err := settingsStore.LoadPassword(password)
	if err != nil {
		log.Fatalf("load password: %v", err)
	}

	ring := applog.New(1000)
	ring.Info("system", "nfs-manager starting", "db="+dbPath)
	collector := metrics.New(gdb)
	quotaWatcher := quota.NewWatcher(gdb, ring, 60*time.Second)
	quotaWatcher.Start()

	guard := auth.New(auth.Config{Password: effectivePassword}, tokenStore)
	nfsSvc := &nfs.Service{ExportsFile: exportsFile}
	oauthState := mcpoauth.NewState(
		gdb,
		mcpoauth.AdminPasswordChecker(guard.CheckPassword),
		func() string {
			cfg, err := settingsStore.LoadMCPSettings()
			if err != nil {
				return ""
			}
			return cfg.PublicBaseURL
		},
	)
	mcpDeps := mcpserver.Deps{
		DB:          gdb,
		NFS:         nfsSvc,
		Settings:    settingsStore,
		Log:         ring,
		Metrics:     collector,
		Quotas:      quotaWatcher,
		BrowseRoots: fsbrowseRoots(),
	}
	handler := &api.Handler{
		DB:          gdb,
		NFS:         nfsSvc,
		Auth:        guard,
		Settings:    settingsStore,
		Tokens:      tokenStore,
		MCPStore:    oauthState.Store,
		Log:         ring,
		Metrics:     collector,
		Quotas:      quotaWatcher,
		BrowseRoots: fsbrowseRoots(),
	}

	app := fiber.New(fiber.Config{AppName: "nfs-manager"})
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"MCP-Protocol-Version", "Mcp-Session-Id", "Last-Event-ID",
		},
		ExposeHeaders: []string{"WWW-Authenticate", "MCP-Session-Id"},
	}))

	// Mount MCP/OAuth before panel firewall so AI clients are not CIDR-blocked when MCP is on.
	mountMCP(app, oauthState, mcpDeps, settingsStore)
	handler.Register(app)
	mountSwagger(app, settingsStore)
	mountFrontend(app)

	log.Printf("nfs-manager listening on %s", addr)
	ring.Info("system", "listening", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}

func mountMCP(app *fiber.App, oauthState *mcpoauth.State, deps mcpserver.Deps, store *settings.Store) {
	mux := http.NewServeMux()
	mcpoauth.Mount(mux, oauthState)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpserver.NewServer(deps)
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	// CORS + bearer gate so unauthenticated initialize returns 401 + WWW-Authenticate
	// (Cursor needs that challenge to show the Authorize button).
	protected := mcpCORS(mcpoauth.RequireBearerToken(oauthState, mcpHandler))
	mux.Handle("/mcp", protected)
	mux.Handle("/mcp/", protected)

	httpHandler := adaptor.HTTPHandler(mux)
	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if !mcpoauth.IsMCPPath(path) {
			return c.Next()
		}
		cfg, err := store.LoadMCPSettings()
		if err != nil || !cfg.Enabled {
			return c.Status(fiber.StatusNotFound).SendString("not found")
		}
		// Ensure Fiber also exposes the challenge header for clients that read it
		// from the outer response.
		c.Set("Access-Control-Expose-Headers", "WWW-Authenticate, MCP-Session-Id")
		return httpHandler(c)
	})
}

func mcpCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, MCP-Session-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func mountSwagger(app *fiber.App, store *settings.Store) {
	spec, err := web.SwaggerJSON()
	if err != nil {
		log.Printf("swagger spec missing: %v", err)
		return
	}

	guard := func(c fiber.Ctx) error {
		cfg, err := store.LoadAppSettings()
		if err != nil || !cfg.SwaggerEnabled {
			return c.Status(fiber.StatusNotFound).SendString("swagger disabled")
		}
		return c.Next()
	}

	app.Get("/swagger.json", guard, func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		return c.Send(spec)
	})
	app.Get("/swagger", guard, func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(swaggerUIHTML)
	})
	app.Get("/swagger/*", guard, func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(swaggerUIHTML)
	})
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>NFS Manager API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/swagger.json',
      dom_id: '#swagger-ui',
      deepLinking: true,
      persistAuthorization: true
    });
  </script>
</body>
</html>`

func mountFrontend(app *fiber.App) {
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatalf("embed frontend: %v", err)
	}

	app.Use("/", static.New("", static.Config{
		FS:     sub,
		Browse: false,
	}))

	app.Get("/*", func(c fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/swagger") ||
			mcpoauth.IsMCPPath(path) {
			return c.Status(fiber.StatusNotFound).SendString("not found")
		}
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("frontend missing")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(data)
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fsbrowseRoots() []string {
	if v := os.Getenv("BROWSE_ROOTS"); v != "" {
		return splitRoots(v)
	}
	if v := os.Getenv("SHARED_DIRECTORY"); v != "" {
		return splitRoots(v)
	}
	return []string{"/mnt"}
}

func splitRoots(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ':' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || !strings.HasPrefix(p, "/") {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return []string{"/mnt"}
	}
	return out
}
