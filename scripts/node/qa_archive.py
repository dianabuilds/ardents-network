"""Preserve exact qualified QA roots. Never removes or rewrites a source."""
import hashlib,json,os,socket,stat,sys,tempfile,time,zipfile
from pathlib import Path

def sha(path):
    with Path(path).open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()

def open_payloads(path):
    """Read only the closed, unencrypted payload methods this tool produces."""
    archive=zipfile.ZipFile(path)
    encryption_flags=0x0001|0x0040|0x2000 # traditional/strong encryption or masked headers
    try:
        if any(entry.compress_type not in (zipfile.ZIP_STORED,zipfile.ZIP_DEFLATED) or entry.flag_bits&encryption_flags
               for entry in archive.infolist()):
            raise RuntimeError('Unsupported QA payload compression or encryption')
    except BaseException:
        archive.close()
        raise
    return archive

def facts(p):
    s=p.lstat()
    return dict(device=s.st_dev,inode=s.st_ino,mode=s.st_mode,uid=s.st_uid,gid=s.st_gid,
                size=s.st_size,mtime_ns=s.st_mtime_ns,ctime_ns=s.st_ctime_ns,nlink=s.st_nlink,
                xattrs={k:os.getxattr(p,k,follow_symlinks=False).hex() for k in os.listxattr(p,follow_symlinks=False)})

def inventory(roots):
    entries={}
    for name in roots:
        root=Path(name)
        if root.resolve()!=root or not root.is_dir():raise RuntimeError('Indirect/missing root '+name)
        device=root.stat().st_dev
        stack=[root]
        while stack:
            p=stack.pop();record=facts(p);mode=record['mode']
            if record['device']!=device:raise RuntimeError('Nested filesystem '+str(p))
            if stat.S_ISLNK(mode):record['target']=os.readlink(p)
            elif stat.S_ISDIR(mode):stack.extend(sorted(p.iterdir(),reverse=True))
            elif stat.S_ISREG(mode):
                record['sha256']=sha(p)
                if facts(p)!={k:v for k,v in record.items() if k!='sha256'}:raise RuntimeError('Changing source '+str(p))
            elif stat.S_ISSOCK(mode):record['historical_socket_inode']=True
            elif stat.S_ISFIFO(mode):record['historical_fifo_inode']=True
            else:raise RuntimeError('Unsupported device entry '+str(p))
            entries[str(p)]=record
    return entries

def sync_directory(path):
    fd=os.open(path,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)

def make_directory(path):
    path.mkdir(mode=0o700)
    sync_directory(path)
    sync_directory(path.parent)

def save_json(path,data):
    with path.open('x',encoding='utf8') as f:json.dump(data,f,indent=2);f.flush();os.fsync(f.fileno())
    sync_directory(path.parent)

