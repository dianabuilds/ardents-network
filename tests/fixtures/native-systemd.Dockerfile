FROM debian:bookworm-slim

ENV container=docker
RUN apt-get update && \
    apt-get install -y --no-install-recommends systemd dbus ca-certificates python3-minimal && \
    systemctl mask dev-hugepages.mount sys-fs-fuse-connections.mount systemd-remount-fs.service && \
    rm -rf /var/lib/apt/lists/*

STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
