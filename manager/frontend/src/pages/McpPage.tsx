import { useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { DataTable, Td, Th } from "@/components/DataTable"
import { Toggle } from "@/components/Toggle"
import {
  api,
  type MCPClient,
  type MCPSession,
  type MCPSettings,
} from "@/lib/api"
import { cn } from "@/lib/utils"

const MCP_SERVER_NAME = "nfs-server"

function formatTs(v?: string | null) {
  if (!v) return "—"
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString()
}

function encodeBase64Json(value: unknown) {
  const json = JSON.stringify(value)
  // ASCII-safe for our URL/config payloads.
  return btoa(json)
}

async function copyText(text: string) {
  await navigator.clipboard.writeText(text)
}

type ClientCard = {
  id: string
  title: string
  blurb: string
  primaryLabel: string
  onPrimary: () => void
  secondaryLabel?: string
  onSecondary?: () => void
}

type SnippetTab = {
  id: string
  label: string
  text: string
}

export function McpPage() {
  const [settings, setSettings] = useState<MCPSettings>({
    enabled: false,
    allowWrite: false,
    publicBaseUrl: "",
  })
  const [sessions, setSessions] = useState<MCPSession[]>([])
  const [clients, setClients] = useState<MCPClient[]>([])
  const [error, setError] = useState("")
  const [message, setMessage] = useState("")
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState("")
  const [snippetTab, setSnippetTab] = useState("cursor")

  const refresh = useCallback(async () => {
    const [cfg, sess, cli] = await Promise.all([
      api.getMCPSettings(),
      api.listMCPSessions(),
      api.listMCPClients(),
    ])
    setSettings(cfg)
    setSessions(sess)
    setClients(cli)
  }, [])

  useEffect(() => {
    refresh().catch((err: Error) => setError(err.message))
  }, [refresh])

  const endpoint = useMemo(() => {
    const base = (settings.publicBaseUrl || window.location.origin).replace(
      /\/$/,
      ""
    )
    return `${base}/mcp`
  }, [settings.publicBaseUrl])

  const cursorConfig = useMemo(() => ({ url: endpoint }), [endpoint])

  const installLinks = useMemo(() => {
    const cursorConfigB64 = encodeURIComponent(encodeBase64Json(cursorConfig))
    const cursorName = encodeURIComponent(MCP_SERVER_NAME)
    const vscodePayload = encodeURIComponent(
      JSON.stringify({
        name: MCP_SERVER_NAME,
        type: "http",
        url: endpoint,
      })
    )
    const vscodeWebConfig = encodeURIComponent(
      JSON.stringify({ type: "http", url: endpoint })
    )
    return {
      cursorDeeplink: `cursor://anysphere.cursor-deeplink/mcp/install?name=${cursorName}&config=${cursorConfigB64}`,
      cursorHttps: `https://cursor.com/install-mcp?name=${cursorName}&config=${cursorConfigB64}`,
      claudeAi: `https://claude.ai/customize/connectors?modal=add-custom-connector&connectorName=${cursorName}&connectorUrl=${encodeURIComponent(endpoint)}`,
      vscode: `vscode:mcp/install?${vscodePayload}`,
      vscodeWeb: `https://vscode.dev/redirect/mcp/install?name=${cursorName}&config=${vscodeWebConfig}`,
      goose: `goose://extension?cmd=none&url=${encodeURIComponent(endpoint)}&id=${cursorName}&name=${cursorName}&description=${encodeURIComponent("NFS Manager MCP")}`,
    }
  }, [cursorConfig, endpoint])

  const snippets = useMemo<SnippetTab[]>(
    () => [
      {
        id: "cursor",
        label: "Cursor",
        text: JSON.stringify(
          { mcpServers: { [MCP_SERVER_NAME]: { url: endpoint } } },
          null,
          2
        ),
      },
      {
        id: "claude-code",
        label: "Claude Code",
        text: [
          `# Add remote MCP (HTTP / Streamable)`,
          `claude mcp add --transport http ${MCP_SERVER_NAME} ${endpoint}`,
          ``,
          `# Or JSON (.mcp.json)`,
          JSON.stringify(
            {
              mcpServers: {
                [MCP_SERVER_NAME]: { type: "http", url: endpoint },
              },
            },
            null,
            2
          ),
        ].join("\n"),
      },
      {
        id: "claude-ai",
        label: "Claude.ai",
        text: [
          `1. Open Claude → Customize → Connectors → Add custom connector`,
          `2. Name: ${MCP_SERVER_NAME}`,
          `3. URL: ${endpoint}`,
          `4. Authorize with username admin + panel password`,
        ].join("\n"),
      },
      {
        id: "codex",
        label: "Codex",
        text: [
          `# CLI`,
          `codex mcp add ${MCP_SERVER_NAME} --url ${endpoint}`,
          ``,
          `# Or ~/.codex/config.toml`,
          `[mcp_servers.${MCP_SERVER_NAME}]`,
          `url = "${endpoint}"`,
        ].join("\n"),
      },
      {
        id: "vscode",
        label: "VS Code",
        text: JSON.stringify(
          {
            servers: {
              [MCP_SERVER_NAME]: { type: "http", url: endpoint },
            },
          },
          null,
          2
        ),
      },
      {
        id: "windsurf",
        label: "Windsurf",
        text: JSON.stringify(
          { mcpServers: { [MCP_SERVER_NAME]: { url: endpoint } } },
          null,
          2
        ),
      },
    ],
    [endpoint]
  )

  const activeSnippet =
    snippets.find((s) => s.id === snippetTab)?.text || snippets[0]?.text || ""

  async function markCopied(key: string, text: string) {
    try {
      await copyText(text)
      setCopied(key)
      window.setTimeout(() => {
        setCopied((cur) => (cur === key ? "" : cur))
      }, 1600)
    } catch {
      setError("Clipboard copy failed")
    }
  }

  const openWithCards: ClientCard[] = [
    {
      id: "cursor",
      title: "Cursor",
      blurb: "Install via deeplink, then Authorize in Cursor.",
      primaryLabel: "Open in Cursor",
      onPrimary: () => {
        window.location.href = installLinks.cursorDeeplink
      },
      secondaryLabel: "Web install",
      onSecondary: () => {
        window.open(installLinks.cursorHttps, "_blank", "noreferrer")
      },
    },
    {
      id: "claude-ai",
      title: "Claude.ai",
      blurb: "Opens Add custom connector with name + URL filled in.",
      primaryLabel: "Open in Claude",
      onPrimary: () => {
        window.open(installLinks.claudeAi, "_blank", "noreferrer")
      },
    },
    {
      id: "claude-code",
      title: "Claude Code",
      blurb: "Copy the CLI command, then Authorize in the terminal UI.",
      primaryLabel: copied === "claude-code-cmd" ? "Copied" : "Copy CLI",
      onPrimary: () => {
        void markCopied(
          "claude-code-cmd",
          `claude mcp add --transport http ${MCP_SERVER_NAME} ${endpoint}`
        )
      },
    },
    {
      id: "codex",
      title: "Codex",
      blurb: "OpenAI Codex CLI / IDE — add remote MCP URL.",
      primaryLabel: copied === "codex-cmd" ? "Copied" : "Copy CLI",
      onPrimary: () => {
        void markCopied(
          "codex-cmd",
          `codex mcp add ${MCP_SERVER_NAME} --url ${endpoint}`
        )
      },
      secondaryLabel: copied === "codex-toml" ? "Copied TOML" : "Copy TOML",
      onSecondary: () => {
        void markCopied(
          "codex-toml",
          `[mcp_servers.${MCP_SERVER_NAME}]\nurl = "${endpoint}"\n`
        )
      },
    },
    {
      id: "vscode",
      title: "VS Code",
      blurb: "GitHub Copilot MCP (HTTP). Works with Insiders too.",
      primaryLabel: "Open in VS Code",
      onPrimary: () => {
        window.location.href = installLinks.vscode
      },
      secondaryLabel: "Web redirect",
      onSecondary: () => {
        window.open(installLinks.vscodeWeb, "_blank", "noreferrer")
      },
    },
    {
      id: "goose",
      title: "Goose",
      blurb: "Block’s Goose desktop extension deeplink.",
      primaryLabel: "Open in Goose",
      onPrimary: () => {
        window.location.href = installLinks.goose
      },
    },
  ]

  async function saveSettings() {
    setBusy(true)
    setError("")
    setMessage("")
    try {
      const saved = await api.saveMCPSettings(settings)
      setSettings(saved)
      setMessage(
        saved.enabled
          ? "MCP enabled. Clients can authorize at /mcp."
          : "MCP disabled."
      )
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed")
    } finally {
      setBusy(false)
    }
  }

  async function revokeSession(id: number) {
    if (!confirm("Revoke this MCP session?")) return
    await api.revokeMCPSession(id)
    await refresh()
  }

  async function deleteClient(id: number) {
    if (!confirm("Delete this OAuth client and revoke its sessions?")) return
    await api.deleteMCPClient(id)
    await refresh()
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="font-medium">MCP</h2>
        <p className="text-sm text-muted-foreground">
          OAuth-protected Model Context Protocol endpoint for AI clients
          (Cursor, Claude, etc.). Off by default. Login:{" "}
          <span className="font-mono">admin</span> + panel password.
        </p>
      </div>

      {(message || error) && (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${
            error ? "border-destructive/40 text-destructive" : "border-border"
          }`}
        >
          {error || message}
        </div>
      )}

      <section className="space-y-4 rounded-xl border border-border p-4">
        <h3 className="font-medium">Configuration</h3>
        <div className="flex flex-wrap gap-6">
          <Toggle
            id="mcp-enabled"
            label={settings.enabled ? "Enabled" : "Disabled"}
            title="Enable Streamable HTTP MCP at /mcp"
            checked={settings.enabled}
            onChange={(next) => setSettings((s) => ({ ...s, enabled: next }))}
          />
          <Toggle
            id="mcp-write"
            label={settings.allowWrite ? "Write tools on" : "Read-only tools"}
            title="Allow create/update/delete export tools (requires nfs:write scope)"
            checked={settings.allowWrite}
            onChange={(next) => setSettings((s) => ({ ...s, allowWrite: next }))}
          />
        </div>
        <label className="block space-y-1.5 text-sm">
          <span>Public base URL</span>
          <input
            value={settings.publicBaseUrl}
            onChange={(e) =>
              setSettings((s) => ({ ...s, publicBaseUrl: e.target.value }))
            }
            placeholder={window.location.origin}
            className="h-9 w-full rounded-lg border border-input bg-background px-3 font-mono text-xs"
          />
          <span className="text-xs text-muted-foreground">
            Must match how AI clients reach this host (used in OAuth discovery).
            Leave empty to derive from the request /{" "}
            <span className="font-mono">MCP_PUBLIC_BASE_URL</span>.
          </span>
        </label>
        <Button type="button" disabled={busy} onClick={() => void saveSettings()}>
          Save MCP settings
        </Button>
      </section>

      <section className="space-y-4">
        <div>
          <h3 className="font-medium">Open with</h3>
          <p className="text-sm text-muted-foreground">
            Endpoint: <span className="font-mono text-xs">{endpoint}</span>
            {!settings.enabled ? (
              <span className="ml-2 text-amber-700 dark:text-amber-300">
                · Enable MCP first so Authorize works
              </span>
            ) : null}
          </p>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {openWithCards.map((card) => (
            <div
              key={card.id}
              className="flex flex-col gap-3 rounded-xl border border-border p-4"
            >
              <div>
                <div className="font-medium">{card.title}</div>
                <p className="mt-1 text-xs text-muted-foreground">{card.blurb}</p>
              </div>
              <div className="mt-auto flex flex-wrap gap-2">
                <Button size="sm" type="button" onClick={card.onPrimary}>
                  {card.primaryLabel}
                </Button>
                {card.secondaryLabel && card.onSecondary ? (
                  <Button
                    size="sm"
                    type="button"
                    variant="outline"
                    onClick={card.onSecondary}
                  >
                    {card.secondaryLabel}
                  </Button>
                ) : null}
              </div>
            </div>
          ))}
        </div>

        <div className="space-y-3 rounded-xl border border-border p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h4 className="text-sm font-medium">Config / commands</h4>
            <Button
              size="sm"
              variant="outline"
              type="button"
              onClick={() => void markCopied(`snippet-${snippetTab}`, activeSnippet)}
            >
              {copied === `snippet-${snippetTab}` ? "Copied" : "Copy"}
            </Button>
          </div>
          <div className="flex flex-wrap gap-2">
            {snippets.map((tab) => (
              <button
                key={tab.id}
                type="button"
                onClick={() => setSnippetTab(tab.id)}
                className={cn(
                  "rounded-lg px-3 py-1.5 text-xs transition-colors",
                  snippetTab === tab.id
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground hover:text-foreground"
                )}
              >
                {tab.label}
              </button>
            ))}
          </div>
          <pre className="overflow-x-auto rounded-lg bg-muted p-3 font-mono text-xs whitespace-pre-wrap">
            {activeSnippet}
          </pre>
          <p className="text-xs text-muted-foreground">
            After connecting, sign in with username{" "}
            <span className="font-mono">admin</span> and the panel password when
            the client shows Authorize.
          </p>
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h3 className="font-medium">Sessions</h3>
          <Button
            size="sm"
            variant="outline"
            onClick={() => void refresh().catch((e: Error) => setError(e.message))}
          >
            Refresh
          </Button>
        </div>
        <DataTable>
          <thead>
            <tr>
              <Th style={{ width: "18%" }}>Client</Th>
              <Th style={{ width: "16%" }}>Scope</Th>
              <Th style={{ width: "18%" }}>Created</Th>
              <Th style={{ width: "18%" }}>Expires</Th>
              <Th style={{ width: "14%" }}>Last used</Th>
              <Th style={{ width: "8%" }}>Status</Th>
              <Th style={{ width: "8%" }}>Actions</Th>
            </tr>
          </thead>
          <tbody>
            {sessions.length === 0 ? (
              <tr>
                <Td colSpan={7} className="py-8 text-center text-muted-foreground">
                  No MCP sessions yet.
                </Td>
              </tr>
            ) : (
              sessions.map((s) => (
                <tr key={s.id} className="border-b border-border last:border-0">
                  <Td>
                    <div className="text-sm">{s.clientName || "—"}</div>
                    <div className="font-mono text-xs text-muted-foreground">
                      {s.clientId}
                    </div>
                  </Td>
                  <Td className="font-mono text-xs">{s.scope || "—"}</Td>
                  <Td className="text-xs">{formatTs(s.createdAt)}</Td>
                  <Td className="text-xs">{formatTs(s.expiresAt)}</Td>
                  <Td className="text-xs">{formatTs(s.lastUsedAt)}</Td>
                  <Td className="text-xs">{s.revoked ? "revoked" : "active"}</Td>
                  <Td>
                    <div className="flex justify-end">
                      <Button
                        size="sm"
                        variant="destructive"
                        disabled={s.revoked}
                        onClick={() => void revokeSession(s.id)}
                      >
                        Revoke
                      </Button>
                    </div>
                  </Td>
                </tr>
              ))
            )}
          </tbody>
        </DataTable>
      </section>

      <section className="space-y-3">
        <h3 className="font-medium">OAuth clients</h3>
        <DataTable>
          <thead>
            <tr>
              <Th style={{ width: "28%" }}>Name</Th>
              <Th style={{ width: "36%" }}>Client ID</Th>
              <Th style={{ width: "20%" }}>Created</Th>
              <Th style={{ width: "16%" }}>Actions</Th>
            </tr>
          </thead>
          <tbody>
            {clients.length === 0 ? (
              <tr>
                <Td colSpan={4} className="py-8 text-center text-muted-foreground">
                  No registered clients.
                </Td>
              </tr>
            ) : (
              clients.map((c) => (
                <tr key={c.id} className="border-b border-border last:border-0">
                  <Td>{c.clientName}</Td>
                  <Td className="font-mono text-xs">{c.clientId}</Td>
                  <Td className="text-xs">{formatTs(c.createdAt)}</Td>
                  <Td>
                    <div className="flex justify-end">
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => void deleteClient(c.id)}
                      >
                        Delete
                      </Button>
                    </div>
                  </Td>
                </tr>
              ))
            )}
          </tbody>
        </DataTable>
      </section>
    </div>
  )
}
