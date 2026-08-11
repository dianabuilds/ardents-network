# Maintained laboratories

This directory contains only human-authored container, topology, and immutable
supply inputs for closed reproducible laboratories.

- `carrier/` belongs to the completed Carrier Lab and R-013 comparison.
- `named-site/` belongs to the completed Gate C Named Unlisted Site tracer.

The maintained Go implementations live under `internal/lab/`; thin executable
adapters live under `cmd/*-lab`. These laboratories are not product runtime,
packaging, deployment, or reusable infrastructure. Future product Modules live
under factual `internal/<responsibility>` packages and never import laboratory
code. A later Delivery Horizon does not extend a closed laboratory in place.

Generated dependencies, images, keys, sockets, captures, run state, and evidence
remain outside the repository.
