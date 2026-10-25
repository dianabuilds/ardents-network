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
import select
import signal
import socket
import stat
import subprocess
import sys
import threading
import time


RECOVERY_NATIVE_TESTS = (
    "TestSourceRecoveryConcurrentNative",
    "TestSourceRecoveryUnexpectedNoticeNative",
    "TestSourcePreemptedAcknowledgementNative",
    "TestSourceDelayedCutCancellationNative",
    "TestSourceReleaseExpiryProfileNative",
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
    "TestSourceCanonicalPhysicalPayloadNative",
    "TestSourceCanonicalPhysicalEarlyImageNative",
    "TestSourceCanonicalPhysicalSealNative",
    "TestSourceCanonicalPhysicalPublicationNative",
    "TestSourceCanonicalPhysicalCompletedImageNative",
    "TestSourceRecoveryPhysicalImageNative",
    "TestSourceRecoveryRepairedImageNative",
    "TestSourceStartupPhysicalPrefixNative",
    "TestSourceRetirementPhysicalPrefixNative",
    "TestSourceRetirementPhysicalImageNative",
    "TestSourcePhysicalCutControlNative",
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


def source_physical_frontiers(operation):
    """Finite outside qualification windows, with no continuation authority."""
    if operation not in ("initial", "replacement"):
        raise RuntimeError("finite original operation required")
    born, seal = (70, 105) if operation == "initial" else (13, 48)
    frontiers = {
        "file-born-before-record": (born - 1, "file-intent", "ardents-node-linux-amd64", "unrecorded-empty"),
        "file-ready-before-record": (born, "file-born", "ardents-node-linux-amd64", "recorded-born-complete-before-ready"),
        "file-born-before-journal-sync": (born, "file-born", "ardents-node-linux-amd64", "recorded-born-empty-before-journal-sync"),
        "seal-before-sealed": (seal, "seal-intent", "generation", "original-sealed-generation-before-record"),
        "binding-before-record": (109 if operation == "initial" else 50, "file-ready" if operation == "initial" else "selection-intent", "binding.json", "original-published-binding-before-record"),
        "unit-before-record": (113 if operation == "initial" else 52, "file-ready" if operation == "initial" else "unit-intent", "ardents-source.service", "original-published-unit-before-record"),
        "reload-before-verified": (114 if operation == "initial" else 54, "reload-intent", "binding-and-unit", "original-completed-publications-before-verified"),
        "journal-complete-before-close": (115 if operation == "initial" else 55, "stopped-verified" if operation == "initial" else "replacement-verified", "binding-and-unit", "original-complete-journal-before-writer-close"),
        "pending-before-unlink": (115 if operation == "initial" else 55, "stopped-verified" if operation == "initial" else "replacement-verified", "pending", "original-closed-journal-before-pending-unlink"),
    }
    if operation == "initial":
        frontiers.update({
            "account-before-complete-birth": (24, "account-born", "account-databases", "completed-helper-incomplete-account-birth"),
            "directory-born-before-record": (42, "directory-intent", "installation-root", "unrecorded-empty-root-directory"),
        })
    return frontiers


def source_profile(arguments, requested_case):
    """Select a finite qualification operation; this grants no runtime right."""
    if arguments not in (["-timeout=3m"], ["-timeout=8m"], ["-timeout=10m"]):
        raise RuntimeError("fixed Source qualification arguments required")
    recovery_profile = arguments == ["-timeout=10m"] and requested_case.startswith("recovery-")
    replacement_profile = arguments == ["-timeout=10m"]
    restart_profile = recovery_profile or replacement_profile or arguments == ["-timeout=8m"]
    bound = 600 if recovery_profile or replacement_profile else 480 if restart_profile else 180
    case, operation, count, fault, concurrent, physical, root_physical, startup_physical, retirement_physical = requested_case, None, None, None, False, None, None, None, None
    if recovery_profile:
        # A positive frontier inventory does not cover unsafe physical windows
        # or replace the separately required causal-negative matrix.
        counts = {"initial": (70, 106, 110, 113, 114, 115), "replacement": (13, 49, 50, 51, 52, 53, 54, 55)}
        selected = {f"recovery-{op}-{number}": (op, number) for op, values in counts.items() for number in values}
        faults = ("held-journal-lease", "copied-journal", "copied-cut-result", "journal-access", "torn-journal", "missing-pending", "partial-completion", "copied-generation-file", "manager-reexec", "timestamp-expiry")
        negatives = {f"recovery-refusal-{op}-{reason}": (op, number, reason)
                     for op, number in (("initial", 70), ("replacement", 49)) for reason in faults}
        # These are distinct predecessor facts. A real process exit, complete
        # candidate or another retained proof cannot replace a missing one.
        negatives.update({f"recovery-refusal-replacement-{reason}": ("replacement", 49, reason)
                          for reason in ("missing-startup-journal", "missing-startup-archive",
                                         "missing-retirement-journal", "missing-retirement-result")})
        concurrent_cases = {f"recovery-concurrent-{op}": (op, number) for op, number in (("initial", 70), ("replacement", 13))}
        physical_cases = {f"recovery-physical-{op}-{window}": (op, frontier[0], window)
                          for op in ("initial", "replacement") for window, frontier in source_physical_frontiers(op).items()}
        root_cases = {f"recovery-root-{op}-{number}-{window}": (op, number, window)
                      for op, values in (("initial", (70, 115)), ("replacement", (13, 55)))
                      for number in values for window in ("completion-before-close", "pending-before-unlink")}
        root_cases.update({f"recovery-root-{op}-{number}-repaired-body-before-record": (op, number, "repaired-body-before-record")
                           for op, number in (("initial", 70), ("replacement", 13))})
        startup_cases = {f"recovery-startup-{attempt}-{window}": {"attempt": attempt, "window": window}
                         for attempt in (0, 1) for window in ("archive-before-archived", "ack-before-written")}
        retirement_cases = {f"recovery-retirement-{attempt}-{window}": {"attempt": attempt, "window": window}
                            for attempt in (0, 1) for window in ("native-before-joined", "result-before-close", "pending-before-unlink")}
        if case in retirement_cases:
            operation, retirement_physical = "retirement", retirement_cases[case]
        elif case in startup_cases:
            operation, startup_physical = "startup", startup_cases[case]
        elif case in root_cases:
            operation, count, root_physical = root_cases[case]
        elif case in physical_cases:
            operation, count, physical = physical_cases[case]
        elif case in concurrent_cases:
            operation, count = concurrent_cases[case]
            concurrent = True
        elif case in negatives:
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
            "case": case, "requested_case": requested_case, "operation": operation, "prefix_records": count, "fault": fault,
            "concurrent": concurrent, "physical": physical, "root_physical": root_physical, "startup_physical": startup_physical, "retirement_physical": retirement_physical}


def source_startup_prediction(matrix, attempt, window):
    if type(attempt) is not int or attempt not in (0, 1) or window not in ("archive-before-archived", "ack-before-written"):
        raise RuntimeError("finite first/subsequent startup cut required")
    expected = {"attempt": attempt, "window": window, "prefix_records": 19 if window == "archive-before-archived" else 21,
                "root_signal": "SIGKILL", "source_join": "original-pidfd-and-cgroup", "pending": "retained",
                "after_cut": "repair-required", "start_permitted": False}
    inventory = matrix.get("startup_cuts")
    if (not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory)
            or json.dumps([value for value in inventory if value.get("attempt") == attempt and value.get("window") == window], sort_keys=True, separators=(",", ":"))
            != json.dumps([expected], sort_keys=True, separators=(",", ":"))):
        raise RuntimeError("separate exact published startup cut prediction required")
    return expected


def source_startup_cut_observation(profile, ready, report=None, raw=None):
    """Root hold/reap evidence only; the outside owner separately joins Source."""
    pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    window = profile.get("window")
    count = 19 if window == "archive-before-archived" else 21
    numeric = ("entry", "root_pid", "held_root_thread", "selected_records")
    archive = ready.get("startup_archive")
    archive_numbers = ("pid", "process_device", "process_inode", "cgroup_device", "cgroup_inode", "started_realtime", "started_monotonic")
    if (profile.get("schema") != "source-startup-physical-cut-observer-v1" or window not in ("archive-before-archived", "ack-before-written")
            or type(profile.get("attempt")) is not int or profile["attempt"] not in (0, 1)
            or any(type(ready.get(name)) is not int or ready[name] <= 0 for name in numeric)
            or ready["selected_records"] != count or ready.get("profile_sha256") != pin
            or ready.get("program_sha256") != profile["program_sha256"] or ready.get("public_receipt_sha256") != profile["public_receipt_sha256"]
            or ready.get("physical_window") != window or ready.get("inspection_only") is not False or ready.get("installed_execution") is not True
            or ready.get("external_cut_ready") is not True or ready.get("external_cut_requested", False) is not False
            or type(ready.get("root_exit")) is not int or ready["root_exit"] != -1
            or type(ready.get("root_signal", 0)) is not int or ready.get("root_signal", 0) != 0
            or any(ready.get(name, False) is not False for name in ("root_kill_requested", "root_killed_by_observer", "external_cancellation_acknowledged", "root_ownership_released_before_exit", "refusal_ownership_released_before_exit", "refusal_original_scope_joined", "recovery_root_only_at_hold", "recovery_held_before_record"))
            or source_report_has_pending_guard(ready) or ready.get("retirement_result") is not None or not isinstance(archive, dict)
            or any(type(archive.get(name)) is not int or archive[name] <= 0 for name in archive_numbers)
            or archive.get("schema") != ("ardents-source-first-start-archive-v1" if profile["attempt"] == 0 else "ardents-source-invocation-archive-v1")
            or not isinstance(archive.get("invocation_id"), str) or len(archive["invocation_id"]) != 32
            or archive["invocation_id"] == "0" * 32 or any(value not in "0123456789abcdef" for value in archive["invocation_id"])
            or archive.get("generation") != profile["generation"] or archive.get("binding_sha256") != profile["binding_sha256"]
            or not isinstance(ready.get("selected_journal_sha256"), str) or len(ready["selected_journal_sha256"]) != 64
            or any(value not in "0123456789abcdef" for value in ready["selected_journal_sha256"])):
        raise RuntimeError("actual original startup held root/archive observation required")
    if report is None:
        if raw is not None:
            raise RuntimeError("completed startup output without report")
        return {name: ready[name] for name in numeric} | {"profile_sha256": pin}
    if (raw is None or any(type(report.get(name)) is not int or report[name] != ready[name] for name in numeric)
            or any(report.get(name) != ready.get(name) for name in ("profile_sha256", "program_sha256", "public_receipt_sha256", "physical_window", "selected_journal_sha256", "startup_archive"))
            or report.get("inspection_only") is not False or report.get("installed_execution") is not True
            or report.get("external_cut_ready") is not True or report.get("external_cut_requested") is not True
            or type(report.get("root_exit")) is not int or report["root_exit"] != -1
            or type(report.get("root_signal")) is not int or report["root_signal"] != 9
            or type(report.get("processes")) is not int or report["processes"] != 1
            or type(report.get("peak_live_processes")) is not int or report["peak_live_processes"] != 1
            or type(report.get("threads_joined")) is not int or not 1 <= report["threads_joined"] <= 256
            or report.get("root_kill_requested") is not True or report.get("root_killed_by_observer") is not True
            or any(report.get(name, False) is not False for name in ("external_cancellation_acknowledged", "root_ownership_released_before_exit", "refusal_ownership_released_before_exit", "refusal_original_scope_joined", "recovery_root_only_at_hold", "recovery_held_before_record", "refusal_exclusion_observed", "refusal_missing_cut_observed"))
            or any(type(report.get(name, 0)) is not int or report.get(name, 0) != 0 for name in ("root_close_entry", "root_close_observations", "refusal_admission_entry", "held_coordinator_thread", "signalled_root_thread"))
            or source_report_has_pending_guard(report) or report.get("retirement_result") is not None or report.get("original_cut_sha256") != ""
            or report.get("output_sha256") != hashlib.sha256(raw).hexdigest()
            or b'"operation":"installation.source"' in raw or b"source command failed" in raw):
        raise RuntimeError("actual startup root SIGKILL/reaps without domain completion required")
    return {name: report[name] for name in numeric + ("threads_joined", "root_signal")} | {"profile_sha256": pin, "output_sha256": report["output_sha256"]}


SOURCE_EXPIRY_FILES = ("ardents-node-linux-amd64", "infrastructure-source.json", "ardents-source.service", "root.json", "timestamp.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json")


def source_release_expiry_prediction(matrix, operation, count):
    if type(count) is not int or (operation, count) not in (("initial", 70), ("replacement", 49)):
        raise RuntimeError("finite original expiry frontier required")
    expected = {"operation": operation, "prefix_records": count, "fault": "timestamp-expiry",
                "observation": "same-eight-files-actual-wall-crossing", "other_bounds": "live-announced-no-emergency",
                "after_cut": "repair-required", "failure_header_permitted": False, "start_permitted": False}
    inventory = matrix.get("release_expiry")
    if (not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory)
            or json.dumps([value for value in inventory if value.get("operation") == operation and value.get("prefix_records") == count], sort_keys=True, separators=(",", ":"))
            != json.dumps([expected], sort_keys=True, separators=(",", ":"))):
        raise RuntimeError("separate exact published Release expiry prediction required")
    return expected


