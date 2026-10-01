"""Inspect one pinned public tar member without executing it or extracting paths."""
import hashlib
import json
import pathlib
import tarfile
root = pathlib.Path('/assets')
archive = root/'otelcol-contrib_0.162.0_linux_amd64.tar.gz'
found = False
with tarfile.open(archive, 'r|gz') as stream:
    for member in stream:
        if member.name not in ('otelcol-contrib','./otelcol-contrib'):
            continue
        if found or not member.isfile() or not 0 < member.size <= 536870912:
            raise RuntimeError('Unexpected selected archive member')
        found = True
        source = stream.extractfile(member)
        digest = hashlib.sha256()
        count = 0
        with (root/'otelcol-contrib').open('xb') as target:
            while True:
                data = source.read(65536)
                if not data: break
                count += len(data)
                if count > member.size: raise RuntimeError('Archive member budget exceeded')
                target.write(data)
                digest.update(data)
        if count != member.size: raise RuntimeError('Incomplete archive member')
        (root/'binary-receipt.json').write_text(json.dumps({'archive_member':member.name,
            'bytes':count,'sha256':digest.hexdigest(),'scope':'public binary inspected, never executed'}))
if not found: raise RuntimeError('Expected public Collector binary absent')
print('Selected public binary extracted without executing target')