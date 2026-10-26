package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceReleaseExpiryObservationRequiresSameInputsAndActualWallCrossing(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
select,predict,check=(loaded[name] for name in ("source_profile","source_release_expiry_prediction","source_release_expiry_observation"))
zero,other="0"*64,"1"*64
def rejection(call):
    try: call()
    except RuntimeError: return
    raise AssertionError("missing, substituted or unpaired expiry evidence accepted")
for operation,count in (("initial",70),("replacement",49)):
    chosen=select(["-timeout=10m"],f"recovery-refusal-{operation}-timestamp-expiry")
    assert chosen["recovery"] is True and chosen["operation"]==operation and chosen["prefix_records"]==count and chosen["fault"]=="timestamp-expiry"
    expected={"operation":operation,"prefix_records":count,"fault":"timestamp-expiry","observation":"same-eight-files-actual-wall-crossing",
              "other_bounds":"live-announced-no-emergency","after_cut":"repair-required","failure_header_permitted":False,"start_permitted":False}
    assert predict({"release_expiry":[expected]},operation,count)==expected
    for inventory in (None,[],[expected,expected],[dict(expected,prefix_records=float(count))],[dict(expected,start_permitted=0)],
                      [dict(expected,after_cut="stopped")],[dict(expected,observation="generic-failure")],
                      [dict(expected,other_bounds="ignored")],[dict(expected,failure_header_permitted=True)]):
        rejection(lambda:predict({"release_expiry":inventory},operation,count))
files={name:zero for name in sorted(loaded["SOURCE_EXPIRY_FILES"])}
before_profile={"schema":"source-release-expiry-observer-v1","public_receipt_sha256":zero,
                "distribution":"/distribution","history":"/history","network":"one-network","phase":"before","files":files}
after_profile=dict(before_profile,phase="expired")
def pin(profile): return hashlib.sha256(json.dumps(profile,separators=(",",":")).encode()+b"\n").hexdigest()
bounds={"root.json":"2030-01-01T01:00:00Z","snapshot.json":"2030-01-01T01:00:00Z","targets.json":"2030-01-01T01:00:00Z"}
for target in ("ardents/linux-amd64/node","ardents/linux-amd64/infrastructure/source"):
    for suffix in ("/no-new","/terminate"): bounds[target+suffix]="2030-01-01T01:00:00Z"
before={"profile_sha256":pin(before_profile),"public_receipt_sha256":zero,"phase":"before","reference_time":"2030-01-01T00:00:01Z",
        "timestamp_expires":"2030-01-01T00:00:02Z","other_bounds":bounds,"files":files,"outcomes":["no-update","no-update"],
        "authorized":[True,True],"floors_unchanged":True,"verifier_closed":True,"installed_execution":True}
after=dict(before,profile_sha256=pin(after_profile),phase="expired",reference_time="2030-01-01T00:00:03Z",
           outcomes=["release-expired","release-expired"],authorized=[False,False])
assert check(before_profile,before)["reference_time"]==before["reference_time"]
assert check(after_profile,after,before)["timestamp_expires"]==before["timestamp_expires"]
rejection(lambda:check(after_profile,after))
rejection(lambda:check(before_profile,before,before))
for profile in (None,{},dict(before_profile,reference_time=before["reference_time"]),dict(before_profile,schema="source-recovery-refusal-observer-v1"),
                dict(before_profile,files={}),dict(before_profile,files=dict(files,unknown=zero)),dict(before_profile,phase="renew")):
    rejection(lambda:check(profile,before))
for report in (None,{},dict(before,unknown=True)):
    rejection(lambda:check(before_profile,report))
for field,value in (("profile_sha256",other),("installed_execution",False),("installed_execution",1),("floors_unchanged",1),
                    ("verifier_closed",False),("public_receipt_sha256",other),("phase","expired"),
                    ("reference_time","2030-01-01T00:00:02Z"),("reference_time",123),("reference_time","2030-01-01T00:00:01"),
                    ("files",dict(files,**{"timestamp.json":other})),("outcomes",["no-update","release-expired"]),("authorized",[1,True])):
    rejection(lambda:check(before_profile,dict(before,**{field:value})))
for field,value in (("profile_sha256",other),("phase","before"),("reference_time",before["reference_time"]),
                    ("timestamp_expires","2030-01-01T00:00:01Z"),("files",dict(files,**{"timestamp.json":other})),
                    ("outcomes",["release-expired","no-update"]),("authorized",[False,0]),("verifier_closed",False)):
    rejection(lambda:check(after_profile,dict(after,**{field:value}),before))
for name in bounds:
    for instant in (before["timestamp_expires"],after["reference_time"],"2029-01-01T00:00:00Z",None):
        rejection(lambda:check(after_profile,dict(after,other_bounds=dict(bounds,**{name:instant})),before))
changed=dict(bounds,**{"root.json":"2030-01-01T02:00:00Z"})
rejection(lambda:check(after_profile,dict(after,other_bounds=changed),before))
rejection(lambda:check(after_profile,dict(after,other_bounds={}),before))
rejection(lambda:check(after_profile,dict(after,other_bounds=dict(bounds,extra="2030-01-01T01:00:00Z")),before))
changed_profile=dict(after_profile,history="/other-history")
rejection(lambda:check(changed_profile,dict(after,profile_sha256=pin(changed_profile)),before))
for count in (True,70.0,71): rejection(lambda:predict({},"initial",count))
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source Release actual wall-expiry observation grammar: %v\n%s", err, output)
	}
}

func TestSourceCompletedClassificationRequiresSeparateReadOnlyPrediction(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import runpy,sys
check=runpy.run_path(sys.argv[1])["source_completed_prediction"]
def rejection(call):
    try: call()
    except RuntimeError: return
    raise AssertionError("missing or widened completed classification accepted")
for attempt in range(4):
    for kind in ("startup","retirement"):
        expected={"attempt":attempt,"journal_kind":kind,"startup_records":22,"retirement_records":12,
                  "after":"stopped-selected","persistent_effects":"none","start_permitted":False}
        assert check({"completed_classifications":[expected]},attempt,kind)==expected
        for inventory in (None,[],[expected,expected],[dict(expected,attempt=float(attempt))],
                          [dict(expected,start_permitted=0)],[dict(expected,start_permitted=True)],
                          [dict(expected,startup_records=21)],[dict(expected,retirement_records=11)],
                          [dict(expected,after="started")],[dict(expected,persistent_effects="append")],
                          [dict(expected,journal_kind="retirement" if kind=="startup" else "startup")],
                          [dict(expected,unknown=True)]):
            rejection(lambda:check({"completed_classifications":inventory},attempt,kind))
        rejection(lambda:check({"startup_cuts":[expected]},attempt,kind))
        rejection(lambda:check({"retirement_cuts":[expected]},attempt,kind))
for attempt in (True,-1,4,0.0):
    rejection(lambda:check({},attempt,"startup"))
for kind in (None,"initial","replacement","start","stop"):
    rejection(lambda:check({},0,kind))
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source completed classification read-only prediction grammar: %v\n%s", err, output)
	}
}

func TestSourceCompletedClassificationRetainsManagerFactsAcrossAvailableMemoryChanges(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import ast,copy,runpy,sys
from pathlib import Path
path=Path(sys.argv[1]);loaded=runpy.run_path(str(path));tree=ast.parse(path.read_bytes())
main=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=="main")
owner=next(n for n in main.body if isinstance(n,ast.FunctionDef) and n.name=="completed_classification")
capture=next(n for n in owner.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=="original_facts" for t in n.targets))
guard=next(n for n in ast.walk(owner) if isinstance(n,ast.If) and any(isinstance(s,ast.Raise) and isinstance(s.exc,ast.Call)
    and s.exc.args and isinstance(s.exc.args[0],ast.Constant) and s.exc.args[0].value=="read-only completed classification changed retained history or stopped manager" for s in n.body))
before={"Manager":":1.0","Version":"255.4","UnitPaths":["/etc/systemd/system"],
        "Unit":{"ActiveState":{"type":"s","data":"inactive"},"InvocationID":{"type":"ay","data":[1,2]}},
        "Service":{"MemoryAvailable":{"type":"t","data":32317673472},"MemoryMax":{"type":"t","data":1048576},
                   "MainPID":{"type":"u","data":0},"ExecMainPID":{"type":"u","data":395},
                   "ExecMainExitTimestamp":{"type":"t","data":123},"ExecMainStatus":{"type":"i","data":0}}}

def check(first,second,*,history="same",manager=":1.0"):
    class Observer:
        def __init__(self): self.samples=iter((first,second))
        def sample(self): return next(self.samples)
    scope=dict(loaded,native_observer=Observer(),retained_completed=lambda:history,retained="same",
               manager_owner=lambda:manager,original_manager=":1.0")
    for statement in (capture,guard):
        exec(compile(ast.fix_missing_locations(ast.Module(body=[statement],type_ignores=[])),str(path),"exec"),scope)

