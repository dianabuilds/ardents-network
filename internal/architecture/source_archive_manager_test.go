package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceArchivedManagerBindsOwnerAndOriginalBrokerAtLiveGuards(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import ast,json,runpy,sys
from pathlib import Path
from types import SimpleNamespace
path=Path(sys.argv[1]);loaded=runpy.run_path(str(path));tree=ast.parse(path.read_bytes())
owner=b's ":1.7"\n';identity='2'*32+'/:1.7'
messages={'retirement_cut_and_refuse':'pre-stop retained Source differs from original archived invocation',
          'retain_invocation':'outside original Source pidfd/proc/cgroup differs from held startup archive'}

# Detached values test the actual preconditions, never an installed manager,
# native process or accepting authority. The full physical profiles stay required.
def fixture():
    state=dict(owner=owner,identity=identity)
    archive=dict(pid=123,invocation_id='1'*32,manager=':1.7',process_device=41,process_inode=500,cgroup_device=51,cgroup_inode=600)
    proc=SimpleNamespace(st_dev=41,st_ino=500);scope=SimpleNamespace(st_dev=51,st_ino=600)
    events=SimpleNamespace(st_dev=51,st_ino=601)
    ns=dict(json=json,archive=archive,source_pid=123,invocation='1'*32,source_pidfd=7,
            original_manager=owner,original_manager_identity=identity,process=proc,process_info=proc,current_process=proc,
            scope_info=scope,current_scope=scope,named_events=events,retained_events=events,exited=False,joined=False,state=state)
    ns.update(source_pidfd_exited=lambda fd:ns['exited'],original_join=lambda:ns['joined'],
              manager_owner=lambda:state['owner'],manager_identity=lambda:state['identity'])
    if 'source_archived_manager_matches' in loaded:
        ns['source_archived_manager_matches']=loaded['source_archived_manager_matches']
    return ns

for name,message in messages.items():
    function=next(n for n in ast.walk(tree) if isinstance(n,ast.FunctionDef) and n.name==name)
    guards=[n for n in ast.walk(function) if isinstance(n,ast.If) and any(isinstance(s,ast.Raise) and isinstance(s.exc,ast.Call)
            and s.exc.args and isinstance(s.exc.args[0],ast.Constant) and s.exc.args[0].value==message for s in n.body)]
    assert len(guards)==1, name
    code=compile(ast.Expression(guards[0].test),str(path),'eval')
    assert eval(code,fixture()) is False, (name,'coherent archived owner and original broker/owner falsely refused')
    changes=[]
    for field,value in [('pid',124),('invocation_id','3'*32),('manager',identity),('manager',':1.8'),('manager',None),
                        ('process_device',42),('process_inode',501),('cgroup_device',52),('cgroup_inode',602)]:
        changes.append((f'archive-{field}-{value}',lambda ns,k=field,v=value:ns['archive'].__setitem__(k,v)))
    changes.extend([
        ('same-owner-new-broker',lambda ns:ns['state'].__setitem__('identity','3'*32+'/:1.7')),
        ('changed-owner',lambda ns:ns['state'].update(owner=b's ":1.8"\n',identity='2'*32+'/:1.8')),
        ('same-owner-malformed-broker',lambda ns:ns['state'].__setitem__('identity',':1.7')),
        ('original-scope-empty',lambda ns:ns.__setitem__('joined',True)),
        ('named-events-device',lambda ns:ns.__setitem__('named_events',SimpleNamespace(st_dev=52,st_ino=601))),
        ('named-events-inode',lambda ns:ns.__setitem__('named_events',SimpleNamespace(st_dev=51,st_ino=602))),
        ('retained-events-inode',lambda ns:ns.__setitem__('retained_events',SimpleNamespace(st_dev=51,st_ino=602)))])
    if name=='retirement_cut_and_refuse':
        changes.append(('original-pidfd-exited',lambda ns:ns.__setitem__('exited',True)))
        changes.append(('observed-process-changed',lambda ns:ns.__setitem__('process',SimpleNamespace(st_dev=41,st_ino=501))))
    else:
        changes.append(('observed-process-changed',lambda ns:ns.__setitem__('current_process',SimpleNamespace(st_dev=41,st_ino=501))))
        changes.append(('observed-scope-changed',lambda ns:ns.__setitem__('current_scope',SimpleNamespace(st_dev=51,st_ino=602))))
    for label,change in changes:
        ns=fixture();change(ns)
        assert eval(code,ns) is True, (name,label,'native identity or original broker check was weakened')

# The value matcher distinguishes the persisted owner from the separately
# retained full identity. It must never normalize a substitute into a match.
match=loaded['source_archived_manager_matches']
assert match(':1.7',owner,identity,identity) is True
for archived,original,retained,fresh in [
    (identity,owner,identity,identity),(':1.8',owner,identity,identity),
    (':1.7',b's ":1.8"\n',identity,identity),(':1.7',owner,identity,'3'*32+'/:1.7'),
    (':1.7',owner,identity,'2'*32+'/:1.8'),(':1.7',owner,':1.7',':1.7'),
    (':1.7',owner,'2'*32+'//:1.7','2'*32+'//:1.7'),
    (':1.7',owner,'A'*32+'/:1.7','A'*32+'/:1.7'),
    (None,owner,identity,identity),(':1.7',None,identity,identity),
    (':1.7','s ":1.7"',identity,identity),(':1.7',b'garbage',identity,identity),
    (':1.7',b's true',identity,identity),(':1.7',owner,True,identity),(':1.7',owner,identity,True)]:
    assert match(archived,original,retained,fresh) is False, (archived,original,retained,fresh)

# Bind the actual initial snapshot before source effects. Mismatched snapshots
# must refuse even when the broker restarts with the same textual owner.
main=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='main')
capture=next(n for n in main.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='original_manager_identity' for t in n.targets))
initial=next(n for n in main.body if isinstance(n,ast.If) and any(isinstance(s,ast.Raise) and isinstance(s.exc,ast.Call)
             and s.exc.args and isinstance(s.exc.args[0],ast.Constant) and s.exc.args[0].value=='original coherent broker/manager snapshot unavailable' for s in n.body))
assert capture.lineno<initial.lineno
capture_code=compile(ast.Module(body=[capture],type_ignores=[]),str(path),'exec')
initial_code=compile(ast.Expression(initial.test),str(path),'eval')
ns=fixture();exec(capture_code,ns);assert eval(initial_code,ns) is False
ns['state']['identity']='3'*32+'/:1.7';assert eval(initial_code,ns) is True
ns=fixture();ns['state']['owner']=b's ":1.8"\n';exec(capture_code,ns);assert eval(initial_code,ns) is True
print('both actual live guards retain original owner/broker and native identity refusals')
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Source archived manager/live guard contract failed: %v\n%s", err, output)
	}
}
