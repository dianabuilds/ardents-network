#!/usr/bin/env python3
"""Pinned initial stopped Source component on a fresh private native manager.

The outside controller holds the one original development lock and joins its
private manager/children. A failing/partial installation is retained as evidence;
the runner never deletes, adopts, resets, starts or enables it.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def pinned(name):
    path = Path(os.environ["ARDENTS_SOURCE_STOPPED_" + name])
    wanted = os.environ["ARDENTS_SOURCE_STOPPED_" + name + "_SHA256"]
    if not path.is_absolute() or path.is_symlink() or not path.is_file() or len(wanted) != 64 or digest(path) != wanted:
        raise RuntimeError("original pinned native input unavailable: " + name)
    return path


def source(root):
    files = json.loads(pinned("SOURCE_MANIFEST").read_text())["files"]
    names = {p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()}
    if not files or names != set(files):
        raise RuntimeError("complete exact source manifest required")
    for name, wanted in files.items():
        path = root / name
        if Path(name).is_absolute() or ".." in Path(name).parts or path.is_symlink() or not path.resolve().is_relative_to(root) or digest(path) != wanted:
            raise RuntimeError("native source differs from build input")


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or sys.argv[1:] != ["-timeout=3m"]:
        raise RuntimeError("isolated Python3.12 and fixed timeout required")
    if sys.platform != "linux" or os.geteuid() != 0:
        raise RuntimeError("native Linux root controller required")
    release = platform.freedesktop_os_release()
    if (release.get("ID"), release.get("VERSION_ID")) != ("ubuntu", "24.04"):
        raise RuntimeError("selected Ubuntu24.04 stand required")
    if Path("/etc/ardents-source-stopped-native-stand").read_text() != "ardents-source-stopped-native-v1\n" or Path("/run/systemd/container").read_text().strip() != "ardents-stopped":
        raise RuntimeError("original private stopped Source manager required")
    for name in ["mnt", "pid", "cgroup", "uts"]:
        a, b = Path("/proc/1/ns/" + name).stat(), Path("/proc/self/ns/" + name).stat()
        if (a.st_dev, a.st_ino) != (b.st_dev, b.st_ino):
            raise RuntimeError("original caller/manager namespaces differ")
    root = Path(__file__).resolve().parents[2]
    source(root)
    command = pinned("COMMAND")
    env = os.environ.copy()
    # The shared fixture helper consumes the same actual canonical command,
    # never an alternate binary or an accepting receipt.
    env["ARDENTS_SOURCE_PREFLIGHT_COMMAND"] = str(command)
    env["ARDENTS_SOURCE_PREFLIGHT_COMMAND_SHA256"] = digest(command)
    env["ARDENTS_SOURCE_PREFLIGHT_FUTURE_PARENT"] = os.environ["ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"]
    case = os.environ.get("ARDENTS_SOURCE_STOPPED_CASE", "compiled")
    command_cases = {"compiled": "TestSourceStoppedNativeCompiledInitialAndRetainedRefusal", "owner": "TestSourceStoppedNativeOriginalOwner", "cut": "TestSourceStoppedNativeCompiledProcessCut", "refusals": "TestSourceStoppedNativeCompiledPreEffectRefusals", "original": "TestSourceStoppedNativeCompiledOriginalInputAndCancellation"}
    if case not in command_cases:
        raise RuntimeError("unknown stopped Source case")
    tests = [
        ("OWNER_TEST", "internal/successor/installation/infrastructure", "^TestSource(AccountDelta|Binding|LoadedPolicy|Request|Template|Input|StoppedNative)", ["TestSourceStoppedNativeAdmissionAfterJournalAndBeforePromotion", "TestSourceStoppedNativeReloadKeepsInputAncestorsOriginal", "TestSourceAccountDeltaPreservesForeignBytesAndLockedIdentity", "TestSourceBindingCanonicalInitialConfiguration", "TestSourceLoadedPolicyRejectsEveryChangedProperty", "TestSourceStoppedNativeBornInodeSubstitutionAndLateClose", "TestSourceStoppedNativeSyncUncertaintyRetainsBytesAndDenial", "TestSourceStoppedNativeActualPartialWriteAndOriginalCancellation", "TestSourceStoppedNativeSelectionOriginalFilesAndLiveClock", "TestSourceStoppedNativeSelectionHeldLastReadAndCloseFailure", "TestSourceStoppedNativeHistoryOriginalDirectoryAndVerifier", "TestSourceStoppedNativeHistoryHeldObservationAndClose"]),
        ("MANAGER_TEST", "internal/successor/installation/systemd", "^TestSource(StoppedNative|ObjectReply)", ["TestSourceObjectReplyRequiresExactOriginalTypedPath", "TestSourceStoppedNativeIdleOriginalManagerAndCopiedSingleReload", "TestSourceStoppedNativeCancellationJoinsHeldOriginalObservation", "TestSourceStoppedNativeIdleCancellationAndActualManagerLoss"]),
        ("COMMAND_TEST", "cmd/ardents-next", "^" + command_cases[case] + "$", [command_cases[case]]),
    ]
    for key, directory, selector, required in tests:
        source(root)
        binary = pinned(key)
        found = subprocess.check_output([str(binary), "-test.list=" + selector], text=True, timeout=15).splitlines()
        if any(name not in found for name in required):
            raise RuntimeError("checked test discovery incomplete: " + key)
        subprocess.run([str(binary), "-test.v", "-test.run=" + selector, "-test.count=1", "-test.timeout=3m"], cwd=root / directory, env=env, check=True)
        pinned(key)
        pinned("COMMAND")
        source(root)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("Source stopped native failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
