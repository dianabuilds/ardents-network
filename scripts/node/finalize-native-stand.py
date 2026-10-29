#!/usr/bin/env python3
"""Archive one genuinely joined QA case and retire its declared binary copies.

Run under the existing shared exclusive development wrapper. This is private QA
retention, never installed/bootstrap/recovery authority. Original state and QA
paths remain in place. Any failure retains the single active reservation.
"""
import base64
import json
import os
from pathlib import Path
import stat
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_stand
import qa_archive
import qa_duplicates


def pinned_json(path, pin):
    if not path.is_absolute() or path.resolve() != path or not path.is_file():
        raise RuntimeError('direct original proof file required')
    before = qa_archive.facts(path)
    raw = path.read_bytes()
    if qa_archive.sha(path) != pin or qa_archive.facts(path) != before:
        raise RuntimeError('original proof bytes or identity changed')
    return json.loads(raw), raw


def verified_inputs(directory, pin, expected):
    receipt, _ = pinned_json(directory / 'verified.json', pin)
    manifest, _ = pinned_json(directory / 'manifest.private.json', receipt['manifest_sha256'])
    if receipt.get('verified') is not True or receipt.get('physical_input_restore') is not True or receipt.get('archive_name') != 'original-inputs.zip' or manifest.get('schema') != 'ardents-qa-inputs-v1' or manifest.get('self_contained_input_bodies') is not True or manifest.get('source_unchanged') is not True:
        raise RuntimeError('complete independently restored immutable input pack required')
    if qa_archive.sha(directory / 'original-inputs.zip') != receipt['archive_sha256']:
        raise RuntimeError('immutable input archive changed')
    entries = manifest['current_entries']
    if not expected or set(entries) != set(expected) or manifest['paths'] != entries:
        raise RuntimeError('complete original input set required')
    for name, record in entries.items():
        path = Path(name)
        if path.resolve() != path or record['sha256'] != expected[name] or qa_archive.facts(path) != {k: v for k, v in record.items() if k != 'sha256'} or qa_archive.sha(path) != expected[name]:
            raise RuntimeError('original immutable input changed: ' + name)
    return entries