def reject(first,second,**kw):
    try: check(first,second,**kw)
    except RuntimeError: return
    raise AssertionError("completed classification accepted changed or malformed original facts")

after=copy.deepcopy(before);after["Service"]["MemoryAvailable"]["data"]+=4096
saved=copy.deepcopy((before,after))
check(before,after);check(before,before)
assert (before,after)==saved,"projection mutated retained raw observations"
for section in ("Unit","Service"):
    for key in before[section]:
        if key=="MemoryAvailable": continue
        changed=copy.deepcopy(after);changed[section][key]={"type":"s","data":"foreign"}
        reject(before,changed)
for key,value in (("Manager",":1.99"),("Version","other"),("UnitPaths",["/foreign"]),("unknown",True)):
    changed=copy.deepcopy(after);changed[key]=value;reject(before,changed)
reject(before,after,history="changed");reject(before,after,manager=":1.99")
for value in (None,{}, {"type":"u","data":1},{"type":"t","data":True},{"type":"t","data":-1},
              {"type":"t","data":1<<64},{"type":"t","data":1,"extra":True}):
    changed=copy.deepcopy(before);changed["Service"]["MemoryAvailable"]=value
    reject(changed,changed);reject(before,changed)
missing=copy.deepcopy(before);del missing["Service"]["MemoryAvailable"]
reject(missing,missing);reject(before,missing)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("completed classification confused available memory with original manager facts: %v\n%s", err, output)
	}
}

func TestSourceRetirementCutRequiresOriginalArchiveAndSeparatePrediction(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
select,predict,observe=(loaded[name] for name in ("source_profile","source_retirement_prediction","source_retirement_cut_observation"))
zero,other="0"*64,"1"*64
def encode(value): return json.dumps(value,separators=(",",":")).encode()+b"\n"
def rejection(call):
    try: call()
    except RuntimeError: return
    raise AssertionError("mixed or incomplete retirement evidence accepted")
for attempt in (0,1):
    for window,count in (("native-before-joined",6),("result-before-close",12),("pending-before-unlink",12)):
        chosen=select(["-timeout=10m"],f"recovery-retirement-{attempt}-{window}")
        assert chosen["retirement_physical"]=={"attempt":attempt,"window":window}
        assert chosen["recovery"] is True and chosen["replacement"] is False and chosen["restart"] is True
        assert chosen["operation"]=="retirement" and chosen["case"]=="restart"
        assert all(chosen[name] is None for name in ("prefix_records","fault","physical","root_physical","startup_physical"))
        assert chosen["concurrent"] is False
        expected={"attempt":attempt,"window":window,"prefix_records":count,"root_signal":"SIGKILL",
                  "source_join":"original-pidfd-and-cgroup-before-root-cut","pending":"retained",
                  "result":"absent" if count==6 else "complete-retained","after_cut":"repair-required","start_permitted":False}
        assert predict({"retirement_cuts":[expected]},attempt,window)==expected
        for inventory in (None,[],[expected,expected],[dict(expected,attempt=float(attempt))],[dict(expected,start_permitted=0)],
                          [dict(expected,source_join="tracer-reaps")],[dict(expected,pending="removed")],
                          [dict(expected,after_cut="stopped")],[dict(expected,result="complete-retained" if count==6 else "absent")]):
            rejection(lambda:predict({"retirement_cuts":inventory},attempt,window))
        rejection(lambda:predict({"startup_cuts":[expected]},attempt,window))
        archive={"schema":"ardents-source-first-start-archive-v1" if attempt==0 else "ardents-source-invocation-archive-v1",
                 "guard_sha256":other,"binding_sha256":zero,"generation":other,"pid":80,"invocation_id":"1"*32,
                 "process_device":5,"process_inode":6,"cgroup_device":7,"cgroup_inode":8,"manager":"broker/:1.2",
                 "started_realtime":100,"started_monotonic":200}
        profile={"schema":"source-retirement-physical-cut-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
                 "attempt":attempt,"window":window,"prefix_records":count,"last_phase":"domain-joined" if count==6 else "retired",
                 "generation":other,"binding_sha256":zero,"start_sha256":hashlib.sha256(encode(archive)).hexdigest(),"start_journal_sha256":other}
        pin=hashlib.sha256(encode(profile)).hexdigest()
        ready={"entry":1000,"root_pid":77,"held_root_thread":78,"selected_records":count,"startup_archive":archive,
               "profile_sha256":pin,"program_sha256":zero,"public_receipt_sha256":zero,"physical_window":window,
               "inspection_only":False,"installed_execution":True,"external_cut_ready":True,"root_only_at_physical_cut":True,"root_exit":-1,"selected_journal_sha256":other}
        result={"schema":"ardents-source-retirement-v1","attempt":attempt,"generation":other,"binding_sha256":zero,
                "start_sha256":profile["start_sha256"],"invocation_id":archive["invocation_id"],"pid":80,"manager":archive["manager"],
                "challenge":"2"*64,"completion_sha256":"", "started_realtime":100,"started_monotonic":200,
                "exited_realtime":101,"exited_monotonic":201,"code":1,"status":0}
        frame=b"ardents-source-domain-joined-v1\n"+(archive["invocation_id"]+other+zero).encode()+bytes.fromhex(result["challenge"])
        result["completion_sha256"]=hashlib.sha256(frame).hexdigest()
        if count==12: ready["retirement_result"]=result
        assert observe(profile,ready)["root_pid"]==77
        report=dict(ready,external_cut_requested=True,root_signal=9,processes=1,peak_live_processes=1,threads_joined=5,root_kill_requested=True,
                    root_killed_by_observer=True,original_cut_sha256="",output_sha256=hashlib.sha256(b"").hexdigest())
        assert observe(profile,ready,report,b"")["root_signal"]==9
        assert observe(profile,ready,dict(report,processes=41,threads_joined=45,peak_live_processes=2),b"")["root_signal"]==9
        for missing in ("root_only_at_physical_cut","processes","peak_live_processes","threads_joined"):
            incomplete=dict(report);del incomplete[missing]
            try: observe(profile,ready,incomplete,b"")
            except RuntimeError: pass
            else: raise AssertionError("missing physical root inventory/proof admitted: "+missing)
        for field,value in (("root_only_at_physical_cut",False),("root_only_at_physical_cut",1),("root_pid",True),("entry",0),("selected_records",count+1),("profile_sha256",other),
                            ("installed_execution",False),("external_cut_ready",False),("external_cut_requested",True),
                            ("root_exit",0),("root_signal",True),("root_kill_requested",True),("startup_archive",{}),
                            ("retirement_result",result if count==6 else None),("refusal_startup_pending_observed",True),
                            ("refusal_retirement_pending_observed",True),("refusal_exclusion_observed",True)):
            rejection(lambda:observe(profile,dict(ready,**{field:value})))
        for field,value in (("pid",True),("process_inode",0),("cgroup_inode",0),("started_realtime",False),
                            ("invocation_id","0"*32),("generation",zero),("binding_sha256",other),("schema","foreign"),
                            ("manager",""),("guard_sha256",zero)):
            rejection(lambda:observe(profile,dict(ready,startup_archive=dict(archive,**{field:value}))))
        if count==12:
            for field,value in (("attempt",float(attempt)),("pid",True),("manager","foreign"),("start_sha256",zero),
                                ("challenge","0"*64),("challenge","F"*64),("completion_sha256",zero),("code",0),
                                ("status",True),("exited_realtime",99),("exited_monotonic",199),("unknown",1)):
                rejection(lambda:observe(profile,dict(ready,retirement_result=dict(result,**{field:value}))))
            rejection(lambda:observe(profile,ready,dict(report,retirement_result=dict(result,pid=80.0)),b""))
        rejection(lambda:observe(profile,ready,dict(report,startup_archive=dict(archive,pid=80.0)),b""))
        for field,value in (("schema","source-startup-physical-cut-observer-v1"),("attempt",True),("window","ack-before-written"),
                            ("prefix_records",count+1),("last_phase","archived"),("start_sha256",zero),("start_journal_sha256","")):
            changed=dict(profile,**{field:value})
            rejection(lambda:observe(changed,dict(ready,profile_sha256=hashlib.sha256(encode(changed)).hexdigest())))
        for field,value in (("held_root_thread",79),("selected_journal_sha256",zero),("startup_archive",{}),
                            ("root_only_at_physical_cut",False),("root_only_at_physical_cut",1),("external_cut_requested",False),("root_signal",15),("root_exit",0),("processes",2),
                            ("peak_live_processes",True),("peak_live_processes",0),("peak_live_processes",2),("threads_joined",True),("threads_joined",0),("root_killed_by_observer",False),
                            ("root_ownership_released_before_exit",True),("root_close_entry",123),
                            ("external_cancellation_acknowledged",True),("refusal_exclusion_observed",True),
                            ("refusal_startup_pending_observed",True),("refusal_retirement_pending_observed",True),
                            ("original_cut_sha256",other),("output_sha256",other)):
            rejection(lambda:observe(profile,ready,dict(report,**{field:value}),b""))
        for raw in (b'source command failed\n',b'{"operation":"installation.source","outcome":"completion-unavailable"}\n'):
            rejection(lambda:observe(profile,ready,dict(report,output_sha256=hashlib.sha256(raw).hexdigest()),raw))
for name in ("recovery-retirement-2-native-before-joined","recovery-retirement-0-retired","recovery-retirement-1-ack-before-written"):
    rejection(lambda:select(["-timeout=10m"],name))
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source retirement cut prediction/archive/root observation grammar: %v\n%s", err, output)
	}
}

