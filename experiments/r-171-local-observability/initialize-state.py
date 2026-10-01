"""Initialize only empty, explicitly mounted research backend volume roots."""
import os
import stat
import sys

names = ("prometheus", "alertmanager", "loki", "alloy", "grafana")
if sys.argv[1:] == ["--journal"]:
    names += ("journal",)
elif sys.argv[1:]:
    raise RuntimeError("Unknown explicit volume profile")
opened = []
new_roots = []
try:
    for name in names:
        fd = os.open("/state/" + name, os.O_PATH | os.O_DIRECTORY | os.O_NOFOLLOW)
        opened.append(fd)
        info = os.fstat(fd)
        if (info.st_uid, info.st_gid, stat.S_IMODE(info.st_mode)) == (10001, 10001, 0o700):
            continue
        if info.st_uid != 0 or info.st_gid != 0:
            raise RuntimeError("Refuse unexpected state root: " + name)
        # Reopen the anchored descriptor only for a root-owned empty volume.
        # O_PATH can inspect admitted UID10001 mode0700 roots without DAC override.
        writable = os.open("/proc/self/fd/" + str(fd), os.O_RDONLY | os.O_DIRECTORY)
        opened.append(writable)
        if os.listdir(writable):
            raise RuntimeError("Refuse nonempty initial state root: " + name)
        new_roots.append(writable)
    for fd in new_roots:
        os.fchmod(fd, 0o700)
        os.fchown(fd, 10001, 10001)
    print(str(len(names)) + " private volume roots admitted; no recursive changes")
finally:
    for fd in opened:
        os.close(fd)