def source_release_expiry_observation(profile, report, before=None):
    """Real Release decisions only; canonical refusal/join is separate evidence."""
    if not isinstance(profile, dict) or not isinstance(report, dict):
        raise RuntimeError("typed original expiry profile and report required")
    def hexadecimal(value):
        return isinstance(value, str) and len(value) == 64 and all(c in "0123456789abcdef" for c in value)

    def instant(value):
        if not isinstance(value, str) or not value.endswith("Z"):
            raise RuntimeError("actual UTC expiry observation required")
        try:
            return datetime.fromisoformat(value.replace("Z", "+00:00"))
        except ValueError as error:
            raise RuntimeError("actual UTC expiry observation required") from error

    profile_fields = {"schema", "public_receipt_sha256", "distribution", "history", "network", "phase", "files"}
    report_fields = {"profile_sha256", "public_receipt_sha256", "phase", "reference_time", "timestamp_expires", "other_bounds", "files", "outcomes", "authorized", "floors_unchanged", "verifier_closed", "installed_execution"}
    phase = profile.get("phase")
    files = profile.get("files")
    if (set(profile) != profile_fields or profile.get("schema") != "source-release-expiry-observer-v1" or phase not in ("before", "expired")
            or not hexadecimal(profile.get("public_receipt_sha256"))
            or any(not isinstance(profile.get(name), str) or not profile[name] for name in ("distribution", "history", "network"))
            or not isinstance(files, dict) or set(files) != set(SOURCE_EXPIRY_FILES) or any(not hexadecimal(value) for value in files.values())):
        raise RuntimeError("closed exact original expiry profile required")
    pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    if (set(report) != report_fields or report.get("profile_sha256") != pin or report.get("public_receipt_sha256") != profile["public_receipt_sha256"]
            or report.get("phase") != phase or report.get("files") != files
            or report.get("floors_unchanged") is not True or report.get("verifier_closed") is not True or report.get("installed_execution") is not True
            or report.get("outcomes") != (["no-update", "no-update"] if phase == "before" else ["release-expired", "release-expired"])
            or not isinstance(report.get("authorized"), list) or len(report["authorized"]) != 2
            or any(value is not (phase == "before") for value in report["authorized"])):
        raise RuntimeError("actual completed installed Release decision pair required")
    reference, expiry = instant(report.get("reference_time")), instant(report.get("timestamp_expires"))
    if (phase == "before") != (reference < expiry):
        raise RuntimeError("reported wall time does not match actual expiry phase")
    bounds = report.get("other_bounds")
    names = {"root.json", "snapshot.json", "targets.json"} | {target + suffix for target in ("ardents/linux-amd64/node", "ardents/linux-amd64/infrastructure/source") for suffix in ("/no-new", "/terminate")}
    if not isinstance(bounds, dict) or set(bounds) != names or any(instant(value) <= reference or instant(value) <= expiry for value in bounds.values()):
        raise RuntimeError("another original authority clock competes with Timestamp expiry")
    if phase == "expired":
        if not isinstance(before, dict):
            raise RuntimeError("actual same-input pre-expiry observation required")
        source_release_expiry_observation(dict(profile, phase="before"), before)
        if (before["timestamp_expires"] != report["timestamp_expires"] or before["other_bounds"] != bounds
                or before["files"] != files or not instant(before["reference_time"]) < expiry <= reference):
            raise RuntimeError("expiry observations changed original inputs or clocks")
    elif before is not None:
        raise RuntimeError("completed before observation cannot borrow an earlier report")
    return {"profile_sha256": pin, "reference_time": report["reference_time"], "timestamp_expires": report["timestamp_expires"],
            "files_sha256": hashlib.sha256(json.dumps(files, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
            "outcomes": report["outcomes"], "floors_unchanged": True, "verifier_closed": True}


def source_completed_manager_facts(facts):
    """Keep all stopped-manager facts except independently varying free RAM.

    systemd's Service.MemoryAvailable includes physical/parent-slice consumption
    by other units (org.freedesktop.systemd1). It is not an original invocation,
    configured limit or retirement fact. Preserve the raw report in the native
    observer log and require its exact uint64 type even when comparing facts.
    """
    service = facts.get("Service") if isinstance(facts, dict) else None
    available = service.get("MemoryAvailable") if isinstance(service, dict) else None
    if (not isinstance(available, dict) or set(available) != {"type", "data"}
            or available["type"] != "t" or type(available["data"]) is not int
            or not 0 <= available["data"] < 1 << 64):
        raise RuntimeError("typed original manager available-memory observation required")
    return dict(facts, Service={key: value for key, value in service.items() if key != "MemoryAvailable"})


def source_completed_prediction(matrix, attempt, kind):
    if type(attempt) is not int or attempt not in (0, 1, 2, 3) or kind not in ("startup", "retirement"):
        raise RuntimeError("finite completed Source classification required")
    expected = {"attempt": attempt, "journal_kind": kind, "startup_records": 22, "retirement_records": 12,
                "after": "stopped-selected", "persistent_effects": "none", "start_permitted": False}
    inventory = matrix.get("completed_classifications")
    if (not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory)
            or json.dumps([value for value in inventory if value.get("attempt") == attempt and value.get("journal_kind") == kind], sort_keys=True, separators=(",", ":"))
            != json.dumps([expected], sort_keys=True, separators=(",", ":"))):
        raise RuntimeError("separate exact completed Source prediction required")
    return expected


def source_retirement_prediction(matrix, attempt, window):
    if type(attempt) is not int or attempt not in (0, 1) or window not in ("native-before-joined", "result-before-close", "pending-before-unlink"):
        raise RuntimeError("finite first/subsequent retirement cut required")
    expected = {"attempt": attempt, "window": window, "prefix_records": 6 if window == "native-before-joined" else 12,
                "root_signal": "SIGKILL", "source_join": "original-pidfd-and-cgroup-before-root-cut", "pending": "retained",
                "result": "absent" if window == "native-before-joined" else "complete-retained",
                "after_cut": "repair-required", "start_permitted": False}
    inventory = matrix.get("retirement_cuts")
    if (not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory)
            or json.dumps([value for value in inventory if value.get("attempt") == attempt and value.get("window") == window], sort_keys=True, separators=(",", ":"))
            != json.dumps([expected], sort_keys=True, separators=(",", ":"))):
        raise RuntimeError("separate exact published retirement cut prediction required")
    return expected


def source_retirement_cut_observation(profile, ready, report=None, raw=None):
    """Root hold/reaps and retained image only; this cannot attest Source join."""
    def hexadecimal(value, size=64):
        return isinstance(value, str) and len(value) == size and all(c in "0123456789abcdef" for c in value)

    pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    window = profile.get("window")
    count = 6 if window == "native-before-joined" else 12
    numeric = ("entry", "root_pid", "held_root_thread", "selected_records")
    archive = ready.get("startup_archive")
    archive_numbers = ("pid", "process_device", "process_inode", "cgroup_device", "cgroup_inode", "started_realtime", "started_monotonic")
    if (profile.get("schema") != "source-retirement-physical-cut-observer-v1" or window not in ("native-before-joined", "result-before-close", "pending-before-unlink")
            or type(profile.get("attempt")) is not int or profile["attempt"] not in (0, 1)
            or profile.get("prefix_records") != count or type(profile.get("prefix_records")) is not int
            or profile.get("last_phase") != ("domain-joined" if count == 6 else "retired")
            or any(not hexadecimal(profile.get(name)) for name in ("program_sha256", "public_receipt_sha256", "generation", "binding_sha256", "start_sha256", "start_journal_sha256"))
            or any(type(ready.get(name)) is not int or ready[name] <= 0 for name in numeric)
            or ready["selected_records"] != count or ready.get("profile_sha256") != pin
            or ready.get("program_sha256") != profile["program_sha256"] or ready.get("public_receipt_sha256") != profile["public_receipt_sha256"]
            or ready.get("physical_window") != window or ready.get("inspection_only") is not False or ready.get("installed_execution") is not True
            or ready.get("external_cut_ready") is not True or ready.get("external_cut_requested", False) is not False
            or type(ready.get("root_exit")) is not int or ready["root_exit"] != -1
            or type(ready.get("root_signal", 0)) is not int or ready.get("root_signal", 0) != 0
            or any(ready.get(name, False) is not False for name in ("root_kill_requested", "root_killed_by_observer", "external_cancellation_acknowledged", "root_ownership_released_before_exit", "refusal_ownership_released_before_exit", "refusal_original_scope_joined", "recovery_root_only_at_hold", "recovery_held_before_record", "refusal_exclusion_observed", "refusal_missing_cut_observed"))
            or source_report_has_pending_guard(ready) or not isinstance(archive, dict)
            or any(type(archive.get(name)) is not int or archive[name] <= 0 for name in archive_numbers)
            or archive.get("schema") != ("ardents-source-first-start-archive-v1" if profile["attempt"] == 0 else "ardents-source-invocation-archive-v1")
            or not hexadecimal(archive.get("invocation_id"), 32) or archive["invocation_id"] == "0" * 32
            or not isinstance(archive.get("manager"), str) or not archive["manager"]
            or archive.get("generation") != profile["generation"] or archive.get("binding_sha256") != profile["binding_sha256"]
            or hashlib.sha256(json.dumps(archive, separators=(",", ":")).encode() + b"\n").hexdigest() != profile["start_sha256"]
            or not hexadecimal(ready.get("selected_journal_sha256"))):
        raise RuntimeError("actual original retirement held root/archive observation required")
    result = ready.get("retirement_result")
    if count == 6:
        if result is not None:
            raise RuntimeError("unrecorded retirement result cannot supply completion")
    else:
        fields = ("schema", "attempt", "generation", "binding_sha256", "start_sha256", "invocation_id", "pid", "manager", "challenge", "completion_sha256", "started_realtime", "started_monotonic", "exited_realtime", "exited_monotonic", "code", "status")
        integers = ("attempt", "pid", "started_realtime", "started_monotonic", "exited_realtime", "exited_monotonic", "code", "status")
        if (not isinstance(result, dict) or set(result) != set(fields) or any(type(result.get(name)) is not int for name in integers)
                or result["schema"] != "ardents-source-retirement-v1" or result["attempt"] != profile["attempt"]
                or result["start_sha256"] != profile["start_sha256"] or result["code"] != 1 or result["status"] != 0
                or any(result[name] != archive[name] for name in ("generation", "binding_sha256", "invocation_id", "pid", "manager", "started_realtime", "started_monotonic"))
                or result["exited_realtime"] < archive["started_realtime"] or result["exited_monotonic"] < archive["started_monotonic"]
                or not hexadecimal(result["challenge"]) or result["challenge"] == "0" * 64):
            raise RuntimeError("retirement result differs from original archived invocation")
        frame = b"ardents-source-domain-joined-v1\n" + (archive["invocation_id"] + profile["generation"] + profile["binding_sha256"]).encode("ascii") + bytes.fromhex(result["challenge"])
        if result["completion_sha256"] != hashlib.sha256(frame).hexdigest():
            raise RuntimeError("retirement result does not bind original domain frame")
    if report is None:
        if raw is not None:
            raise RuntimeError("completed retirement output without report")
        return {name: ready[name] for name in numeric} | {"profile_sha256": pin}
    if (raw is None or any(type(report.get(name)) is not int or report[name] != ready[name] for name in numeric)
            or any(json.dumps(report.get(name), sort_keys=True, separators=(",", ":")) != json.dumps(ready.get(name), sort_keys=True, separators=(",", ":"))
                   for name in ("profile_sha256", "program_sha256", "public_receipt_sha256", "physical_window", "selected_journal_sha256", "startup_archive", "retirement_result"))
            or report.get("inspection_only") is not False or report.get("installed_execution") is not True
            or report.get("external_cut_ready") is not True or report.get("external_cut_requested") is not True
            or type(report.get("root_exit")) is not int or report["root_exit"] != -1
            or type(report.get("root_signal")) is not int or report["root_signal"] != 9
            or type(report.get("processes")) is not int or report["processes"] != 1
            or type(report.get("peak_live_processes")) is not int or report["peak_live_processes"] != 1
            or type(report.get("threads_joined")) is not int or not 1 <= report["threads_joined"] <= 256
            or report.get("root_kill_requested") is not True or report.get("root_killed_by_observer") is not True
            or any(report.get(name, False) is not False for name in ("external_cancellation_acknowledged", "root_ownership_released_before_exit", "refusal_ownership_released_before_exit", "refusal_original_scope_joined", "recovery_root_only_at_hold", "recovery_held_before_record", "refusal_exclusion_observed", "refusal_missing_cut_observed"))
            or any(type(report.get(name, 0)) is not int or report.get(name, 0) != 0 for name in ("root_close_entry", "root_close_observations", "refusal_admission_entry", "held_coordinator_thread", "signalled_root_thread"))
            or source_report_has_pending_guard(report) or report.get("original_cut_sha256") != ""
            or report.get("output_sha256") != hashlib.sha256(raw).hexdigest()
            or b'"operation":"installation.source"' in raw or b"source command failed" in raw):
        raise RuntimeError("actual retirement root SIGKILL/reaps without graceful completion required")
    return {name: report[name] for name in numeric + ("threads_joined", "root_signal")} | {"profile_sha256": pin, "output_sha256": report["output_sha256"]}


def source_root_recovery_prediction(matrix, operation, count, window):
    if (type(count) is not int or (operation, count) not in (("initial", 70), ("initial", 115), ("replacement", 13), ("replacement", 55))
            or window not in ("repaired-body-before-record", "completion-before-close", "pending-before-unlink")
            or window == "repaired-body-before-record" and (operation, count) not in (("initial", 70), ("replacement", 13))):
        raise RuntimeError("finite original recovery root window required")
    expected = {"operation": operation, "prefix_records": count, "window": window, "root_signal": "SIGKILL",
                "completion": "partial-retained-no-append" if window == "repaired-body-before-record" else "complete-retained",
                "pending": "retained", "after_cut": "repair-required", "start_permitted": False}
    inventory = matrix.get("root_recovery_cuts")
    if not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory):
        raise RuntimeError("separate published root recovery predictions required")
    selected = [value for value in inventory if value.get("operation") == operation and value.get("prefix_records") == count and value.get("window") == window]
    if json.dumps(selected, sort_keys=True, separators=(",", ":")) != json.dumps([expected], sort_keys=True, separators=(",", ":")):
        raise RuntimeError("exact root process cut prediction required")
    return expected


def source_root_recovery_observation(profile, report, raw):
    # This detached report describes the Linux wait status, even when its
    # grammar is checked on Windows, whose signal module has no SIGKILL.
    linux_sigkill = 9
    pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    numbers = ("entry", "root_pid", "root_exit", "root_signal", "threads_joined", "processes", "peak_live_processes", "held_root_thread", "signalled_root_thread", "selected_records")
    if any(type(report.get(name)) is not int for name in numbers) or source_report_has_directed_cut(report) or source_report_has_pending_guard(report):
        raise RuntimeError("actual root process cut numeric inventory required")
    arguments = profile.get("arguments")
    if (profile.get("schema") != "source-recovery-physical-cut-observer-v1" or not isinstance(arguments, list) or len(arguments) != 8
            or arguments[:2] != ["installation", "recover-source"] or profile.get("window") not in ("repaired-body-before-record", "completion-before-close", "pending-before-unlink")
            or report.get("profile_sha256") != pin or report.get("program_sha256") != profile["program_sha256"]
            or report.get("public_receipt_sha256") != profile["public_receipt_sha256"] or report.get("selected_journal_sha256") != profile["journal_sha256"]
            or report.get("physical_window") != profile["window"] or report["selected_records"] != profile["prefix_records"]
            or report.get("inspection_only") is not False or report.get("installed_execution") is not True
            or report["entry"] <= 0 or report["root_pid"] <= 0 or report["root_exit"] != -1 or report["root_signal"] != linux_sigkill
            or report["processes"] != 1 or report["peak_live_processes"] != 1 or not 1 <= report["threads_joined"] <= 256 or report["held_root_thread"] <= 0 or report["signalled_root_thread"] != 0
            or report.get("root_kill_requested") is not True or report.get("root_killed_by_observer") is not True
            or report.get("external_cancellation_acknowledged") is not False or report.get("root_ownership_released_before_exit", False) is not False
            or any(type(report.get(name, 0)) is not int or report.get(name, 0) != 0 for name in ("root_close_entry", "root_close_observations", "held_coordinator_thread", "recovery_hold_entry", "recovery_cancellation_elapsed_ns", "refusal_admission_entry", "refusal_stopped_threads"))
            or any(report.get(name, False) is not False for name in ("recovery_held_before_record", "recovery_held_image_preserved", "refusal_admission_observed", "refusal_ownership_released_before_exit", "refusal_original_scope_joined", "recovery_root_only_at_hold", "refusal_exclusion_observed", "refusal_missing_cut_observed"))
            or report.get("held_coordinator_killed", False) is not False or report.get("original_cut_sha256") != ""
            or report.get("output_sha256") != hashlib.sha256(raw).hexdigest()
            or b'"operation":"installation.source"' in raw or b"source command failed" in raw):
        raise RuntimeError("actual held root SIGKILL/reaps without Close/cancellation claims required")
    return {name: report[name] for name in numbers} | {"profile_sha256": pin, "output_sha256": report["output_sha256"]}


def source_report_has_directed_cut(report):
    return (report.get("external_cut_ready", False) is not False
            or report.get("external_cut_requested", False) is not False
            or report.get("startup_archive") is not None or report.get("retirement_result") is not None)


def source_report_has_startup_pending(report):
    return (report.get("refusal_startup_pending_observed", False) is not False
            or report.get("refusal_startup_pending_device", 0) != 0
            or report.get("refusal_startup_pending_inode", 0) != 0
            or report.get("refusal_startup_pending_sha256", "") != "")


def source_report_has_retirement_pending(report):
    return (report.get("refusal_retirement_pending_observed", False) is not False
            or report.get("refusal_retirement_pending_device", 0) != 0
            or report.get("refusal_retirement_pending_inode", 0) != 0
            or report.get("refusal_retirement_pending_sha256", "") != "")


