# Supported Platforms

## v1 Matrix

| Surface | Supported | Qualification |
| --- | --- | --- |
| Linux amd64 binaries | yes | canonical build and runtime target |
| Native Linux amd64 systemd service | qualification | Debian/systemd CI proves install/start/restart/reinstall, old-to-new upgrade, failed and explicit rollback, stopped-node backup/restore, and uninstall; distribution-matrix evidence remains |
| Linux arm64 binaries | unsupported | Waku/RLN requires CGO; native build and runtime qualification are required |
| Docker Engine 29.x + Compose v2 on Linux amd64 | yes | canonical deployment and multi-node E2E target |
| Windows 11 amd64 host with WSL2/Docker Desktop | orchestration | tests and daemons run in Linux containers; host resources must be monitored |
| macOS | unsupported | no maintained runtime/deployment evidence |
| Native Windows daemon | unsupported | no release/service/ACL acceptance gate yet |
| Kubernetes / multi-host scheduler | unsupported | no `v1` deployment contract or acceptance environment |

Supported Go source builds use the exact toolchain declared by `go.mod`.
Container builds use the versioned Dockerfile. A platform becomes supported only
after startup, persistence, networking, security, upgrade, and rollback evidence
exists on that platform; compilation alone is insufficient.
