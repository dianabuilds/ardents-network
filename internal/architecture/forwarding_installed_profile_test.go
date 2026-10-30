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
assert tool["V1_CASES"] == {
    "lifetime-refill":"TestForwardingInstalledNativeLifetimeAndRefill",
    "retirement-history-missing":"TestForwardingInstalledNativeRetirementHistoryMissing",
    "retirement-history-damaged":"TestForwardingInstalledNativeRetirementHistoryDamaged",
    "unknown-domain-join":"TestForwardingInstalledNativeUnknownDomainJoin"}
matches = tool["matches_scope"]
artifacts = {"ardents-node":"a"*64, "ardents-control":"b"*64, "forwarding-installed.test":"c"*64}
pins = {"artifacts":artifacts}
receipt = {"schema":"ardents-forwarding-bootstrap-receipt-v1", "source_manifest_sha256":"d"*64,
           "artifacts":artifacts.copy(), "cases":list(tool["V1_CASES"]),
           "case_inputs":{case:{name:hashlib.sha256((case+name).encode()).hexdigest() for name in tool["INPUTS"]} for case in tool["V1_CASES"]}}
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

// Finite recipe consistency is still not proof of build, native completion or
// direct Product Owner acceptance. These controls never invoke installed tests.
func TestForwardingTransitionProfileRequiresCompleteDistinctProgramScope(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import copy, hashlib, runpy, sys
tool = runpy.run_path(sys.argv[1])
legacy = ["lifetime-refill","retirement-history-missing","retirement-history-damaged","unknown-domain-join"]
expected = legacy + ["replacement-lifetime-refill","replacement-unknown-domain-join","replacement-changed-account","replacement-stale-authority"]
expected += [f"replacement-competing-{role}-{kind}" for role in ("issuer","descriptor","introduction","join") for kind in ("service","socket")]
for prefix, count in [("initial",120),("replacement",62),("start-initial",26),("start-selected",26),("restart",26),("retirement-initial",18),("retirement-selected",18)]:
    expected += [f"{prefix}-cut-{number:03d}" for number in range(1,count+1)]
assert len(expected) == 312 and len(set(expected)) == 312
assert list(tool["V1_CASES"]) == legacy and list(tool["CASES"]) == expected
artifacts = {"ardents-node":"a"*64,"ardents-control":"b"*64,"forwarding-installed.test":"c"*64,
             "ardents-node-successor":"e"*64,"ardents-control-successor":"f"*64,"forwarding-control.test":"1"*64}
pins = {"artifacts":artifacts,"source_manifest":"d"*64,"runtime_source_manifests":{"initial":"2"*64,"successor":"3"*64}}
old_inputs = {"binding","plan","bundle","profile","presentations","peer_plan","peer_presentations","peer_certificate"}
def input_set(case):
    if case.startswith("initial-cut-"): return {"initial_cut_inputs"}
    if case in ("replacement-lifetime-refill","replacement-unknown-domain-join","replacement-changed-account","replacement-stale-authority") or case.startswith(("replacement-competing-","replacement-cut-","start-selected-cut-","restart-cut-","retirement-selected-cut-")): return old_inputs | {"successors"}
    return old_inputs
receipt = {"schema":"ardents-forwarding-bootstrap-receipt-v2","source_manifest_sha256":"d"*64,
           "runtime_source_manifests":pins["runtime_source_manifests"].copy(),"artifacts":artifacts.copy(),
           "cases":expected,"case_inputs":{case:{name:hashlib.sha256((case+name).encode()).hexdigest() for name in input_set(case)} for case in expected}}
matches = tool["matches_scope"]
assert matches(receipt,pins,"d"*64)
for case in expected:
    assert tool["input_names"](case) == input_set(case)
    assert tool["matches_inputs"](receipt,case,receipt["case_inputs"][case])
    assert not tool["matches_inputs"](receipt,case,{})
for case in ["replacement-lifetime-refill","replacement-unknown-domain-join","replacement-changed-account","replacement-stale-authority","initial-cut-001","initial-cut-120","replacement-cut-057","start-initial-cut-018","start-selected-cut-026","restart-cut-026","retirement-initial-cut-017","retirement-selected-cut-018"] + [c for c in expected if c.startswith("replacement-competing-")]:
    changed=copy.deepcopy(receipt); changed["cases"].remove(case); changed["case_inputs"].pop(case)
    assert not matches(changed,pins,"d"*64),case
    for name in input_set(case):
        changed=copy.deepcopy(receipt); changed["case_inputs"][case].pop(name)
        assert not matches(changed,pins,"d"*64),(case,name)
for field,value in [("schema","ardents-forwarding-bootstrap-receipt-v1"),("cases",legacy),("cases",expected[::-1]),("case_inputs",{}),("runtime_source_manifests",{}),("source_manifest_sha256","a"*64)]:
    changed=copy.deepcopy(receipt); changed[field]=value; changed["accepted"]=True
    assert not matches(changed,pins,"d"*64),(field,value)
for source in ("initial","successor"):
    changed=copy.deepcopy(receipt); changed["runtime_source_manifests"][source]="4"*64
    assert not matches(changed,pins,"d"*64),source
same=copy.deepcopy(pins); same["runtime_source_manifests"]["successor"]=same["runtime_source_manifests"]["initial"]
changed=copy.deepcopy(receipt); changed["runtime_source_manifests"]=same["runtime_source_manifests"].copy()
assert not matches(changed,same,"d"*64)
same=copy.deepcopy(pins); same["artifacts"]["ardents-node-successor"]=same["artifacts"]["ardents-node"]
changed=copy.deepcopy(receipt); changed["artifacts"]=same["artifacts"].copy()
assert not matches(changed,same,"d"*64)
for name in artifacts:
    missing=copy.deepcopy(pins); missing["artifacts"].pop(name)
    changed=copy.deepcopy(receipt); changed["artifacts"].pop(name)
    assert not matches(changed,missing,"d"*64),name
for invalid in ({},[],None):
    assert not matches(invalid,pins,"d"*64)
    assert not matches(receipt,invalid,"d"*64)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-forwarding-installed.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Forwarding transition scope consistency: %v\n%s", err, output)
	}
}

