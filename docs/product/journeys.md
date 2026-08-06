# Product journeys

These journeys define observable product behavior. They deliberately avoid
selecting protocols, libraries, or implementation languages.

## J-01 — First launch

**Actor:** Person

**Start:** A newly installed Ardents Client

**Flow:** Create Vault → protect recovery method → name Device → create first
Persona

**Done when:** no phone, email, wallet, or central registration was required;
recovery was verified rather than merely displayed.

## J-02 — Open a private site

**Actor:** Person

**Start:** A human-readable Service Name or Invite

**Flow:** Resolve name → verify proof and expiry → select safe route → retrieve
and authenticate release → render in isolation

**Done when:** the intended release opens and publisher and visitor have not
learned each other's network location within the Interactive Route contract.

## J-03 — Publish a site or client application

**Actor:** Developer

**Start:** Project assets and a desired Service Name

**Flow:** obtain name → declare Capabilities → lint → build reproducibly → sign →
select Replicas → publish

**Done when:** another Client can verify and open the release after one Replica
is made unavailable.

## J-04 — Talk asynchronously to a Service

**Actors:** Person and Private Service

**Start:** An opened Service

**Flow:** approve mailbox Capability → send through Shielded Route → Person goes
offline → Service replies → Client later retrieves reply

**Done when:** payload remains end-to-end protected, delivery survives offline
time, and the Service receives no universal Person identifier.

## J-05 — Establish a Contact

**Actors:** Two People

**Start:** QR, Invite, or comparison in another trusted context

**Flow:** verify invitation → accept Message Request → establish pairwise
relationship → assign local display name

**Done when:** the relationship is authenticated without a public directory and
does not link either Person's unrelated Personas.

## J-06 — Create a private Space

**Actor:** Space steward

**Start:** One or more Contacts

**Flow:** create Space → choose recovery policy → invite members → delegate names
→ install Service → choose admission policy

**Done when:** members can collaborate under Space-scoped Personas, Capabilities,
and names without creating a global authority.

## J-07 — Recover after losing a Device

**Actor:** Person

**Start:** Protected Recovery Root and a replacement Device

**Flow:** authorize replacement → recover selected Personas → revoke lost Device
→ re-establish sessions as required

**Done when:** the lost Device cannot obtain future protected data and unrelated
Personas were neither exposed nor unnecessarily rotated.

## J-08 — Continue under network blocking

**Actor:** Person

**Start:** Ordinary bootstrap or entry path is blocked

**Flow:** detect failure class → obtain a Bridge through an alternate channel →
reconnect → rotate exposed entry metadata

**Done when:** normal product use resumes without manual protocol configuration
and without claiming that the Bridge alone creates anonymity.

## J-09 — Contribute resources

**Actor:** Network Contributor

**Start:** A host with bounded bandwidth, disk, or later compute

**Flow:** install → choose roles and limits → run self-check → participate →
observe health → leave gracefully

**Done when:** the Node helps the network without reading protected content,
owning users, or leaving unfulfilled retained-data obligations.

## Cross-journey failure cases

Every implementation proposal must exercise at least these cases:

- one ordinary relay is malicious, slow, or absent;
- one Replica disappears permanently;
- a name record is stale, expired, rolled back, or equivocating;
- a Device is stolen while another Device is offline;
- sender or recipient stays offline beyond normal retry intervals;
- a malicious Service requests excessive Capabilities or fingerprints the Client;
- a censor blocks known entry addresses and protocol fingerprints;
- nominally different Nodes share one operator, network, or jurisdiction;
- an official update channel is compromised or unavailable.