def source_report_has_pending_guard(report):
    return source_report_has_startup_pending(report) or source_report_has_retirement_pending(report)

def recovery_fault_retention(installation, generation):
    # Keep native originals on the same parent filesystem, outside both the
    # generation inventory and checkReplacementJournals' evidence prefix.
    retained = installation.parent / (".retained-source-fault-" + generation)
    if retained.is_relative_to(installation) or retained.name.startswith(installation.name + ".source-replacement-"):
        raise RuntimeError("fault retention conflicts with product evidence")
    return retained


def source_retained_leaf(path, read_file):
    """Retain a stale native socket as identity, never as readable file data."""
    before = path.lstat()
    if stat.S_ISSOCK(before.st_mode):
        def identity(info):
            return (info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode), info.st_uid, info.st_gid, info.st_ctime_ns, info.st_mtime_ns)
        if identity(path.lstat()) != identity(before):
            raise RuntimeError("original retained socket changed during observation")
        return (("socket", identity(before)[:5], before.st_ctime_ns, before.st_mtime_ns), 0)
    raw, native = read_file(path, 32 << 20)
    return (("file", native, hashlib.sha256(raw).hexdigest()), len(raw))


def source_concurrent_prediction(matrix, operation, count):
    """Require the separate published prediction before the original cut."""
    if type(count) is not int or (operation, count) not in (("initial", 70), ("replacement", 13)):
        raise RuntimeError("finite original first-program birth required")
    expected = {"operation": operation, "prefix_records": count, "hold_phase": "file-ready",
                "held_file": "ardents-node-linux-amd64", "second_refusal": "original-release-flock",
                "after_cancellation": "repair-required", "partial_completion": "retained-no-append", "start_permitted": False}
    inventory = matrix.get("concurrent_cancellation")
    if not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory):
        raise RuntimeError("separate published concurrent prediction inventory required")
    selected = [value for value in inventory if value.get("operation") == operation]
    if json.dumps(selected, sort_keys=True, separators=(",", ":")) != json.dumps([expected], sort_keys=True, separators=(",", ":")):
        raise RuntimeError("exact published concurrent recovery/cancellation prediction required")
    return expected


def source_physical_prediction(matrix, operation, count, window):
    """No prefix/ACK prediction can stand in for an unrecorded physical effect."""
    frontier = source_physical_frontiers(operation).get(window)
    if frontier is None or type(count) is not int or count != frontier[0]:
        raise RuntimeError("finite original physical frontier required")
    expected = {"operation": operation, "prefix_records": count, "window": window, "held_file": frontier[2], "payload": frontier[3],
                "predicted_outcome": "repair-required", "original_cut_proof": "absent", "completion": "absent", "start_permitted": False}
    inventory = matrix.get("physical_cuts")
    if not isinstance(inventory, list) or any(not isinstance(value, dict) for value in inventory):
        raise RuntimeError("separate published physical-cut inventory required")
    selected = [value for value in inventory if value.get("operation") == operation and value.get("window") == window]
    if json.dumps(selected, sort_keys=True, separators=(",", ":")) != json.dumps([expected], sort_keys=True, separators=(",", ":")):
        raise RuntimeError("exact physical-effect repair-required prediction required")
    return expected


class RecoveryRefusalComplete(Exception):
    """Outside profile control flow after a fully observed terminal refusal."""


def source_pidfd_exited(descriptor):
    poller = select.poll()
    poller.register(descriptor, select.POLLIN)
    events = poller.poll(0)
    if any(fd != descriptor or mask & ~(select.POLLIN | select.POLLHUP) for fd, mask in events):
        raise RuntimeError("original Source pidfd exit observation failed")
    return bool(events)


def source_refusal_observation(profile, report, raw, exclusion_identity=None, missing_cut_parent_identity=None, startup_pending_identity=None, retirement_pending_identity=None, early_prefix_identity=None):
    """Check the pinned observer's actual completed canonical invocation only."""
    if source_report_has_directed_cut(report):
        raise RuntimeError("directed physical cut cannot supply a before-return refusal")
    numeric = ("entry", "refusal_admission_entry", "root_pid", "root_exit", "threads_joined", "processes", "peak_live_processes", "refusal_stopped_threads")
    if any(type(report.get(name)) is not int for name in numeric):
        raise RuntimeError("actual refusal observer numeric inventory required")
    profile_pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    if (report.get("profile_sha256") != profile_pin or report.get("inspection_only") is not False or report.get("installed_execution") is not True
            or report.get("refusal_admission_observed") is not True or report.get("refusal_ownership_released_before_exit") is not True
            or report.get("refusal_original_scope_joined") is not True
            or report.get("recovery_root_only_at_hold", False) is not (profile.get("recovery_hold") is not None)
            or report["entry"] <= 0 or report["refusal_admission_entry"] <= 0 or report["entry"] == report["refusal_admission_entry"]
            or report["root_pid"] <= 0 or report["root_exit"] != 2
            or not 1 <= report["peak_live_processes"] <= 32
            or (report["processes"] > 1 and report["peak_live_processes"] < 2)
            or not report["peak_live_processes"] <= report["processes"] <= report["threads_joined"]
            or not 2 <= report["refusal_stopped_threads"] <= report["threads_joined"] <= 256
            or report.get("program_sha256") != profile["program_sha256"] or report.get("public_receipt_sha256") != profile["public_receipt_sha256"]
            or report.get("output_sha256") != hashlib.sha256(raw).hexdigest() or not raw.endswith(b"source command failed\n")):
        raise RuntimeError("actual canonical before-return refusal/cleanup/join proof missing")

    exclusion = "release" if profile["external_release_lease"] else "journal" if profile["external_journal_lease"] else None
    if exclusion is not None:
        if (exclusion_identity is None or len(exclusion_identity) != 2
                or any(type(value) is not int or value <= 0 for value in exclusion_identity)
                or report.get("refusal_exclusion_observed") is not True or report.get("refusal_exclusion_kind") != exclusion
                or type(report.get("refusal_exclusion_device")) is not int or type(report.get("refusal_exclusion_inode")) is not int
                or (report["refusal_exclusion_device"], report["refusal_exclusion_inode"]) != tuple(exclusion_identity)):
            raise RuntimeError("actual original kernel flock exclusion/inode proof missing")
    missing_cut = profile.get("missing_original_cut", False)
    if missing_cut is True:
        if (profile.get("schema") != "source-recovery-missing-cut-observer-v1"
                or exclusion is not None or profile.get("recovery_hold") is not None or profile.get("startup_pending") is not None or profile.get("retirement_pending") is not None
                or profile.get("journal_mode") != "0600"
                or missing_cut_parent_identity is None or len(missing_cut_parent_identity) != 2
                or any(type(value) is not int or value <= 0 for value in missing_cut_parent_identity)
                or report.get("refusal_missing_cut_observed") is not True
                or type(report.get("refusal_missing_cut_parent_device")) is not int
                or type(report.get("refusal_missing_cut_parent_inode")) is not int
                or (report["refusal_missing_cut_parent_device"], report["refusal_missing_cut_parent_inode"]) != tuple(missing_cut_parent_identity)):
            raise RuntimeError("actual original-cut read ENOENT/parent inode proof missing")
    elif (missing_cut is not False or profile.get("schema") == "source-recovery-missing-cut-observer-v1"
          or missing_cut_parent_identity is not None or report.get("refusal_missing_cut_observed", False) is not False
          or report.get("refusal_missing_cut_parent_device", 0) != 0 or report.get("refusal_missing_cut_parent_inode", 0) != 0):
        raise RuntimeError("mixed original-cut observation purpose")
    pending = profile.get("startup_pending")
    if pending is not None:
        if (not isinstance(pending, dict) or set(pending) != {"attempt", "gid"}
                or type(pending.get("attempt")) is not int or not 0 <= pending["attempt"] < 64
                or type(pending.get("gid")) is not int or not 100 <= pending["gid"] <= 999
                or profile.get("schema") != "source-startup-recovery-refusal-observer-v1"
                or profile.get("journal_mode") != "0600" or profile.get("recovery_hold") is not None
                or missing_cut is not False or exclusion is not None or exclusion_identity is not None or profile.get("retirement_pending") is not None
                or report.get("refusal_exclusion_observed", False) is not False
                or startup_pending_identity is None or len(startup_pending_identity) != 3
                or any(type(value) is not int or value <= 0 for value in startup_pending_identity[:2])
                or not isinstance(startup_pending_identity[2], str) or len(startup_pending_identity[2]) != 64
                or any(value not in "0123456789abcdef" for value in startup_pending_identity[2])
                or report.get("refusal_startup_pending_observed") is not True
                or type(report.get("refusal_startup_pending_device")) is not int
                or type(report.get("refusal_startup_pending_inode")) is not int
                or (report["refusal_startup_pending_device"], report["refusal_startup_pending_inode"], report.get("refusal_startup_pending_sha256")) != tuple(startup_pending_identity)):
            raise RuntimeError("actual original startup pending nofollow stat/identity proof missing")
    elif (profile.get("schema") == "source-startup-recovery-refusal-observer-v1"
          or startup_pending_identity is not None or source_report_has_startup_pending(report)):
        raise RuntimeError("mixed startup pending observation purpose")
    retirement_pending = profile.get("retirement_pending")
    if retirement_pending is not None:
        if (not isinstance(retirement_pending, dict) or set(retirement_pending) != {"attempt", "gid"}
                or type(retirement_pending.get("attempt")) is not int or not 0 <= retirement_pending["attempt"] < 64
                or type(retirement_pending.get("gid")) is not int or not 100 <= retirement_pending["gid"] <= 999
                or profile.get("schema") != "source-retirement-recovery-refusal-observer-v1"
                or profile.get("journal_mode") != "0600" or profile.get("recovery_hold") is not None
                or missing_cut is not False or exclusion is not None or exclusion_identity is not None or profile.get("startup_pending") is not None
                or report.get("refusal_exclusion_observed", False) is not False
                or retirement_pending_identity is None or len(retirement_pending_identity) != 3
                or any(type(value) is not int or value <= 0 for value in retirement_pending_identity[:2])
                or not isinstance(retirement_pending_identity[2], str) or len(retirement_pending_identity[2]) != 64
                or any(value not in "0123456789abcdef" for value in retirement_pending_identity[2])
                or report.get("refusal_retirement_pending_observed") is not True
                or type(report.get("refusal_retirement_pending_device")) is not int
                or type(report.get("refusal_retirement_pending_inode")) is not int
                or (report["refusal_retirement_pending_device"], report["refusal_retirement_pending_inode"], report.get("refusal_retirement_pending_sha256")) != tuple(retirement_pending_identity)):
            raise RuntimeError("actual original retirement pending nofollow stat/identity proof missing")
    elif (profile.get("schema") == "source-retirement-recovery-refusal-observer-v1"
          or retirement_pending_identity is not None or source_report_has_retirement_pending(report)):
        raise RuntimeError("mixed retirement pending observation purpose")
    early_count = profile.get("early_initial_prefix", 0)
    early_fields = ("refusal_early_prefix_observed", "refusal_early_prefix_closed", "refusal_early_prefix_records",
                    "refusal_early_prefix_device", "refusal_early_prefix_inode", "refusal_early_prefix_sha256")
    if early_count != 0:
        arguments = profile.get("arguments")
        if (type(early_count) is not int or early_count not in (24, 42)
                or profile.get("schema") != "source-early-prefix-recovery-refusal-observer-v1"
                or not isinstance(arguments, list) or len(arguments) != 8 or not isinstance(arguments[7], str)
                or profile.get("journal_mode") != "0600" or not arguments[7].endswith(".source-initial-journal")
                or exclusion is not None or exclusion_identity is not None or missing_cut is not False
                or profile.get("recovery_hold") is not None or pending is not None or retirement_pending is not None
                or report.get("refusal_exclusion_observed", False) is not False
                or early_prefix_identity is None or len(early_prefix_identity) != 3
                or any(type(value) is not int or value <= 0 for value in early_prefix_identity[:2])
                or not isinstance(early_prefix_identity[2], str) or len(early_prefix_identity[2]) != 64
                or any(value not in "0123456789abcdef" for value in early_prefix_identity[2])
                or early_prefix_identity[2] != profile.get("journal_sha256")
                or report.get("refusal_early_prefix_observed") is not True or report.get("refusal_early_prefix_closed") is not True
                or any(type(report.get(name)) is not int for name in early_fields[2:5])
                or report["refusal_early_prefix_records"] != early_count
                or (report["refusal_early_prefix_device"], report["refusal_early_prefix_inode"], report.get("refusal_early_prefix_sha256")) != tuple(early_prefix_identity)):
            raise RuntimeError("actual fresh original early-prefix lease/read/close proof missing")
    elif (type(early_count) is not int or profile.get("schema") == "source-early-prefix-recovery-refusal-observer-v1"
          or early_prefix_identity is not None
          or any(report.get(name, False) is not False for name in early_fields[:2])
          or any(type(report.get(name, 0)) is not int or report.get(name, 0) != 0 for name in early_fields[2:5])
          or report.get(early_fields[5], "") != ""):
        raise RuntimeError("mixed early-prefix custody observation purpose")
    result = {"profile_sha256": report["profile_sha256"], "program_sha256": report["program_sha256"],
              "output_sha256": report["output_sha256"], "root_exit": report["root_exit"],
              "root_pid": report["root_pid"],
              "original_scope_joined": True, "processes": report["processes"], "peak_live_processes": report["peak_live_processes"],
              "stopped_threads_before_return": report["refusal_stopped_threads"], "threads_joined": report["threads_joined"]}
    if exclusion is not None:
        result["exclusion"] = {"observed": True, "kind": exclusion, "device": report["refusal_exclusion_device"], "inode": report["refusal_exclusion_inode"]}
    if missing_cut:
        result["missing_original_cut"] = {"observed": True, "parent_device": report["refusal_missing_cut_parent_device"],
                                          "parent_inode": report["refusal_missing_cut_parent_inode"]}
    if pending is not None:
        result["startup_pending"] = {"observed": True, "attempt": pending["attempt"], "device": report["refusal_startup_pending_device"],
                                     "inode": report["refusal_startup_pending_inode"], "sha256": report["refusal_startup_pending_sha256"]}
    if retirement_pending is not None:
        result["retirement_pending"] = {"observed": True, "attempt": retirement_pending["attempt"], "device": report["refusal_retirement_pending_device"],
                                     "inode": report["refusal_retirement_pending_inode"], "sha256": report["refusal_retirement_pending_sha256"]}
    if early_count:
        result["early_prefix"] = {"observed": True, "closed": True, "records": early_count,
                                  "device": report["refusal_early_prefix_device"], "inode": report["refusal_early_prefix_inode"],
                                  "sha256": report["refusal_early_prefix_sha256"]}
    return result


