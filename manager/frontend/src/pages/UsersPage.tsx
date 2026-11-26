export function UsersPage() {
  return (
    <div className="space-y-4">
      <div>
        <h2 className="font-medium">Users</h2>
        <p className="text-sm text-muted-foreground">
          Panel access is currently a single shared password (`PANEL_PASSWORD`).
        </p>
      </div>

      <section className="rounded-xl border border-border p-4 text-sm">
        <h3 className="font-medium">Current auth model</h3>
        <ul className="mt-3 list-disc space-y-1 ps-5 text-muted-foreground">
          <li>One operator password from the container environment</li>
          <li>Session cookie + Bearer token for API calls</li>
          <li>No per-user accounts in SQLite yet</li>
        </ul>
        <p className="mt-4 text-muted-foreground">
          Multi-user management can be added here later (create/disable users,
          rotate credentials) without changing the storages workflow.
        </p>
      </section>
    </div>
  )
}
