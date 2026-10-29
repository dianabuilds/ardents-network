#!/usr/bin/env python3
"""Preserve a campaign's exact private immutable inputs once for all its cases.

The pinned map comes from the independent qualification preparation. Capturing
it does not accept a bootstrap receipt or grant installed execution permission.
"""
import json
import os
from pathlib import Path
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
import qa_archive
from native_stand import WORK_ROOT


def main():
    if sys.version_info[:2] != (3, 12) or not sys.flags.isolated or not sys.flags.no_site or os.geteuid() != 0:
        raise RuntimeError('Linux root Python3.12 -I -S required')
    if len(sys.argv) != 4:
        raise RuntimeError('usage: prepare-native-inputs.py INPUT_MAP_JSON INPUT_MAP_SHA256 EXTERNAL_DESTINATION')
    source, destination = Path(sys.argv[1]), Path(sys.argv[3])
    repository = Path(__file__).resolve().parents[2]
    if not source.is_absolute() or source.resolve() != source or qa_archive.sha(source) != sys.argv[2]:
        raise RuntimeError('original independently pinned input map required')
    if not destination.is_absolute() or destination.resolve() != destination or repository == destination or repository in destination.parents or WORK_ROOT == destination or WORK_ROOT in destination.parents:
        raise RuntimeError('fresh external private input archive directory required')
    files = json.loads(source.read_bytes())
    if not isinstance(files, dict) or not files:
        raise RuntimeError('complete original immutable input map required')
    print(json.dumps(qa_archive.preserve_inputs(files, destination)), flush=True)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError) as error:
        print('immutable QA input preservation refused:', error, file=sys.stderr)
        sys.exit(2)
