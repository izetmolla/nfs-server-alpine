import { useCallback, useEffect, useMemo, useState } from "react"
import CreatableSelect from "react-select/creatable"
import { Button } from "@/components/ui/button"
import { Dialog } from "@/components/Dialog"
import { api, type FsListing } from "@/lib/api"
import { selectStyles, selectTheme, type SelectOption } from "@/lib/selectStyles"

type Props = {
  value: string
  onChange: (path: string) => void
}

export function PathBrowser({ value, onChange }: Props) {
  const [open, setOpen] = useState(false)
  const [listing, setListing] = useState<FsListing | null>(null)
  const [cwd, setCwd] = useState(value || "/mnt")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [newName, setNewName] = useState<SelectOption | null>(null)

  const load = useCallback(async (path: string) => {
    setBusy(true)
    setError("")
    try {
      const data = await api.listFilesystem(path)
      setListing(data)
      setCwd(data.path)
    } catch (err) {
      setError(err instanceof Error ? err.message : "browse failed")
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    if (!open) return
    void load(value || "/mnt")
  }, [open, value, load])

  const folders = useMemo(
    () => (listing?.entries || []).filter((e) => e.isDir),
    [listing]
  )

  const folderOptions = useMemo(
    () =>
      folders.map((e) => ({
        value: e.path,
        label: e.name,
      })),
    [folders]
  )

  async function createFolder(name: string) {
    const base = listing?.path || cwd
    const next = `${base.replace(/\/$/, "")}/${name.replace(/^\/+|\/+$/g, "")}`
    setBusy(true)
    setError("")
    try {
      const data = await api.mkdirFilesystem(next)
      setListing(data)
      setCwd(data.path)
      setNewName(null)
      onChange(data.path)
    } catch (err) {
      setError(err instanceof Error ? err.message : "mkdir failed")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.preventDefault()
          }}
          className="h-9 min-w-0 flex-1 rounded-lg border border-input bg-background px-3 font-mono text-xs"
          required
        />
        <Button type="button" variant="outline" onClick={() => setOpen(true)}>
          Browse…
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        On Apply, missing folders are created automatically. Browse to pick an
        existing directory or create a new one under the shared volume.
      </p>

      <Dialog
        open={open}
        title="Choose export directory"
        onClose={() => setOpen(false)}
        confirmLabel="Use this path"
        cancelLabel="Cancel"
        onConfirm={() => {
          onChange(cwd)
          setOpen(false)
        }}
      >
        <div className="space-y-3">
          {error ? (
            <div className="rounded-lg border border-destructive/40 px-3 py-2 text-destructive">
              {error}
            </div>
          ) : null}

          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className="text-muted-foreground">Allowed roots:</span>
            {(listing?.allowedRoots || []).map((root) => (
              <button
                key={root}
                type="button"
                className="rounded-md border border-border px-2 py-0.5 font-mono hover:bg-muted"
                onClick={() => void load(root)}
              >
                {root}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={!listing?.parent || busy}
              onClick={() => listing?.parent && void load(listing.parent)}
            >
              Up
            </Button>
            <code className="truncate rounded-md bg-muted px-2 py-1 text-xs">
              {cwd}
            </code>
            {!listing?.exists ? (
              <span className="text-xs text-amber-600 dark:text-amber-400">
                does not exist yet
              </span>
            ) : null}
          </div>

          <div className="max-h-48 overflow-y-auto rounded-lg border border-border">
            {folders.length === 0 ? (
              <p className="px-3 py-4 text-xs text-muted-foreground">
                {busy ? "Loading…" : "No subfolders here."}
              </p>
            ) : (
              <ul>
                {folders.map((entry) => (
                  <li key={entry.path}>
                    <button
                      type="button"
                      className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-muted"
                      onClick={() => void load(entry.path)}
                      onDoubleClick={() => {
                        onChange(entry.path)
                        setOpen(false)
                      }}
                    >
                      <span className="font-mono text-xs">{entry.name}/</span>
                      <span className="text-xs text-muted-foreground">open</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="space-y-1.5">
            <p className="text-xs font-medium">Create or jump to folder</p>
            <CreatableSelect
              classNamePrefix="rs"
              styles={selectStyles()}
              theme={selectTheme}
              isClearable
              isDisabled={busy}
              options={folderOptions}
              value={newName}
              onChange={(opt) => {
                const o = opt as SelectOption | null
                setNewName(o)
                if (o?.value.startsWith("/")) void load(o.value)
              }}
              onCreateOption={(input) => void createFolder(input)}
              formatCreateLabel={(input) => `Create “${input}” under ${cwd}`}
              placeholder="Type a new folder name…"
              onKeyDown={(e) => {
                if (e.key === "Enter") e.stopPropagation()
              }}
            />
            <p className="text-xs text-muted-foreground">
              Type a name and press Enter / choose Create to mkdir under the
              current path, or select an existing child.
            </p>
          </div>
        </div>
      </Dialog>
    </div>
  )
}
