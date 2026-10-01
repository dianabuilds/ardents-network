"""Finite offline WSL filesystem experiment; root bootstrap, no Node data."""
import argparse
import errno
import json
import re
import os
import pathlib
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--name', default='a')
args = parser.parse_args()
if not re.fullmatch(r'[a-z0-9]{1,16}', args.name):
    parser.error('Select one short experiment name')
root = pathlib.Path('/tmp') / ('ardents-r171-disk-' + args.name)
if os.geteuid() != 0:
    raise RuntimeError('Explicit offline WSL root bootstrap required')
if root.exists() or root.is_symlink():
    raise RuntimeError('Refuse existing experiment root')
os.umask(0o077)
root.mkdir(mode=0o700)
image = root / 'state.img'
mount = root / 'state'
mount.mkdir(mode=0o700)
receipt = {'complete': False, 'cleanup_complete': False, 'image_bytes': 67108864,
           'scope': 'one new offline ext4 image; WSL root bootstrap only; no Docker/backend/host reboot claim'}
loop = None
mounted = False

def command(*args):
    return subprocess.run(args, check=True, capture_output=True, text=True, timeout=20).stdout.strip()

def selected_loop():
    if loop is None or command('losetup', '--list', '--noheadings', '--output', 'BACK-FILE', loop) != str(image):
        raise RuntimeError('Selected loop association changed')

def unmount():
    global mounted
    selected_loop()
    if command('findmnt', '--noheadings', '--output', 'SOURCE', '--mountpoint', str(mount)) != loop:
        raise RuntimeError('Selected mount identity changed')
    command('umount', str(mount))
    mounted = False

try:
    with image.open('xb') as output:
        output.truncate(receipt['image_bytes'])
        output.flush()
        os.fsync(output.fileno())
    # Format the new regular file, never a block-device path.
    command('mkfs.ext4', '-q', '-F', str(image))
    loop = command('losetup', '--find', '--show', '--nooverlap', str(image))
    selected_loop()
    command('mount', '-t', 'ext4', '-o', 'nodev,nosuid,noexec', loop, str(mount))
    mounted = True
    info = os.statvfs(mount)
    receipt['filesystem_capacity_bytes'] = info.f_blocks * info.f_frsize
    if not 0 < receipt['filesystem_capacity_bytes'] <= receipt['image_bytes']:
        raise RuntimeError('Filesystem capacity exceeds selected image')
    marker = mount / 'owned-marker'
    with marker.open('xb') as output:
        output.write(b'bounded-persistent-marker')
        output.flush()
        os.fsync(output.fileno())
    directory = os.open(mount, os.O_DIRECTORY | os.O_RDONLY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)
    filler = mount / 'owned-pressure'
    enospc = False
    chunk = b'0' * (128 << 10)
    with filler.open('xb', buffering=0) as output:
        for _ in range(640):  # At most80MiB attempted, all on the new64MiB image.
            try:
                output.write(chunk)
            except OSError as error:
                if error.errno != errno.ENOSPC:
                    raise
                enospc = True
                break
    receipt['native_enospc_observed'] = enospc
    receipt['pressure_file_bytes'] = filler.stat().st_size
    if not enospc:
        raise RuntimeError('Selected filesystem write ceiling not observed')
    filler.unlink()  # Only this new root-private synthetic file; no host pressure.
    receipt['available_bytes_after_recovery'] = os.statvfs(mount).f_bavail * os.statvfs(mount).f_frsize
    if receipt['available_bytes_after_recovery'] <= 0:
        raise RuntimeError('Selected filesystem space did not recover')
    unmount()
    selected_loop()
    command('mount', '-t', 'ext4', '-o', 'ro,nodev,nosuid,noexec', loop, str(mount))
    mounted = True
    receipt['after_unmount_remount_marker_matches'] = marker.read_bytes() == b'bounded-persistent-marker'
    if not receipt['after_unmount_remount_marker_matches']:
        raise RuntimeError('Persistent marker not recovered')
    receipt['complete'] = True
except Exception as error:
    receipt['operation_failure'] = type(error).__name__
    raise
finally:
    cleanup_errors = []
    try:
        # Setup may take effect before its command fails/returns. Discover the
        # exact image associations and mount rather than trust assignment flags.
        associations = command('losetup', '--associated', str(image), '--noheadings', '--output', 'NAME').splitlines()
        if len(associations) > 1:
            raise RuntimeError('Multiple selected-image associations; refuse ambiguous cleanup')
        observed = subprocess.run(('findmnt', '--noheadings', '--output', 'SOURCE', '--mountpoint', str(mount)),
                                  capture_output=True, text=True, timeout=20)
        if observed.returncode not in (0, 1) or (observed.returncode == 1 and observed.stdout.strip()):
            raise RuntimeError('Selected mount reconciliation unavailable')
        mounted_source = observed.stdout.strip() if observed.returncode == 0 else None
        loop = associations[0] if associations else None
        if mounted_source is not None:
            if loop is None or mounted_source != loop:
                raise RuntimeError('Selected mount association is ambiguous')
            mounted = True
            unmount()
        else:
            mounted = False
        if loop is not None:
            selected_loop()
            command('losetup', '--detach', loop)
        remaining = command('losetup', '--associated', str(image), '--noheadings', '--output', 'NAME')
        observed = subprocess.run(('findmnt', '--noheadings', '--output', 'SOURCE', '--mountpoint', str(mount)),
                                  capture_output=True, text=True, timeout=20)
        if remaining or observed.returncode != 1 or observed.stdout.strip():
            raise RuntimeError('Selected filesystem cleanup not confirmed')
    except Exception as error:
        cleanup_errors.append(type(error).__name__)
    receipt['cleanup_complete'] = not cleanup_errors
    receipt['cleanup_errors'] = cleanup_errors
    if cleanup_errors:
        receipt['complete'] = False
    receipt['retained_image_allocated_bytes'] = image.stat().st_blocks * 512 if image.exists() else None
    (root / 'receipt.json').write_text(json.dumps(receipt, indent=2))
    print(json.dumps(receipt))
    if cleanup_errors:
        raise RuntimeError('Selected filesystem cleanup incomplete')
