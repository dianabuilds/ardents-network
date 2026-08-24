# Endpoint command end to end

This package builds the real `ardents` command and runs it from the exact
artifact placed in a synthetic closed-alpha bundle. It proves that
`endpoint enrollment-check` accepts an independently pinned, exact local
inventory and refuses a changed manifest before parsing it.

The bundle uses test bytes and no signer, network source, Release Decision, or
participant contact. It is process evidence for the Enrollment Pin boundary;
it is not a successful enrolled runtime or alpha-provenance qualification.

For an explicitly cross-compiled Linux qualification only, the test runner may
provide the matching command through `ARDENTS_E2E_COMMAND`; ordinary local and
CI runs leave it unset and build the command from the current checkout.
