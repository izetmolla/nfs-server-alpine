import { useNavigate } from "react-router-dom"
import { StorageForm } from "@/components/StorageForm"
import { api, type ExportInput } from "@/lib/api"

export function StorageCreatePage() {
  const navigate = useNavigate()

  async function handleSubmit(input: ExportInput, apply: boolean) {
    await api.createExport(input)
    if (apply) await api.applyExports()
    navigate("/storages")
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="font-medium">New storage export</h2>
        <p className="text-sm text-muted-foreground">
          Configure every NFS option with selects — no freehand export lines.
        </p>
      </div>
      <StorageForm submitLabel="Create" onSubmit={handleSubmit} />
    </div>
  )
}
