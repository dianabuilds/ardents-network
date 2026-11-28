"""Exercise the R-172 research candidate outside the product module graph."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
MODULE = "github.com/qoke/mp-quic-go"
VERSION = "v0.3.1"
CHECKSUM = "h1:dz9LkG+QZr2DY/voKFZuNdlmDY4HbrKHrg2jA5DGiNM="
ARDENTS = "github.com/dianabuilds/ardents-network"


def run(*args, cwd, capture=False):
    result = subprocess.run(
        ["go", *args], cwd=cwd, text=True,
        env={**os.environ, "GOWORK": "off"},
        stdout=subprocess.PIPE if capture else None,
    )
    if result.returncode and capture:
        print(result.stdout, file=sys.stderr)
    result.check_returncode()
    return result.stdout


def main():
    with tempfile.TemporaryDirectory(prefix="ardents-r172-") as tmp:
        work = Path(tmp)
        (work / "go.mod").write_text(
            f"module {ARDENTS}/experiments/r172\n\ngo 1.27.4\n"
            f"require (\n{ARDENTS} v0.0.0\n{MODULE} {VERSION}\n)\n"
            f"replace {ARDENTS} => {json.dumps(str(ROOT))}\n"
        )
        dep = json.loads(run("mod", "download", "-json", f"{MODULE}@{VERSION}", cwd=work, capture=True))
        if dep["Sum"] != CHECKSUM:
            raise ValueError("candidate module checksum mismatch")
        source = work / "candidate"
        shutil.copytree(dep["Dir"], source)
        source.chmod(0o700)
        shutil.copyfile(HERE / "budget_test.go.in", source / "ardents_budget_test.go")
        run("test", "-mod=readonly", "-count=1", "-v",
            "-run", "^Test(ArdentsBudgetToSocket|ConnectionServerPathProbeAmplificationLimit|ConnectionServerPathChallengesOnNewPath)$", ".", cwd=source)
        shutil.copyfile(HERE / "node_tls_test.go.in", work / "node_tls_test.go")
        run("test", "-mod=mod", "-count=1", "-v", "-run", "^TestArdentsNodeTLS$", ".", cwd=work)


if __name__ == "__main__":
    main()
