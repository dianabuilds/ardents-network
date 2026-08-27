# R-110 foreground replacement prototype — throwaway

Question: can an explicitly invoked unprivileged helper own the small
`stage → stop → atomic activate → self-test → restart` state machine without
touching Authority state or inferring a successful recovery?

Run from the repository root:

```sh
sh experiments/r-110-safe-endpoint-replacement/run.sh
```

The script creates a fresh scratch tree, prints every relevant state after each
transition, and removes that tree on exit. It is deliberately **not product
code**, does not call `internal/update`, authenticate any candidate, manage a
unit, or prove crash durability. It only falsifies an ownership/model question:
the helper can leave program bytes and protected roots separate while retaining
an unambiguous recovery record.

Expected result: an active replacement is refused; interruption before rename
keeps v1; interruption after rename leaves v2 plus `self-test-required` rather
than pretending commit; self-test failure requires separately authorized
rollback; only a successful self-test permits restart. Record the conclusion in
R-110, then delete or replace this prototype.
