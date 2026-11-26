# nfs-server-alpine

Lightweight **NFSv4** server on Alpine Linux, with a built-in **web management panel** for exports, quotas, reports, API tokens, and live status.

**Author:** [Izet Molla](https://github.com/izetmolla) · [github.com/izetmolla/nfs-server-alpine](https://github.com/izetmolla/nfs-server-alpine)

| Port | Service |
| --- | --- |
| **2049/tcp** | NFSv4 |
| **3000/tcp** | Manager panel + REST API (+ Swagger) |

Image: `izetmolla/nfs-server-alpine:latest`  
Base: Alpine **3.24.4** · NFSv4 only (no UDP / no v2)

---

## Features

- NFSv4 over TCP (`fsid=0` pseudo-root)
- Web UI to create/edit exports with typed NFS options (no freehand export lines on forms)
- Directory browser for export paths
- Storage reports + **per-export quotas** (unlimited by default) with **HTTP postback** when full
- Dashboard metrics (auth / apply / API activity) with time-range filters
- Live logs, settings, panel firewall CIDRs, Swagger toggle
- IP-scoped API tokens for automation

---

## Quick start (Docker)

```bash
docker run -d --name nfs \
  --privileged \
  -p 2049:2049 \
  -p 3000:3000 \
  -v /data/nfs:/mnt \
  -v nfs-manager-data:/var/lib/nfs-manager \
  -e SHARED_DIRECTORY=/mnt \
  -e PANEL_PASSWORD=changeme \
  izetmolla/nfs-server-alpine:latest
```

Open the panel: [http://localhost:3000](http://localhost:3000)  
Login password: value of `PANEL_PASSWORD` (example above: `changeme`).

Mount from a client (NFSv4 root = `SHARED_DIRECTORY`):

```bash
sudo mkdir -p /mnt/nfs
sudo mount -t nfs4 -o nfsvers=4,proto=tcp HOST:/ /mnt/nfs
```

Unmount:

```bash
sudo umount /mnt/nfs
```

> **Privileged mode** is required so the container can run kernel `nfsd`. Prefer a dedicated host/path volume (OverlayFS often cannot be exported).

---

## Docker Compose

Example layout (adjust host path / network to your environment):

```yaml
services:
  nfs-server:
    image: izetmolla/nfs-server-alpine:latest
    build: .
    container_name: nfs-server
    privileged: true
    restart: unless-stopped
    ports:
      - "2049:2049"
      - "3000:3000"
    environment:
      SHARED_DIRECTORY: /mnt
      PANEL_PASSWORD: changeme
      PANEL_ADDR: ":3000"
      MANAGER_DB: /var/lib/nfs-manager/manager.db
    volumes:
      - /data/nfs:/mnt
      - nfs-manager-data:/var/lib/nfs-manager

  # Optional smoke client that mounts the share and writes a test file
  example-app:
    image: ubuntu:latest
    privileged: true
    depends_on: [nfs-server]
    environment:
      NFS_SERVER: nfs-server   # or the server IP
      NFS_PATH: /
      NFS_MOUNT: /mnt/nfs
    command:
      - bash
      - -lc
      - |
        apt-get update -qq && apt-get install -y -qq nfs-common
        mkdir -p "$$NFS_MOUNT"
        mount -t nfs4 -o nfsvers=4,proto=tcp "$$NFS_SERVER:$$NFS_PATH" "$$NFS_MOUNT"
        mkdir -p "$$NFS_MOUNT/example-app"
        echo "ok $$(date -Is)" > "$$NFS_MOUNT/example-app/hello.txt"
        ls -la "$$NFS_MOUNT/example-app"
        sleep infinity

volumes:
  nfs-manager-data:
```

```bash
docker compose up -d --build
docker logs -f example-app   # should show "NFS write test OK" / hello.txt
```

The repo [docker-compose.yaml](https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/docker-compose.yaml) may pin a custom network/IP for lab use — change it to match your network.

---

## Kubernetes

Full sample manifests are in [k8s.yaml](https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/k8s.yaml): Namespace, panel Secret, hostPath PVs/PVCs for data + manager DB, privileged Deployment, ClusterIP/NodePort Services, and an optional NFS client example.

Important points:

1. Pod must run **`privileged: true`** (kernel `nfsd`).
2. Data volume must be a real filesystem (`hostPath` / local PV) — not OverlayFS.
3. Single replica with `Recreate` strategy.
4. Expose **2049** (NFS) and **3000** (panel); set `PANEL_PASSWORD` via Secret.
5. Do **not** mount a ConfigMap over `/etc/exports` — the panel owns that file.

```bash
# Edit hostPath paths, nodeSelector, and Secret password first
kubectl apply -f https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/k8s.yaml
kubectl -n nfs get pods,svc,pvc
kubectl -n nfs port-forward svc/nfs-panel 3000:3000
```

In-cluster client mount (path is **relative to the NFSv4 root** / `fsid=0`):

```yaml
volumes:
  - name: nfs-vol
    nfs:
      server: nfs-server.nfs.svc.cluster.local
      path: /          # or /subdir under SHARED_DIRECTORY
```

---

## Manager panel

### Login

1. Browse to `http://<host>:3000`
2. Enter `PANEL_PASSWORD`
3. Session cookie + optional Bearer password for API tools

### What you can do

| Area | Purpose |
| --- | --- |
| **Dashboard** | Live NFS health, clients, auth/apply counters, charts (default last 7 days) |
| **Storages** | List exports · create/edit with typed NFS options · browse/create folders |
| **Reports** | Filesystem usage + per-export **quotas** · “Check quotas now” |
| **Logs** | Searchable table with level filters, sort, pagination |
| **Tokens** | Create API tokens limited by CIDR + scopes |
| **Settings** | Password, NFS defaults, **Swagger on/off**, **panel firewall CIDRs** |
| **Swagger** | `/swagger` when enabled (default on) |

### Create an export (typical flow)

1. **Storages → New storage**
2. Choose path (Browse… under `/mnt` or your shared volume)
3. Set clients (`*` or a LAN CIDR such as `192.168.200.0/24`)
4. Tune access / squash / fsid (exactly one enabled export should use `fsid=0`)
5. Optional **quota**: amount + unit (MB/GB/TB) and postback URL
6. **Save & Apply** → writes `/etc/exports` and runs `exportfs -rv`  
   Missing directories are created automatically (`mkdir -p`).

### Quotas & postback

- Default quota = **unlimited** (`quotaBytes: 0`)
- Usage is measured as **directory size** under the export path
- When usage ≥ quota, the manager POSTs once:

```http
POST <quotaPostbackUrl>
Content-Type: application/json
X-NFS-Manager-Event: quota.full
```

```json
{
  "event": "quota.full",
  "exportId": 1,
  "path": "/mnt/media",
  "usedBytes": 10737418240,
  "quotaBytes": 10737418240,
  "usedPct": 100,
  "message": "storage export reached its configured quota"
}
```

Automatic checks run about every **60s**; you can also trigger **Reports → Check quotas now**.

### Panel security tips

- Change `PANEL_PASSWORD` immediately
- Settings → **Panel firewall**: restrict to management CIDRs (default `0.0.0.0/0`)
- Prefer client CIDRs on exports instead of `*` in production
- Disable Swagger in production if unused

---

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `SHARED_DIRECTORY` | _(required for seed)_ | NFSv4 root path inside the container |
| `PANEL_PASSWORD` | _(required)_ | Panel / API password |
| `PANEL_ADDR` | `:3000` | Panel listen address |
| `MANAGER_DB` | `/var/lib/nfs-manager/manager.db` | SQLite database path |
| `EXPORTS_FILE` | `/etc/exports` | Exports file path |
| `BROWSE_ROOTS` | `SHARED_DIRECTORY` or `/mnt` | Paths the UI may browse/create |
| `PERMITTED` | `*` | Initial client ACL when seeding from env |
| `READ_ONLY` | unset → `rw` | Seed read-only if set |
| `SYNC` | unset → `async` | Seed sync mode if set |
| `SHARED_DIRECTORY_2` | unset | Optional second share (must be under root) |

Persist `/var/lib/nfs-manager` so panel settings, tokens, metrics, and exports survive restarts.

---

## REST API (short)

Base path: `/api` · Auth: `Authorization: Bearer <PANEL_PASSWORD>` or `nfs_<token>`

Useful endpoints:

- `GET /api/dashboard?range=last_7_days`
- `GET/POST /api/exports` · `PUT/DELETE /api/exports/:id` · `POST /api/exports/apply`
- `GET /api/exports/:id/usage` · `PUT /api/exports/:id/quota`
- `GET /api/reports/storage` · `GET /api/reports/quotas` · `POST /api/reports/quotas/check`
- `GET/POST /api/tokens`
- `GET /api/public-config`

Interactive docs: `/swagger` (when enabled).

---

## Client notes

```bash
# Explicit NFSv4
sudo mount -t nfs4 -o nfsvers=4,proto=tcp,port=2049 SERVER:/ /mnt/nfs

# Subdirectory of the root share (relative to fsid=0)
sudo mount -t nfs4 SERVER:/media /mnt/media
```

- macOS / many containers need export option **`insecure`** (panel default).
- `showmount -e` may not work usefully; prefer `exportfs -v` on the server or the panel status view.
- Ensure host modules are available: `modprobe nfs nfsd` if needed.

---

## Build from source

```bash
git clone https://github.com/izetmolla/nfs-server-alpine.git
cd nfs-server-alpine
docker build -t izetmolla/nfs-server-alpine:latest .
```

Multi-stage build: Bun frontend → Go manager → UPX shrink → Alpine runtime.

---

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Mount connection refused on 2049 | Container privileged? `nfsd` threads > 0? `ss -lntp` shows 2049? |
| Permission denied from client | Export clients CIDR, `insecure`, squash options |
| Panel unreachable | Port 3000 published? Panel firewall CIDR? |
| Quota postback not firing | URL must be `http(s)://…` · check Logs for `quota` · run **Check quotas now** |
| Empty second share | Must be a **subdirectory** of `SHARED_DIRECTORY` (NFSv4 tree) |

Successful NFS startup log includes `Startup successful.` and a listening socket on **2049**.

---

## Licence / source

**Author / Maintainer:** [Izet Molla](https://github.com/izetmolla) · [github.com/izetmolla/nfs-server-alpine](https://github.com/izetmolla/nfs-server-alpine)

| File | Raw (works from Docker Hub README) |
| --- | --- |
| [Dockerfile](https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/Dockerfile) | `https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/Dockerfile` |
| [docker-compose.yaml](https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/docker-compose.yaml) | `https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/docker-compose.yaml` |
| [k8s.yaml](https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/k8s.yaml) | `https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/k8s.yaml` |
| [LICENSE](https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/LICENSE) | `https://raw.githubusercontent.com/izetmolla/nfs-server-alpine/main/LICENSE` |

This image is intended for trusted networks. Do not expose NFS or the panel to the public internet without strong network controls.
