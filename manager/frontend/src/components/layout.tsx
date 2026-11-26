import { useEffect, useState } from "react"
import { NavLink, Outlet } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"

const links = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/storages", label: "Storages" },
  { to: "/files", label: "Files" },
  { to: "/reports", label: "Reports" },
  { to: "/logs", label: "Logs" },
  { to: "/tokens", label: "Tokens" },
  { to: "/mcp-admin", label: "MCP" },
  { to: "/users", label: "Users" },
  { to: "/settings", label: "Settings" },
]

export function AppLayout() {
  const { logout } = useAuth()
  const [swaggerEnabled, setSwaggerEnabled] = useState(true)

  useEffect(() => {
    api
      .publicConfig()
      .then((cfg) => setSwaggerEnabled(cfg.swaggerEnabled))
      .catch(() => setSwaggerEnabled(true))
  }, [])

  return (
    <div className="mx-auto flex min-h-svh w-full max-w-7xl flex-col gap-6 p-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-medium">NFS Manager</h1>
          <p className="text-sm text-muted-foreground">
            Manage NFS exports, review status, and panel settings.
          </p>
        </div>
        <div className="flex gap-2">
          {swaggerEnabled ? (
            <a
              href="/swagger"
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-8 items-center rounded-lg border border-border px-3 text-sm hover:bg-muted"
            >
              Swagger
            </a>
          ) : null}
          <Button variant="secondary" onClick={() => void logout()}>
            Logout
          </Button>
        </div>
      </header>

      <nav className="flex flex-wrap gap-2 border-b border-border pb-3">
        {links.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            end={link.end}
            className={({ isActive }) =>
              cn(
                "rounded-lg px-3 py-1.5 text-sm transition-colors",
                isActive
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )
            }
          >
            {link.label}
          </NavLink>
        ))}
      </nav>

      <Outlet />
    </div>
  )
}
