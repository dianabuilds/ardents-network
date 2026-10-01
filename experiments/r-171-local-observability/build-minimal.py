"""Bounded assembly of official Collector components; public inputs only."""
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import time

root = pathlib.Path('/build')
pathlib.Path('/cache/work').mkdir(mode=0o700)
artifact = pathlib.Path('/assets/ocb_0.162.0_linux_amd64')
expected = '7c74640d726f23689d8853e0d5a55707ad8b524417ca7416c036c4ecb9ddb01f'
if artifact.stat().st_size != 8417442 or hashlib.sha256(artifact.read_bytes()).hexdigest() != expected:
    raise RuntimeError('Builder artifact identity mismatch')
builder = pathlib.Path('/tmp/ocb')
shutil.copyfile(artifact, builder)
builder.chmod(0o500)
manifest = pathlib.Path('/probe/builder.yml')
shutil.copyfile(manifest, root/'builder.yml')
receipt = {'complete':False, 'started':time.time(), 'builder_sha256':expected,
           'manifest_sha256':hashlib.sha256(manifest.read_bytes()).hexdigest(),
           'scope':'official-component synthetic investigation; no product inputs'}
try:
    with (root/'build.log').open('xb') as log:
        result = subprocess.run([str(builder), '--config', str(root/'builder.yml')],
                                stdout=log, stderr=subprocess.STDOUT, timeout=600)
    receipt['build_exit'] = result.returncode
    if result.returncode:
        raise RuntimeError('Official Builder failed; see preserved build.log')
    binary = root/'generated'/'otelcol-local-probe'
    receipt['binary_bytes'] = binary.stat().st_size
    receipt['binary_sha256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
    files = [p for p in root.rglob('*') if p.is_file()]
    receipt['retained_bytes'] = sum(p.stat().st_size for p in files)
    if receipt['retained_bytes'] > 128*1024*1024:
        raise RuntimeError('Public build outputs exceed128MiB budget')
    with (root/'binary-buildinfo.txt').open('xb') as log:
        subprocess.run(['go','version','-m',str(binary)],stdout=log,stderr=subprocess.STDOUT,check=True,timeout=15)
    receipt['complete'] = True
finally:
    receipt['ended'] = time.time()
    (root/'build-receipt.json').write_text(json.dumps(receipt,indent=2))
print('Official minimal Collector assembled; actual closure still requires inspection')
