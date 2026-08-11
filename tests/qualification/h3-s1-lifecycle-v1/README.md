# H3 S1 lifecycle qualification

This profile owns the S1-2 black-box process tracer. It starts two static TLS
sources and two separately keyed Node processes, executes authenticated probes,
observes refresh withdrawal, waits for old process termination, then restarts
the Nodes against the successor assignment and proves that bounded work resumes
without old/new process overlap. Resource and Docker qualification belong to
the later S1-3 profile.
