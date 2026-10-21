#!/usr/bin/env python3
"""Read-only cut-prefix oracle, invoked AFTER the original outside stand join.

This test tool never adopts, repairs, resumes, deletes or starts a prefix. Its
classification grants no product authority and does not itself prove the join.
The outside controller separately records original unit/PID/cgroup completion.
"""
import hashlib
import json
from pathlib import Path
import stat
import sys


def inspect(root, future):
    def native(name):
        path = Path(name)
        if not path.is_absolute() or ".." in path.parts:
            raise ValueError("noncanonical native path")
        actual = root / str(path).lstrip("/")
        # Resolve beneath only this retained private rootfs, never a host alias.
        if not actual.resolve().is_relative_to(root):
            raise ValueError("foreign prefix path")
        return actual

    marker = json.loads(native(future + "/cut-observation.json").read_bytes())
    request = json.loads(native(marker["request"]).read_bytes())
    raw = native(marker["journal"]).read_bytes()
    if not raw or len(raw) > 4 << 20:
        raise ValueError("missing or oversized original journal")
    # A torn final append is a retained uncertain prefix. No earlier complete
    # phase can turn it into a resumable/accepting transaction.
    complete = raw.splitlines(keepends=True)
    torn = not complete[-1].endswith(b"\n")
    if torn:
        complete.pop()
    records = []
    phases = {"initial", "admitted", "account-preimage", "account-auxiliary-preimage", "account-intent", "account-born", "account-auxiliary-result", "account-auxiliary-cleared", "account-ready", "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "seal-intent", "sealed", "selected", "reload-intent", "stopped-verified"}
    fields = ["sequence", "phase", "path", "device", "inode", "sha256", "mode", "uid", "gid"]
    for number, line in enumerate(complete):
        record = json.loads(line)
        if list(record) != fields or record["sequence"] != number or record["phase"] not in phases:
            raise ValueError("unrecognized original prefix")
        records.append(record)
    if not records or records[0]["phase"] != "initial" or not any(r["phase"] == marker["phase"] for r in records):
        raise ValueError("selected physical cut lacks original journal evidence")
    if marker.get("control") == "process-cut" and records[-1]["phase"] != marker["phase"]:
        raise ValueError("physical cut advanced beyond the required frontier; requested uncertainty class unproven")
    # Independent physical identity/access/hash checks for every completed leaf.
    # A born-only leaf may contain zero, partial or complete intended bytes;
    # without its durable ready record it remains uncertain and repair-required.
    latest = {}
    for record in records:
        if record["phase"] in {"file-born", "file-ready", "directory-born", "directory-ready", "sealed", "account-born", "account-auxiliary-result"}:
            latest[record["path"]] = record
    completed = 0
    for name, record in latest.items():
        path = native(name)
        info = path.lstat()
        if (info.st_dev, info.st_ino) != (record["device"], record["inode"]) or stat.S_ISLNK(info.st_mode):
            raise ValueError("recorded original birth substituted")
        if record["phase"] in {"file-born", "directory-born"}:
            continue
        if (stat.S_IMODE(info.st_mode), info.st_uid, info.st_gid) != (record["mode"], record["uid"], record["gid"]):
            raise ValueError("completed original access changed")
        if record["phase"] in {"file-ready", "account-born", "account-auxiliary-result"}:
            if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
                raise ValueError("completed original leaf type changed")
            with path.open("rb") as stream:
                actual = hashlib.file_digest(stream, "sha256").hexdigest()
            if actual != record["sha256"]:
                raise ValueError("completed original bytes changed")
            completed += 1
    for key in ["state_root", "participation_root"]:
        path = native(request[key])
        if marker.get("control") == "semantic-content" and key == "state_root":
            if not path.is_dir() or sorted(p.name for p in path.iterdir()) != ["foreign-history"] or (path / "foreign-history").read_bytes() != b"foreign semantic state\n":
                raise ValueError("causal foreign semantic contents were removed or changed")
            continue
        if path.exists() and (not path.is_dir() or any(path.iterdir())):
            raise ValueError("installation initialized semantic history")
    if not native(marker["history"]).is_dir() or not native(marker["distribution"]).is_dir():
        raise ValueError("original Release history or signed inputs removed")
    print(json.dumps({"outcome": "repair-required", "requested_cut": marker["phase"], "last_complete_phase": records[-1]["phase"], "torn_tail": torn, "completed_leaves_rechecked": completed}, sort_keys=True))


if __name__ == "__main__":
    if not sys.flags.isolated or not sys.flags.no_site or len(sys.argv) != 3:
        raise SystemExit("isolated read-only oracle requires original stand root and in-stand future parent")
    inspect(Path(sys.argv[1]).resolve(strict=True), sys.argv[2])
