import { useCallback, useEffect, useState, type FormEvent } from "react"
import { Button } from "@/components/ui/button"
import { api, setToken, type AppSettings, type Status } from "@/lib/api"
import { validateClientsInput } from "@/lib/ip"

const emptySettings: AppSettings = {
  defaultClients: "*",
  defaultReadOnly: false,
  defaultSync: false,
  defaultExtraOptions: "",
  globalExtraOptions: "",
  swaggerEnabled: true,
  panelAllowedCidrs: "0.0.0.0/0",
  hasCustomPassword: false,
}

export function SettingsPage() {
  const [settings, setSettings] = useState<AppSettings>(emptySettings)
  const [status, setStatus] = useState<Status | null>(null)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [message, setMessage] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")

  const refresh = useCallback(async () => {
    setBusy(true)
    setError("")
    try {
      const [cfg, st, net] = await Promise.all([
        api.getSettings(),
        api.status(),
        api.network(),
      ])
      setSettings(cfg)
      setStatus(st)
      setSuggestions(net.suggestions || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load settings")
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    const clientsCheck = validateClientsInput(settings.defaultClients)
    if (!clientsCheck.ok) {
      setError(clientsCheck.error || "invalid default clients")
      return
    }
    setBusy(true)
    setError("")
    setMessage("")
    try {
      const firewallCheck = validateClientsInput(settings.panelAllowedCidrs)
      if (!firewallCheck.ok) {
        setError(firewallCheck.error || "invalid panel firewall CIDRs")
        return
      }
      const saved = await api.saveSettings({
        defaultClients: clientsCheck.normalized,
        defaultReadOnly: settings.defaultReadOnly,
        defaultSync: settings.defaultSync,
        defaultExtraOptions: settings.defaultExtraOptions,
        globalExtraOptions: settings.globalExtraOptions,
        swaggerEnabled: settings.swaggerEnabled,
        panelAllowedCidrs: firewallCheck.normalized,
      })
      setSettings(saved)
      setMessage("Settings saved")
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed")
    } finally {
      setBusy(false)
    }
  }

  async function handleApply() {
    setBusy(true)
    setError("")
    setMessage("")
    try {
      const res = await api.applySettings()
      setSettings(res.settings)
      setStatus(res.status)
      setMessage(res.output || "Settings applied to NFS exports")
    } catch (err) {
      setError(err instanceof Error ? err.message : "apply failed")
    } finally {
      setBusy(false)
    }
  }

  async function handlePassword(e: FormEvent) {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      setError("New password confirmation does not match")
      return
    }
    setBusy(true)
    setError("")
    setMessage("")
    try {
      const res = await api.changePassword(currentPassword, newPassword)
      setToken(res.token)
      setCurrentPassword("")
      setNewPassword("")
      setConfirmPassword("")
      setMessage("Password updated. Session token refreshed.")
      const cfg = await api.getSettings()
      setSettings(cfg)
    } catch (err) {
      setError(err instanceof Error ? err.message : "password change failed")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-medium">Settings</h2>
          <p className="text-sm text-muted-foreground">
            Change panel password and NFS defaults, then store or apply.
          </p>
        </div>
        <Button variant="outline" onClick={() => void refresh()} disabled={busy}>
          Refresh
        </Button>
      </div>

      {(message || error) && (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${
            error
              ? "border-destructive/40 text-destructive"
              : "border-border text-foreground"
          }`}
        >
          {error || message}
        </div>
      )}

      <form
        onSubmit={handlePassword}
        className="space-y-4 rounded-xl border border-border p-4"
      >
        <div>
          <h3 className="font-medium">Panel password</h3>
          <p className="text-sm text-muted-foreground">
            {settings.hasCustomPassword
              ? "Using a password stored in SQLite (overrides PANEL_PASSWORD)."
              : "Currently using PANEL_PASSWORD from the environment."}
          </p>
        </div>
        <div className="grid gap-4 sm:grid-cols-3">
          <label className="space-y-1.5 text-sm">
            <span>Current password</span>
            <input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              className="h-9 w-full rounded-lg border border-input bg-background px-3 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              required
            />
          </label>
          <label className="space-y-1.5 text-sm">
            <span>New password</span>
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="h-9 w-full rounded-lg border border-input bg-background px-3 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              required
              minLength={4}
            />
          </label>
          <label className="space-y-1.5 text-sm">
            <span>Confirm new password</span>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="h-9 w-full rounded-lg border border-input bg-background px-3 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              required
              minLength={4}
            />
          </label>
        </div>
        <Button type="submit" disabled={busy}>
          Change password
        </Button>
      </form>

      <form
        onSubmit={handleSave}
        className="space-y-4 rounded-xl border border-border p-4"
      >
        <div>
          <h3 className="font-medium">Panel access & API docs</h3>
          <p className="text-sm text-muted-foreground">
            Control who can reach this panel and whether Swagger UI is published.
          </p>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={settings.swaggerEnabled}
            onChange={(e) =>
              setSettings({ ...settings, swaggerEnabled: e.target.checked })
            }
          />
          Enable Swagger UI at <code className="ms-1 font-mono text-xs">/swagger</code>
          <span className="text-muted-foreground">(default on)</span>
        </label>
        <label className="block space-y-1.5 text-sm">
          <span>Panel firewall — allowed IPs / CIDRs</span>
          <input
            value={settings.panelAllowedCidrs}
            onChange={(e) =>
              setSettings({ ...settings, panelAllowedCidrs: e.target.value })
            }
            className="h-9 w-full rounded-lg border border-input bg-background px-3 font-mono text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            placeholder="0.0.0.0/0"
          />
          <span className="text-xs text-muted-foreground">
            Default <code className="font-mono">0.0.0.0/0</code> allows every
            IPv4 client. Narrow to your management LAN (e.g.{" "}
            <code className="font-mono">192.168.200.0/24</code>) to lock the
            panel. Denied requests are logged and counted on the dashboard.
          </span>
          {suggestions.length > 0 ? (
            <div className="flex flex-wrap gap-2 pt-1">
              <button
                type="button"
                className="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted"
                onClick={() =>
                  setSettings({ ...settings, panelAllowedCidrs: "0.0.0.0/0" })
                }
              >
                0.0.0.0/0
              </button>
              {suggestions.map((cidr) => (
                <button
                  key={`fw-${cidr}`}
                  type="button"
                  className="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted"
                  onClick={() =>
                    setSettings({ ...settings, panelAllowedCidrs: cidr })
                  }
                >
                  {cidr}
                </button>
              ))}
            </div>
          ) : null}
        </label>
        <Button type="submit" disabled={busy}>
          Store panel settings
        </Button>
      </form>

      <form
        onSubmit={handleSave}
        className="space-y-4 rounded-xl border border-border p-4"
      >
        <div>
          <h3 className="font-medium">Generic NFS config</h3>
          <p className="text-sm text-muted-foreground">
            Defaults for new storages and global options appended on apply.
          </p>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <label className="space-y-1.5 text-sm sm:col-span-2">
            <span>Default clients</span>
            <input
              value={settings.defaultClients}
              onChange={(e) =>
                setSettings({ ...settings, defaultClients: e.target.value })
              }
              className="h-9 w-full rounded-lg border border-input bg-background px-3 font-mono text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              placeholder="* or 192.168.200.0/24"
            />
            {suggestions.length > 0 ? (
              <div className="flex flex-wrap gap-2 pt-2">
                {suggestions.map((cidr) => (
                  <button
                    key={cidr}
                    type="button"
                    className="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted"
                    onClick={() =>
                      setSettings({ ...settings, defaultClients: cidr })
                    }
                  >
                    {cidr}
                  </button>
                ))}
                <button
                  type="button"
                  className="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted"
                  onClick={() =>
                    setSettings({ ...settings, defaultClients: "*" })
                  }
                >
                  *
                </button>
              </div>
            ) : null}
          </label>

          <label className="space-y-1.5 text-sm sm:col-span-2">
            <span>Global extra options</span>
            <input
              value={settings.globalExtraOptions}
              onChange={(e) =>
                setSettings({ ...settings, globalExtraOptions: e.target.value })
              }
              className="h-9 w-full rounded-lg border border-input bg-background px-3 font-mono text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              placeholder="crossmnt,nohide"
            />
            <span className="text-xs text-muted-foreground">
              Comma-separated options appended to every export on Apply.
            </span>
          </label>

          <label className="space-y-1.5 text-sm sm:col-span-2">
            <span>Default extra options (new storages)</span>
            <input
              value={settings.defaultExtraOptions}
              onChange={(e) =>
                setSettings({
                  ...settings,
                  defaultExtraOptions: e.target.value,
                })
              }
              className="h-9 w-full rounded-lg border border-input bg-background px-3 font-mono text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              placeholder="optional notes for operators"
            />
          </label>

          <div className="flex flex-wrap items-center gap-4 text-sm sm:col-span-2">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={settings.defaultReadOnly}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    defaultReadOnly: e.target.checked,
                  })
                }
              />
              Default read-only
            </label>
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={settings.defaultSync}
                onChange={(e) =>
                  setSettings({ ...settings, defaultSync: e.target.checked })
                }
              />
              Default sync
            </label>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={busy}>
            Store settings
          </Button>
          <Button
            type="button"
            variant="secondary"
            disabled={busy}
            onClick={() => void handleApply()}
          >
            Apply to NFS
          </Button>
        </div>
      </form>

      <section className="rounded-xl border border-border p-4 text-sm">
        <h3 className="font-medium">NFS apply state</h3>
        <dl className="mt-3 grid gap-3 sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">mountd</dt>
            <dd className="mt-1 font-medium">
              {status?.mountdRunning ? "running" : "not detected"}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Last error</dt>
            <dd className="mt-1 font-medium break-all">
              {status?.lastError || "none"}
            </dd>
          </div>
        </dl>
        {status?.lastOutput ? (
          <pre className="mt-4 overflow-x-auto rounded-lg bg-muted p-3 text-xs whitespace-pre-wrap">
            {status.lastOutput}
          </pre>
        ) : null}
      </section>
    </div>
  )
}
