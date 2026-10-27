#!/usr/bin/env python3
"""Prepare a fixed public Forwarding signing plan from actual build records.

This public transformation signs nothing and reads no private key. The Forwarding
signer independently retains and validates the plan/resources/keys; Release alone
authorizes the exact targets. The public bootstrap receipt and direct PO decision
remain separate from all files and observations produced here.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from datetime import datetime, timezone


COMMANDS = ("ardents-node", "ardents-control")


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise RuntimeError("duplicate public field refused")
        result[key] = value
    return result


def check_time_bounds(now, expires, no_new, terminate):
    if (not now < no_new < terminate <= expires
            or (expires - now).total_seconds() > 7 * 86400):
        raise RuntimeError("current finite initial Forwarding purpose bounds required")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("build-root", "descriptor", "template", "qualification-evidence", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    for name in ("network", "expires", "no-new-work-after", "terminate-after", "qualification-evidence-sha256"):
        parser.add_argument("--" + name, required=True)
    p = parser.parse_args()
    if sys.platform != "linux" or os.geteuid() != 0 or not sys.flags.isolated or not sys.flags.no_site:
        raise RuntimeError("isolated Linux root Python with -I -S required")
    observed = {}

    def protected(path, directory=False):
        if not path.is_absolute() or path != Path(os.path.normpath(path)):
            raise RuntimeError("canonical absolute public path required")
        for item in (path, *path.parents):
            stat = item.lstat()
            if item.is_symlink() or stat.st_uid != 0 or stat.st_mode & 0o022:
                raise RuntimeError("original root-controlled public input required")
        if not (path.is_dir() if directory else path.is_file()):
            raise RuntimeError("public input unavailable")
        return path

    def read(path, limit=1 << 20):
        protected(path)
        if path.stat().st_size > limit:
            raise RuntimeError("public input bound exceeded")
        raw = path.read_bytes()
        if len(raw) > limit:
            raise RuntimeError("public input grew beyond bound")
        pin = sha(raw)
        if path in observed and observed[path] != pin:
            raise RuntimeError("original public bytes changed")
        observed[path] = pin
        return raw

    def document(path):
        return json.loads(read(path), object_pairs_hook=unique)

    def exact(value, fields):
        if not isinstance(value, dict) or set(value) != set(fields):
            raise RuntimeError("closed build record required")

    protected(p.build_root, True)
    expected = {"build-1", "build-2", "build-1.json", "build-2.json", "build-inputs.json", "modules.json", "sbom.json", "READY"}
    if {item.name for item in p.build_root.iterdir()} != expected:
        raise RuntimeError("complete original build inventory required")
    ready = document(p.build_root / "READY")
    exact(ready, ("matching_builds", "independent_builders", "installed_qualification", "source_manifest_sha256"))
    if ready["matching_builds"] is not True or ready["independent_builders"] is not False or ready["installed_qualification"] is not False:
        raise RuntimeError("bounded controlled rebuild observation required")
    receipts = [document(p.build_root / ("build-" + str(i) + ".json")) for i in (1, 2)]
    fields = ("schema", "source_revision", "source_manifest_sha256", "build_input_commitment", "build_identity", "dependency_identity", "sbom_identity", "builder_identity", "independent_builders", "files")
    for number, receipt in zip((1, 2), receipts):
        exact(receipt, fields)
        if receipt["schema"] != "ardents-forwarding-build-execution-v1" or receipt["builder_identity"] != "project-controlled-execution-" + str(number) or receipt["independent_builders"] is not False:
            raise RuntimeError("actual separate project-controlled build records required")
        exact(receipt["files"], COMMANDS)
        directory = protected(p.build_root / ("build-" + str(number)), True)
        if {item.name for item in directory.iterdir()} != set(COMMANDS):
            raise RuntimeError("fixed compiled command inventory required")
        for name, wanted in receipt["files"].items():
            if not isinstance(wanted, str) or not re.fullmatch("[0-9a-f]{64}", wanted) or sha(read(directory / name, 64 << 20)) != wanted:
                raise RuntimeError("original compiled command differs from its build record")
    common = {key: receipts[0][key] for key in fields if key != "builder_identity"}
    if common != {key: receipts[1][key] for key in common} or common["source_manifest_sha256"] != ready["source_manifest_sha256"]:
        raise RuntimeError("the actual build records disagree")
    # These are source-bound observations, not a supplied accepting flag. The
    # final bootstrap review additionally verifies their actual execution and
    # committed source identity; preliminary snapshot records stay labelled.
    if sha(read(p.build_root / "build-inputs.json")) != common["build_input_commitment"] or sha(read(p.build_root / "modules.json")) != common["dependency_identity"] or sha(read(p.build_root / "sbom.json", 8 << 20)) != common["sbom_identity"]:
        raise RuntimeError("build input/module/SBOM commitments differ")
    recipe, sbom = document(p.build_root / "build-inputs.json"), document(p.build_root / "sbom.json")
    exact(sbom, ("schema", "source_revision", "toolchain", "commands", "modules", "package_imports"))
    if sbom["schema"] != "ardents-forwarding-build-sbom-v1" or sbom["toolchain"] != "go1.27.3 linux/amd64" or not isinstance(sbom["package_imports"], list) or not sbom["package_imports"]:
        raise RuntimeError("actual Forwarding compiled inventory required")
    if common["build_identity"] != common["build_input_commitment"] or recipe.get("source_manifest_sha256") != common["source_manifest_sha256"] or sbom.get("source_revision") != common["source_revision"] or sbom.get("commands") != list(COMMANDS) or sbom.get("modules") != document(p.build_root / "modules.json"):
        raise RuntimeError("source-bound build and SBOM facts disagree")
    evidence = read(p.qualification_evidence, 256 << 10)
    if not evidence.strip() or not re.fullmatch("[0-9a-f]{64}", p.qualification_evidence_sha256) or sha(evidence) != p.qualification_evidence_sha256:
        raise RuntimeError("separately reviewed qualification evidence commitment required")
    descriptor = document(p.descriptor)
    template = read(p.template, 64 << 10)
    program = p.build_root / "build-1" / "ardents-node"
    version = descriptor.get("release_version")
    exact(descriptor, ("schema", "platform", "release_identity", "release_version", "invocation", "files"))
    if (type(version) is not int or version != 1
            or descriptor["schema"] != "ardents-infrastructure-artifact-v1" or descriptor["platform"] != "linux-amd64"
            or descriptor["invocation"] != "forwarding" or not isinstance(descriptor["release_identity"], str)
            or not descriptor["release_identity"].strip()
            or descriptor["files"] != {"ardents-node-linux-amd64": common["files"]["ardents-node"], "ardents-forwarding.service": sha(template)}):
        raise RuntimeError("Forwarding purpose/resources disagree with actual builds")
    now = datetime.now(timezone.utc).replace(microsecond=0)

    def instant(text):
        value = datetime.fromisoformat(text.replace("Z", "+00:00"))
        if value.tzinfo is None or value.microsecond or value.utcoffset().total_seconds() != 0:
            raise RuntimeError("whole-second UTC public time required")
        return value

    expires, no_new, terminate = map(instant, (p.expires, p.no_new_work_after, p.terminate_after))
    check_time_bounds(now, expires, no_new, terminate)
    if not p.network.strip():
        raise RuntimeError("nonempty Forwarding network required")

    def utc(value):
        return value.isoformat().replace("+00:00", "Z")

    def custom(target_pin):
        result = {key: common[key] for key in ("source_revision", "build_input_commitment", "build_identity", "dependency_identity", "sbom_identity")}
        result.update(schema_version=1, profile="ardents-h3-release-v1", platform="linux-amd64", architecture="amd64", environment="alpha", network=p.network,
                      release_identity=descriptor["release_identity"], release_version=version, attestation_policy="two-builder", qualification="qualified", build_state="current",
                      protocol_phase="announced", build_safety_no_new_work_after=utc(no_new), build_safety_terminate_after=utc(terminate))
        result["builder_attestations"] = [{"builder_identity": receipt["builder_identity"], "build_identity": common["build_identity"],
                                           "source_revision": common["source_revision"], "build_input_commitment": common["build_input_commitment"],
                                           "target_sha256": target_pin} for receipt in receipts]
        return result

    schema = "ardents-forwarding-initial-release-plan-v1"
    plan = {"schema": schema, "network": p.network, "reference_time": utc(now), "expires": utc(expires),
            "program": str(program), "descriptor": str(p.descriptor), "template": str(p.template),
            "program_custom": custom(common["files"]["ardents-node"]), "generation_custom": custom(observed[p.descriptor])}
    # Metadata's existing qualified value is proposed bounded qualification; it
    # is not installed qualification. Its exact evidence/limits belong in the
    # public bootstrap receipt and require direct PO acceptance before execution.
    for path, wanted in observed.items():
        if sha(read(path, 64 << 20)) != wanted:
            raise RuntimeError("original public input changed before handoff")
    parent = protected(p.output.parent, True)
    if parent.stat().st_mode & 0o077 or not p.output.is_absolute() or p.output.is_relative_to(p.build_root):
        raise RuntimeError("separate fresh private output parent required")
    os.umask(0o077)
    p.output.mkdir(mode=0o700)
    raw = canonical(plan)
    with (p.output / "forwarding-release-plan.json").open("xb") as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())
    print(json.dumps({"operation": "forwarding.prepare-release-plan", "plan_sha256": sha(raw), "source_manifest_sha256": common["source_manifest_sha256"],
                      "qualification_evidence_sha256": p.qualification_evidence_sha256, "qualification_scope": "prestart-components-only",
                      "independent_builders": False, "installed_qualification": False}, separators=(",", ":")))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, TypeError, AttributeError) as error:
        print("Forwarding public Release plan failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
