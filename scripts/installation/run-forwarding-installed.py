#!/usr/bin/env python3
"""Run a finite pinned Forwarding installed profile in its original private stand.

The outside controller must obtain the separate direct PO bootstrap decision,
hold the shared development lock and retain its actual manager/descendants until
join, including on failure. A matching public receipt is not an approval token.
This runner never prepares authority, creates a replacement stand or retries.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys

CASES = {
    "lifetime-refill": "TestForwardingInstalledNativeLifetimeAndRefill",
    "retirement-history-missing": "TestForwardingInstalledNativeRetirementHistoryMissing",
    "retirement-history-damaged": "TestForwardingInstalledNativeRetirementHistoryDamaged",
    "unknown-domain-join": "TestForwardingInstalledNativeUnknownDomainJoin",
}

INPUTS = {"binding", "plan", "bundle", "profile", "presentations",
          "peer_plan", "peer_presentations", "peer_certificate"}


def matches_scope(receipt, pins, source_digest):
    """Check public consistency only; no value here supplies PO acceptance."""
    artifacts = pins.get("artifacts", {})
    required = {"ardents-node", "ardents-control", "forwarding-installed.test"}
    cases = receipt.get("case_inputs", {})
    return (receipt.get("schema") == "ardents-forwarding-bootstrap-receipt-v1"
            and len(source_digest) == 64
            and all(c in "0123456789abcdef" for c in source_digest)
            and receipt.get("source_manifest_sha256") == source_digest
            and isinstance(artifacts, dict) and required <= set(artifacts)
            and all(isinstance(value, str) and len(value) == 64
                    and all(c in "0123456789abcdef" for c in value)
                    for value in artifacts.values())
            and receipt.get("artifacts") == artifacts
            and receipt.get("cases") == list(CASES)
            and isinstance(cases, dict) and set(cases) == set(CASES)
            and all(isinstance(inputs, dict) and set(inputs) == INPUTS
                    and all(isinstance(value, str) and len(value) == 64
                            and all(c in "0123456789abcdef" for c in value)
                            for value in inputs.values())
                    for inputs in cases.values()))


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def matches_inputs(receipt, case, actual):
    """Bind this stand's observed input bytes, without granting runtime rights."""
    return (case in CASES and isinstance(actual, dict) and set(actual) == INPUTS
            and receipt.get("case_inputs", {}).get(case) == actual)


def prepared_inputs():
    base = Path("/var/lib/forwarding-coherent-initial")
    binding = base / "runtime/node/binding.json"
    if any(item.is_symlink() for item in (binding, *binding.parents)) or not binding.is_file():
        raise RuntimeError("original prepared binding required")
    generation = json.loads(binding.read_bytes())["generation"]
    if (not isinstance(generation, str) or len(generation) != 64
            or any(c not in "0123456789abcdef" for c in generation)):
        raise RuntimeError("canonical prepared generation required")
    paths = {
        "binding": binding,
        "plan": binding.parent / "generations" / generation / "forwarding-plan.json",
        "bundle": base / "bundle.bin", "profile": base / "profile.bin",
        "presentations": base / "delivery/forwarding-presentations.json",
        "peer_plan": base / "component-peer/forwarding-plan.json",
        "peer_presentations": base / "delivery/forwarding-peer-presentations.json",
        "peer_certificate": base / "delivery/peer-certificate/node-certificate.pem",
    }
    for path in paths.values():
        if not path.is_file() or any(item.is_symlink() for item in (path, *path.parents)):
            raise RuntimeError("original prepared input required")
    return {name: digest(path) for name, path in paths.items()}


def pinned(name):
    prefix = "ARDENTS_FORWARDING_INSTALLED_" + name
    path = Path(os.environ[prefix])
    expected = os.environ[prefix + "_SHA256"]
    if (not path.is_absolute() or path.is_symlink() or not path.is_file()
            or len(expected) != 64 or digest(path) != expected):
        raise RuntimeError("original pinned input differs: " + name)
    return path


def exact_source(root):
    manifest = json.loads(pinned("SOURCE_MANIFEST").read_text())["files"]
    actual = {path.relative_to(root).as_posix()
              for path in root.rglob("*") if path.is_file()}
    if not manifest or actual != set(manifest):
        raise RuntimeError("complete exact source inventory required")
    for name, entry in manifest.items():
        path = root / name
        if (Path(name).is_absolute() or ".." in Path(name).parts or path.is_symlink()
                or not path.resolve().is_relative_to(root)
                or entry["mode"] not in ["100644", "100755"]
                or path.stat().st_mode & 0o777 != int(entry["mode"][-3:], 8)
                or digest(path) != entry["sha256"]):
            raise RuntimeError("source differs from native build input")


def bootstrap(root):
    receipt_path = pinned("BOOTSTRAP")
    if receipt_path != Path("/artifacts/forwarding-bootstrap-receipt.json"):
        raise RuntimeError("fixed reviewed bootstrap location required")
    receipt = json.loads(receipt_path.read_text())
    pins = json.loads(Path("/artifacts/pins.json").read_text())
    if not matches_scope(receipt, pins, digest(pinned("SOURCE_MANIFEST"))):
        raise RuntimeError("completed exact Forwarding bootstrap scope differs")
    if not matches_inputs(receipt, os.environ.get("ARDENTS_FORWARDING_INSTALLED_CASE"),
                          prepared_inputs()):
        raise RuntimeError("actual stand inputs differ from its exact reviewed case")
    exact_source(root)
    return receipt_path


def main():
    if (sys.version_info[:2] != (3, 12) or not sys.flags.isolated
            or not sys.flags.no_site or sys.argv[1:] != ["-timeout=4m"]):
        raise RuntimeError("isolated Python3.12 and fixed four-minute profile required")
    if sys.platform != "linux" or os.geteuid() != 0:
        raise RuntimeError("original Linux root driver required")
    release = platform.freedesktop_os_release()
    if (release.get("ID"), release.get("VERSION_ID")) != ("ubuntu", "24.04"):
        raise RuntimeError("selected Ubuntu24.04 stand required")
    if Path("/etc/forwarding-initial-component-marker").read_text() != "ardents-forwarding-coherent-preparation-v1\n":
        raise RuntimeError("original private Forwarding manager required")
    for kind in ["mnt", "pid", "net", "uts", "ipc", "cgroup"]:
        caller, manager = Path("/proc/self/ns/" + kind).stat(), Path("/proc/1/ns/" + kind).stat()
        if (caller.st_dev, caller.st_ino) != (manager.st_dev, manager.st_ino):
            raise RuntimeError("original driver and manager namespaces differ")
    case = os.environ.get("ARDENTS_FORWARDING_INSTALLED_CASE")
    if case not in CASES:
        raise RuntimeError("explicit finite installed Forwarding case required")
    root = Path(__file__).resolve().parents[2]
    receipt = bootstrap(root)
    binary = pinned("TEST")
    selected = CASES[case]
    names = subprocess.check_output([str(binary), "-test.list=^" + selected + "$"],
                                    text=True, timeout=15).splitlines()
    if names != [selected]:
        raise RuntimeError("checked installed test discovery differs")
    subprocess.run([str(binary), "-test.run=^" + selected + "$", "-test.v",
                    "-test.count=1", "-test.timeout=4m"], cwd=root / "cmd/ardents-next",
                   check=True)
    pinned("TEST")
    if receipt != bootstrap(root):
        raise RuntimeError("original bootstrap location changed")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("Forwarding installed profile failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
