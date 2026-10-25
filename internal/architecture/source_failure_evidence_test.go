package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceQualificationRetainsPrivateFailureCauses(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import ast,json,os,re,runpy,subprocess,sys,tempfile,types
from pathlib import Path
path=Path(sys.argv[1]);module=runpy.run_path(str(path))
failure=module['source_failure_evidence']
first=subprocess.CalledProcessError(2,['node','installation','stop-source','history','binding'])
first.__cause__=TimeoutError('original exchange unavailable')
second=RuntimeError('late original cleanup failure')
errors=[ExceptionGroup('held original stop failed',[first]),second]
result=failure(errors)
encoded=json.dumps(result,ensure_ascii=False).encode()
assert len(encoded)<=64<<10 and result['schema']=='source-private-failure-evidence-v1'
nodes=result['errors']
assert any(n.get('returncode')==2 and 'CalledProcessError' in n['type'] for n in nodes)
assert any(n['relation']=='cause' and n['message']=='original exchange unavailable' for n in nodes)
assert any(n['message']=='late original cleanup failure' for n in nodes)
assert errors[0].exceptions[0] is first and first.__cause__.args==('original exchange unavailable',)
long=RuntimeError('x'*1000000);long.__cause__=long
bounded=failure([long]*1000)
assert bounded['truncated'] and len(json.dumps(bounded,ensure_ascii=False).encode())<=64<<10

# Execute the actual terminal failure branch, not a copied writer. Diagnostic
# output cannot replace either original failure or grant a passing outcome.
tree=ast.parse(path.read_bytes());main=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='main')
terminal=next(n for n in main.body if isinstance(n,ast.If) and ast.unparse(n.test)=='failures'
              and any(isinstance(x,ast.Raise) for x in n.body))
# Windows executes the writer's ordinary file operations; native no-follow and
# close-on-exec flags are separately mandatory in its Linux profile.
private_os=types.SimpleNamespace(open=os.open,fdopen=os.fdopen,fsync=os.fsync,
    **{n:getattr(os,n,0) for n in ('O_WRONLY','O_CREAT','O_EXCL','O_NOFOLLOW','O_CLOEXEC')})
with tempfile.TemporaryDirectory() as directory:
    scope=dict(output=Path(directory),failures=errors.copy(),source_failure_evidence=failure,json=json,os=private_os)
    try:exec(compile(ast.fix_missing_locations(ast.Module(body=[terminal],type_ignores=[])),str(path),'exec'),scope)
    except ExceptionGroup as raised:
        assert raised.exceptions[:2]==tuple(errors)
    else:raise AssertionError('private diagnostics converted failed qualification to success')
    saved=json.loads((Path(directory)/'private-failure.json').read_bytes())
    assert saved==result
    try:exec(compile(ast.fix_missing_locations(ast.Module(body=[terminal],type_ignores=[])),str(path),'exec'),scope)
    except ExceptionGroup as raised:
        assert raised.exceptions[:2]==tuple(errors)
        assert any(isinstance(e,FileExistsError) for e in raised.exceptions[2:])
    else:raise AssertionError('existing failure evidence overwritten')
    assert json.loads((Path(directory)/'private-failure.json').read_bytes())==result

# Exercise the actual cleanup branch: failed diagnostic acquisition still
# reaches native unit cleanup and retains the two original causes first.
cleanup=next(n for n in ast.walk(main) if isinstance(n,ast.If)
             and ast.unparse(n.test)=='source_started and source_pidfd is not None and (scope_fd is not None)')
events=[]
def unavailable(*args):
    events.append('capture');raise OSError('diagnostic unavailable')
scope=dict(source_started=True,source_pidfd=11,scope_fd=12,manager_owner=lambda:'original',original_manager='original',
           failures=errors.copy(),source_failure_journal=unavailable,output=Path('private'),invocation='a'*32,
           subprocess=types.SimpleNamespace(run=lambda argv,check:events.append(('cleanup',argv,check))))
exec(compile(ast.fix_missing_locations(ast.Module(body=[cleanup],type_ignores=[])),str(path),'exec'),scope)
assert events==['capture',('cleanup',['/usr/bin/systemctl','stop','ardents-source.service'],True)]
assert scope['failures'][:2]==errors and isinstance(scope['failures'][2],OSError)

# Fixed terminal-message capture has one original invocation, finite selector
# and child timeout. The fixture executes no journal or installed command.
journal=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='source_failure_journal')
calls=[]
def read_journal(argv,stdout,stderr,check,timeout):
    assert argv[:5]==['/usr/bin/journalctl','--no-pager','--output=cat','--lines=32','--unit=ardents-source.service']
    assert argv[-1]=='_SYSTEMD_INVOCATION_ID='+'a'*32 and check and timeout==3
    assert argv[5]=='--case-sensitive=yes', 'journalctl lowercase grep defaults to case-insensitive'
    pattern=argv[6].removeprefix('--grep=')
    for message in ('source command failed','context canceled','{"operation":"source.start-installed","outcome":"retired"}',
                    '{"operation":"source.start-installed","outcome":"retirement-failed"}',
                    '{"operation":"source.start-installed","outcome":"completion-unavailable"}'):
        assert re.fullmatch(pattern,message)
    for message in ('private key data','source command failed extra','SOURCE COMMAND FAILED','{"operation":"other","outcome":"retired"}'):
        assert not re.fullmatch(pattern,message)
    stdout.write(b'source command failed\n');calls.append(argv)
scope=dict(os=private_os,subprocess=types.SimpleNamespace(run=read_journal,STDOUT=-2))
exec(compile(ast.fix_missing_locations(ast.Module(body=[journal],type_ignores=[])),str(path),'exec'),scope)
with tempfile.TemporaryDirectory() as directory:
    scope['source_failure_journal'](Path(directory),'a'*32)
    assert (Path(directory)/'private-source-terminal.log').read_bytes()==b'source command failed\n'
    try:scope['source_failure_journal'](Path(directory),'bad')
    except RuntimeError:pass
    else:raise AssertionError('foreign journal selector admitted')
    assert len(calls)==1
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("private Source failure causes were lost or changed qualification outcome: %v\n%s", err, output)
	}
}
