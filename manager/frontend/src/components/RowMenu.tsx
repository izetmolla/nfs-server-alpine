import { MoreVertical } from "lucide-react"
import { useEffect, useId, useRef, useState } from "react"
import { cn } from "@/lib/utils"

export type RowMenuItem = {
  label: string
  onSelect: () => void
  tone?: "default" | "danger"
  disabled?: boolean
}

type Props = {
  items: RowMenuItem[]
  disabled?: boolean
  align?: "left" | "right"
}

export function RowMenu({ items, disabled, align = "right" }: Props) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const menuId = useId()

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false)
    }
    document.addEventListener("mousedown", onDoc)
    window.addEventListener("keydown", onKey)
    return () => {
      document.removeEventListener("mousedown", onDoc)
      window.removeEventListener("keydown", onKey)
    }
  }, [open])

  return (
    <div ref={rootRef} className="relative inline-flex justify-end">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        disabled={disabled}
        title="Actions"
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "inline-flex size-8 items-center justify-center rounded-lg border border-border text-muted-foreground hover:bg-muted hover:text-foreground",
          disabled && "pointer-events-none opacity-50"
        )}
      >
        <MoreVertical className="size-4" />
      </button>
      {open ? (
        <div
          id={menuId}
          role="menu"
          className={cn(
            "absolute z-40 mt-1 min-w-36 overflow-hidden rounded-lg border border-border bg-popover py-1 text-popover-foreground shadow-lg",
            align === "right" ? "right-0" : "left-0",
            "top-full"
          )}
        >
          {items.map((item) => (
            <button
              key={item.label}
              type="button"
              role="menuitem"
              disabled={item.disabled}
              className={cn(
                "flex w-full px-3 py-1.5 text-left text-sm hover:bg-muted disabled:opacity-50",
                item.tone === "danger" && "text-destructive"
              )}
              onClick={() => {
                setOpen(false)
                item.onSelect()
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  )
}
