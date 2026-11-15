# Go ownership guidance

Follow Ardents' single module, package map and cohesive domain packages.

- Use unexported state and intention-revealing operations for actual invariants.
  Plain values and pure functions suit codecs and decisions; getters alone do
  not make a rich model.
- Define interfaces at real consumer seams only when needed. Repository per
  aggregate, embedded AggregateRoot and generic publisher are not requirements.
- Context cancellation, goroutines, callbacks, descriptors and Close/Wait results
  belong to the operation contract. Cancellation requests completion; it does
  not prove join. Preserve original and cleanup failures.
- Keep foreign I/O and joins outside state locks. A local commit barrier compares
  original local facts under documented ordering, not foreign policy callbacks.
- Pass narrow owner-specific proofs/borrows, not shared mutable contexts, private
  keys, live roots or accepting booleans.
- Extract common native mechanisms with independent ownership per invocation.
  Similar role transitions may have different policy.
- Preserve canonical bytes, bounds and floors. Package renaming grants no wire
  or storage migration.

Use existing behavior/race/profile machinery. Tests should distinguish invariant
violations at real boundaries, not mirror method structure or manufacture an
otherwise unused API's consumer.
