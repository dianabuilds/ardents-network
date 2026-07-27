FROM docker.io/library/debian:bookworm-slim@sha256:63a496b5d3b99214b39f5ed70eb71a61e590a77979c79cbee4faf991f8c0783e

ENV container=docker
RUN apt-get update && \
    apt-get install -y --no-install-recommends systemd dbus ca-certificates python3-minimal && \
    systemctl mask dev-hugepages.mount sys-fs-fuse-connections.mount systemd-remount-fs.service && \
    rm -rf /var/lib/apt/lists/*

STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
