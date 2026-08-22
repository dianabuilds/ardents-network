# Scoped high-risk exceptions

Status: **no active exceptions.** First-party `unsafe`, cgo, and implicit
`init` are forbidden unless this register names one exact source path, accepted
superseding ADR, dedicated risk test, affected platform, owner, and removal or
revalidation condition. A broad directory, package, or future placeholder is
not an exception.

Before an admitted row can take effect, `internal/architecture` must check it
against the exact source import/declaration and named test. Removing an
exception removes the source use and this row together. A changed path, risk,
platform, ADR, or test invalidates the row rather than inheriting approval.

No active exceptions are registered at the Stage 8 entry.
