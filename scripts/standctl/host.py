"""Standctl host commands. One SSH account and local Docker daemon per host."""

import contextlib
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile

ROOT = Path.home() / ".local/share/standctl"
STATE = ROOT / "lease.json"
TOKEN = re.compile(r"[0-9a-f]{32}")
DAEMON = ["docker", "--host", "unix://" + str(Path.home() / ".local/share/standctl/docker.sock")]
UNREADY = 69
BUSY = 75
GIB = 1024**3


class StandError(Exception):
    pass


@contextlib.contextmanager
def lock():
    with (ROOT / "operation.lock").open("ab") as file:
        try:
            fcntl.flock(file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            yield None
        else:
            yield file.fileno()


def load():
    try:
        return json.loads(STATE.read_text())
    except FileNotFoundError:
        return None


def save(lease):
    stage = STATE.with_suffix(".writing")
    with stage.open("w") as file:
        json.dump(lease, file)
        file.flush()
        os.fsync(file.fileno())
    stage.replace(STATE)
    sync_root()


def sync_root():
    descriptor = os.open(ROOT, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def docker(*args, **kwargs):
    # The socket is set explicitly at install time and shared by every lease.
    env = {
        key: value for key, value in os.environ.items()
        if not key.startswith(("COMPOSE_", "DOCKER_"))
    }
    env.update(kwargs.pop("env", {}))
    return subprocess.run([*DAEMON, *args], env=env, **kwargs)


def capture(*args, stderr=None):
    command = docker(*args, check=True, stdout=subprocess.PIPE, stderr=stderr, text=True, timeout=30)
    return command.stdout.strip()


def daemon_id():
    return capture("info", "--format", "{{.ID}}")


def project(lease):
    return "standctl-" + lease["id"]


def directory(lease):
    return ROOT / project(lease)


def digest(token):
    if not TOKEN.fullmatch(token):
        raise StandError("invalid lease token")
    return hashlib.sha256(token.encode()).hexdigest()


def readiness(request):
    required = request.get("requirements", {})
    result = {"issues": []}
    issues = result["issues"]
    if not hasattr(tarfile, "data_filter"):
        issues.append("Python tarfile data extraction filter is required")
    try:
        info = json.loads(capture("info", "--format", "{{json .}}", stderr=subprocess.PIPE))
    except (OSError, subprocess.SubprocessError):
        issues.append("Docker daemon unavailable")
        return result
    if not info["ID"]:
        issues.append("Docker daemon ID is empty")
    cpus = info["NCPU"]
    compose_cpus = request.get("compose_cpus") or cpus
    cpu_limit = request.get("cpu_limits", {}).get(request.get("service"), compose_cpus)
    memory = info["MemTotal"]
    result.update(cpus=cpus, effective_cpus=min(cpus, cpu_limit), memory_bytes=memory)
    if cpu_limit > cpus:
        issues.append(f"configured CPU limit {cpu_limit:g} exceeds host {cpus}")
    if result["effective_cpus"] < required.get("cpus", 0):
        issues.append(f"requires {required['cpus']} CPUs")
    if memory < required.get("memory_gib", 0) * GIB:
        issues.append(f"requires {required['memory_gib']} GiB RAM")
    if required.get("cpus") and not info.get("CpuCfsQuota"):
        issues.append("Docker CPU quota unavailable")
    if required.get("memory_gib") and not info.get("MemoryLimit"):
        issues.append("Docker memory limit unavailable")
    try:
        result["compose"] = capture("compose", "version", "--short", stderr=subprocess.PIPE)
    except (OSError, subprocess.SubprocessError):
        issues.append("Docker Compose unavailable")
    image = required.get("image")
    if image:
        try:
            result["image_id"] = capture("image", "inspect", "--format", "{{.Id}}", image, stderr=subprocess.PIPE)
        except (OSError, subprocess.SubprocessError):
            issues.append(f"local image missing or unreadable: {image}")
    result["daemon"] = info["ID"]
    return result


def acquire(request):
    token_hash = digest(request["token"])
    lease = load()
    if lease:
        return 0 if lease["token_hash"] == token_hash else BUSY
    result = readiness(request)
    if result["issues"]:
        print(", ".join(result["issues"]), file=sys.stderr)
        return UNREADY
    lease = {
        "id": token_hash[:24],
        "token_hash": token_hash,
        "owner": request["owner"],
        "commit": request["commit"],
        "compose": request["compose"],
        "cpu_limits": request.get("cpu_limits", {}),
        "daemon": result["daemon"],
        "since": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "phase": "leased",
    }
    save(lease)
    return 0


def stand_state():
    with lock() as descriptor:
        lease = load()
        if lease is None:
            return {"state": "busy" if descriptor is None else "free"}
        phase = lease["phase"]
        state = "busy" if descriptor is None else (
            "leased" if phase == "leased" else "recovery-required"
        )
        public = {key: lease[key] for key in ("id", "owner", "commit", "since")}
        return dict(public, state=state, phase=phase)


def owned(token):
    lease = load()
    if lease is None or digest(token) != lease["token_hash"]:
        raise StandError("lease is no longer held by this token")
    return lease


def upload(lease, request):
    if not hasattr(tarfile, "data_filter"):
        raise StandError("Python tarfile data extraction filter is required")
    base = directory(lease)
    source = base / "source"
    if source.exists():
        raise StandError("source already uploaded for this lease")
    base.mkdir(exist_ok=True)
    stage = base / "incoming"
    if stage.exists():
        shutil.rmtree(stage)
    stage.mkdir()
    archive = base / "source.tar"
    remaining = request["size"]
    if not isinstance(remaining, int) or not 0 < remaining <= 1024**3:
        raise StandError("invalid archive size (maximum 1 GiB)")
    hasher = hashlib.sha256()
    with archive.open("wb") as file:
        while remaining:
            chunk = sys.stdin.buffer.read(min(remaining, 256 * 1024))
            if not chunk:
                raise StandError("source upload was interrupted")
            file.write(chunk)
            hasher.update(chunk)
            remaining -= len(chunk)
    if hasher.hexdigest() != request["sha256"]:
        raise StandError("source checksum mismatch")
    with tarfile.open(archive, "r:") as bundle:
        bundle.extractall(stage, filter="data")
    compose_file = (stage / lease["compose"]).resolve()
    if not compose_file.is_relative_to(stage) or not compose_file.is_file():
        raise StandError("Compose file must be inside the uploaded revision")
    # The staging tree sits under a private host directory. Inside the
    # read-only bind mount, the container's distinct UID must be able to
    # traverse every source directory created under our restrictive umask.
    for source_directory, _, _ in os.walk(stage):
        Path(source_directory).chmod(0o755)
    stage.rename(source)
    archive.unlink()
    evidence = base / "evidence"
    evidence.mkdir(exist_ok=True)
    # Containers can run as a UID distinct from the SSH account. The enclosing
    # standctl state directory is private to the account on the host.
    evidence.chmod(0o777)


def compose(lease, args, descriptor):
    base = directory(lease)
    source = base / "source"
    if not source.is_dir():
        raise StandError("upload source before running Compose")
    files = ["--file", str(source / lease["compose"])]
    limits = lease.get("cpu_limits", {})
    if limits:
        override = base / "compose-override.json"
        override.write_text(json.dumps({
            "services": {service: {"cpus": cpus} for service, cpus in limits.items()},
        }))
        files.extend(["--file", str(override)])
    return docker(
        "compose", "--project-directory", str(source), "--project-name", project(lease),
        *files, *args,
        cwd=source, pass_fds=(descriptor,), env={
            "ARDENTS_DIAGNOSTIC_SOURCE": str(source),
            "ARDENTS_DIAGNOSTIC_EVIDENCE": str(base / "evidence"),
            "ARDENTS_DIAGNOSTIC_SOURCE_SHA": lease["commit"],
        },
    ).returncode


def resources(lease, kind):
    label = "label=com.docker.compose.project=" + project(lease)
    args = ["--quiet", "--filter", label]
    if kind == "container":
        args.insert(0, "--all")
    return capture(kind, "ls", *args).splitlines()


def cleanup(lease, descriptor):
    for kind in ("container", "network", "volume"):
        for resource in resources(lease, kind):
            flags = ["--force"] if kind == "container" else []
            docker(kind, "rm", *flags, resource, check=True, pass_fds=(descriptor,))
    if any(resources(lease, kind) for kind in ("container", "network", "volume")):
        raise StandError("Docker resources remain after cleanup")
    base = directory(lease)
    if base.exists():
        results = ROOT / "results" / lease["id"]
        results.mkdir(parents=True, exist_ok=True)
        for name in ("operations.log", "evidence"):
            artifact = base / name
            if artifact.exists():
                artifact.rename(results / name)
        shutil.rmtree(base)
    STATE.unlink()
    sync_root()
    return 0


def worker(descriptor, request):
    lease = owned(request["token"])
    action = request["action"]
    if daemon_id() != lease["daemon"]:
        raise StandError("Docker daemon changed")
    if action in ("release", "recover"):
        return cleanup(lease, descriptor)
    code = compose(lease, request["args"], descriptor)
    lease["phase"] = "leased"
    lease["exit_code"] = code
    save(lease)
    return 0


def start_worker(descriptor, lease, request):
    base = directory(lease)
    base.mkdir(exist_ok=True)
    action = request["action"]
    lease["phase"] = action
    save(lease)
    log = base / "operations.log"
    with log.open("ab") as output:
        output.write(("\n--- " + action + " ---\n").encode())
        output.flush()
        return subprocess.Popen(
            [sys.executable, __file__, "_worker", str(descriptor), json.dumps(request)],
            stdin=subprocess.DEVNULL, stdout=output, stderr=output,
            start_new_session=True, pass_fds=(descriptor,),
        )


def dispatch(request):
    action = request["action"]
    if action == "status":
        print(json.dumps(stand_state()))
        return 0
    if action == "doctor":
        result = readiness(request)
        result.pop("daemon", None)
        result.update(stand_state())
        print(json.dumps(result))
        return 0
    if action == "probe":
        if not daemon_id():
            raise StandError("Docker daemon ID is empty")
        print(capture("compose", "version", "--short"))
        if not hasattr(tarfile, "data_filter"):
            raise StandError("Python tarfile data extraction filter is required")
        return 0
    if action == "logs":
        lease_id = digest(request["token"])[:24]
        paths = (
            ROOT / ("standctl-" + lease_id) / "operations.log",
            ROOT / "results" / lease_id / "operations.log",
        )
        for log in paths:
            try:
                file = log.open("rb")
            except FileNotFoundError:
                continue
            with file:
                shutil.copyfileobj(file, sys.stdout.buffer)
            return 0
        lease = load()
        if lease is None or lease["token_hash"] != digest(request["token"]):
            raise StandError("no logs for this lease")
        return 0
    with lock() as descriptor:
        if descriptor is None:
            return BUSY
        if action == "acquire":
            return acquire(request)
        lease = owned(request["token"])
        if lease["phase"] != "leased" and action != "recover":
            raise StandError("lease needs recovery; inspect operations.log first")
        if action == "upload":
            upload(lease, request)
            return 0
        if action not in ("compose", "release", "recover"):
            raise StandError("unknown action")
        process = start_worker(descriptor, lease, request)
        # The worker keeps the same flock while this SSH request waits. If the
        # request dies, the worker retains the lock and its output stays on disk.
        code = process.wait()
        if code:
            raise StandError("host operation interrupted or failed; inspect logs")
        if action == "compose":
            print(json.dumps({"exit_code": owned(request["token"])["exit_code"]}))
        return 0


def main():
    os.umask(0o077)
    if len(sys.argv) > 1 and sys.argv[1] == "_worker":
        return worker(int(sys.argv[2]), json.loads(sys.argv[3]))
    return dispatch(json.loads(sys.stdin.buffer.readline(65537)))


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (StandError, OSError, ValueError, tarfile.TarError, subprocess.SubprocessError) as error:
        print(f"standctl host: {error}", file=sys.stderr)
        sys.exit(1)
