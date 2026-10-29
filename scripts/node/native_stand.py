"""The one fresh native development slot and its physical reference checks.

No product authority, original-root adoption, recursive deletion or retry.
Callers hold the existing shared exclusive development lock throughout use.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time

WORK_ROOT = Path('/var/lib/ardents-development-work')


def sync_directory(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def native(path):
    s = path.lstat()
    return (s.st_dev, s.st_ino, s.st_mode, s.st_uid, s.st_gid)


def references(active):
    """Check native objects and equivalent paths in every visible namespace."""
    failures = []
    mount_namespaces = {}
    net_namespaces = {}
    processes = [p for p in Path('/proc').iterdir() if p.name.isdecimal()]
    for proc in processes:
        try:
            ns = os.readlink(proc / 'ns/mnt')
            if ns not in mount_namespaces:
                mount_namespaces[ns] = (proc / 'mountinfo').read_text(encoding='utf8')
            ns = os.readlink(proc / 'ns/net')
            if ns not in net_namespaces:
                net_namespaces[ns] = (proc / 'net/unix').read_text(encoding='utf8')
        except (FileNotFoundError, ProcessLookupError):
            pass
    device = WORK_ROOT.stat().st_dev
    aliases = {str(active)}
    mounts = []
    def unescape(value):
        return re.sub(r'\\([0-7]{3})', lambda match: chr(int(match[1], 8)), value)
    for ns, contents in mount_namespaces.items():
        for line in contents.splitlines():
            fields = line.split(' - ', 1)[0].split()
            major, minor = (int(part) for part in fields[2].split(':'))
            root, target = (Path(unescape(fields[i])) for i in [3, 4])
            same_device = os.makedev(major, minor) == device
            mounts.append((ns, root, target, same_device))
            if same_device:
                try:
                    aliases.add(str(target / active.relative_to(root)))
                except ValueError:
                    pass
    def inspect(value, owner):
        if any(alias in value for alias in aliases):
            failures.append(owner)
    for ns, root, target, same_device in mounts:
        # A stable-parent integration bind is allowed; a mounted active child
        # or a mount placed inside any equivalent active path remains blocking.
        if same_device and (root == active or active in root.parents):
            failures.append('mounted-child:' + ns)
        inspect(str(target), 'mount-target:' + ns)
    identities = set()
    if active.exists():
        for directory, dirs, files in os.walk(active, followlinks=False):
            for path in [Path(directory), *(Path(directory) / n for n in files + dirs)]:
                st = path.lstat()
                identities.add((st.st_dev, st.st_ino))
    def inspect_native(path, owner):
        st = path.stat()
        if (st.st_dev, st.st_ino) in identities:
            failures.append(owner)
    for proc in processes:
        try:
            for key in ['cwd', 'root', 'exe']:
                try:
                    inspect(os.readlink(proc / key), 'process-path:' + proc.name)
                    inspect_native(proc / key, 'process-native:' + proc.name)
                except FileNotFoundError:
                    pass
            inspect((proc / 'cmdline').read_bytes().decode('utf8', errors='replace'), 'cmdline:' + proc.name)
            maps = (proc / 'maps').read_text(encoding='utf8', errors='replace')
            inspect(maps, 'maps-path:' + proc.name)
            for line in maps.splitlines():
                fields = line.split(None, 5)
                major, minor = (int(part, 16) for part in fields[3].split(':'))
                if (os.makedev(major, minor), int(fields[4])) in identities:
                    failures.append('maps-native:' + proc.name)
            for fd in (proc / 'fd').iterdir():
                try:
                    inspect(os.readlink(fd), 'fd-path:' + proc.name)
                    inspect_native(fd, 'fd-native:' + proc.name)
                except FileNotFoundError:
                    pass
        except (FileNotFoundError, ProcessLookupError):
            pass
    for ns, contents in net_namespaces.items():
        inspect(contents, 'socket:' + ns)
    ids = subprocess.check_output(['docker', 'ps', '-aq'], text=True).split()
    if ids:
        containers = subprocess.check_output(['docker', 'inspect', *ids], text=True)
        # Even a stopped container configured for the stable root prevents reuse.
        if str(WORK_ROOT) in containers:
            failures.append('docker-container')
        inspect(containers, 'docker-alias')
    units = json.loads(subprocess.check_output(['systemctl', 'list-units', '--all', '--output=json', '--no-pager'], text=True))
    names = [u['unit'] for u in units]
    for offset in range(0, len(names), 64):
        inspect(subprocess.check_output(['systemctl', 'show', '--', *names[offset:offset + 64]], text=True), 'loaded-unit')
    if failures:
        raise RuntimeError('native stand remains referenced: ' + repr(sorted(set(failures))))
    return {'mount_namespaces': len(mount_namespaces), 'net_namespaces': len(net_namespaces), 'native_objects': len(identities), 'containers': len(ids), 'loaded_units': len(names), 'references': 0}


def prepare_root():
    created = False
    if not WORK_ROOT.exists():
        WORK_ROOT.mkdir(mode=0o711)
        WORK_ROOT.chmod(0o711)
        (WORK_ROOT / 'owner').write_text('ardents-development-work-v1\n', encoding='utf8')
        (WORK_ROOT / 'owner').chmod(0o600)
        with (WORK_ROOT / 'owner').open('rb') as stream:
            os.fsync(stream.fileno())
        sync_directory(WORK_ROOT)
        sync_directory(WORK_ROOT.parent)
        created = True
    s = WORK_ROOT.lstat()
    if WORK_ROOT.resolve() != WORK_ROOT or not stat.S_ISDIR(s.st_mode) or s.st_uid != 0 or s.st_gid != 0 or stat.S_IMODE(s.st_mode) != 0o711:
        raise RuntimeError('unsafe native development work root')
    marker = WORK_ROOT / 'owner'
    if marker.is_symlink() or native(marker)[2:] != (stat.S_IFREG | 0o600, 0, 0) or marker.read_text(encoding='utf8') != 'ardents-development-work-v1\n':
        raise RuntimeError('unowned native development work root')
    if not {p.name for p in WORK_ROOT.iterdir()} <= {'owner', 'active', 'cases', 'case-history', 'case-active.json'}:
        raise RuntimeError('unknown native development work entry')
    if subprocess.check_output(['findmnt', '-T', str(WORK_ROOT), '-n', '-o', 'FSTYPE'], text=True).strip() != 'ext4':
        raise RuntimeError('native ext4 native development work root required')
    return created


def check_idle():
    """Keep completed QA in place; an unfinalized or unknown case blocks work."""
    prepare_root()
    if any(p.exists() or p.is_symlink() for p in [WORK_ROOT / 'active', WORK_ROOT / 'case-active.json']):
        raise RuntimeError('previous native stand is retained; disposition required before another allocation')
    cases, history = WORK_ROOT / 'cases', WORK_ROOT / 'case-history'
    if cases.is_symlink() or history.is_symlink():
        raise RuntimeError('indirect native case ledger directory')
    if not cases.exists() and not history.exists():
        return
    for directory, mode in [(cases, 0o711), (history, 0o700)]:
        if directory.is_symlink() or native(directory)[2:] != (stat.S_IFDIR | mode, 0, 0):
            raise RuntimeError('unsafe native case ledger directory')
    names = {p.name for p in cases.iterdir()}
    records = {p.name for p in history.iterdir()}
    if records != {name + '.json' for name in names}:
        raise RuntimeError('unrecorded native case or missing finalized history')
    for name in sorted(names):
        if not re.fullmatch(r'(preparation|installed)[1-9][0-9]*', name):
            raise RuntimeError('unknown native case name')
        record = read_control(history / (name + '.json'))
        case = cases / name
        if record.get('schema') != 'ardents-native-case-finalized-v1' or record.get('case') != name or record.get('state') != 'retained' or list(native(case)) != record.get('native') or case.is_symlink() or not case.is_dir():
            raise RuntimeError('changed finalized native case')
        for key in ['reservation_sha256', 'qa_verified_sha256', 'input_verified_sha256', 'disposition_sha256']:
            if not re.fullmatch(r'[0-9a-f]{64}', record.get(key, '')):
                raise RuntimeError('incomplete finalized native case proof')
        check_case_proofs(record)
        references(case)


def read_control(path):
    if path.is_symlink() or native(path)[2:] != (stat.S_IFREG | 0o600, 0, 0):
        raise RuntimeError('unsafe native stand control')
    with path.open('rb') as stream:
        body = stream.read(1 << 20)
        if stream.read(1):
            raise RuntimeError('oversized native stand control')
    return json.loads(body)


def write_control(path, record):
    body = (json.dumps(record, sort_keys=True, separators=(',', ':')) + '\n').encode()
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        os.fchmod(stream.fileno(), 0o600)
        stream.write(body)
        stream.flush()
        os.fsync(stream.fileno())
    sync_directory(path.parent)
    return hashlib.sha256(body).hexdigest()


def reserve_case(case, evidence):
    created = prepare_root()
    check_idle()
    if not re.fullmatch(r'(preparation|installed)[1-9][0-9]*', case):
        raise RuntimeError('closed native case name required')
    cases, history = WORK_ROOT / 'cases', WORK_ROOT / 'case-history'
    if not cases.exists():
        cases.mkdir(mode=0o711)
        cases.chmod(0o711)
        history.mkdir(mode=0o700)
        history.chmod(0o700)
        sync_directory(cases)
        sync_directory(history)
        sync_directory(WORK_ROOT)
    active = cases / case
    before = references(active)
    active.mkdir(mode=0o711)
    active.chmod(0o711)
    sync_directory(active)
    sync_directory(cases)
    result = {'at': time.time(), 'case': case, 'state': 'reserved', 'active': str(active), 'native': native(active), 'work_root_created': created, 'references': before, 'installed_admission': False}
    receipt = evidence / (case + '-reservation.json')
    # External QA must support file and directory synchronization too. Refuse
    # unavailable durability rather than releasing an unrecorded case.
    pin = write_control(receipt, result)
    write_control(WORK_ROOT / 'case-active.json', {'schema': 'ardents-native-case-reservation-v1', 'case': case, 'native': native(active), 'receipt': str(receipt), 'reservation_sha256': pin})
    return result


def retained_case(case, evidence, reservation_sha256):
    if not re.fullmatch(r'(preparation|installed)[1-9][0-9]*', case):
        raise RuntimeError('closed native case name required')
    prepare_root()
    marker = read_control(WORK_ROOT / 'case-active.json')
    receipt = evidence / (case + '-reservation.json')
    active = WORK_ROOT / 'cases' / case
    if marker.get('schema') != 'ardents-native-case-reservation-v1' or marker.get('case') != case or marker.get('receipt') != str(receipt) or marker.get('reservation_sha256') != reservation_sha256:
        raise RuntimeError('original native case reservation required')
    if receipt.is_symlink() or hashlib.sha256(receipt.read_bytes()).hexdigest() != reservation_sha256:
        raise RuntimeError('changed original native case receipt')
    original = json.loads(receipt.read_bytes())
    if active.resolve() != active or active.is_symlink() or original.get('active') != str(active) or original.get('case') != case or original.get('native') != list(native(active)) or marker.get('native') != original['native']:
        raise RuntimeError('changed original native case identity')
    return active, marker


def check_case_proofs(record):
    for key in ['qa_verified', 'input_verified', 'disposition']:
        path = Path(record[key])
        if not path.is_absolute() or path.resolve() != path or WORK_ROOT == path or WORK_ROOT in path.parents or not path.is_file():
            raise RuntimeError('original external retained-case proof required')
        with path.open('rb') as stream:
            if hashlib.file_digest(stream, 'sha256').hexdigest() != record[key + '_sha256']:
                raise RuntimeError('retained-case archive or disposition proof changed')


def release_case(case, evidence, reservation_sha256, proof):
    if set(proof) != {'qa_verified', 'qa_verified_sha256', 'input_verified', 'input_verified_sha256', 'disposition', 'disposition_sha256'}:
        raise RuntimeError('closed archive/disposition proof required')
    active, marker = retained_case(case, evidence, reservation_sha256)
    references(active)
    record = {'schema': 'ardents-native-case-finalized-v1', 'case': case, 'state': 'retained', 'native': native(active), 'reservation_sha256': reservation_sha256, **proof}
    for key in ['qa_verified_sha256', 'input_verified_sha256', 'disposition_sha256']:
        if not re.fullmatch(r'[0-9a-f]{64}', record.get(key, '')):
            raise RuntimeError('complete archive/disposition proof required')
    check_case_proofs(record)
    write_control(WORK_ROOT / 'case-history' / (case + '.json'), record)
    if read_control(WORK_ROOT / 'case-active.json') != marker:
        raise RuntimeError('original active reservation changed')
    # Final commit only after durable history; a lost unlink restores refusal.
    (WORK_ROOT / 'case-active.json').unlink()
    return record


def allocate():
    created = prepare_root()
    check_idle()
    active = WORK_ROOT / 'active'
    before = references(active)
    active.mkdir(mode=0o711)
    active.chmod(0o711)
    sync_directory(active)
    sync_directory(WORK_ROOT)
    return active, created, before