func TestSourceStartupCutRequiresSeparatePredictionAndRootJoin(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
select,predict,observe=(loaded[name] for name in ("source_profile","source_startup_prediction","source_startup_cut_observation"))
zero,other="0"*64,"1"*64
for attempt in (0,1):
    for window,count in (("archive-before-archived",19),("ack-before-written",21)):
        selected=select(["-timeout=10m"],f"recovery-startup-{attempt}-{window}")
        assert selected["startup_physical"]=={"attempt":attempt,"window":window}
        assert selected["recovery"] is True and selected["replacement"] is False and selected["restart"] is True
        assert selected["operation"]=="startup" and selected["case"]=="restart" and selected["prefix_records"] is None
        assert selected["fault"] is None and selected["physical"] is None and selected["root_physical"] is None and selected["concurrent"] is False
        expected={"attempt":attempt,"window":window,"prefix_records":count,"root_signal":"SIGKILL","source_join":"original-pidfd-and-cgroup","pending":"retained","after_cut":"repair-required","start_permitted":False}
        assert predict({"startup_cuts":[expected]},attempt,window)==expected
        for inventory in (None,[],[expected,expected],[dict(expected,attempt=float(attempt))],[dict(expected,start_permitted=0)],
                          [dict(expected,source_join="tracer-reaps")],[dict(expected,pending="removed")],[dict(expected,after_cut="stopped")]):
            try: predict({"startup_cuts":inventory},attempt,window)
            except RuntimeError: pass
            else: raise AssertionError("missing or widened startup prediction accepted")
        profile={"schema":"source-startup-physical-cut-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
                 "attempt":attempt,"window":window,"generation":other,"binding_sha256":zero}
        pin=hashlib.sha256(json.dumps(profile,separators=(",",":")).encode()+b"\n").hexdigest()
        archive={"schema":"ardents-source-first-start-archive-v1" if attempt==0 else "ardents-source-invocation-archive-v1",
                 "generation":other,"binding_sha256":zero,"pid":80,"invocation_id":"1"*32,
                 "process_device":5,"process_inode":6,"cgroup_device":7,"cgroup_inode":8,
                 "started_realtime":100,"started_monotonic":200}
        ready={"entry":1000,"root_pid":77,"held_root_thread":78,"selected_records":count,"startup_archive":archive,
               "profile_sha256":pin,"program_sha256":zero,"public_receipt_sha256":zero,"physical_window":window,
               "inspection_only":False,"installed_execution":True,"external_cut_ready":True,"root_only_at_physical_cut":True,"root_exit":-1,"selected_journal_sha256":other}
        assert observe(profile,ready)["root_pid"]==77
        report=dict(ready,external_cut_requested=True,root_signal=9,processes=1,peak_live_processes=1,threads_joined=5,root_kill_requested=True,
                    root_killed_by_observer=True,original_cut_sha256="",output_sha256=hashlib.sha256(b"").hexdigest())
        assert observe(profile,ready,report,b"")["root_signal"]==9
        assert observe(profile,ready,dict(report,processes=41,threads_joined=45,peak_live_processes=2),b"")["root_signal"]==9
        for missing in ("root_only_at_physical_cut","processes","peak_live_processes","threads_joined"):
            incomplete=dict(report);del incomplete[missing]
            try: observe(profile,ready,incomplete,b"")
            except RuntimeError: pass
            else: raise AssertionError("missing physical root inventory/proof admitted: "+missing)
        for field,value in (("root_only_at_physical_cut",False),("root_only_at_physical_cut",1),("root_pid",True),("entry",0),("selected_records",count+1),("profile_sha256",other),
                            ("installed_execution",False),("external_cut_ready",False),("external_cut_requested",True),
                            ("root_exit",0),("root_signal",True),("root_kill_requested",True),("startup_archive",{}),("retirement_result",{}),
                            ("refusal_startup_pending_observed",True)):
            try: observe(profile,dict(ready,**{field:value}))
            except RuntimeError: pass
            else: raise AssertionError("invalid held startup observation accepted: "+field)
        for field,value in (("pid",True),("process_inode",0),("cgroup_inode",0),("started_realtime",False),
                            ("invocation_id","0"*32),("generation",zero),("binding_sha256",other),("schema","foreign")):
            try: observe(profile,dict(ready,startup_archive=dict(archive,**{field:value})))
            except RuntimeError: pass
            else: raise AssertionError("invalid held archive accepted: "+field)
        for field,value in (("held_root_thread",79),("selected_journal_sha256",zero),("startup_archive",{}),("retirement_result",{}),
                            ("root_only_at_physical_cut",False),("root_only_at_physical_cut",1),("external_cut_requested",False),("root_signal",15),("root_exit",0),("processes",2),
                            ("peak_live_processes",True),("peak_live_processes",0),("peak_live_processes",2),("threads_joined",True),("threads_joined",0),("root_killed_by_observer",False),
                            ("root_ownership_released_before_exit",True),("root_close_entry",123),
                            ("external_cancellation_acknowledged",True),("refusal_exclusion_observed",True),
                            ("refusal_startup_pending_observed",True),("original_cut_sha256",other),("output_sha256",other)):
            try: observe(profile,ready,dict(report,**{field:value}),b"")
            except RuntimeError: pass
            else: raise AssertionError("mixed/incomplete startup root reap accepted: "+field)
        for raw in (b'source command failed\n',b'{"operation":"installation.source","outcome":"completion-unavailable"}\n'):
            try: observe(profile,ready,dict(report,output_sha256=hashlib.sha256(raw).hexdigest()),raw)
            except RuntimeError: pass
            else: raise AssertionError("graceful result replaced root cut")
for name in ("recovery-startup-2-archive-before-archived","recovery-startup-0-archived","recovery-startup-1-ack-written"):
    try: select(["-timeout=10m"],name)
    except RuntimeError: pass
    else: raise AssertionError("unselected startup physical variant accepted")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source startup cut prediction/root observation grammar: %v\n%s", err, output)
	}
}

