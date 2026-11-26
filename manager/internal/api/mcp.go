package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"gorm.io/gorm"
)

func (h *Handler) registerMCPAdmin(api fiber.Router) {
	api.Get("/mcp/settings", h.Auth.RequirePanel(), h.GetMCPSettings)
	api.Put("/mcp/settings", h.Auth.RequirePanel(), h.SaveMCPSettings)
	api.Get("/mcp/sessions", h.Auth.RequirePanel(), h.ListMCPSessions)
	api.Delete("/mcp/sessions/:id", h.Auth.RequirePanel(), h.RevokeMCPSession)
	api.Get("/mcp/clients", h.Auth.RequirePanel(), h.ListMCPClients)
	api.Delete("/mcp/clients/:id", h.Auth.RequirePanel(), h.DeleteMCPClient)
}

func (h *Handler) GetMCPSettings(c fiber.Ctx) error {
	cfg, err := h.Settings.LoadMCPSettings()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(cfg)
}

func (h *Handler) SaveMCPSettings(c fiber.Ctx) error {
	var in models.MCPSettings
	if err := c.Bind().JSON(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	cfg, err := h.Settings.SaveMCPSettings(in)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("mcp", "mcp settings saved", "enabled="+strconv.FormatBool(cfg.Enabled)+" write="+strconv.FormatBool(cfg.AllowWrite))
	return c.JSON(cfg)
}

func (h *Handler) ListMCPSessions(c fiber.Ctx) error {
	if h.MCPStore == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "mcp store unavailable"})
	}
	items, err := h.MCPStore.ListSessions()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(items)
}

func (h *Handler) RevokeMCPSession(c fiber.Ctx) error {
	if h.MCPStore == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "mcp store unavailable"})
	}
	id64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.MCPStore.RevokeToken(uint(id64)); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("mcp", "session revoked", "id="+strconv.FormatUint(id64, 10))
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListMCPClients(c fiber.Ctx) error {
	if h.MCPStore == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "mcp store unavailable"})
	}
	items, err := h.MCPStore.ListClients()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	// Hide secrets in list response.
	type clientView struct {
		ID           uint     `json:"id"`
		ClientID     string   `json:"clientId"`
		ClientName   string   `json:"clientName"`
		RedirectURIs []string `json:"redirectUris"`
		CreatedAt    any      `json:"createdAt"`
	}
	out := make([]clientView, 0, len(items))
	for _, item := range items {
		out = append(out, clientView{
			ID:           item.ID,
			ClientID:     item.ClientID,
			ClientName:   item.ClientName,
			RedirectURIs: item.RedirectURIList(),
			CreatedAt:    item.CreatedAt,
		})
	}
	return c.JSON(out)
}

func (h *Handler) DeleteMCPClient(c fiber.Ctx) error {
	if h.MCPStore == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "mcp store unavailable"})
	}
	id64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.MCPStore.DeleteClient(uint(id64)); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("mcp", "client deleted", "id="+strconv.FormatUint(id64, 10))
	return c.SendStatus(fiber.StatusNoContent)
}
