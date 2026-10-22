#!/usr/bin/env python3
"""Build the fixed Source command inventory twice from an exact source snapshot.

The outside controller holds the development lock, supplies the immutable source
manifest/toolchain/module-cache environment and joins the whole build scope.
This tool signs nothing and starts no installed process. Both executions remain
project-controlled; their different names never claim independent builders.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys


COMMANDS = ("ardents", "ardents-node", "ardents-control", "ardents-custody", "ardents-text")
FLAGS = ["-trimpath", "-buildvcs=false", "-ldflags=-w"]


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def file_digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def direct(path, directory=False):
    if not path.is_absolute() or path != Path(os.path.normpath(path)):
        raise RuntimeError("canonical absolute input required")
    for item in (path, *path.parents):
        if item.is_symlink():
            raise RuntimeError("symlink input refused")
    if not (path.is_dir() if directory else path.is_file()):
        raise RuntimeError("original input unavailable")
    return path


def write_new(path, raw):
    with path.open("xb") as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def check_source(root, manifest):
    files = manifest.get("files")
    names = set()
    for path in root.rglob("*"):
        if path.is_symlink():
            raise RuntimeError("symlink in source snapshot refused")
        if path.is_file():
            names.add(path.relative_to(root).as_posix())
    if not isinstance(files, dict) or not files or names != set(files):
        raise RuntimeError("complete exact source inventory required")
    for name, wanted in files.items():
        path = root / name
        if Path(name).is_absolute() or ".." in Path(name).parts or not re.fullmatch("[0-9a-f]{64}", wanted):
            raise RuntimeError("invalid source commitment")
        direct(path)
        if file_digest(path) != wanted:
            raise RuntimeError("source bytes changed")


def command(args, root, env):
    # A timeout is not a joined build scope. The outside original controller
    # owns interruption and complete descendant join, including tool failure.
    return subprocess.check_output(args, cwd=root, env=env)


def compiled_dependencies(raw):
    decoder, remaining, found, packages = json.JSONDecoder(), raw.decode(), {}, set()
    while remaining.strip():
        item, end = decoder.raw_decode(remaining.lstrip())
        remaining = remaining.lstrip()[end:]
        if item.get("Error") or item.get("DepsErrors") or not item.get("ImportPath"):
            raise RuntimeError("unavailable compiled package refused")
        packages.add(item["ImportPath"])
        module = item.get("Module")
        if module is None:
            if not item.get("Standard"):
                raise RuntimeError("nonstandard package has no module provenance")
            continue
        if module.get("Replace") or module.get("Error"):
            raise RuntimeError("replaced or unavailable compiled module refused")
        record = {key: module[key] for key in ("Path", "Version", "Sum", "GoModSum", "Main") if key in module}
        if record["Path"] in found and found[record["Path"]] != record:
            raise RuntimeError("conflicting compiled module provenance")
        found[record["Path"]] = record
    if not found or not packages:
        raise RuntimeError("empty dependency inventory")
    return sorted(found.values(), key=lambda item: item["Path"]), sorted(packages)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-manifest", required=True, type=Path)
    parser.add_argument("--source-manifest-sha256", required=True)
    parser.add_argument("--source-revision", required=True)
    parser.add_argument("--output", required=True, type=Path)
    p = parser.parse_args()
    if sys.platform != "linux" or not sys.flags.isolated or not sys.flags.no_site:
        raise RuntimeError("isolated Linux Python with -I -S required")
    if not re.fullmatch("[0-9a-f]{64}", p.source_manifest_sha256) or not (re.fullmatch("[0-9a-f]{40}", p.source_revision) or p.source_revision == "snapshot-sha256:" + p.source_manifest_sha256):
        raise RuntimeError("exact source revision and manifest pin required")
    root = Path(__file__).resolve().parents[2]
    if not os.statvfs(root).f_flag & os.ST_RDONLY:
        raise RuntimeError("original read-only source snapshot required")
    original_manifest = direct(p.source_manifest).read_bytes()
    if digest(original_manifest) != p.source_manifest_sha256:
        raise RuntimeError("source manifest differs from independent pin")
    manifest = json.loads(original_manifest)
    check_source(root, manifest)
    parent = direct(p.output.parent, True)
    if not p.output.is_absolute() or p.output.name in ("", ".", "..") or p.output.is_relative_to(root):
        raise RuntimeError("fresh output outside source required")
    if parent.stat().st_uid != os.geteuid() or parent.stat().st_mode & 0o077:
        raise RuntimeError("original private owned output parent required")
    os.umask(0o077)
    p.output.mkdir(mode=0o700)  # No existing directory, reset or prefix adoption.
    env = os.environ.copy()
    env.update(GOENV="off", GOTOOLCHAIN="local", GOFLAGS="-mod=readonly", CGO_ENABLED="0", GOTELEMETRY="off", GOPROXY="off", GOSUMDB="off", GOOS="linux", GOARCH="amd64", GOAMD64="v1", GOEXPERIMENT="", GOWORK="off")
    if command(["go", "version"], root, env) != b"go version go1.27.3 linux/amd64\n":
        raise RuntimeError("selected Go1.27.3 Linux-amd64 toolchain required")
    command(["go", "mod", "verify"], root, env)
    used_modules, used_packages = compiled_dependencies(command(["go", "list", "-deps", "-json", *["./cmd/" + name for name in COMMANDS]], root, env))
    dependency = canonical(used_modules)
    write_new(p.output / "modules.json", dependency)
    sbom = canonical({"schema": "ardents-source-build-sbom-v1", "source_revision": p.source_revision,
                      "toolchain": "go1.27.3 linux/amd64", "commands": list(COMMANDS), "modules": used_modules, "package_imports": used_packages})
    write_new(p.output / "sbom.json", sbom)
    recipe = {"source_manifest_sha256": p.source_manifest_sha256, "toolchain": "go1.27.3 linux/amd64",
              "python_version": platform.python_version(), "python_executable_sha256": file_digest(Path(sys.executable).resolve()),
              "flags": FLAGS, "cgo_enabled": False, "module_mode": "readonly", "modules_sha256": digest(dependency),
              "go_mod_sha256": file_digest(root / "go.mod"), "go_sum_sha256": file_digest(root / "go.sum")}
    build_input = digest(canonical(recipe))
    write_new(p.output / "build-inputs.json", canonical(recipe))
    receipts = []
    for number in (1, 2):
        check_source(root, manifest)
        target = p.output / ("build-" + str(number))
        target.mkdir(mode=0o700)
        for name in COMMANDS:
            command(["go", "build", *FLAGS, "-o", str(target / name), "./cmd/" + name], root, env)
        inventory = {name: file_digest(direct(target / name)) for name in COMMANDS}
        check_source(root, manifest)
        receipt = {"schema": "ardents-source-build-execution-v1", "source_revision": p.source_revision,
                   "source_manifest_sha256": p.source_manifest_sha256, "build_input_commitment": build_input,
                   "build_identity": build_input, "dependency_identity": digest(dependency), "sbom_identity": digest(sbom),
                   "builder_identity": "project-controlled-execution-" + str(number), "independent_builders": False,
                   "files": inventory}
        write_new(p.output / ("build-" + str(number) + ".json"), canonical(receipt))
        receipts.append(receipt)
    if receipts[0]["files"] != receipts[1]["files"]:
        raise RuntimeError("the two actual build outputs differ")
    command(["go", "mod", "verify"], root, env)
    check_source(root, manifest)
    if direct(p.source_manifest).read_bytes() != original_manifest:
        raise RuntimeError("original source manifest changed")
    for receipt, number in zip(receipts, (1, 2)):
        for name, wanted in receipt["files"].items():
            if file_digest(direct(p.output / ("build-" + str(number)) / name)) != wanted:
                raise RuntimeError("original command output changed")
    write_new(p.output / "READY", canonical({"matching_builds": True, "independent_builders": False,
                                             "installed_qualification": False, "source_manifest_sha256": p.source_manifest_sha256}))
    print(json.dumps({"operation": "source.build-candidate", "matching_builds": True, "independent_builders": False,
                      "installed_qualification": False, "output": str(p.output)}, separators=(",", ":")))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("Source candidate build failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