func TestSourceRootRecoveryRequiresSeparateCutPredictionAndObservation(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,signal,sys
# Detached Linux reports must also validate on hosts without this signal.
if hasattr(signal,"SIGKILL"):
    del signal.SIGKILL
loaded=runpy.run_path(sys.argv[1])
select,predict,observe=(loaded[name] for name in ("source_profile","source_root_recovery_prediction","source_root_recovery_observation"))
zero,other="0"*64,"1"*64
for operation,counts in (("initial",(70,115)),("replacement",(13,55))):
    for count in counts:
        for window in (("repaired-body-before-record",) if count in (70,13) else ())+("completion-before-close","pending-before-unlink"):
            case=select(["-timeout=10m"],f"recovery-root-{operation}-{count}-{window}")
            assert (case["operation"],case["prefix_records"],case["root_physical"])==(operation,count,window)
            assert case["physical"] is None and case["concurrent"] is False and case["fault"] is None
            expected={"operation":operation,"prefix_records":count,"window":window,"root_signal":"SIGKILL","completion":"partial-retained-no-append" if window=="repaired-body-before-record" else "complete-retained","pending":"retained","after_cut":"repair-required","start_permitted":False}
            assert predict({"root_recovery_cuts":[expected]},operation,count,window)==expected
            for inventory in (None,[],[expected,expected],[dict(expected,start_permitted=True)],[dict(expected,start_permitted=0)],
                              [dict(expected,pending="absent")],[dict(expected,completion="adoptable")],[dict(expected,prefix_records=float(count))]):
                try: predict({"root_recovery_cuts":inventory},operation,count,window)
                except RuntimeError: pass
                else: raise AssertionError("missing/widened root prediction admitted")
            profile={"schema":"source-recovery-physical-cut-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
                     "arguments":["installation","recover-source","/distribution","/history","alpha","network","/request","/journal"],
                     "window":window,"prefix_records":count,"journal_sha256":other}
            pin=hashlib.sha256(json.dumps(profile,separators=(",",":")).encode()+b"\n").hexdigest()
            report={"entry":1234,"root_pid":77,"root_exit":-1,"root_signal":9,"threads_joined":8,"processes":1,"peak_live_processes":1,"root_only_at_physical_cut":True,
                    "held_root_thread":79,"signalled_root_thread":0,"selected_records":count,"profile_sha256":pin,
                    "program_sha256":zero,"public_receipt_sha256":zero,"selected_journal_sha256":other,
                    "physical_window":window,"inspection_only":False,"installed_execution":True,
                    "root_kill_requested":True,"root_killed_by_observer":True,"external_cancellation_acknowledged":False,
                    "original_cut_sha256":"","output_sha256":hashlib.sha256(b"").hexdigest()}
            assert observe(profile,report,b"")["root_signal"]==9
            assert observe(profile,dict(report,processes=41,threads_joined=45,peak_live_processes=2),b"")["processes"]==41
            for missing in ("root_only_at_physical_cut","processes","peak_live_processes","threads_joined"):
                incomplete=dict(report);del incomplete[missing]
                try: observe(profile,incomplete,b"")
                except RuntimeError: pass
                else: raise AssertionError("missing root scope/native inventory admitted: "+missing)
            for field,value in (("entry",True),("root_pid",0),("root_exit",0),("root_signal",15),("threads_joined",0),
                                ("root_only_at_physical_cut",False),("root_only_at_physical_cut",1),("threads_joined",257),("processes",2),("held_root_thread",0),("signalled_root_thread",78),
                                ("selected_records",count+1),("profile_sha256",other),("program_sha256",other),
                                ("public_receipt_sha256",other),("selected_journal_sha256",zero),("physical_window","other"),
                                ("inspection_only",True),("installed_execution",False),("root_kill_requested",False),
                                ("root_killed_by_observer",False),("external_cancellation_acknowledged",True),
                                ("root_ownership_released_before_exit",True),("root_close_entry",100),("root_close_observations",True),
                                ("refusal_ownership_released_before_exit",True),("recovery_held_before_record",True),("original_cut_sha256",zero),
                                ("external_cut_ready",True),("external_cut_requested",True),("startup_archive",{}),("retirement_result",{})):
                try: observe(profile,dict(report,**{field:value}),b"")
                except RuntimeError: pass
                else: raise AssertionError("mixed/incomplete root cut admitted: "+field)
            for raw in (b'source command failed\n',b'{"operation":"installation.source","outcome":"completion-unavailable"}\n'):
                try: observe(profile,dict(report,output_sha256=hashlib.sha256(raw).hexdigest()),raw)
                except RuntimeError: pass
                else: raise AssertionError("graceful completion/cancellation substituted for process cut")
for name in ("recovery-root-initial-106-completion-before-close","recovery-root-replacement-49-pending-before-unlink","recovery-root-initial-70-file-ready","recovery-root-initial-115-repaired-body-before-record","recovery-root-replacement-55-repaired-body-before-record"):
    try: select(["-timeout=10m"],name)
    except RuntimeError: pass
    else: raise AssertionError("unselected root recovery variant admitted")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source root recovery prediction/observation grammar: %v\n%s", err, output)
	}
}

// Detached report grammar controls. A syntactically complete claim below is
// not an installed Source, manager, physical cleanup or accepting authority.
func TestSourceRefusalObservationRejectsIncompleteOrSubstitutedClaims(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib, json, runpy, sys
check = runpy.run_path(sys.argv[1])["source_refusal_observation"]
zero, other = "0" * 64, "1" * 64
profile = {"schema":"source-recovery-refusal-observer-v1", "program_sha256":zero, "public_receipt_sha256":zero,
           "arguments":["installation","recover-source","/unopened/distribution","/unopened/history","alpha","one-network","/unopened/request","/unopened/source.source-initial-journal"],
           "journal_sha256":zero, "journal_mode":"0600", "external_journal_lease":False, "external_release_lease":False}
raw = b"source command failed\n"
report = {"profile_sha256":hashlib.sha256(json.dumps(profile,separators=(",",":")).encode()+b"\n").hexdigest(),
          "inspection_only":False, "installed_execution":True, "refusal_admission_observed":True,
          "refusal_ownership_released_before_exit":True, "refusal_original_scope_joined":True, "entry":1000, "refusal_admission_entry":2000,
          "root_pid":77, "root_exit":2, "processes":1,"peak_live_processes":1, "refusal_stopped_threads":4, "threads_joined":4,
          "program_sha256":zero, "public_receipt_sha256":zero, "output_sha256":hashlib.sha256(raw).hexdigest()}
check(profile, report, raw)
joined = check(profile, dict(report, processes=41, threads_joined=45, peak_live_processes=2), raw)
assert joined["original_scope_joined"] is True and joined["processes"] == 41 and joined["peak_live_processes"] == 2
assert check(profile, dict(report, processes=32, threads_joined=40, peak_live_processes=32), raw)["peak_live_processes"] == 32
for altered in (dict(report, processes=2), dict(report, processes=33, threads_joined=40, peak_live_processes=33),
                dict(report, processes=257, threads_joined=257, peak_live_processes=2)):
    try:
        check(profile, altered, raw)
    except RuntimeError:
        pass
    else:
        raise AssertionError("contradictory peak or actual live/lifetime bound admitted")
for field, value in (
    ("inspection_only",True), ("installed_execution",False), ("refusal_admission_observed",False),
    ("refusal_ownership_released_before_exit",False), ("entry",0), ("entry",True), ("refusal_admission_entry",1000),
    ("root_pid",0), ("root_exit",0), ("root_exit",True), ("processes",0), ("processes",33), ("processes",True), ("peak_live_processes",True), ("peak_live_processes",0), ("peak_live_processes",33), ("peak_live_processes",2),
    ("processes",5), ("refusal_original_scope_joined",False), ("refusal_original_scope_joined",1),
    ("recovery_root_only_at_hold",True), ("refusal_stopped_threads",1),
    ("threads_joined",3), ("threads_joined",257), ("program_sha256",other), ("public_receipt_sha256",other),
    ("profile_sha256",other), ("output_sha256",other),
    ("root_only_at_physical_cut",True),("external_cut_ready",True),("external_cut_requested",True),("startup_archive",{}),("retirement_result",{}),
):
    altered = dict(report, **{field:value})
    try:
        check(profile, altered, raw)
    except RuntimeError:
        pass
    else:
        raise AssertionError("incomplete/substituted report claim admitted: " + field)
for field in ("peak_live_processes", "entry", "refusal_admission_observed", "refusal_ownership_released_before_exit", "refusal_original_scope_joined"):
    altered = dict(report)
    del altered[field]
    try:
        check(profile, altered, raw)
    except RuntimeError:
        pass
    else:
        raise AssertionError("missing original observation admitted: " + field)
changed = dict(profile, journal_mode="0640")
try:
    check(changed, report, raw)
except RuntimeError:
    pass
else:
    raise AssertionError("report for a different original profile admitted")
wrong_output = b"incomplete output\n"
consistent = dict(report, output_sha256=hashlib.sha256(wrong_output).hexdigest())
try:
    check(profile, consistent, wrong_output)
except RuntimeError:
    pass
else:
    raise AssertionError("missing final canonical refusal accepted with matching output hash")
for journal, release in ((True,False),(False,True),(True,True)):
    selected = dict(profile, external_journal_lease=journal, external_release_lease=release)
    kind = "release" if release else "journal"
    declared = dict(report, profile_sha256=hashlib.sha256(json.dumps(selected,separators=(",",":")).encode()+b"\n").hexdigest(),
                    refusal_exclusion_observed=True, refusal_exclusion_kind=kind,
                    refusal_exclusion_device=55, refusal_exclusion_inode=66)
    summary = check(selected, declared, raw, (55,66))
    assert summary["exclusion"] == {"observed":True,"kind":kind,"device":55,"inode":66}
    for identity in (None,(55,67),(54,66),(True,66),(55,True),(55,66.0),(0,66)):
        try:
            check(selected, declared, raw, identity)
        except RuntimeError:
            pass
        else:
            raise AssertionError("foreign/missing independent exclusion identity admitted")
    for field,value in (("refusal_exclusion_observed",False),("refusal_exclusion_kind","other"),
                        ("refusal_exclusion_device",54),("refusal_exclusion_inode",67),("refusal_exclusion_inode",True)):
        try:
            check(selected, dict(declared, **{field:value}), raw, (55,66))
        except RuntimeError:
            pass
        else:
            raise AssertionError("missing/substituted kernel exclusion claim admitted: "+field)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source refusal observation grammar: %v\n%s", err, output)
	}
}

