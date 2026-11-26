import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { StorageForm } from "@/components/StorageForm"
import { api, type ExportInput } from "@/lib/api"

export function StorageEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [initial, setInitial] = useState<ExportInput | null>(null)
  const [error, setError] = useState("")

  useEffect(() => {
    const num = Number(id)
    if (!num) {
      setError("invalid id")
      return
    }
    api
      .getExport(num)
      .then((exp) =>
        setInitial({
          path: exp.path,
          description: exp.description ?? "",
          clients: exp.clients,
          accessMode: exp.accessMode,
          commitMode: exp.commitMode,
          writeDelay: exp.writeDelay,
          subtreeCheck: exp.subtreeCheck,
          portMode: exp.portMode,
          lockMode: exp.lockMode,
          hideMode: exp.hideMode,
          crossmnt: exp.crossmnt,
          rootSquash: exp.rootSquash,
          allSquash: exp.allSquash,
          security: exp.security,
          fsid: exp.fsid,
          anonUid: exp.anonUid ?? null,
          anonGid: exp.anonGid ?? null,
          enabled: exp.enabled,
          quotaBytes: exp.quotaBytes ?? 0,
          quotaPostbackUrl: exp.quotaPostbackUrl ?? "",
        })
      )
      .catch((err: Error) => setError(err.message))
  }, [id])

  async function handleSubmit(input: ExportInput, apply: boolean) {
    await api.updateExport(Number(id), input)
    if (apply) await api.applyExports()
    navigate("/storages")
  }

  if (error) {
    return <p className="text-sm text-destructive">{error}</p>
  }
  if (!initial) {
    return <p className="text-sm text-muted-foreground">Loading…</p>
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="font-medium">Edit storage #{id}</h2>
        <p className="text-sm text-muted-foreground">
          Advanced NFS options with live export-line preview.
        </p>
      </div>
      <StorageForm
        key={id}
        initial={initial}
        submitLabel="Save"
        onSubmit={handleSubmit}
      />
    </div>
  )
}