// A serial receipt admits only its independently prepared original case. The
// unchanged full catalog and controller/source commitments cannot be replaced
// with an easy subset, and another stand cannot borrow that case's input pins.
func TestForwardingSerialProfileBindsOneOriginalStandAndCompleteCampaign(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import copy, hashlib, runpy, sys
tool = runpy.run_path(sys.argv[1])
catalog = list(tool["CASES"])
artifacts = {"ardents-node":"a"*64,"ardents-control":"b"*64,"forwarding-installed.test":"c"*64,
             "ardents-node-successor":"e"*64,"ardents-control-successor":"f"*64,"forwarding-control.test":"1"*64}
sources = {"initial":"2"*64,"successor":"3"*64}
controllers = {"controller":"4"*64,"worker":"5"*64,"input_preparer":"6"*64}
campaigns = set()
matches = tool["matches_scope"]
for number, case in enumerate(catalog, 1):
    actual = {name:hashlib.sha256((case+name).encode()).hexdigest() for name in tool["input_names"](case)}
    stand = {"case":f"installed{number}","reservation_sha256":hashlib.sha256(case.encode()).hexdigest()}
    receipt = {"schema":"ardents-forwarding-bootstrap-receipt-v3","source_manifest_sha256":"d"*64,
               "runtime_source_manifests":sources.copy(),"artifacts":artifacts.copy(),"cases":catalog.copy(),
               "controllers":controllers.copy(),"selected_case":case,"native_stand":stand.copy(),
               "case_inputs":{case:actual.copy()}}
    receipt["campaign_sha256"] = tool["campaign_digest"](receipt)
    pins = {"source_manifest":"d"*64,"runtime_source_manifests":sources.copy(),"artifacts":artifacts.copy(),
            "controllers":controllers.copy(),"selected_case":case,"native_stand":stand.copy(),
            "campaign_sha256":receipt["campaign_sha256"]}
    campaigns.add(receipt["campaign_sha256"])
    assert matches(receipt,pins,"d"*64),case
    assert len(tool["profile_cases"](receipt)) == 312
    assert tool["matches_inputs"](receipt,case,actual)
    other = catalog[number % len(catalog)]
    assert not tool["matches_inputs"](receipt,other,actual),(case,other)
    for name in actual:
        wrong=actual.copy(); wrong[name]="0"*64
        assert not tool["matches_inputs"](receipt,case,wrong),(case,name)
        changed=copy.deepcopy(receipt); changed["case_inputs"][case].pop(name)
        assert not matches(changed,pins,"d"*64),(case,name)
    changed=copy.deepcopy(receipt); changed["case_inputs"][other]=actual.copy()
    assert not matches(changed,pins,"d"*64),case
    changed=copy.deepcopy(receipt); changed["selected_case"]=other
    assert not matches(changed,pins,"d"*64),case
    # An all-catalog discovery obligation remains even for a single prepared case.
    changed=copy.deepcopy(receipt); changed["cases"]=[case]
    changed["campaign_sha256"]=tool["campaign_digest"](changed)
    changed_pins=copy.deepcopy(pins);changed_pins["campaign_sha256"]=changed["campaign_sha256"]
    assert not matches(changed,changed_pins,"d"*64),case
assert len(campaigns)==1, "fresh case data must not redefine campaign identity"
for field in ("selected_case","native_stand","controllers","campaign_sha256"):
    changed=copy.deepcopy(receipt);changed.pop(field)
    assert not matches(changed,pins,"d"*64),field
    changed=copy.deepcopy(pins);changed.pop(field)
    assert not matches(receipt,changed,"d"*64),field
for field in ("case","reservation_sha256"):
    changed=copy.deepcopy(receipt);changed["native_stand"][field]="installed999" if field=="case" else "0"*64
    assert not matches(changed,pins,"d"*64),field
non_ascii_decimal_case = "installed١"
assert non_ascii_decimal_case == "installed" + chr(0x0661)
for bad in ("preparation1","installed0","installed01","installed-1","installed1/other",non_ascii_decimal_case,[],None):
    changed=copy.deepcopy(receipt); changed["native_stand"]["case"]=bad
    changed_pins=copy.deepcopy(pins);changed_pins["native_stand"]=changed["native_stand"].copy()
    assert not matches(changed,changed_pins,"d"*64),bad
for name in controllers:
    changed=copy.deepcopy(receipt);changed["controllers"][name]="0"*64
    assert not matches(changed,pins,"d"*64),name
    # Even jointly changed controller declarations invalidate the campaign pin.
    changed_pins=copy.deepcopy(pins);changed_pins["controllers"]=changed["controllers"].copy()
    assert not matches(changed,changed_pins,"d"*64),name
for field in ("source_manifest_sha256","runtime_source_manifests","artifacts","cases","controllers"):
    changed=copy.deepcopy(receipt)
    if field=="source_manifest_sha256":changed[field]="0"*64
    elif field=="cases":changed[field]=changed[field][::-1]
    else:changed[field][next(iter(changed[field]))]="0"*64
    assert tool["campaign_digest"](changed)!=receipt["campaign_sha256"],field
for bad in ({},[],None):
    assert not matches(bad,pins,"d"*64)
    assert not matches(receipt,bad,"d"*64)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-forwarding-installed.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Forwarding serial scope consistency: %v\n%s", err, output)
	}
}
