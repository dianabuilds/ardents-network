# Standalone Admission commands

The local consumer is `ardents-next admission`. It operates the successor domain
without the existing Node, Endpoint or Route. These commands do not authenticate
Network state, open a transport or grant physical Hosting capacity. Profile files
are explicit operator assertions for this independent local execution, read anew
at every authority boundary with the current UTC wall clock. Removing/changing
the file withdraws subsequent admission; reading signed bytes alone is never
promoted into trusted Network authority.

## Invocation and input

Every command takes `--config /absolute/file.json`. Unknown/duplicate JSON fields,
null values, malformed input and relative state/profile paths refuse. Configuration
is bounded to 64 KiB, except allocation which accommodates the existing 3 MiB
allocation journal after base64 encoding. Holder and receiver consume one JSON
object per stdin line (maximum 64 KiB) and return one JSON result per line. EOF,
`close`, cancellation or the five-minute command lifetime releases their leases.
Interrupt closes and joins command I/O. Failure diagnostics contain categories,
not keys, file paths or received payloads.

JSON byte slices use base64; fixed byte arrays use arrays of integers. Time values
use RFC3339. Embedded contracts use their exported Go field names. No private
holder or issuer key is accepted by or emitted from these commands.

| Command | Configuration | Work |
| --- | --- | --- |
| `holder` | `root`, `profile`, `role` (1 User, 2 Publisher) | Own one volatile holder and the durable presentation journal |
| `allocate` | `request`, `journal`, `network`, `authority` | Produce an unsigned allocation decision and candidate journal |
| `issue-current` | `plan`, `profile`, `batch`, `kind` | Re-observe facts around the single durable quota/result issuance operation |
| `receiver` | `root`, `profile`, `receiver`, `not_after` | Spend exact tokens and retain one finite allowance receipt |

`profile` names a file containing `admission.AuthorityFacts`, including its complete
class/hour public-key inventory. `plan` is `issuer.Plan`: AdmissionRoot,
AdmissionBinding, KeyRoot, KeyBinding and ResultRoot. Roots are initialized using
the existing quota/key/result commands. `receiver` is `receiving.Receiver`, pinned
to the exact profile, State generation/digest, Node ID and duty generation.
Holder and receiver require existing private empty roots on first use. Holder
journal durability is Linux-only; other platforms refuse opening it.

## Holder operations

The `operation` field selects:

- `request` with `maxima`: return the public holder-signed request and `digest`.
- `import` with `digest`, `payload`: accept the exact signed Permission response.
- `begin` with `intent`: reserve one issuance attempt and return copied `request`
  bytes and its immutable `deadline`. Intent contains Challenges, Selection,
  Bootstrap, Refill and Deadline; delivery selection is supplied by the caller.
- `complete` with `payload`: finalize a successful issuer response. `failed:true`
  records a failed exchange while retaining the exact retry and allocation.
- `discard`: erase the active blind attempt without refunding allocation.
- `take` with `presentation`, `class`: durably mark presentation and return one
  exact token; refusal after that mark never restores it.
- `refill-plan` with `receivers`, `class`, `requires_token`: inspect the bounded
  Control-token replenishment intent, without creating a network operation.
- `status` or `close`: inspect defensive counts or revoke volatile holder state.

No command executes a hidden exchange, automatically chooses another recipient,
or rebuilds an expired request with a new deadline.

## Independent end-to-end use

1. Start `holder`, send `request`, and hand the public request to `allocate` with
   the authority's current retained allocation journal.
2. `allocate` returns `unsigned-decision`, Permission, candidate journal and an
   exact-repeat flag. The external Custody owner must serialize, durably commit
   and read back the candidate before purpose-specific signing. The command
   neither signs nor maintains a second unencrypted authoritative journal.
3. Send the externally signed response to holder `import`; send `begin` with
   exact receiver challenges. Pass its batch to `issue-current`.
4. Send the issuer response to holder `complete`, then `take` with the exact
   receiver and presentation nonce. Present the returned token to `receiver`.
5. Receiver `accept` takes `token`, `class`, `deadline`. `refill` takes another
   `token` and `remaining`; it replaces allowance without extending its deadline.
   `release` retires the receipt. This command has no physical workload and
   deliberately acquires no Hosting reservation. Reopening its root still
   rejects an already spent token.

The compiled-process scenario is
`TestAdmissionStandaloneCommandsIssuePresentReceiveAndReopen` in
`cmd/ardents-next/admission_cycle_linux_test.go`. It checks unsigned allocation,
external commit-before-sign sequencing, exact issuance replay, holder restart,
foreign receiver duty refusal, positive redemption, refill and spend replay after
restart. Its external signing fixture is not encrypted Custody qualification.