func TestSourceEarlyPrefixObservationRequiresFreshNativeCustody(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
check,select=loaded["source_refusal_observation"],loaded["source_profile"]
zero,other="0"*64,"1"*64
raw=b"source command failed\n"
base={"schema":"source-early-prefix-recovery-refusal-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
      "arguments":["installation","recover-source","/distribution","/history","alpha","one-network","/request","/source.source-initial-journal"],
      "journal_sha256":zero,"journal_mode":"0600","external_journal_lease":False,"external_release_lease":False}
report={"inspection_only":False,"installed_execution":True,"refusal_admission_observed":True,
        "refusal_ownership_released_before_exit":True, "refusal_original_scope_joined":True,"entry":1000,"refusal_admission_entry":2000,"root_pid":77,
        "root_exit":2,"processes":1,"peak_live_processes":1,"refusal_stopped_threads":4,"threads_joined":4,"program_sha256":zero,
        "public_receipt_sha256":zero,"output_sha256":hashlib.sha256(raw).hexdigest(),"refusal_early_prefix_observed":True,
        "refusal_early_prefix_closed":True,"refusal_early_prefix_device":55,"refusal_early_prefix_inode":66,"refusal_early_prefix_sha256":zero}
def pinned(p,r):
    return dict(r,profile_sha256=hashlib.sha256(json.dumps(p,separators=(",",":")).encode()+b"\n").hexdigest())
def reject(p,r,identity=(55,66,zero),**kwargs):
    try: check(p,pinned(p,r),raw,early_prefix_identity=identity,**kwargs)
    except RuntimeError: return
    raise AssertionError("missing or borrowed early-prefix custody accepted")
for operation,count,window in (("initial",24,"account-before-complete-birth"),("initial",42,"directory-born-before-record"),("initial",69,"file-born-before-record"),("replacement",12,"file-born-before-record")):
    case=select(["-timeout=10m"],"recovery-physical-"+operation+"-"+window)
    assert case["prefix_records"]==count and case["physical"]==window
    if count in (24,42):
        try: select(["-timeout=10m"],"recovery-physical-replacement-"+window)
        except RuntimeError: pass
        else: raise AssertionError("initial-only effect admitted as replacement")
    else:
        assert select(["-timeout=10m"],"recovery-physical-replacement-"+window)["replacement"] is True
    purpose_field="early_replacement_prefix" if operation=="replacement" else "early_initial_prefix"
    p,r=dict(base,**{purpose_field:count}),dict(report,refusal_early_prefix_records=count)
    p["arguments"]=list(base["arguments"])
    if operation=="replacement":p["arguments"][7]="/source.source-replacement-"+zero+"-journal"
    reject(dict(p,early_initial_prefix=24,early_replacement_prefix=12),r)
    assert check(p,pinned(p,r),raw,early_prefix_identity=(55,66,zero))["early_prefix"]=={
        "observed":True,"closed":True,"records":count,"device":55,"inode":66,"sha256":zero}
    for field in ("refusal_early_prefix_observed","refusal_early_prefix_closed","refusal_early_prefix_records",
                  "refusal_early_prefix_device","refusal_early_prefix_inode","refusal_early_prefix_sha256"):
        changed=dict(r)
        del changed[field]
        reject(p,changed)
    for field,value in (("refusal_early_prefix_observed",1),("refusal_early_prefix_closed",False),
                        ("refusal_early_prefix_records",float(count)),("refusal_early_prefix_records",count+1),
                        ("refusal_early_prefix_device",True),("refusal_early_prefix_inode",66.0),
                        ("refusal_early_prefix_inode",67),("refusal_early_prefix_sha256",other),
                        ("refusal_exclusion_observed",True),("refusal_missing_cut_observed",True)):
        reject(p,dict(r,**{field:value}))
    for field,value in (("schema","source-recovery-refusal-observer-v1"),(purpose_field,count+1),
                        (purpose_field,0),(purpose_field,float(count)),("journal_mode","0640"),
                        ("external_journal_lease",True),("external_release_lease",True),("missing_original_cut",True),
                        ("startup_pending",{}),("retirement_pending",{}),("recovery_hold",{}),
                        ("arguments",[]),("journal_sha256",other)):
        reject(dict(p,**{field:value}),r)
    for identity in (None,(55,66),(True,66,zero),(55,66.0,zero),(55,67,zero),(55,66,other),(55,66,1)):
        reject(p,r,identity)
    reject(p,r,missing_cut_parent_identity=(55,66))
    reject(p,r,exclusion_identity=(55,66))
    plain=dict(p,schema="source-recovery-refusal-observer-v1",**{purpose_field:0})
    reject(plain,r,None)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source early-prefix native custody grammar: %v\n%s", err, output)
	}
}

func TestSourceMissingCutObservationRequiresOriginalCauseAndParent(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib, json, runpy, sys
check = runpy.run_path(sys.argv[1])["source_refusal_observation"]
zero = "0"*64
profile = {"schema":"source-recovery-missing-cut-observer-v1", "program_sha256":zero, "public_receipt_sha256":zero,
           "journal_mode":"0600", "external_journal_lease":False, "external_release_lease":False, "missing_original_cut":True}
raw = b"source command failed\n"
report = {"inspection_only":False, "installed_execution":True, "refusal_admission_observed":True,
          "refusal_ownership_released_before_exit":True, "refusal_original_scope_joined":True, "entry":1000, "refusal_admission_entry":2000,
          "root_pid":77, "root_exit":2, "processes":1,"peak_live_processes":1, "refusal_stopped_threads":4, "threads_joined":4,
          "program_sha256":zero, "public_receipt_sha256":zero, "output_sha256":hashlib.sha256(raw).hexdigest(),
          "refusal_missing_cut_observed":True, "refusal_missing_cut_parent_device":55, "refusal_missing_cut_parent_inode":66}
def pinned(p, r):
    return dict(r, profile_sha256=hashlib.sha256(json.dumps(p,separators=(",",":")).encode()+b"\n").hexdigest())
def reject(p, r, identity=(55,66)):
    try:
        check(p, pinned(p,r), raw, missing_cut_parent_identity=identity)
    except RuntimeError:
        return
    raise AssertionError("missing/foreign original-cut cause admitted")
summary = check(profile, pinned(profile,report), raw, missing_cut_parent_identity=(55,66))
assert summary["missing_original_cut"] == {"observed":True,"parent_device":55,"parent_inode":66}
for identity in (None,(54,66),(55,67),(True,66),(55,True),(55,66.0),(0,66)):
    reject(profile,report,identity)
for field in ("refusal_missing_cut_observed","refusal_missing_cut_parent_device","refusal_missing_cut_parent_inode"):
    altered = dict(report)
    del altered[field]
    reject(profile,altered)
for field,value in (("refusal_missing_cut_observed",False),("refusal_missing_cut_observed",1),
                    ("refusal_missing_cut_parent_device",True),("refusal_missing_cut_parent_inode",66.0),
                    ("refusal_missing_cut_parent_device",54),("refusal_missing_cut_parent_inode",67)):
    reject(profile,dict(report,**{field:value}))
for field,value in (("schema","source-recovery-refusal-observer-v1"),("missing_original_cut",False),
                    ("missing_original_cut",1),("journal_mode","0640"),("recovery_hold",{}),
                    ("external_journal_lease",True),("external_release_lease",True)):
    reject(dict(profile,**{field:value}),report)
base = dict(profile, schema="source-recovery-refusal-observer-v1")
del base["missing_original_cut"]
# Even a correctly pinned base/cancellation report must not borrow this cause.
reject(base,report,None)
reject(dict(base,schema="source-recovery-cancellation-observer-v1"),report,None)
without_cause = {key:value for key,value in report.items() if not key.startswith("refusal_missing_cut_")}
check(base,pinned(base,without_cause),raw)
reject(base,without_cause)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source original-cut causal observation grammar: %v\n%s", err, output)
	}
}

