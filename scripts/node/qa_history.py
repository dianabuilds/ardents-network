"""Fresh custody of finalized QA evidence, never original runtime identity.

Linux device numbers can differ between native environments. Original receipts
remain immutable. Only fully finalized, archived, disposed history may acquire
a separate current-boot observation; active reservations have no such path.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat

import qa_archive


def boot_id():
    value = Path('/proc/sys/kernel/random/boot_id').read_text().strip()
    if not re.fullmatch(r'[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}', value):
        raise RuntimeError('actual kernel boot identity required')
    return value


def observer_pin():
    return qa_archive.sha(Path(__file__))


def private(path, mode, directory=False):
    s = path.lstat()
    kind = stat.S_ISDIR if directory else stat.S_ISREG
    if path.resolve() != path or not kind(s.st_mode) or stat.S_IMODE(s.st_mode) != mode or s.st_uid != 0 or s.st_gid != 0 or (not directory and s.st_nlink != 1):
        raise RuntimeError('private direct historical evidence custody required')


def requirements(work_root):
    """An observed case cannot return to the legacy path when a snapshot is lost."""
    base = Path(work_root) / 'history-observations'
    if not base.exists() and not base.is_symlink():
        return {}
    private(base, 0o700, True)
    if {p.name for p in base.iterdir()} != {'required', 'snapshots'}:
        raise RuntimeError('complete historical observation registry required')
    for directory in (base / 'required', base / 'snapshots'):
        private(directory, 0o700, True)
    result = {}
    for path in (base / 'required').iterdir():
        if not re.fullmatch(r'(preparation|installed)[1-9][0-9]*\.json', path.name):
            raise RuntimeError('unknown historical observation requirement')
        private(path, 0o600)
        row = json.loads(path.read_bytes())
        case = path.stem
        history = Path(work_root) / 'case-history' / path.name
        if set(row) != {'schema', 'case', 'history_sha256'} or row['schema'] != 'ardents-qa-history-required-v1' or row['case'] != case or qa_archive.sha(history) != row['history_sha256']:
            raise RuntimeError('historical observation requirement changed')
        result[case] = row
    if {p.name for p in (base / 'snapshots').iterdir()} != set(result):
        raise RuntimeError('historical observation registry and case set differ')
    return result


def pinned(path, digest, cache):
    path = Path(path)
    if not path.is_absolute() or path.resolve() != path or not path.is_file():
        raise RuntimeError('direct retained QA proof required')
    before = qa_archive.facts(path)
    key = (str(path), json.dumps(before, sort_keys=True))
    if key not in cache:
        cache[key] = qa_archive.sha(path)
    if cache[key] != digest or qa_archive.facts(path) != before:
        raise RuntimeError('retained QA proof changed: ' + str(path))
    return path


def inspect(root, record, cache):
    """Compare retained evidence to its authenticated pre-disposition inventory."""
    root = Path(root)
    if record.get('schema') != 'ardents-native-case-finalized-v1' or record.get('state') != 'retained' or record.get('case') != root.name:
        raise RuntimeError('only finalized retained QA history may be observed')
    receipts = {key: pinned(record[key], record[key + '_sha256'], cache)
                for key in ('qa_verified', 'input_verified', 'disposition')}
    verified = json.loads(receipts['qa_verified'].read_bytes())
    if not all(verified.get(key) is True for key in ('verified', 'physical_full_package_restore', 'source_unchanged')):
        raise RuntimeError('complete original physical QA verification required')
    archive = receipts['qa_verified'].parent
    manifest_path = pinned(archive / 'manifest.private.json', verified['manifest_sha256'], cache)
    manifest = json.loads(manifest_path.read_bytes())
    payload = pinned(archive / 'qa-payloads.zip', verified['archive_sha256'], cache)
    with qa_archive.open_payloads(payload):
        pass
    external = manifest.get('external_pack')
    if external:
        base = Path(external['directory'])
        pinned(base / 'manifest.private.json', external['manifest_sha256'], cache)
        if external['archive_name'] not in ('binary-payloads.zip', 'original-inputs.zip'):
            raise RuntimeError('closed historical input archive required')
        with qa_archive.open_payloads(pinned(base / external['archive_name'], external['archive_sha256'], cache)):
            pass
    qualification = manifest['qualification']
    if manifest['roots'] != [str(root)] or qualification.get('roots') != [str(root)] or qualification.get('qualified_terminal_qa') is not True:
        raise RuntimeError('exact original finalized QA root required')
    if len(qualification['receipts']) != 1:
        raise RuntimeError('one original terminal receipt required')
    receipt = qualification['receipts'][0]
    raw = base64.b64decode(receipt['terminal_base64'], validate=True)
    if hashlib.sha256(raw).hexdigest() != receipt['terminal_sha256']:
        raise RuntimeError('original terminal commitment changed')
    terminal = json.loads(raw)
    if terminal.get('schema') != 'ardents-native-case-terminal-v1' or terminal.get('case') != root.name or terminal.get('reservation_sha256') != record['reservation_sha256'] or terminal.get('original_scope_joined') is not True or terminal.get('original_resources_joined') is not True:
        raise RuntimeError('original joined terminal case required')
    expected = manifest['entries']
    deleted = terminal['disposable_binary_paths']
    if not isinstance(deleted, list) or len(deleted) != len(set(deleted)) or any(str(root) == name or root not in Path(name).parents for name in deleted):
        raise RuntimeError('exact original disposable binary set required')
    disposition = json.loads(receipts['disposition'].read_bytes())
    if disposition.get('deleted_files') != len(deleted):
        raise RuntimeError('original disposition count changed')
    parents = {str(Path(name).parent) for name in deleted}
    if deleted and (disposition.get('remaining_entries_unchanged_except_parent_directory_metadata') is not True or disposition.get('changed_parent_directories') != sorted(parents) or disposition.get('full_qa_archive') != str(archive)):
        raise RuntimeError('complete original duplicate disposition required')
    if not deleted and disposition.get('remaining_entries_unchanged') is not True:
        raise RuntimeError('complete empty disposition required')
    log = receipts['disposition'].parent / 'deleted.jsonl'
    rows = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
    if len(rows) != len(deleted) or {row['path'] for row in rows} != set(deleted) or any(row['original'] != expected[row['path']] for row in rows):
        raise RuntimeError('original deletion transcript changed')
    current = qa_archive.inventory([str(root)])
    if set(current) != set(expected) - set(deleted):
        raise RuntimeError('retained QA entry set changed')
    original_device = record['native'][0]
    current_device = root.stat().st_dev
    for name, actual in current.items():
        original = expected[name]
        if original['device'] != original_device or actual['device'] != current_device:
            raise RuntimeError('mixed original or current QA filesystems')
        ignored = {'device'} | ({'size', 'mtime_ns', 'ctime_ns'} if name in parents else set())
        if {k: v for k, v in original.items() if k not in ignored} != {k: v for k, v in actual.items() if k not in ignored}:
            raise RuntimeError('retained QA content or native metadata changed: ' + name)
    return current


def observation_path(work_root, name):
    if not re.fullmatch(r'(preparation|installed)[1-9][0-9]*', name):
        raise RuntimeError('closed historical case name required')
    return Path(work_root) / 'history-observations' / 'snapshots' / name / boot_id() / (observer_pin() + '.json')


def checked_observation(work_root, root, record, cache):
    """Read only a separately acquired current-boot evidence snapshot."""
    path = observation_path(work_root, root.name)
    if root.name not in requirements(work_root):
        raise RuntimeError('persistent historical observation requirement missing')
    if not path.exists() and not path.is_symlink():
        raise RuntimeError('fresh current-boot observer-version snapshot required')
    for directory in (path.parent.parent.parent.parent, path.parent.parent.parent, path.parent.parent, path.parent):
        private(directory, 0o700, True)
    private(path, 0o600)
    before = qa_archive.facts(path)
    observed = json.loads(path.read_bytes())
    history = Path(work_root) / 'case-history' / (root.name + '.json')
    native = list((lambda s: (s.st_dev, s.st_ino, s.st_mode, s.st_uid, s.st_gid))(root.lstat()))
    if set(observed) != {'schema', 'boot_id', 'case', 'history_sha256', 'observer_sha256', 'original_native', 'current_native', 'entries'} or observed['schema'] != 'ardents-qa-history-observation-v1' or observed['boot_id'] != boot_id() or observed['case'] != root.name or observed['history_sha256'] != qa_archive.sha(history) or observed['observer_sha256'] != observer_pin() or observed['original_native'] != record['native'] or observed['current_native'] != native:
        raise RuntimeError('current exact historical observation required')
    if observed['entries'] != inspect(root, record, cache) or qa_archive.facts(path) != before:
        raise RuntimeError('retained evidence changed after observation')
    return observed
