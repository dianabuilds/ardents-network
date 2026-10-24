//go:build linux

package architecture

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSourceRecoveryNativeProfilePreservesRecordedAccess(t *testing.T) {
	// Execute the actual runner's child owner and native-controls call with a
	// file-only probe. It supplies no Source, manager or recovery authority.
	command := exec.CommandContext(t.Context(), "python3", "-I", "-S", "-c", `
import ast, hashlib, json, os, pathlib, signal, stat, subprocess, sys, tempfile, threading, time
path = pathlib.Path(sys.argv[1])
tree = ast.parse(path.read_bytes())
main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "main")
owner = next(node for node in main.body if isinstance(node, ast.FunctionDef) and node.name == "run")
calls = [node for node in ast.walk(main) if isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
         and node.func.id == "run" and len(node.args) > 1 and isinstance(node.args[1], ast.Constant)
         and node.args[1].value == "recovery-native-controls"]
assert len(calls) == 1
previous = os.umask(0o077)
try:
    with tempfile.TemporaryDirectory() as name:
        root = pathlib.Path(name)
        output = root / "output"
        output.mkdir(mode=0o700)
        probe = root / "probe"
        probe.write_text("#!/usr/bin/env python3\nimport json,os,pathlib\nr=pathlib.Path(os.environ['PROFILE_PROBE_ROOT'])\nfor n,m in [('binding',0o644),('program',0o555)]:\n f=os.open(r/n,os.O_WRONLY|os.O_CREAT|os.O_EXCL,m);os.close(f)\nprint(json.dumps({n:(r/n).stat().st_mode&0o777 for n in ['binding','program']}))\n")
        probe.chmod(0o700)
        scope = dict(subprocess=subprocess, threading=threading, signal=signal, time=time, sys=sys,
                     observe=lambda: None, output=output, interrupted=threading.Event(), clock_errors=[],
                     started=time.monotonic(), bound=5, paths={"RECOVERY_OBSERVER": probe},
                     native_pattern="^file-only-probe$", native_environment=dict(os.environ, PROFILE_PROBE_ROOT=str(root)))
        exec(compile(ast.fix_missing_locations(ast.Module(body=[owner], type_ignores=[])), str(path), "exec"), scope)
        raw = eval(compile(ast.Expression(calls[0]), str(path), "eval"), scope)
        assert json.loads(raw) == {"binding": 0o644, "program": 0o555}, raw
        assert stat.S_IMODE((output / "recovery-native-controls.log").stat().st_mode) == 0o600
        inherited = os.umask(0o077)
        assert inherited == 0o077, inherited
finally:
    os.umask(previous)
`, filepath.Join("..", "..", "scripts", "installation", "run-source-startup.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native profile changed recorded access or private parent policy: %v\n%s", err, output)
	}
}