func TestSourceStartupPendingObservationRequiresOriginalGuard(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
check=loaded["source_refusal_observation"]
zero,other="0"*64,"1"*64
raw=b"source command failed\n"
base={"schema":"source-startup-recovery-refusal-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
      "journal_mode":"0600","external_journal_lease":False,"external_release_lease":False,"startup_pending":{"attempt":0,"gid":500}}
report={"inspection_only":False,"installed_execution":True,"refusal_admission_observed":True,
        "refusal_ownership_released_before_exit":True, "refusal_original_scope_joined":True,"entry":1000,"refusal_admission_entry":2000,"root_pid":77,
        "root_exit":2,"processes":1,"peak_live_processes":1,"refusal_stopped_threads":4,"threads_joined":4,"program_sha256":zero,
        "public_receipt_sha256":zero,"output_sha256":hashlib.sha256(raw).hexdigest(),
        "refusal_startup_pending_observed":True,"refusal_startup_pending_device":55,"refusal_startup_pending_inode":66,
        "refusal_startup_pending_sha256":other}
def pinned(p,r):
    return dict(r,profile_sha256=hashlib.sha256(json.dumps(p,separators=(",",":")).encode()+b"\n").hexdigest())
def reject(p,r,identity=(55,66,other)):
    try: check(p,pinned(p,r),raw,startup_pending_identity=identity)
    except RuntimeError: return
    raise AssertionError("missing/mixed/foreign startup guard accepted")
for attempt in range(64):
    p=dict(base,startup_pending={"attempt":attempt,"gid":500})
    summary=check(p,pinned(p,report),raw,startup_pending_identity=(55,66,other))
    assert summary["startup_pending"]=={"observed":True,"attempt":attempt,"device":55,"inode":66,"sha256":other}
for identity in (None,(55,66),(54,66,other),(55,67,other),(55,66,zero),(True,66,other),(55,66.0,other),(55,66,"F"*64),(55,66,123)):
    reject(base,report,identity)
for field in ("refusal_startup_pending_observed","refusal_startup_pending_device","refusal_startup_pending_inode","refusal_startup_pending_sha256"):
    changed=dict(report)
    del changed[field]
    reject(base,changed)
for field,value in (("refusal_startup_pending_observed",1),("refusal_startup_pending_observed",False),
                    ("refusal_startup_pending_device",True),("refusal_startup_pending_inode",66.0),
                    ("refusal_startup_pending_sha256",zero),("refusal_missing_cut_observed",True),
                    ("refusal_exclusion_observed",True),("refusal_retirement_pending_observed",True),("external_cut_requested",True)):
    reject(base,dict(report,**{field:value}))
for value in ({},{"attempt":True,"gid":500},{"attempt":64,"gid":500},{"attempt":-1,"gid":500},
              {"attempt":0,"gid":0},{"attempt":0,"gid":1000},{"attempt":0,"gid":500,"other":True}):
    reject(dict(base,startup_pending=value),report)
for field,value in (("retirement_pending",{"attempt":0,"gid":500}),("schema","source-recovery-refusal-observer-v1"),("startup_pending",None),("recovery_hold",{}),
                    ("missing_original_cut",True),("journal_mode","0640"),("external_journal_lease",True),("external_release_lease",True)):
    reject(dict(base,**{field:value}),report)
without_cause={key:value for key,value in report.items() if not key.startswith("refusal_startup_pending_")}
ordinary=dict(base,schema="source-recovery-refusal-observer-v1")
del ordinary["startup_pending"]
check(ordinary,pinned(ordinary,without_cause),raw)
reject(ordinary,without_cause)
for schema in ("source-recovery-refusal-observer-v1","source-recovery-cancellation-observer-v1","source-recovery-missing-cut-observer-v1"):
    reject(dict(ordinary,schema=schema),report,None)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source startup pending causal observation grammar: %v\n%s", err, output)
	}
}

func TestSourceRetirementPendingObservationRequiresOriginalGuard(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
check=loaded["source_refusal_observation"]
zero,other="0"*64,"1"*64
raw=b"source command failed\n"
base={"schema":"source-retirement-recovery-refusal-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
      "journal_mode":"0600","external_journal_lease":False,"external_release_lease":False,"retirement_pending":{"attempt":0,"gid":500}}
report={"inspection_only":False,"installed_execution":True,"refusal_admission_observed":True,
        "refusal_ownership_released_before_exit":True, "refusal_original_scope_joined":True,"entry":1000,"refusal_admission_entry":2000,"root_pid":77,
        "root_exit":2,"processes":1,"peak_live_processes":1,"refusal_stopped_threads":4,"threads_joined":4,"program_sha256":zero,
        "public_receipt_sha256":zero,"output_sha256":hashlib.sha256(raw).hexdigest(),
        "refusal_retirement_pending_observed":True,"refusal_retirement_pending_device":55,"refusal_retirement_pending_inode":66,
        "refusal_retirement_pending_sha256":other}
def pinned(p,r):
    return dict(r,profile_sha256=hashlib.sha256(json.dumps(p,separators=(",",":")).encode()+b"\n").hexdigest())
def reject(p,r,identity=(55,66,other)):
    try: check(p,pinned(p,r),raw,retirement_pending_identity=identity)
    except RuntimeError: return
    raise AssertionError("missing/mixed/foreign retirement guard accepted")
for attempt in range(64):
    p=dict(base,retirement_pending={"attempt":attempt,"gid":500})
    summary=check(p,pinned(p,report),raw,retirement_pending_identity=(55,66,other))
    assert summary["retirement_pending"]=={"observed":True,"attempt":attempt,"device":55,"inode":66,"sha256":other}
for identity in (None,(55,66),(54,66,other),(55,67,other),(55,66,zero),(True,66,other),(55,66.0,other),(55,66,"F"*64),(55,66,123)):
    reject(base,report,identity)
for field in ("refusal_retirement_pending_observed","refusal_retirement_pending_device","refusal_retirement_pending_inode","refusal_retirement_pending_sha256"):
    changed=dict(report)
    del changed[field]
    reject(base,changed)
for field,value in (("refusal_retirement_pending_observed",1),("refusal_retirement_pending_observed",False),
                    ("refusal_retirement_pending_device",True),("refusal_retirement_pending_inode",66.0),
                    ("refusal_retirement_pending_sha256",zero),("refusal_missing_cut_observed",True),
                    ("refusal_exclusion_observed",True),("refusal_startup_pending_observed",True),("external_cut_requested",True)):
    reject(base,dict(report,**{field:value}))
for value in ({},{"attempt":True,"gid":500},{"attempt":64,"gid":500},{"attempt":-1,"gid":500},
              {"attempt":0,"gid":0},{"attempt":0,"gid":1000},{"attempt":0,"gid":500,"other":True}):
    reject(dict(base,retirement_pending=value),report)
for field,value in (("startup_pending",{"attempt":0,"gid":500}),("schema","source-startup-recovery-refusal-observer-v1"),("schema","source-recovery-refusal-observer-v1"),("retirement_pending",None),("recovery_hold",{}),
                    ("missing_original_cut",True),("journal_mode","0640"),("external_journal_lease",True),("external_release_lease",True)):
    reject(dict(base,**{field:value}),report)
without_cause={key:value for key,value in report.items() if not key.startswith("refusal_retirement_pending_")}
ordinary=dict(base,schema="source-recovery-refusal-observer-v1")
del ordinary["retirement_pending"]
check(ordinary,pinned(ordinary,without_cause),raw)
reject(ordinary,without_cause)
for schema in ("source-recovery-refusal-observer-v1","source-recovery-cancellation-observer-v1","source-recovery-missing-cut-observer-v1"):
    reject(dict(ordinary,schema=schema),report,None)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source retirement pending causal observation grammar: %v\n%s", err, output)
	}
}

