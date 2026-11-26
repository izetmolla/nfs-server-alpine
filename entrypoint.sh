#!/bin/bash
set -euo pipefail

mkdir -p /var/lib/nfs-manager

if [ -z "${PANEL_PASSWORD:-}" ]; then
  echo "PANEL_PASSWORD is required for the NFS manager panel"
  exit 1
fi

echo "Starting nfs-manager on ${PANEL_ADDR:-:3000}..."
/usr/bin/nfs-manager &

# Replace this shell with nfsd.sh so it remains PID 1 and owns NFS signals.
# The manager stays attached as a child of PID 1.
exec /usr/bin/nfsd.sh
