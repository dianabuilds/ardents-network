"""Exercise the R-172 research candidate outside the product module graph.

  run.py                 candidate: socket, library, Node TLS and return-path checks
  run.py return-path     one return-path run; --module selects the implementation
  run.py controls        the return-path run against each pinned official release
  run.py patched         the return path against the patched official release

The implementation named "patched" is official quic-go v0.66.0 with the
retained quic-go-v0.66.0-probe-budget.patch applied to a temporary copy.

Every run verifies the published module checksum, renders the tests into a
temporary module that imports the current Ardents transport owners, and keeps
builds and caches outside the repository.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
ARDENTS = "github.com/dianabuilds/ardents-network"
TOKEN = "ARDENTS_QUIC_MODULE"
MODULES = {
    "candidate": ("github.com/qoke/mp-quic-go", "v0.3.1", "h1:dz9LkG+QZr2DY/voKFZuNdlmDY4HbrKHrg2jA5DGiNM="),
    "patched": ("github.com/quic-go/quic-go", "v0.66.0", "h1:NCsBBGZrI2HZC6aKt4I89T5i4Crt39EYNqkh0w2Wgmg="),
    "v0.62.0": ("github.com/quic-go/quic-go", "v0.62.0", "h1:ZHDjCk5OacATwGvs8PWE97CTvX7AqZiVoW7++ZOXTf8="),
    "v0.63.0": ("github.com/quic-go/quic-go", "v0.63.0", "h1:LIFGHI4PFUhhw2dDD1ARHdCff143ffMHwZtbnbuJ78A="),
    "v0.66.0": ("github.com/quic-go/quic-go", "v0.66.0", "h1:NCsBBGZrI2HZC6aKt4I89T5i4Crt39EYNqkh0w2Wgmg="),
}
EXCEEDED = "return-path budget exceeded"
EVIDENCE = re.compile(
    r"server accepted|client wrote|server sent \d+ bytes|first server datagram|"
    r"through validated rebinding|^--- |^\s+--- |return-path budget exceeded|"
    r"did not become quiet|is not the original mapping|still peers|did not close|did not return"
)


def run(*args, cwd, check=True, show=False):
    result = subprocess.run(
        ["go", *args], cwd=cwd, text=True, capture_output=True,
        env={**os.environ, "GOWORK": "off"},
    )
    if show or (check and result.returncode):
        print(result.stdout, result.stderr, sep="", end="", flush=True)
    if check:
        result.check_returncode()
    return result


def write_module(work, module, version):
    (work / "go.mod").write_text(
        f"module {ARDENTS}/experiments/r172\n\ngo 1.27.4\n"
        f"require (\n{ARDENTS} v0.0.0\n{module} {version}\n)\n"
        f"replace {ARDENTS} => {json.dumps(str(ROOT))}\n"
    )


def fetch(work, module, version, checksum):
    dep = json.loads(run("mod", "download", "-json", f"{module}@{version}", cwd=work).stdout)
    if dep["Sum"] != checksum:
        raise ValueError(f"{module}@{version} checksum mismatch: {dep['Sum']}")
    return dep


def apply_patch(root, patch):
    """Apply a unified Git diff to the files below root. Every hunk must match
    its context exactly, so the patch also proves the identity of the base."""
    files, path, hunk = {}, None, None
    for line in patch.splitlines(keepends=True):
        if line.startswith("diff --git "):
            path, hunk = None, None
        elif line.startswith("+++ ") and path is None:
            if not line[4:].strip().startswith("b/"):
                raise ValueError("unexpected patch target")
            path = line[6:].strip()
            files[path] = []
        elif line.startswith("@@") and path is not None:
            header = re.match(r"@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@", line)
            hunk = {"start": int(header[1]), "count": int(header[2] or 1), "lines": []}
            files[path].append(hunk)
        elif hunk is not None and not line.startswith("\\"):
            hunk["lines"].append(line)
    for name, hunks in files.items():
        target = root / name
        lines = target.read_text().splitlines(keepends=True)
        result, done = [], 0
        for h in hunks:
            old = [line[1:] for line in h["lines"] if line[0] in " -"]
            new = [line[1:] for line in h["lines"] if line[0] in " +"]
            start = h["start"] - 1 if h["count"] else h["start"]
            if lines[start:start + len(old)] != old:
                raise ValueError(f"the patch does not apply to {name} at line {h['start']}")
            result += lines[done:start] + new
            done = start + len(old)
        target.write_text("".join(result + lines[done:]))
    return sorted(files)


def patched_official(work, checksum):
    """Copy the verified official module, apply the retained patch, and point
    the temporary module at the copy. The module path stays the official one."""
    module, version, _ = MODULES["patched"]
    dep = fetch(work, module, version, checksum)
    source = work / "quic-go-patched"
    shutil.copytree(dep["Dir"], source)
    for path in source.rglob("*"):
        path.chmod(0o700 if path.is_dir() else 0o600)
    patch = (HERE / "quic-go-v0.66.0-probe-budget.patch").read_text()
    apply_patch(source, patch)
    with (work / "go.mod").open("a") as manifest:
        manifest.write(f"\nreplace {module} => {json.dumps(str(source))}\n")
    digest = hashlib.sha256(patch.encode()).hexdigest()
    print(f"== {module}@{version} {checksum} + quic-go-v0.66.0-probe-budget.patch sha256 {digest} ==", flush=True)
    return dep["Origin"]["Hash"]


def render_return_path(work, module):
    target = work / "returnpath"
    target.mkdir()
    rendered = target / "return_path_test.go"
    rendered.write_text((HERE / "return_path_test.go.in").read_text().replace(TOKEN, module))
    if TOKEN in rendered.read_text():
        raise ValueError("unreplaced module token")
    gofmt = Path(shutil.which("go")).with_name("gofmt")
    unformatted = subprocess.run([str(gofmt), "-l", str(rendered)], cwd=work, text=True, capture_output=True, check=True).stdout.strip()
    if unformatted:
        raise ValueError(f"rendered test is not gofmt-clean: {unformatted}")
    return target


def return_path(name, flags, pattern):
    module, version, checksum = MODULES[name]
    with tempfile.TemporaryDirectory(prefix="ardents-r172-") as tmp:
        work = Path(tmp)
        write_module(work, module, version)
        fetch(work, module, version, checksum)
        target = render_return_path(work, module)
        if name == "patched":
            patched_official(work, checksum)
        else:
            print(f"== {module}@{version} {checksum} ==", flush=True)
        result = run("test", "-mod=mod", "-count=1", "-v", *flags, "-run", pattern, "./returnpath", cwd=work, check=False)
    print(result.stdout, result.stderr, sep="", end="", flush=True)
    return result.returncode, result.stdout + result.stderr


def candidate(work):
    module, version, checksum = MODULES["candidate"]
    dep = fetch(work, module, version, checksum)
    source = work / "candidate"
    shutil.copytree(dep["Dir"], source)
    source.chmod(0o700)
    shutil.copyfile(HERE / "budget_test.go.in", source / "ardents_budget_test.go")
    run("test", "-mod=readonly", "-count=1", "-v",
        "-run", "^Test(ArdentsBudgetToSocket|ConnectionServerPathProbeAmplificationLimit|ConnectionServerPathChallengesOnNewPath)$", ".", cwd=source, show=True)
    write_module(work, module, version)
    shutil.copyfile(HERE / "node_tls_test.go.in", work / "node_tls_test.go")
    run("test", "-mod=mod", "-count=1", "-v", "-run", "^TestArdentsNodeTLS$", ".", cwd=work, show=True)
    code, _ = return_path("candidate", [], "^TestReturnPathBudget$")
    return code


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("command", nargs="?", default="candidate", choices=["candidate", "return-path", "controls", "patched"])
    parser.add_argument("--module", default="candidate", choices=sorted(MODULES))
    parser.add_argument("--race", action="store_true")
    parser.add_argument("--backend", choices=["native", "generic"])
    parser.add_argument("--case", help="one subtest name, for example repeated-32-byte-packets")
    args = parser.parse_args()
    flags = ["-race"] if args.race else []
    pattern = "/".join([("^TestReturnPathBudget"), args.backend or "[^/]+", args.case or ".+"]) + "$"
    if args.command == "candidate":
        with tempfile.TemporaryDirectory(prefix="ardents-r172-") as tmp:
            raise SystemExit(candidate(Path(tmp)))
    names = {"controls": ["v0.62.0", "v0.63.0", "v0.66.0"], "patched": ["patched"]}.get(args.command, [args.module])
    failures = 0
    for name in names:
        code, output = return_path(name, flags, pattern)
        print("\n".join(line for line in output.splitlines() if EVIDENCE.search(line)), flush=True)
        if args.command == "controls":
            if code == 0:
                print(f"CONTROL NOT DISTINGUISHED: {name} satisfied the bound", flush=True)
                failures += 1
            elif EXCEEDED not in output:
                print(f"CONTROL INCONCLUSIVE: {name} failed for another reason", flush=True)
                failures += 1
        elif code:
            failures += 1
    raise SystemExit(1 if failures else 0)


if __name__ == "__main__":
    main()
