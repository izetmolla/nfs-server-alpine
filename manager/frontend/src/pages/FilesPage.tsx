import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from "react"
import {
  ArrowLeft,
  ArrowUp,
  ChevronRight,
  File,
  FilePlus,
  Folder,
  FolderOpen,
  FolderPlus,
  HardDrive,
  RefreshCw,
  Trash2,
  Upload,
} from "lucide-react"
import { ContextMenu, ExplorerShell, type ContextMenuItem } from "@/components/ContextMenu"
import { Dialog } from "@/components/Dialog"
import { Button } from "@/components/ui/button"
import { api, type FsEntry, type FsListing } from "@/lib/api"
import { cn } from "@/lib/utils"

type Clipboard =
  | { mode: "copy" | "cut"; entries: FsEntry[] }
  | null

type MenuState = { x: number; y: number; target: FsEntry | null } | null

type PromptState =
  | { kind: "mkdir" | "touch" | "rename"; seed: string; target?: FsEntry }
  | { kind: "delete"; targets: FsEntry[] }
  | { kind: "properties"; target: FsEntry }
  | null

function joinPath(dir: string, name: string) {
  return `${dir.replace(/\/$/, "")}/${name.replace(/^\/+|\/+$/g, "")}`
}

function formatBytes(n: number) {
  if (!Number.isFinite(n) || n < 0) return "—"
  if (n < 1024) return `${n} bytes`
  const units = ["KB", "MB", "GB", "TB"]
  let v = n
  let i = -1
  do {
    v /= 1024
    i++
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

function formatDate(iso: string) {
  if (!iso) return "—"
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function entryType(e: FsEntry) {
  if (e.isDir) return "File folder"
  const ext = e.name.includes(".") ? e.name.split(".").pop()?.toUpperCase() : ""
  return ext ? `${ext} File` : "File"
}

function uniqueName(base: string, existing: Set<string>) {
  if (!existing.has(base.toLowerCase())) return base
  const dot = base.lastIndexOf(".")
  const stem = dot > 0 ? base.slice(0, dot) : base
  const ext = dot > 0 ? base.slice(dot) : ""
  let i = 2
  while (existing.has(`${stem} (${i})${ext}`.toLowerCase())) i++
  return `${stem} (${i})${ext}`
}

export function FilesPage() {
  const [listing, setListing] = useState<FsListing | null>(null)
  const [cwd, setCwd] = useState("/mnt")
  const [address, setAddress] = useState("/mnt")
  const historyRef = useRef<string[]>(["/mnt"])
  const histIdxRef = useRef(0)
  const [histIdx, setHistIdx] = useState(0)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [status, setStatus] = useState("")
  const [clipboard, setClipboard] = useState<Clipboard>(null)
  const [menu, setMenu] = useState<MenuState>(null)
  const [prompt, setPrompt] = useState<PromptState>(null)
  const [promptValue, setPromptValue] = useState("")
  const [treeOpen, setTreeOpen] = useState<Record<string, boolean>>({ "/mnt": true })
  const [treeChildren, setTreeChildren] = useState<Record<string, FsEntry[]>>({})
  const [dragOver, setDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const paneRef = useRef<HTMLDivElement>(null)
  const lastClickRef = useRef<string | null>(null)

  const entries = listing?.entries || []
  const roots = listing?.allowedRoots || ["/mnt"]
  const writable = listing?.writable ?? false

  const load = useCallback(async (path: string, opts?: { pushHistory?: boolean }) => {
    setBusy(true)
    setError("")
    try {
      const data = await api.listFilesystem(path)
      setListing(data)
      setCwd(data.path)
      setAddress(data.path)
      setSelected(new Set())
      setStatus(
        `${data.entries.length} item${data.entries.length === 1 ? "" : "s"}`
      )
      if (opts?.pushHistory !== false) {
        const prev = historyRef.current.slice(0, histIdxRef.current + 1)
        if (prev[prev.length - 1] !== data.path) {
          historyRef.current = [...prev, data.path]
          histIdxRef.current = historyRef.current.length - 1
          setHistIdx(histIdxRef.current)
        }
      }
      setTreeChildren((prev) => ({
        ...prev,
        [data.path]: data.entries.filter((e) => e.isDir),
      }))
      setTreeOpen((prev) => ({ ...prev, [data.path]: true }))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to open folder")
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    void load("/mnt", { pushHistory: false })
  }, [load])

  const selectedEntries = useMemo(
    () => entries.filter((e) => selected.has(e.path)),
    [entries, selected]
  )

  const navigateTo = (path: string) => {
    void load(path)
  }

  const goBack = () => {
    if (histIdxRef.current <= 0) return
    const nextIdx = histIdxRef.current - 1
    histIdxRef.current = nextIdx
    setHistIdx(nextIdx)
    void load(historyRef.current[nextIdx], { pushHistory: false })
  }

  const goUp = () => {
    if (listing?.parent) void load(listing.parent)
  }

  const refresh = () => void load(cwd, { pushHistory: false })

  const toggleSelect = (entry: FsEntry, e: ReactMouseEvent) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (e.metaKey || e.ctrlKey) {
        if (next.has(entry.path)) next.delete(entry.path)
        else next.add(entry.path)
        return next
      }
      if (e.shiftKey && lastClickRef.current) {
        const paths = entries.map((x) => x.path)
        const a = paths.indexOf(lastClickRef.current)
        const b = paths.indexOf(entry.path)
        if (a >= 0 && b >= 0) {
          const [lo, hi] = a < b ? [a, b] : [b, a]
          for (let i = lo; i <= hi; i++) next.add(paths[i])
          return next
        }
      }
      return new Set([entry.path])
    })
    lastClickRef.current = entry.path
  }

  const openEntry = (entry: FsEntry) => {
    if (entry.isDir) navigateTo(entry.path)
    else void api.downloadFilesystem(entry.path).catch((err) => {
      setError(err instanceof Error ? err.message : "Download failed")
    })
  }

  const ensureTree = async (path: string) => {
    if (treeChildren[path]) return
    try {
      const data = await api.listFilesystem(path)
      setTreeChildren((prev) => ({
        ...prev,
        [path]: data.entries.filter((e) => e.isDir),
      }))
    } catch {
      /* ignore tree expand errors */
    }
  }

  const runPrompt = async () => {
    if (!prompt) return
    setBusy(true)
    setError("")
    try {
      if (prompt.kind === "mkdir") {
        const name = promptValue.trim()
        if (!name) return
        await api.mkdirFilesystem(joinPath(cwd, name))
        setStatus(`Created folder “${name}”`)
      } else if (prompt.kind === "touch") {
        const name = promptValue.trim()
        if (!name) return
        await api.touchFilesystem(joinPath(cwd, name))
        setStatus(`Created file “${name}”`)
      } else if (prompt.kind === "rename" && prompt.target) {
        const name = promptValue.trim()
        if (!name) return
        const dest = joinPath(cwd, name)
        await api.renameFilesystem(prompt.target.path, dest)
        setStatus(`Renamed to “${name}”`)
      } else if (prompt.kind === "delete") {
        await api.deleteFilesystem(prompt.targets.map((t) => t.path))
        setStatus(`Deleted ${prompt.targets.length} item(s)`)
      }
      setPrompt(null)
      await load(cwd, { pushHistory: false })
    } catch (err) {
      setError(err instanceof Error ? err.message : "Operation failed")
    } finally {
      setBusy(false)
    }
  }

  const pasteClipboard = async () => {
    if (!clipboard || !clipboard.entries.length) return
    setBusy(true)
    setError("")
    try {
      const existing = new Set(entries.map((e) => e.name.toLowerCase()))
      for (const entry of clipboard.entries) {
        const destName = uniqueName(entry.name, existing)
        existing.add(destName.toLowerCase())
        const dest = joinPath(cwd, destName)
        if (clipboard.mode === "copy") {
          await api.copyFilesystem(entry.path, dest)
        } else {
          await api.moveFilesystem(entry.path, dest)
        }
      }
      if (clipboard.mode === "cut") setClipboard(null)
      setStatus(
        `${clipboard.mode === "copy" ? "Copied" : "Moved"} ${clipboard.entries.length} item(s)`
      )
      await load(cwd, { pushHistory: false })
    } catch (err) {
      setError(err instanceof Error ? err.message : "Paste failed")
    } finally {
      setBusy(false)
    }
  }

  const uploadFiles = async (files: FileList | File[]) => {
    const list = Array.from(files)
    if (!list.length) return
    setBusy(true)
    setError("")
    try {
      for (const f of list) {
        await api.uploadFilesystem(cwd, f)
      }
      setStatus(`Uploaded ${list.length} file(s)`)
      await load(cwd, { pushHistory: false })
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed")
    } finally {
      setBusy(false)
    }
  }

  const buildMenuItems = (target: FsEntry | null): ContextMenuItem[] => {
    const focus = target
      ? selected.has(target.path)
        ? selectedEntries
        : [target]
      : selectedEntries
    const hasSel = focus.length > 0
    const single = focus.length === 1 ? focus[0] : null
    const items: ContextMenuItem[] = []

    if (single?.isDir) {
      items.push({
        id: "open",
        label: "Open",
        onSelect: () => openEntry(single),
      })
    } else if (single && !single.isDir) {
      items.push({
        id: "open",
        label: "Open / Download",
        onSelect: () => openEntry(single),
      })
    }

    items.push(
      { id: "sep1", label: "", separator: true },
      {
        id: "new-folder",
        label: "New folder",
        disabled: !writable,
        onSelect: () => {
          setPromptValue("New folder")
          setPrompt({ kind: "mkdir", seed: "New folder" })
        },
      },
      {
        id: "new-file",
        label: "New text document",
        disabled: !writable,
        onSelect: () => {
          setPromptValue("New Text Document.txt")
          setPrompt({ kind: "touch", seed: "New Text Document.txt" })
        },
      },
      {
        id: "upload",
        label: "Upload files…",
        disabled: !writable,
        onSelect: () => fileInputRef.current?.click(),
      },
      { id: "sep2", label: "", separator: true },
      {
        id: "cut",
        label: "Cut",
        shortcut: "Ctrl+X",
        disabled: !hasSel || !writable,
        onSelect: () => setClipboard({ mode: "cut", entries: focus }),
      },
      {
        id: "copy",
        label: "Copy",
        shortcut: "Ctrl+C",
        disabled: !hasSel,
        onSelect: () => setClipboard({ mode: "copy", entries: focus }),
      },
      {
        id: "paste",
        label: "Paste",
        shortcut: "Ctrl+V",
        disabled: !clipboard || !writable,
        onSelect: () => void pasteClipboard(),
      },
      { id: "sep3", label: "", separator: true },
      {
        id: "rename",
        label: "Rename",
        shortcut: "F2",
        disabled: !single || !writable,
        onSelect: () => {
          if (!single) return
          setPromptValue(single.name)
          setPrompt({ kind: "rename", seed: single.name, target: single })
        },
      },
      {
        id: "delete",
        label: "Delete",
        shortcut: "Del",
        danger: true,
        disabled: !hasSel || !writable,
        onSelect: () => setPrompt({ kind: "delete", targets: focus }),
      }
    )

    if (single && !single.isDir) {
      items.push({
        id: "download",
        label: "Download",
        onSelect: () => void api.downloadFilesystem(single.path),
      })
    }

    items.push(
      { id: "sep4", label: "", separator: true },
      {
        id: "refresh",
        label: "Refresh",
        shortcut: "F5",
        onSelect: refresh,
      }
    )

    if (single) {
      items.push({
        id: "props",
        label: "Properties",
        onSelect: () => setPrompt({ kind: "properties", target: single }),
      })
    }

    return items
  }

  const onPaneContext = (e: ReactMouseEvent) => {
    e.preventDefault()
    const row = (e.target as HTMLElement).closest("[data-entry-path]")
    const path = row?.getAttribute("data-entry-path")
    const target = path ? entries.find((x) => x.path === path) || null : null
    if (target && !selected.has(target.path)) {
      setSelected(new Set([target.path]))
      lastClickRef.current = target.path
    }
    setMenu({ x: e.clientX, y: e.clientY, target })
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (prompt) return
      const tag = (e.target as HTMLElement)?.tagName
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return
      const meta = e.metaKey || e.ctrlKey
      if (e.key === "F5") {
        e.preventDefault()
        refresh()
      } else if (e.key === "F2" && selectedEntries.length === 1 && writable) {
        e.preventDefault()
        const t = selectedEntries[0]
        setPromptValue(t.name)
        setPrompt({ kind: "rename", seed: t.name, target: t })
      } else if (e.key === "Delete" && selectedEntries.length && writable) {
        e.preventDefault()
        setPrompt({ kind: "delete", targets: selectedEntries })
      } else if (e.key === "Enter" && selectedEntries.length === 1) {
        e.preventDefault()
        openEntry(selectedEntries[0])
      } else if (e.key === "Backspace" && !meta) {
        e.preventDefault()
        goUp()
      } else if (meta && e.key.toLowerCase() === "a") {
        e.preventDefault()
        setSelected(new Set(entries.map((x) => x.path)))
      } else if (meta && e.key.toLowerCase() === "c" && selectedEntries.length) {
        e.preventDefault()
        setClipboard({ mode: "copy", entries: selectedEntries })
        setStatus(`Copied ${selectedEntries.length} item(s)`)
      } else if (
        meta &&
        e.key.toLowerCase() === "x" &&
        selectedEntries.length &&
        writable
      ) {
        e.preventDefault()
        setClipboard({ mode: "cut", entries: selectedEntries })
        setStatus(`Cut ${selectedEntries.length} item(s)`)
      } else if (meta && e.key.toLowerCase() === "v") {
        e.preventDefault()
        void pasteClipboard()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [
    prompt,
    selectedEntries,
    writable,
    entries,
    clipboard,
    cwd,
    listing?.parent,
  ])

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    if (e.dataTransfer.files?.length) void uploadFiles(e.dataTransfer.files)
  }

  const crumbs = useMemo(() => {
    const parts = cwd.split("/").filter(Boolean)
    const items: { label: string; path: string }[] = []
    let acc = ""
    for (const p of parts) {
      acc += `/${p}`
      items.push({ label: p, path: acc })
    }
    return items
  }, [cwd])

  const renderTree = (path: string, depth: number) => {
    const kids = treeChildren[path] || []
    return (
      <div key={path}>
        {kids.map((folder) => {
          const isActive = cwd === folder.path
          const childOpen = !!treeOpen[folder.path]
          return (
            <div key={folder.path}>
              <button
                type="button"
                className={cn(
                  "flex w-full items-center gap-1 rounded-sm px-1 py-0.5 text-left text-[12px] hover:bg-[#e5f3ff] dark:hover:bg-muted",
                  isActive && "bg-[#cce8ff] dark:bg-muted"
                )}
                style={{ paddingLeft: 6 + depth * 12 }}
                onClick={() => navigateTo(folder.path)}
              >
                <span
                  className="inline-flex size-4 items-center justify-center text-[#666]"
                  onClick={(e) => {
                    e.stopPropagation()
                    setTreeOpen((prev) => ({
                      ...prev,
                      [folder.path]: !prev[folder.path],
                    }))
                    void ensureTree(folder.path)
                  }}
                >
                  <ChevronRight
                    className={cn("size-3 transition", childOpen && "rotate-90")}
                  />
                </span>
                {childOpen ? (
                  <FolderOpen className="size-3.5 shrink-0 text-[#e6b422]" />
                ) : (
                  <Folder className="size-3.5 shrink-0 text-[#e6b422]" />
                )}
                <span className="truncate">{folder.name}</span>
              </button>
              {childOpen ? renderTree(folder.path, depth + 1) : null}
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-lg font-medium">Files</h2>
        <p className="text-sm text-muted-foreground">
          Windows-style explorer for storage roots. Right-click for actions.
        </p>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/40 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <ExplorerShell>
        {/* Title bar */}
        <div className="flex items-center justify-between border-b border-[#d0d0d0] bg-gradient-to-b from-[#ffffff] to-[#e8e8e8] px-3 py-1.5 dark:border-border dark:from-card dark:to-muted">
          <div className="flex items-center gap-2 text-sm font-medium">
            <HardDrive className="size-4 text-[#0078d4]" />
            {cwd} — File Explorer
          </div>
          <div className="text-[11px] text-[#666] dark:text-muted-foreground">
            {busy ? "Working…" : writable ? "Read / Write" : "Read only"}
          </div>
        </div>

        {/* Toolbar */}
        <div className="flex flex-wrap items-center gap-1 border-b border-[#d0d0d0] bg-[#f0f0f0] px-2 py-1.5 dark:border-border dark:bg-muted/40">
          <ToolBtn
            title="Back"
            disabled={histIdx <= 0}
            onClick={goBack}
            icon={<ArrowLeft className="size-4" />}
          />
          <ToolBtn
            title="Up"
            disabled={!listing?.parent}
            onClick={goUp}
            icon={<ArrowUp className="size-4" />}
          />
          <ToolBtn
            title="Refresh"
            onClick={refresh}
            icon={<RefreshCw className={cn("size-4", busy && "animate-spin")} />}
          />
          <div className="mx-1 h-5 w-px bg-[#cfcfcf] dark:bg-border" />
          <ToolBtn
            title="New folder"
            disabled={!writable}
            onClick={() => {
              setPromptValue("New folder")
              setPrompt({ kind: "mkdir", seed: "New folder" })
            }}
            icon={<FolderPlus className="size-4" />}
            label="New folder"
          />
          <ToolBtn
            title="New file"
            disabled={!writable}
            onClick={() => {
              setPromptValue("New Text Document.txt")
              setPrompt({ kind: "touch", seed: "New Text Document.txt" })
            }}
            icon={<FilePlus className="size-4" />}
            label="New file"
          />
          <ToolBtn
            title="Upload"
            disabled={!writable}
            onClick={() => fileInputRef.current?.click()}
            icon={<Upload className="size-4" />}
            label="Upload"
          />
          <ToolBtn
            title="Delete"
            disabled={!writable || !selectedEntries.length}
            onClick={() =>
              setPrompt({ kind: "delete", targets: selectedEntries })
            }
            icon={<Trash2 className="size-4" />}
            label="Delete"
          />
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={(e) => {
              if (e.target.files) void uploadFiles(e.target.files)
              e.target.value = ""
            }}
          />
        </div>

        {/* Address bar */}
        <div className="flex items-center gap-2 border-b border-[#d0d0d0] bg-white px-2 py-1.5 dark:border-border dark:bg-background">
          <span className="text-[11px] text-[#666] dark:text-muted-foreground">
            Address
          </span>
          <form
            className="flex min-w-0 flex-1 items-center gap-1"
            onSubmit={(e) => {
              e.preventDefault()
              navigateTo(address.trim() || roots[0])
            }}
          >
            <div className="flex min-w-0 flex-1 items-center overflow-hidden rounded-sm border border-[#7a7a7a] bg-white px-2 py-1 text-[12px] dark:border-border dark:bg-background">
              <HardDrive className="mr-1 size-3.5 shrink-0 text-[#0078d4]" />
              <div className="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto">
                {crumbs.map((c, i) => (
                  <button
                    key={c.path}
                    type="button"
                    className="inline-flex shrink-0 items-center gap-0.5 rounded px-1 hover:bg-[#e5f3ff] dark:hover:bg-muted"
                    onClick={() => navigateTo(c.path)}
                  >
                    {i > 0 ? (
                      <ChevronRight className="size-3 text-[#888]" />
                    ) : null}
                    {c.label}
                  </button>
                ))}
              </div>
              <input
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                className="ml-2 w-40 shrink-0 border-l border-[#ddd] bg-transparent pl-2 font-mono text-[11px] outline-none dark:border-border"
                spellCheck={false}
              />
            </div>
            <Button type="submit" size="sm" variant="outline">
              Go
            </Button>
          </form>
        </div>

        {/* Body */}
        <div className="grid min-h-[520px] grid-cols-[220px_1fr] bg-white dark:bg-background">
          {/* Navigation pane */}
          <aside className="overflow-auto border-r border-[#d0d0d0] bg-[#f8f8f8] p-2 dark:border-border dark:bg-muted/20">
            <p className="mb-1 px-1 text-[11px] font-semibold uppercase tracking-wide text-[#666] dark:text-muted-foreground">
              This PC
            </p>
            {roots.map((root) => (
              <div key={root} className="mb-1">
                <button
                  type="button"
                  className={cn(
                    "flex w-full items-center gap-1.5 rounded-sm px-1 py-1 text-left text-[12px] hover:bg-[#e5f3ff] dark:hover:bg-muted",
                    cwd === root && "bg-[#cce8ff] dark:bg-muted"
                  )}
                  onClick={() => {
                    setTreeOpen((p) => ({ ...p, [root]: true }))
                    void ensureTree(root)
                    navigateTo(root)
                  }}
                >
                  <HardDrive className="size-3.5 text-[#0078d4]" />
                  <span className="truncate font-medium">{root}</span>
                </button>
                {treeOpen[root] ? renderTree(root, 1) : null}
              </div>
            ))}
          </aside>

          {/* Details pane */}
          <div
            ref={paneRef}
            tabIndex={0}
            className={cn(
              "relative flex min-h-0 flex-col outline-none",
              dragOver && "ring-2 ring-inset ring-[#0078d4]"
            )}
            onContextMenu={onPaneContext}
            onDragOver={(e) => {
              e.preventDefault()
              setDragOver(true)
            }}
            onDragLeave={() => setDragOver(false)}
            onDrop={onDrop}
            onClick={(e) => {
              if (e.target === e.currentTarget) setSelected(new Set())
            }}
          >
            <div className="grid grid-cols-[minmax(0,1fr)_140px_110px_90px] gap-2 border-b border-[#d0d0d0] bg-[#f5f5f5] px-3 py-1 text-[11px] font-semibold text-[#555] dark:border-border dark:bg-muted/30 dark:text-muted-foreground">
              <span>Name</span>
              <span>Date modified</span>
              <span>Type</span>
              <span className="text-right">Size</span>
            </div>
            <div className="flex-1 overflow-auto">
              {entries.length === 0 ? (
                <div className="flex h-40 flex-col items-center justify-center gap-2 text-sm text-[#777] dark:text-muted-foreground">
                  <FolderOpen className="size-10 opacity-40" />
                  {busy ? "Loading…" : "This folder is empty."}
                  <p className="text-xs">Right-click to create or upload.</p>
                </div>
              ) : (
                entries.map((entry) => {
                  const isSel = selected.has(entry.path)
                  const dimCut =
                    clipboard?.mode === "cut" &&
                    clipboard.entries.some((c) => c.path === entry.path)
                  return (
                    <div
                      key={entry.path}
                      data-entry-path={entry.path}
                      className={cn(
                        "grid cursor-default grid-cols-[minmax(0,1fr)_140px_110px_90px] gap-2 border-b border-transparent px-3 py-1 text-[12px] hover:bg-[#e5f3ff] dark:hover:bg-muted/50",
                        isSel && "bg-[#cce8ff] hover:bg-[#cce8ff] dark:bg-muted",
                        dimCut && "opacity-50"
                      )}
                      onClick={(e) => toggleSelect(entry, e)}
                      onDoubleClick={() => openEntry(entry)}
                    >
                      <div className="flex min-w-0 items-center gap-2">
                        {entry.isDir ? (
                          <Folder className="size-4 shrink-0 text-[#e6b422]" />
                        ) : (
                          <File className="size-4 shrink-0 text-[#5b9bd5]" />
                        )}
                        <span className="truncate">{entry.name}</span>
                      </div>
                      <span className="truncate text-[#555] dark:text-muted-foreground">
                        {formatDate(entry.modTime)}
                      </span>
                      <span className="truncate text-[#555] dark:text-muted-foreground">
                        {entryType(entry)}
                      </span>
                      <span className="truncate text-right text-[#555] dark:text-muted-foreground">
                        {entry.isDir ? "" : formatBytes(entry.size)}
                      </span>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </div>

        {/* Status bar */}
        <div className="flex items-center justify-between border-t border-[#d0d0d0] bg-[#f0f0f0] px-3 py-1 text-[11px] text-[#555] dark:border-border dark:bg-muted/40 dark:text-muted-foreground">
          <span>
            {selected.size
              ? `${selected.size} item${selected.size === 1 ? "" : "s"} selected`
              : status}
            {clipboard
              ? ` · Clipboard: ${clipboard.mode} (${clipboard.entries.length})`
              : ""}
          </span>
          <span>{cwd}</span>
        </div>
      </ExplorerShell>

      {menu ? (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          items={buildMenuItems(menu.target)}
          onClose={() => setMenu(null)}
        />
      ) : null}

      {prompt && prompt.kind !== "properties" && prompt.kind !== "delete" ? (
        <Dialog
          open
          title={
            prompt.kind === "mkdir"
              ? "New folder"
              : prompt.kind === "touch"
                ? "New file"
                : "Rename"
          }
          onClose={() => setPrompt(null)}
          confirmLabel={prompt.kind === "rename" ? "Rename" : "Create"}
          onConfirm={() => void runPrompt()}
        >
          <label className="block space-y-1.5">
            <span className="text-xs text-muted-foreground">Name</span>
            <input
              autoFocus
              value={promptValue}
              onChange={(e) => setPromptValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault()
                  void runPrompt()
                }
              }}
              className="h-9 w-full rounded-lg border border-input bg-background px-3 text-sm"
            />
          </label>
        </Dialog>
      ) : null}

      {prompt?.kind === "delete" ? (
        <Dialog
          open
          title="Delete items"
          tone="danger"
          confirmLabel="Delete"
          onClose={() => setPrompt(null)}
          onConfirm={() => void runPrompt()}
        >
          <p>
            Are you sure you want to permanently delete{" "}
            {prompt.targets.length === 1
              ? `“${prompt.targets[0].name}”`
              : `${prompt.targets.length} items`}
            ?
          </p>
          <ul className="mt-2 max-h-40 overflow-auto rounded-md border border-border bg-muted/40 px-3 py-2 font-mono text-xs">
            {prompt.targets.map((t) => (
              <li key={t.path}>{t.path}</li>
            ))}
          </ul>
        </Dialog>
      ) : null}

      {prompt?.kind === "properties" ? (
        <Dialog
          open
          title="Properties"
          hideCancel
          confirmLabel="Close"
          onClose={() => setPrompt(null)}
        >
          <dl className="space-y-2 text-sm">
            <div>
              <dt className="text-xs text-muted-foreground">Name</dt>
              <dd className="font-medium">{prompt.target.name}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Location</dt>
              <dd className="break-all font-mono text-xs">{prompt.target.path}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Type</dt>
              <dd>{entryType(prompt.target)}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Size</dt>
              <dd>
                {prompt.target.isDir
                  ? "—"
                  : `${formatBytes(prompt.target.size)} (${prompt.target.size.toLocaleString()} bytes)`}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Modified</dt>
              <dd>{formatDate(prompt.target.modTime)}</dd>
            </div>
            {prompt.target.mode ? (
              <div>
                <dt className="text-xs text-muted-foreground">Mode</dt>
                <dd className="font-mono text-xs">{prompt.target.mode}</dd>
              </div>
            ) : null}
          </dl>
        </Dialog>
      ) : null}
    </div>
  )
}

function ToolBtn({
  icon,
  label,
  title,
  onClick,
  disabled,
}: {
  icon: ReactNode
  label?: string
  title: string
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      title={title}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "inline-flex h-8 items-center gap-1.5 rounded-md border border-transparent px-2 text-[12px] hover:border-[#adadad] hover:bg-white disabled:opacity-40 dark:hover:border-border dark:hover:bg-muted"
      )}
    >
      {icon}
      {label ? <span>{label}</span> : null}
    </button>
  )
}
