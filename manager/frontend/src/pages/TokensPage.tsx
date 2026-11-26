import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react"
import Select from "react-select"
import CreatableSelect from "react-select/creatable"
import { Button } from "@/components/ui/button"
import { DataTable, Td, Th } from "@/components/DataTable"
import { api, type APIToken } from "@/lib/api"
import { selectStyles, selectTheme, type SelectOption } from "@/lib/selectStyles"

const SCOPE_OPTIONS = [
  { value: "exports:read", label: "exports:read" },
  { value: "exports:write", label: "exports:write" },
  { value: "exports:delete", label: "exports:delete" },
  { value: "settings:read", label: "settings:read" },
  { value: "settings:write", label: "settings:write" },
  { value: "logs:read", label: "logs:read" },
  { value: "tokens:admin", label: "tokens:admin" },
]

export function TokensPage() {
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [name, setName] = useState("")
  const [cidrs, setCidrs] = useState<SelectOption[]>([{ value: "*", label: "*" }])
  const [scopes, setScopes] = useState<SelectOption[]>([
    { value: "exports:read", label: "exports:read" },
    { value: "exports:write", label: "exports:write" },
  ])
  const [rawToken, setRawToken] = useState("")
  const [error, setError] = useState("")
  const [message, setMessage] = useState("")
  const [busy, setBusy] = useState(false)

  const refresh = useCallback(async () => {
    const [list, net] = await Promise.all([api.listTokens(), api.network()])
    setTokens(list)
    setSuggestions(net.suggestions || [])
  }, [])

  useEffect(() => {
    refresh().catch((err: Error) => setError(err.message))
  }, [refresh])

  const cidrOptions = useMemo(
    () =>
      ["*", ...suggestions].map((v) => ({
        value: v,
        label: v,
      })),
    [suggestions]
  )

  async function handleCreate(e?: FormEvent | React.MouseEvent) {
    e?.preventDefault()
    setBusy(true)
    setError("")
    setMessage("")
    setRawToken("")
    try {
      const created = await api.createToken({
        name,
        allowedCidrs: cidrs.map((c) => c.value).join(" "),
        scopes: scopes.map((s) => s.value),
      })
      setRawToken(created.rawToken)
      setMessage("Token created. Copy it now — it will not be shown again.")
      setName("")
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : "create failed")
    } finally {
      setBusy(false)
    }
  }

  async function toggleEnabled(tok: APIToken) {
    await api.updateToken(tok.id, { enabled: !tok.enabled })
    await refresh()
  }

  async function remove(id: number) {
    if (!confirm("Delete this API token?")) return
    await api.deleteToken(id)
    await refresh()
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="font-medium">API tokens</h2>
        <p className="text-sm text-muted-foreground">
          Issue Bearer tokens scoped to CIDRs for create/update/delete via API /
          Swagger.
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

      {rawToken ? (
        <pre className="overflow-x-auto rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 font-mono text-xs">
          {rawToken}
        </pre>
      ) : null}

      <form
        onSubmit={(e) => {
          e.preventDefault()
          void handleCreate(e)
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            const t = e.target as HTMLElement
            if (t.closest(".rs__control") || t.tagName === "INPUT") {
              // Let creatable add values; block accidental full-form navigation.
              if (t.tagName === "INPUT" && !t.closest(".rs__control")) {
                e.preventDefault()
              } else {
                e.stopPropagation()
              }
            }
          }
        }}
        className="space-y-4 rounded-xl border border-border p-4"
      >
        <h3 className="font-medium">Create token</h3>
        <label className="block space-y-1.5 text-sm">
          <span>Name</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="h-9 w-full rounded-lg border border-input bg-background px-3"
            required
          />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span>Allowed CIDRs / IPs</span>
          <CreatableSelect
            isMulti
            classNamePrefix="rs"
            styles={selectStyles()}
            theme={selectTheme}
            options={cidrOptions}
            value={cidrs}
            onChange={(v) => setCidrs(v ? [...v] : [])}
            formatCreateLabel={(input) => `Add CIDR “${input}”`}
            placeholder="Type CIDR/IP and press Enter"
            onKeyDown={(e) => {
              if (e.key === "Enter") e.stopPropagation()
            }}
          />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span>Scopes</span>
          <Select
            isMulti
            classNamePrefix="rs"
            styles={selectStyles()}
            theme={selectTheme}
            options={SCOPE_OPTIONS}
            value={scopes}
            onChange={(v) => setScopes(v ? [...v] : [])}
            onKeyDown={(e) => {
              if (e.key === "Enter") e.stopPropagation()
            }}
          />
        </label>
        <Button type="button" disabled={busy} onClick={(e) => void handleCreate(e)}>
          Create token
        </Button>
      </form>

      <DataTable>
        <thead>
          <tr>
            <Th style={{ width: "16%" }}>Name</Th>
            <Th style={{ width: "14%" }}>Prefix</Th>
            <Th style={{ width: "18%" }}>CIDRs</Th>
            <Th style={{ width: "22%" }}>Scopes</Th>
            <Th style={{ width: "10%" }}>Enabled</Th>
            <Th style={{ width: "20%" }}>Actions</Th>
          </tr>
        </thead>
        <tbody>
          {tokens.map((tok) => (
            <tr key={tok.id} className="border-b border-border last:border-0">
              <Td>{tok.name}</Td>
              <Td className="font-mono text-xs">{tok.tokenPrefix}…</Td>
              <Td className="font-mono text-xs">{tok.allowedCidrs}</Td>
              <Td className="text-xs">{tok.scopes}</Td>
              <Td>{tok.enabled ? "yes" : "no"}</Td>
              <Td>
                <div className="flex flex-wrap justify-end gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void toggleEnabled(tok)}
                  >
                    {tok.enabled ? "Disable" : "Enable"}
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => void remove(tok.id)}
                  >
                    Delete
                  </Button>
                </div>
              </Td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  )
}
