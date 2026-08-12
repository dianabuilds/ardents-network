# H3 Node qualification

This profile owns the complete local black-box Node qualification. It starts
two authenticated sources and two separately keyed Node processes, exercises
role probes, refresh withdrawal, restart without overlapping duty, bounded
resource pressure, hostile connections, and terminal cleanup.

`ardents-qualify prepare-node` creates keys, state, plans, clock files, and
evidence outside Git. Compose receives that absolute directory through
`ARDENTS_NODE_ROOT` and mounts only each process's owned state and credentials.
On a native Linux Docker Engine host, preparation must include
`--linux-uid-ownership` so the fixed Compose role UIDs own only their declared
paths. Ordinary unit-test preparation omits that flag and remains unprivileged.

Docker Desktop may run the development matrix locally without making a
qualification claim. An official Stage 1 campaign requires the preflighted
dedicated Ubuntu Docker Engine/cgroup-v2 environment defined by the Stage 1
brief; a local run is not a substitute.
