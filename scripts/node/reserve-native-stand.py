#!/usr/bin/env python3
"""Reserve the sole fresh native preparation/installed stand before any copy.

The outside controller holds run-development.py exclusive through the complete
campaign and checked archival disposition. This command grants no execution or
bootstrap permission, never adopts a root and never removes a retained slot.
"""
import json
import os
from pathlib import Path
import re
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
from native_stand import WORK_ROOT, reserve_case


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or os.geteuid() != 0:
        raise RuntimeError('Linux root Python3.12 -I -S required')
    if len(sys.argv) != 3 or not re.fullmatch(r'(preparation|installed)[1-9][0-9]*', sys.argv[2]):
        raise RuntimeError('usage: reserve-native-stand.py EXTERNAL_EVIDENCE preparationN|installedN')
    evidence = Path(sys.argv[1])
    repository = Path(__file__).resolve().parents[2]
    if not evidence.is_absolute() or evidence.is_symlink() or evidence.resolve() != evidence or not evidence.is_dir() or repository == evidence or repository in evidence.parents or WORK_ROOT == evidence or WORK_ROOT in evidence.parents:
        raise RuntimeError('existing real external evidence directory required')
    receipt = evidence / (sys.argv[2] + '-reservation.json')
    if receipt.exists():
        raise RuntimeError('original reservation receipt already exists')
    result = reserve_case(sys.argv[2], evidence)
    print(json.dumps(result), flush=True)
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError) as error:
        print('native stand reservation refused:', error, file=sys.stderr)
        sys.exit(2)
