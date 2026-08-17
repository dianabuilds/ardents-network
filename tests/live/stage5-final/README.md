# Stage 5 final campaign inputs

These versioned files define the non-secret S5.5 topology, cgroup, network,
workload, and observer contracts. They are copied to a new external
configuration root before `blocked-entry-lab -prepare-final-root` runs.

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

This directory is not evidence and cannot satisfy S5.5 by itself. The final
campaign remains incomplete until the frozen bundle runs on the qualifying
non-overcommitted host and the independent verifier emits `pass`.

Build the maintained runner outside the repository with:

```text
go test -tags=live -c -o <external>/network-live.test ./tests/live/network
```

The same frozen binary will own the streaming runner and exact-cell worker. The
current implementation validates the 594-cell schedule and maps the 144
non-hostile cells, but rejects every mapped worker before startup until real
terminal/observer/cleanup evidence, bounded output, and frozen source/image
hooks exist. The 450-cell hostile matrix is also unimplemented, so this slice
cannot produce `pass`.
