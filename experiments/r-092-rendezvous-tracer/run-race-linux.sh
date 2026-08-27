#!/bin/sh
set -eu

lab_root="$(mktemp -d /tmp/ardents-r092-race.XXXXXX)"
binary="$lab_root/r092-rendezvous"
server_out="$lab_root/server.out"
server_err="$lab_root/server.err"
initiator_out="$lab_root/initiator.out"
initiator_err="$lab_root/initiator.err"
responder_out="$lab_root/responder.out"
responder_err="$lab_root/responder.err"
server_pid=""
initiator_pid=""
responder_pid=""

stop_children() {
    for pid in "$initiator_pid" "$responder_pid" "$server_pid"; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
    done
}
trap stop_children EXIT INT TERM

mkdir -p /tmp/go-build /tmp/go-tmp
GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp CGO_ENABLED=1 go build -race -trimpath -o "$binary" \
    experiments/r-092-rendezvous-tracer/main.go \
    experiments/r-092-rendezvous-tracer/identity.go \
    experiments/r-092-rendezvous-tracer/binding.go \
    experiments/r-092-rendezvous-tracer/transcript.go \
    experiments/r-092-rendezvous-tracer/client.go \
    experiments/r-092-rendezvous-tracer/server.go

deadline="$(( $(date +%s) + 20 ))"
"$binary" server -listen 127.0.0.1:47927 -deadline-unix "$deadline" \
    -handshakes 2 -waiting 2 -pairs 1 -expect-pairs 1 >"$server_out" 2>"$server_err" &
server_pid=$!

ready=0
attempt=0
while [ "$attempt" -lt 100 ]; do
    if grep -q '"event":"ready"' "$server_out" 2>/dev/null; then
        ready=1
        break
    fi
    if ! kill -0 "$server_pid" 2>/dev/null; then
        break
    fi
    attempt=$((attempt + 1))
    sleep 0.05
done
if [ "$ready" -ne 1 ]; then
    echo "race server did not become ready" >&2
    cat "$server_err" >&2
    exit 1
fi

"$binary" client -endpoint 127.0.0.1:47927 -side initiator -token race-exact-pair \
    -deadline-unix "$deadline" >"$initiator_out" 2>"$initiator_err" &
initiator_pid=$!
"$binary" client -endpoint 127.0.0.1:47927 -side responder -token race-exact-pair \
    -deadline-unix "$deadline" >"$responder_out" 2>"$responder_err" &
responder_pid=$!

wait "$initiator_pid"
initiator_pid=""
wait "$responder_pid"
responder_pid=""
wait "$server_pid"
server_pid=""

for path in "$server_err" "$initiator_err" "$responder_err"; do
    if [ -s "$path" ]; then
        echo "race stderr was not empty: $path" >&2
        cat "$path" >&2
        exit 1
    fi
done
grep -q '"successful_pairs":1' "$server_out"
grep -q '"cleanup_joined":true' "$server_out"
grep -q '"final_connections":0' "$server_out"
grep -q '"payload_bytes":262144' "$initiator_out"
grep -q '"payload_bytes":262144' "$responder_out"
grep -q '743cdf7849dc6ffdc775371adb60313afb6ecb74e2a8ef22f63c8d419e0b36ec' "$initiator_out"
grep -q '743cdf7849dc6ffdc775371adb60313afb6ecb74e2a8ef22f63c8d419e0b36ec' "$responder_out"

echo '{"case":"linux-race-exact-pair","passed":true,"pairs":1,"final_connections":0}'
