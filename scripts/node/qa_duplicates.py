"""Exact allowlisted ELF unlink, only after verified full QA archive.

Caller holds original shared development lock. No recursive source deletion,
no live-inode replacement, no unit/container/process changes.
"""
import hashlib,importlib.util,json,os,stat,sys,time
from pathlib import Path
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('qa_archive',Path(__file__).with_name('qa_archive.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
sys.path.insert(0,str(Path(__file__).resolve().parent))
from native_stand import references

def execute(plan_path,output,apply=False,expected_sha=None):
    plan=json.loads(Path(plan_path).read_text());out=Path(output)
    plan_sha=m.sha(plan_path)
    if apply and not expected_sha:raise RuntimeError('Apply requires externally pinned plan SHA256')
    if expected_sha and expected_sha!=plan_sha:raise RuntimeError('Plan differs from external reviewed SHA256')
    if out.exists():raise RuntimeError('Output must be new')
    if plan.get('owner_qualified_duplicate_files') is not True or not plan.get('files'):raise RuntimeError('Requires exact owner file qualification')
    pack=Path(plan['full_qa_archive']);receipt=json.loads((pack/'verified.json').read_text());manifest=json.loads((pack/'manifest.private.json').read_text())
    if m.sha(pack/'verified.json')!=plan['full_qa_verified_sha256']:raise RuntimeError('Full QA receipt differs from plan')
    if not receipt['verified'] or not receipt['physical_full_package_restore'] or m.sha(pack/'manifest.private.json')!=receipt['manifest_sha256'] or m.sha(pack/'qa-payloads.zip')!=receipt['archive_sha256']:
        raise RuntimeError('Full archive integrity failure')
    entries=manifest['entries'];roots=manifest['roots'];files=plan['files']
    if len(files)!=len(set(files)):raise RuntimeError('Repeated file')
    keepers=plan['keep_original_canonical']
    keeper_inputs=plan.get('immutable_keeper_inputs',{})
    def check_keepers():
        for h in {entries[name]['sha256'] for name in files}:
            keeper=keepers.get(h)
            if not keeper or keeper in files:raise RuntimeError('Missing retained original keeper '+h)
            r=entries.get(keeper,keeper_inputs.get(keeper));p=Path(keeper)
            if r is None:raise RuntimeError('Unpinned retained original keeper '+h)
            if r.get('sha256')!=h or p.resolve()!=p or m.facts(p)!={k:v for k,v in r.items() if k!='sha256'} or m.sha(p)!=h:
                raise RuntimeError('Retained keeper changed '+keeper)
    check_keepers()
    closure=Path(plan['input_closure']);closure_receipt=json.loads((closure/'verified.json').read_text())
    if m.sha(closure/'verified.json')!=plan['input_closure_verified_sha256']:raise RuntimeError('Input closure receipt differs from plan')
    if not closure_receipt['verified'] or m.sha(closure/'original-inputs.zip')!=closure_receipt['archive_sha256'] or m.sha(closure/'manifest.private.json')!=closure_receipt['manifest_sha256']:
        raise RuntimeError('Input closure archive integrity failure')
    closure_manifest=json.loads((closure/'manifest.private.json').read_text())
    for keeper in keepers.values():
        if keeper not in entries and keeper_inputs.get(keeper)!=closure_manifest.get('current_entries',{}).get(keeper):
            raise RuntimeError('External keeper is not an exact verified immutable input '+keeper)
    if closure_manifest.get('all_original_qa_input_pins_preserved') is True:
        if not closure_manifest.get('source_unchanged') or not (closure_manifest.get('dependencies') or closure_manifest.get('self_contained_input_bodies')):raise RuntimeError('Incomplete input closure')
        for dependency in closure_manifest['dependencies']:
            for key in ('archive','manifest','receipt'):
                if m.sha(dependency[key])!=dependency[key+'_sha256']:raise RuntimeError('Input dependency changed '+dependency[key])
    elif not closure_manifest.get('source_tar_body_restore_verified') or not closure_manifest.get('public_controllers_build_records_runner_source_and_delegation_pins_match') or not closure_manifest.get('source_unchanged'):
        raise RuntimeError('Incomplete external QA input closure')
    sources={}
    with m.open_payloads(pack/'qa-payloads.zip') as z:
        sources.update({n.removeprefix('sha256/'):(pack/'qa-payloads.zip',n) for n in z.namelist()})
    ext=manifest.get('external_pack')
    if ext:
        path=Path(ext['directory'])/ext.get('archive_name','binary-payloads.zip')
        if m.sha(path)!=ext['archive_sha256']:raise RuntimeError('External payload integrity failure')
        with m.open_payloads(path) as z:sources.update({n.removeprefix('sha256/'):(path,n) for n in z.namelist()})
    for name in files:
        r=entries[name];p=Path(name)
        if not stat.S_ISREG(r['mode']) or r['nlink']!=1 or r['size']<4 or r['sha256'] not in sources:raise RuntimeError('Not eligible ELF payload '+name)
        if not any(Path(root) in p.parents for root in roots):raise RuntimeError('Outside qualified roots')
        if p.resolve()!=p or m.facts(p)!={k:v for k,v in r.items() if k!='sha256'} or m.sha(p)!=r['sha256']:raise RuntimeError('Changed original '+name)
        with p.open('rb') as f:
            if f.read(4)!=b'\x7fELF':raise RuntimeError('Not ELF '+name)
    guards={root:references(Path(root)) for root in roots}
    if m.inventory(roots)!=entries:raise RuntimeError('Full package changed since verification')
    m.make_directory(out)
    m.save_json(out/'preview.json',{'at':time.time(),'files':files,'bytes':sum(entries[p]['size'] for p in files),'guards':guards,'apply':apply,'plan_sha256':m.sha(plan_path)})
    if not apply:return
    deleted=[]
    with (out/'deleted.jsonl').open('x') as log:
        log.flush();os.fsync(log.fileno());m.sync_directory(out)
        for root in roots:
            selected=[p for p in files if Path(root) in Path(p).parents]
            if not selected:continue
            references(Path(root))
            check_keepers()
            for name in selected:
                p=Path(name);r=entries[name]
                if p.resolve()!=p or m.facts(p)!={k:v for k,v in r.items() if k!='sha256'} or m.sha(p)!=r['sha256']:raise RuntimeError('Changed before unlink '+name)
                p.unlink();m.sync_directory(p.parent);deleted.append(name);log.write(json.dumps({'path':name,'original':r,'at':time.time()})+'\n');log.flush();os.fsync(log.fileno())
    check_keepers()
    after=m.inventory(roots);expected=set(entries)-set(deleted)
    if set(after)!=expected:raise RuntimeError('Unexpected addition/removal after retirement')
    changed_parents={str(Path(p).parent) for p in deleted}
    for name,r in after.items():
        old=entries[name]
        ignore={'mtime_ns','ctime_ns','size'} if name in changed_parents else set()
        if {k:v for k,v in r.items() if k not in ignore}!={k:v for k,v in old.items() if k not in ignore}:raise RuntimeError('Non-target entry changed '+name)
    m.save_json(out/'applied.json',{'at':time.time(),'deleted_files':len(deleted),'logical_bytes':sum(entries[p]['size'] for p in deleted),'remaining_entries_unchanged_except_parent_directory_metadata':True,'changed_parent_directories':sorted(changed_parents),'full_qa_archive':str(pack)})