func TestSourceCancellationObservationRejectsMixedOrPrematureClaims(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib, json, runpy, sys
check = runpy.run_path(sys.argv[1])["source_cancellation_observation"]
zero, other = "0"*64, "1"*64
profile = {"schema":"source-recovery-cancellation-observer-v1", "program_sha256":zero, "public_receipt_sha256":zero,
           "journal_sha256":zero, "external_journal_lease":False, "external_release_lease":False,
           "recovery_hold":{"prefix_records":70}}
ready = {"profile_sha256":hashlib.sha256(json.dumps(profile,separators=(",",":")).encode()+b"\n").hexdigest(),
         "inspection_only":False,"installed_execution":True,"refusal_admission_observed":True,
         "recovery_held_before_record":True,"recovery_root_only_at_hold":True,"external_cancellation_acknowledged":False,
         "entry":1000,"refusal_admission_entry":2000,"recovery_hold_entry":3000,"root_pid":77,"held_root_thread":78,
         "program_sha256":zero,"public_receipt_sha256":zero,"selected_journal_sha256":zero}
notice = b'{"operation":"installation.source","outcome":"completion-unavailable"}\n'
raw = notice + b"source command failed\n"
report = dict(ready, external_cancellation_acknowledged=True, refusal_ownership_released_before_exit=True, refusal_original_scope_joined=True,
              recovery_held_image_preserved=True, signalled_root_thread=79, recovery_cancellation_elapsed_ns=1234567,
              root_exit=2,processes=1,peak_live_processes=1,refusal_stopped_threads=4,threads_joined=5,output_sha256=hashlib.sha256(raw).hexdigest())
assert check(profile,ready)["root_pid"] == 77
assert check(profile,ready,report,raw)["held_image_preserved"] is True
assert check(profile,ready,dict(report,processes=41,threads_joined=45,peak_live_processes=2),raw)["original_scope_joined"] is True
def reject(selected, held, final=report, output=raw):
    try:
        check(selected,held,final,output)
    except RuntimeError:
        return
    raise AssertionError("foreign/incomplete cancellation observation admitted")
for field,value in (("root_pid",True),("held_root_thread",0),("entry",True),("recovery_hold_entry",1000),
                    ("profile_sha256",other),("program_sha256",other),("public_receipt_sha256",other),
                    ("selected_journal_sha256",other),("recovery_held_before_record",False),
                    ("installed_execution",False),("external_cancellation_acknowledged",True),
                    ("refusal_ownership_released_before_exit",True),("refusal_original_scope_joined",True),
                    ("recovery_root_only_at_hold",False)):
    reject(profile,dict(ready,**{field:value}))
for field,value in (("root_pid",80),("held_root_thread",79),("recovery_hold_entry",3001),
                    ("selected_journal_sha256",other),("recovery_held_before_record",False),
                    ("recovery_held_image_preserved",False),("external_cancellation_acknowledged",False),
                    ("signalled_root_thread",78),("signalled_root_thread",True),
                    ("recovery_cancellation_elapsed_ns",True),("recovery_cancellation_elapsed_ns",0),
                    ("recovery_cancellation_elapsed_ns",120000000000),
                    ("refusal_ownership_released_before_exit",False),("refusal_original_scope_joined",False),
                    ("refusal_original_scope_joined",1),("threads_joined",3)):
    reject(profile,ready,dict(report,**{field:value}))
for field,value in (("schema","source-recovery-refusal-observer-v1"),("external_journal_lease",True),
                    ("external_release_lease",True),("recovery_hold",{"prefix_records":49}),
                    ("recovery_hold",{"prefix_records":70.0})):
    reject(dict(profile,**{field:value}),ready)
for output in (b"source command failed\n",notice+raw,b'{"outcome":"source-recovery-stopped-selected"}\n'+raw,b"foreign-prefix"+raw):
    reject(profile,ready,dict(report,output_sha256=hashlib.sha256(output).hexdigest()),output)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source cancellation observation grammar: %v\n%s", err, output)
	}
}

// Only the outside helper transport is exercised here. Detached child messages
// below are never canonical observations, Source authority or installed evidence.
func TestSourceCancellationObserverTransportJoinsAndRejectsDuplicateEvents(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import os, pathlib, queue, runpy, sys, tempfile
observer = runpy.run_path(sys.argv[1])["RecoveryCancellationObserver"]
fixture = '''import sys
print('SOURCE_RECOVERY_HOLD_READY {"transport_only":true}',flush=True)
line=sys.stdin.buffer.readline()
if line==b"cancel\\n":
    print('SOURCE_REFUSAL_OBSERVATION {"transport_only":true}',flush=True)
'''
with tempfile.TemporaryDirectory() as directory:
    root=pathlib.Path(directory)
    def start(name, source):
        return observer([sys.executable,"-I","-S","-c",source],dict(os.environ),root/name,lambda:None)
    def joined(instance):
        assert instance.closed and instance.child.poll() is not None and not instance.reader.is_alive()
        assert instance.child.stdin.closed and instance.child.stdout.closed and instance.log.closed
    first=start("complete.log",fixture)
    assert first.ready == {"transport_only":True}
    first.cancel()
    assert first.finish() == {"transport_only":True}
    joined(first)
    first.close()
    boundary=start("timeout-eof.log",fixture)
    class FinalTimeoutQueue(queue.Queue):
        def get(self, block=True, timeout=None):
            if block:
                # Force the real final message to arrive after the caller's
                # timed wait but before its EOF check. No timing-only sleep.
                boundary.child.wait(timeout=5)
                boundary.reader.join(timeout=5)
                assert not boundary.reader.is_alive()
                raise queue.Empty
            return super().get(block=False)
    boundary.events=FinalTimeoutQueue(maxsize=3)
    boundary.cancel()
    assert boundary.finish() == {"transport_only":True}
    joined(boundary)
    aborted=start("aborted.log",fixture)
    aborted.close()
    joined(aborted)
    duplicate=start("duplicate.log",fixture.replace("line=sys.stdin", "print('SOURCE_RECOVERY_HOLD_READY {}',flush=True)\nline=sys.stdin"))
    duplicate.cancel()
    try:
        duplicate.finish()
    except BaseExceptionGroup:
        pass
    else:
        raise AssertionError("duplicate held observation accepted")
    joined(duplicate)
    early=start("early.log",fixture)
    try:
        early.finish()
    except BaseExceptionGroup:
        pass
    else:
        raise AssertionError("completion without cancellation admitted")
    joined(early)
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source cancellation observer transport: %v\n%s", err, output)
	}
}

func TestSourceConcurrentRecoveryRequiresSeparateFinitePrediction(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import runpy, sys
loaded=runpy.run_path(sys.argv[1])
select,check=loaded["source_profile"],loaded["source_concurrent_prediction"]
for op,count in (("initial",70),("replacement",13)):
    selected=select(["-timeout=10m"],"recovery-concurrent-"+op)
    assert selected["concurrent"] is True and selected["operation"]==op and selected["prefix_records"]==count
    assert selected["fault"] is None and selected["bound"]==600
    expected={"operation":op,"prefix_records":count,"hold_phase":"file-ready","held_file":"ardents-node-linux-amd64",
              "second_refusal":"original-release-flock","after_cancellation":"repair-required",
              "partial_completion":"retained-no-append","start_permitted":False}
    assert check({"concurrent_cancellation":[expected]},op,count)==expected
    for inventory in (None,{},[],[expected,expected],
                      [dict(expected,prefix_records=49)],[dict(expected,start_permitted=True)],
                      [dict(expected,start_permitted=0)],[dict(expected,prefix_records=float(count))],
                      [dict(expected,second_refusal="generic-error")],[dict(expected,partial_completion="append")],
                      [dict(expected,after_cancellation="stopped-selected")],[dict(expected,extra=True)]):
        try:
            check({"concurrent_cancellation":inventory},op,count)
        except RuntimeError:
            pass
        else:
            raise AssertionError("missing/widened concurrent prediction admitted")
    for number in (count+1,True,float(count)):
        try:
            check({"concurrent_cancellation":[expected]},op,number)
        except RuntimeError:
            pass
        else:
            raise AssertionError("wrong original frontier admitted")
for case in ("recovery-concurrent-initial-70","recovery-concurrent-replacement-49","recovery-concurrent-startup"):
    try:
        select(["-timeout=10m"],case)
    except RuntimeError:
        pass
    else:
        raise AssertionError("unselected concurrent operation admitted")
for args in (["-timeout=3m"],["-timeout=8m"]):
    try:
        select(args,"recovery-concurrent-initial")
    except RuntimeError:
        pass
    else:
        raise AssertionError("wrong execution profile admitted")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source concurrent recovery finite prediction: %v\n%s", err, output)
	}
}

