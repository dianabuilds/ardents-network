"""Fill/free exactly one new synthetic Loki volume file, never product storage."""
import errno
import json
import os
import pathlib
import stat
import sys
root = pathlib.Path('/state')
path = root/'r171-pressure.bin'
report = pathlib.Path('/reports/fill-receipt.json')
if sys.argv[1] == 'fill':
    written = 0
    exhausted = False
    try:
        with path.open('xb', buffering=0) as target:
            while written < 536870912+1048576:
                try:
                    written += target.write(bytes(1048576))
                except OSError as exc:
                    if exc.errno != errno.ENOSPC: raise
                    exhausted = True
                    break
    finally:
        observed = path.lstat()
        fs = os.statvfs(root)
        record = {'device':observed.st_dev,'inode':observed.st_ino,'size':observed.st_size,
                  'enospc':exhausted,'free_bytes':fs.f_bfree*fs.f_frsize,
                  'capacity_bytes':fs.f_blocks*fs.f_frsize}
        report.write_text(json.dumps(record))
    if not exhausted or record['free_bytes'] >= 4096:
        raise RuntimeError('Actual bounded filesystem exhaustion not observed')
    print('Synthetic Loki filesystem exhaustion observed')
elif sys.argv[1] == 'free':
    before = json.loads(report.read_text())
    observed = path.lstat()
    if (not stat.S_ISREG(observed.st_mode) or observed.st_nlink != 1 or
        observed.st_uid != os.getuid() or observed.st_dev != before['device'] or
        observed.st_ino != before['inode'] or observed.st_size != before['size']):
        raise RuntimeError('Refuse removal of a changed or non-owned injection file')
    path.unlink()
    fs = os.statvfs(root)
    free = fs.f_bfree*fs.f_frsize
    pathlib.Path('/reports/free-receipt.json').write_text(json.dumps({'free_bytes':free}))
    if free < before['size']: raise RuntimeError('Filesystem space recovery not observed')
    print('Removed only the validated synthetic injection file')
else:
    raise RuntimeError('Unknown explicit pressure action')