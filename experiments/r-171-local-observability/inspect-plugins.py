"""Inspect pinned public plugin archives; never execute extracted backends."""
import hashlib
import json
import pathlib
import stat
import zipfile

root=pathlib.Path('/assets')
receipt=[]
expected={'prometheus':'8bdd6583e398f84d497dabec0287563b9eee471d711dea08a21482db5cd5a294',
          'loki':'ad64433709b43eee8744b4817f1e183bd08266dae12ac943e037b56670d49c52'}
for name,archive in [('prometheus','prometheus-13.2.3.linux_amd64.zip'),('loki','loki-13.2.1.linux_amd64.zip')]:
    with (root/archive).open('rb') as source:
        if hashlib.file_digest(source,'sha256').hexdigest()!=expected[name]:
            raise RuntimeError('Archive identity changed before inspection')
    inventory=[]; candidates=[]
    with zipfile.ZipFile(root/archive) as package:
        entries=package.infolist()
        if len(entries)>4096 or sum(item.file_size for item in entries)>128*1024*1024:
            raise RuntimeError('Archive inventory/expansion budget exceeded')
        for item in entries:
            path=pathlib.PurePosixPath(item.filename)
            mode=item.external_attr>>16
            if path.is_absolute() or '..' in path.parts or '\\' in item.filename or ':' in item.filename or stat.S_ISLNK(mode) or item.flag_bits&1:
                raise RuntimeError('Unsupported archive entry')
            inventory.append({'name':item.filename,'bytes':item.file_size})
            if path.name.endswith('_linux_amd64') and not item.is_dir(): candidates.append(item)
        if len(candidates)!=1 or candidates[0].file_size>64*1024*1024:
            raise RuntimeError('Backend identity absent/ambiguous/oversized')
        binary=package.read(candidates[0])
        if binary[:4]!=b'\x7fELF': raise RuntimeError('Backend is not ELF')
        (root/(name+'-backend')).write_bytes(binary)
        metadata={}
        for basename,budget in [('plugin.json',65536),('MANIFEST.txt',262144)]:
            selected=[item for item in entries if pathlib.PurePosixPath(item.filename).name==basename]
            if len(selected)>1 or (selected and selected[0].file_size>budget): raise RuntimeError('Metadata ambiguous/oversized')
            if selected:
                data=package.read(selected[0]); (root/(name+'-'+basename)).write_bytes(data)
                metadata[basename]={'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()}
        receipt.append({'role':name,'archive':archive,'backend_entry':candidates[0].filename,
                        'binary_bytes':len(binary),'binary_sha256':hashlib.sha256(binary).hexdigest(),
                        'metadata':metadata,'inventory':inventory})
(root/'plugin-inspection.json').write_text(json.dumps(receipt,indent=2))
print('Public plugin backends inspected; none executed or admitted')
