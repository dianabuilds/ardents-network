# Stage 5 final campaign inputs

These versioned files define the non-secret S5.5-to-S9.6 topology, cgroup,
network, workload, and observer handoff contracts. They are copied to a new external
configuration root before the now-retired Stage-5 campaign generator ran. R-080
retains this material as historical provenance only.

The external root must additionally contain:

- `configuration/invites.sha256`, a canonical line-oriented inventory of the
  preloaded signed Invite files; and
- `configuration/route-credentials.sha256`, a canonical line-oriented
  inventory of the preloaded per-role credential files.

The two inventories contain only SHA-256, byte count, and manifest-relative
path. Raw Invites, addresses, TLS/path secrets, and role keys stay outside Git.
Preparation rejects missing files, mutable tracked source, an existing output
root, or an output root inside the repository. The resulting `final-spec.json`
precommits all 594 cells and a unique 32-byte seed for each cell.
The live fixture purpose-separates that seed for Application streams, bounded
capacity offers, partial handshakes, and C5/C6 probe corpora; it does not reuse
the development fixture's fixed corpus bytes in a final campaign.
P0 uses four ordinal-specific derivations and four named peer pairs.

This directory is not evidence and cannot satisfy S9.6 qualification by itself. The final
campaign remains incomplete until the frozen bundle runs on the qualifying
non-overcommitted host and the independent verifier emits `pass`.

Before final preparation, preload the separately receipt-verified Carrier
tooling image plus an offline Go 1.26.6 `linux/amd64` builder image by their
exact Docker IDs. Build the latter only from `go-builder.Dockerfile`, the
official archive, and a deterministic `gomodcache.tar.gz` containing the full
`go mod download all` graph for the committed `go.mod`/`go.sum`. Freeze both
external-file hashes in `supply.lock.json` and pass the hash of the exact
recipe as `BUILDER_RECIPE_SHA256`. The recipe verifies and embeds all three
hashes; preparation checks its exact base ancestry, labels, embedded receipts,
and `go version`. The builder therefore contains the hash-bound module graph
before final preparation disables its network:

```text
go run ./scripts/generate-stage5-module-cache.go \
  -workspace . \
  -output <external-builder-context>/gomodcache.tar.gz
```

The generator starts from an empty external cache, runs `go mod download all`,
`go mod verify`, and `go list -m all`, embeds hashes of the committed
`go.mod`/`go.sum` plus the exact module list, and emits a canonical tar/gzip
stream. The product build repeats the source hashes, module-list comparison,
and `go mod verify` with network disabled before compiling anything.

```text
docker image inspect --format {{.Id}} <offline-go-builder-name>
docker image inspect --format {{.Id}} <carrier-tooling-name>
```

Before using either ID, replace the four `pending-qualifying-stand` values in
`supply.lock.json` with the reviewed Go-builder image ID, Carrier tooling image
ID, module-cache SHA-256, and Carrier binary SHA-256 in one scoped commit. Preparation reads this
lock from `git archive HEAD`; command-line IDs cannot override it. Keeping the
pending values is the intentional no-stand development state and makes final
preparation fail closed.

Pass those IDs to `-go-builder-image-id` and `-tool-image-id` during
`-prepare-final-root`. Preparation rejects tracked worktree changes, creates
and hashes `git archive HEAD`, extracts it to a new external temporary tree,
and builds the product from that tree with `--network none`, `--pull=false`,
and the exact builder ID. The resulting content-addressed product ID and its
embedded source/seven-executable hashes are frozen into `final-spec.json`.
Preparation also verifies the tooling image's base/tool-lock/source/binary
receipt and copies Compose from the same frozen source tree. Product and tool
images must remain preloaded; final workers use `--no-build` and fail if an ID,
receipt, rootfs ancestry, or the pre/post worker Compose hash changes.
The final runner receives only `/usr/bin:/bin`, the local Docker Unix socket,
and an empty owner-only Docker configuration. Ambient Docker contexts,
credential helpers, home directories, and host `PATH` are not inherited.
Preparation exports `/usr/local/bin/network-live.test` from that exact product
image to `<external-spec-root>/runtime/network-live.test`, verifies it against
the product receipt, and returns it as `runner_path`. Use only that path for the
campaign `-runner`; the harness and verifier reject every other hash.

The same frozen binary owns the streaming runner and exact-cell worker. The
current implementation validates the 594-episode suite (564 candidate cells
plus six five-episode evidence-integrity campaigns), verifies the frozen
supply, and dispatches all 144 non-hostile and 420 product-hostile candidate
cells. The six evidence-integrity campaigns are materialized separately from
retained candidate evidence and must independently produce `invalid`. A
selected worker reports its
measured terminal and reads the role-owned path/DNS observer results; only the
parent runner, after the worker exits, removes and rechecks the token-owned
Docker project, verifies the parent-owned worker root is empty, and rejects a
retained process-group descendant. It then marks evidence complete only when
the pre-cleanup terminal marker and final cleanup share the parent monotonic
clock, every batch/exact Endpoint has all nine boundary results, and each
boundary has its own three controls captured on the manifest-bound interface.
Those checksummed controls carry a per-run boundary nonce and use the local
interface MAC, so another namespace or mutable synchronization file cannot
substitute their attribution.
Cells without that complete boundary coverage remain inadmissible. The
separately built verifier reopens the retained observer, telemetry, hostile,
and mutation artifacts and independently derives the candidate and mutation
components. A final `pass` is deliberately impossible until S9.6 supplies the
reviewed stand identities, process-derived runtime allocation attestation, and
the complete qualifying run.
