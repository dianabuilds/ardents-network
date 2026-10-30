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
import re
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

TRANSITION_SCHEMAS = ("ardents-forwarding-bootstrap-receipt-v2",
                      "ardents-forwarding-bootstrap-receipt-v3")

V1_CASES = CASES.copy()
CASES = {**V1_CASES,
         "replacement-lifetime-refill": "TestForwardingInstalledNativeReplacementLifetimeAndRefill",
         "replacement-unknown-domain-join": "TestForwardingInstalledNativeReplacementUnknownDomainJoin",
         "replacement-changed-account": "TestForwardingInstalledNativeReplacementChangedAccount",
         "replacement-stale-authority": "TestForwardingInstalledNativeReplacementStaleAuthority"}
for role in ("issuer", "descriptor", "introduction", "join"):
    for kind in ("service", "socket"):
        CASES[f"replacement-competing-{role}-{kind}"] = "TestForwardingInstalledNativeReplacementCompetingSlot"
for prefix, count, test in [
    ("initial", 120, "TestForwardingInstalledNativeOriginalInitialPrefix"),
    ("replacement", 62, "TestForwardingInstalledNativeOriginalReplacementPrefix"),
    ("start-initial", 26, "TestForwardingInstalledNativeOriginalStartPrefix"),
    ("start-selected", 26, "TestForwardingInstalledNativeOriginalStartPrefix"),
    ("restart", 26, "TestForwardingInstalledNativeOriginalStartPrefix"),
    ("retirement-initial", 18, "TestForwardingInstalledNativeOriginalRetirementPrefix"),
    ("retirement-selected", 18, "TestForwardingInstalledNativeOriginalRetirementPrefix"),
]:
    for number in range(1, count + 1):
        CASES[f"{prefix}-cut-{number:03d}"] = test


def input_names(case):
    if case.startswith("initial-cut-"):
        return {"initial_cut_inputs"}
    if (case in ("replacement-lifetime-refill", "replacement-unknown-domain-join", "replacement-changed-account", "replacement-stale-authority") or case.startswith((
            "replacement-competing-", "replacement-cut-", "start-selected-cut-", "restart-cut-", "retirement-selected-cut-"))):
        return INPUTS | {"successors"}
    return INPUTS


def profile_cases(receipt):
    if not isinstance(receipt, dict):
        return {}
    if receipt.get("schema") == "ardents-forwarding-bootstrap-receipt-v1":
        return V1_CASES
    if receipt.get("schema") in TRANSITION_SCHEMAS:
        return CASES
    return {}


def hash_string(value):
    return isinstance(value, str) and len(value) == 64 and all(c in "0123456789abcdef" for c in value)


def campaign_digest(receipt):
    """Commit the whole implementation/catalog, independently of fresh cases.

    This is a grouping commitment, never evidence that another case has prepared
    inputs, executed successfully, or received the separate bootstrap decision.
    """
    scope = {name: receipt.get(name) for name in (
        "source_manifest_sha256", "runtime_source_manifests", "artifacts", "cases", "controllers")}
    return hashlib.sha256(json.dumps(scope, sort_keys=True, separators=(",", ":"),
                                     ensure_ascii=True).encode("ascii")).hexdigest()


def serial_scope(receipt, pins):
    """One actual original stand, with no placeholders for future case inputs."""
    case = receipt.get("selected_case")
    stand = receipt.get("native_stand")
    controllers = receipt.get("controllers")
    return (isinstance(case, str) and case in CASES and pins.get("selected_case") == case
            and isinstance(stand, dict) and set(stand) == {"case", "reservation_sha256"}
            and isinstance(stand["case"], str)
            and re.fullmatch(r"installed[1-9][0-9]*", stand["case"], flags=re.ASCII) is not None
            and hash_string(stand["reservation_sha256"]) and pins.get("native_stand") == stand
            and isinstance(controllers, dict)
            and set(controllers) == {"controller", "worker", "input_preparer"}
            and all(hash_string(value) for value in controllers.values())
            and pins.get("controllers") == controllers
            and all(isinstance(name, str) and all(32 <= ord(char) <= 126 for char in name)
                    for name in receipt.get("artifacts", {}))
            and receipt.get("campaign_sha256") == campaign_digest(receipt)
            and pins.get("campaign_sha256") == receipt.get("campaign_sha256"))


