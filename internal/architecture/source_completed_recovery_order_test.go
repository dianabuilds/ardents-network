package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceCompletedClassificationFollowsActualRecovery(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import ast,sys,types
from pathlib import Path
path=Path(sys.argv[1]);tree=ast.parse(path.read_bytes())
main=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=="main")
stop=next(n for n in ast.walk(main) if isinstance(n,ast.FunctionDef) and n.name=="explicit_stop")
def close_scope(n):
    return isinstance(n,ast.Expr) and isinstance(n.value,ast.Call) and ast.unparse(n.value)=="os.close(scope_fd)"
start=next(i for i,n in enumerate(stop.body) if close_scope(n))
tail=ast.fix_missing_locations(ast.Module(body=stop.body[start:],type_ignores=[]))
account=next(n for n in ast.walk(main) if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=="expected" for t in n.targets)
             and isinstance(n.value,ast.ListComp) and "journal_kind" not in ast.unparse(n.value)
             and "startup" in ast.unparse(n.value) and "retirement" in ast.unparse(n.value))

def check(number,replacement,recovered,positive=True):
    events=[]
    def close(fd):
        assert fd in (11,12)
        events.append(("close",fd))
    def classify(n):
        assert events==[("close",11),("close",12)],"classification preceded original observer release"
        events.append(("classification",n))
    def run(argv,name,**kw):
        assert argv==["owner.test","-test.run=^TestSourceHistoryAcquisitionNative$","-test.v","-test.timeout=1m"]
        assert kw["env"]==dict(ARDENTS_SOURCE_HISTORY_BINDING="binding",ARDENTS_SOURCE_HISTORY_RELEASE="history",ARDENTS_SOURCE_HISTORY_CASE="success")
        events.append(("predecessor-history",name))
    scope=dict(os=types.SimpleNamespace(close=close,environ={}),scope_fd=11,source_pidfd=12,source_started=True,
               positive_recovery=positive,replacement_profile=replacement,recovery_evidence={"journal":"original"} if recovered else {},
               outcome={"recovery":{"operation":"replacement" if replacement else "initial"}} if recovered else {},
               number=number,completed_classification=classify,run=run,paths={"OWNER_TEST":"owner.test"},binding="binding",history="history",
               invocations=[],invocation="original-invocation")
    exec(compile(tail,str(path),"exec"),scope)
    expected=[("close",11),("close",12)]
    if positive and recovered: expected.append(("classification",number))
    if replacement: expected.append(("predecessor-history","history-success-"+str(number)))
    assert events==expected,(number,replacement,recovered,positive,events)
    assert scope["scope_fd"] is None and scope["source_pidfd"] is None and scope["source_started"] is False
    assert scope["invocations"]==["original-invocation"]

# Old A is genuinely retired and checked, but no recovery has happened yet.
check(0,True,False)
# Every actual post-recovery B/B/C invocation still needs both classifications.
for number in (1,2,3): check(number,True,True)
for number in (0,1,2): check(number,False,True)
for replacement in (False,True):
    check(0,replacement,False,False)
    scope=dict(replacement_profile=replacement)
    exec(compile(ast.fix_missing_locations(ast.Module(body=[account],type_ignores=[])),str(path),"exec"),scope)
    numbers=(1,2,3) if replacement else (0,1,2)
    assert scope["expected"]==[(n,kind) for n in numbers for kind in ("startup","retirement")],scope["expected"]
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("completed classification did not follow actual recovery with retained predecessor joins/history: %v\n%s", err, output)
	}
}
