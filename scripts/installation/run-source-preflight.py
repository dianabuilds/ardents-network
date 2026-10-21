#!/usr/bin/env python3
"""Run pinned, exact-source native Source preflight tests, never install Source.

The outside controller owns the shared run-development.py exclusive lock and
the finite private Ubuntu manager. Missing prerequisites fail, never skip.
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


def checked_path(name):
    path = Path(os.environ["ARDENTS_SOURCE_PREFLIGHT_" + name])
    expected = os.environ["ARDENTS_SOURCE_PREFLIGHT_" + name + "_SHA256"]
    if not path.is_absolute() or path.is_symlink() or not path.is_file() or len(expected) != 64 or digest(path) != expected:
        raise RuntimeError("original pinned native input unavailable: " + name)
    return path


def source(root):
    manifest = checked_path("SOURCE_MANIFEST")
    files = json.loads(manifest.read_text())["files"]
    names = {p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()}
    if not files or names != set(files):
        raise RuntimeError("source manifest must cover the entire build snapshot")
    for name, wanted in files.items():
        path = root / name
        if Path(name).is_absolute() or ".." in Path(name).parts or path.is_symlink() or not path.resolve().is_relative_to(root) or digest(path) != wanted:
            raise RuntimeError("source differs from native build inputs")


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or sys.argv[1:] != ["-timeout=3m"]:
        raise RuntimeError("reviewed isolated Python3.12 and fixed timeout required")
    if sys.platform != "linux" or os.geteuid() != 0:
        raise RuntimeError("native Linux root controller required")
    release = platform.freedesktop_os_release()
    if (release.get("ID"), release.get("VERSION_ID")) != ("ubuntu", "24.04"):
        raise RuntimeError("selected Ubuntu24.04 stand required")
    if Path("/etc/ardents-source-preflight-native-stand").read_text() != "ardents-source-preflight-native-v1\n" or Path("/run/systemd/container").read_text().strip() != "ardents-preflight":
        raise RuntimeError("original private test manager required")
    root = Path(__file__).resolve().parents[2]
    source(root)
    checked_path("COMMAND")
    tests = [
        ("OWNER_TEST", "internal/successor/installation/infrastructure", "^Test(CanonicalDescriptor|ZeroAndDetached|Coherence|Source(Request|Template|Input|AccountDelta|LoadedPolicy|PreflightNative))", ["TestCanonicalDescriptorIndependentOracle", "TestZeroAndDetachedFactsCannotCreateAuthority", "TestCoherenceIncludesCommonSafetyAndReference", "TestSourceRequestClosedGrammarAndFutureBoundaries", "TestSourceTemplateExactPolicyAndRendering", "TestSourceInputCopiesRetainFailureAndIndependentClose", "TestSourceInputCloseWaitsForOriginalBorrow", "TestSourceAccountDeltaPreservesForeignBytesAndLockedIdentity", "TestSourceLoadedPolicyRejectsEveryChangedProperty", "TestSourcePreflightNativeHeldOriginalInputCancellationAndClose", "TestSourcePreflightNativeInputMutationAndAncestorAccess", "TestSourcePreflightNativeHeldLastInputReobservesOriginalPaths"]),
        ("MANAGER_TEST", "internal/successor/installation/systemd", "^TestSourcePreflightNative", ["TestSourcePreflightNativeOriginalManagerCopiesAndLateUse", "TestSourcePreflightNativeOriginalManagerLossAndCloseFailure", "TestSourcePreflightNativeOriginalManagerIdentityMismatch", "TestSourcePreflightNativeOriginalCallerCancellation", "TestSourcePreflightNativeHeldManagerObservationJoinsBeforeClose", "TestSourcePreflightNativeLateManagerSocketLoss"]),
        ("COMMAND_TEST", "cmd/ardents-next", "^TestSourcePreflightNative", ["TestSourcePreflightNativeCompiledInitialCandidateAndNoEffects", "TestSourcePreflightNativeIsolatedForeignAccountUnitAndAlias"]),
    ]
    for key, directory, selector, required in tests:
        source(root)
        binary = checked_path(key)
        discovered = subprocess.check_output([str(binary), "-test.list=" + selector], cwd=root / directory, text=True, timeout=15).splitlines()
        if any(name not in discovered for name in required):
            raise RuntimeError("checked native test discovery incomplete: " + key)
        # Each process runs synchronously through actual joins. Do not turn an
        # outside observation timeout into completion or run another profile.
        subprocess.run([str(binary), "-test.v", "-test.run=" + selector, "-test.count=1", "-test.timeout=3m"], cwd=root / directory, check=True)
        checked_path(key)
        checked_path("COMMAND")
        source(root)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("Source preflight native failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
