import type { StylesConfig, GroupBase, Theme } from "react-select"

export type SelectOption = { value: string; label: string }

function cssVar(name: string, fallback: string) {
  if (typeof window === "undefined") return fallback
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

export function selectTheme(theme: Theme) {
  const isDark = document.documentElement.classList.contains("dark")
  return {
    ...theme,
    borderRadius: 8,
    colors: {
      ...theme.colors,
      primary: cssVar("--ring", isDark ? "#888" : "#666"),
      primary75: cssVar("--accent", isDark ? "#333" : "#eee"),
      primary50: cssVar("--muted", isDark ? "#333" : "#eee"),
      primary25: cssVar("--muted", isDark ? "#2a2a2a" : "#f5f5f5"),
      danger: cssVar("--destructive", "#e11"),
      dangerLight: "transparent",
      neutral0: cssVar("--popover", isDark ? "#222" : "#fff"),
      neutral5: cssVar("--muted", isDark ? "#2a2a2a" : "#f5f5f5"),
      neutral10: cssVar("--muted", isDark ? "#2a2a2a" : "#f0f0f0"),
      neutral20: cssVar("--border", isDark ? "#444" : "#ddd"),
      neutral30: cssVar("--border", isDark ? "#555" : "#ccc"),
      neutral40: cssVar("--muted-foreground", isDark ? "#aaa" : "#888"),
      neutral50: cssVar("--muted-foreground", isDark ? "#999" : "#777"),
      neutral60: cssVar("--foreground", isDark ? "#eee" : "#333"),
      neutral70: cssVar("--foreground", isDark ? "#f5f5f5" : "#222"),
      neutral80: cssVar("--foreground", isDark ? "#fafafa" : "#111"),
      neutral90: cssVar("--foreground", isDark ? "#fff" : "#000"),
    },
  }
}

export function selectStyles(
  opts?: { invalid?: boolean }
): StylesConfig<SelectOption, boolean, GroupBase<SelectOption>> {
  const isDark = document.documentElement.classList.contains("dark")
  const bg = cssVar("--background", isDark ? "#171717" : "#fff")
  const popover = cssVar("--popover", isDark ? "#262626" : "#fff")
  const fg = cssVar("--foreground", isDark ? "#fafafa" : "#171717")
  const mutedFg = cssVar("--muted-foreground", isDark ? "#a3a3a3" : "#737373")
  const border = cssVar("--border", isDark ? "rgba(255,255,255,0.12)" : "#e5e5e5")
  const accent = cssVar("--accent", isDark ? "#404040" : "#f5f5f5")
  const accentFg = cssVar("--accent-foreground", fg)
  const input = cssVar("--input", border)
  const primary = cssVar("--primary", isDark ? "#e5e5e5" : "#171717")
  const primaryFg = cssVar("--primary-foreground", isDark ? "#171717" : "#fafafa")
  const danger = cssVar("--destructive", isDark ? "#f87171" : "#dc2626")
  const invalid = Boolean(opts?.invalid)

  return {
    control: (base, state) => ({
      ...base,
      minHeight: 36,
      backgroundColor: bg,
      borderColor: invalid
        ? danger
        : state.isFocused
          ? cssVar("--ring", border)
          : input,
      boxShadow: invalid
        ? `0 0 0 2px color-mix(in oklch, ${danger} 30%, transparent)`
        : state.isFocused
          ? `0 0 0 2px color-mix(in oklch, ${cssVar("--ring", "#888")} 35%, transparent)`
          : "none",
      color: fg,
      ":hover": { borderColor: invalid ? danger : cssVar("--ring", border) },
    }),
    menu: (base) => ({
      ...base,
      backgroundColor: popover,
      border: `1px solid ${border}`,
      boxShadow: "0 8px 24px rgba(0,0,0,0.35)",
      zIndex: 60,
      overflow: "hidden",
    }),
    menuList: (base) => ({
      ...base,
      backgroundColor: popover,
      padding: 4,
    }),
    option: (base, state) => ({
      ...base,
      backgroundColor: state.isSelected
        ? primary
        : state.isFocused
          ? accent
          : popover,
      color: state.isSelected ? primaryFg : state.isFocused ? accentFg : fg,
      cursor: "pointer",
      borderRadius: 6,
      ":active": {
        backgroundColor: state.isSelected ? primary : accent,
      },
    }),
    singleValue: (base) => ({ ...base, color: fg }),
    multiValue: (base) => ({
      ...base,
      backgroundColor: accent,
      borderRadius: 6,
    }),
    multiValueLabel: (base) => ({ ...base, color: accentFg }),
    multiValueRemove: (base) => ({
      ...base,
      color: mutedFg,
      ":hover": { backgroundColor: "transparent", color: fg },
    }),
    input: (base) => ({ ...base, color: fg }),
    placeholder: (base) => ({ ...base, color: mutedFg }),
    indicatorsContainer: (base) => ({ ...base, color: mutedFg }),
    dropdownIndicator: (base) => ({
      ...base,
      color: mutedFg,
      ":hover": { color: fg },
    }),
    clearIndicator: (base) => ({
      ...base,
      color: mutedFg,
      ":hover": { color: fg },
    }),
    indicatorSeparator: (base) => ({ ...base, backgroundColor: border }),
    valueContainer: (base) => ({ ...base, color: fg }),
    noOptionsMessage: (base) => ({ ...base, color: mutedFg }),
  }
}
