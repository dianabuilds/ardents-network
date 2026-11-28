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
import copy, json, os, pathlib, runpy, subprocess, sys, tempfile
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
# Exercise the Go tool's real module JSON shape in an isolated module.
with tempfile.TemporaryDirectory() as temporary:
    root = pathlib.Path(temporary).resolve()
    source = root / "third_party" / "quic-go"
    source.mkdir(parents=True)
    (root / "go.mod").write_text("module example.test/build\n\ngo 1.27.4\n\nrequire github.com/quic-go/quic-go v0.66.0\n\nreplace github.com/quic-go/quic-go => ./third_party/quic-go\n")
    (root / "main.go").write_text('package main\nimport "github.com/quic-go/quic-go"\nfunc main() { _ = quic.Value }\n')
    (source / "go.mod").write_text("module github.com/quic-go/quic-go\n\ngo 1.27.4\n")
    (source / "quic.go").write_text("package quic\nconst Value = 1\n")
    env = dict(os.environ, GOWORK="off", GOPROXY="off", GOSUMDB="off", GOENV="off", GOFLAGS="")
    raw = subprocess.check_output(["go", "list", "-deps", "-json", "."], cwd=root, env=env)
    files = {"third_party/quic-go/go.mod": {"sha256": "a"*64, "mode": "100644"},
             "third_party/quic-go/quic.go": {"sha256": "b"*64, "mode": "100644"},
             tool["QUIC_PATCH"]: {"sha256": "c"*64, "mode": "100644"}}
    manifest = {"files": files}
    records, packages = modules(raw, root, manifest)
    selected = next(item for item in records if item["Path"] == tool["QUIC_MODULE"])
    assert selected["Version"] == "v0.66.0"
    assert selected["Replace"]["Path"] == "./third_party/quic-go"
    assert selected["Replace"]["PublishedSum"] == tool["QUIC_SUM"]
    assert selected["Replace"]["PatchSHA256"] == "c"*64
    assert selected["Replace"]["SourceManifestSHA256"] == tool["digest"](tool["canonical"]({key: value for key, value in files.items() if key.startswith("third_party/quic-go/")}))
    assert "github.com/quic-go/quic-go" in packages
    decoder, tail, module_row = json.JSONDecoder(), raw.decode(), None
    while tail.strip():
        item, end = decoder.raw_decode(tail.lstrip())
        tail = tail.lstrip()[end:]
        if item.get("ImportPath") == "github.com/quic-go/quic-go":
            module_row = item
    assert module_row is not None
    variants = []
    for path, value in (
        (("Module", "Version"), "v0.65.0"),
        (("Module", "Path"), "example.test/other"),
        (("Module", "Dir"), str(root / "other")),
        (("Module", "GoMod"), str(root / "other" / "go.mod")),
        (("Module", "Replace", "Path"), "../outside"),
        (("Module", "Replace", "Dir"), str(root / "other")),
        (("Module", "Replace", "GoMod"), str(root / "other" / "go.mod")),
        (("Module", "Replace", "Version"), "v0.66.0"),
    ):
        changed = copy.deepcopy(module_row)
        obj = changed
        for part in path[:-1]:
            obj = obj[part]
        obj[path[-1]] = value
        variants.append((changed, manifest))
    variants.append((module_row, {"files": {key: value for key, value in files.items() if key != tool["QUIC_PATCH"]}}))
    variants.append((module_row, {"files": {key: value for key, value in files.items() if not key.startswith("third_party/quic-go/")}}))
    for changed, input_manifest in variants:
        try:
            modules(encoded([changed]), root, input_manifest)
        except RuntimeError:
            pass
        else:
            raise AssertionError(("incorrect QUIC replacement admitted", changed))
    # A successful Go command with no matching test cannot prove the source.
    verify = tool["verify_quic_source"]
    scope = verify.__globals__
    original = scope["command"]
    try:
        scope["command"] = lambda args, cwd, env: b"testing: warning: no tests to run\nPASS\n"
        try:
            verify(root, env)
        except RuntimeError:
            pass
        else:
            raise AssertionError("missing integrity test was accepted")
        scope["command"] = lambda args, cwd, env: b"--- PASS: TestQuicGoSourceIsPublishedModuleWithOnePatch (0.01s)\nPASS\n"
        verify(root, env)
    finally:
        scope["command"] = original

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