def matches_scope(receipt, pins, source_digest):
    """Check public consistency only; no value here supplies PO acceptance."""
    if not isinstance(receipt, dict) or not isinstance(pins, dict):
        return False
    artifacts = pins.get("artifacts", {})
    if not isinstance(artifacts, dict):
        return False
    required = {"ardents-node", "ardents-control", "forwarding-installed.test"}
    selected = profile_cases(receipt)
    if receipt.get("schema") in TRANSITION_SCHEMAS:
        required |= {"ardents-node-successor", "ardents-control-successor", "forwarding-control.test"}
        sources = pins.get("runtime_source_manifests", {})
        if (not isinstance(sources, dict) or set(sources) != {"initial", "successor"}
                or not all(hash_string(value) for value in sources.values())
                or sources["initial"] == sources["successor"]
                or receipt.get("runtime_source_manifests") != sources
                or pins.get("source_manifest") != source_digest
                or artifacts.get("ardents-node") == artifacts.get("ardents-node-successor")):
            return False
    cases = receipt.get("case_inputs", {})
    if receipt.get("schema") == "ardents-forwarding-bootstrap-receipt-v3":
        if not serial_scope(receipt, pins):
            return False
        prepared = {receipt["selected_case"]}
    else:
        prepared = set(selected)
    return (bool(selected) and hash_string(source_digest)
            and receipt.get("source_manifest_sha256") == source_digest
            and isinstance(artifacts, dict) and required <= set(artifacts)
            and all(hash_string(value) for value in artifacts.values())
            and receipt.get("artifacts") == artifacts
            and receipt.get("cases") == list(selected)
            and isinstance(cases, dict) and set(cases) == prepared
            and all(isinstance(inputs, dict) and set(inputs) == input_names(case)
                    and all(hash_string(value) for value in inputs.values())
                    for case, inputs in cases.items()))


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def matches_inputs(receipt, case, actual):
    """Bind this stand's observed input bytes, without granting runtime rights."""
    if not isinstance(receipt, dict) or not isinstance(case, str):
        return False
    if (receipt.get("schema") == "ardents-forwarding-bootstrap-receipt-v3"
            and receipt.get("selected_case") != case):
        return False
    return (case in profile_cases(receipt) and isinstance(actual, dict) and set(actual) == input_names(case)
            and receipt.get("case_inputs", {}).get(case) == actual)


def prepared_inputs():
    base = Path("/var/lib/forwarding-coherent-initial")
    case = os.environ.get("ARDENTS_FORWARDING_INSTALLED_CASE", "")
    if case.startswith("initial-cut-"):
        paths = {"initial_cut_inputs": base / "initial-cut-inputs.json"}
    else:
        # The initial sealed selection remains immutable across replacement.
        # A mutable current selection is an output, not a prepared-input pin.
        descriptor = base / "delivery/distribution/infrastructure-forwarding.json"
        if not descriptor.is_file() or any(item.is_symlink() for item in (descriptor, *descriptor.parents)):
            raise RuntimeError("original initial public descriptor required")
        generation = digest(descriptor)
        sealed = base / "runtime/node/generations" / generation
        paths = {
            "binding": sealed / "binding.json", "plan": sealed / "forwarding-plan.json",
            "bundle": base / "bundle.bin", "profile": base / "profile.bin",
            "presentations": base / "delivery/forwarding-presentations.json",
            "peer_plan": base / "component-peer/forwarding-plan.json",
            "peer_presentations": base / "delivery/forwarding-peer-presentations.json",
            "peer_certificate": base / "delivery/peer-certificate/node-certificate.pem",
        }
        if "successors" in input_names(case):
            paths["successors"] = base / "delivery/successor-deliveries.json"
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
    # A transition campaign cannot begin with only its easy lifecycle test present.
    # Every required original-prefix entry must exist in this exact artifact.
    required = set(profile_cases(json.loads(receipt.read_text())).values())
    expression = "^(" + "|".join(sorted(required)) + ")$"
    discovered = subprocess.check_output([str(binary), "-test.list=" + expression],
                                         text=True, timeout=15).splitlines()
    if len(discovered) != len(required) or set(discovered) != required:
        raise RuntimeError("complete checked campaign implementation is unavailable")
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
