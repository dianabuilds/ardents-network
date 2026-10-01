"""Finite research exporter of one explicitly selected private safe snapshot."""
import argparse
import datetime
import http.server
import json
import math
import os
import socket
import stat
import threading
import time

from monitor_metrics import MAX_BYTES, project


def selected_directory(path):
    if not os.path.isabs(path) or os.path.normpath(path) != path:
        raise ValueError("canonical absolute snapshot directory required")
    # Inspect every ancestor; do not silently resolve redirected source roots.
    current = path
    while True:
        if stat.S_ISLNK(os.lstat(current).st_mode):
            raise ValueError("redirected snapshot directory")
        parent = os.path.dirname(current)
        if parent == current:
            break
        current = parent
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    info = os.fstat(descriptor)
    if info.st_uid != os.getuid() or info.st_mode & 0o077:
        os.close(descriptor)
        raise ValueError("private owned snapshot directory required")
    return descriptor


def snapshot(descriptor):
    fd = os.open("monitor.json", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK,
                 dir_fd=descriptor)
    try:
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid()
                or info.st_mode & 0o077 or info.st_size > MAX_BYTES):
            raise ValueError("private bounded safe snapshot required")
        with os.fdopen(fd, "rb", closefd=False) as source:
            data = source.read(MAX_BYTES+1)
        if len(data) > MAX_BYTES:
            raise ValueError("snapshot exceeded read budget")
        return json.loads(data)
    finally:
        os.close(fd)


class Exporter(http.server.HTTPServer):
    def __init__(self, address, directory, sample_max_age):
        self.directory = directory
        self.sample_max_age = sample_max_age
        self.active = None
        self.active_lock = threading.Lock()
        super().__init__(address, Handler)
        self.timeout = 0.5

    def get_request(self):
        connection, address = super().get_request()
        connection.settimeout(2)
        with self.active_lock:
            self.active = connection
        return connection, address

    def close_request(self, request):
        with self.active_lock:
            if self.active is request:
                self.active = None
        super().close_request(request)

    def interrupt(self):
        with self.active_lock:
            if self.active is not None:
                try:
                    self.active.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass

    def handle_error(self, request, address):
        # Do not echo arbitrary request bytes or source-file errors.
        pass


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/metrics":
            self.reply(404, b"Not found\n")
            return
        try:
            body = project(snapshot(self.server.directory),
                           datetime.datetime.now(datetime.timezone.utc).timestamp(),
                           self.server.sample_max_age).encode()
            if len(body) > 16384:
                raise ValueError("metric response exceeded budget")
        except (OSError, ValueError, TypeError, KeyError, OverflowError, RecursionError):
            self.reply(503, b"Selected safe monitor snapshot unavailable or invalid\n")
            return
        self.reply(200, body)

    def reply(self, status, body):
        self.send_response(status)
        self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--snapshot-dir", required=True)
    parser.add_argument("--sample-max-age", type=float, required=True)
    parser.add_argument("--duration", type=float, default=300)
    parser.add_argument("--port", type=int, default=9102)
    parser.add_argument("--container", action="store_true",
                        help="Explicit internal-container binding instead of host loopback")
    args = parser.parse_args()
    if (not math.isfinite(args.duration) or not 0 < args.duration <= 600
            or not math.isfinite(args.sample_max_age) or args.sample_max_age <= 0
            or not 0 <= args.port <= 65535):
        raise ValueError("invalid finite exporter budget")
    descriptor = selected_directory(args.snapshot_dir)
    server = None
    timer = None
    stop = threading.Event()

    def expire():
        stop.set()
        server.interrupt()

    try:
        server = Exporter(("0.0.0.0" if args.container else "127.0.0.1", args.port),
                          descriptor, args.sample_max_age)
        timer = threading.Timer(args.duration, expire)
        timer.start()
        print(json.dumps({"port": server.server_port}), flush=True)
        while not stop.is_set():
            server.handle_request()
    finally:
        if timer is not None:
            timer.cancel()
            timer.join()
        if server is not None:
            server.server_close()
        os.close(descriptor)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError):
        print("Selected snapshot exporter unavailable", file=__import__("sys").stderr)
        raise SystemExit(1)