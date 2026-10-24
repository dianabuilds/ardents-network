//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"os"
	"os/exec"
	"testing"
)

// The actual native root writer/tracer and maintained outside pipe owner are
// exercised together. No Source/manager/account/Release authority is supplied.
func TestSourcePhysicalCutControlNative(t *testing.T) {
	program, runner := os.Getenv("ARDENTS_SOURCE_CUT_CONTROL_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_CONTROL_RUNNER")
	if program == "" && runner == "" {
		return
	}
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 || !sourcePath(program) || !sourcePath(runner) {
		t.Fatal("actual native directed-cut control prerequisites required", err)
	}
	raw, err := os.ReadFile(runner)
	if err != nil || sourcePhysicalSHA(raw) != os.Getenv("ARDENTS_SOURCE_CUT_CONTROL_RUNNER_SHA256") {
		t.Fatal("independently pinned maintained pipe owner required", err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "python3", "-I", "-S", "-c", `
import fcntl,json,os,pathlib,runpy,subprocess,sys,tempfile,time
helper,program,runner=sys.argv[1:]
loaded=runpy.run_path(runner)
Observer=loaded["PhysicalCutObserver"]
pidfd_exited=loaded["source_pidfd_exited"]
# Actual pidfd exit readiness is distinct from a tracer's reaps. This private
# Python child supplies no Source/unit/domain completion or installed evidence.
child=subprocess.Popen([sys.executable,"-I","-S","-c","import sys;sys.stdin.buffer.read(1)"],stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
pidfd=None
try:
    pidfd=os.pidfd_open(child.pid,0)
    assert pidfd_exited(pidfd) is False
    child.stdin.close()
    assert child.wait()==0 and pidfd_exited(pidfd) is True
finally:
    if not child.stdin.closed: child.stdin.close()
    child.wait()
    if pidfd is not None: os.close(pidfd)
print("SOURCE_PIDFD_NATIVE_CONTROL live-then-exited-and-joined")
with tempfile.TemporaryDirectory(prefix="cut-",dir=os.environ["TMPDIR"]) as name:
    base=pathlib.Path(name)
    for scenario in ("cut","wrong-control","duplicate-control","eof"):
        root=base/scenario
        root.mkdir(mode=0o700)
        log=base/(scenario+".log")
        environment=dict(os.environ,ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW="root-controlled-file-ready-before-record",
                         ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM=program,ARDENTS_SOURCE_CUT_OBSERVER_ROOT=str(root),
                         ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT=str(base/(scenario+"-original.log")))
        deadline=time.monotonic()+20
        def observe():
            if time.monotonic()>=deadline:
                raise RuntimeError("native control exceeded original observation bound")
        actor=Observer([helper,"-test.run=^TestSourcePhysicalCutObserverNative$","-test.v","-test.timeout=30s"],environment,log,observe)
        try:
            ready=actor.ready
            assert ready["installed_execution"] is False and ready["external_cut_ready"] is True
            assert ready.get("external_cut_requested",False) is False and ready.get("root_kill_requested",False) is False
            assert ready.get("startup_archive") is None and ready["held_root_thread"]>0
            assert pathlib.Path("/proc",str(ready["root_pid"]),"exe").stat()
            journal=root/"original.source-initial-journal"
            fd=os.open(journal,os.O_RDONLY|os.O_NOFOLLOW)
            try:
                try: fcntl.flock(fd,fcntl.LOCK_EX|fcntl.LOCK_NB)
                except BlockingIOError: pass
                else: raise AssertionError("original writer lease released before outside cut")
            finally: os.close(fd)
            if scenario=="cut":
                actor.cut()
                result=actor.finish()
            elif scenario=="eof":
                actor.close()
            else:
                actor.child.stdin.write(b"cancel\n" if scenario=="wrong-control" else b"cut\ncut\n")
                actor.child.stdin.flush()
                actor.cancelled=True
                try: actor.finish()
                except BaseExceptionGroup: pass
                else: raise AssertionError("wrong/duplicate physical directive accepted")
        finally:
            actor.close()
        assert actor.closed and actor.child.poll() is not None and not actor.reader.is_alive()
        assert actor.child.stdin.closed and actor.child.stdout.closed and actor.log.closed
        text=log.read_text()
        reports=[json.loads(line[len("SOURCE_PHYSICAL_CUT_OBSERVATION "):]) for line in text.splitlines() if line.startswith("SOURCE_PHYSICAL_CUT_OBSERVATION ")]
        assert len(reports)==1
        result=reports[0]
        assert result["installed_execution"] is False and result["root_signal"]==9 and result["root_exit"]==-1
        assert result["processes"]==1 and result["threads_joined"]>=1
        assert result["external_cancellation_acknowledged"] is False and result.get("root_close_entry",0)==0
        assert result.get("root_close_observations",0)==0 and result.get("root_ownership_released_before_exit",False) is False
        assert result["external_cut_ready"] is True and result.get("external_cut_requested",False) is (scenario=="cut")
        assert (actor.child.returncode==0) is (scenario=="cut")
        if scenario=="cut":
            assert result["root_kill_requested"] is True and result["root_killed_by_observer"] is True
        fd=os.open(journal,os.O_RDONLY|os.O_NOFOLLOW)
        try: fcntl.flock(fd,fcntl.LOCK_EX|fcntl.LOCK_NB)
        finally: os.close(fd)
        assert (root/"next-effect").read_bytes()==b"original next effect\n"
        for suffix in (".original-cut-journal",".original-cut-result",".original-cut-pending",".recovery-completion"):
            assert not pathlib.Path(str(journal)+suffix).exists()
        print("SOURCE_DIRECTED_CUT_NATIVE_OBSERVATION",scenario,json.dumps(result,separators=(",",":")))
        if scenario!="cut":
            print("EXPECTED_NATIVE_CONTROL_REFUSAL",scenario,text)
`, helper, program, runner)
	output, err := command.CombinedOutput()
	t.Logf("%s", output)
	if err != nil {
		t.Fatal("actual held-root cut directive and joined refusal controls", err)
	}
}
