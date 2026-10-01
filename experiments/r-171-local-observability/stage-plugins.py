"""Stage exact public signed plugin trees for a disposable Grafana probe."""
import hashlib
import json
import pathlib
import stat
import zipfile

source=pathlib.Path('/assets'); root=pathlib.Path('/output')
receipt={'complete':False,'signature':'Grafana must verify at load; hashes are not signatures','plugins':[]}
try:
    for name,version,digest in [
        ('prometheus','13.2.3','8bdd6583e398f84d497dabec0287563b9eee471d711dea08a21482db5cd5a294'),
        ('loki','13.2.1','ad64433709b43eee8744b4817f1e183bd08266dae12ac943e037b56670d49c52')]:
        archive=source/(name+'-'+version+'.linux_amd64.zip')
        with archive.open('rb') as stream:
            if hashlib.file_digest(stream,'sha256').hexdigest()!=digest:
                raise RuntimeError('Unexpected public archive')
        files={}; total=0
        with zipfile.ZipFile(archive) as package:
            entries=package.infolist()
            if len(entries)>4096 or sum(e.file_size for e in entries)>128*1024*1024:
                raise RuntimeError('Archive inventory budget exceeded')
            for entry in entries:
                path=pathlib.PurePosixPath(entry.filename)
                if path.is_absolute() or '..' in path.parts or '\\' in entry.filename or ':' in entry.filename or stat.S_ISLNK(entry.external_attr>>16) or entry.flag_bits&1:
                    raise RuntimeError('Unsupported archive path/type')
                if not path.parts or path.parts[0]!=name:
                    raise RuntimeError('Unexpected plugin root')
                target=root/'plugins'/path
                if entry.is_dir():
                    target.mkdir(parents=True,exist_ok=True); continue
                if '/'.join(path.parts[1:]) in files: raise RuntimeError('Duplicate archive file')
                data=package.read(entry); total+=len(data)
                target.parent.mkdir(parents=True,exist_ok=True)
                with target.open('xb') as stream: stream.write(data)
                target.chmod(0o755 if path.name.endswith('_linux_amd64') else 0o644)
                files['/'.join(path.parts[1:])]=hashlib.sha256(data).hexdigest()
        metadata=json.loads((root/'plugins'/name/'plugin.json').read_text())
        if metadata['id']!=name or metadata['info']['version']!=version:
            raise RuntimeError('Unexpected plugin identity/version')
        manifest=(root/'plugins'/name/'MANIFEST.txt').read_text()
        signed=json.loads(manifest.split('\n\n',1)[1].split('\n-----BEGIN PGP SIGNATURE-----',1)[0])
        declared=signed['files']
        if signed['plugin']!=name or signed['version']!=version or declared!={k:v for k,v in files.items() if k!='MANIFEST.txt'}:
            raise RuntimeError('Manifest file hashes differ from extracted tree')
        receipt['plugins'].append({'id':name,'version':version,'archive_sha256':digest,'expanded_bytes':total,'files':files})
    receipt['complete']=True
finally:
    (root/'stage-receipt.json').write_text(json.dumps(receipt,indent=2))
print('Pinned complete plugin trees staged; signatures remain for Grafana verification')