func TestSourcePhysicalRecoveryRequiresSeparateEffectPredictionAndOriginalObservation(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import hashlib,json,runpy,sys
loaded=runpy.run_path(sys.argv[1])
select,predict,observe=(loaded[name] for name in ("source_profile","source_physical_prediction","source_physical_observation"))
zero,other="0"*64,"1"*64
raw=b'{"operation":"installation.source","outcome":"completion-unavailable"}\nsource command failed\n'
for op,born in (("initial",70),("replacement",13)):
    windows={"file-born-before-record":(born-1,"ardents-node-linux-amd64","unrecorded-empty"),
             "file-ready-before-record":(born,"ardents-node-linux-amd64","recorded-born-complete-before-ready"),
             "file-born-before-journal-sync":(born,"ardents-node-linux-amd64","recorded-born-empty-before-journal-sync"),
             "seal-before-sealed":(105 if op=="initial" else 48,"generation","original-sealed-generation-before-record"),
             "binding-before-record":(109 if op=="initial" else 50,"binding.json","original-published-binding-before-record"),
             "unit-before-record":(113 if op=="initial" else 52,"ardents-source.service","original-published-unit-before-record"),
             "reload-before-verified":(114 if op=="initial" else 54,"binding-and-unit","original-completed-publications-before-verified"),
             "journal-complete-before-close":(115 if op=="initial" else 55,"binding-and-unit","original-complete-journal-before-writer-close"),
             "pending-before-unlink":(115 if op=="initial" else 55,"pending","original-closed-journal-before-pending-unlink")}
    if op=="initial":
        windows.update({"account-before-complete-birth":(24,"account-databases","completed-helper-incomplete-account-birth"),
                        "directory-born-before-record":(42,"installation-root","unrecorded-empty-root-directory")})
    for window,(count,held,payload) in windows.items():
        case=select(["-timeout=10m"],f"recovery-physical-{op}-{window}")
        assert case["operation"]==op and case["prefix_records"]==count and case["physical"]==window
        assert case["concurrent"] is False and case["fault"] is None
        expected={"operation":op,"prefix_records":count,"window":window,"held_file":held,"payload":payload,
                  "predicted_outcome":"repair-required","original_cut_proof":"absent","completion":"absent","start_permitted":False}
        assert predict({"physical_cuts":[expected]},op,count,window)==expected
        for inventory in (None,[],[expected,expected],[dict(expected,start_permitted=True)],
                          [dict(expected,start_permitted=0)],[dict(expected,original_cut_proof="eligible")],
                          [dict(expected,completion="allowed-header")],[dict(expected,prefix_records=float(count))],
                          [dict(expected,payload="other")],[dict(expected,predicted_outcome="stopped-selected")]):
            try:
                predict({"physical_cuts":inventory},op,count,window)
            except RuntimeError:
                pass
            else:
                raise AssertionError("missing/widened physical prediction admitted")
        profile={"schema":"source-physical-cut-observer-v1","program_sha256":zero,"public_receipt_sha256":zero,
                 "window":window,"prefix_records":count,
                 "arguments":["installation","install-initial-stopped","/distribution",zero,"/history","alpha","one-network","/request"]
                 if op=="initial" else ["installation","replace-source","/distribution","/history","alpha","one-network","/source/binding.json","/request"]}
        report={"profile_sha256":hashlib.sha256(json.dumps(profile,separators=(",",":")).encode()+b"\n").hexdigest(),
                "entry":1000,"root_pid":77,"root_exit":2,"threads_joined":8,"processes":2,"peak_live_processes":2,
                "held_coordinator_thread":79,"signalled_root_thread":78,"selected_records":count,
                "inspection_only":False,"installed_execution":True,"program_sha256":zero,"public_receipt_sha256":zero,
                "physical_window":window,"held_coordinator_killed":True,"external_cancellation_acknowledged":True,
                "coordinator_exit_held_before_join":True,"coordinator_exit_released_after_notice":True,
                "original_cut_sha256":"","output_sha256":hashlib.sha256(raw).hexdigest()}
        preimage={"device":55,"inode":66,"sha256":other} if op=="replacement" and window in ("binding-before-record","unit-before-record") else None
        if preimage is not None:
            report.update(physical_preimage_observed=True,physical_preimage_device=55,physical_preimage_inode=66,physical_preimage_sha256=other)
            for original in (None,{},dict(preimage,device=True),dict(preimage,inode=66.0),dict(preimage,device=54),dict(preimage,sha256=zero)):
                try:
                    observe(profile,report,raw,original)
                except RuntimeError:
                    pass
                else:
                    raise AssertionError("missing/foreign independently retained preimage admitted")
        check=lambda p,r,o:observe(p,r,o,preimage)
        assert check(profile,report,raw)["root_exit"]==2
        for field,value in (("entry",True),("root_pid",0),("root_exit",0),("threads_joined",1),("threads_joined",257),
                            ("peak_live_processes",True),("peak_live_processes",0),("peak_live_processes",1),("peak_live_processes",33),("peak_live_processes",3),("processes",1),("processes",33),("held_coordinator_thread",77),("signalled_root_thread",79),
                            ("selected_records",count+1),("inspection_only",True),("installed_execution",False),
                            ("program_sha256",other),("public_receipt_sha256",other),("profile_sha256",other),
                            ("root_only_at_physical_cut",True),("physical_window","other"),("held_coordinator_killed",False),
                            ("external_cancellation_acknowledged",False),("original_cut_sha256",zero),("output_sha256",other),
                            ("coordinator_exit_held_before_join",False),("coordinator_exit_held_before_join",1),
                            ("coordinator_exit_released_after_notice",False),("coordinator_exit_released_after_notice",1),
                            ("external_cut_ready",True),("external_cut_requested",True),("startup_archive",{}),("retirement_result",{})):
            try:
                check(profile,dict(report,**{field:value}),raw)
            except RuntimeError:
                pass
            else:
                raise AssertionError("mixed/incomplete physical observation admitted: "+field)
        for output in (b"source command failed\n",b"prefix"+raw,raw+raw):
            try:
                check(profile,dict(report,output_sha256=hashlib.sha256(output).hexdigest()),output)
            except RuntimeError:
                pass
            else:
                raise AssertionError("missing/foreign/duplicate processed notice admitted")
        if preimage is not None:
            for field,value in (("physical_preimage_observed",False),("physical_preimage_device",True),
                                ("physical_preimage_inode",67),("physical_preimage_sha256",zero)):
                try:
                    check(profile,dict(report,**{field:value}),raw)
                except RuntimeError:
                    pass
                else:
                    raise AssertionError("foreign/incomplete actual preimage observation admitted")
        else:
            try:
                check(profile,dict(report,physical_preimage_observed=True),raw)
            except RuntimeError:
                pass
            else:
                raise AssertionError("mixed publication preimage purpose admitted")
        if window in ("binding-before-record","unit-before-record"):
            for arguments in (None,[],["installation"],["installation","recover-source"]+["x"]*6):
                altered=dict(profile,arguments=arguments)
                declared=dict(report,profile_sha256=hashlib.sha256(json.dumps(altered,separators=(",",":")).encode()+b"\n").hexdigest())
                try:
                    check(altered,declared,raw)
                except RuntimeError:
                    pass
                else:
                    raise AssertionError("missing/foreign publication command admitted")
for name in ("recovery-physical-initial-root-file-ready-before-record","recovery-physical-replacement-seal-before-record"):
    try:
        select(["-timeout=10m"],name)
    except RuntimeError:
        pass
    else:
        raise AssertionError("unselected physical window admitted")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source physical recovery prediction/observation grammar: %v\n%s", err, output)
	}
}

func TestSourcePhysicalRefusalPlannerSeparatesUnsupportedPrefixCause(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import ast,json,runpy,sys
from pathlib import Path
source=Path(sys.argv[1])
tree=ast.parse(source.read_bytes())
main=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='main')
function=next(n for n in main.body if isinstance(n,ast.FunctionDef) and n.name=='physical_cut_and_refuse')
call=next(n for n in ast.walk(function) if isinstance(n,ast.Call) and isinstance(n.func,ast.Name) and n.func.id=='before_return_refusal')
keywords={v.arg:v.value for v in call.keywords}
allowed=(ast.Expression,ast.BoolOp,ast.Compare,ast.Name,ast.Load,ast.Constant,ast.Tuple,ast.List,ast.And,ast.Or,ast.Eq,ast.NotEq,ast.In,ast.NotIn,ast.IfExp,ast.UnaryOp,ast.Not)
def evaluate(expression,env):
    assert all(isinstance(n,allowed) for n in ast.walk(expression))
    return eval(compile(ast.Expression(expression),str(source),'eval'),{'__builtins__':{}},env)
frontiers=runpy.run_path(str(source))['source_physical_frontiers']
cases=[('account-before-complete-birth',24,24,True),('directory-born-before-record',42,42,True),('file-born-before-record',69,69,False),('binding-before-record',109,0,False),('unit-before-record',113,0,False),('file-born-before-journal-sync',70,0,False),('file-ready-before-record',70,0,False)]
mismatches=[]
cases=[('initial',*case) for case in cases]+[('replacement','file-born-before-record',12,12,False),('replacement','file-born-before-journal-sync',13,0,False),('replacement','binding-before-record',50,0,False),('replacement','unit-before-record',52,0,False)]
for operation,window,count,wanted,structural in cases:
    assert frontiers(operation)[window][0]==count
    env=dict(operation=operation,recovery_physical=window,recovery_count=count)
    for n in function.body:
        if isinstance(n,ast.Assign) and len(n.targets)==1 and isinstance(n.targets[0],ast.Name):
            try:value=evaluate(n.value,env)
            except (AssertionError,NameError):continue
            env[n.targets[0].id]=value
    selected=evaluate(keywords['early_prefix'],env)
    assert env['early'] is structural,'do not widen the resource-preservation early branch'
    missing=keywords['missing_cut_parent_identity'];assert isinstance(missing,ast.IfExp)
    selected_branch=missing.body if evaluate(missing.test,env) else missing.orelse
    needs_missing=not (isinstance(selected_branch,ast.Constant) and selected_branch.value is None)
    if selected!=wanted or needs_missing!=(wanted==0):
        mismatches.append(dict(operation=operation,window=window,prefix=count,expected_full_read_close_prefix=wanted,actual_early_prefix=selected,actual_missing_cut_requirement=needs_missing))
print(json.dumps(dict(actual_callsite=str(source),mismatches=mismatches,installed_execution=False,scope='planner-only, native causal proof still required')))
assert not mismatches,'physical unsupported-prefix planner selected an unvisited missing-cut cause'
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("physical refusal planner must retain the genuine prefix cause: %v\n%s", err, output)
	}
}
