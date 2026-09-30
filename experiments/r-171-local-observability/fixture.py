"""Finite synthetic input; no Node readiness, product traffic or profiles."""
import datetime
import http.server
import json
import pathlib
import signal
import threading
import time

ROOT = pathlib.Path('/fixture')
DURATION_SECONDS = 600
started = time.monotonic()
lock = threading.Lock()
state = {'queue': 0, 'heartbeat': time.time(), 'events': 0}

class Metrics(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != '/metrics':
            self.send_error(404)
            return
        with lock:
            current = dict(state)
        body = ('# HELP diagnostic_fixture_queue_items Synthetic queue, not Node duty.\n'
                '# TYPE diagnostic_fixture_queue_items gauge\n'
                f'diagnostic_fixture_queue_items {current["queue"]}\n'
                '# HELP diagnostic_fixture_heartbeat_seconds Unix observation timestamp.\n'
                '# TYPE diagnostic_fixture_heartbeat_seconds gauge\n'
                f'diagnostic_fixture_heartbeat_seconds {current["heartbeat"]}\n'
                '# HELP diagnostic_fixture_events_total Generated synthetic events.\n'
                '# TYPE diagnostic_fixture_events_total counter\n'
                f'diagnostic_fixture_events_total {current["events"]}\n').encode()
        self.send_response(200)
        self.send_header('Content-Type', 'text/plain; version=0.0.4')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, format, *args):
        pass

ROOT.mkdir(mode=0o700, exist_ok=True)
server = http.server.HTTPServer(('0.0.0.0', 9101), Metrics)
server.timeout = 0.5
stop = threading.Event()
signal.signal(signal.SIGTERM, lambda *_: stop.set())
signal.signal(signal.SIGINT, lambda *_: stop.set())
def serve():
    while not stop.is_set():
        server.handle_request()
worker = threading.Thread(target=serve)
worker.start()
try:
    with (ROOT / 'events.ndjson').open('x', encoding='utf-8') as output:
        while not stop.is_set() and time.monotonic() - started < DURATION_SECONDS:
            elapsed = time.monotonic() - started
            phase = 'pressure' if 30 <= elapsed < 90 else 'normal'
            with lock:
                state['queue'] = 20 if phase == 'pressure' else 0
                state['heartbeat'] = time.time()
                state['events'] += 1
                sequence = state['events']
            event = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                     'level': 'warning' if phase == 'pressure' else 'info',
                     'scope': 'synthetic_fixture', 'event': phase, 'sequence': sequence}
            output.write(json.dumps(event) + '\n')
            output.flush()
            print(json.dumps(event), flush=True)
            stop.wait(1)
finally:
    stop.set()
    worker.join(timeout=2)
    server.server_close()
    if worker.is_alive():
        raise RuntimeError('Fixture HTTP worker did not join')