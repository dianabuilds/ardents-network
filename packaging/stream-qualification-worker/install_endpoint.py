#!/usr/bin/env python3
"""Install the fixed qualification Endpoint, leaving its explicit run stopped."""
import grp
import hashlib
import json
import os
from pathlib import Path
import pwd
import subprocess
import sys

from install import install_file, rooted, static_binary

SOURCE = Path(__file__).resolve().parent
BINARY = Path("/usr/lib/ardents/qualification/ardents-qualification")
PLAN = Path("/etc/ardents/qualification-plan.json")
UNIT = Path("/etc/systemd/system/ardents-endpoint.service")
MANIFEST = Path("/etc/ardents/qualification-endpoint-artifact.json")
HOME = Path("/var/lib/ardents/endpoint")
MARKER = b"# Managed by Ardents qualification Endpoint installer.\n"


def command(*arguments):
    return subprocess.run(arguments, check=True, capture_output=True, text=True, timeout=15)


def exact_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise RuntimeError("duplicate qualification plan field")
        result[key] = value
    return result


def endpoint_account():
    try:
        account = pwd.getpwnam("ardents-endpoint")
    except KeyError:
        command("useradd", "--system", "--user-group", "--home-dir", str(HOME),
                "--shell", "/usr/sbin/nologin", "ardents-endpoint")
        account = pwd.getpwnam("ardents-endpoint")
    group = grp.getgrnam("ardents-endpoint")
    if account.pw_uid == 0 or account.pw_gid != group.gr_gid or group.gr_gid == 0:
        raise RuntimeError("Endpoint account must have its own unprivileged group")
    return account


def main():
    if os.geteuid() != 0 or len(sys.argv) != 3:
        raise RuntimeError("usage as root: install_endpoint.py <prepared-static-runner> <local-plan.json>")
    binary = static_binary(Path(sys.argv[1]))
    with Path(sys.argv[2]).open("rb") as source:
        plan = source.read((256 << 10) + 1)
    if len(plan) > 256 << 10:
        raise RuntimeError("qualification plan exceeds its bound")
    parsed = json.loads(plan, object_pairs_hook=exact_object)
    if not isinstance(parsed, dict) or not {"Participants"} <= set(parsed) or not set(parsed) <= {"Mode", "Participants"}:
        raise RuntimeError("qualification plan inventory invalid")
    mode = parsed.get("Mode", "stream")
    if mode not in ("stream", "net32-idle"):
        raise RuntimeError("qualification plan mode invalid")
    participants = parsed["Participants"]
    if not isinstance(participants, list) or not 1 <= len(participants) <= 5:
        raise RuntimeError("qualification participant count invalid")
    if mode == "net32-idle" and len(participants) != 1:
        raise RuntimeError("NET-32 idle plan participant count invalid")
    # Full authority and path validation remains with the actual Endpoint.
    # Installation never manufactures permissions, State or a provider period.
    for path in (BINARY, PLAN, UNIT, MANIFEST, HOME.parent):
        rooted(path)
    if UNIT.exists() and not UNIT.read_bytes().startswith(MARKER):
        raise RuntimeError("existing Endpoint unit is not this qualification installation")
    state = command("systemctl", "show", "ardents-endpoint.service", "--property=ActiveState",
                    "--property=DropInPaths", "--property=FragmentPath").stdout
    values = dict(line.split("=", 1) for line in state.splitlines() if "=" in line)
    if values.get("ActiveState") not in ("inactive", "failed") or values.get("DropInPaths"):
        raise RuntimeError("Endpoint must be stopped and have no overriding drop-ins")
    if values.get("FragmentPath") not in ("", str(UNIT)):
        raise RuntimeError("another installed Endpoint owns this service")
    units = command("systemctl", "list-units", "ardents-stream-qualification-*@*.service",
                    "--state=active,activating,deactivating", "--plain", "--no-legend")
    if units.stdout.strip():
        raise RuntimeError("qualification worker cleanup must finish before installation")
    unit = (SOURCE / UNIT.name).read_bytes()
    if not unit.startswith(MARKER):
        raise RuntimeError("qualification Endpoint template identity invalid")
    account = endpoint_account()
    HOME.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
    if not HOME.exists():
        HOME.mkdir(mode=0o700)
        os.chown(HOME, account.pw_uid, account.pw_gid)
    home_stat = HOME.lstat()
    if HOME.is_symlink() or not HOME.is_dir() or home_stat.st_uid != account.pw_uid or home_stat.st_mode & 0o077:
        raise RuntimeError("Endpoint home is not its private directory")
    inventory = {BINARY: binary, PLAN: plan, UNIT: unit}
    for path, body in inventory.items():
        install_file(path, body, 0o555 if path == BINARY else 0o640 if path == PLAN else 0o644)
        if path == PLAN:
            os.chown(path, 0, account.pw_gid)
    manifest = {"schema": "ardents-qualification-endpoint-artifact-v1",
                "files": {str(path): hashlib.sha256(body).hexdigest() for path, body in inventory.items()}}
    install_file(MANIFEST, json.dumps(manifest, sort_keys=True, separators=(",", ":")).encode() + b"\n", 0o644)
    command("systemctl", "daemon-reload")
    print(json.dumps(manifest, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