def preserve(qualification,destination,external=None):
    qpath=Path(qualification);q=json.loads(qpath.read_text(encoding='utf8'))
    if q.get('qualified_terminal_qa') is not True or not q.get('roots') or not q.get('receipts'):
        raise RuntimeError('Requires owner-qualified terminal roots and receipts')
    roots=q['roots']
    if len(roots)!=len(set(roots)) or any(str(Path(r))!=r for r in roots):raise RuntimeError('Noncanonical/repeated root')
    if any(Path(a) in Path(b).parents for a in roots for b in roots if a!=b):raise RuntimeError('Overlapping roots')
    out=Path(destination);make_directory(out)
    entries=inventory(roots);groups={}
    # A case archive must not silently split native links across package roots.
    link_roots={}
    for name,record in entries.items():
        if stat.S_ISREG(record['mode']):
            owner=next(root for root in roots if name==root or Path(root) in Path(name).parents)
            link_roots.setdefault((record['device'],record['inode']),set()).add(owner)
    if any(len(owners)>1 for owners in link_roots.values()):
        raise RuntimeError('Cross-package hardlink topology requires explicit preservation')
    for p,r in entries.items():
        if 'sha256' in r:groups.setdefault(r['sha256'],[]).append(p)
    external_index={};external_receipt=None;external_zip=None
    if external:
        ext=Path(external);receipt=json.loads((ext/'verified.json').read_text());manifest=json.loads((ext/'manifest.private.json').read_text())
        external_name=receipt.get('archive_name','binary-payloads.zip')
        if external_name not in ('binary-payloads.zip','original-inputs.zip'):
            raise RuntimeError('Unknown immutable input pack format')
        if not receipt['verified'] or sha(ext/external_name)!=receipt['archive_sha256'] or sha(ext/'manifest.private.json')!=receipt['manifest_sha256']:
            raise RuntimeError('External pack integrity failure')
        external_index={r['sha256']:r['size'] for r in manifest['paths'].values()}
        external_receipt={'directory':str(ext),'archive_name':external_name,'archive_sha256':receipt['archive_sha256'],'manifest_sha256':receipt['manifest_sha256']}
        external_zip=open_payloads(ext/external_name)
    archive_path=out/'qa-payloads.zip';stored=set()
    try:
        with zipfile.ZipFile(archive_path,'x',compression=zipfile.ZIP_DEFLATED,compresslevel=1,allowZip64=True) as z:
            for number,(h,paths) in enumerate(sorted(groups.items()),1):
                if h in external_index:
                    if external_index[h]!=entries[paths[0]]['size']:raise RuntimeError('External length mismatch')
                else:
                    with Path(paths[0]).open('rb') as source,z.open('sha256/'+h,'w',force_zip64=True) as target:
                        while chunk:=source.read(1024*1024):target.write(chunk)
                    stored.add(h)
                if number%250==0:print(json.dumps({'preserved_payloads':number,'total':len(groups)}),flush=True)
        with archive_path.open('r+b') as durable:
            durable.flush();os.fsync(durable.fileno())
        sync_directory(out)
        # Restore each complete package, preserving links, modes, owners, timestamps and xattrs.
        with open_payloads(archive_path) as own,tempfile.TemporaryDirectory(prefix='qa-restore-',dir='/var/tmp') as tmp:
            if set(own.namelist())!={'sha256/'+h for h in stored}:raise RuntimeError('Unexpected archive membership')
            for number,root in enumerate(roots):
                print(json.dumps({'restoring_package':number+1,'packages':len(roots)}),flush=True)
                restore=Path(tmp)/str(number);restore.mkdir();selected={p:r for p,r in entries.items() if p==root or Path(root) in Path(p).parents}
                inodes={}
                for p,r in sorted(selected.items(),key=lambda item:(len(Path(item[0]).parts),item[0])):
                    target=restore/Path(p).relative_to(root);mode=r['mode']
                    if stat.S_ISDIR(mode):target.mkdir(exist_ok=True)
                    elif stat.S_ISLNK(mode):target.symlink_to(r['target'])
                    elif stat.S_ISFIFO(mode):os.mkfifo(target)
                    elif stat.S_ISSOCK(mode):
                        # Use an owned directory FD to avoid AF_UNIX path limits.
                        fd=os.open(target.parent,os.O_RDONLY|os.O_DIRECTORY)
                        try:
                            with socket.socket(socket.AF_UNIX) as s:s.bind('/proc/self/fd/'+str(fd)+'/'+target.name)
                        finally:os.close(fd)
                    else:
                        key=(r['device'],r['inode'])
                        if key in inodes:os.link(inodes[key],target)
                        else:
                            source_zip=own if r['sha256'] in stored else external_zip
                            with source_zip.open('sha256/'+r['sha256']) as source,target.open('xb') as dest:
                                while chunk:=source.read(1024*1024):dest.write(chunk)
                                dest.flush();os.fsync(dest.fileno())
                            inodes[key]=target
                        if sha(target)!=r['sha256'] or target.stat().st_size!=r['size']:raise RuntimeError('Restored body mismatch '+p)
                for p,r in sorted(selected.items(),key=lambda item:len(Path(item[0]).parts),reverse=True):
                    target=restore/Path(p).relative_to(root);os.chown(target,r['uid'],r['gid'],follow_symlinks=False)
                    if not stat.S_ISLNK(r['mode']):os.chmod(target,stat.S_IMODE(r['mode']))
                    for k,v in r['xattrs'].items():os.setxattr(target,k,bytes.fromhex(v),follow_symlinks=False)
                    os.utime(target,ns=(r['mtime_ns'],r['mtime_ns']),follow_symlinks=False)
                    actual=facts(target)
                    for k in ('mode','uid','gid','mtime_ns','xattrs'):
                        if actual[k]!=r[k]:raise RuntimeError('Restore metadata mismatch '+p+' '+k)
                    if 'target' in r and os.readlink(target)!=r['target']:raise RuntimeError('Restore symlink mismatch')
                # Only temporary copies are removed; TemporaryDirectory owns this root.
                import shutil
                shutil.rmtree(restore)
        if inventory(roots)!=entries:raise RuntimeError('Source changed during preservation')
        manifest={'created':time.time(),'roots':roots,'entries':entries,'qualification_sha256':sha(qpath),'qualification':q,'external_pack':external_receipt,
                  'restore_scope':'bytes, uid/gid/mode/mtime_ns/xattrs/symlinks, intra-root hardlinks; inode numbers are historical provenance, never adopted authority'}
        save_json(out/'manifest.private.json',manifest)
        receipt={'verified':True,'complete_qa_packages':len(roots),'entries':len(entries),'unique_payloads':len(groups),'local_payloads':len(stored),
                 'archive_bytes':archive_path.stat().st_size,'archive_sha256':sha(archive_path),'manifest_sha256':sha(out/'manifest.private.json'),
                 'source_unchanged':True,'physical_full_package_restore':True,'not_delete_allowlist':True}
        save_json(out/'verified.json',receipt);return receipt
    finally:
        if external_zip:external_zip.close()

