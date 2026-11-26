import { useEffect, useRef, type ReactNode } from "react"
import { cn } from "@/lib/utils"

export type ContextMenuItem = {
  id: string
  label: string
  shortcut?: string
  disabled?: boolean
  danger?: boolean
  separator?: boolean
  onSelect?: () => void
}

type Props = {
  x: number
  y: number
  items: ContextMenuItem[]
  onClose: () => void
}

export function ContextMenu({ x, y, items, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("mousedown", onDoc)
    window.addEventListener("keydown", onKey)
    return () => {
      document.removeEventListener("mousedown", onDoc)
      window.removeEventListener("keydown", onKey)
    }
  }, [onClose])

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    let left = x
    let top = y
    if (left + rect.width > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - rect.width - 8)
    }
    if (top + rect.height > window.innerHeight - 8) {
      top = Math.max(8, window.innerHeight - rect.height - 8)
    }
    el.style.left = `${left}px`
    el.style.top = `${top}px`
  }, [x, y, items])

  return (
    <div
      ref={ref}
      role="menu"
      className="fixed z-[200] min-w-52 overflow-hidden rounded-md border border-[#a0a0a0] bg-[#f2f2f2] py-1 text-[13px] text-[#1a1a1a] shadow-xl dark:border-border dark:bg-popover dark:text-popover-foreground"
      style={{ left: x, top: y }}
    >
      {items.map((item) =>
        item.separator ? (
          <div
            key={item.id}
            className="my-1 border-t border-[#cdcdcd] dark:border-border"
          />
        ) : (
          <button
            key={item.id}
            type="button"
            role="menuitem"
            disabled={item.disabled}
            className={cn(
              "flex w-full items-center justify-between gap-6 px-3 py-1.5 text-left hover:bg-[#cce8ff] disabled:opacity-40 dark:hover:bg-muted",
              item.danger && "text-[#c42b1c] dark:text-destructive"
            )}
            onClick={() => {
              if (item.disabled) return
              onClose()
              item.onSelect?.()
            }}
          >
            <span>{item.label}</span>
            {item.shortcut ? (
              <span className="text-[11px] text-[#666] dark:text-muted-foreground">
                {item.shortcut}
              </span>
            ) : null}
          </button>
        )
      )}
    </div>
  )
}

export function ExplorerShell({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-hidden rounded-lg border border-[#8a8a8a] bg-[#f3f3f3] text-[#1b1b1b] shadow-sm dark:border-border dark:bg-card dark:text-foreground">
      {children}
    </div>
  )
}
