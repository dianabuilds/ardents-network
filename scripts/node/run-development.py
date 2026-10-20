#!/usr/bin/env python3
"""Finite native development runs and a shared, fail-fast own-job lock.

Use exclusive mode for builds/scanners/checks on either Windows or Linux,
pointing both at the SAME external lock parent (WSL uses its /mnt/c spelling).
No stale-lock takeover, retry, global host tuning or foreign process control.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import time


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def external_directory(value):
    path = Path(value)
    if not path.is_absolute() or not path.is_dir() or path.is_symlink():
        raise RuntimeError("existing absolute external directory required")
    root = Path(__file__).resolve().parents[2]
    if path.resolve() == root or root in path.resolve().parents:
        raise RuntimeError("generated evidence/lock must be outside source")
    return path.resolve()


def native_facts(storage):
    if platform.system() != "Linux" or os.geteuid() != 0:
        raise RuntimeError("Linux root driver required")
    release = platform.freedesktop_os_release()
    if (release.get("ID"), release.get("VERSION_ID")) != ("ubuntu", "24.04"):
        raise RuntimeError("declared Ubuntu24.04 development stand unavailable")
    interpreter = Path(sys.executable).resolve().stat()
    facts = {
        "os": {"id": release["ID"], "version": release["VERSION_ID"]},
        "python": {"version": platform.python_version(), "device": interpreter.st_dev, "inode": interpreter.st_ino, "ctime_ns": interpreter.st_ctime_ns},
        "kernel": platform.release(),
        "manager": subprocess.check_output(["systemctl", "show", "-p", "Version", "--value"], text=True, timeout=5).strip(),
        "manager_owner": subprocess.check_output(["busctl", "--system", "call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetNameOwner", "s", "org.freedesktop.systemd1"], text=True, timeout=5).strip(),
        "manager_clock": Path("/proc/1/stat").read_text().rsplit(")", 1)[1].split()[19],
        "storage": subprocess.check_output(["findmnt", "-T", str(storage), "-n", "-o", "FSTYPE,SOURCE,OPTIONS"], text=True, timeout=5).strip(),
        "controllers": Path("/sys/fs/cgroup/cgroup.controllers").read_text().strip(),
    }
    if not facts["manager"].startswith("255.") or not facts["storage"].startswith("ext4 "):
        raise RuntimeError("declared systemd255/native ext4 development stand unavailable")
    if not {"cpu", "memory", "pids"} <= set(facts["controllers"].split()):
        raise RuntimeError("mandatory cgroup-v2 controllers unavailable")
    return facts


def artifacts():
    result = {}
    for key in ["COMMAND", "TEST", "RESOURCE_TEST"]:
        prefix = "ARDENTS_NODE_RESOURCE_TEST" if key == "RESOURCE_TEST" else "ARDENTS_NODE_NATIVE_" + key
        path = Path(os.environ[prefix])
        expected = os.environ[prefix + "_SHA256"]
        if not path.is_absolute() or path.is_symlink() or not os.access(path, os.X_OK) or len(expected) != 64 or digest(path) != expected:
            raise RuntimeError("independently pinned native artifact unavailable: " + key)
        result[key] = expected
    return result


def source_identity(root):
    manifest = Path(os.environ["ARDENTS_NODE_NATIVE_SOURCE_MANIFEST"])
    expected = os.environ["ARDENTS_NODE_NATIVE_SOURCE_SHA256"]
    if not manifest.is_absolute() or digest(manifest) != expected:
        raise RuntimeError("independent source manifest pin differs")
    files = json.loads(manifest.read_text())["files"]
    if not files:
        raise RuntimeError("empty source evidence")
    if (root / ".git").exists():
        names = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], cwd=root).decode().strip("\0").split("\0")
    else:
        names = [path.relative_to(root).as_posix() for path in root.rglob("*") if path.is_file()]
    if set(files) != set(names):
        raise RuntimeError("source manifest does not cover the complete build snapshot")
    for name, wanted in files.items():
        path = root / name
        if Path(name).is_absolute() or ".." in Path(name).parts or path.is_symlink() or not path.resolve().is_relative_to(root) or digest(path) != wanted:
            raise RuntimeError("source differs from artifact input")
    return expected


def active_units():
    rows = json.loads(subprocess.check_output(["systemctl", "list-units", "--all", "--no-legend", "--output=json", "ardents-*"], text=True, timeout=5))
    return sorted(row["unit"] for row in rows if row["active"] not in ["inactive", "failed"])


def baseline(output):
    import resource  # Linux-only measurements; exclusive mode also runs on Windows.
    root = Path(__file__).resolve().parents[2]
    pins = artifacts()
    source = source_identity(root)
    storage = Path(os.environ.get("TMPDIR", "/tmp"))
    original = native_facts(storage)
    if active_units():
        raise RuntimeError("native stand already has active Ardents work; do not touch foreign work")
    # Genuine non-test command consumer checks actual sole-process placement,
    # hard limits, original proc/cgroup/PSI, elapsed CPU and freshness.
    probe = ["systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit=ardents-development-preflight-" + str(os.getpid()),
             "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryHigh=768M", "--property=MemoryMax=1G",
             "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=10s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB",
             "--", os.environ["ARDENTS_NODE_NATIVE_COMMAND"], "node", "inspect-process"]
    with (output / "preflight.log").open("x") as log:
        try:
            subprocess.run(probe, stdout=log, stderr=subprocess.STDOUT, check=True, timeout=15)
        except BaseException as error:
            try:
                remaining = active_units()
            except (OSError, ValueError, KeyError, subprocess.SubprocessError) as unavailable:
                raise UnjoinedWork("preflight completion unavailable; retain owned lock") from unavailable
            if remaining:
                raise UnjoinedWork("preflight physical work remains; retain owned lock") from error
            raise
    profiles = ["node-process-check", "node-issuer-check", "node-descriptor-check", "node-introduction-check", "node-join-check"]
    schedule = [{"iteration": iteration, "profile": profile, "state": "pending"} for iteration in range(1, 4) for profile in profiles]
    receipt = {"source": source, "artifacts": pins, "environment": original, "schedule": schedule}
    def save():
        (output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    save()  # The complete finite schedule exists before the first profile.
    failed = False
    started = time.monotonic()
    cpu_started = time.process_time()
    child_started = resource.getrusage(resource.RUSAGE_CHILDREN)
    observations = 0
    for number, case in enumerate(schedule):
        try:
            if source_identity(root) != source or artifacts() != pins or native_facts(storage) != original:
                raise RuntimeError("continued original prerequisite validity lost")
        except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError):
            case.update(state="invalid", invalid_environment="original prerequisite unavailable before profile")
            save()
            raise
        name = str(case["iteration"]) + "-" + case["profile"]
        began = time.monotonic()
        invalid = None
        with (output / (name + ".log")).open("x") as log:
            process = subprocess.Popen(["make", case["profile"]], cwd=root, stdout=log, stderr=subprocess.STDOUT)
            try:
                # Validity loss denies the run verdict immediately. The original
                # child still finishes its physical joins before the lock returns.
                while process.poll() is None:
                    try:
                        observed = native_facts(storage)
                        if observed != original and invalid is None:
                            invalid = "original native environment changed"
                            case["first_invalid_environment"] = {"elapsed_seconds": time.monotonic() - began, "observed": observed}
                            case["invalid_environment"] = invalid
                            save()
                        observations += 1
                    except (OSError, subprocess.SubprocessError, RuntimeError):
                        if invalid is None:
                            invalid = "original native evidence unavailable"
                            case["first_invalid_environment"] = {"elapsed_seconds": time.monotonic() - began}
                            case["invalid_environment"] = invalid
                            save()
                    time.sleep(.5)
                code = process.wait()
            except BaseException as error:
                raise UnjoinedWork("profile observation interrupted; original completion unproven, retain owned lock") from error
        case.update(exit=code, elapsed_seconds=time.monotonic() - began, state="failed" if code or invalid else "passed")
        if invalid:
            case["invalid_environment"] = invalid
        try:
            if source_identity(root) != source or artifacts() != pins:
                raise RuntimeError("source or artifact changed")
        except (OSError, ValueError, KeyError, RuntimeError):
            case["state"] = "failed"
            case.setdefault("invalid_environment", "source or artifact changed after profile")
        # A crashed driver can leave a bounded transient original service.
        # Never release the serialization lock while such work remains active.
        joined_until = time.monotonic() + 130
        while True:
            try:
                remaining = active_units()
            except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
                raise UnjoinedWork("physical completion evidence unavailable; retain owned lock") from error
            if not remaining:
                break
            case["state"] = "failed"
            case.setdefault("invalid_environment", "profile returned before transient physical work joined")
            try:
                save()
            except OSError as error:
                raise UnjoinedWork("receipt unavailable while physical work remains; retain owned lock") from error
            if time.monotonic() > joined_until:
                raise UnjoinedWork("physical work remains; retain owned lock until independently joined")
            time.sleep(.5)
        failed |= case["state"] != "passed"
        save()
        print(name, case["state"], "elapsed", round(case["elapsed_seconds"], 3), flush=True)
        if time.monotonic() - started > 90 * 60:
            raise RuntimeError("finite series budget exceeded; physical child joined, remaining cases unrun")
    receipt["elapsed_seconds"] = time.monotonic() - started
    receipt["driver_cpu_seconds"] = time.process_time() - cpu_started
    child = resource.getrusage(resource.RUSAGE_CHILDREN)
    receipt["driver_child_cpu_seconds"] = child.ru_utime + child.ru_stime - child_started.ru_utime - child_started.ru_stime
    receipt["validity_observations"] = observations
    save()
    return int(failed)


class UnjoinedWork(RuntimeError):
    pass


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site:
        raise RuntimeError("reviewed Python3.12 isolated standard-library mode (-I -S) required")
    if len(sys.argv) < 3 or sys.argv[1] not in ["exclusive", "baseline"]:
        raise RuntimeError("usage: run-development.py exclusive LOCK_PARENT -- COMMAND ... | baseline LOCK_PARENT OUTPUT -timeout=90m")
    lock = external_directory(sys.argv[2]) / "ardents-development.lock"
    lock.mkdir(mode=0o700)  # Atomic across cooperating Windows/WSL jobs; no waiting/retry.
    joined = True
    try:
        if sys.argv[1] == "exclusive":
            if len(sys.argv) < 5 or sys.argv[3] != "--":
                raise RuntimeError("exclusive requires an actual command")
            return subprocess.run(sys.argv[4:], check=False).returncode
        if len(sys.argv) != 5 or sys.argv[4] != "-timeout=90m":
            raise RuntimeError("baseline requires its registered finite timeout")
        output = external_directory(sys.argv[3])
        if any(output.iterdir()):
            raise RuntimeError("fresh empty evidence directory required; earlier attempts must remain")
        return baseline(output)
    except UnjoinedWork:
        joined = False
        raise
    except (KeyboardInterrupt, SystemExit):
        # An interrupted command may still own descendants or transient services.
        # No automatic unlock, takeover or claim that those users have joined.
        joined = False
        raise
    finally:
        if joined:
            lock.rmdir()


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("native development failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
    except KeyboardInterrupt:
        print("native development interrupted; retain owned lock until original jobs are joined", file=sys.stderr)
        sys.exit(130)
