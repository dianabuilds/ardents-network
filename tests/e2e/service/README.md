# Service command end to end

These tests build the Service and Application commands and exercise their
public Unix-socket boundaries. They cover readiness timeout and cleanup plus an
Exact-Target stream that retains one Service Connection when its current Route
fails and the first replacement also fails. The recovery scenario mocks only
the external Route socket; Service, publication, Application IPC, continuity,
and workload processes are real commands.

The Reference C-2 scenario starts seven bounded fixture roles — Publisher, User,
Destination Resolution Gateway, Introduction, Initiator, Rendezvous, and
Responder — as separate processes. Its transit duties reopen accepted signed
native-route State, require one valid refresh wave from two mutually
authenticated State Source processes, and then drain through their maintained
Node lifecycle. It drives the selected Target Link through the private lookup
Entry/Initiator/Gateway carrier and then C-2, and verifies the bounded static
Reference Site document, declared stylesheet, and declared SVG. In its
explicit-browser mode it waits for the Publisher to observe all three requests;
opening Firefox alone is not accepted as evidence.
Its separate offline case first retains an authentic published descriptor, then
removes the Publisher's local Introduction slot. The User must report bounded
`service unavailable`, receive no Reference URL, and make no browser request.
For an explicitly cross-compiled Linux qualification only, a
test runner may provide the matching fixture through
`ARDENTS_E2E_FIXTURE_REFERENCE_C2`; ordinary local and CI runs leave it unset
and rebuild the fixture from the current checkout. Both C-2 scenarios,
including the declared document, stylesheet, SVG, and offline result, passed
from a clean local Linux Go container on 2026-08-25. This is process and
Linux-runtime evidence, not a multi-host qualification claim.
On a Windows qualification host with an explicitly selected Firefox executable, set
`ARDENTS_REFERENCE_C2_FIREFOX` to its absolute path. The User then passes only
the already-authenticated scoped loopback Reference URL to the maintained
Firefox launcher; the scenario has no browser profile, proxy, DNS, VPN, or
trust-store configuration authority. This opt-in run passed with Firefox 154
on 2026-08-25. It is browser compatibility evidence for this exact C-2 fixture,
not an H4-7 browser-isolation or location-privacy claim.
