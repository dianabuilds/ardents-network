#!/usr/bin/env python3
"""Explicit fresh custody of finalized evidence; no runtime or retry authority."""
import json
import os
from pathlib import Path
import stat
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_stand
import qa_archive
import qa_history


def private_directory(path):
    if not path.exists():
        path.mkdir(mode=0o700)
        native_stand.sync_directory(path)
        native_stand.sync_directory(path.parent)
    s = path.lstat()
    if path.resolve() != path or not stat.S_ISDIR(s.st_mode) or stat.S_IMODE(s.st_mode) != 0o700 or s.st_uid != 0 or s.st_gid != 0:
        raise RuntimeError('private direct observation directory required')


def observe(evidence):
    work = native_stand.WORK_ROOT
    repository = Path(__file__).resolve().parents[2]
    if evidence.resolve() != evidence or not evidence.is_dir() or repository == evidence or repository in evidence.parents or work == evidence or work in evidence.parents:
        raise RuntimeError('existing external evidence directory required')
    output = evidence / 'native-history-observation.json'
    if output.exists():
        raise RuntimeError('earlier observation report must not be overwritten')
    native_stand.prepare_root()
    if any(p.exists() or p.is_symlink() for p in (work / 'active', work / 'case-active.json')):
        raise RuntimeError('active native custody cannot be reobserved')
    cases, histories = work / 'cases', work / 'case-history'
    for directory, mode in ((cases, 0o711), (histories, 0o700)):
        if directory.resolve() != directory or native_stand.native(directory)[2:] != (stat.S_IFDIR | mode, 0, 0):
            raise RuntimeError('original direct case ledger required')
    names = {p.name for p in cases.iterdir()}
    if {p.name for p in histories.iterdir()} != {name + '.json' for name in names}:
        raise RuntimeError('case/history bijection required')
    cache, rows = {}, []
    base = work / 'history-observations'
    private_directory(base)
    private_directory(base / 'required')
    private_directory(base / 'snapshots')
    registered = qa_history.requirements(work)
    if not set(registered) <= names:
        raise RuntimeError('observation requirement without original finalized case')
    for name in sorted(names):
        path = qa_history.observation_path(work, name)
        case = cases / name
        history = histories / (name + '.json')
        record = native_stand.read_control(history)
        pin = qa_archive.sha(history)
        native_stand.check_case_proofs(record)
        if path.exists() or path.is_symlink():
            observation = qa_history.checked_observation(work, case, record, cache)
        else:
            original = qa_history.inspect(case, record, cache)
            native_stand.references(case)
            if original != qa_history.inspect(case, record, cache) or qa_archive.sha(history) != pin:
                raise RuntimeError('finalized evidence changed during observation')
            private_directory(path.parent.parent)
            private_directory(path.parent)
            if name not in registered:
                native_stand.write_control(base / 'required' / (name + '.json'),
                    {'schema': 'ardents-qa-history-required-v1', 'case': name, 'history_sha256': pin})
                registered[name] = {'history_sha256': pin}
            observation = {'schema': 'ardents-qa-history-observation-v1', 'boot_id': qa_history.boot_id(),
                           'case': name, 'history_sha256': pin, 'observer_sha256': qa_history.observer_pin(),
                           'original_native': record['native'], 'current_native': list(native_stand.native(case)), 'entries': original}
            native_stand.write_control(path, observation)
            qa_history.checked_observation(work, case, record, cache)
        native_stand.references(case)
        rows.append({'case': name, 'history_sha256': pin, 'observation_sha256': qa_archive.sha(path), 'entries': len(observation['entries'])})
        print(json.dumps(rows[-1]), flush=True)
    native_stand.check_idle()
    report = {'schema': 'ardents-qa-history-observation-report-v1', 'boot_id': qa_history.boot_id(),
              'cases': rows, 'historical_records_unchanged': True, 'runtime_authority': False,
              'installed_permission': False, 'original_identity_continuity_claimed': False}
    native_stand.write_control(output, report)
    return report


def main():
    if sys.platform != 'linux' or sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or os.geteuid() != 0 or os.getegid() != 0 or len(sys.argv) != 2:
        raise RuntimeError('Linux root Python3.12 -I -S and external evidence required')
    print(json.dumps(observe(Path(sys.argv[1]))), flush=True)


if __name__ == '__main__':
    main()
