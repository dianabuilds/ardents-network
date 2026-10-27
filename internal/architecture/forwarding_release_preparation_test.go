package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestForwardingBuildRecordsAndSourceManifestRules(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import json, runpy, sys
tool = runpy.run_path(sys.argv[1])
assert tool["COMMANDS"] == ("ardents-node", "ardents-control")
record = tool["source_record"]
for mode in ("100644", "100755"):
    assert record("cmd/ardents-node/main.go", {"sha256":"a"*64, "mode":mode}) == ("a"*64, int(mode[-3:], 8))
for name, value in (
    ("../outside", {"sha256":"a"*64, "mode":"100644"}),
    ("/outside", {"sha256":"a"*64, "mode":"100644"}),
    ("", {"sha256":"a"*64, "mode":"100644"}),
    (".", {"sha256":"a"*64, "mode":"100644"}),
    ("main.go", "a"*64),
    ("main.go", {"sha256":"A"*64, "mode":"100644"}),
    ("main.go", {"sha256":"a"*64, "mode":"120000"}),
    ("main.go", {"sha256":"a"*64, "mode":"100644", "accept":True}),
):
    try:
        record(name, value)
    except RuntimeError:
        pass
    else:
        raise AssertionError(("foreign source byte/mode record admitted", name, value))
modules = tool["compiled_dependencies"]
def encoded(rows): return "\n".join(json.dumps(row) for row in rows).encode()
valid = [{"ImportPath":"fmt", "Standard":True}, {"ImportPath":"example.test/main", "Module":{"Path":"example.test/main", "Main":True}}]
assert modules(encoded(valid)) == ([{"Path":"example.test/main", "Main":True}], ["example.test/main", "fmt"])
for rows in (
    [], [{"ImportPath":"foreign"}],
    [{"ImportPath":"foreign", "Error":{"Err":"missing"}}],
    [{"ImportPath":"foreign", "Module":{"Path":"foreign", "Replace":{"Path":"other"}}}],
    [{"ImportPath":"a", "Module":{"Path":"same", "Sum":"one"}}, {"ImportPath":"b", "Module":{"Path":"same", "Sum":"two"}}],
):
    try:
        modules(encoded(rows))
    except RuntimeError:
        pass
    else:
        raise AssertionError("unavailable or substituted compiled dependency admitted")
try:
    json.loads('{"files":{},"files":{}}', object_pairs_hook=tool["unique"])
except RuntimeError:
    pass
else:
    raise AssertionError("duplicate manifest field admitted")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "build-forwarding-candidate.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Forwarding build record rules: %v\n%s", err, output)
	}
}

func TestForwardingInitialReleasePreparationRequiresCompleteSafetyHorizon(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import runpy, sys
from datetime import datetime, timedelta, timezone
tool = runpy.run_path(sys.argv[1])
assert tool["COMMANDS"] == ("ardents-node", "ardents-control")
check = tool["check_time_bounds"]
now = datetime(2026, 10, 27, tzinfo=timezone.utc)
no_new, terminate = now + timedelta(hours=2), now + timedelta(hours=3)
for expires in (terminate, now + timedelta(days=7)):
    check(now, expires, no_new, terminate)
for expires, stop_new, stop_all in (
    (now + timedelta(minutes=10), no_new, terminate),
    (terminate, now, terminate),
    (terminate, no_new, no_new),
    (terminate, terminate, no_new),
    (now + timedelta(days=7, seconds=1), no_new, terminate),
):
    try:
        check(now, expires, stop_new, stop_all)
    except RuntimeError:
        pass
    else:
        raise AssertionError("initial Forwarding admitted incomplete safety horizon")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "prepare-forwarding-release-plan.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Forwarding initial release bounds: %v\n%s", err, output)
	}
}
