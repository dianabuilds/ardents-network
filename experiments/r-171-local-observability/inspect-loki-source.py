"""Inspect exact public Loki source package graph; no plugin execution."""
import hashlib
import json
import os
import pathlib
import shutil
import subprocess

root=pathlib.Path('/evidence')
module='github.com/grafana/grafana-loki-datasource@v0.0.0-20260928100302-9f30ae5c1569'
pathlib.Path('/cache/work').mkdir(mode=0o700)
receipt={'complete':False,'module':module,'scope':'source package graph, not reproducible artifact proof'}
try:
    result=subprocess.run(['go','mod','download','-json',module],capture_output=True,timeout=180)
    (root/'download.json').write_bytes(result.stdout); (root/'download.stderr').write_bytes(result.stderr)
    receipt['download_exit']=result.returncode
    if result.returncode: raise RuntimeError('Exact public module download failed')
    metadata=json.loads(result.stdout); source=pathlib.Path(metadata['Dir'])
    for name in ('go.mod','go.sum'):
        actual=(source/name).read_bytes(); expected=(pathlib.Path('/primary')/name).read_bytes()
        if actual!=expected: raise RuntimeError('Module source differs from exact upstream commit: '+name)
        shutil.copyfile(source/name,root/name)
    receipt['module_sum']=metadata['Sum']; receipt['go_mod_sum']=metadata['GoModSum']
    with (root/'packages.json').open('xb') as out, (root/'packages.stderr').open('xb') as err:
        result=subprocess.run(['go','list','-mod=readonly','-tags=arrow_json_stdlib','-deps','-json','./pkg'],cwd=source,stdout=out,stderr=err,timeout=180)
    receipt['list_exit']=result.returncode
    if result.returncode: raise RuntimeError('Exact source package graph failed')
    text=(root/'packages.json').read_text()
    if len(text)>32*1024*1024: raise RuntimeError('Package graph evidence budget exceeded')
    decoder=json.JSONDecoder(); offset=0; packages=[]
    while offset<len(text):
        while offset<len(text) and text[offset].isspace(): offset+=1
        if offset==len(text): break
        package,offset=decoder.raw_decode(text,offset); packages.append(package)
    receipt['package_count']=len(packages)
    receipt['openpgp_packages']=[p['ImportPath'] for p in packages if p['ImportPath'].startswith('golang.org/x/crypto/openpgp')]
    receipt['crypto_packages']=[p['ImportPath'] for p in packages if p['ImportPath'].startswith('golang.org/x/crypto/')]
    receipt['modules']={p['Module']['Path']:p['Module'].get('Version','main') for p in packages if p.get('Module')}
    receipt['build_tags']=['arrow_json_stdlib']
    receipt['goos']=os.environ.get('GOOS','linux')
    receipt['goarch']=os.environ.get('GOARCH','amd64')
    receipt['toolchain']=subprocess.check_output(['go','version'],text=True).strip()
    buildinfo=pathlib.Path('/artifact/loki-buildinfo.txt').read_text()
    binary=pathlib.Path('/artifact/loki-backend')
    if binary.stat().st_size!=40472738: raise RuntimeError('Unexpected plugin length')
    if hashlib.sha256(binary.read_bytes()).hexdigest()!='283f0a20397b1767f6348ac3f3bb845e0d04dca3c6ac2f1c9db9484b6b654412':
        raise RuntimeError('Unexpected plugin binary')
    fresh_buildinfo=subprocess.check_output(['go','version','-m',str(binary)],text=True,timeout=20)
    if fresh_buildinfo.splitlines()[1:]!=buildinfo.splitlines()[1:]:
        raise RuntimeError('Retained metadata differs from actual binary')
    (root/'artifact-buildinfo.txt').write_text(fresh_buildinfo)
    artifact_modules={}; previous=None
    for line in buildinfo.splitlines():
        fields=line.strip().split('\t')
        if fields[0]=='dep':
            previous=fields[1]; artifact_modules[previous]=fields[1:4]
        elif fields[0]=='=>':
            artifact_modules[previous]=fields[1:4]
    source_modules={}
    for package in packages:
        entry=package.get('Module')
        if entry and entry.get('Version'):
            effective=entry.get('Replace',entry)
            source_modules[entry['Path']]=[effective['Path'],effective.get('Version'),effective.get('Sum')]
    differences=[]
    for name in sorted(set(artifact_modules)|set(source_modules)):
        if artifact_modules.get(name)!=source_modules.get(name):
            differences.append({'module':name,'artifact':artifact_modules.get(name),'source':source_modules.get(name)})
    receipt['module_differences']=differences
    receipt['dynamic_plugin_package_present']=any(p['ImportPath']=='plugin' for p in packages)
    receipt['artifact_buildinfo_sha256']=hashlib.sha256(buildinfo.encode()).hexdigest()
    if differences: raise RuntimeError('Source graph does not match binary dependency closure')
    receipt['complete']=True
finally:
    (root/'source-graph-receipt.json').write_text(json.dumps(receipt,indent=2))
print('Exact source package graph inspected; binary non-applicability still requires review')
