#!/usr/bin/env python3
"""Selected private Source startup qualification after direct PO acceptance.

Do not invoke this profile before the completed exact public bootstrap receipt
has been directly accepted by the Product Owner. Recording its pin here is
cohort identification, never technical evidence of that human decision or a
runtime permission. Product commands obtain genuine fresh owner admission.

The outside controller owns the private rootfs/manager and original whole-scope
events descriptor, places this caller and children in its supervisor cgroup,
retains the development lock, and joins every descendant before returning it.
Missing prerequisites fail. No host-wide changes, reset, retry/adoption or
Source protection override is admitted. Each selected case needs a fresh stand.
"""
import base64
from datetime import datetime
import errno
import hashlib
import json
import os
from pathlib import Path
import platform
import queue
import signal
import socket
import stat
import subprocess
import sys
import threading
import time


RECOVERY_NATIVE_TESTS = (
    "TestSourceRecoveryPhysicalNativeProcessCuts",
    "TestSourcePhysicalRefusalCloseRetainsIndependentFailure",
    "TestSourceTransitionNativeOriginalScope",
    "TestSourceTransitionNativePacketRefusalReleasesReceivedDescriptors",
    "TestSourceTransitionNativePacketClosedGrammar",
    "TestSourceRecoveryPrefixNativeExclusionAndRefusalCleanup",
    "TestSourceRecoveryPrefixNativeMutationAndCopiedLineage",
    "TestSourceRecoveryPrefixNativeCanceledReadAndCloseJoin",
    "TestSourceRecoveryCompletionNativeCustodyAndCrossFileReobservation",
    "TestSourceRecoveryCompletionNativeAcquisitionRefusals",
    "TestSourceRecoveryCompletionNativeCanceledReadJoinsBeforeReturn",
    "TestSourceRecoveryWritingNativeBirthsAndPartialEvidence",
    "TestSourceRecoveryWritingNativeCompleteGrammarEnvelope",
    "TestSourceRecoveryWritingNativeCloseWaitsAndRetainsLateFailure",
    "TestSourceRecoveryFileNativeContinuesOnlyOriginalEmptyBirth",
    "TestSourceRecoveryFileNativeRefusesUnknownOrSubstitutedWrite",
    "TestSourceRecoveryOutcomeNativePrefixDoesNotProveOriginalSuccess",
    "TestSourceRecoveryInitialPendingNativePublicationAfterClose",
    "TestSourceRecoveryInitialPendingNativeFailureRetainsGuard",
    "TestSourceRecoveryInitialEvidenceNativePresenceForcesRecoveredPath",
)


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def pinned(name):
    prefix = "ARDENTS_SOURCE_STARTUP_" + name
    path = Path(os.environ[prefix])
    wanted = os.environ[prefix + "_SHA256"]
    if not path.is_absolute() or path.is_symlink() or not path.is_file() or len(wanted) != 64 or digest(path) != wanted:
        raise RuntimeError("original pinned profile input unavailable: " + name)
    return path


def source_profile(arguments, requested_case):
    """Select a finite qualification operation; this grants no runtime right."""
    if arguments not in (["-timeout=3m"], ["-timeout=8m"], ["-timeout=10m"]):
        raise RuntimeError("fixed Source qualification arguments required")
    recovery_profile = arguments == ["-timeout=10m"] and requested_case.startswith("recovery-")
    replacement_profile = arguments == ["-timeout=10m"]
    restart_profile = recovery_profile or replacement_profile or arguments == ["-timeout=8m"]
    bound = 600 if recovery_profile or replacement_profile else 480 if restart_profile else 180
    case, operation, count, fault = requested_case, None, None, None
    if recovery_profile:
        # A positive frontier inventory does not cover unsafe physical windows
        # or replace the separately required causal-negative matrix.
        counts = {"initial": (70, 106, 110, 113, 114, 115), "replacement": (13, 49, 50, 51, 52, 53, 54, 55)}
        selected = {f"recovery-{op}-{number}": (op, number) for op, values in counts.items() for number in values}
        faults = ("held-journal-lease", "copied-journal", "copied-cut-result", "journal-access", "torn-journal", "missing-pending", "partial-completion", "copied-generation-file", "manager-reexec")
        negatives = {f"recovery-refusal-{op}-{reason}": (op, number, reason)
                     for op, number in (("initial", 70), ("replacement", 49)) for reason in faults}
        # These are distinct predecessor facts. A real process exit, complete
        # candidate or another retained proof cannot replace a missing one.
        negatives.update({f"recovery-refusal-replacement-{reason}": ("replacement", 49, reason)
                          for reason in ("missing-startup-journal", "missing-startup-archive",
                                         "missing-retirement-journal", "missing-retirement-result")})
        if case in negatives:
            operation, count, fault = negatives[case]
        elif case in selected:
            operation, count = selected[case]
        else:
            raise RuntimeError("unknown finite recovery case")
        replacement_profile = operation == "replacement"
        case = "replacement" if replacement_profile else "restart"
    replacement_cases = ("replacement", "replacement-credential-loss", "replacement-state-loss", "replacement-candidate-collision", "replacement-selection-readonly", "replacement-unit-readonly", "replacement-account-substitution", "replacement-binding-substitution", "replacement-unit-substitution", "replacement-expired-release", "replacement-reload-cancel")
    replacement_cases += tuple("replacement-history-" + fault for fault in ("before-lock", "malformed-attempt", "gapped-attempt", "transcript", "retirement"))
    cases = replacement_cases if replacement_profile else ("restart", "restart-credential-loss", "restart-state-loss") if restart_profile else ("positive", "credential-substitution", "clock-loss")
    if case not in cases:
        raise RuntimeError("unknown finite Source startup case")
    return {"recovery": recovery_profile, "replacement": replacement_profile, "restart": restart_profile, "bound": bound,
            "case": case, "requested_case": requested_case, "operation": operation, "prefix_records": count, "fault": fault}


def recovery_fault_retention(installation, generation):
    # Keep native originals on the same parent filesystem, outside both the
    # generation inventory and checkReplacementJournals' evidence prefix.
    retained = installation.parent / (".retained-source-fault-" + generation)
    if retained.is_relative_to(installation) or retained.name.startswith(installation.name + ".source-replacement-"):
        raise RuntimeError("fault retention conflicts with product evidence")
    return retained


class RecoveryRefusalComplete(Exception):
    """Outside profile control flow after a fully observed terminal refusal."""