def source_cancellation_observation(profile, ready, report=None, raw=None):
    """Bind the held and completed observations of one original canonical root."""
    if (profile.get("schema") != "source-recovery-cancellation-observer-v1"
            or profile.get("external_journal_lease") is not False or profile.get("external_release_lease") is not False
            or type(profile.get("recovery_hold", {}).get("prefix_records")) is not int
            or profile["recovery_hold"]["prefix_records"] not in (70, 13)):
        raise RuntimeError("separate canonical cancellation purpose required")
    pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    entries = ("entry", "refusal_admission_entry", "recovery_hold_entry")
    numeric = entries + ("root_pid", "held_root_thread")
    if (any(type(ready.get(name)) is not int or ready[name] <= 0 for name in numeric)
            or len({ready[name] for name in entries}) != 3
            or ready.get("profile_sha256") != pin or ready.get("inspection_only") is not False
            or ready.get("installed_execution") is not True or ready.get("refusal_admission_observed") is not True
            or ready.get("recovery_held_before_record") is not True
            or ready.get("recovery_root_only_at_hold") is not True
            or ready.get("refusal_original_scope_joined", False) is not False
            or ready.get("external_cancellation_acknowledged") is not False
            or ready.get("refusal_ownership_released_before_exit", False) is not False
            or ready.get("program_sha256") != profile["program_sha256"]
            or ready.get("public_receipt_sha256") != profile["public_receipt_sha256"]
            or ready.get("selected_journal_sha256") != profile["journal_sha256"]):
        raise RuntimeError("actual original pinned recovery hold missing")
    if report is None:
        if raw is not None:
            raise RuntimeError("completed cancellation output without report")
        return {"root_pid": ready["root_pid"], "held_root_thread": ready["held_root_thread"], "profile_sha256": pin}
    result = source_refusal_observation(profile, report, raw)
    stable = numeric + ("selected_journal_sha256",)
    if (any(type(report.get(name)) is not int for name in numeric)
            or any(report.get(name) != ready[name] for name in stable)
            or report.get("recovery_held_before_record") is not True or report.get("recovery_held_image_preserved") is not True
            or report.get("external_cancellation_acknowledged") is not True
            or type(report.get("signalled_root_thread")) is not int or report["signalled_root_thread"] <= 0
            or report["signalled_root_thread"] == report["held_root_thread"]
            or type(report.get("recovery_cancellation_elapsed_ns")) is not int
            or not 0 < report["recovery_cancellation_elapsed_ns"] < 120_000_000_000
            or raw.splitlines().count(b'{"operation":"installation.source","outcome":"completion-unavailable"}') != 1
            or b'"outcome":"source-recovery-stopped-selected"' in raw):
        raise RuntimeError("actual original causal cancellation/preserved-image/join proof missing")
    result.update(root_pid=report["root_pid"], held_root_thread=report["held_root_thread"],
                  signalled_root_thread=report["signalled_root_thread"],
                  cancellation_elapsed_ns=report["recovery_cancellation_elapsed_ns"],
                  original_journal_sha256=report["selected_journal_sha256"], held_image_preserved=True)
    return result


def source_physical_observation(profile, report, raw, publication_preimage=None):
    if source_report_has_directed_cut(report) or source_report_has_pending_guard(report):
        raise RuntimeError("directed root cut cannot supply managed cancellation")
    numeric = ("entry", "root_pid", "root_exit", "threads_joined", "processes", "peak_live_processes", "held_coordinator_thread", "signalled_root_thread", "selected_records")
    if any(type(report.get(name)) is not int for name in numeric):
        raise RuntimeError("actual physical cut numeric inventory required")
    pin = hashlib.sha256(json.dumps(profile, separators=(",", ":")).encode() + b"\n").hexdigest()
    if (profile.get("schema") != "source-physical-cut-observer-v1" or report.get("profile_sha256") != pin
            or report.get("inspection_only") is not False or report.get("installed_execution") is not True
            or report.get("program_sha256") != profile["program_sha256"] or report.get("public_receipt_sha256") != profile["public_receipt_sha256"]
            or report.get("physical_window") != profile["window"] or report["selected_records"] != profile["prefix_records"]
            or report["entry"] <= 0 or report["root_pid"] <= 0 or report["root_exit"] != 2
            or report["processes"] < 2 or not 2 <= report["peak_live_processes"] <= 32
            or not report["peak_live_processes"] <= report["processes"] <= report["threads_joined"] <= 256
            or report["held_coordinator_thread"] <= 0 or report["held_coordinator_thread"] == report["root_pid"]
            or report["signalled_root_thread"] <= 0 or report["signalled_root_thread"] == report["held_coordinator_thread"]
            or report.get("held_coordinator_killed") is not True or report.get("external_cancellation_acknowledged") is not True
            or report.get("original_cut_sha256") != "" or report.get("output_sha256") != hashlib.sha256(raw).hexdigest()
            or not raw.endswith(b"source command failed\n")
            or raw.splitlines().count(b'{"operation":"installation.source","outcome":"completion-unavailable"}') != 1):
        raise RuntimeError("actual original physical cut/cancellation/native join proof missing")
    publication = profile["window"] in ("binding-before-record", "unit-before-record")
    arguments = profile.get("arguments")
    if publication and (not isinstance(arguments, list) or len(arguments) != 8 or arguments[0] != "installation"
                        or arguments[1] not in ("install-initial-stopped", "replace-source")):
        raise RuntimeError("closed original publication command required")
    required = publication and arguments[1] == "replace-source"
    if required:
        if (not isinstance(publication_preimage, dict) or set(publication_preimage) != {"device", "inode", "sha256"}
                or any(type(publication_preimage[name]) is not int or publication_preimage[name] <= 0 for name in ("device", "inode"))
                or not isinstance(publication_preimage["sha256"], str) or len(publication_preimage["sha256"]) != 64
                or any(value not in "0123456789abcdef" for value in publication_preimage["sha256"])
                or report.get("physical_preimage_observed") is not True
                or type(report.get("physical_preimage_device")) is not int or type(report.get("physical_preimage_inode")) is not int
                or (report["physical_preimage_device"], report["physical_preimage_inode"], report.get("physical_preimage_sha256"))
                != (publication_preimage["device"], publication_preimage["inode"], publication_preimage["sha256"])):
            raise RuntimeError("actual original pre-Commit publication preimage missing")
    elif (publication_preimage is not None or report.get("physical_preimage_observed", False) is not False
          or report.get("physical_preimage_device", 0) != 0 or report.get("physical_preimage_inode", 0) != 0
          or report.get("physical_preimage_sha256", "") != ""):
        raise RuntimeError("mixed publication preimage purpose")
    result = {"profile_sha256": pin, "root_pid": report["root_pid"], "root_exit": report["root_exit"],
            "held_coordinator_thread": report["held_coordinator_thread"], "threads_joined": report["threads_joined"],
            "processes": report["processes"], "peak_live_processes": report["peak_live_processes"], "physical_window": report["physical_window"], "output_sha256": report["output_sha256"]}
    if required:
        result["publication_preimage"] = dict(publication_preimage, observed=True)
    return result


class RecoveryCancellationObserver:
    """Own one sibling tracer, its control pipe, bounded output and final join."""

    ready_marker = b"SOURCE_RECOVERY_HOLD_READY "
    completion_marker = b"SOURCE_REFUSAL_OBSERVATION "
    control_command = b"cancel\n"

    def __init__(self, argv, environment, log_path, observe):
        self.observe, self.errors = observe, []
        self.events = queue.Queue(maxsize=3)
        self.closed = self.cancelled = False
        self.log = log_path.open("xb")
        try:
            self.child = subprocess.Popen([str(value) for value in argv], stdin=subprocess.PIPE,
                                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=environment)
        except BaseException:
            self.log.close()
            raise

        def drain():
            total = 0
            try:
                while True:
                    line = self.child.stdout.readline(65538)
                    if not line:
                        break
                    total += len(line)
                    if len(line) > 65537 or total > 4 << 20:
                        raise RuntimeError("recovery cancellation observer output exceeds finite bound")
                    self.log.write(line)
                    for marker, kind in ((self.ready_marker, "ready"), (self.completion_marker, "completed")):
                        if line.startswith(marker):
                            self.events.put_nowait((kind, json.loads(line[len(marker):])))
            except BaseException as error:
                self.errors.append(error)
                # Retain the actual pipe until EOF even after malformed output.
                try:
                    while self.child.stdout.read(65536):
                        pass
                except OSError as cleanup:
                    self.errors.append(cleanup)
            finally:
                try:
                    self.child.stdout.close()
                except OSError as error:
                    self.errors.append(error)

        self.reader = threading.Thread(target=drain)
        try:
            self.reader.start()
            kind, self.ready = self.receive()
            if kind != "ready":
                raise RuntimeError("original recovery hold must precede completion")
        except BaseException as error:
            try:
                self.close()
            except BaseException as cleanup:
                raise BaseExceptionGroup("recovery hold initialization and join failed", [error, cleanup])
            raise

    def receive(self):
        while True:
            self.observe()
            if self.errors:
                raise BaseExceptionGroup("recovery cancellation output failed", list(self.errors))
            try:
                return self.events.get(timeout=0.05)
            except queue.Empty:
                if self.child.poll() is not None and not self.reader.is_alive():
                    # The reader may enqueue and exit after get() timed out.
                    # Once joined/finished, one final queue read is authoritative.
                    try:
                        return self.events.get_nowait()
                    except queue.Empty:
                        raise RuntimeError("recovery observer ended before original observation") from None

    def cancel(self):
        self.observe()
        if self.closed or self.cancelled or self.child.poll() is not None:
            raise RuntimeError("one cancellation of the original held recovery required")
        self.child.stdin.write(self.control_command)
        self.child.stdin.flush()
        self.cancelled = True

    def finish(self):
        errors, report = [], None
        try:
            if not self.cancelled:
                raise RuntimeError("outside control must precede completion")
            kind, report = self.receive()
            if kind != "completed":
                raise RuntimeError("one completed original recovery observation required")
        except BaseException as error:
            errors.append(error)
        try:
            self.close(require_success=True)
        except BaseException as error:
            errors.append(error)
        if errors:
            raise BaseExceptionGroup("recovery cancellation completion/join failed", errors)
        return report

    def close(self, require_success=False):
        if self.closed:
            return
        self.closed = True
        errors = []
        try:
            # EOF makes the tracer abort and join its root if still held.
            self.child.stdin.close()
        except OSError as error:
            errors.append(error)
        if self.reader.ident is None:
            try:
                self.child.stdout.close()
            except OSError as error:
                errors.append(error)
        code = self.child.wait()
        if self.reader.ident is not None:
            self.reader.join()
        try:
            self.log.close()
        except OSError as error:
            errors.append(error)
        errors.extend(self.errors)
        if require_success and (code != 0 or not self.events.empty()):
            errors.append(RuntimeError("recovery observer failed or emitted duplicate observations"))
        if errors:
            raise BaseExceptionGroup("original recovery observer pipe/process join failed", errors)


class PhysicalCutObserver(RecoveryCancellationObserver):
    """Own the held native root until one explicit outside cut directive."""

    ready_marker = b"SOURCE_PHYSICAL_CUT_READY "
    completion_marker = b"SOURCE_PHYSICAL_CUT_OBSERVATION "
    control_command = b"cut\n"

    def cut(self):
        super().cancel()


class StartupPhysicalCutObserver(PhysicalCutObserver):
    completion_marker = b"SOURCE_STARTUP_PHYSICAL_OBSERVATION "


class RetirementPhysicalCutObserver(PhysicalCutObserver):
    completion_marker = b"SOURCE_RETIREMENT_PHYSICAL_OBSERVATION "


