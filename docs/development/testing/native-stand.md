# Native development stand lifecycle

Use this reference only for the affected owner/profile. [Testing policy](../testing.md)
owns mandatory gates, [evidence limits](../testing.md#evidence-interpretation) and
result validity. The [checked registry](../../../tests/profiles/profiles.json),
[Makefile](../../../Makefile) and linked profile README own execution/prerequisites;
this reference retains the distinct causal oracles, not a second run ledger.

## Allocation, evidence and disposal

## Disposable development stand lifecycle

New local component, preparation and installed controllers must use one stable native
staging root, `/var/lib/ardents-development-work`, under the existing shared exclusive
development lock. Components allocate its fresh `active` child; preparation and
installed cases allocate their original `cases/<case>` path and one separate active
reservation shared with the component lane. A retained active slot blocks another
allocation; a new case name, retry or source identity must not allocate another stand
beside it. Historical controllers and their qualified source/receipt bytes remain
immutable and are not templates for allocating new per-run Docker bind roots.

`scripts/node/run-filesystem-component.py` implements this lifecycle for the existing
isolated Go filesystem-component lane. Invoke it with Python3.12 `-I -S`, an external
evidence directory and `intentN`, inside `run-development.py exclusive`. It consumes
that directory's original `intentN-work.sh` and `sourceN.sha256`; the script uses
`/testtmp/active/tmp` for native temporary files. Source and evidence are read-only
stable mounts; the one native staging mount is `/testtmp`. The existing shared
compiler-cache volume and all profile/test/source checks remain required. Docker Desktop
may retain that permanent integration bind after `--rm`; it does not retain a separately
mounted root for every completed case.

The controller joins its exact container, checks process descriptors/maps, mount
namespaces, retained container configuration and loaded unit references, preserves and
verifies discovery files outside Git, then removes only those known files and empty
original directories. A failed test, changed original, unknown leftover or unavailable
join/reference check retains the occupied slot and an explicit result. No recursive
deletion or automatic recovery is allowed. The caller must still run the shared
compiler-cache retention after the attempt; maintenance results do not replace the test
result.

Preparation and installed controllers call `scripts/node/reserve-native-stand.py` with
the original external evidence directory and `preparationN` or `installedN` inside the
same exclusive invocation, before copying rootfs/source/artifacts. This uses the
component controller's actual `native_stand.py` lifecycle; a reservation remains
occupied after the command returns and across process exits. The returned case path is
original from birth, rather than a renamed or extracted earlier stand. It supplies no
installed permission. Raw native state and histories remain on that original path after
completion. Releasing its separate active reservation requires complete original
process/manager/child join proof and a checked private archive and manifest preserving
bytes, ownership, access, timestamps, links and extended attributes, plus fresh
reference checks and exact duplicate-binary disposition. A finalized case is retained
history, not another active stand or a root available for adoption. Archived native
numbers describe the original observation; extraction cannot recreate original
inode/lease authority or authorize recovery. Unknown state, active references and all
Windows QA histories stay protected. A directory name, inactive unit, process exit or
old passing receipt is insufficient disposition. Missing lifecycle preparation blocks a
new campaign; it is not a passing skip. The component controller supplies no installed
admission or bootstrap acceptance.

Qualification preparation captures its complete explicit immutable input map once with
`scripts/node/prepare-native-inputs.py INPUT_MAP_JSON INPUT_MAP_SHA256
EXTERNAL_DESTINATION`, using root Python3.12 `-I -S` inside the same exclusive wrapper.
`qa_archive.preserve_inputs` physically restores every body and its
access/owner/time/xattrs, rechecks unchanged originals and synchronizes the
content-addressed archive before publishing its verification receipt. Hardlinked or
indirect immutable input paths refuse. This one private input pack supplies the shared
bodies for subsequent case archives; large binary copies must not be recompressed into a
new complete payload archive for every case. The archive, manifest, verification and
disposition paths also require successful directory synchronization, including newly
created destination directories and their parent entries. Unsupported or failed
synchronization is a refusal before duplicate deletion or active-slot release, not a
skipped durability check.

After a genuine original controller finishes, the qualified recipe invokes
`scripts/node/finalize-native-stand.py EXTERNAL_EVIDENCE CASE TERMINAL_JSON
TERMINAL_SHA256`. The closed `ardents-native-case-terminal-v1` record contains `schema`,
`case`, `reservation_sha256`, `test_exit`, `original_scope_joined`,
`original_resources_joined`, pinned `controller` and `result` path/SHA pairs, the
complete `immutable_inputs` path/SHA map, `input_closure` directory and
verification-receipt SHA, and exact `disposable_binary_paths`. Its original in-case
`ardents-native-case-result-v1` binds that same case, reservation and controller SHA,
actual exit and original joins. A new qualified controller must derive those fields from
its actual owner/manager/child completion checks; neither file presence nor this QA
schema can create a missing domain join. Installed execution still requires its
separately accepted exact bootstrap receipt before the controller runs.

A nonzero original test result remains a failed result. If both original physical join
facts are proved and the independently pinned result matches its exact exit, the
finalizer may archive that failed case and retire only its declared verified duplicate
binaries. The complete original result and exit remain in the archive; qualification for
QA disposition grants no product acceptance or retry. Unknown join, exit mismatch,
active references or incomplete archival keep the reservation. A later case requires a
fresh reservation; the original failed root is never adopted, reset or reused.

The finalizer independently matches the original reservation, result, current input
bodies/native metadata and restored immutable pack. It archives the full case through
`qa_archive.preserve`, including native provenance and state, and physically verifies
restoration before calling `qa_duplicates.execute`. Only declared regular, independently
born ELF copies with an exact retained immutable input keeper may be unlinked, including
small auxiliary binaries. Unknown ELF files, foreign/linked paths, missing keepers,
changed inputs, unknown joins or active references retain the slot. For a multi-root
maintenance disposition, all keepers are checked before and after the batch; each root
freshly checks the keepers for its actual unlink targets. A two-root native fixture
observes real SHA reads for each matching keeper at all three boundaries. Input pack and
case archives remain private evidence; they grant no installation, runtime or
extracted-inode authority.

After exact disposition, all other entries remain unchanged except the immediate parent
directory's expected unlink metadata; the original metadata remains archived. State,
histories, links and QA files stay at their original case paths. Only after durable
archive/disposition proof does the finalizer record `case-history/<case>.json` and
release the active reservation. The allocator checks the case/history bijection,
original native identities, referenced proof hashes and fresh physical references before
another allocation. Unknown or orphan cases never become idle by choosing another name.
It creates no per-case Docker mount or compiler cache and performs no global deletion or
process stop.

After a native environment changes its device numbering, finalized QA history may
acquire separate current evidence custody with `python3 -I -S
scripts/node/reobserve-native-history.py EXTERNAL_EVIDENCE`, under the same exclusive
development wrapper. This is an explicit read-only acquisition over finalized case
trees, with new root-only observations outside those trees; it never rewrites original
reservations, terminal results, native numbers, archives or case-history records. Active
or unknown cases cannot use this path. The observer verifies original full QA and input
payload commitments, the exact original joined terminal and disposable-binary set,
disposition, complete retained entry set, every remaining byte, inode, link, access,
owner, timestamp and xattr. Only the device number may differ; the already recorded
unlink parents retain their original narrow size/mtime/ctime exception during
acquisition. The new snapshot captures those parents' actual complete metadata too. A
separate immutable per-case requirement prevents a missing snapshot or a later boot from
selecting the earlier directory-only check, even if the old device number recurs.
Snapshots are named by case, kernel boot and observer-source hash; an observer upgrade
acquires a fresh successor while preserving all earlier snapshots and requirements.
Subsequent allocation rechecks this exact current snapshot and its original archive
commitments, current kernel boot, original history hash and observer bytes. Unknown
entries, changed evidence or stale observations refuse allocation. This supplies current
custody of forensic evidence, never original runtime inode continuity, an active lease,
join, recovery, fixture qualification, bootstrap acceptance or retry. A previous fixture
whose native stand identity changed must be freshly built and qualified; its old
identity cannot be rebound through this observation. New installed execution requires
its own completed receipt and separate Product Owner acceptance as usual.

`scripts/node/check-qa-retention.py EXTERNAL_EVIDENCE` checks this tooling on a fresh
ext4 fixture under root Python3.12 `-I -S` and the existing exclusive wrapper. Before
case allocation it checks real STORED/DEFLATE reads and refuses BZIP2, LZMA, Zstandard,
unknown methods and encryption from literal ZIP headers; a failure-only decompressor
sentinel proves refusal precedes decoding. All archive/disposition readers use that same
closed payload-method check. The fixture runs actual auxiliary controllers, sequential
case finalization, shared-pack reuse, retained state/native
identities/hardlinks/symlinks/xattrs, component/case exclusion and causal unknown-join,
failed-exit, held-reader, foreign-path, corrupted archive/proof and
unavailable-directory-sync refusals. Two actual nonzero controller exits retain their
exact results and state through verified archival, reject forged zero exits and unknown
joins, and require a fresh next-case reservation. Expected failure QA is archived before
the exact disposable fixture teardown. These are tooling fixtures, without a product
installation, accepted bootstrap or installed Forwarding recovery qualification.

## Finalized history disposition

Finalized native development QA may retire duplicate raw cases and archived ELF payloads
only after original scope/resource joins, a fresh zero-reference scan, and complete
non-executable evidence preservation. The separate root-private `history-dispositions`
registry pins the immutable original case-history record, actual cleanup receipt and
old/new archive disposition receipts. Historical verification stays historical; the
disposed archive is not relabeled original. Missing disposition, changed proof,
reappearing raw path, unfinalized case and recycled case name refuse allocation. Every
allocation still runs the same fresh batched equivalent-path/native reference scan,
including disposed path names. Retained cases still require all original proofs and
current native observations.
