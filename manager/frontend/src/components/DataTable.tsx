import type { CSSProperties, ReactNode } from "react"
import { cn } from "@/lib/utils"

/** Full-width table shell: page scrolls, no nested viewport scroll. */
export function DataTable({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div className={cn("w-full rounded-xl border border-border", className)}>
      <table className="w-full table-fixed text-left text-sm">{children}</table>
    </div>
  )
}

export function Th({
  children,
  className,
  style,
}: {
  children?: ReactNode
  className?: string
  style?: CSSProperties
}) {
  return (
    <th
      style={style}
      className={cn(
        "border-b border-border bg-muted/50 px-3 py-2 font-medium text-muted-foreground",
        className
      )}
    >
      {children}
    </th>
  )
}

export function Td({
  children,
  className,
  colSpan,
}: {
  children?: ReactNode
  className?: string
  colSpan?: number
}) {
  return (
    <td colSpan={colSpan} className={cn("px-3 py-2 align-top break-words", className)}>
      {children}
    </td>
  )
}
