# Installed NET-32 short projection

`run-windows.ps1` installs one explicit `net32-idle` Endpoint plan on an
existing User host. It completes the real Reader permission handover, waits for
ordinary network-ready state, and observes ten minutes with no Service
Connection, publication or Application worker. The runner records raw
one-second whole-owner CPU, RSS and provider-interface counters, reconciles the
hosting ledger, checks the declared State profile is at most 64 KiB, and emits
an upper 24-hour traffic projection against the 1,000,000,000-byte NET-32
ceiling.

This is deliberately labelled a ten-minute observation and projection. It is
not a claimed 24-hour run. Startup occurs before the measurement window. The
plan must refer to existing current State and hosting-period roots; the script
does not create or reset either. Evidence must be a new directory outside the
repository. The mandatory full `SourceCommit` must equal the clean checked-out
HEAD. Evidence includes candidate inputs, environment, the complete
checksummed journal, the independent completion verdict and failed attempts.
