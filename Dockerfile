# syntax=docker/dockerfile:1

FROM oven/bun:1-alpine AS frontend
WORKDIR /src
COPY manager/frontend/package.json manager/frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY manager/frontend/ ./
RUN bun run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY manager/go.mod manager/go.sum ./
RUN go mod download
COPY manager/ ./
COPY --from=frontend /src/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /main .

# --- shrink stage: strip + UPX inside the toolkit image ---
FROM izetmolla/containerws:binoptimization AS shrink
COPY --from=build /main /out/main
RUN ls -lh /out/main && optimize /out/main --best --lzma

FROM alpine:3.24.4
LABEL author="Izet Molla <izetmolla@icloud.com>"
LABEL maintainer="Izet Molla <izetmolla@icloud.com>"
LABEL source="https://github.com/izetmolla/nfs-server-alpine"
LABEL branch="main"
COPY Dockerfile README.md /

RUN apk add --no-cache --update --verbose nfs-utils bash iproute2 && \
    rm -rf /var/cache/apk/* /sbin/halt /sbin/poweroff /sbin/reboot && \
    mkdir -p /var/lib/nfs/rpc_pipefs /var/lib/nfs/v4recovery /var/lib/nfs-manager && \
    echo "rpc_pipefs    /var/lib/nfs/rpc_pipefs rpc_pipefs      defaults        0       0" >> /etc/fstab && \
    echo "nfsd  /proc/fs/nfsd   nfsd    defaults        0       0" >> /etc/fstab

COPY exports /etc/
COPY nfsd.sh /usr/bin/nfsd.sh
COPY entrypoint.sh /usr/bin/entrypoint.sh
COPY .bashrc /root/.bashrc
COPY --from=shrink /out/main /usr/bin/nfs-manager

RUN chmod +x /usr/bin/nfsd.sh /usr/bin/entrypoint.sh /usr/bin/nfs-manager

ENV PANEL_ADDR=:3000 \
    MANAGER_DB=/var/lib/nfs-manager/manager.db \
    EXPORTS_FILE=/etc/exports

EXPOSE 2049 3000

ENTRYPOINT ["/usr/bin/entrypoint.sh"]
