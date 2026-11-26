import type { ExportInput } from "@/lib/api"

export type PendingChange = {
  key: keyof ExportInput | "clients" | "apply"
  next: unknown
  title: string
  tone: "warning" | "danger"
  bullets: string[]
}

export function warningForEnumChange(
  key: keyof ExportInput,
  value: string
): PendingChange | null {
  const map: Partial<
    Record<keyof ExportInput, Record<string, Omit<PendingChange, "key" | "next">>>
  > = {
    accessMode: {
      rw: {
        title: "Enable read/write access?",
        tone: "warning",
        bullets: [
          "Clients will be able to create, modify, and delete files on this export.",
          "Combine carefully with Clients and root squash settings.",
        ],
      },
    },
    commitMode: {
      async: {
        title: "Use async commits?",
        tone: "warning",
        bullets: [
          "The server may acknowledge writes before they hit stable storage.",
          "A crash or power loss can drop recent writes the client already considered done.",
          "Prefer sync for VMs, databases, or anything that must not lose data.",
        ],
      },
    },
    portMode: {
      insecure: {
        title: "Allow insecure (non-reserved) client ports?",
        tone: "warning",
        bullets: [
          "Needed for macOS, containers, and most non-root mounts.",
          "Any local user process can speak NFS as a client — keep Clients CIDRs tight.",
        ],
      },
    },
    rootSquash: {
      no_root_squash: {
        title: "Disable root squash?",
        tone: "danger",
        bullets: [
          "Remote root keeps UID 0 on the server for this path.",
          "A compromised client with root can rewrite ownership and delete anything on the share.",
          "Only confirm on trusted networks with a narrow Clients list.",
        ],
      },
    },
    allSquash: {
      all_squash: {
        title: "Squash all users to anonymous?",
        tone: "warning",
        bullets: [
          "Every client UID/GID becomes anonuid/anongid.",
          "Set anonuid/anongid to a user that can write the directory, or clients will get permission denied.",
        ],
      },
    },
  }

  const entry = map[key]?.[value]
  if (!entry) return null
  return { key, next: value, ...entry }
}

export function warningForClients(clients: string): PendingChange | null {
  const parts = clients.replaceAll(",", " ").split(/\s+/).filter(Boolean)
  if (!parts.includes("*")) return null
  return {
    key: "clients",
    next: clients,
    title: "Allow all clients (*)?",
    tone: "danger",
    bullets: [
      "Any host that can reach this NFS server may attempt to mount the export.",
      "With rw and no_root_squash this is especially dangerous.",
      "Prefer a LAN CIDR such as 192.168.200.0/24 when possible.",
    ],
  }
}

export function warningForApply(form: ExportInput): PendingChange {
  const bullets = [
    `Rewrite /etc/exports and run exportfs -rv for path ${form.path}.`,
    "If the directory does not exist yet, it will be created (mkdir -p, mode 0755).",
    "Live NFS clients may briefly see export table changes.",
  ]
  if (form.clients.split(/\s+/).includes("*")) {
    bullets.push("Clients is * — this export is world-mountable on the network path to the server.")
  }
  if (form.rootSquash === "no_root_squash") {
    bullets.push("no_root_squash is active — remote root maps to local root.")
  }
  if (form.commitMode === "async") {
    bullets.push("async is active — crash safety is reduced.")
  }
  return {
    key: "apply",
    next: true,
    title: "Save and apply live NFS exports?",
    tone:
      form.clients.includes("*") || form.rootSquash === "no_root_squash"
        ? "danger"
        : "warning",
    bullets,
  }
}