def finalize(evidence, case, terminal_path, terminal_sha256):
    repository = Path(__file__).resolve().parents[2]
    if not evidence.is_absolute() or evidence.resolve() != evidence or not evidence.is_dir() or repository == evidence or repository in evidence.parents or native_stand.WORK_ROOT == evidence or native_stand.WORK_ROOT in evidence.parents:
        raise RuntimeError('existing external QA directory required')
    terminal, terminal_raw = pinned_json(terminal_path, terminal_sha256)
    required = {'schema', 'case', 'reservation_sha256', 'test_exit', 'original_scope_joined', 'original_resources_joined', 'controller', 'result', 'immutable_inputs', 'input_closure', 'disposable_binary_paths'}
    if set(terminal) != required or terminal['schema'] != 'ardents-native-case-terminal-v1' or terminal['case'] != case:
        raise RuntimeError('exact original terminal case proof required')
    active, _ = native_stand.retained_case(case, evidence, terminal['reservation_sha256'])
    if type(terminal['test_exit']) is not int or terminal['original_scope_joined'] is not True or terminal['original_resources_joined'] is not True:
        raise RuntimeError('unknown original exit or joins retain the active case')
    native_stand.references(active)
    closure = Path(terminal['input_closure']['directory'])
    inputs = verified_inputs(closure, terminal['input_closure']['verified_sha256'], terminal['immutable_inputs'])
    controller = terminal['controller']
    if controller['path'] not in inputs or inputs[controller['path']]['sha256'] != controller['sha256']:
        raise RuntimeError('original controller is not a pinned immutable input')
    result_path = Path(terminal['result']['path'])
    if active not in result_path.parents:
        raise RuntimeError('original case result must remain in its own QA tree')
    result, result_raw = pinned_json(result_path, terminal['result']['sha256'])
    if result.get('schema') != 'ardents-native-case-result-v1' or result.get('case') != case or result.get('controller_sha256') != controller['sha256'] or result.get('reservation_sha256') != terminal['reservation_sha256'] or type(result.get('test_exit')) is not int or result['test_exit'] != terminal['test_exit'] or result.get('original_scope_joined') is not True or result.get('original_resources_joined') is not True:
        raise RuntimeError('actual original controller/result join proof required')
    entries = qa_archive.inventory([str(active)])
    declared = terminal['disposable_binary_paths']
    if not isinstance(declared, list) or len(declared) != len(set(declared)):
        raise RuntimeError('exact finite disposable binary paths required')
    original_elf = set()
    for name, record in entries.items():
        if stat.S_ISREG(record['mode']) and record['size'] >= 4:
            with Path(name).open('rb') as stream:
                if stream.read(4) == b'\x7fELF':
                    original_elf.add(name)
    if set(declared) != original_elf:
        raise RuntimeError('unknown or missing disposable binary; retain the case')
    keepers, keeper_inputs = {}, {}
    for name in declared:
        record = entries[name]
        if record['nlink'] != 1:
            raise RuntimeError('linked original binary needs explicit preservation')
        digest = record['sha256']
        candidates = sorted(path for path, original in inputs.items() if original['sha256'] == digest and active not in Path(path).parents)
        if not candidates:
            raise RuntimeError('immutable canonical binary keeper required')
        keepers[digest] = candidates[0]
        keeper_inputs[candidates[0]] = inputs[candidates[0]]
    output = evidence / (case + '-finalized')
    qa_archive.make_directory(output)
    qualification = output / 'qualification.private.json'
    qa_archive.save_json(qualification, {
        'qualified_terminal_qa': True, 'roots': [str(active)],
        'original_test_exit': terminal['test_exit'], 'product_acceptance': False,
        'receipts': [{'terminal_sha256': terminal_sha256, 'terminal_base64': base64.b64encode(terminal_raw).decode(),
                      'result_base64': base64.b64encode(result_raw).decode(), 'controller_sha256': controller['sha256']}],
    })
    native_stand.retained_case(case, evidence, terminal['reservation_sha256'])
    native_stand.references(active)
    archive = output / 'qa'
    qa_archive.preserve(qualification, archive, closure)
    if qa_archive.inventory([str(active)]) != entries:
        raise RuntimeError('original case changed during archival')
    plan = {
        'owner_qualified_duplicate_files': True, 'files': declared,
        'full_qa_archive': str(archive), 'full_qa_verified_sha256': qa_archive.sha(archive / 'verified.json'),
        'keep_original_canonical': keepers, 'immutable_keeper_inputs': keeper_inputs,
        'input_closure': str(closure), 'input_closure_verified_sha256': terminal['input_closure']['verified_sha256'],
    }
    plan_path = output / 'disposition-plan.private.json'
    qa_archive.save_json(plan_path, plan)
    disposition = output / 'disposition'
    native_stand.retained_case(case, evidence, terminal['reservation_sha256'])
    pinned_json(terminal_path, terminal_sha256)
    verified_inputs(closure, terminal['input_closure']['verified_sha256'], terminal['immutable_inputs'])
    if declared:
        # The exact generated plan is bound to the externally pinned terminal
        # path set and restored immutable input pack, not a generic cleanup flag.
        qa_duplicates.execute(plan_path, disposition, apply=True, expected_sha=qa_archive.sha(plan_path))
    else:
        native_stand.references(active)
        if qa_archive.inventory([str(active)]) != entries:
            raise RuntimeError('case changed before empty disposition')
        qa_archive.make_directory(disposition)
        qa_archive.save_json(disposition / 'applied.json', {'deleted_files': 0, 'remaining_entries_unchanged': True})
    # The actual test result remains in both pinned original records and the
    # full archive. Physical disposition grants no product pass or retry.
    # Root and metadata stay at their original paths. Only this fully completed
    # case's active reservation may be released after its durable archive proof.
    return native_stand.release_case(case, evidence, terminal['reservation_sha256'], {
        'qa_verified': str(archive / 'verified.json'), 'qa_verified_sha256': qa_archive.sha(archive / 'verified.json'),
        'input_verified': str(closure / 'verified.json'), 'input_verified_sha256': terminal['input_closure']['verified_sha256'],
        'disposition': str(disposition / 'applied.json'), 'disposition_sha256': qa_archive.sha(disposition / 'applied.json'),
    })


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or os.geteuid() != 0:
        raise RuntimeError('Linux root Python3.12 -I -S required')
    if len(sys.argv) != 5:
        raise RuntimeError('usage: finalize-native-stand.py EXTERNAL_EVIDENCE CASE TERMINAL_JSON TERMINAL_SHA256')
    print(json.dumps(finalize(Path(sys.argv[1]), sys.argv[2], Path(sys.argv[3]), sys.argv[4])), flush=True)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError) as error:
        print('QA case finalization refused; original slot retained:', error, file=sys.stderr)
        sys.exit(2)
