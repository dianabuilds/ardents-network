# H3 Node qualification

This profile owns the complete local black-box Node qualification. It starts
two authenticated sources and two separately keyed Node processes, exercises
role probes, refresh withdrawal, restart without overlapping duty, bounded
resource pressure, hostile connections, and terminal cleanup.

`ardents-qualify prepare-node` creates keys, state, plans, clock files, and
evidence outside Git. Compose receives that absolute directory through
`ARDENTS_NODE_ROOT` and mounts only each process's owned state and credentials.

Docker Desktop runs the full qualification matrix locally. A small external
host is optional for a connector check between a local Node and a remote
source; it is not required to host the complete matrix.
