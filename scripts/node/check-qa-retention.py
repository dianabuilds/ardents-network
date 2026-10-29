#!/usr/bin/env python3
"""Native lifecycle fixtures, without product installation or bootstrap rights.

All native objects are born in one disposable ext4 fixture root. Its successful
teardown preserves complete private archives outside Git; a failed check retains
the native fixture and all observations. Never uses a different production lock.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import struct
import sys
import tempfile
import zipfile

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_stand
import qa_archive

spec = importlib.util.spec_from_file_location('qa_case_finalizer', Path(__file__).with_name('finalize-native-stand.py'))
finalizer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(finalizer)

CONTROLLER = '''import hashlib,json,os,shutil,socket,sys
from pathlib import Path
root=Path(sys.argv[1]);source=Path(sys.argv[2]);reservation=sys.argv[3];case=sys.argv[4];code=int(sys.argv[5])
for name in ('program-A','program-B'):shutil.copyfile(source,root/name)
shutil.copyfile(sys.argv[6],root/'small-program')
state=root/'state';state.write_bytes(b'original state and history\\n');os.chmod(state,0o600)
os.setxattr(state,'user.qa-retention',b'original')
os.link(state,root/'state-link');(root/'state-alias').symlink_to('state')
os.mkfifo(root/'ended-pipe')
with socket.socket(socket.AF_UNIX) as ended:ended.bind(str(root/'ended.socket'))
result={'schema':'ardents-native-case-result-v1','case':case,'reservation_sha256':reservation,
'controller_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'test_exit':code,
'original_scope_joined':True,'original_resources_joined':True}
with (root/'case-result.json').open('x') as f:json.dump(result,f);f.flush();os.fsync(f.fileno())
sys.exit(code)
'''


def require_refusal(action, note, errors=(RuntimeError,)):
    try:
        action()
    except errors:
        print('REFUSED as required:', note, flush=True)
    else:
        raise AssertionError('missing refusal: ' + note)


def check_payload_compression(output):
    directory = output / 'compression-controls'
    qa_archive.make_directory(directory)
    body = b'bounded private QA payload\n'
    member = 'sha256/' + hashlib.sha256(body).hexdigest()
    for method in [zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED]:
        path = directory / ('allowed-' + str(method) + '.zip')
        with zipfile.ZipFile(path, 'x', compression=method) as archive:
            archive.writestr(member, body)
        with qa_archive.open_payloads(path) as archive:
            with archive.open(member) as source:
                assert source.read(len(body) + 1) == body
    source = (directory / 'allowed-0.zip').read_bytes()
    assert source[:4] == b'PK\x03\x04'
    central = source.index(b'PK\x01\x02')
    for method, flags in [(12, 0), (14, 0), (93, 0), (99, 0), (0, 1), (0, 0x40), (0, 0x2000)]:
        # A small literal metadata vector, never a decompression bomb. Both
        # headers agree; rejection must precede selection of any decompressor.
        raw = bytearray(source)
        struct.pack_into('<H', raw, 8, method)
        struct.pack_into('<H', raw, central + 10, method)
        struct.pack_into('<H', raw, 6, flags)
        struct.pack_into('<H', raw, central + 8, flags)
        path = directory / ('refused-' + str(method) + '-' + str(flags) + '.zip')
        with path.open('xb') as target:
            target.write(raw)
            target.flush()
            os.fsync(target.fileno())
        original = zipfile._get_decompressor
        def unexpected_decompressor(*args):
            raise AssertionError('unadmitted QA decompressor reached')
        zipfile._get_decompressor = unexpected_decompressor
        try:
            require_refusal(lambda: qa_archive.open_payloads(path), 'payload method/encryption before decompressor')
        finally:
            zipfile._get_decompressor = original
    qa_archive.sync_directory(directory)



def check_keeper_hash_coverage(base, output, expected, pack):
    """Each removed body needs its own fresh keeper, plus the global brackets."""
    programs = [base / 'canonical-program', base / 'small-canonical-program']
    roots = [base / 'keeper-coverage-one', base / 'keeper-coverage-two']
    copies = []
    for root, program in zip(roots, programs):
        qa_archive.make_directory(root)
        copy = root / 'copied-program'
        copy.write_bytes(program.read_bytes())
        copies.append(str(copy))
    qualification = output / 'keeper-coverage-qualification.json'
    qa_archive.save_json(qualification, {'qualified_terminal_qa': True, 'roots': [str(root) for root in roots],
                                       'receipts': [{'fixture': 'two closed copied-file roots; no spawned runtime'}]})
    archive = output / 'keeper-coverage-archive'
    qa_archive.preserve(qualification, archive, pack)
    inputs = json.loads((pack / 'manifest.private.json').read_bytes())['current_entries']
    plan = {'owner_qualified_duplicate_files': True, 'files': copies,
            'full_qa_archive': str(archive), 'full_qa_verified_sha256': qa_archive.sha(archive / 'verified.json'),
            'keep_original_canonical': {expected[str(program)]: str(program) for program in programs},
            'immutable_keeper_inputs': {str(program): inputs[str(program)] for program in programs},
            'input_closure': str(pack), 'input_closure_verified_sha256': qa_archive.sha(pack / 'verified.json')}
    plan_path = output / 'keeper-coverage-plan.json'
    qa_archive.save_json(plan_path, plan)
    duplicates = finalizer.qa_duplicates
    original_sha = duplicates.m.sha
    calls = {str(program): 0 for program in programs}
    def observed_sha(path):
        name = str(path)
        if name in calls:
            calls[name] += 1
        return original_sha(path)  # Observe real reads; never supply a hash.
    duplicates.m.sha = observed_sha
    try:
        duplicates.execute(plan_path, output / 'keeper-coverage-disposition', apply=True,
                           expected_sha=qa_archive.sha(plan_path))
    finally:
        duplicates.m.sha = original_sha
    assert calls == {str(program): 3 for program in programs}, calls
    assert all(not Path(copy).exists() for copy in copies)
    assert all(qa_archive.sha(program) == expected[str(program)] for program in programs)
    qa_archive.save_json(output / 'keeper-coverage-results.json', {
        'two_original_roots': True, 'exact_keeper_sha_reads': calls,
        'keeper_bytes_hashed': sum(program.stat().st_size * calls[str(program)] for program in programs),
        'initial_and_final_global_checks_retained': True, 'each_root_checks_its_own_targets': True,
        'actual_duplicate_files_removed': len(copies), 'product_acceptance': False})


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or os.geteuid() != 0 or os.getegid() != 0 or len(sys.argv) != 2:
        raise RuntimeError('Linux root Python3.12 -I -S and fresh external evidence required')
    output = Path(sys.argv[1])
    repository = Path(__file__).resolve().parents[2]
    if not output.is_absolute() or output.resolve() != output or repository == output or repository in output.parents:
        raise RuntimeError('fresh external fixture evidence directory required')
    qa_archive.make_directory(output)
    check_payload_compression(output)
    base = Path(tempfile.mkdtemp(prefix='ardents-qa-retention-', dir='/var/tmp'))
    original_base = native_stand.native(base)
    qa_archive.save_json(output / 'native-fixture.json', {'base': str(base), 'native': original_base, 'no_installed_qualification': True})
    # The production root has no override. Only this in-process test isolates
    # its own native births; real proc/mount/unit/container guards still run.
    native_stand.WORK_ROOT = base / 'work'
    os.umask(0o077)
    controller = base / 'controller.py'
    controller.write_text(CONTROLLER, encoding='utf8')
    program = base / 'canonical-program'
    program.write_bytes(b'\x7fELF' + b'Q' * (4 * 1024 * 1024))
    os.chmod(program, 0o555)
    small_program = base / 'small-canonical-program'
    small_program.write_bytes(b'\x7fELFsmall immutable fixture')
    os.chmod(small_program, 0o555)
    expected = {str(path): qa_archive.sha(path) for path in [controller, program, small_program]}
    pack = output / 'inputs'
    qa_archive.preserve_inputs(expected, pack)
    pack_sha = qa_archive.sha(pack / 'original-inputs.zip')
    observations = []

    def produce(case, code=0):
        result = native_stand.reserve_case(case, output)
        root = Path(result['active'])
        reservation = qa_archive.sha(output / (case + '-reservation.json'))
        completed = subprocess.run([sys.executable, '-I', '-S', str(controller), str(root), str(program), reservation, case, str(code), str(small_program)], check=False)
        if completed.returncode != code:
            raise AssertionError('fixture controller failed unexpectedly')
        terminal = {
            'schema': 'ardents-native-case-terminal-v1', 'case': case, 'reservation_sha256': reservation,
            'test_exit': code, 'original_scope_joined': True, 'original_resources_joined': True,
            'controller': {'path': str(controller), 'sha256': expected[str(controller)]},
            'result': {'path': str(root / 'case-result.json'), 'sha256': qa_archive.sha(root / 'case-result.json')},
            'immutable_inputs': expected,
            'input_closure': {'directory': str(pack), 'verified_sha256': qa_archive.sha(pack / 'verified.json')},
            'disposable_binary_paths': [str(root / 'program-A'), str(root / 'program-B'), str(root / 'small-program')],
        }
        return root, terminal

    def complete(case, terminal, suffix=''):
        path = output / (case + '-terminal' + suffix + '.json')
        qa_archive.save_json(path, terminal)
        return finalizer.finalize(output, case, path, qa_archive.sha(path))

    first_state = None
    for case in ['preparation900101', 'installed900102']:
        root, terminal = produce(case)
        before = qa_archive.inventory([str(root)])
        require_refusal(native_stand.allocate, 'component allocation while a case is active')
        require_refusal(lambda: native_stand.reserve_case('preparation900199', output), 'a second preparation or installed case')
        if case.startswith('preparation'):
            bad = {**terminal, 'original_scope_joined': False}
            require_refusal(lambda: complete(case, bad, '-unknown-join'), 'unknown original join')
            bad = {**terminal, 'disposable_binary_paths': terminal['disposable_binary_paths'] + [str(program)]}
            require_refusal(lambda: complete(case, bad, '-foreign-path'), 'disposable path outside the exact case')
            with (root / 'state').open('rb'):
                require_refusal(lambda: complete(case, terminal, '-held-reader'), 'still-held original state descriptor')
            packed = pack / 'original-inputs.zip'
            original_pack = packed.read_bytes()
            packed.write_bytes(original_pack + b'controlled archive corruption')
            (output / 'corrupted-input-pack.zip').write_bytes(packed.read_bytes())
            require_refusal(lambda: complete(case, terminal, '-corrupt-pack'), 'corrupted shared input archive')
            packed.write_bytes(original_pack)
        complete(case, terminal)
        after = qa_archive.inventory([str(root)])
        assert set(after) == set(before) - set(terminal['disposable_binary_paths'])
        for name, record in after.items():
            ignored = {'mtime_ns', 'ctime_ns', 'size'} if name == str(root) else set()
            assert {k: v for k, v in record.items() if k not in ignored} == {k: v for k, v in before[name].items() if k not in ignored}
        assert not (native_stand.WORK_ROOT / 'case-active.json').exists()
        assert qa_archive.sha(program) == expected[str(program)]
        assert qa_archive.sha(small_program) == expected[str(small_program)]
        if first_state is None:
            first_state = (root / 'state', qa_archive.facts(root / 'state'))
        else:
            assert qa_archive.facts(first_state[0]) == first_state[1]
        archived = output / (case + '-finalized') / 'qa'
        manifest = json.loads((archived / 'manifest.private.json').read_bytes())
        assert manifest['external_pack']['directory'] == str(pack)
        with qa_archive.open_payloads(archived / 'qa-payloads.zip') as z:
            assert 'sha256/' + expected[str(program)] not in z.namelist()
            assert 'sha256/' + expected[str(small_program)] not in z.namelist()
        observations.append({'case': case, 'state_retained_in_place': True, 'duplicate_files_removed': 3, 'small_binary_copy_removed': True, 'shared_input_pack': pack_sha})
    assert qa_archive.sha(pack / 'original-inputs.zip') == pack_sha
    native_stand.check_idle()
    check_keeper_hash_coverage(base, output, expected, pack)
    proof = output / 'preparation900101-finalized' / 'disposition' / 'applied.json'
    original_proof = proof.read_bytes()
    proof.write_bytes(original_proof + b' ')
    require_refusal(native_stand.check_idle, 'changed retained-case disposition proof')
    proof.write_bytes(original_proof)
    component, _, _ = native_stand.allocate()
    require_refusal(lambda: native_stand.reserve_case('preparation900103', output), 'case allocation while component slot is occupied')
    native_stand.references(component)
    component.rmdir()
    root, terminal = produce('preparation900103')
    before_sync_failure = qa_archive.inventory([str(root)])
    original_sync = qa_archive.sync_directory
    def unavailable_sync(path):
        if path == output / 'preparation900103-finalized':
            raise OSError('controlled unavailable archive directory synchronization')
        original_sync(path)
    qa_archive.sync_directory = unavailable_sync
    try:
        require_refusal(lambda: complete('preparation900103', terminal), 'archive directory synchronization unavailable', (OSError,))
    finally:
        qa_archive.sync_directory = original_sync
    assert qa_archive.inventory([str(root)]) == before_sync_failure
    require_refusal(native_stand.allocate, 'unsynchronized archive retains the active reservation')
    qualification = output / 'sync-failure-fixture-qualification.json'
    qa_archive.save_json(qualification, {'qualified_terminal_qa': True, 'roots': [str(root)], 'receipts': [terminal]})
    qa_archive.preserve(qualification, output / 'sync-failure-fixture-archive', pack)
    # Actual failed product results remain failures. Known physical completion
    # still permits verified forensic archival and exact copy disposition.
    native_stand.WORK_ROOT = base / 'failed-work'
    failed_observations = []
    for case, code in [('preparation900104', 7), ('installed900105', 23)]:
        root, failed = produce(case, code)
        before = qa_archive.inventory([str(root)])
        result_raw = (root / 'case-result.json').read_bytes()
        require_refusal(native_stand.allocate, 'unarchived failed case remains occupied')
        bad = {**failed, 'original_resources_joined': False}
        require_refusal(lambda: complete(case, bad, '-unknown-resources'), 'failed case with unknown original resources')
        bad = {**failed, 'test_exit': 0}
        require_refusal(lambda: complete(case, bad, '-forged-pass'), 'failed original result cannot become zero')
        with (root / 'state').open('rb'):
            require_refusal(lambda: complete(case, failed, '-held-reader'), 'failed case still-held original state descriptor')
        assert qa_archive.inventory([str(root)]) == before
        complete(case, failed)
        after = qa_archive.inventory([str(root)])
        assert set(after) == set(before) - set(failed['disposable_binary_paths'])
        for name, record in after.items():
            ignored = {'mtime_ns', 'ctime_ns', 'size'} if name == str(root) else set()
            assert {k: v for k, v in record.items() if k not in ignored} == {k: v for k, v in before[name].items() if k not in ignored}
        assert (root / 'case-result.json').read_bytes() == result_raw
        assert json.loads(result_raw)['test_exit'] == code
        qualification = json.loads((output / (case + '-finalized') / 'qualification.private.json').read_bytes())
        assert qualification['original_test_exit'] == code and qualification['product_acceptance'] is False
        assert not (native_stand.WORK_ROOT / 'case-active.json').exists()
        native_stand.check_idle()
        require_refusal(lambda: native_stand.reserve_case(case, output), 'archived failed environment cannot be adopted', (OSError, RuntimeError))
        assert qa_archive.facts(first_state[0]) == first_state[1]
        assert qa_archive.sha(program) == expected[str(program)]
        assert qa_archive.sha(small_program) == expected[str(small_program)]
        failed_observations.append({'case': case, 'actual_test_exit': code, 'original_result_preserved': True,
                                    'product_acceptance': False, 'duplicate_files_removed': 3,
                                    'state_retained_in_place': True, 'next_case_requires_fresh_reservation': True})
    native_stand.references(base)
    assert native_stand.native(base) == original_base
    # Only this exact original disposable test tree is retired; all successful
    # and expected-failure QA/input archives remain in external evidence.
    for directory, dirs, files in os.walk(base, topdown=False, followlinks=False):
        for name in files:
            (Path(directory) / name).unlink()
        for name in dirs:
            path = Path(directory) / name
            path.unlink() if path.is_symlink() else path.rmdir()
    base.rmdir()
    qa_archive.save_json(output / 'results.json', {'passed': True, 'cases': observations, 'failed_cases': failed_observations, 'unknown_failed_case_retains_reservation': True, 'directory_sync_failure_keeps_copies_and_reservation': True, 'physical_archive_restore': True, 'no_installed_qualification': True})
    print('PASS: sequential original-path cases, shared binary pack, exact duplicate disposition, state/history/links/xattrs preserved, known-ended failed-case archival, active/unknown/mismatched-exit/held-reader refusals', flush=True)


if __name__ == '__main__':
    try:
        main()
    except BaseException as error:
        print('QA retention fixture failed; native root and evidence retained:', error, file=sys.stderr)
        raise