def main():
    profile = source_profile(sys.argv[1:], os.environ["ARDENTS_SOURCE_STARTUP_CASE"])
    recovery_profile, replacement_profile, restart_profile = (profile[name] for name in ("recovery", "replacement", "restart"))
    bound, case, requested_case = (profile[name] for name in ("bound", "case", "requested_case"))
    recovery_operation, recovery_count, recovery_fault = (profile[name] for name in ("operation", "prefix_records", "fault"))
    if not sys.flags.isolated or not sys.flags.no_site or sys.version_info[:2] != (3, 12):
        raise RuntimeError("fixed isolated Python3.12 profile required")
    if sys.platform != "linux" or os.geteuid() != 0:
        raise RuntimeError("private native Linux root controller required")
    release = platform.freedesktop_os_release()
    if (release.get("ID"), release.get("VERSION_ID")) != ("ubuntu", "24.04"):
        raise RuntimeError("selected Ubuntu24.04 profile required")
    if Path("/etc/ardents-source-startup-native-stand").read_text() != "ardents-source-startup-native-v1\n" or Path("/run/systemd/container").read_text().strip() != "ardents-source-startup":
        raise RuntimeError("original private Source startup manager required")
    for name in ("mnt", "pid", "cgroup", "uts"):
        a, b = Path("/proc/1/ns/" + name).stat(), Path("/proc/self/ns/" + name).stat()
        if (a.st_dev, a.st_ino) != (b.st_dev, b.st_ino):
            raise RuntimeError("original caller/manager namespaces differ")
    if Path("/proc/self/cgroup").read_text() != "0::/../supervisor\n":
        raise RuntimeError("caller must belong to the outside original stand supervisor scope")
    probe = os.pidfd_open(os.getpid(), 0)
    try:
        signal.pidfd_send_signal(probe, 0)
    finally:
        os.close(probe)
    initial_format = os.environ.get("ARDENTS_SOURCE_STARTUP_INITIAL_JOURNAL_FORMAT")
    if replacement_profile and initial_format not in ("legacy112", "recoverable115"):
        raise RuntimeError("exact predecessor initial producer format required in the checked cohort")
    if recovery_profile and not replacement_profile and initial_format != "recoverable115":
        raise RuntimeError("positive initial recovery requires the exact115 producer")
    root = Path(__file__).resolve().parents[2]
    manifest = json.loads(pinned("SOURCE_MANIFEST").read_text())["files"]

    def source():
        names = {p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()}
        if not manifest or names != set(manifest):
            raise RuntimeError("complete original source snapshot required")
        for name, wanted in manifest.items():
            path = root / name
            if Path(name).is_absolute() or ".." in Path(name).parts or path.is_symlink() or digest(path) != wanted:
                raise RuntimeError("source differs from actual candidate input")

    source()
    paths = {name: pinned(name) for name in ("NODE", "CONTROL", "MANAGER_TEST", "REQUEST", "BUNDLE", "EPOCH", "UNSIGNED_EPOCH", "CLIENT_CERTIFICATE", "CLIENT_KEY", "SERVER_CA", "PUBLIC_RECEIPT")}
    if replacement_profile:
        names = ("NODE_B", "NODE_C", "REQUEST_B", "REQUEST_C", "OWNER_TEST") + (() if recovery_profile else ("NODE_EXPIRED",))
        paths.update({name: pinned(name) for name in names})
    recovery_matrix = None
    if recovery_profile:
        paths.update({name: pinned(name) for name in ("RECOVERY_OBSERVER", "RECOVERY_MATRIX")})
        with paths["RECOVERY_MATRIX"].open("rb") as stream:
            matrix_raw = stream.read((16 << 20) + 1)
        if len(matrix_raw) > 16 << 20:
            raise RuntimeError("recovery matrix exceeds finite bound")
        recovery_matrix = json.loads(matrix_raw)
        if recovery_matrix["schema"] != "source-recovery-static-predictions-v1" or any(manifest.get(name) != sha for name, sha in recovery_matrix["source_sha256"].items()):
            raise RuntimeError("exact published source-bound recovery matrix required")
    if case == "replacement-reload-cancel":
        paths["CUT_TEST"] = pinned("CUT_TEST")
    distribution = Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION"])
    if paths["NODE"] != distribution / "ardents-node-linux-amd64":
        raise RuntimeError("execute the actual enrolled initial program")
    active_node = paths["NODE"]
    initial_distribution = distribution
    if replacement_profile:
        candidate_suffixes = ("B", "C") if recovery_profile else ("B", "C", "EXPIRED")
        for suffix in candidate_suffixes:
            candidate = Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_" + suffix])
            if paths["NODE_" + suffix] != candidate / "ardents-node-linux-amd64":
                raise RuntimeError("execute each actual candidate program")
        if digest(paths["NODE"]) == digest(paths["NODE_B"]) or any(digest(paths["NODE_B"]) != digest(paths["NODE_" + suffix]) for suffix in candidate_suffixes if suffix != "B"):
            raise RuntimeError("actual program replacement followed by unchanged-program resource replacement required")
    request = json.loads(paths["REQUEST"].read_text())
    binding = Path(request["installation_root"]) / "binding.json"
    history = os.environ["ARDENTS_SOURCE_STARTUP_HISTORY"]
    network = os.environ["ARDENTS_SOURCE_STARTUP_NETWORK"]
    output = Path(os.environ["ARDENTS_SOURCE_STARTUP_OUTPUT"])
    if not output.is_absolute() or output.exists() or output.is_symlink() or output.is_relative_to(root):
        raise RuntimeError("fresh private evidence output required")
    os.umask(0o077)
    output.mkdir(mode=0o700)
    interrupted = threading.Event()
    for kind in (signal.SIGTERM, signal.SIGINT):
        signal.signal(kind, lambda *_: interrupted.set())
    started = time.monotonic()
    clock_stop = threading.Event()
    clock_errors = []
    clock_thread = None
    clock_fd = None
    scope_fd = None
    source_pid = None
    source_pidfd = None
    invocation = None
    source_started = False
    native_observer = None
    failures = []
    outcome = {"case": requested_case, "installed_qualification": False, "original_scope_joined": False,
               "public_receipt_sha256": os.environ["ARDENTS_SOURCE_STARTUP_PUBLIC_RECEIPT_SHA256"],
               "source_manifest_sha256": os.environ["ARDENTS_SOURCE_STARTUP_SOURCE_MANIFEST_SHA256"]}
    recovery_evidence = {}

    def observe():
        source()
        for name, path in paths.items():
            if pinned(name) != path:
                raise RuntimeError("original profile declaration changed")
        if interrupted.is_set() or clock_errors or time.monotonic() - started >= bound:
            raise RuntimeError("original caller/clock/profile bound unavailable")

    def run(argv, name, refusal=False, **options):
        observe()
        # Direct children finish through their actual product owner cleanup.
        # The outside original scope still owns interruption/descendant join.
        child_failures = []
        with (output / (name + ".log")).open("xb") as log:
            child = subprocess.Popen([str(a) for a in argv], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **options)
            drain_errors = []

            def drain():
                total = 0
                try:
                    while True:
                        raw = child.stdout.read(65536)
                        if not raw:
                            break
                        total += len(raw)
                        if total > 32 << 20:
                            if not drain_errors:
                                drain_errors.append(RuntimeError("original product output exceeds finite evidence bound"))
                        elif not drain_errors:
                            try:
                                log.write(raw)
                            except OSError as error:
                                drain_errors.append(error)
                except BaseException as error:
                    drain_errors.append(error)
                finally:
                    try:
                        child.stdout.close()
                    except OSError as error:
                        drain_errors.append(error)

            reader = threading.Thread(target=drain)
            signalled, notice = False, False
            try:
                reader.start()
                while child.poll() is None:
                    if not signalled and (interrupted.is_set() or clock_errors or drain_errors or time.monotonic() - started >= bound):
                        # Signal only the original unreaped direct child. Its
                        # owner joins original work before this actual return.
                        signalled = True
                        child.send_signal(signal.SIGTERM)
                    if not notice and time.monotonic() - started >= bound:
                        print("Original product completion unavailable; retaining child and stand lock", file=sys.stderr)
                        notice = True
                    time.sleep(0.02)
            except BaseException as error:
                child_failures.append(error)
            finally:
                if child.poll() is None:
                    try:
                        child.send_signal(signal.SIGTERM)
                    except OSError as error:
                        child_failures.append(error)
                returncode = child.wait()  # Never abandon the original child.
                if reader.ident is not None:
                    while reader.is_alive():
                        reader.join(0.1)
                        if not notice and time.monotonic() - started >= bound:
                            print("Original output completion unavailable; retaining reader and stand lock", file=sys.stderr)
                            notice = True
                else:
                    child.stdout.close()
                child_failures.extend(drain_errors)
        try:
            observe()
        except BaseException as error:
            child_failures.append(error)
        if returncode != 0 and not refusal:
            child_failures.append(RuntimeError("original product operation failed: " + name))
        if child_failures:
            raise BaseExceptionGroup("original product operation/observation failed", child_failures)
        raw = (output / (name + ".log")).read_bytes()
        return (returncode, raw) if refusal else raw

    def native_identity(info):
        return (info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode), info.st_uid, info.st_gid)

    def native_image(path, maximum=4 << 20):
        # Regular/nonblocking/nofollow input, bounded before allocation, with
        # native identity and size observed again after the actual read.
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
        try:
            stream = os.fdopen(descriptor, "rb")
        except BaseException as error:
            try:
                os.close(descriptor)
            except OSError as cleanup:
                raise BaseExceptionGroup("evidence stream acquisition and descriptor Close failed", [error, cleanup])
            raise
        with stream:
            before = os.fstat(stream.fileno())
            if not stat.S_ISREG(before.st_mode) or not 0 <= before.st_size <= maximum:
                raise RuntimeError("bounded original regular evidence required")
            raw = stream.read(maximum + 1)
            after, named = os.fstat(stream.fileno()), path.lstat()
            if len(raw) != before.st_size or native_identity(before) != native_identity(after) or native_identity(before) != native_identity(named) or before.st_size != after.st_size:
                raise RuntimeError("original native evidence changed during read")
        return raw, native_identity(before)

    def canonical_rows(raw):
        if not raw or len(raw) > 4 << 20 or not raw.endswith(b"\n"):
            raise RuntimeError("complete bounded canonical records required")
        rows = [json.loads(line) for line in raw.splitlines()]
        if len(rows) > 115 or [row["sequence"] for row in rows] != list(range(len(rows))):
            raise RuntimeError("canonical original sequence differs")
        if any(json.dumps(row, separators=(",", ":"), ensure_ascii=False).encode() != line for row, line in zip(rows, raw.splitlines(), strict=True)):
            raise RuntimeError("canonical original spelling differs")
        return rows

    def recovery_snapshot(declaration, installation=False):
        # This is a finite private-stand preservation oracle, not State or
        # Release admission. It does not export private bytes or repair files.
        saved, total = {}, 0
        roots = [Path(declaration[name]) for name in ("state_root", "participation_root", "credential_directory")] + [Path(history)]
        if installation:
            roots.append(Path(declaration["installation_root"]))
        for directory in roots:
            info = directory.lstat()
            if not stat.S_ISDIR(info.st_mode):
                raise RuntimeError("original semantic/credential directory missing")
            saved[str(directory)] = ("directory", native_identity(info))
            for path in directory.rglob("*"):
                if len(saved) >= 4096:
                    raise RuntimeError("private preservation snapshot exceeds finite inventory")
                info = path.lstat()
                if stat.S_ISDIR(info.st_mode):
                    saved[str(path)] = ("directory", native_identity(info))
                else:
                    raw, identity = native_image(path, 32 << 20)
                    total += len(raw)
                    if total > 128 << 20:
                        raise RuntimeError("private preservation bytes exceed finite bound")
                    saved[str(path)] = ("file", identity, hashlib.sha256(raw).hexdigest())
        account = {"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}
        account.update(row["resource"] for row in recovery_matrix["canonical_prefixes"] if row["operation"] == "initial-recoverable" and row["phase"] == "account-auxiliary-preimage")
        for name in sorted(account):
            path = Path(name)
            if not path.exists() and not path.is_symlink():
                saved[name] = ("absent",)
            else:
                raw, identity = native_image(path, 32 << 20)
                saved[name] = ("file", identity, hashlib.sha256(raw).hexdigest())
        return saved

    def recovered_transcript():
        # Only the exact original journal from this actual cut has a suffix
        # route in this outside oracle. Startup/retirement remain complete-only.
        evidence = recovery_evidence
        for path, expected in evidence["immutable"].items():
            raw, identity = native_image(Path(path))
            if (identity, hashlib.sha256(raw).hexdigest()) != expected:
                raise RuntimeError("original cut or recovery completion changed")
        return evidence["transcript"]

    def cut_and_recover(command, operation, declaration_path):
        if recovery_evidence or operation != recovery_operation:
            raise RuntimeError("one exact recovery cut per fresh stand required")
        declaration = json.loads(declaration_path.read_text())
        installation = Path(declaration["installation_root"])
        generation = digest(Path(command[3]) / "infrastructure-source.json")
        journal = Path(str(installation) + (".source-initial-journal" if operation == "initial" else ".source-replacement-" + generation + "-journal"))
        pending = Path(str(installation) + ".source-initial-pending") if operation == "initial" else installation / "source-replacement-pending"
        matrix_operation = "initial-recoverable" if operation == "initial" else "replacement"
        phases = [row["phase"] for row in recovery_matrix["canonical_prefixes"] if row["operation"] == matrix_operation]
        row = [row for row in recovery_matrix["canonical_prefixes"] if row["operation"] == matrix_operation and row["prefix_records"] == recovery_count]
        if len(row) != 1 or row[0]["predicted_outcome"] != "conditional-stopped-continuation":
            raise RuntimeError("published matrix does not admit the selected positive frontier")
        if recovery_fault is not None:
            predictions = [value for value in recovery_matrix["causal_refusals"] if value["operation"] == operation and value["prefix_records"] == recovery_count and value["fault"] == recovery_fault]
            if len(predictions) != 1 or predictions[0]["predicted_outcome"] != "repair-required" or predictions[0]["failure_header_permitted"] != (recovery_fault in ("missing-pending", "copied-generation-file")):
                raise RuntimeError("exact published terminal refusal prediction required")
        profile = {"schema": "source-original-cut-observer-v1", "program_sha256": digest(Path(command[0])),
                   "public_receipt_sha256": outcome["public_receipt_sha256"], "arguments": [str(value) for value in command[1:]],
                   "journal": str(journal), "prefix_records": recovery_count, "last_phase": row[0]["phase"]}
        profile_path = output / "original-cut-profile.json"
        with profile_path.open("xb") as stream:
            stream.write(json.dumps(profile, separators=(",", ":")).encode() + b"\n")
        observer_environment = dict(os.environ, ARDENTS_SOURCE_CUT_OBSERVER_MODE="installed-cut",
                                    ARDENTS_SOURCE_CUT_OBSERVER_PROFILE=str(profile_path), ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256=digest(profile_path),
                                    ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=str(command[0]), ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(installation),
                                    ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(output / "original-interrupted-command.log"))
        report_raw = run([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceCutObserverNative$", "-test.v", "-test.timeout=3m"], "original-kernel-cut", env=observer_environment)
        reports = [json.loads(line[len(b"SOURCE_CUT_OBSERVATION "):]) for line in report_raw.splitlines() if line.startswith(b"SOURCE_CUT_OBSERVATION ")]
        if len(reports) != 1:
            raise RuntimeError("one actual original-cut observer result required")
        report = reports[0]
        if report["inspection_only"] or not report["installed_execution"] or not report["external_cancellation_acknowledged"] or report["root_exit"] != 2 or report["selected_records"] != recovery_count or report["acknowledgement_hits"] != recovery_count - 1 or report["threads_joined"] < 2 or report["processes"] < 2:
            raise RuntimeError("actual original root cancellation/frontier/native join missing")
        original, identity = native_image(journal)
        rows = canonical_rows(original)
        if len(rows) != recovery_count or [value["phase"] for value in rows] != phases[:recovery_count] or hashlib.sha256(original).hexdigest() != report["selected_journal_sha256"]:
            raise RuntimeError("observed cut differs from exact published prefix")
        control, result, guard = (Path(str(journal) + suffix) for suffix in (".original-cut-journal", ".original-cut-result", ".original-cut-pending"))
        if guard.exists() or guard.is_symlink():
            raise RuntimeError("original physical proof remained uncertain/guarded")
        control_raw, control_identity = native_image(control)
        proof_raw, proof_identity = native_image(result)
        cut_phases = [value["phase"] for value in recovery_matrix["canonical_prefixes"] if value["operation"] == "original-cut"]
        if [value["phase"] for value in canonical_rows(control_raw)] != cut_phases:
            raise RuntimeError("original physical control proof incomplete")
        proof = json.loads(proof_raw)
        boundary = proof["boundary"]
        if proof["schema"] != "ardents-source-original-cut-v1" or proof["acknowledged"] != recovery_count - 2 or (proof["code"], proof["status"], proof["reaped"]) != (2, signal.SIGKILL, 1) or boundary["journal"] != str(journal) or boundary["operation"] != operation or boundary["generation"] != generation or boundary["records"] != recovery_count or (boundary["device"], boundary["inode"]) != identity[:2] or boundary["prefix_sha256"] != report["selected_journal_sha256"] or proof["origin"]["program_sha256"] != profile["program_sha256"] or not boundary["manager_identity"] or hashlib.sha256(proof_raw).hexdigest() != report["original_cut_sha256"]:
            raise RuntimeError("original native cut lineage differs")
        pending_raw, pending_identity = native_image(pending)
        immutable = {str(path): (info, hashlib.sha256(raw).hexdigest()) for path, raw, info in ((journal, original, identity), (control, control_raw, control_identity), (result, proof_raw, proof_identity))}
        before = recovery_snapshot(declaration)
        # These are attempted ordinary operations under the interrupted state,
        # not a claim that the pending marker is their only possible refusal.
        for argv, name in ((command, "interrupted-replay"),
                           ([command[0], "installation", "start-source", command[3], history, "alpha", network, binding], "interrupted-start")):
            code, _ = run(argv, name, refusal=True)
            if code == 0 or native_image(pending) != (pending_raw, pending_identity):
                raise RuntimeError("interrupted ordinary operation accepted or changed blocker")
        if recovery_snapshot(declaration) != before:
            raise RuntimeError("refused ordinary operations changed original semantic/account history")
        for path, expected in immutable.items():
            raw, info = native_image(Path(path))
            if (info, hashlib.sha256(raw).hexdigest()) != expected:
                raise RuntimeError("refused operation rewrote original interruption proof")
        if recovery_fault is not None:
            # One terminal fault per private stand. Keep damaged history and
            # originals; never restore it and count the same stand as positive.
            import fcntl
            lease = None
            retained_root = None
            watched = [journal, control, result, pending, Path(str(journal) + ".recovery-completion")]

            def synchronize_parent(path):
                descriptor = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
                errors = []
                try:
                    os.fsync(descriptor)
                except BaseException as error:
                    errors.append(error)
                finally:
                    try:
                        os.close(descriptor)
                    except OSError as error:
                        errors.append(error)
                if errors:
                    raise BaseExceptionGroup("fault parent synchronization/Close failed", errors)

            def retain_original(path):
                nonlocal retained_root
                if retained_root is None:
                    retained_root = recovery_fault_retention(installation, generation)
                    retained_root.mkdir(mode=0o700)  # Fresh, never adopted.
                    info = retained_root.lstat()
                    if not stat.S_ISDIR(info.st_mode) or native_identity(info)[2:] != (0o700, 0, 0):
                        raise RuntimeError("private native fault retention required")
                    watched.append(retained_root)
                    synchronize_parent(retained_root)
                if path.lstat().st_dev != retained_root.lstat().st_dev:
                    raise RuntimeError("same-filesystem original retention required")
                retained = retained_root / path.name
                if retained.exists() or retained.is_symlink():
                    raise RuntimeError("fresh retained original required")
                path.rename(retained)
                watched.append(retained)
                synchronize_parent(path)
                synchronize_parent(retained)
                return retained

            def retain_and_copy(path):
                body, native = native_image(path, 32 << 20)
                retained = retain_original(path)
                # Exclusive creation prevents adopting any existing object.
                with path.open("xb") as stream:
                    stream.write(body)
                    stream.flush()
                    os.fchown(stream.fileno(), native[3], native[4])
                    os.fchmod(stream.fileno(), native[2])
                    os.fsync(stream.fileno())
                synchronize_parent(path)
                if retained.parent != path.parent:
                    synchronize_parent(retained)
                copied, replacement_native = native_image(path, 32 << 20)
                if copied != body or replacement_native[:2] == native[:2] or native_image(retained, 32 << 20) != (body, native):
                    raise RuntimeError("actual equal-byte distinct-inode substitution missing")

            def preserved_refusal_state():
                state = recovery_snapshot(declaration, installation=True)
                for path in watched:
                    if not path.exists() and not path.is_symlink():
                        state[str(path)] = ("absent",)
                    elif path.is_dir() and not path.is_symlink():
                        state[str(path)] = ("directory", native_identity(path.lstat()), tuple(sorted(child.name for child in path.iterdir())))
                    else:
                        body, native = native_image(path, 32 << 20)
                        state[str(path)] = ("file", native, hashlib.sha256(body).hexdigest())
                return state

            fault_failures = []
            try:
                if recovery_fault == "held-journal-lease":
                    lease = os.open(journal, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
                    fcntl.flock(lease, fcntl.LOCK_EX | fcntl.LOCK_NB)
                elif recovery_fault in ("copied-journal", "copied-cut-result"):
                    retain_and_copy(journal if recovery_fault == "copied-journal" else result)
                elif recovery_fault == "journal-access":
                    os.chmod(journal, 0o640, follow_symlinks=False)
                elif recovery_fault == "torn-journal":
                    with journal.open("ab") as stream:
                        stream.write(b"{")
                        stream.flush()
                        os.fsync(stream.fileno())
                elif recovery_fault == "missing-pending":
                    retained = retain_original(pending)
                    if native_image(retained) != (pending_raw, pending_identity):
                        raise RuntimeError("original pending identity/bytes changed during retention")
                elif recovery_fault == "partial-completion":
                    with watched[4].open("xb") as stream:
                        stream.write(b"{")
                        stream.flush()
                        os.fsync(stream.fileno())
                    synchronize_parent(watched[4])
                elif recovery_fault == "copied-generation-file":
                    candidates = [Path(value["path"]) for value in rows if value["phase"] in ("file-born", "file-ready") and Path(value["path"]).is_relative_to(installation / "generations")]
                    if not candidates:
                        raise RuntimeError("actual original recorded generation file required")
                    retain_and_copy(candidates[-1])
                elif recovery_fault == "manager-reexec":
                    # Only the private stand's PID1 is involved. No Source is
                    # live and all retained read-only references are closed.
                    if native_observer is not None or source_started or source_pidfd is not None or scope_fd is not None:
                        raise RuntimeError("private manager control requires joined original Source and observer")
                    previous = manager_identity()
                    if previous != boundary["manager_identity"]:
                        raise RuntimeError("original actual manager differs before the selected fault")
                    manager_proc = Path("/proc/1").stat()
                    manager_exe = Path("/proc/1/exe").stat()
                    manager_namespaces = {name: Path("/proc/1/ns", name).stat() for name in ("mnt", "pid", "cgroup", "uts", "net")}
                    original_disk = preserved_refusal_state()
                    run(["/usr/bin/systemctl", "daemon-reexec"], "terminal-private-manager-reexec")
                    limit = min(started + bound, time.monotonic() + 10)
                    while True:
                        observe()
                        current = manager_identity()
                        if current != previous:
                            break
                        if time.monotonic() >= limit:
                            raise RuntimeError("actual private manager reexec did not change broker/unique-owner identity")
                        time.sleep(0.02)
                    if manager_identity() != current:
                        raise RuntimeError("new private manager identity is not coherent")
                    for path, old in [(Path("/proc/1"), manager_proc), (Path("/proc/1/exe"), manager_exe)] + [(Path("/proc/1/ns", name), old) for name, old in manager_namespaces.items()]:
                        actual = path.stat()
                        if (old.st_dev, old.st_ino) != (actual.st_dev, actual.st_ino):
                            raise RuntimeError("private manager process/program/namespace changed outside reexec control")
                    if subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "MainPID", "--value"]).strip() != b"0":
                        raise RuntimeError("manager control created a live Source")
                    if preserved_refusal_state() != original_disk:
                        raise RuntimeError("private manager fault changed original disk history")
                    outcome["manager_reexec"] = {"original_identity": previous, "new_identity": current,
                                                 "original_process_and_namespaces_retained": True,
                                                 "original_source_not_live": True}
                elif recovery_fault in ("missing-startup-journal", "missing-startup-archive",
                                        "missing-retirement-journal", "missing-retirement-result"):
                    # All these terminal cases cut B after A's genuine first
                    # contact and committed retirement. Select the actual
                    # recorded predecessor, not a fabricated proof or attempt.
                    retirement = Path(rows[4]["path"])
                    retired_raw, _ = native_image(retirement)
                    retired = json.loads(retired_raw)
                    if operation != "replacement" or recovery_count != 49 or rows[4]["phase"] != "predecessor-retired" or hashlib.sha256(retired_raw).hexdigest() != rows[4]["sha256"] or retired["schema"] != "ardents-source-retirement-v1" or retired["attempt"] != 0 or retirement != installation / "source-startup" / "source-retirement.json":
                        raise RuntimeError("actual complete first predecessor retirement required for history fault")
                    missing = {"missing-startup-journal": Path(str(installation) + ".source-first-start-journal"),
                               "missing-startup-archive": retirement.parent / "start-archived.json",
                               "missing-retirement-journal": retirement.parent / "source-retirement-journal",
                               "missing-retirement-result": retirement}[recovery_fault]
                    old_image = native_image(missing)
                    watched.append(missing)  # Startup journal is outside the tree.
                    retained = retain_original(missing)
                    if missing.exists() or missing.is_symlink() or native_image(retained) != old_image:
                        raise RuntimeError("single missing original history fact was not retained exactly")
                else:
                    raise RuntimeError("unrecognized terminal recovery fault")
                damaged = preserved_refusal_state()
                header = {"schema": "ardents-source-recovery-completion-v1", "operation": operation, "journal": str(journal),
                          "device": identity[0], "inode": identity[1], "prefix_sha256": report["selected_journal_sha256"],
                          "prefix_records": recovery_count, "generation": generation, "binding_sha256": boundary["binding_sha256"]}
                allowed_failed_header = json.dumps(header, separators=(",", ":"), ensure_ascii=False).encode()[:-1] + b',"records":['
                for attempt in range(2):
                    code, refused = run([command[0], "installation", "recover-source", command[3], history, "alpha", network, declaration_path, journal], f"terminal-recovery-refusal-{attempt}", refusal=True)
                    actual = preserved_refusal_state()
                    # Allow only a new exclusive empty failure header on the
                    # first refusal; a second caller cannot append/adopt it.
                    completion_key = str(watched[4])
                    if attempt == 0 and damaged[completion_key] == ("absent",) and actual[completion_key] != ("absent",):
                        if not predictions[0]["failure_header_permitted"]:
                            raise RuntimeError("early refusal created unauthorized completion evidence")
                        failed_header, failed_native = native_image(watched[4])
                        if failed_header != allowed_failed_header or failed_native[2:] != (0o600, 0, 0):
                            raise RuntimeError("refusal produced unauthorized completion evidence")
                        damaged[completion_key] = actual[completion_key]
                    if code != 2 or b'"outcome":"source-recovery-stopped-selected"' in refused or actual != damaged:
                        raise RuntimeError("terminal recovery fault accepted or changed preserved history")
                outcome["recovery_refusal"] = {"operation": operation, "prefix_records": recovery_count, "fault": recovery_fault,
                                                "original_failed_exit": report["root_exit"], "original_threads_joined": report["threads_joined"],
                                                "separate_refused_invocations": 2, "original_failure_retained": True,
                                                "matrix_sha256": digest(paths["RECOVERY_MATRIX"])}
                if operation == "initial":
                    if source_started or source_pidfd is not None or scope_fd is not None or outcome["original_scope_joined"]:
                        raise RuntimeError("never-started initial refusal acquired a Source runtime scope")
                    # The canonical initial operation and both recovery calls
                    # perform no Start. This reports that no Source scope was
                    # born, rather than claiming a join from root child counts.
                    outcome["source_scope_not_started"] = True
            except BaseException as error:
                fault_failures.append(error)
            finally:
                if lease is not None:
                    try:
                        os.close(lease)
                    except OSError as error:
                        fault_failures.append(error)
            if fault_failures:
                raise BaseExceptionGroup("terminal refusal/held lease Close failed", fault_failures)
            raise RecoveryRefusalComplete()
        raw = run([command[0], "installation", "recover-source", command[3], history, "alpha", network, declaration_path, journal], "explicit-fresh-recovery")
        if json.loads(raw) != {"operation": "installation.source", "outcome": "source-recovery-stopped-selected"} or pending.exists() or pending.is_symlink() or recovery_snapshot(declaration) != before:
            raise RuntimeError("fresh stopped recovery or original preservation missing")
        completion = Path(str(journal) + ".recovery-completion")
        completion_raw, completion_identity = native_image(completion)
        suffix = json.loads(completion_raw)
        fields = ["schema", "operation", "journal", "device", "inode", "prefix_sha256", "prefix_records", "generation", "binding_sha256", "records", "transcript_sha256"]
        if list(suffix) != fields or suffix["schema"] != "ardents-source-recovery-completion-v1" or suffix["operation"] != operation or suffix["journal"] != str(journal) or (suffix["device"], suffix["inode"]) != identity[:2] or suffix["prefix_sha256"] != report["selected_journal_sha256"] or suffix["prefix_records"] != recovery_count or suffix["generation"] != generation or suffix["binding_sha256"] != boundary["binding_sha256"] or len(suffix["records"]) + len(rows) != len(phases):
            raise RuntimeError("separate original-bound completion envelope differs")
        transcript = original + b"".join(json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode() + b"\n" for value in suffix["records"])
        if [value["phase"] for value in canonical_rows(transcript)] != phases or hashlib.sha256(transcript).hexdigest() != suffix["transcript_sha256"]:
            raise RuntimeError("independent complete recovered transcript differs")
        immutable[str(completion)] = (completion_identity, hashlib.sha256(completion_raw).hexdigest())
        recovery_evidence.update(journal=journal, immutable=immutable, transcript=transcript)
        recovered_transcript()  # Recheck original bytes after all suffix reads.
        outcome["recovery"] = {"operation": operation, "prefix_records": recovery_count, "last_phase": row[0]["phase"],
                               "original_failed_exit": report["root_exit"], "original_threads_joined": report["threads_joined"],
                               "original_prefix_sha256": report["selected_journal_sha256"], "completion_sha256": digest(completion),
                               "matrix_sha256": digest(paths["RECOVERY_MATRIX"]), "original_failure_retained": True}
        return raw

    class NativeObserver:
        # This independent read-only reference preserves the native exit oracle
        # across systemd collection. Its reports never enter product admission.
        def __init__(self, name="original-native-observer"):
            self.log = (output / (name + ".log")).open("xb")
            self.events = queue.Queue(maxsize=34)
            self.errors = []
            self.closed = False
            env = dict(os.environ, ARDENTS_SOURCE_STARTUP_READ_ONLY_OBSERVER="1")
            try:
                self.child = subprocess.Popen([str(paths["MANAGER_TEST"]), "-test.run=^TestSourceRetirementObserverNative$", "-test.v", "-test.timeout=" + ("10m" if replacement_profile else "8m")],
                                              stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=env)
            except BaseException:
                self.log.close()
                raise

            def drain():
                total = 0
                try:
                    while True:
                        raw = self.child.stdout.readline(65538)
                        if not raw:
                            break
                        total += len(raw)
                        if len(raw) > 65537 or total > 4 << 20:
                            raise RuntimeError("original native observer output exceeds finite bound")
                        self.log.write(raw)
                        if raw.startswith(b"SOURCE_OBSERVATION "):
                            self.events.put_nowait(json.loads(raw[len(b"SOURCE_OBSERVATION "):]))
                        elif raw == b"SOURCE_OBSERVER_CLOSED\n":
                            self.events.put_nowait("closed")
                except BaseException as error:
                    self.errors.append(error)
                    # Continue draining the original pipe even after refusal.
                    try:
                        while self.child.stdout.read(65536):
                            pass
                    except OSError as drain_error:
                        self.errors.append(drain_error)
                finally:
                    try:
                        self.child.stdout.close()
                    except OSError as error:
                        self.errors.append(error)

            self.reader = threading.Thread(target=drain)
            try:
                self.reader.start()
            except BaseException:
                self.child.stdin.close()
                self.child.stdout.close()
                self.child.wait()
                self.log.close()
                raise
            # The helper emits an initial observation before accepting commands.
            # Consume it now so every later sample is the reply to that sample's
            # actual observe request, rather than the preceding queued snapshot.
            try:
                if not isinstance(self.receive(), dict):
                    raise RuntimeError("original initial native observation missing")
            except BaseException as error:
                try:
                    self.close()
                except BaseException as cleanup:
                    raise BaseExceptionGroup("original observer initialization and cleanup failed", [error, cleanup])
                raise

        def receive(self):
            while True:
                observe()
                if self.errors:
                    raise BaseExceptionGroup("original observer output failed", self.errors)
                try:
                    return self.events.get(timeout=0.05)
                except queue.Empty:
                    if self.child.poll() is not None and not self.reader.is_alive():
                        raise RuntimeError("original native observer ended without requested observation")

        def sample(self):
            self.child.stdin.write(b"observe\n")
            self.child.stdin.flush()
            result = self.receive()
            if not isinstance(result, dict):
                raise RuntimeError("original native observation missing")
            return result

        def close(self):
            if self.closed:
                return
            self.closed = True
            errors = []
            try:
                self.child.stdin.write(b"finish\n")
                self.child.stdin.flush()
            except OSError as error:
                errors.append(error)
            try:
                self.child.stdin.close()
            except OSError as error:
                errors.append(error)
            # Original child/output join, even after profile cancellation.
            code = self.child.wait()
            self.reader.join()
            try:
                self.log.close()
            except OSError as error:
                errors.append(error)
            errors.extend(self.errors)
            events = []
            while not self.events.empty():
                events.append(self.events.get_nowait())
            if code != 0 or events != ["closed"]:
                errors.append(RuntimeError("original observer did not close cleanly"))
            if errors:
                raise BaseExceptionGroup("original native observer cleanup failed", errors)

    def manager_owner():
        return subprocess.check_output(["/usr/bin/busctl", "--timeout=3s", "--system", "call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetNameOwner", "s", "org.freedesktop.systemd1"])

    def manager_identity():
        owner = manager_owner()
        broker = subprocess.check_output(["/usr/bin/busctl", "--timeout=3s", "--system", "call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetId"])
        if not owner.startswith(b's ":1.') or not broker.startswith(b's "') or manager_owner() != owner:
            raise RuntimeError("fresh coherent private manager identity unavailable")
        owner, broker = json.loads(owner[2:]), json.loads(broker[2:])
        if len(broker) != 32 or any(c not in "0123456789abcdefABCDEF" for c in broker):
            raise RuntimeError("actual broker instance identity is invalid")
        return broker.lower() + "/" + owner

    original_manager = manager_owner()
    if not original_manager.startswith(b's ":1.') or not subprocess.check_output(["/usr/bin/systemctl", "show", "-p", "Version", "--value"]).startswith(b"255."):
        raise RuntimeError("selected original manager unavailable")

    def original_join():
        # Only this retained kernel object supplies the Source join observation.
        try:
            os.lseek(scope_fd, 0, os.SEEK_SET)
            body = os.read(scope_fd, 1025)
            if len(body) > 1024:
                raise RuntimeError("original scope observation exceeds bound")
            fields = dict(line.split() for line in body.decode("ascii").splitlines())
            if fields.get("populated") not in ("0", "1") or fields.get("frozen") not in ("0", "1"):
                raise RuntimeError("original kernel scope observation is invalid")
            return fields["populated"] == "0"
        except OSError as error:
            if error.errno != errno.ENODEV:
                raise
            return True

    try:
        if recovery_profile:
            observer_name = "TestSourceCutObserverNative"
            discovered = run([paths["RECOVERY_OBSERVER"], "-test.list=^" + observer_name + "$"], "recovery-observer-discovery")
            if discovered.splitlines() != [observer_name.encode("ascii")]:
                raise RuntimeError("same-source native recovery observer is absent")
            native_pattern = "^(" + "|".join(RECOVERY_NATIVE_TESTS) + ")$"
            discovered = run([paths["RECOVERY_OBSERVER"], "-test.list=" + native_pattern], "recovery-native-discovery")
            if sorted(discovered.splitlines()) != sorted(name.encode("ascii") for name in RECOVERY_NATIVE_TESTS):
                raise RuntimeError("required native recovery ownership/process control is absent")
            # The same pinned native binary supplies the file/process controls;
            # it obtains no installed or semantic authority from their result.
            native_root = output / "native-component"
            native_root.mkdir(mode=0o700)
            native_environment = dict(os.environ, ARDENTS_SOURCE_STOPPED_FUTURE_PARENT=str(native_root))
            run([paths["RECOVERY_OBSERVER"], "-test.run=" + native_pattern, "-test.v", "-test.timeout=90s"],
                "recovery-native-controls", env=native_environment)
        manager_tests = ("TestSourceControlRetainsStoppedUnitNative", "TestSourceCanceledObservationCloseNative")
        if restart_profile:
            manager_tests += ("TestSourceRetirementObserverNative",)
        discovered = run([paths["MANAGER_TEST"], "-test.list=^(" + "|".join(manager_tests) + ")$"], "manager-test-discovery")
        if sorted(discovered.splitlines()) != sorted(name.encode("ascii") for name in manager_tests):
            raise RuntimeError("required native manager control is absent")
        if replacement_profile:
            owner_tests = ("TestSourceHistoryAcquisitionNative", "TestSourceStoppedNativeHistoryHeldObservationAndClose", "TestSourceStoppedNativeSelectionHeldLastReadAndCloseFailure", "TestSourceInitialNativeHeldReadPreservesCancellationAndClose")
            if case == "replacement-reload-cancel":
                owner_tests += ("TestSourceReplacementNativeClosePreservesFailureAndJoins",)
            discovered = run([paths["OWNER_TEST"], "-test.list=^(" + "|".join(owner_tests) + ")$"], "history-owner-test-discovery")
            if sorted(discovered.splitlines()) != sorted(name.encode("ascii") for name in owner_tests):
                raise RuntimeError("original history FD/flock regression is absent")
        if case == "replacement-reload-cancel":
            name = "TestSourceReplacementReloadCancellationNative"
            discovered = run([paths["CUT_TEST"], "-test.list=^" + name + "$"], "replacement-kernel-control-discovery")
            if discovered.splitlines() != [name.encode("ascii")]:
                raise RuntimeError("actual replacement kernel control is absent")
        clock = Path(request["clock"])
        clock_fd = os.open(clock, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
        clock_info = os.fstat(clock_fd)
        if not stat.S_ISREG(clock_info.st_mode) or clock_info.st_uid != 0 or stat.S_IMODE(clock_info.st_mode) != 0o644:
            raise RuntimeError("original root clock writer/access required")
        os.utime(clock_fd, None)

        def clock_writer():
            while not clock_stop.is_set():
                try:
                    current = clock.stat()
                    if (current.st_dev, current.st_ino) != (clock_info.st_dev, clock_info.st_ino):
                        raise RuntimeError("original clock writer inode changed")
                    os.utime(clock_fd, None)
                except BaseException as error:
                    clock_errors.append(error)
                    return
                clock_stop.wait(0.2)

        clock_thread = threading.Thread(target=clock_writer)
        clock_thread.start()
        # Offline State acceptance already needs a fresh independent clock;
        # keep this same original writer live during installation and preparation.
        initial_command = [paths["NODE"], "installation", "install-initial-stopped", distribution, os.environ["ARDENTS_SOURCE_STARTUP_PIN"], history, "alpha", network, paths["REQUEST"]]
        if recovery_operation == "initial":
            cut_and_recover(initial_command, "initial", paths["REQUEST"])
        else:
            run(initial_command, "initial-stopped")
        selected_document = selected = json.loads(binding.read_text())
        uid, gid = selected["uid"], selected["gid"]
        if not 100 <= uid <= 999 or uid != gid:
            raise RuntimeError("actual fixed Source principal required")
        plan = Path(request["installation_root"]) / "generations" / selected["generation"] / "source-plan.json"
        # BUNDLE and CONTROL must have a separate root-controlled public
        # delivery traversable by this actual UID; private key directories stay
        # private. There is no fixture-root or root-user State substitution.
        run([paths["CONTROL"], "source", "prepare-state", plan, paths["BUNDLE"]], "state-preparation", user=uid, group=gid, extra_groups=[])

        # An inactive unit may have left the manager's in-memory inventory after
        # initial installation. Root control must acquire its own original
        # reference, not depend on an observer keeping that unit loaded.
        run([paths["MANAGER_TEST"], "-test.run=^TestSourceControlRetainsStoppedUnitNative$", "-test.v", "-test.timeout=30s"], "stopped-unit-reference")
        run([paths["MANAGER_TEST"], "-test.run=^TestSourceCanceledObservationCloseNative$", "-test.v", "-test.timeout=30s"], "canceled-manager-observation")
        if restart_profile:
            native_observer = NativeObserver()
            initial_facts = native_observer.sample()
            if initial_facts["Unit"]["ActiveState"] != {"type": "s", "data": "inactive"} or initial_facts["Service"]["MainPID"] != {"type": "u", "data": 0}:
                raise RuntimeError("independent native observer did not retain original stopped unit")

        # From here even a failed/uncertain start belongs to the outside own
        # stand completion obligation. The product joins its failed attempt.
        source_started = True
        raw = run([paths["NODE"], "installation", "start-source", distribution, history, "alpha", network, binding], "first-start")
        if json.loads(raw) != {"operation": "installation.source", "outcome": "first-start-completed"}:
            raise RuntimeError("foreign first-start completion projection")
        def retain_invocation():
            nonlocal source_pid, source_pidfd, scope_fd, invocation
            source_pid = int(subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "MainPID", "--value"]))
            if source_pid <= 1:
                raise RuntimeError("actual original non-root Source process missing")
            process_path = Path("/proc") / str(source_pid)
            process_info = process_path.stat()
            source_pidfd = os.pidfd_open(source_pid, 0)
            signal.pidfd_send_signal(source_pidfd, 0)
            current_process = process_path.stat()
            if (current_process.st_dev, current_process.st_ino) != (process_info.st_dev, process_info.st_ino):
                raise RuntimeError("original Source process changed during pidfd acquisition")
            expected = [str(plan.parent / "ardents-node-linux-amd64"), "source", "start-installed", str(binding)]
            if (Path("/proc") / str(source_pid) / "cmdline").read_bytes().split(b"\0") != [s.encode() for s in expected] + [b""]:
                raise RuntimeError("actual original Source argv differs")
            status = dict(line.split(":", 1) for line in (Path("/proc") / str(source_pid) / "status").read_text().splitlines() if ":" in line)
            if [int(value) for value in status["Uid"].split()] != [uid] * 4 or [int(value) for value in status["Gid"].split()] != [gid] * 4:
                raise RuntimeError("actual Source real/effective/saved/filesystem credentials differ")
            if any(int(value) != gid for value in status["Groups"].split()) or any(int(status[key].strip(), 16) for key in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb")):
                raise RuntimeError("actual Source supplementary groups/capabilities differ")
            actual_program, selected_program = (Path("/proc") / str(source_pid) / "exe").stat(), (plan.parent / "ardents-node-linux-amd64").stat()
            if (actual_program.st_dev, actual_program.st_ino) != (selected_program.st_dev, selected_program.st_ino):
                raise RuntimeError("actual original Source executable differs")
            scope_fd = os.open("/sys/fs/cgroup/system.slice/ardents-source.service/cgroup.events", os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
            if os.fstat(scope_fd).st_dev != Path("/sys/fs/cgroup").stat().st_dev or original_join():
                raise RuntimeError("original active kernel Source scope unavailable")
            invocation = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "InvocationID", "--value"]).decode().strip()
            if len(invocation) != 32 or any(c not in "0123456789abcdef" for c in invocation):
                raise RuntimeError("actual original invocation identity unavailable")
            ready = False
            while not ready:
                observe()
                if manager_owner() != original_manager:
                    raise RuntimeError("original manager changed before runtime readiness")
                journal = subprocess.check_output(["/usr/bin/journalctl", "--no-pager", "--output=cat", "--lines=32", "--unit=ardents-source.service", "_SYSTEMD_INVOCATION_ID=" + invocation])
                if len(journal) > 64 << 10:
                    raise RuntimeError("original readiness observation exceeds bound")
                for line in journal.splitlines():
                    if line == b'{"operation":"source.start-installed","outcome":"listening"}':
                        ready = True
                if not ready:
                    if original_join():
                        raise RuntimeError("original Source ended before genuine runtime readiness")
                    time.sleep(0.05)

        retain_invocation()
        inspect = [paths["CONTROL"], "source", "inspect-latest", plan, paths["CLIENT_CERTIFICATE"], paths["CLIENT_KEY"], paths["SERVER_CA"],
                   os.environ["ARDENTS_SOURCE_STARTUP_SERVER_NAME"], os.environ["ARDENTS_SOURCE_STARTUP_SERVER_PIN"], os.environ["ARDENTS_SOURCE_STARTUP_SOURCE_ID"],
                   os.environ["ARDENTS_SOURCE_STARTUP_FAMILY"], os.environ["ARDENTS_SOURCE_STARTUP_HANDLE"], "0"]
        def actual_contact(name):
            response = json.loads(run(inspect, name))
            expected_payload = base64.b64encode(paths["BUNDLE"].read_bytes()).decode("ascii")
            # AREP's protocol identity hashes the prepared unsigned envelope. The
            # signed file commitment includes its signer section and is different.
            # Exact bundle equality separately covers the complete signed payload.
            if response.get("operation") != "source.inspect-latest" or response.get("status") != "ok" or response.get("payload") != expected_payload or bytes(response["object_digest"]).hex() != digest(paths["UNSIGNED_EPOCH"]):
                raise RuntimeError("genuine Source response differs from accepted prepared State")
            return response

        response = actual_contact("actual-client")
        outcome["response_epoch_sha256"] = digest(paths["EPOCH"])
        if restart_profile:
            installation = Path(request["installation_root"])
            immutable = ([binding] if not replacement_profile else []) + [*plan.parent.iterdir(), *Path(request["credential_directory"]).iterdir()]

            def identity(path):
                info = path.stat()
                return (info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode), info.st_uid, info.st_gid)

            originals = {path: (identity(path), digest(path)) for path in immutable}
            roots = {Path(request[name]): identity(Path(request[name])) for name in ("state_root", "participation_root")}
            roots[Path(history)] = identity(Path(history))
            selection_inodes = {path: identity(path) for path in (binding, Path("/etc/systemd/system/ardents-source.service"))}
            invocations, challenges = [], []
            lineage_initial = selected["generation"]
            start_phases = ["first-start", "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready",
                            "socket-intent", "socket-born", "socket-ready", "file-intent", "file-born", "file-ready", "start-attempt",
                            "invocation-observed", "completion-received", "file-intent", "file-born", "file-ready", "archived", "ack-attempt", "ack-written"]
            stop_phases = ["stop", "socket-intent", "socket-born", "socket-ready", "interrupt-attempt", "domain-joined", "native-joined", "native-closed", "file-intent", "file-born", "file-ready", "retired"]

            def retained():
                if json.loads(binding.read_text()) != selected_document:
                    raise RuntimeError("immutable selected generation/account changed")
                for path, expected in originals.items():
                    if (identity(path), digest(path)) != expected:
                        raise RuntimeError("immutable original generation or credential changed")
                for path, expected in roots.items():
                    if identity(path) != expected or not any(path.iterdir()):
                        raise RuntimeError("original semantic history root replaced or emptied")
                if replacement_profile and any(identity(path) != expected for path, expected in selection_inodes.items()):
                    raise RuntimeError("replacement changed an original selection/unit inode or access")

            def records(path, phases):
                raw = recovered_transcript() if recovery_evidence.get("journal") == path else path.read_bytes()
                if not raw or len(raw) > 128 << 10 or not raw.endswith(b"\n"):
                    raise RuntimeError("complete bounded original transcript required")
                rows = [json.loads(line) for line in raw.splitlines()]
                if [row["sequence"] for row in rows] != list(range(len(phases))) or [row["phase"] for row in rows] != phases:
                    raise RuntimeError("independent transcript order differs")
                return rows

            def refuse_restart(name):
                code, _ = run([active_node, "installation", "restart-source", distribution, history, "alpha", network, binding], name, refusal=True)
                if code == 0:
                    raise RuntimeError("restart admitted without original complete retirement")
                if int(subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "MainPID", "--value"])) != source_pid or original_join():
                    raise RuntimeError("refused attempt changed the live original Source")

            def explicit_stop(number):
                nonlocal source_started, scope_fd, source_pidfd
                current_directory = installation / ("source-startup" if number == 0 else f"source-invocation-{number:08d}")
                journal = Path(str(installation) + (".source-first-start-journal" if number == 0 else f".source-invocation-{number:08d}-journal"))
                phases = start_phases.copy()
                if number:
                    phases[0] = "invocation"
                records(journal, phases)
                if (current_directory / "source-start-pending").exists():
                    raise RuntimeError("full-looking startup transcript remained uncommitted")
                stop_command = [active_node, "installation", "stop-source", history, binding]
                if number == 0 and case in ("restart", "replacement"):
                    # Hold the actual original process, rather than inventing a
                    # completion callback. Root can interrupt it but cannot get
                    # its domain frame or native join until this hold releases.
                    signal.pidfd_send_signal(source_pidfd, signal.SIGSTOP)
                    stop_result, stop_errors = [], []

                    def stop_original():
                        try:
                            stop_result.append(run(stop_command, "explicit-stop-held-original"))
                        except BaseException as error:
                            stop_errors.append(error)

                    stopper = threading.Thread(target=stop_original)
                    began = False
                    try:
                        stopper.start()
                        began = True
                        while True:
                            observe()
                            stop_journal = current_directory / "source-retirement-journal"
                            if stop_journal.exists():
                                body = stop_journal.read_bytes()
                                if body.endswith(b"\n") and b'"phase":"interrupt-attempt"' in body:
                                    break
                            if not stopper.is_alive():
                                raise RuntimeError("original explicit stop ended before held join observation")
                            time.sleep(0.01)
                        process_status = (Path("/proc") / str(source_pid) / "status").read_text()
                        state_lines = [line for line in process_status.splitlines() if line.startswith("State:")]
                        if len(state_lines) != 1 or state_lines[0].split()[1] != "T" or original_join() or not stopper.is_alive():
                            raise RuntimeError("actual original process hold was not established")
                        if (current_directory / "source-retirement.json").exists() or b'"phase":"domain-joined"' in body:
                            raise RuntimeError("held original process supplied domain retirement")
                        refuse_restart("held-original-concurrent-restart")
                        if replacement_profile:
                            refuse_replacement("B", "held-original-concurrent-replacement")
                        if (installation / "source-invocation-00000001").exists() or not stopper.is_alive():
                            raise RuntimeError("held original retirement released successor admission")
                        outcome["held_original_join_denied_restart"] = True
                        if replacement_profile:
                            outcome["held_original_join_denied_replacement"] = True
                    finally:
                        try:
                            signal.pidfd_send_signal(source_pidfd, signal.SIGCONT)
                        finally:
                            if began:
                                stopper.join()  # Retain the original command through join.
                    if stop_errors:
                        raise BaseExceptionGroup("held original stop failed", stop_errors)
                    if len(stop_result) != 1:
                        raise RuntimeError("original held stop result unavailable")
                    raw = stop_result[0]
                else:
                    raw = run(stop_command, f"explicit-stop-{number}")
                if json.loads(raw) != {"operation": "installation.source", "outcome": "source-stopped-completed"}:
                    raise RuntimeError("foreign explicit stop outcome")
                # A command return or file is not a physical join. This is the
                # outside observer's independently retained original object.
                while not original_join():
                    observe()
                    time.sleep(0.02)
                outcome["original_scope_joined"] = True
                archive_path = current_directory / "start-archived.json"
                archive = json.loads(archive_path.read_text())
                proof_path = current_directory / "source-retirement.json"
                proof = json.loads(proof_path.read_text())
                if proof["attempt"] != number or proof["invocation_id"] != invocation or proof["pid"] != source_pid or proof["generation"] != selected["generation"] or proof["binding_sha256"] != digest(binding) or proof["start_sha256"] != digest(archive_path):
                    raise RuntimeError("retirement does not bind exact original invocation")
                challenge = bytes.fromhex(proof["challenge"])
                frame = b"ardents-source-domain-joined-v1\n" + (invocation + selected["generation"] + digest(binding)).encode("ascii") + challenge
                if len(challenge) != 32 or not any(challenge) or challenge in challenges or hashlib.sha256(frame).hexdigest() != proof["completion_sha256"]:
                    raise RuntimeError("original fresh retirement exchange differs")
                challenges.append(challenge)
                rows = records(current_directory / "source-retirement-journal", stop_phases)
                if rows[-1]["sha256"] != digest(proof_path) or (current_directory / "source-retirement-pending").exists():
                    raise RuntimeError("retirement proof remained partial or blocked")
                for _ in range(2):
                    facts = native_observer.sample()
                    service, unit = facts["Service"], facts["Unit"]
                    if facts["Manager"] != proof["manager"] or unit["ActiveState"] != {"type": "s", "data": "inactive"} or unit["SubState"] != {"type": "s", "data": "dead"}:
                        raise RuntimeError("independent original native manager completion differs")
                    expected = {"MainPID": ("u", 0), "ExecMainPID": ("u", source_pid), "ExecMainCode": ("i", 1), "ExecMainStatus": ("i", 0),
                                "ExecMainStartTimestamp": ("t", proof["started_realtime"]), "ExecMainStartTimestampMonotonic": ("t", proof["started_monotonic"]),
                                "ExecMainExitTimestamp": ("t", proof["exited_realtime"]), "ExecMainExitTimestampMonotonic": ("t", proof["exited_monotonic"])}
                    if any(service[key] != {"type": signature, "data": value} for key, (signature, value) in expected.items()):
                        raise RuntimeError("independent native exit clocks/status differ from original retirement")
                if proof["started_realtime"] != archive["started_realtime"] or proof["started_monotonic"] != archive["started_monotonic"]:
                    raise RuntimeError("retirement followed another invocation's clocks")
                retained()
                if replacement_profile:
                    host, port = request["address"].rsplit(":", 1)
                    host = host.removeprefix("[").removesuffix("]")
                    family = socket.AF_INET6 if ":" in host else socket.AF_INET
                    with socket.socket(family, socket.SOCK_STREAM) as probe:
                        probe.settimeout(0.5)
                        try:
                            probe.connect((host, int(port)))
                        except OSError as error:
                            if error.errno != errno.ECONNREFUSED:
                                raise RuntimeError("old listener completion is uncertain") from error
                        else:
                            raise RuntimeError("old Source listener still accepts after original domain/native join")
                    outcome["old_listeners_closed"] = len(invocations) + 1
                os.close(scope_fd)
                scope_fd = None
                os.close(source_pidfd)
                source_pidfd = None
                source_started = False
                invocations.append(invocation)
                if replacement_profile:
                    run([paths["OWNER_TEST"], "-test.run=^TestSourceHistoryAcquisitionNative$", "-test.v", "-test.timeout=1m"], f"history-success-{number}",
                        env=dict(os.environ, ARDENTS_SOURCE_HISTORY_BINDING=str(binding), ARDENTS_SOURCE_HISTORY_RELEASE=history, ARDENTS_SOURCE_HISTORY_CASE="success"))

            def replacement_command(suffix):
                if suffix == "A":
                    return [active_node, "installation", "replace-source", initial_distribution, history, "alpha", network, binding, paths["REQUEST"]]
                if suffix == "EXPIRED":
                    return [paths["NODE_EXPIRED"], "installation", "replace-source", Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_EXPIRED"]), history, "alpha", network, binding, paths["REQUEST_B"]]
                return [paths["NODE_" + suffix], "installation", "replace-source", Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_" + suffix]), history, "alpha", network, binding, paths["REQUEST_" + suffix]]

            def refuse_replacement(suffix, name):
                before = binding.read_bytes()
                before_unit = Path("/etc/systemd/system/ardents-source.service").read_bytes()
                generations = sorted(path.name for path in (installation / "generations").iterdir())
                journals = sorted(path.name for path in installation.parent.glob(installation.name + ".source-replacement-*-journal"))
                code, _ = run(replacement_command(suffix), name, refusal=True)
                if code == 0 or binding.read_bytes() != before or Path("/etc/systemd/system/ardents-source.service").read_bytes() != before_unit:
                    raise RuntimeError("refused replacement changed selection/unit")
                if sorted(path.name for path in (installation / "generations").iterdir()) != generations or sorted(path.name for path in installation.parent.glob(installation.name + ".source-replacement-*-journal")) != journals or (installation / "source-replacement-pending").exists():
                    raise RuntimeError("pre-effect replacement refusal created a transition")

            def replace_generation(suffix, number):
                nonlocal selected_document, selected, plan, request, distribution, active_node, native_observer, selected_floors
                predecessor = dict(selected)
                predecessor_descriptor = json.loads((plan.parent / "infrastructure-source.json").read_text())
                before_floors = selected_floors
                predecessor_sha = digest(binding)
                predecessor_unit = digest(Path("/etc/systemd/system/ardents-source.service"))
                retirement_path = installation / ("source-startup" if number == 0 else f"source-invocation-{number:08d}") / "source-retirement.json"
                retirement_sha = digest(retirement_path)
                # Return the independent native observer's own fixed-unit
                # reference before replacement. The product must establish its
                # own manager custody even after collection.
                native_observer.close()
                native_observer = None
                recovering = recovery_operation == "replacement" and suffix == "B"
                raw = cut_and_recover(replacement_command(suffix), "replacement", paths["REQUEST_" + suffix]) if recovering else run(replacement_command(suffix), "replacement-" + suffix)
                expected_outcome = "source-recovery-stopped-selected" if recovering else "source-replacement-stopped-selected"
                if json.loads(raw) != {"operation":"installation.source", "outcome":expected_outcome}:
                    raise RuntimeError("foreign replacement completion projection")
                selected_document = json.loads(binding.read_text())
                if list(selected_document) != ["schema", "configuration", "initial_generation", "predecessor_binding_sha256", "retirement_sha256", "predecessor_attempt"] or selected_document["schema"] != "ardents-source-replacement-binding-v1" or selected_document["initial_generation"] != lineage_initial or selected_document["predecessor_binding_sha256"] != predecessor_sha or selected_document["retirement_sha256"] != retirement_sha or selected_document["predecessor_attempt"] != number:
                    raise RuntimeError("replacement lineage commitment differs")
                selected = selected_document["configuration"]
                if selected["schema"] != "ardents-source-replacement-configuration-v1" or selected["predecessor"] != predecessor["generation"] or selected["generation"] == predecessor["generation"] or (selected["uid"], selected["gid"]) != (uid, gid):
                    raise RuntimeError("replacement is not strictly forward on original principal")
                directory = installation / "generations" / selected["generation"]
                plan = directory / "source-plan.json"
                descriptor = json.loads((directory / "infrastructure-source.json").read_text())
                if descriptor["release_version"] <= predecessor_descriptor["release_version"]:
                    raise RuntimeError("replacement descriptor is not strictly forward")
                after_floors = release_floors()
                if before_floors["root"] != after_floors["root"] or any(after_floors[role][0] <= before_floors[role][0] for role in ("timestamp", "snapshot", "targets")):
                    raise RuntimeError("replacement reset Root continuity or failed to advance Release floors")
                selected_floors = after_floors
                request = json.loads(paths["REQUEST_" + suffix].read_text())
                if suffix == "C" and (selected["program_sha256"] != predecessor["program_sha256"] or request["address"] == predecessor["request"]["address"]):
                    raise RuntimeError("resource-only replacement did not preserve program and change address")
                if suffix == "B" and selected["program_sha256"] == predecessor["program_sha256"]:
                    raise RuntimeError("program replacement did not change actual bytes")
                names = sorted(path.name for path in directory.iterdir())
                if len(names) != 12 or (directory / "binding.json").read_bytes() != binding.read_bytes():
                    raise RuntimeError("complete sealed replacement inventory missing")
                phases = ["initial", "replacement-admitted", "predecessor-binding", "predecessor-unit", "predecessor-retired"] + ["file-intent", "file-born", "file-ready"] + ["directory-intent", "directory-born", "directory-ready"] + [phase for _ in names for phase in ("file-intent", "file-born", "file-ready")] + ["seal-intent", "sealed", "selection-intent", "selection-written", "unit-intent", "unit-written", "reload-intent", "replacement-verified"]
                journal = Path(str(installation) + ".source-replacement-" + selected["generation"] + "-journal")
                rows = records(journal, phases)
                transcript = recovered_transcript() if recovering else journal.read_bytes()
                if len(rows) != 55 or any(list(row) != ["sequence", "phase", "path", "device", "inode", "sha256", "mode", "uid", "gid"] for row in rows) or any(json.dumps(row, separators=(",", ":")).encode() != line for row, line in zip(rows, transcript.splitlines(), strict=True)):
                    raise RuntimeError("canonical replacement transcript differs")
                if rows[2]["sha256"] != predecessor_sha or rows[3]["sha256"] != predecessor_unit or rows[4]["sha256"] != retirement_sha or rows[-1]["sha256"] != digest(binding):
                    raise RuntimeError("independent replacement preimage commitments differ")
                hi = Path(history).stat()
                commitment = {"schema":"ardents-source-replacement-history-v1", "binding_sha256":digest(binding), "history":history, "device":hi.st_dev, "inode":hi.st_ino, "mode":stat.S_IMODE(hi.st_mode), "uid":hi.st_uid, "gid":hi.st_gid}
                if rows[1]["sha256"] != hashlib.sha256(json.dumps(commitment, separators=(",", ":")).encode() + b"\n").hexdigest():
                    raise RuntimeError("replacement is not committed to original Release native history")
                for index, name in enumerate(names):
                    triplet = rows[11 + index * 3:14 + index * 3]
                    path = directory / name
                    info = path.stat()
                    for row in triplet:
                        if row["path"] != str(path) or row["sha256"] != digest(path) or row["mode"] != (0o555 if name == "ardents-node-linux-amd64" else 0o444) or (row["uid"], row["gid"]) != (0, 0):
                            raise RuntimeError("independent replacement file commitment differs")
                    if (triplet[0]["device"], triplet[0]["inode"]) != (0, 0) or any((row["device"], row["inode"]) != (info.st_dev, info.st_ino) for row in triplet[1:]):
                        raise RuntimeError("replacement file birth/readback native identity differs")
                    originals[path] = (identity(path), digest(path))
                for path, intent, written in ((binding, 49, 50), (Path("/etc/systemd/system/ardents-source.service"), 51, 52)):
                    info = path.stat()
                    for row in (rows[intent], rows[written]):
                        if row["path"] != str(path) or row["sha256"] != digest(path) or (row["device"], row["inode"]) != (info.st_dev, info.st_ino):
                            raise RuntimeError("same-inode replacement publication differs")
                if (installation / "source-replacement-pending").exists():
                    raise RuntimeError("replacement completion retained blocker")
                distribution = Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_" + suffix])
                active_node = paths["NODE_" + suffix]
                inspect[3] = plan
                retained()
                refuse_replacement(suffix, "replacement-replay-" + suffix)
                refuse_replacement("A", "replacement-rollback-A-after-" + suffix)
                native_observer = NativeObserver("native-observer-after-" + suffix)
                stopped = native_observer.sample()
                if stopped["Service"]["MainPID"] != {"type":"u", "data":0} or stopped["Unit"]["ActiveState"] != {"type":"s", "data":"inactive"}:
                    raise RuntimeError("replacement started a runtime without explicit fresh start")
                outcome["replacement_" + suffix + "_generation"] = selected["generation"]

            def partial_replacement():
                previous = binding.read_bytes()
                unit = Path("/etc/systemd/system/ardents-source.service")
                previous_unit = unit.read_bytes()
                candidate = Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_B"])
                generation = digest(candidate / "infrastructure-source.json")
                journal = Path(str(installation) + ".source-replacement-" + generation + "-journal")
                mounted = None
                partial_errors = []
                try:
                    if case == "replacement-candidate-collision":
                        # A real independent preexisting directory causes the
                        # exclusive birth to fail. It supplies no successful
                        # candidate, authority, byte receipt or continuation.
                        (installation / "generations" / generation).mkdir(mode=0o755)
                    else:
                        target = binding if case == "replacement-selection-readonly" else unit
                        original = identity(target)
                        subprocess.run(["/usr/bin/mount", "--bind", str(target), str(target)], check=True)
                        mounted = target
                        subprocess.run(["/usr/bin/mount", "-o", "remount,bind,ro", str(mounted)], check=True)
                        if identity(mounted) != original or not os.statvfs(mounted).f_flag & os.ST_RDONLY:
                            raise RuntimeError("original same-inode read-only native failure not established")
                    code, _ = run(replacement_command("B"), "partial-replacement", refusal=True)
                    body = journal.read_bytes()
                    rows = [json.loads(line) for line in body.splitlines()]
                    pending = installation / "source-replacement-pending"
                    if code == 0 or not pending.is_file() or not body.endswith(b"\n") or not 8 < len(rows) < 55 or [row["sequence"] for row in rows] != list(range(len(rows))):
                        raise RuntimeError("failed native transition did not preserve original partial blocker/journal")
                    if unit.read_bytes() != previous_unit or any(identity(path) != expected for path, expected in selection_inodes.items()):
                        raise RuntimeError("partial replacement changed fixed-unit bytes or original publication inodes")
                    if case == "replacement-unit-readonly":
                        if binding.read_bytes() == previous or rows[-1]["phase"] != "selection-written":
                            raise RuntimeError("unit failure did not follow actual separate selection publication")
                    elif binding.read_bytes() != previous:
                        raise RuntimeError("pre-selection failure published new selection")
                    saved = {path: path.read_bytes() for path in (binding, unit, pending, journal)}
                except BaseException as error:
                    partial_errors.append(error)
                finally:
                    if mounted is not None:
                        try:
                            subprocess.run(["/usr/bin/umount", str(mounted)], check=True)
                        except BaseException as error:
                            partial_errors.append(error)
                if partial_errors:
                    raise BaseExceptionGroup("original partial replacement and native fault cleanup failed", partial_errors)
                # Releasing the physical failure is not repair/continuation.
                # The original partial journal and blocker still deny effects.
                for command, name in ((replacement_command("B"), "partial-replacement-replay"),
                                      ([paths["NODE_B"], "installation", "restart-source", candidate, history, "alpha", network, binding], "partial-replacement-start")):
                    code, _ = run(command, name, refusal=True)
                    if code == 0 or any(path.read_bytes() != raw for path, raw in saved.items()) or (installation / "source-invocation-00000001").exists():
                        raise RuntimeError("partial transition admitted continuation/start or rewrote evidence")
                outcome["partial_native_transition_denied"] = True

            def canceled_reload():
                # A separate kernel observer retains all original command
                # threads through reaping. The native manager is never replaced
                # by a successful test neighbor or an accepting callback.
                nonlocal native_observer
                with Path("/etc/ardents-source-stopped-native-stand").open("x") as marker:
                    marker.write("ardents-source-stopped-native-v1\n")
                run([paths["OWNER_TEST"], "-test.run=^TestSourceReplacementNativeClosePreservesFailureAndJoins$", "-test.v", "-test.timeout=30s"], "replacement-terminal-owner-causes",
                    env=dict(os.environ, ARDENTS_SOURCE_STOPPED_FUTURE_PARENT="/var/lib"))
                native_observer.close()
                native_observer = None
                command = [str(part) for part in replacement_command("B")]
                run([paths["CUT_TEST"], "-test.run=^TestSourceReplacementReloadCancellationNative$", "-test.v", "-test.timeout=2m"], "original-reload-cancellation",
                    env=dict(os.environ, ARDENTS_SOURCE_REPLACEMENT_COMMAND=json.dumps(command),
                             ARDENTS_SOURCE_REPLACEMENT_CUT_OUTPUT=str(output / "original-replacement.log")))
                generation = digest(Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_B"]) / "infrastructure-source.json")
                journal = Path(str(installation) + ".source-replacement-" + generation + "-journal")
                pending = installation / "source-replacement-pending"
                unit = Path("/etc/systemd/system/ardents-source.service")
                body = journal.read_bytes()
                phases = ["initial", "replacement-admitted", "predecessor-binding", "predecessor-unit", "predecessor-retired"] + ["file-intent", "file-born", "file-ready"] + ["directory-intent", "directory-born", "directory-ready"] + [phase for _ in range(12) for phase in ("file-intent", "file-born", "file-ready")] + ["seal-intent", "sealed", "selection-intent", "selection-written", "unit-intent", "unit-written", "reload-intent"]
                rows = records(journal, phases)
                if len(rows) != 54 or not pending.is_file() or any(list(row) != ["sequence", "phase", "path", "device", "inode", "sha256", "mode", "uid", "gid"] for row in rows) or any(json.dumps(row, separators=(",", ":")).encode() != line for row, line in zip(rows, body.splitlines(), strict=True)):
                    raise RuntimeError("canceled actual Reload lost its exact original prefix/blocker")
                saved = {path: path.read_bytes() for path in (binding, unit, pending, journal)}
                for command, name in ((replacement_command("B"), "canceled-replacement-replay"),
                                      ([paths["NODE_B"], "installation", "restart-source", Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_B"]), history, "alpha", network, binding], "canceled-replacement-start")):
                    code, _ = run(command, name, refusal=True)
                    if code == 0 or any(path.read_bytes() != raw for path, raw in saved.items()) or (installation / "source-invocation-00000001").exists():
                        raise RuntimeError("uncertain Reload admitted later replacement/start or rewrote original evidence")
                outcome["original_reload_cancellation_join_and_refusal_proved"] = True

            def release_floors():
                current = (Path(history) / "current").read_text()
                if len(current) != 65 or not current.endswith("\n") or any(c not in "0123456789abcdef" for c in current[:-1]):
                    raise RuntimeError("complete original Release pointer required")
                raw = (Path(history) / "generations" / current[:-1] / "state.bin").read_text()
                result = {}
                for line in raw.splitlines():
                    role, version, sha = line.split()
                    if not role.startswith("role=") or not version.startswith("version=") or not sha.startswith("digest="):
                        raise RuntimeError("independent Release floor framing differs")
                    name = role[5:]
                    if name in result or len(sha[7:]) != 64:
                        raise RuntimeError("independent Release floor inventory differs")
                    result[name] = (int(version[8:]), sha[7:])
                if set(result) != {"root", "timestamp", "snapshot", "targets"}:
                    raise RuntimeError("complete original Release floors required")
                return result

            selected_floors = release_floors() if replacement_profile else None
            for number in range(4 if replacement_profile else 3):
                if invocation in invocations:
                    raise RuntimeError("successor reused an old invocation identity")
                refuse_restart(f"pre-retirement-restart-{number}")
                if (installation / f"source-invocation-{number + 1:08d}").exists():
                    raise RuntimeError("refused pre-retirement restart created a successor attempt")
                explicit_stop(number)
                if number == (3 if replacement_profile else 2):
                    break
                if replacement_profile:
                    if number == 0 and case == "replacement-reload-cancel":
                        canceled_reload()
                        break
                    if number == 0 and case == "replacement-expired-release":
                        metadata = Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION_EXPIRED"]) / "timestamp.json"
                        expires = datetime.fromisoformat(json.loads(metadata.read_text())["signed"]["expires"].replace("Z", "+00:00")).timestamp()
                        while time.time() <= expires:
                            observe()
                            time.sleep(0.2)
                        refuse_replacement("EXPIRED", "replacement-actual-expired-release")
                        retained()
                        outcome["expired_release_denied"] = True
                        break
                    if number == 0 and case.startswith("replacement-history-"):
                        fault = case[len("replacement-history-"):]
                        run([paths["OWNER_TEST"], "-test.run=^TestSourceHistoryAcquisitionNative$", "-test.v", "-test.timeout=1m"], "original-history-refusal-cleanup",
                            env=dict(os.environ, ARDENTS_SOURCE_HISTORY_BINDING=str(binding), ARDENTS_SOURCE_HISTORY_RELEASE=history, ARDENTS_SOURCE_HISTORY_CASE=fault))
                        if fault == "retirement":
                            with Path("/etc/ardents-source-stopped-native-stand").open("x") as marker:
                                marker.write("ardents-source-stopped-native-v1\n")
                            with Path("/var/lib/source-initial-readback.json").open("x") as fixture:
                                json.dump({"binding":str(binding), "history":history, "initial_journal_format":initial_format}, fixture)
                            run([paths["OWNER_TEST"], "-test.run=^(" + "|".join(owner_tests[1:]) + ")$", "-test.v", "-test.timeout=1m"], "original-held-read-and-close-errors",
                                env=dict(os.environ, ARDENTS_SOURCE_STOPPED_FUTURE_PARENT="/var/lib"))
                            outcome["original_held_read_close_errors_retained"] = True
                        outcome["original_history_refusal_cleanup_proved"] = True
                        break
                    if number == 0 and case in ("replacement-account-substitution", "replacement-binding-substitution", "replacement-unit-substitution"):
                        if case == "replacement-account-substitution":
                            passwd = Path("/etc/passwd")
                            rows = passwd.read_text().splitlines()
                            matches = [i for i, row in enumerate(rows) if row.startswith("ardents-source:")]
                            if len(matches) != 1:
                                raise RuntimeError("original fixed Source account unavailable")
                            fields = rows[matches[0]].split(":")
                            if int(fields[2]) != uid:
                                raise RuntimeError("original Source UID differs before native control")
                            (output / "passwd-preimage").write_bytes(passwd.read_bytes())
                            fields[2] = "99999"
                            rows[matches[0]] = ":".join(fields)
                            passwd.write_text("\n".join(rows) + "\n")
                        else:
                            changed = binding if case == "replacement-binding-substitution" else Path("/etc/systemd/system/ardents-source.service")
                            raw, info = changed.read_bytes(), changed.stat()
                            changed.rename(changed.with_name(changed.name + ".qualification-retained"))
                            with changed.open("xb") as stream:
                                stream.write(raw)
                            os.chown(changed, info.st_uid, info.st_gid)
                            os.chmod(changed, stat.S_IMODE(info.st_mode))
                            if identity(changed)[:2] == (info.st_dev, info.st_ino):
                                raise RuntimeError("original publication inode substitution was not established")
                        refuse_replacement("B", "replacement-retained-identity-substitution")
                        outcome["retained_identity_substitution_denied"] = True
                        break
                    if number == 0 and case in ("replacement-candidate-collision", "replacement-selection-readonly", "replacement-unit-readonly"):
                        partial_replacement()
                        break
                    if number == 0 and case in ("replacement-credential-loss", "replacement-state-loss"):
                        lost = Path(request["credential_directory"]) / "certificate.pem" if case == "replacement-credential-loss" else Path(request["state_root"]) / ".ardents-network-state-v1"
                        original_lost = lost.with_name(lost.name + ".qualification-retained")
                        old_identity, old_digest = identity(lost), digest(lost)
                        lost.rename(original_lost)
                        refuse_replacement("B", "replacement-missing-retained-input")
                        if lost.exists() or identity(original_lost) != old_identity or digest(original_lost) != old_digest:
                            raise RuntimeError("refused replacement recreated lost semantic input")
                        outcome["missing_retained_input_denied"] = True
                        break
                    if number < 2:
                        replace_generation("B" if number == 0 else "C", number)
                if not replacement_profile and number == 0 and case != "restart":
                    lost = Path(request["credential_directory"]) / "certificate.pem" if case == "restart-credential-loss" else Path(request["state_root"]) / ".ardents-network-state-v1"
                    original_lost = lost.with_name(lost.name + ".qualification-retained")
                    old_identity, old_digest = identity(lost), digest(lost)
                    lost.rename(original_lost)
                    source_started = True  # Own any partial new invocation.
                    code, _ = run([paths["NODE"], "installation", "restart-source", distribution, history, "alpha", network, binding], "missing-retained-input-restart", refusal=True)
                    if (case == "restart-credential-loss" and code == 0) or lost.exists() or identity(original_lost) != old_identity or digest(original_lost) != old_digest:
                        raise RuntimeError("missing retained input admitted restart or was recreated")
                    # The startup ACK precedes State construction. A missing
                    # State marker must deny the actual runtime even if root's
                    # finite startup operation already returned its own result.
                    # That result cannot supply clean retirement for attempt 1.
                    while True:
                        facts = native_observer.sample()
                        if facts["Service"]["MainPID"] == {"type": "u", "data": 0} and facts["Unit"]["ActiveState"]["data"] in ("inactive", "failed"):
                            break
                        observe()
                        time.sleep(0.05)
                    if case == "restart-state-loss" and facts["Service"]["ExecMainStatus"] != {"type": "i", "data": 2}:
                        raise RuntimeError("actual retained State loss did not fail the installed command")
                    if (installation / "source-invocation-00000001" / "source-retirement.json").exists():
                        raise RuntimeError("failed retained-input attempt produced clean retirement")
                    denied, _ = run([paths["NODE"], "installation", "restart-source", distribution, history, "alpha", network, binding], "failed-invocation-cannot-restart", refusal=True)
                    if denied == 0 or (installation / "source-invocation-00000002").exists():
                        raise RuntimeError("failed invocation admitted a later successor")
                    subprocess.run(["/usr/bin/systemctl", "stop", "ardents-source.service"], check=True)
                    outcome["missing_retained_input_denied"] = True
                    # Failed attempt cleanup is not successful retirement or a
                    # future restart right; outside stand still joins all users.
                    break
                source_started = True
                raw = run([active_node, "installation", "restart-source", distribution, history, "alpha", network, binding], f"restart-{number + 1}")
                if json.loads(raw) != {"operation": "installation.source", "outcome": "source-restart-completed"}:
                    raise RuntimeError("foreign subsequent invocation completion")
                retain_invocation()
                retained()
                actual_contact(f"actual-client-{number + 1}")
            outcome["completed_explicit_retirements"] = len(invocations)
            outcome["distinct_original_invocations"] = len(set(invocations))
        elif case == "credential-substitution":
            certificate = Path(request["credential_directory"]) / "certificate.pem"
            data, info = certificate.read_bytes(), certificate.stat()
            certificate.rename(certificate.with_name("certificate.pem.original"))
            with certificate.open("xb") as stream:
                stream.write(data)
            os.chmod(certificate, stat.S_IMODE(info.st_mode))
            os.chown(certificate, info.st_uid, info.st_gid)
            # A new physical client request follows the completed inode change;
            # no second family, Plan retry or alternate source is introduced.
            refused, response_raw = run(inspect, "changed-input-client", refusal=True)
            if refused == 0:
                message = json.loads(response_raw)
                if message.get("status") == "ok":
                    raise RuntimeError("changed original credential admitted new response")
            outcome["changed_original_input_denied"] = True
        elif case == "clock-loss":
            clock_stop.set()
            clock_thread.join()
            os.utime(clock_fd, (1, 1))
            if os.fstat(clock_fd).st_mtime_ns != 1_000_000_000:
                raise RuntimeError("original clock loss was not established")
            # The current Source contract distributes retained signed bytes;
            # current membership/time authority is a separate State observation.
            # Neither inspection nor offline preparation establishes that right.
            after = json.loads(run(inspect, "stale-clock-retained-distribution"))
            if after != response:
                raise RuntimeError("retained Source distribution changed after clock loss")
            outcome["stale_clock_retained_distribution_observed"] = True
        observe()
    except RecoveryRefusalComplete:
        pass
    except BaseException as error:
        failures.append(error)
    finally:
        if source_started:
            try:
                if manager_owner() != original_manager:
                    raise RuntimeError("original manager lost; outside original stand retains completion custody")
                if source_pidfd is not None and scope_fd is not None:
                    # Signal the original process before asking the manager to
                    # deactivate its unit. Otherwise a legitimate active-state
                    # loss can race the ordinary signal and poison that oracle.
                    try:
                        signal.pidfd_send_signal(source_pidfd, signal.SIGTERM)
                    except ProcessLookupError:
                        if not original_join():
                            raise
                else:
                    # An uncertain first-start failure has no published original
                    # process/scope pair. The own fixed unit remains cleanup only.
                    subprocess.run(["/usr/bin/systemctl", "stop", "ardents-source.service"], check=True)
            except BaseException as error:
                failures.append(error)
        if scope_fd is not None:
            notice, join_error = False, None
            while True:
                try:
                    if original_join():
                        break
                except BaseException as error:
                    if join_error is None:
                        join_error = error
                if not notice and time.monotonic() - started >= bound:
                    print("Source original completion unavailable; retaining scope and outside lock", file=sys.stderr)
                    notice = True
                time.sleep(0.1)
            if join_error is not None:
                failures.append(join_error)
            outcome["original_scope_joined"] = True
            try:
                os.close(scope_fd)
            except OSError as error:
                failures.append(error)
        if source_started and source_pidfd is not None and scope_fd is not None:
            try:
                if manager_owner() != original_manager:
                    raise RuntimeError("original manager changed after Source join")
                subprocess.run(["/usr/bin/systemctl", "stop", "ardents-source.service"], check=True)
            except BaseException as error:
                failures.append(error)
        if source_pidfd is not None:
            try:
                os.close(source_pidfd)
            except OSError as error:
                failures.append(error)
        # State cleanup may still observe trusted time. Keep this original
        # writer alive until the actual Source scope joins, even on interruption.
        clock_stop.set()
        if clock_thread is not None:
            clock_thread.join()
        if clock_fd is not None:
            try:
                os.close(clock_fd)
            except OSError as error:
                failures.append(error)
        if native_observer is not None:
            try:
                native_observer.close()
            except BaseException as error:
                failures.append(error)
    try:
        observe()
        if recovery_profile and recovery_fault is None:
            if not outcome.get("recovery") or not recovery_evidence:
                raise RuntimeError("selected positive recovery was not executed")
            recovered_transcript()
        if recovery_fault is not None:
            if not outcome.get("recovery_refusal") or recovery_evidence or source_started or source_pidfd is not None or scope_fd is not None:
                raise RuntimeError("terminal recovery refusal/native completion missing")
            outcome["retirement_outcome"] = "terminal-recovery-refusal"
            outcome["clean_retirement"] = False
        else:
            if not outcome["original_scope_joined"]:
                raise RuntimeError("no original Source scope completion observation")
            if restart_profile:
                expected = 4 if case == "replacement" else 3 if case == "restart" else 1
                if outcome.get("completed_explicit_retirements") != expected or outcome.get("distinct_original_invocations") != expected:
                    raise RuntimeError("required genuine original retirement sequence incomplete")
                if replacement_profile and outcome.get("old_listeners_closed") != expected:
                    raise RuntimeError("independent old-listener closure observations missing")
                if case in ("restart", "replacement") and not outcome.get("held_original_join_denied_restart"):
                    raise RuntimeError("actual held original join refusal missing")
                if case in ("restart-credential-loss", "restart-state-loss", "replacement-credential-loss", "replacement-state-loss") and not outcome.get("missing_retained_input_denied"):
                    raise RuntimeError("causal retained-input refusal missing")
                if case in ("replacement-candidate-collision", "replacement-selection-readonly", "replacement-unit-readonly") and not outcome.get("partial_native_transition_denied"):
                    raise RuntimeError("causal partial native transition refusal missing")
                if case in ("replacement-account-substitution", "replacement-binding-substitution", "replacement-unit-substitution") and not outcome.get("retained_identity_substitution_denied"):
                    raise RuntimeError("causal original identity substitution refusal missing")
                if case.startswith("replacement-history-") and not outcome.get("original_history_refusal_cleanup_proved"):
                    raise RuntimeError("original history refusal FD/flock regression missing")
                if case == "replacement-history-retirement" and not outcome.get("original_held_read_close_errors_retained"):
                    raise RuntimeError("original held-read/cancellation/Close-error regression missing")
                if case == "replacement-expired-release" and not outcome.get("expired_release_denied"):
                    raise RuntimeError("actual signed expired Release refusal missing")
                if case == "replacement-reload-cancel" and not outcome.get("original_reload_cancellation_join_and_refusal_proved"):
                    raise RuntimeError("actual coordinator Reload cancellation/original join/refusal proof missing")
                outcome["retirement_outcome"] = "explicit-retirements-proved"
                if case == "replacement" and (not outcome.get("held_original_join_denied_replacement") or not outcome.get("replacement_B_generation") or not outcome.get("replacement_C_generation")):
                    raise RuntimeError("complete program and resource-only original replacement sequence missing")
                outcome["clean_retirement"] = case in ("restart", "replacement")
            else:
                # Normal return through the command's joined supervisor is different
                # from a manager timeout/SIGKILL. No clean physical result is inferred
                # from path disappearance or the Stop command's own return.
                code = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "ExecMainCode", "--value"]).strip()
                status = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "ExecMainStatus", "--value"]).strip()
                if code != b"1" or status not in (b"0", b"2"):
                    raise RuntimeError("Source did not return through its actual command supervisor")
                terminal = None
                while terminal is None:
                    observe()
                    if manager_owner() != original_manager:
                        raise RuntimeError("original manager changed before retirement observation")
                    journal = subprocess.check_output(["/usr/bin/journalctl", "--no-pager", "--output=cat", "--lines=32", "--unit=ardents-source.service", "_SYSTEMD_INVOCATION_ID=" + invocation])
                    if len(journal) > 64 << 10:
                        raise RuntimeError("original retirement observation exceeds bound")
                    lines = journal.splitlines()
                    if b'{"operation":"source.start-installed","outcome":"completion-unavailable"}' in lines:
                        raise RuntimeError("original completion was unavailable before final join")
                    reports = [value for value in ("retired", "retirement-failed")
                               if ('{"operation":"source.start-installed","outcome":"' + value + '"}').encode() in lines]
                    if len(reports) > 1:
                        raise RuntimeError("conflicting original retirement reports")
                    if reports:
                        terminal = reports[0]
                    else:
                        time.sleep(0.05)
                outcome["retirement_outcome"] = terminal
                outcome["clean_retirement"] = terminal == "retired"
                if case == "positive" and not outcome["clean_retirement"]:
                    raise RuntimeError("positive Source retirement retained an original failure")
                # Negative cases prove causal refusal and original scope join. A failed
                # terminal report remains failed evidence, never clean cleanup or full
                # installed qualification. The positive retirement gate is separate.
    except BaseException as error:
        failures.append(error)
    outcome["case_passed"] = not failures
    try:
        (output / "private-outcome.json").write_text(json.dumps(outcome, sort_keys=True) + "\n")
    except OSError as error:
        failures.append(error)
    if failures:
        raise BaseExceptionGroup("original Source qualification/cleanup failed", failures)
    print(json.dumps({"operation": "source.startup-qualification", **outcome}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError, BaseExceptionGroup) as error:
        print("Source startup profile failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
