# Service command end to end

These tests build the Service and Application commands and exercise their
public Unix-socket boundaries. They cover readiness timeout and cleanup plus an
Exact-Target stream that retains one Service Connection when its current Route
fails and the first replacement also fails. The recovery scenario mocks only
the external Route socket; Service, publication, Application IPC, continuity,
and workload processes are real commands.

The Reference C-2 scenario starts seven synthetic roles — Publisher, User,
Destination Resolution Gateway, Introduction, Initiator, Rendezvous, and
Responder — as separate processes. It drives the selected Target Link through
the private lookup Entry/Initiator/Gateway carrier and then C-2, verifies the
bounded static Reference Site response, and has each transit role itself prove
its drained terminal usage.
For an explicitly cross-compiled Linux qualification only, a
test runner may provide the matching fixture through
`ARDENTS_E2E_FIXTURE_REFERENCE_C2`; ordinary local and CI runs leave it unset
and rebuild the fixture from the current checkout. The same test passed in a
clean local Linux Go container on 2026-08-25. This is process and Linux-runtime
evidence, not a browser or multi-host qualification claim.
