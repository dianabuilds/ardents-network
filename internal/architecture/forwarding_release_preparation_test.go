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
assert tool["COMMANDS"] == ("ardents-node",)
assert tool["COMMAND_PACKAGES"] == {"ardents-node": "ardents-next"}
record = tool["source_record"]
for mode in ("100644", "100755"):
    assert record("cmd/ardents-next/main.go", {"sha256":"a"*64, "mode":mode}) == ("a"*64, int(mode[-3:], 8))
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

// The former TestForwardingInitialReleasePreparationRequiresCompleteSafetyHorizon
// retired with prepare-forwarding-release-plan.py (#604 FJD stage D, ADR-0125):
// the public plan preparer bound the old ardents-node program and the retired
// Forwarding template. The platform-led signing cutover keeps the canonical
// prepare-release-plan producer and signer; a new-path Forwarding candidate
// build and release plan builder is a separate follow-up, exactly as for Source
// (#604 stage 1.6). The shared candidate builder above remains the build-scope
// reference for the retained JOIN/Descriptor cohorts.
