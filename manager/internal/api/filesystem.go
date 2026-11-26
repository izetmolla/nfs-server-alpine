package api

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/fsbrowse"
)

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

func (h *Handler) StatFilesystem(c fiber.Ctx) error {
	path := c.Query("path")
	if strings.TrimSpace(path) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path is required"})
	}
	entry, err := fsbrowse.Stat(path, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entry)
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

func (h *Handler) TouchFilesystem(c fiber.Ctx) error {
	var body struct {
		Path string `json:"path"`
	}
	if err := c.Bind().JSON(&body); err != nil || strings.TrimSpace(body.Path) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path is required"})
	}
	entry, err := fsbrowse.Touch(body.Path, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("filesystem", "touch", entry.Path)
	return c.Status(fiber.StatusCreated).JSON(entry)
}

func (h *Handler) RenameFilesystem(c fiber.Ctx) error {
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := c.Bind().JSON(&body); err != nil || strings.TrimSpace(body.From) == "" || strings.TrimSpace(body.To) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from and to are required"})
	}
	entry, err := fsbrowse.Rename(body.From, body.To, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("filesystem", "rename", fmt.Sprintf("%s -> %s", body.From, body.To))
	return c.JSON(entry)
}

func (h *Handler) MoveFilesystem(c fiber.Ctx) error {
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := c.Bind().JSON(&body); err != nil || strings.TrimSpace(body.From) == "" || strings.TrimSpace(body.To) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from and to are required"})
	}
	entry, err := fsbrowse.Move(body.From, body.To, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("filesystem", "move", fmt.Sprintf("%s -> %s", body.From, body.To))
	return c.JSON(entry)
}

func (h *Handler) CopyFilesystem(c fiber.Ctx) error {
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := c.Bind().JSON(&body); err != nil || strings.TrimSpace(body.From) == "" || strings.TrimSpace(body.To) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from and to are required"})
	}
	entry, err := fsbrowse.Copy(body.From, body.To, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("filesystem", "copy", fmt.Sprintf("%s -> %s", body.From, body.To))
	return c.JSON(entry)
}

func (h *Handler) DeleteFilesystem(c fiber.Ctx) error {
	var body struct {
		Path  string   `json:"path"`
		Paths []string `json:"paths"`
	}
	_ = c.Bind().JSON(&body)
	paths := append([]string{}, body.Paths...)
	if strings.TrimSpace(body.Path) != "" {
		paths = append(paths, body.Path)
	}
	if q := strings.TrimSpace(c.Query("path")); q != "" {
		paths = append(paths, q)
	}
	if len(paths) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path or paths required"})
	}
	for _, p := range paths {
		if err := fsbrowse.Delete(p, h.browseRoots()); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		h.Log.Info("filesystem", "delete", p)
	}
	return c.JSON(fiber.Map{"ok": true, "deleted": len(paths)})
}

func (h *Handler) UploadFilesystem(c fiber.Ctx) error {
	dir := strings.TrimSpace(c.FormValue("path"))
	if dir == "" {
		dir = strings.TrimSpace(c.FormValue("dir"))
	}
	if dir == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path (destination directory) is required"})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}
	name := filepath.Base(file.Filename)
	if name == "" || name == "." || name == ".." {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid filename"})
	}
	dest := filepath.Join(dir, name)
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	defer src.Close()
	entry, err := fsbrowse.SaveUpload(dest, src, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.Log.Info("filesystem", "upload", entry.Path)
	return c.Status(fiber.StatusCreated).JSON(entry)
}

func (h *Handler) DownloadFilesystem(c fiber.Ctx) error {
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path is required"})
	}
	clean, err := fsbrowse.Resolve(path, h.browseRoots())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	info, err := os.Stat(clean)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if info.IsDir() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot download a directory"})
	}
	ctype := mime.TypeByExtension(filepath.Ext(clean))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	c.Set("Content-Type", ctype)
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(clean)))
	return c.SendFile(clean)
}
