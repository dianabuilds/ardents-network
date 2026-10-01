"""Exhaust/free only one new file on the selected synthetic collector state volume."""
import errno
import json
import os
import pathlib
import stat
import sys
root = pathlib.Path('/state')
path = root/'r171-collector-pressure.bin'
reports = pathlib.Path('/reports')
activation = pathlib.Path('/fixture/collector-pressure-active')
fs = os.statvfs(root)
owner = root.lstat()
if not stat.S_ISDIR(owner.st_mode) or owner.st_uid != os.getuid() or owner.st_mode & 0o077:
    raise RuntimeError('Selected collector state root must be owned and private')
if fs.f_blocks*fs.f_frsize != 67108864:
    raise RuntimeError('Expected exact isolated 64 MiB collector filesystem')
if sys.argv[1] == 'fill':
    with activation.open('xb') as gate:
        os.chmod(activation, 0o600)
        gate.write(b'1')
    gate_stat = activation.lstat()
    written = 0
    exhausted = False
    with path.open('xb', buffering=0) as target:
        os.chmod(path, 0o600)
        while written < 67108864+1048576:
            try:
                written += target.write(bytes(1048576))
            except OSError as exc:
                if exc.errno != errno.ENOSPC: raise
                exhausted = True
                break
    observed = path.lstat()
    fs = os.statvfs(root)
    record = {'device': observed.st_dev, 'inode': observed.st_ino, 'size': observed.st_size,
              'enospc': exhausted, 'free_bytes': fs.f_bfree*fs.f_frsize,
              'capacity_bytes': fs.f_blocks*fs.f_frsize,
              'activation_device': gate_stat.st_dev, 'activation_inode': gate_stat.st_ino}
    (reports/'fill-receipt.json').write_text(json.dumps(record))
    if not exhausted or record['free_bytes'] >= 4096:
        raise RuntimeError('Actual collector filesystem exhaustion not observed')
    print('Selected synthetic collector filesystem reached ENOSPC')
elif sys.argv[1] == 'free':
    before = json.loads((reports/'fill-receipt.json').read_text())
    observed = path.lstat()
    if (not stat.S_ISREG(observed.st_mode) or observed.st_nlink != 1 or
        observed.st_uid != os.getuid() or observed.st_dev != before['device'] or
        observed.st_ino != before['inode'] or observed.st_size != before['size']):
        raise RuntimeError('Refuse removal of changed or non-owned injection file')
    gate_stat = activation.lstat()
    if (not stat.S_ISREG(gate_stat.st_mode) or gate_stat.st_nlink != 1 or
        gate_stat.st_uid != os.getuid() or gate_stat.st_size != 1 or
        gate_stat.st_dev != before['activation_device'] or gate_stat.st_ino != before['activation_inode']):
        raise RuntimeError('Refuse removal of changed activation file')
    activation.unlink()
    path.unlink()
    fs = os.statvfs(root)
    free = fs.f_bfree*fs.f_frsize
    (reports/'free-receipt.json').write_text(json.dumps({'free_bytes': free}))
    if free < before['size']: raise RuntimeError('Collector filesystem recovery not observed')
    print('Removed only the verified collector injection file')
else:
    raise RuntimeError('Unknown explicit collector pressure action')