def preserve_inputs(input_files, destination):
    """Capture one campaign's exact immutable inputs once, before its cases.

    This private body pack is also the shared CAS for completed case archives.
    It conveys no bootstrap, runtime, native-inode or recovery authority.
    """
    if not input_files:
        raise RuntimeError('Complete explicit input pins required')
    out=Path(destination)
    make_directory(out)
    entries={}
    groups={}
    for name,wanted in sorted(input_files.items()):
        p=Path(name)
        if not p.is_absolute() or p.resolve()!=p:
            raise RuntimeError('Direct original input required '+name)
        original=facts(p)
        if not stat.S_ISREG(original['mode']) or original['nlink']!=1:
            raise RuntimeError('Only independent regular immutable inputs are supported '+name)
        if sha(p)!=wanted or facts(p)!=original:
            raise RuntimeError('Changed original input '+name)
        entries[name]={**original,'sha256':wanted}
        groups.setdefault(wanted,[]).append(name)
    archive=out/'original-inputs.zip'
    with zipfile.ZipFile(archive,'x',compression=zipfile.ZIP_DEFLATED,compresslevel=1,allowZip64=True) as z:
        for h,names in sorted(groups.items()):
            with Path(names[0]).open('rb') as source,z.open('sha256/'+h,'w',force_zip64=True) as target:
                while chunk:=source.read(1024*1024):target.write(chunk)
    with archive.open('r+b') as durable:
        durable.flush();os.fsync(durable.fileno())
    sync_directory(out)
    with open_payloads(archive) as z,tempfile.TemporaryDirectory(prefix='qa-input-restore-',dir='/var/tmp') as tmp:
        if set(z.namelist())!={'sha256/'+h for h in groups}:
            raise RuntimeError('Unexpected immutable pack membership')
        for number,(name,r) in enumerate(entries.items(),1):
            restored=Path(tmp)/str(number)
            with z.open('sha256/'+r['sha256']) as source,restored.open('xb') as target:
                while chunk:=source.read(1024*1024):target.write(chunk)
                target.flush();os.fsync(target.fileno())
            os.chown(restored,r['uid'],r['gid'])
            os.chmod(restored,stat.S_IMODE(r['mode']))
            for key,value in r['xattrs'].items():os.setxattr(restored,key,bytes.fromhex(value))
            os.utime(restored,ns=(r['mtime_ns'],r['mtime_ns']))
            actual=facts(restored)
            if sha(restored)!=r['sha256'] or any(actual[key]!=r[key] for key in ('size','mode','uid','gid','mtime_ns','xattrs')):
                raise RuntimeError('Input physical restore differs '+name)
            restored.unlink()
            if number%250==0:
                print(json.dumps({'restored_inputs':number,'inputs':len(entries)}),flush=True)
    for name,r in entries.items():
        if facts(Path(name))!={k:v for k,v in r.items() if k!='sha256'} or sha(name)!=r['sha256']:
            raise RuntimeError('Original input changed during preservation '+name)
    manifest={'schema':'ardents-qa-inputs-v1','all_original_qa_input_pins_preserved':True,
              'source_unchanged':True,'self_contained_input_bodies':True,'current_entries':entries,
              'paths':entries,'dependencies':[],
              'restore_scope':'bytes, uid/gid/mode/mtime_ns/xattrs; original inode/device/ctime are historical metadata only'}
    save_json(out/'manifest.private.json',manifest)
    result={'verified':True,'archive_name':'original-inputs.zip','input_paths':len(entries),
            'physical_input_restore':True,'archive_sha256':sha(archive),
            'manifest_sha256':sha(out/'manifest.private.json'),'source_unchanged':True}
    save_json(out/'verified.json',result)
    return result


if __name__=='__main__':print(json.dumps(preserve(*sys.argv[1:]),indent=2),flush=True)
