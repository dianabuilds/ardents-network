#!/usr/bin/env python3
"""Install the fixed root-owned qualification inventory from a prepared binary."""
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import tempfile

PREFIX = "ardents-stream-qualification"
ROOT = Path("/usr/lib/ardents/network-stream-worker-root")
MANIFEST = Path("/etc/ardents/network-stream-worker-artifact.json")
RULE = Path("/usr/share/polkit-1/rules.d/50-ardents-stream-qualification.rules")
SOURCE = Path(__file__).resolve().parent
DIRECTORIES = ("dev", "proc", "sys", "run", "tmp", "etc", "root", "usr", "var", "var/tmp")


def rooted(path):
    for item in (path, *path.parents):
        if item.exists() or item.is_symlink():
            stat = item.lstat()
            if item.is_symlink() or stat.st_uid != 0 or stat.st_mode & 0o022:
                raise RuntimeError("installation path is not root-controlled: " + str(item))


def static_binary(path):
    with path.open("rb") as source:
        body = source.read((64 << 20) + 1)
    if len(body) < 64 or len(body) > 64 << 20 or body[:7] != b"\x7fELF\x02\x01\x01":
        raise RuntimeError("worker must be a bounded Linux amd64 ELF binary")
    machine = struct.unpack_from("<H", body, 18)[0]
    offset = struct.unpack_from("<Q", body, 32)[0]
    size, count = struct.unpack_from("<HH", body, 54)
    if machine != 62 or size != 56 or not 0 < count <= 1024 or offset + size * count > len(body):
        raise RuntimeError("worker ELF program headers invalid")
    for index in range(count):
        kind = struct.unpack_from("<I", body, offset + index * size)[0]
        if kind in (2, 3):
            raise RuntimeError("worker must be linked statically (CGO_ENABLED=0)")
    return body


def install_file(path, body, mode):
    rooted(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
    rooted(path.parent)
    descriptor, temporary = tempfile.mkstemp(prefix=".qualification-install-", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as output:
            os.fchmod(output.fileno(), mode)
            output.write(body)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main():
    if os.geteuid() != 0 or len(sys.argv) != 2:
        raise RuntimeError("usage as root: install.py <prepared-static-worker>")
    binary = static_binary(Path(sys.argv[1]))
    inventory = {ROOT / PREFIX: binary, RULE: (SOURCE / RULE.name).read_bytes()}
    for role in ("reader", "publisher"):
        for suffix in ("@.service", ".socket"):
            name = PREFIX + "-" + role + suffix
            inventory[Path("/etc/systemd/system") / name] = (SOURCE / name).read_bytes()
    for path in (*inventory, MANIFEST, ROOT):
        rooted(path)
    if ROOT.exists():
        allowed = {ROOT / entry for entry in (*DIRECTORIES, PREFIX)}
        if any(path not in allowed for path in ROOT.rglob("*")):
            raise RuntimeError("worker root contains unexpected resources")
    # Prevent new socket activation before inspecting currently running workers.
    for role in ("reader", "publisher"):
        unit = PREFIX + "-" + role + ".socket"
        if Path("/etc/systemd/system", unit).exists():
            subprocess.run(["systemctl", "stop", unit], check=True, timeout=15)
    running = subprocess.run(
        ["systemctl", "list-units", PREFIX + "-*@*.service",
         "--state=active,activating,deactivating", "--plain", "--no-legend"],
        check=True, capture_output=True, text=True, timeout=10)
    if running.stdout.strip():
        raise RuntimeError("active qualification workers must finish before installation")
    ROOT.mkdir(parents=True, exist_ok=True, mode=0o555)
    for entry in DIRECTORIES:
        directory = ROOT / entry
        rooted(directory)
        directory.mkdir(parents=True, exist_ok=True, mode=0o555)
        directory.chmod(0o555)
    for path, body in inventory.items():
        install_file(path, body, 0o555 if path.parent == ROOT else 0o644)
    ROOT.chmod(0o555)
    manifest = {"schema": "ardents-network-stream-worker-artifact-v1",
                "files": dict(sorted((str(path), hashlib.sha256(body).hexdigest())
                                    for path, body in inventory.items()))}
    encoded = json.dumps(manifest, separators=(",", ":")).encode()
    install_file(MANIFEST, encoded + b"\n", 0o644)
    subprocess.run(["systemctl", "daemon-reload"], check=True, timeout=15)
    for role in ("reader", "publisher"):
        subprocess.run(["systemctl", "enable", "--now", PREFIX + "-" + role + ".socket"],
                       check=True, timeout=15)
    print(json.dumps({"manifest_sha256": hashlib.sha256(encoded + b"\n").hexdigest(),
                      "files": manifest["files"]}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
