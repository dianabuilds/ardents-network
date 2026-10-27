package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Public receipt consistency is not startup or an accepting bootstrap fixture.
// These pure controls execute no native profile, command or installed runtime.
func TestForwardingInstalledProfileRequiresExactPreparedScope(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import ast, copy, hashlib, runpy, sys
from pathlib import Path
tool = runpy.run_path(sys.argv[1])
assert tool["CASES"] == {
    "lifetime-refill":"TestForwardingInstalledNativeLifetimeAndRefill",
    "retirement-history-missing":"TestForwardingInstalledNativeRetirementHistoryMissing",
    "retirement-history-damaged":"TestForwardingInstalledNativeRetirementHistoryDamaged",
    "unknown-domain-join":"TestForwardingInstalledNativeUnknownDomainJoin"}
matches = tool["matches_scope"]
artifacts = {"ardents-node":"a"*64, "ardents-control":"b"*64, "forwarding-installed.test":"c"*64}
pins = {"artifacts":artifacts}
receipt = {"schema":"ardents-forwarding-bootstrap-receipt-v1", "source_manifest_sha256":"d"*64,
           "artifacts":artifacts.copy(), "cases":list(tool["CASES"]),
           "case_inputs":{case:{name:hashlib.sha256((case+name).encode()).hexdigest() for name in tool["INPUTS"]} for case in tool["CASES"]}}
assert matches(receipt, pins, "d"*64)
for case, original in receipt["case_inputs"].items():
    assert tool["matches_inputs"](receipt,case,original)
    assert not tool["matches_inputs"](receipt,"unreviewed",original)
    for other, foreign in receipt["case_inputs"].items():
        if case != other:
            assert not tool["matches_inputs"](receipt,case,foreign), (case,other)
    for name in original:
        missing = original.copy(); del missing[name]
        assert not tool["matches_inputs"](receipt,case,missing), (case,name)
for key, value in (("schema","source-bootstrap-v1"), ("source_manifest_sha256","f"*64),
                   ("cases",[]), ("cases",["lifetime-refill","unreviewed"]),
                   ("cases",["lifetime-refill","lifetime-refill"]),
                   ("artifacts",{}), ("case_inputs",{})):
    changed = copy.deepcopy(receipt); changed[key] = value; changed["accepted"] = True
    assert not matches(changed,pins,"d"*64), (key,value)
for name in artifacts:
    changed = copy.deepcopy(receipt); changed["artifacts"][name] = "f"*64
    assert not matches(changed,pins,"d"*64), name
    missing = copy.deepcopy(pins); del missing["artifacts"][name]
    assert not matches(receipt,missing,"d"*64), name
for case, inputs in receipt["case_inputs"].items():
    changed = copy.deepcopy(receipt); del changed["case_inputs"][case]
    assert not matches(changed,pins,"d"*64), case
    for name in inputs:
        changed = copy.deepcopy(receipt); del changed["case_inputs"][case][name]
        assert not matches(changed,pins,"d"*64), (case,name)
        changed = copy.deepcopy(receipt); changed["case_inputs"][case][name] = "not-a-hash"
        assert not matches(changed,pins,"d"*64), (case,name)
changed = copy.deepcopy(receipt); changed["case_inputs"]["unreviewed"] = inputs.copy()
assert not matches(changed,pins,"d"*64)
changed = copy.deepcopy(receipt); changed["case_inputs"][case]["unreviewed"] = "f"*64
assert not matches(changed,pins,"d"*64)
assert not matches(receipt,pins,"D"*64)
tree = ast.parse(Path(sys.argv[1]).read_bytes())
main = next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=="main")
calls = list(ast.walk(main))
bootstrap = [n.lineno for n in calls if isinstance(n,ast.Call) and isinstance(n.func,ast.Name) and n.func.id=="bootstrap"]
subprocesses = [n.lineno for n in calls if isinstance(n,ast.Call) and isinstance(n.func,ast.Attribute) and isinstance(n.func.value,ast.Name) and n.func.value.id=="subprocess"]
assert len(bootstrap)==2 and min(bootstrap)<min(subprocesses) and max(bootstrap)>max(subprocesses)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-forwarding-installed.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Forwarding installed scope preparation: %v\n%s", err, output)
	}
}