def main():
    profile = source_profile(sys.argv[1:], os.environ["ARDENTS_SOURCE_STARTUP_CASE"])
    recovery_profile, replacement_profile, restart_profile = (profile[name] for name in ("recovery", "replacement", "restart"))
    bound, case, requested_case = (profile[name] for name in ("bound", "case", "requested_case"))
    recovery_operation, recovery_count, recovery_fault = (profile[name] for name in ("operation", "prefix_records", "fault"))
    recovery_concurrent = profile["concurrent"]
    recovery_physical = profile["physical"]
    recovery_root_physical = profile["root_physical"]
    startup_physical = profile["startup_physical"]
    retirement_physical = profile["retirement_physical"]
    positive_recovery = recovery_profile and recovery_fault is None and not recovery_concurrent and recovery_physical is None and recovery_root_physical is None and startup_physical is None and retirement_physical is None
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
        paths.update({name: pinned(name) for name in ("RECOVERY_OBSERVER", "RECOVERY_MATRIX", "CUT_CONTROL_RUNNER")})
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
    active_request = paths["REQUEST"]
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

    def recovery_snapshot(declaration, installation=False, *, early=False):
        # This is a finite private-stand preservation oracle, not State or
        # Release admission. It does not export private bytes or repair files.
        saved, total = {}, 0
        roots = [Path(declaration[name]) for name in ("state_root", "participation_root", "credential_directory")] + [Path(history)]
        if installation:
            roots.append(Path(declaration["installation_root"]))
        for directory in roots:
            if early and directory != Path(history) and directory != Path(declaration["installation_root"]):
                if directory.exists() or directory.is_symlink():
                    raise RuntimeError("early prefix acquired a future semantic or credential root")
            if early and not directory.exists() and not directory.is_symlink():
                saved[str(directory)] = ("absent",)
                continue
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
                    entry, size = source_retained_leaf(path, native_image)
                    total += size
                    if total > 128 << 20:
                        raise RuntimeError("private preservation bytes exceed finite bound")
                    saved[str(path)] = entry
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

    def prepare_refusal_observer(command, declaration_path, installation, journal, name, *, external_journal=False, external_release=False, hold=None, missing_cut=False, startup_pending=None, retirement_pending=None, early_prefix=0):
        if early_prefix and (type(early_prefix) is not int or early_prefix not in (24, 42) or external_journal or external_release or hold is not None or missing_cut or startup_pending is not None or retirement_pending is not None):
            raise RuntimeError("separate early-prefix custody purpose required")
        if (startup_pending is not None or retirement_pending is not None) and (missing_cut or hold is not None or external_journal or external_release or startup_pending is not None and retirement_pending is not None):
            raise RuntimeError("separate startup pending refusal purpose required")
        if missing_cut and (hold is not None or external_journal or external_release):
            raise RuntimeError("separate original-cut refusal purpose required")
        refusal_command = [command[0], "installation", "recover-source", command[3], history, "alpha", network, declaration_path, journal]
        declaration = {"schema": "source-early-prefix-recovery-refusal-observer-v1" if early_prefix else "source-retirement-recovery-refusal-observer-v1" if retirement_pending is not None else "source-startup-recovery-refusal-observer-v1" if startup_pending is not None else "source-recovery-missing-cut-observer-v1" if missing_cut else "source-recovery-cancellation-observer-v1" if hold is not None else "source-recovery-refusal-observer-v1",
                       "program_sha256": digest(Path(command[0])), "public_receipt_sha256": outcome["public_receipt_sha256"],
                       "arguments": [str(value) for value in refusal_command[1:]], "journal_sha256": digest(journal),
                       "journal_mode": f"{stat.S_IMODE(journal.lstat().st_mode):04o}",
                       "external_journal_lease": external_journal, "external_release_lease": external_release}
        if hold is not None:
            declaration["recovery_hold"] = hold
        if missing_cut:
            declaration["missing_original_cut"] = True
        if startup_pending is not None:
            declaration["startup_pending"] = startup_pending
        if retirement_pending is not None:
            declaration["retirement_pending"] = retirement_pending
        if early_prefix:
            declaration["early_initial_prefix"] = early_prefix
        path = output / (name + "-profile.json")
        with path.open("xb") as stream:
            stream.write(json.dumps(declaration, separators=(",", ":")).encode() + b"\n")
        original_output = output / (name + "-original.log")
        environment = dict(os.environ, ARDENTS_SOURCE_REFUSAL_OBSERVER_MODE="installed-early-prefix" if early_prefix else "installed-retirement-pending" if retirement_pending is not None else "installed-startup-pending" if startup_pending is not None else "installed-missing-cut" if missing_cut else "installed-cancellation" if hold is not None else "installed-refusal",
                           ARDENTS_SOURCE_CUT_OBSERVER_PROFILE=str(path), ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256=digest(path),
                           ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=str(command[0]), ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(installation),
                           ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(original_output))
        return declaration, environment, original_output

    def before_return_refusal(command, declaration_path, installation, journal, name, *, external_journal=False, external_release=False, exclusion_identity=None, missing_cut_parent_identity=None, startup_pending=None, startup_pending_identity=None, retirement_pending=None, retirement_pending_identity=None, early_prefix=0, early_prefix_identity=None):
        selected, environment, original_output = prepare_refusal_observer(command, declaration_path, installation, journal, name, external_journal=external_journal, external_release=external_release, missing_cut=missing_cut_parent_identity is not None, startup_pending=startup_pending, retirement_pending=retirement_pending, early_prefix=early_prefix)
        observed = run([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceRefusalObserverNative$", "-test.v", "-test.timeout=3m"], name + "-observer", env=environment)
        reports = [json.loads(line[len(b"SOURCE_REFUSAL_OBSERVATION "):]) for line in observed.splitlines() if line.startswith(b"SOURCE_REFUSAL_OBSERVATION ")]
        if len(reports) != 1 or reports[0].get("profile_sha256") != digest(Path(environment["ARDENTS_SOURCE_CUT_OBSERVER_PROFILE"])):
            raise RuntimeError("one original pinned before-return refusal report required")
        refused, _ = native_image(original_output, 1 << 20)
        return source_refusal_observation(selected, reports[0], refused, exclusion_identity, missing_cut_parent_identity, startup_pending_identity, retirement_pending_identity, early_prefix_identity), refused

    def physical_sealed_generation(rows, installation, generation, program_pin, *, before_sealed=True):
        directory = installation / "generations" / generation
        info = directory.lstat()
        native = native_identity(info)
        intents = [row for row in rows if row["phase"] == "seal-intent" and row["path"] == str(directory)]
        if len(intents) != 1:
            raise RuntimeError("one original seal intent required")
        last = intents[0]
        if (not stat.S_ISDIR(info.st_mode) or native[2:] != (0o555, 0, 0)
                or (last["path"], last["device"], last["inode"], last["sha256"], last["mode"], last["uid"], last["gid"])
                != (str(directory), native[0], native[1], generation, 0o555, 0, 0)):
            raise RuntimeError("actual original sealed generation and seal intent required")
        if before_sealed:
            if last["sequence"] != len(rows)-1:
                raise RuntimeError("seal effect must precede sealed record")
        elif last["sequence"]+1 >= len(rows) or rows[last["sequence"]+1] != dict(last, sequence=last["sequence"]+1, phase="sealed"):
            raise RuntimeError("original completed seal pair required")
        names = sorted(("ardents-node-linux-amd64", "ardents-source.service", "binding.json", "infrastructure-enrollment.json",
                        "infrastructure-source.json", "rendered-source.service", "root.json", "snapshot.json", "source-plan.json",
                        "source-request.json", "targets.json", "timestamp.json"))
        if sorted(path.name for path in directory.iterdir()) != names:
            raise RuntimeError("exact sealed generation inventory required")
        retained_files, total = [], 0
        for name in names:
            path = directory / name
            matches = [row for row in rows if row["phase"] == "file-ready" and row["path"] == str(path)]
            if len(matches) != 1:
                raise RuntimeError("one original sealed file ready record required")
            ready = matches[0]
            body, identity = native_image(path, 64 << 20)
            total += len(body)
            mode = 0o555 if name == "ardents-node-linux-amd64" else 0o444
            if (total > 128 << 20 or identity != (ready["device"], ready["inode"], mode, 0, 0)
                    or (ready["mode"], ready["uid"], ready["gid"]) != (mode, 0, 0)
                    or hashlib.sha256(body).hexdigest() != ready["sha256"]
                    or name == "ardents-node-linux-amd64" and ready["sha256"] != program_pin):
                raise RuntimeError("actual original sealed generation file differs")
            retained_files.append(path)
        after = directory.lstat()
        if (native_identity(after) != native or after.st_ctime_ns != info.st_ctime_ns or after.st_mtime_ns != info.st_mtime_ns
                or sorted(path.name for path in directory.iterdir()) != names):
            raise RuntimeError("sealed generation changed during readback")
        return (directory, *retained_files)

    def physical_cut_and_refuse(command, operation, declaration_path):
        source_physical_prediction(recovery_matrix, operation, recovery_count, recovery_physical)
        declaration = json.loads(declaration_path.read_text())
        installation = Path(declaration["installation_root"])
        generation = digest(Path(command[3]) / "infrastructure-source.json")
        journal = Path(str(installation) + (".source-initial-journal" if operation == "initial" else ".source-replacement-" + generation + "-journal"))
        pending = Path(str(installation) + ".source-initial-pending") if operation == "initial" else installation / "source-replacement-pending"
        matrix_operation = "initial-recoverable" if operation == "initial" else "replacement"
        phases = [row["phase"] for row in recovery_matrix["canonical_prefixes"] if row["operation"] == matrix_operation]
        last_phase = source_physical_frontiers(operation)[recovery_physical][1]
        if len(phases) < recovery_count or phases[recovery_count-1] != last_phase:
            raise RuntimeError("published original physical phase/frontier differs")
        selected = {"schema": "source-physical-cut-observer-v1", "program_sha256": digest(Path(command[0])),
                    "public_receipt_sha256": outcome["public_receipt_sha256"], "arguments": [str(value) for value in command[1:]],
                    "journal": str(journal), "prefix_records": recovery_count, "last_phase": last_phase,
                    "window": recovery_physical, "generation": generation}
        path = output / "original-physical-cut-profile.json"
        with path.open("xb") as stream:
            stream.write(json.dumps(selected, separators=(",", ":")).encode() + b"\n")
        original_output = output / "original-physical-command.log"
        environment = dict(os.environ, ARDENTS_SOURCE_CANONICAL_PHYSICAL_OBSERVER_MODE="installed-physical",
                           ARDENTS_SOURCE_CUT_OBSERVER_PROFILE=str(path), ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256=digest(path),
                           ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=str(command[0]), ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(installation),
                           ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(original_output))
        publication_preimage = None
        fixed = None
        if recovery_physical in ("binding-before-record", "unit-before-record"):
            fixed = installation / "binding.json" if recovery_physical == "binding-before-record" else Path("/etc/systemd/system/ardents-source.service")
            if operation == "replacement":
                before, native = native_image(fixed, 32 << 10)
                if native[2:] != (0o644, 0, 0):
                    raise RuntimeError("original fixed publication preimage access required")
                publication_preimage = {"device": native[0], "inode": native[1], "sha256": hashlib.sha256(before).hexdigest()}
        observed = run([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceCanonicalPhysicalCutObserverNative$", "-test.v", "-test.timeout=3m"],
                       "original-physical-observer", env=environment)
        reports = [json.loads(line[len(b"SOURCE_CANONICAL_PHYSICAL_OBSERVATION "):]) for line in observed.splitlines() if line.startswith(b"SOURCE_CANONICAL_PHYSICAL_OBSERVATION ")]
        if len(reports) != 1 or reports[0].get("profile_sha256") != digest(path):
            raise RuntimeError("one original pinned canonical physical observation required")
        original_raw, _ = native_image(original_output, 1 << 20)
        physical = source_physical_observation(selected, reports[0], original_raw, publication_preimage)
        original, identity = native_image(journal)
        rows = canonical_rows(original)
        if len(rows) != recovery_count or [row["phase"] for row in rows] != phases[:recovery_count] or hashlib.sha256(original).hexdigest() != reports[0].get("selected_journal_sha256"):
            raise RuntimeError("actual physical cut differs from published original prefix")
        sealed_files = ()
        early = operation == "initial" and recovery_physical in ("account-before-complete-birth", "directory-born-before-record")
        completed = recovery_physical in ("reload-before-verified", "journal-complete-before-close", "pending-before-unlink")
        if early:
            if recovery_physical == "account-before-complete-birth":
                if rows[-1]["path"] != "/etc/passwd" or installation.exists() or installation.is_symlink():
                    raise RuntimeError("incomplete account birth acquired an installation directory")
            elif rows[-1]["path"] != str(installation):
                raise RuntimeError("original first-directory intent differs")
            if any(row["phase"] in ("file-born", "directory-born") for row in rows[5:]):
                # The only earlier file-born is the initial pending blocker.
                raise RuntimeError("early prefix includes dependent native births")
        elif recovery_physical == "seal-before-sealed" or fixed is not None or completed:
            sealed_files = physical_sealed_generation(rows, installation, generation, selected["program_sha256"], before_sealed=recovery_physical == "seal-before-sealed")
            if completed:
                def one(phase, target):
                    matches = [row for row in rows if row["phase"] == phase and row["path"] == str(target)]
                    if len(matches) != 1:
                        raise RuntimeError("one exact original completed publication record required")
                    return matches[0]

                hashes = {}
                for target, name, intent, written, predecessor in (
                        (installation / "binding.json", "binding.json", "selection-intent", "selection-written", "predecessor-binding"),
                        (Path("/etc/systemd/system/ardents-source.service"), "rendered-source.service", "unit-intent", "unit-written", "predecessor-unit")):
                    candidate, _ = native_image(installation / "generations" / generation / name, 32 << 10)
                    actual, native = native_image(target, 32 << 10)
                    ready = one("file-ready" if operation == "initial" else written, target)
                    if (actual != candidate or native != (ready["device"], ready["inode"], 0o644, 0, 0)
                            or (ready["mode"], ready["uid"], ready["gid"]) != (0o644, 0, 0)
                            or ready["sha256"] != hashlib.sha256(actual).hexdigest()):
                        raise RuntimeError("actual original completed fixed publication differs")
                    if operation == "initial":
                        born, planned = one("file-born", target), one("file-intent", target)
                        if (born != dict(ready, sequence=ready["sequence"]-1, phase="file-born")
                                or planned != dict(ready, sequence=ready["sequence"]-2, phase="file-intent", device=0, inode=0)):
                            raise RuntimeError("original completed publication birth triple differs")
                    else:
                        previous = one(predecessor, target)
                        if (one(intent, target) != dict(ready, sequence=ready["sequence"]-1, phase=intent)
                                or previous["sequence"] >= ready["sequence"]-1
                                or (previous["device"], previous["inode"], previous["mode"], previous["uid"], previous["gid"]) != native):
                            raise RuntimeError("original completed publication pair/predecessor differs")
                    hashes[name] = ready["sha256"]
                    sealed_files += (target,)
                unit_path = Path("/etc/systemd/system/ardents-source.service")
                reload_record = {"sequence": recovery_count-1, "phase": "reload-intent", "path": str(unit_path),
                                 "device": 0, "inode": 0, "sha256": hashes["rendered-source.service"], "mode": 0o644, "uid": 0, "gid": 0}
                if recovery_physical != "reload-before-verified":
                    reload_record["sequence"] -= 1
                    complete_record = dict(reload_record, sequence=recovery_count-1, phase=last_phase,
                                           path=str(installation / "binding.json"), sha256=hashes["binding.json"])
                    if rows[-1] != complete_record:
                        raise RuntimeError("original complete frontier differs")
                if one("reload-intent", unit_path) != reload_record:
                    raise RuntimeError("original reload frontier differs")
            if fixed is not None:
                name = "binding.json" if recovery_physical == "binding-before-record" else "rendered-source.service"
                candidate, _ = native_image(installation / "generations" / generation / name, 32 << 10)
                actual, native = native_image(fixed, 32 << 10)
                last = rows[-1]
                if (actual != candidate or last["path"] != str(fixed) or last["sha256"] != hashlib.sha256(actual).hexdigest()
                        or native != (last["device"], last["inode"], 0o644, 0, 0)
                        or (last["mode"], last["uid"], last["gid"]) != (0o644, 0, 0)
                        or publication_preimage is not None and native[:2] != (publication_preimage["device"], publication_preimage["inode"])):
                    raise RuntimeError("actual original fixed-file publication differs")
                sealed_files += (fixed,)
        else:
            payload = installation / "generations" / generation / "ardents-node-linux-amd64"
            body, payload_identity = native_image(payload, 64 << 20)
            last = rows[-1]
            if last["path"] != str(payload) or last["sha256"] != selected["program_sha256"] or (last["mode"], last["uid"], last["gid"]) != (0o555, 0, 0):
                raise RuntimeError("original physical first-program record differs")
            if recovery_physical == "file-born-before-record":
                if body or payload_identity[2:] != (0o600, 0, 0) or (last["device"], last["inode"]) != (0, 0):
                    raise RuntimeError("actual empty unrecorded original program birth required")
            elif recovery_physical == "file-born-before-journal-sync":
                if body or payload_identity != (last["device"], last["inode"], 0o600, 0, 0):
                    raise RuntimeError("actual empty original born program before journal Sync required")
            elif (hashlib.sha256(body).hexdigest() != selected["program_sha256"]
                  or payload_identity != (last["device"], last["inode"], 0o555, 0, 0)):
                raise RuntimeError("actual original born program body before ready required")
        absent = tuple(Path(str(journal) + suffix) for suffix in (".original-cut-journal", ".original-cut-result", ".original-cut-pending", ".recovery-completion"))
        watched = (journal, pending, Path("/etc/systemd/system/ardents-source.service")) + absent

        def retained_image():
            saved = recovery_snapshot(declaration, installation=True, early=early)
            if early:
                if recovery_physical == "directory-born-before-record":
                    native = installation.lstat()
                    if not stat.S_ISDIR(native.st_mode) or native_identity(native)[2:] != (0o700, 0, 0) or list(installation.iterdir()):
                        raise RuntimeError("actual unrecorded empty root0700 directory changed")
                    saved[("early-directory-times", str(installation))] = (native.st_ctime_ns, native.st_mtime_ns)
                elif saved[str(installation)] != ("absent",):
                    raise RuntimeError("incomplete account history acquired an installation root")
                for field in ("state_root", "participation_root", "credential_directory", "installation_root"):
                    ancestor = Path(declaration[field]).parent
                    while True:
                        native = ancestor.lstat()
                        if not stat.S_ISDIR(native.st_mode) or native.st_uid != 0 or native.st_mode & 0o022:
                            raise RuntimeError("trusted early native ancestor required")
                        saved[("early-ancestor", str(ancestor))] = native_identity(native)
                        if ancestor == ancestor.parent:
                            break
                        ancestor = ancestor.parent
                for name in tuple(saved):
                    if isinstance(name, str) and name.startswith("/etc/") and saved[name][0] == "file":
                        native = Path(name).lstat()
                        saved[("early-account-times", name)] = (native.st_ctime_ns, native.st_mtime_ns)
            parent = journal.parent.lstat()
            if not stat.S_ISDIR(parent.st_mode) or parent.st_uid != 0 or parent.st_mode & 0o022:
                raise RuntimeError("trusted original journal parent required")
            saved[str(journal.parent)] = ("directory", parent.st_dev, parent.st_ino, stat.S_IMODE(parent.st_mode), parent.st_uid, parent.st_gid)
            for item in sealed_files:
                native = item.lstat()
                saved[("sealed-file-times", str(item))] = (native.st_ctime_ns, native.st_mtime_ns)
            for item in watched:
                if not item.exists() and not item.is_symlink():
                    saved[str(item)] = ("absent",)
                else:
                    raw, native = native_image(item, 32 << 20)
                    saved[str(item)] = ("file", native, hashlib.sha256(raw).hexdigest())
            return saved

        retained = retained_image()
        if retained[str(pending)][0] != "file" or any(retained[str(item)] != ("absent",) for item in absent):
            raise RuntimeError("uncertain physical cut lost pending or manufactured eligible evidence")
        startup = [command[0], "installation", "start-source", command[3], history, "alpha", network, binding]
        if operation == "replacement":
            startup = [paths["NODE"], "installation", "restart-source", initial_distribution, history, "alpha", network, binding]
        # This is the applicable ordinary entrypoint, not a claim that pending
        # is its sole possible refusal: current Release/State remain separate.
        for argv, name in ((command, "physical-interrupted-replay"), (startup, "physical-interrupted-startup")):
            code, _ = run(argv, name, refusal=True)
            if code == 0 or retained_image() != retained:
                raise RuntimeError("ordinary operation accepted or changed physical-cut history")
        refused = []
        for attempt in range(2):
            proof, _ = before_return_refusal(command, declaration_path, installation, journal, f"physical-recovery-refusal-{attempt}",
                                           missing_cut_parent_identity=None if early else retained[str(journal.parent)][1:3],
                                           early_prefix=recovery_count if early else 0,
                                           early_prefix_identity=(*identity[:2], hashlib.sha256(original).hexdigest()) if early else None)
            refused.append(proof)
            if retained_image() != retained:
                raise RuntimeError("physical recovery refusal changed history or manufactured completion")
        outcome["recovery_refusal"] = {"operation": operation, "prefix_records": recovery_count, "fault": recovery_physical,
                                        "original_failed_exit": physical["root_exit"], "original_failure_retained": True,
                                        "physical_cut": physical, "before_return_refusals": refused, "separate_refused_invocations": 2,
                                        "history_preserved": True, "original_prefix_sha256": hashlib.sha256(original).hexdigest(),
                                        "eligible_cut_proof_absent": True, "completion_absent": True, "matrix_sha256": digest(paths["RECOVERY_MATRIX"])}
        if operation == "initial":
            if source_started or source_pidfd is not None or scope_fd is not None or outcome["original_scope_joined"]:
                raise RuntimeError("never-started initial physical cut acquired a Source runtime scope")
            outcome["source_scope_not_started"] = True
        raise RecoveryRefusalComplete()

    def expiry_artifact_image(distribution):
        image = {}
        for name in sorted(SOURCE_EXPIRY_FILES):
            raw, native = native_image(Path(distribution) / name, 64 << 20)
            image[name] = (hashlib.sha256(raw).hexdigest(), native, len(raw))
        return image

    def release_expiry_observer(command, phase, original, before=None):
        if expiry_artifact_image(command[3]) != original:
            raise RuntimeError("expiry candidate changed after original canonical cut")
        selected = {"schema": "source-release-expiry-observer-v1", "public_receipt_sha256": outcome["public_receipt_sha256"],
                    "distribution": str(command[3]), "history": history, "network": network, "phase": phase,
                    "files": {name: value[0] for name, value in sorted(original.items())}}
        path = output / ("release-expiry-" + phase + "-profile.json")
        with path.open("xb") as stream:
            stream.write(json.dumps(selected, separators=(",", ":")).encode() + b"\n")
        environment = dict(os.environ, ARDENTS_SOURCE_RELEASE_EXPIRY_OBSERVER_MODE="installed-release-expiry",
                           ARDENTS_SOURCE_RELEASE_EXPIRY_PROFILE=str(path), ARDENTS_SOURCE_RELEASE_EXPIRY_PROFILE_SHA256=digest(path))
        began = time.time()
        raw = run([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceReleaseExpiryObserverNative$", "-test.v", "-test.timeout=45s"], "release-expiry-" + phase, env=environment)
        ended = time.time()
        reports = [json.loads(line[len(b"SOURCE_RELEASE_EXPIRY_OBSERVATION "):]) for line in raw.splitlines() if line.startswith(b"SOURCE_RELEASE_EXPIRY_OBSERVATION ")]
        if len(reports) != 1 or expiry_artifact_image(command[3]) != original:
            raise RuntimeError("one actual original unchanged expiry observation required")
        report = reports[0]
        proof = source_release_expiry_observation(selected, report, before)
        reference = datetime.fromisoformat(report["reference_time"].replace("Z", "+00:00")).timestamp()
        if not began <= reference <= ended:
            raise RuntimeError("actual Release reference time escaped original observation interval")
        return report, proof


    def cut_and_recover(command, operation, declaration_path):
        if recovery_evidence or operation != recovery_operation:
            raise RuntimeError("one exact recovery cut per fresh stand required")
        if recovery_physical is not None:
            return physical_cut_and_refuse(command, operation, declaration_path)
        declaration = json.loads(declaration_path.read_text())
        installation = Path(declaration["installation_root"])
        generation = digest(Path(command[3]) / "infrastructure-source.json")
        expiry_original = expiry_artifact_image(command[3]) if recovery_fault == "timestamp-expiry" else None
        if expiry_original is not None:
            source_release_expiry_prediction(recovery_matrix, operation, recovery_count)
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
        if recovery_concurrent:
            source_concurrent_prediction(recovery_matrix, operation, recovery_count)
        if recovery_root_physical is not None:
            source_root_recovery_prediction(recovery_matrix, operation, recovery_count, recovery_root_physical)
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
        if (any(type(report.get(name)) is not int for name in ("processes", "peak_live_processes", "threads_joined"))
                or not 2 <= report["peak_live_processes"] <= 32
                or not report["peak_live_processes"] <= report["processes"] <= report["threads_joined"] <= 256):
            raise RuntimeError("actual original live process bound and complete historical join required")
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
        startup = [command[0], "installation", "start-source", command[3], history, "alpha", network, binding]
        if operation == "replacement":
            startup = [paths["NODE"], "installation", "restart-source", initial_distribution, history, "alpha", network, binding]
        for argv, name in ((command, "interrupted-replay"), (startup, "interrupted-startup")):
            code, _ = run(argv, name, refusal=True)
            if code == 0 or native_image(pending) != (pending_raw, pending_identity):
                raise RuntimeError("interrupted ordinary operation accepted or changed blocker")
        if recovery_snapshot(declaration) != before:
            raise RuntimeError("refused ordinary operations changed original semantic/account history")
        for path, expected in immutable.items():
            raw, info = native_image(Path(path))
            if (info, hashlib.sha256(raw).hexdigest()) != expected:
                raise RuntimeError("refused operation rewrote original interruption proof")

        if recovery_root_physical is not None:
            completion = Path(str(journal) + ".recovery-completion")
            if completion.exists() or completion.is_symlink():
                raise RuntimeError("fresh recovery completion required before root process cut")
            recovery_command = [command[0], "installation", "recover-source", command[3], history, "alpha", network, declaration_path, journal]
            selected = {"schema": "source-recovery-physical-cut-observer-v1", "program_sha256": digest(Path(command[0])),
                        "public_receipt_sha256": outcome["public_receipt_sha256"], "arguments": [str(value) for value in recovery_command[1:]],
                        "journal": str(journal), "prefix_records": recovery_count, "last_phase": row[0]["phase"],
                        "window": recovery_root_physical, "generation": generation, "binding_sha256": boundary["binding_sha256"],
                        "journal_sha256": hashlib.sha256(original).hexdigest(), "original_cut_sha256": hashlib.sha256(proof_raw).hexdigest()}
            path = output / "root-recovery-cut-profile.json"
            with path.open("xb") as stream:
                stream.write(json.dumps(selected, separators=(",", ":")).encode() + b"\n")
            original_output = output / "root-recovery-cut-original.log"
            environment = dict(os.environ, ARDENTS_SOURCE_RECOVERY_PHYSICAL_OBSERVER_MODE="installed-recovery-physical",
                               ARDENTS_SOURCE_CUT_OBSERVER_PROFILE=str(path), ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256=digest(path),
                               ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=str(command[0]), ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(installation),
                               ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(original_output))
            observed = run([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceRecoveryPhysicalCutObserverNative$", "-test.v", "-test.timeout=3m"],
                           "root-recovery-process-cut", env=environment)
            reports = [json.loads(line[len(b"SOURCE_RECOVERY_PHYSICAL_OBSERVATION "):]) for line in observed.splitlines() if line.startswith(b"SOURCE_RECOVERY_PHYSICAL_OBSERVATION ")]
            if len(reports) != 1:
                raise RuntimeError("one original root recovery cut report required")
            original_raw, _ = native_image(original_output, 1 << 20)
            physical = source_root_recovery_observation(selected, reports[0], original_raw)
            completion_raw, completion_identity = native_image(completion)
            partial = recovery_root_physical == "repaired-body-before-record"
            ending = b'],"transcript_sha256":""}'
            encoded = completion_raw + ending if partial else completion_raw.rstrip(b"\n")
            suffix = json.loads(encoded)
            fields = ["schema", "operation", "journal", "device", "inode", "prefix_sha256", "prefix_records", "generation", "binding_sha256", "records", "transcript_sha256"]
            if (list(suffix) != fields or suffix["schema"] != "ardents-source-recovery-completion-v1" or suffix["operation"] != operation
                    or suffix["journal"] != str(journal) or (suffix["device"], suffix["inode"]) != identity[:2]
                    or suffix["prefix_sha256"] != selected["journal_sha256"] or suffix["prefix_records"] != recovery_count
                    or suffix["generation"] != generation or suffix["binding_sha256"] != boundary["binding_sha256"]
                    or not isinstance(suffix["records"], list)
                    or partial and (suffix["records"] != [] or suffix["transcript_sha256"] != "" or not completion_raw.endswith(b'"records":['))):
                raise RuntimeError("independent original-bound recovery envelope required")
            if (json.dumps(suffix, separators=(",", ":"), ensure_ascii=False).encode() + (b"" if partial else b"\n")
                    != (encoded if partial else completion_raw)
                    or not partial and len(suffix["records"]) + len(rows) != len(phases)):
                raise RuntimeError("canonical recovery envelope differs from selected physical window")
            transcript = original + b"".join(json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode() + b"\n" for value in suffix["records"])
            combined = canonical_rows(transcript)
            if ([value["phase"] for value in combined] != (phases[:recovery_count] if partial else phases)
                    or not partial and hashlib.sha256(transcript).hexdigest() != suffix["transcript_sha256"]):
                raise RuntimeError("recovery root transcript differs from selected matrix window")
            if recovery_snapshot(declaration) != before or native_image(pending) != (pending_raw, pending_identity):
                raise RuntimeError("root recovery cut changed semantic history or removed original pending")
            for item, expected in immutable.items():
                body, native = native_image(Path(item))
                if (native, hashlib.sha256(body).hexdigest()) != expected:
                    raise RuntimeError("root recovery rewrote original interruption evidence")
            if partial:
                payload = installation / "generations" / generation / "ardents-node-linux-amd64"
                body, native = native_image(payload, 64 << 20)
                born = rows[-1]
                if (born["phase"] != "file-born" or born["path"] != str(payload) or born["sha256"] != selected["program_sha256"]
                        or hashlib.sha256(body).hexdigest() != born["sha256"]
                        or native != (born["device"], born["inode"], born["mode"], born["uid"], born["gid"])):
                    raise RuntimeError("actual complete repaired original born inode required")
                retained_files = (payload,)
            else:
                retained_files = physical_sealed_generation(combined, installation, generation, selected["program_sha256"], before_sealed=False)
            publications = () if partial else ((installation / "binding.json", "binding.json", "file-ready" if operation == "initial" else "selection-written"),
                                        (Path("/etc/systemd/system/ardents-source.service"), "rendered-source.service", "file-ready" if operation == "initial" else "unit-written"))
            for target, name, phase in publications:
                matches = [value for value in combined if value["phase"] == phase and value["path"] == str(target)]
                if len(matches) != 1:
                    raise RuntimeError("one completed original publication required")
                ready = matches[0]
                candidate, _ = native_image(installation / "generations" / generation / name, 32 << 10)
                actual, native = native_image(target, 32 << 10)
                if (actual != candidate or native != (ready["device"], ready["inode"], 0o644, 0, 0)
                        or (ready["mode"], ready["uid"], ready["gid"]) != (0o644, 0, 0) or ready["sha256"] != hashlib.sha256(actual).hexdigest()):
                    raise RuntimeError("root recovery fixed publication differs")
                retained_files += (target,)
            unit = Path("/etc/systemd/system/ardents-source.service")
            watched = (journal, control, result, pending, completion) + ((unit,) if not partial or operation == "replacement" else ())

            def root_cut_image():
                saved = recovery_snapshot(declaration, installation=True)
                if partial and operation == "initial" and (unit.exists() or unit.is_symlink()):
                    raise RuntimeError("initial repaired-file root cut published a unit")
                for item in watched:
                    body, native = native_image(item, 32 << 20)
                    saved[str(item)] = ("file", native, hashlib.sha256(body).hexdigest())
                for item in (*watched, *retained_files):
                    info = item.lstat()
                    saved[("root-cut-native-times", str(item))] = (info.st_ctime_ns, info.st_mtime_ns)
                if guard.exists() or guard.is_symlink():
                    raise RuntimeError("root recovery changed original proof guard")
                return saved

            retained = root_cut_image()
            for argv, name in ((command, "root-cut-ordinary-replay"), (startup, "root-cut-ordinary-startup")):
                code, _ = run(argv, name, refusal=True)
                if code == 0 or root_cut_image() != retained:
                    raise RuntimeError("ordinary operation accepted or changed root-cut history")
            refused = []
            for attempt in range(2):
                proof, _ = before_return_refusal(command, declaration_path, installation, journal, f"root-cut-fresh-refusal-{attempt+1}")
                refused.append(proof)
                if root_cut_image() != retained:
                    raise RuntimeError("fresh recovery adopted or changed existing root-cut completion")
            outcome["recovery_refusal"] = {"operation": operation, "prefix_records": recovery_count, "fault": recovery_root_physical,
                                            "root_cut": physical, "separate_refused_invocations": 2, "before_return_refusals": refused,
                                            "original_failed_exit": report["root_exit"], "original_failure_retained": True,
                                            "completion": "partial-retained-no-append" if partial else "complete-retained", "pending": "retained", "history_preserved": True,
                                            "matrix_sha256": digest(paths["RECOVERY_MATRIX"])}
            if operation == "initial":
                if source_started or source_pidfd is not None or scope_fd is not None or outcome["original_scope_joined"]:
                    raise RuntimeError("never-started root recovery acquired a Source runtime")
                outcome["source_scope_not_started"] = True
            raise RecoveryRefusalComplete()

        if recovery_concurrent:
            # Independent sibling helpers own their own original Wait4 roots.
            # Snapshot after the admitted repair, before the second invocation.
            completion = Path(str(journal) + ".recovery-completion")
            if completion.exists() or completion.is_symlink():
                raise RuntimeError("fresh first recovery completion required")
            payload = installation / "generations" / generation / "ardents-node-linux-amd64"
            if len(rows) != recovery_count or rows[-1]["phase"] != "file-born" or rows[-1]["path"] != str(payload) or rows[-1]["sha256"] != digest(Path(command[0])):
                raise RuntimeError("exact original first program birth required for concurrent recovery")
            hold = {"phase": "file-ready", "path": str(payload), "body_sha256": digest(Path(command[0])),
                    "generation": generation, "binding_sha256": boundary["binding_sha256"], "prefix_records": recovery_count}
            watched = (journal, control, result, pending, completion, Path("/etc/systemd/system/ardents-source.service"))

            def held_image():
                snapshot = recovery_snapshot(declaration, installation=True)
                for path in watched:
                    if not path.exists() and not path.is_symlink():
                        snapshot[str(path)] = ("absent",)
                    else:
                        raw, native = native_image(path, 32 << 20)
                        snapshot[str(path)] = ("file", native, hashlib.sha256(raw).hexdigest())
                return snapshot

            baseline = held_image()
            if recovery_snapshot(declaration) != before or native_image(pending) != (pending_raw, pending_identity):
                raise RuntimeError("pre-recovery semantic/account/credential/Release or pending history changed")
            for path, expected in immutable.items():
                raw, native = native_image(Path(path))
                if (native, hashlib.sha256(raw).hexdigest()) != expected:
                    raise RuntimeError("pre-recovery original cut/failure evidence changed")
            if baseline.get(str(completion)) != ("absent",) or baseline.get(str(payload)) != ("file", (rows[-1]["device"], rows[-1]["inode"], 0o600, 0, 0), hashlib.sha256(b"").hexdigest()):
                raise RuntimeError("independent original empty born inode and absent suffix required")
            selected, environment, original_output = prepare_refusal_observer(command, declaration_path, installation, journal, "concurrent-first-recovery", hold=hold)
            helper, errors = None, []
            try:
                observe()
                helper = RecoveryCancellationObserver([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceRefusalObserverNative$", "-test.v", "-test.timeout=3m"],
                                                       environment, output / "concurrent-first-recovery-observer.log", observe)
                source_cancellation_observation(selected, helper.ready)
                header = {"schema": "ardents-source-recovery-completion-v1", "operation": operation, "journal": str(journal),
                          "device": identity[0], "inode": identity[1], "prefix_sha256": report["selected_journal_sha256"],
                          "prefix_records": recovery_count, "generation": generation, "binding_sha256": boundary["binding_sha256"]}
                expected_header = json.dumps(header, separators=(",", ":"), ensure_ascii=False).encode()[:-1] + b',"records":['
                actual_header, header_identity = native_image(completion)
                body, body_identity = native_image(payload, 64 << 20)
                if (actual_header != expected_header or header_identity[2:] != (0o600, 0, 0)
                        or hashlib.sha256(body).hexdigest() != hold["body_sha256"]
                        or body_identity != (rows[-1]["device"], rows[-1]["inode"], 0o555, 0, 0)):
                    raise RuntimeError("actual preserved partial envelope and repaired original program required")
                preserved = dict(baseline)
                preserved[str(payload)] = ("file", body_identity, hashlib.sha256(body).hexdigest())
                preserved[str(completion)] = ("file", header_identity, hashlib.sha256(actual_header).hexdigest())
                if held_image() != preserved:
                    raise RuntimeError("held recovery changed history beyond exact original program repair and new empty envelope")
                release_lock = Path(history) / ".ardents-release-decision-lock"
                _, release_identity = native_image(release_lock)
                second, _ = before_return_refusal(command, declaration_path, installation, journal, "concurrent-second-refusal", external_journal=True, external_release=True,
                                                  exclusion_identity=release_identity[:2])
                if second["root_pid"] == helper.ready["root_pid"] or helper.child.poll() is not None or held_image() != preserved:
                    raise RuntimeError("second original caller did not preserve the still-held recovery")
                helper.cancel()  # Only after the actual second exclusion and join.
                completed = helper.finish()
                first_raw, _ = native_image(original_output, 1 << 20)
                first = source_cancellation_observation(selected, helper.ready, completed, first_raw)
                if held_image() != preserved:
                    raise RuntimeError("canceled recovery changed retained original/partial evidence")
                later, _ = before_return_refusal(command, declaration_path, installation, journal, "concurrent-later-partial-refusal")
                if held_image() != preserved:
                    raise RuntimeError("later caller appended/adopted the partial recovery")
                outcome["recovery_refusal"] = {"operation": operation, "prefix_records": recovery_count, "fault": "concurrent-cancellation",
                                                "original_failed_exit": report["root_exit"], "original_threads_joined": report["threads_joined"],
                                                "original_failure_retained": True, "first_cancellation": first,
                                                "second_exclusion_refusal": second, "later_partial_refusal": later,
                                                "held_history_preserved": True, "partial_completion_sha256": digest(completion),
                                                "matrix_sha256": digest(paths["RECOVERY_MATRIX"])}
                if operation == "initial":
                    if source_started or source_pidfd is not None or scope_fd is not None or outcome["original_scope_joined"]:
                        raise RuntimeError("never-started initial concurrent recovery acquired a Source scope")
                    outcome["source_scope_not_started"] = True
            except BaseException as error:
                errors.append(error)
            finally:
                if helper is not None:
                    try:
                        helper.close()
                    except BaseException as error:
                        errors.append(error)
            if errors:
                raise BaseExceptionGroup("concurrent original recovery/cancellation/refusal failed", errors)
            raise RecoveryRefusalComplete()

        if recovery_fault is not None:
            # One terminal fault per private stand. Keep damaged history and
            # originals; never restore it and count the same stand as positive.
            import fcntl
            lease = None
            retained_root = None
            watched = [journal, control, result, pending, Path(str(journal) + ".recovery-completion")]
            if expiry_original is not None:
                siblings = sorted(installation.parent.glob(installation.name + ".source-*"))
                if len(siblings) > 512:
                    raise RuntimeError("finite original expiry sibling history required")
                watched += [Path(command[3]) / name for name in SOURCE_EXPIRY_FILES]
                watched += [Path("/etc/systemd/system/ardents-source.service"), *siblings]

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
                if expiry_original is not None:
                    state["expiry-root-sibling-names"] = tuple(sorted(path.name for path in installation.parent.glob(installation.name + ".source-*")))
                    state["expiry-original-distribution"] = expiry_artifact_image(command[3])
                for path in watched:
                    if not path.exists() and not path.is_symlink():
                        state[str(path)] = ("absent",)
                    elif path.is_dir() and not path.is_symlink():
                        state[str(path)] = ("directory", native_identity(path.lstat()), tuple(sorted(child.name for child in path.iterdir())))
                    else:
                        body, native = native_image(path, 64 << 20)
                        state[str(path)] = ("file", native, hashlib.sha256(body).hexdigest())
                return state

            fault_failures = []
            try:
                if recovery_fault == "timestamp-expiry":
                    prediction = source_release_expiry_prediction(recovery_matrix, operation, recovery_count)
                    undamaged = preserved_refusal_state()
                    before_report, before_proof = release_expiry_observer(command, "before", expiry_original)
                    if preserved_refusal_state() != undamaged:
                        raise RuntimeError("pre-expiry observation changed original recovery history")
                    expiry = datetime.fromisoformat(before_report["timestamp_expires"].replace("Z", "+00:00")).timestamp()
                    while time.time() < expiry:
                        observe()
                        if preserved_refusal_state() != undamaged:
                            raise RuntimeError("original recovery inputs changed while Timestamp elapsed")
                        time.sleep(min(0.2, max(0, expiry-time.time())))
                    expired_report, expired_proof = release_expiry_observer(command, "expired", expiry_original, before_report)
                    if preserved_refusal_state() != undamaged:
                        raise RuntimeError("actual expiry observation changed original recovery history")
                    outcome["release_expiry"] = {"prediction": prediction, "before": before_proof, "expired": expired_proof,
                                                  "same_original_distribution": True, "history_unchanged": True}
                elif recovery_fault == "held-journal-lease":
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
                before_return_refusals = []
                for attempt in range(2):
                    name = f"terminal-recovery-refusal-{attempt}"
                    observed, refused = before_return_refusal(command, declaration_path, installation, journal, name, external_journal=lease is not None,
                                                              exclusion_identity=identity[:2] if lease is not None else None)
                    before_return_refusals.append(observed)
                    code = observed["root_exit"]
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
                                                "before_return_refusals": before_return_refusals,
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

    def retain_invocation(*, archive=None, wait_ready=True):
        nonlocal source_pid, source_pidfd, scope_fd, invocation
        if source_pidfd is not None or scope_fd is not None:
            raise RuntimeError("prior original Source custody must finish before retaining another")
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
        scope = Path("/sys/fs/cgroup/system.slice/ardents-source.service")
        scope_info = scope.lstat()
        scope_fd = os.open(scope / "cgroup.events", os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
        if os.fstat(scope_fd).st_dev != Path("/sys/fs/cgroup").stat().st_dev or original_join():
            raise RuntimeError("original active kernel Source scope unavailable")
        invocation = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "InvocationID", "--value"]).decode().strip()
        if len(invocation) != 32 or any(c not in "0123456789abcdef" for c in invocation):
            raise RuntimeError("actual original invocation identity unavailable")
        if archive is not None:
            signal.pidfd_send_signal(source_pidfd, 0)
            current_process, current_scope = process_path.stat(), scope.lstat()
            named_events, retained_events = (scope / "cgroup.events").stat(), os.fstat(scope_fd)
            if (archive["pid"] != source_pid or archive["invocation_id"] != invocation
                    or (archive["process_device"], archive["process_inode"]) != (process_info.st_dev, process_info.st_ino)
                    or (current_process.st_dev, current_process.st_ino) != (process_info.st_dev, process_info.st_ino)
                    or (archive["cgroup_device"], archive["cgroup_inode"]) != (scope_info.st_dev, scope_info.st_ino)
                    or (current_scope.st_dev, current_scope.st_ino) != (scope_info.st_dev, scope_info.st_ino)
                    or (named_events.st_dev, named_events.st_ino) != (retained_events.st_dev, retained_events.st_ino)
                    or archive["manager"] != manager_identity() or manager_owner() != original_manager or original_join()):
                raise RuntimeError("outside original Source pidfd/proc/cgroup differs from held startup archive")
            facts = native_observer.sample()
            clocks = {"MainPID": ("u", source_pid), "ExecMainPID": ("u", source_pid),
                      "ExecMainStartTimestamp": ("t", archive["started_realtime"]),
                      "ExecMainStartTimestampMonotonic": ("t", archive["started_monotonic"])}
            if facts["Manager"] != archive["manager"] or any(facts["Service"][name] != {"type": signature, "data": value} for name, (signature, value) in clocks.items()):
                raise RuntimeError("outside original manager Source execution differs from startup archive")
        if not wait_ready:
            return
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


    def completed_classification(number):
        # Inspect only this latest complete committed original invocation.
        if source_started or source_pidfd is not None or scope_fd is not None or not outcome["original_scope_joined"]:
            raise RuntimeError("read-only classification requires separately joined original Source")
        installation = Path(request["installation_root"])
        directory = installation / ("source-startup" if number == 0 else f"source-invocation-{number:08d}")
        startup = Path(str(installation) + (".source-first-start-journal" if number == 0 else f".source-invocation-{number:08d}-journal"))
        if digest(active_request) != selected["request_sha256"] or json.loads(active_request.read_text()) != request:
            raise RuntimeError("exact original selected declaration required for completed classification")

        def retained_completed():
            image = recovery_snapshot(request, installation=True)
            siblings = sorted(installation.parent.glob(installation.name + ".source-*"))
            if len(siblings) > 512:
                raise RuntimeError("finite retained original journal inventory required")
            for path in [Path("/etc/systemd/system/ardents-source.service"), *siblings]:
                info = path.lstat()
                leaf, _ = source_retained_leaf(path, native_image)
                image[str(path)] = (leaf, info.st_ctime_ns, info.st_mtime_ns)
            for path in [directory / "source-start-pending", directory / "source-retirement-pending", installation / f"source-invocation-{number + 1:08d}"]:
                if path.exists() or path.is_symlink():
                    raise RuntimeError("completed classification has a blocker or later invocation")
            return image

        retained = retained_completed()
        original_facts = source_completed_manager_facts(native_observer.sample())
        for kind, journal in (("startup", startup), ("retirement", directory / "source-retirement-journal")):
            prediction = source_completed_prediction(recovery_matrix, number, kind)
            raw = run([active_node, "installation", "recover-source", distribution, history, "alpha", network, active_request, journal],
                      f"completed-{number}-{kind}-classification")
            if json.loads(raw) != {"operation": "installation.source", "outcome": "source-recovery-stopped-selected"}:
                raise RuntimeError("completed classification returned a foreign outcome")
            if retained_completed() != retained or manager_owner() != original_manager or source_completed_manager_facts(native_observer.sample()) != original_facts:
                raise RuntimeError("read-only completed classification changed retained history or stopped manager")
            outcome.setdefault("completed_source_classifications", []).append({"attempt": number, "journal_kind": kind, "prediction": prediction,
                                                                              "generation": selected["generation"], "output_sha256": hashlib.sha256(raw).hexdigest(),
                                                                              "original_history_unchanged": True})


    def retirement_cut_and_refuse(number):
        nonlocal source_started, scope_fd, source_pidfd
        window = retirement_physical["window"]
        prediction = source_retirement_prediction(recovery_matrix, number, window)
        installation = Path(request["installation_root"])
        directory = installation / ("source-startup" if number == 0 else f"source-invocation-{number:08d}")
        start_journal = Path(str(installation) + (".source-first-start-journal" if number == 0 else f".source-invocation-{number:08d}-journal"))
        journal = directory / "source-retirement-journal"
        archive_raw, archive_identity = native_image(directory / "start-archived.json", 64 << 10)
        archive = json.loads(archive_raw)
        if (archive_identity[2:] != (0o600, 0, 0) or json.dumps(archive, separators=(",", ":")).encode() + b"\n" != archive_raw
                or source_pidfd is None or scope_fd is None or not source_started):
            raise RuntimeError("original live Source custody required before stop launch")
        signal.pidfd_send_signal(source_pidfd, 0)
        process = (Path("/proc") / str(source_pid)).stat()
        scope = Path("/sys/fs/cgroup/system.slice/ardents-source.service")
        scope_info, named_events, retained_events = scope.lstat(), (scope / "cgroup.events").stat(), os.fstat(scope_fd)
        if (source_pidfd_exited(source_pidfd) or original_join() or manager_owner() != original_manager
                or archive["pid"] != source_pid or archive["invocation_id"] != invocation or archive["manager"] != manager_identity()
                or (archive["process_device"], archive["process_inode"]) != (process.st_dev, process.st_ino)
                or (archive["cgroup_device"], archive["cgroup_inode"]) != (scope_info.st_dev, scope_info.st_ino)
                or (named_events.st_dev, named_events.st_ino) != (retained_events.st_dev, retained_events.st_ino)):
            raise RuntimeError("pre-stop retained Source differs from original archived invocation")
        facts = native_observer.sample()
        clocks = {"MainPID": ("u", source_pid), "ExecMainPID": ("u", source_pid),
                  "ExecMainStartTimestamp": ("t", archive["started_realtime"]),
                  "ExecMainStartTimestampMonotonic": ("t", archive["started_monotonic"])}
        if facts["Manager"] != archive["manager"] or any(facts["Service"][name] != {"type": signature, "data": value} for name, (signature, value) in clocks.items()):
            raise RuntimeError("pre-stop original manager execution clocks differ")
        command = [active_node, "installation", "stop-source", history, binding]
        selected_profile = {"schema": "source-retirement-physical-cut-observer-v1", "program_sha256": digest(active_node),
                            "public_receipt_sha256": outcome["public_receipt_sha256"], "arguments": [str(value) for value in command[1:]],
                            "journal": str(journal), "prefix_records": prediction["prefix_records"],
                            "last_phase": "domain-joined" if window == "native-before-joined" else "retired",
                            "window": window, "attempt": number, "generation": selected["generation"], "binding_sha256": digest(binding),
                            "start_sha256": hashlib.sha256(archive_raw).hexdigest(), "start_journal_sha256": digest(start_journal), "gid": gid}
        profile_path = output / "retirement-physical-profile.json"
        with profile_path.open("xb") as stream:
            stream.write(json.dumps(selected_profile, separators=(",", ":")).encode() + b"\n")
        original_output = output / "retirement-physical-original.log"
        environment = dict(os.environ, ARDENTS_SOURCE_RETIREMENT_PHYSICAL_OBSERVER_MODE="installed-retirement-physical",
                           ARDENTS_SOURCE_CUT_OBSERVER_PROFILE=str(profile_path), ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256=digest(profile_path),
                           ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=str(active_node), ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(installation),
                           ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(original_output))

        def partial_image():
            saved = {}
            entries = sorted(directory.iterdir())
            if len(entries) > 16:
                raise RuntimeError("finite original retirement directory required")
            for path in [start_journal, binding, Path("/etc/systemd/system/ardents-source.service"), *entries]:
                info = path.lstat()
                leaf, _ = source_retained_leaf(path, native_image)
                saved[str(path)] = (leaf, info.st_ctime_ns, info.st_mtime_ns)
            info = directory.lstat()
            saved[str(directory)] = (native_identity(info), info.st_ctime_ns, info.st_mtime_ns)
            return saved

        # The Source handles above belong to the earlier actual start and are
        # live before launching root; no already exited PID is reacquired.
        tracer = RetirementPhysicalCutObserver([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceRetirementPhysicalCutObserverNative$", "-test.v", "-test.timeout=3m"],
                                             environment, output / "retirement-physical-observer.log", observe)
        try:
            source_retirement_cut_observation(selected_profile, tracer.ready)
            if tracer.ready["startup_archive"] != archive:
                raise RuntimeError("held stop follows another archived original Source")
            original, _ = native_image(journal, 128 << 10)
            rows = canonical_rows(original)
            if len(rows) != prediction["prefix_records"] or hashlib.sha256(original).hexdigest() != tracer.ready["selected_journal_sha256"]:
                raise RuntimeError("held retirement prefix differs from independently published prediction")
            if prediction["result"] == "absent":
                path = directory / "source-retirement.json"
                if path.exists() or path.is_symlink():
                    raise RuntimeError("native-before-joined cut supplied an unrecorded result")
            else:
                result_raw, result_identity = native_image(directory / "source-retirement.json", 64 << 10)
                result = json.loads(result_raw)
                if result_identity[2:] != (0o600, 0, 0) or json.dumps(result, separators=(",", ":")).encode() + b"\n" != result_raw or result != tracer.ready["retirement_result"]:
                    raise RuntimeError("outside original retirement result differs from held native image")
            retained_partial = partial_image()
            # Root waits at a real stop boundary while PID1 reaps its original
            # Source. Both retained kernel objects must independently finish.
            while not original_join() or not source_pidfd_exited(source_pidfd):
                observe()
                if manager_owner() != original_manager:
                    raise RuntimeError("original manager changed before retirement cut")
                time.sleep(0.02)
            if manager_owner() != original_manager or partial_image() != retained_partial:
                raise RuntimeError("original retirement image changed through separate Source join")
            outcome["original_scope_joined"] = True
            tracer.cut()
            report = tracer.finish()
            original_raw, _ = native_image(original_output, 1 << 20)
            root_proof = source_retirement_cut_observation(selected_profile, tracer.ready, report, original_raw)
            if partial_image() != retained_partial:
                raise RuntimeError("root retirement cut changed retained original image")
        except BaseException as error:
            try:
                tracer.close()
            except BaseException as cleanup:
                raise BaseExceptionGroup("retirement cut and original tracer join failed", [error, cleanup])
            raise
        else:
            tracer.close()
        run(["/usr/bin/systemctl", "stop", "ardents-source.service"], "retirement-cut-unit-stop")
        if not original_join() or not source_pidfd_exited(source_pidfd) or manager_owner() != original_manager:
            raise RuntimeError("original Source join lost after retirement root cut")
        original_scope, scope_fd = scope_fd, None
        os.close(original_scope)
        original_process, source_pidfd = source_pidfd, None
        os.close(original_process)
        source_started = False
        pending_raw, pending_identity = native_image(directory / "source-retirement-pending", 4096)
        pending_proof = (*pending_identity[:2], hashlib.sha256(pending_raw).hexdigest())
        next_directory = installation / f"source-invocation-{number + 1:08d}"

        def retained_image():
            image = recovery_snapshot(request, installation=True)
            image["retirement-physical"] = partial_image()
            for path in [next_directory, Path(str(journal) + ".recovery-completion")]:
                if path.exists() or path.is_symlink():
                    raise RuntimeError("retirement cut manufactured continuation or later invocation")
            return image

        retained = retained_image()
        refused = []
        # The refusal transport takes its distribution from this canonical
        # command shape; stop-source's argv does not contain a distribution.
        refusal_command = [active_node, "installation", "restart-source", distribution, history, "alpha", network, binding]
        for attempt in range(2):
            proof, _ = before_return_refusal(refusal_command, paths["REQUEST"], installation, journal, f"retirement-recovery-refusal-{attempt}",
                                           retirement_pending={"attempt": number, "gid": gid}, retirement_pending_identity=pending_proof)
            refused.append(proof)
            if retained_image() != retained:
                raise RuntimeError("refused retirement recovery changed original histories")
        for operation in ("start-source", "restart-source"):
            code, _ = run([active_node, "installation", operation, distribution, history, "alpha", network, binding], "retirement-cut-refused-" + operation, refusal=True)
            if code == 0 or retained_image() != retained:
                raise RuntimeError("ordinary startup admitted or changed interrupted retirement")
        outcome["recovery_refusal"] = {"operation": "retirement", "attempt": number, "window": window, "prediction": prediction,
                                       "root_cut": root_proof, "separate_source_pidfd_exit_and_scope_join_before_root_cut": True,
                                       "original_retirement_sha256": hashlib.sha256(original).hexdigest(), "refusals": refused}
        raise RecoveryRefusalComplete()


    def startup_cut_and_refuse(number):
        nonlocal source_started, scope_fd, source_pidfd
        window = startup_physical["window"]
        prediction = source_startup_prediction(recovery_matrix, number, window)
        installation = Path(request["installation_root"])
        directory = installation / ("source-startup" if number == 0 else f"source-invocation-{number:08d}")
        journal = Path(str(installation) + (".source-first-start-journal" if number == 0 else f".source-invocation-{number:08d}-journal"))
        command = [active_node, "installation", "start-source" if number == 0 else "restart-source", distribution, history, "alpha", network, binding]
        selected_profile = {"schema": "source-startup-physical-cut-observer-v1", "program_sha256": digest(active_node),
                            "public_receipt_sha256": outcome["public_receipt_sha256"], "arguments": [str(value) for value in command[1:]],
                            "journal": str(journal), "prefix_records": prediction["prefix_records"],
                            "last_phase": "file-ready" if window == "archive-before-archived" else "ack-attempt",
                            "window": window, "attempt": number, "generation": selected["generation"], "binding_sha256": digest(binding), "gid": gid}
        profile_path = output / "startup-physical-profile.json"
        with profile_path.open("xb") as stream:
            stream.write(json.dumps(selected_profile, separators=(",", ":")).encode() + b"\n")
        original_output = output / "startup-physical-original.log"
        environment = dict(os.environ, ARDENTS_SOURCE_STARTUP_PHYSICAL_OBSERVER_MODE="installed-startup-physical",
                           ARDENTS_SOURCE_CUT_OBSERVER_PROFILE=str(profile_path), ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256=digest(profile_path),
                           ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=str(active_node), ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(installation),
                           ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(original_output))
        source_started = True  # Own a partial PID1 invocation before launch.
        tracer = StartupPhysicalCutObserver([paths["RECOVERY_OBSERVER"], "-test.run=^TestSourceStartupPhysicalCutObserverNative$", "-test.v", "-test.timeout=3m"],
                                          environment, output / "startup-physical-observer.log", observe)
        try:
            source_startup_cut_observation(selected_profile, tracer.ready)
            archive_raw, archive_identity = native_image(directory / "start-archived.json", 64 << 10)
            archive = json.loads(archive_raw)
            if (archive != tracer.ready["startup_archive"] or archive_identity[2:] != (0o600, 0, 0)
                    or json.dumps(archive, separators=(",", ":")).encode() + b"\n" != archive_raw):
                raise RuntimeError("outside original startup archive differs from held native observation")
            # Do not wait for listening: archive-before-archived has no ACK yet.
            retain_invocation(archive=archive, wait_ready=False)

            def partial_image():
                saved = {}
                entries = sorted(directory.iterdir())
                if len(entries) > 8:
                    raise RuntimeError("finite original startup directory required")
                for path in [journal, binding, Path("/etc/systemd/system/ardents-source.service"), *entries]:
                    info = path.lstat()
                    leaf, _ = source_retained_leaf(path, native_image)
                    saved[str(path)] = (leaf, info.st_ctime_ns, info.st_mtime_ns)
                info = directory.lstat()
                saved[str(directory)] = (native_identity(info), info.st_ctime_ns, info.st_mtime_ns)
                return saved

            original, _ = native_image(journal, 128 << 10)
            rows = canonical_rows(original)
            if len(rows) != prediction["prefix_records"] or hashlib.sha256(original).hexdigest() != tracer.ready["selected_journal_sha256"]:
                raise RuntimeError("held startup prefix differs from independently published prediction")
            retained_partial = partial_image()
            # Recheck from the original kernel handles immediately before cut.
            signal.pidfd_send_signal(source_pidfd, 0)
            if source_pidfd_exited(source_pidfd) or original_join() or manager_owner() != original_manager:
                raise RuntimeError("original Source ended or manager changed before startup cut")
            tracer.cut()
            report = tracer.finish()
            original_raw, _ = native_image(original_output, 1 << 20)
            root_proof = source_startup_cut_observation(selected_profile, tracer.ready, report, original_raw)
            if partial_image() != retained_partial:
                raise RuntimeError("root startup cut changed retained original image")
        except BaseException as error:
            try:
                tracer.close()
            except BaseException as cleanup:
                raise BaseExceptionGroup("startup cut and original tracer join failed", [error, cleanup])
            raise
        else:
            tracer.close()
        # The tracer cannot join a PID1 child. Retain both originals until the
        # Source pidfd reports exit AND its original kernel scope is empty.
        try:
            signal.pidfd_send_signal(source_pidfd, signal.SIGTERM)
        except ProcessLookupError:
            pass
        while not original_join() or not source_pidfd_exited(source_pidfd):
            observe()
            time.sleep(0.02)
        outcome["original_scope_joined"] = True
        if manager_owner() != original_manager or partial_image() != retained_partial:
            raise RuntimeError("original manager/startup image changed through separate Source join")
        run(["/usr/bin/systemctl", "stop", "ardents-source.service"], "startup-cut-unit-stop")
        original_scope, scope_fd = scope_fd, None
        os.close(original_scope)
        original_process, source_pidfd = source_pidfd, None
        os.close(original_process)
        source_started = False
        pending_raw, pending_identity = native_image(directory / "source-start-pending", 4096)
        pending_proof = (*pending_identity[:2], hashlib.sha256(pending_raw).hexdigest())
        absent = [directory / "source-retirement-journal", directory / "source-retirement.json", directory / "source-retirement-pending",
                  Path(str(journal) + ".recovery-completion")]
        next_directory = installation / f"source-invocation-{number + 1:08d}"

        def retained_image():
            image = recovery_snapshot(request, installation=True)
            image["startup-physical"] = partial_image()
            for path in [*absent, next_directory]:
                if path.exists() or path.is_symlink():
                    raise RuntimeError("startup cut manufactured retirement, continuation or later invocation")
            return image

        retained = retained_image()
        refused = []
        for attempt in range(2):
            proof, _ = before_return_refusal(command, paths["REQUEST"], installation, journal, f"startup-recovery-refusal-{attempt}",
                                           startup_pending={"attempt": number, "gid": gid}, startup_pending_identity=pending_proof)
            refused.append(proof)
            if retained_image() != retained:
                raise RuntimeError("refused startup recovery changed original histories")
        for operation in ("start-source", "restart-source"):
            code, _ = run([active_node, "installation", operation, distribution, history, "alpha", network, binding], "startup-cut-refused-" + operation, refusal=True)
            if code == 0 or retained_image() != retained:
                raise RuntimeError("ordinary startup admitted or changed interrupted original")
        outcome["recovery_refusal"] = {"operation": "startup", "attempt": number, "window": window, "prediction": prediction,
                                       "root_cut": root_proof, "separate_source_pidfd_exit_and_scope_join": True,
                                       "original_startup_sha256": hashlib.sha256(original).hexdigest(), "refusals": refused}
        raise RecoveryRefusalComplete()

    try:
        if recovery_profile:
            observer_names = ("TestSourceCutObserverNative", "TestSourceRefusalObserverNative", "TestSourceCanonicalPhysicalCutObserverNative",
                              "TestSourceRecoveryPhysicalCutObserverNative", "TestSourceStartupPhysicalCutObserverNative", "TestSourceRetirementPhysicalCutObserverNative", "TestSourceReleaseExpiryObserverNative")
            discovered = run([paths["RECOVERY_OBSERVER"], "-test.list=^(" + "|".join(observer_names) + ")$"], "recovery-observer-discovery")
            if sorted(discovered.splitlines()) != sorted(name.encode("ascii") for name in observer_names):
                raise RuntimeError("same-source native recovery observer is absent")
            native_pattern = "^(" + "|".join(RECOVERY_NATIVE_TESTS) + ")$"
            discovered = run([paths["RECOVERY_OBSERVER"], "-test.list=" + native_pattern], "recovery-native-discovery")
            if sorted(discovered.splitlines()) != sorted(name.encode("ascii") for name in RECOVERY_NATIVE_TESTS):
                raise RuntimeError("required native recovery ownership/process control is absent")
            # The same pinned native binary supplies the file/process controls;
            # it obtains no installed or semantic authority from their result.
            native_root = output / "native-component"
            native_root.mkdir(mode=0o700)
            native_environment = dict(os.environ, ARDENTS_SOURCE_STOPPED_FUTURE_PARENT=str(native_root),
                                      ARDENTS_SOURCE_RECOVERY_CONCURRENT_PROGRAM=str(paths["RECOVERY_OBSERVER"]),
                                      ARDENTS_SOURCE_CUT_CONTROL_PROGRAM=str(paths["RECOVERY_OBSERVER"]),
                                      ARDENTS_SOURCE_CUT_CONTROL_RUNNER=str(paths["CUT_CONTROL_RUNNER"]),
                                      ARDENTS_SOURCE_CUT_CONTROL_RUNNER_SHA256=digest(paths["CUT_CONTROL_RUNNER"]))
            run([paths["RECOVERY_OBSERVER"], "-test.run=" + native_pattern, "-test.v", "-test.timeout=90s"],
                # File-only controls observe exact public resource access. Set
                # their child mask without changing this runner's private mask.
                "recovery-native-controls", env=native_environment, umask=0o022)
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
        if startup_physical is not None and startup_physical["attempt"] == 0:
            startup_cut_and_refuse(0)
        raw = run([paths["NODE"], "installation", "start-source", distribution, history, "alpha", network, binding], "first-start")
        if json.loads(raw) != {"operation": "installation.source", "outcome": "first-start-completed"}:
            raise RuntimeError("foreign first-start completion projection")
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
                if retirement_physical is not None and retirement_physical["attempt"] == number:
                    retirement_cut_and_refuse(number)
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
                if positive_recovery:
                    completed_classification(number)
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
                nonlocal selected_document, selected, plan, request, active_request, distribution, active_node, native_observer, selected_floors
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
                active_request = paths["REQUEST_" + suffix]
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
                if startup_physical is not None and number + 1 == startup_physical["attempt"]:
                    startup_cut_and_refuse(number + 1)
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
        if scope_fd is not None or source_pidfd is not None:
            notice, join_error = False, None
            while True:
                try:
                    # Keep any partially acquired originals too. An empty
                    # scope cannot replace the retained process's exit event.
                    if (scope_fd is None or original_join()) and (source_pidfd is None or source_pidfd_exited(source_pidfd)):
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
            if scope_fd is not None:
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
        if positive_recovery:
            if not outcome.get("recovery") or not recovery_evidence:
                raise RuntimeError("selected positive recovery was not executed")
            recovered_transcript()
            classified = outcome.get("completed_source_classifications", [])
            expected = [(number, kind) for number in range(4 if replacement_profile else 3) for kind in ("startup", "retirement")]
            if [(item.get("attempt"), item.get("journal_kind")) for item in classified] != expected:
                raise RuntimeError("complete read-only startup/retirement classification sequence missing")
        if recovery_fault is not None or recovery_concurrent or recovery_physical is not None or recovery_root_physical is not None or startup_physical is not None or retirement_physical is not None:
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
