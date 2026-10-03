# Admission

Admission owns permission allocation, issuance accounting, holder stock and
irreversible token redemption. Its exact responsibilities, exclusions,
and evidence limits are in
[the boundary review](../../../docs/technical/successor-admission-boundary.md).

The duplicate issuer and key-profile implementations have been removed.
Offline and current-authority issuance share one path:

    issuer.Issue
        -> quota.Ledger.DebitVerified
        -> issuance.ResultStore.Issue

A durable quota debit produces opaque confirmation. Only that confirmation can
authorize private signing and a retained response. Reopening preserves the
debit and exact response; failure never refunds allocation.

| Package | Responsibility |
| --- | --- |
| admission | Permission and batch grammar, token classes, external authority facts |
| quota | Durable issuance quota, exact retry and opaque debit confirmation |
| allocation | Permission allocation decisions and aggregate role maxima |
| issuerprofile | Canonical public key inventory and signed profile |
| issuance | Private issuer material and retained issuance results |
| issuer | Application coordination of quota, key and result owners |
| token | Holder blind state, token challenge, finalization and verification |
| stock | Holder permission, pending exact batch, available tokens and presentation; application executes the exchange |
| attempts | Durable holder presentation history |
| receiving | Receiver binding, bounded verification, redemption, finite allowance and reservation transfer |
| spending | Durable receiver token spend history |

Network currentness, Custody keys, Node identity, physical Hosting budgets,
transport sessions, Introduction registrations and permission file delivery
are outside Admission. External facts do not become authoritative merely by
being put into an Admission struct.

Stock/application separation, currentness re-observation in issuer.IssueCurrent
and receiving policy composition are implemented. The root retains contracts;
quota owns its ledger and confirmation. No additional domain responsibility is
planned. Maintained holder, receiver, allocation and current-issuer consumers
are available in ardents-next; see [standalone commands](../../../docs/technical/successor-admission-commands.md).
Verification combines the behavioral matrix, Linux `make admission-check` and
the repository-wide `make check`. Execution results belong to the issue ledger.

The compiled ardents-next process scenario exercises holder, allocation, current
issuer and receiver commands, durable reopen and cancellation. It does not
certify live Network integration or encrypted Custody. Existing runtime consumers
remain unchanged. Run `make admission-check` on Linux for all Admission packages
and the command scenarios under the race detector.
