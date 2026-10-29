#!/usr/bin/env python3
"""One joined filesystem-component stand in a stable Docker bind root.

Run under run-development.py exclusive. This is not an installed profile.
Unknown leftovers retain the occupied slot and prevent the next allocation.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
from native_stand import WORK_ROOT, allocate, native, references
IMAGE = 'golang@sha256:21ade014faf952365b9ecbd8dbdb87176fbacb82fac9fb132d9f2a7b8cc3f132'
DISCOVERY = {'selected-tests', 'selected-completion-tests', 'selected-fixedfile-tests'}


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def write(path, data):
    with path.open('x', encoding='utf8') as stream:
        json.dump(data, stream, indent=2)
        stream.flush()
        os.fsync(stream.fileno())

def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or os.geteuid() != 0:
        raise RuntimeError('Linux root Python3.12 -I -S required')
    if len(sys.argv) != 3 or not re.fullmatch(r'intent[1-9][0-9]*', sys.argv[2]):
        raise RuntimeError('usage: run-filesystem-component.py EXTERNAL_EVIDENCE intentN')
    evidence = Path(sys.argv[1])
    run = sys.argv[2]
    repository = Path(__file__).resolve().parents[2]
    if not evidence.is_absolute() or evidence.is_symlink() or evidence.resolve() != evidence or repository == evidence or repository in evidence.parents:
        raise RuntimeError('existing real external evidence directory required')
    number = run.removeprefix('intent')
    script = evidence / (run + '-work.sh')
    manifest = evidence / ('source' + number + '.sha256')
    if not script.is_file() or script.is_symlink() or not manifest.is_file() or manifest.is_symlink():
        raise RuntimeError('original work script and source manifest required')
    result = evidence / (run + '-stand.json')
    if result.exists() or (evidence / (run + '.log')).exists() or (evidence / (run + '.exit')).exists():
        raise RuntimeError('fresh evidence paths required; earlier attempts are immutable')
    name = 'ardents-component-' + run
    names = subprocess.check_output(['docker', 'ps', '-a', '--format', '{{.Names}}'], text=True).splitlines()
    if name in names:
        raise RuntimeError('original container name already exists')
    active, created, before = allocate()
    tmp = active / 'tmp'
    tmp.mkdir(mode=0o700)
    identity = native(active)
    receipt = {'created': time.time(), 'run': run, 'image': IMAGE, 'work_root_created': created, 'original_active': identity, 'script_sha256': digest(script), 'source_manifest_sha256': digest(manifest), 'before': before, 'state': 'retained'}
    command = ['docker', 'run', '--rm', '--name', name, '--network', 'none', '--cpus=2', '--memory=4g', '--user', '0:0']
    for key, value in {'GOENV':'off', 'GOTOOLCHAIN':'local', 'GOFLAGS':'-mod=readonly -buildvcs=false', 'GOTELEMETRY':'off', 'GOPROXY':'off', 'GOSUMDB':'off', 'GOCACHE':'/test-cache', 'GOMODCACHE':'/go/pkg/mod', 'GOMAXPROCS':'2', 'TMPDIR':'/testtmp/active/tmp'}.items():
        command += ['-e', key + '=' + value]
    for source, target, readonly in [(repository, '/repo', True), (evidence, '/evidence', True), (WORK_ROOT, '/testtmp', False), (Path('/mnt/c/Users/vitek/AppData/Local/Temp/ardents-network-quality/go-mod'), '/go/pkg/mod', True)]:
        command += ['--mount', 'type=bind,source=' + str(source) + ',target=' + target + (',readonly' if readonly else '')]
    command += ['-v', 'ardents-network-go-build:/test-cache', '-w', '/repo', IMAGE, 'sh', '-c', 'ln -s /evidence/source' + number + '.sha256 /source.sha256 && exec sh /evidence/' + run + '-work.sh']
    with (evidence / (run + '.log')).open('xb') as log:
        code = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, check=False).returncode
    receipt['test_exit'] = code
    (evidence / (run + '.exit')).write_text(str(code) + '\n', encoding='utf8')
    try:
        names = subprocess.check_output(['docker', 'ps', '-a', '--format', '{{.Names}}'], text=True).splitlines()
        if name in names:
            raise RuntimeError('original component container has not joined')
        receipt['after'] = references(active)
        if digest(script) != receipt['script_sha256'] or digest(manifest) != receipt['source_manifest_sha256']:
            raise RuntimeError('original work script or manifest changed')
        if code:
            raise RuntimeError('failed component stand retained')
        if native(active) != identity or {p.name for p in active.iterdir()} != {'tmp'} or tmp.is_symlink():
            raise RuntimeError('original component stand changed')
        files = list(tmp.iterdir())
        if any(p.name not in DISCOVERY or not stat.S_ISREG(p.lstat().st_mode) or p.is_symlink() for p in files):
            raise RuntimeError('unknown component leftovers retained')
        discovery = evidence / (run + '-discovery')
        discovery.mkdir(mode=0o700)
        receipt['discovery'] = {}
        for p in files:
            original = native(p)
            data = p.read_bytes()
            with (discovery / p.name).open('xb') as f:
                f.write(data);f.flush();os.fsync(f.fileno())
            if (discovery / p.name).read_bytes() != data or native(p) != original:
                raise RuntimeError('discovery preservation changed')
            receipt['discovery'][p.name] = {'sha256': digest(p), 'native': original}
        references(active)
        for p in files:
            p.unlink()
        tmp.rmdir()
        active.rmdir()
        receipt['state'] = 'joined-and-removed'
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        receipt['cleanup_failure'] = str(error)
        code = code or 2
    receipt['completed'] = time.time()
    write(result, receipt)
    print(json.dumps({'run': run, 'test_exit': receipt['test_exit'], 'stand': receipt['state'], 'result': str(result)}), flush=True)
    return code


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print('component stand refused/retained:', error, file=sys.stderr)
        sys.exit(2)
