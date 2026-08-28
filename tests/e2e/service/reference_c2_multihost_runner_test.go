//go:build h4_3b_multihost || h4_8_a11

package service_test

import (
	"os/exec"
	"strings"
	"testing"
)

func h43RemoteRunner() string {
	return `#!/bin/sh
set -eu
work=/work
pids=""
transit_pids=""
transit_processes=""
stream_paths=""
started_pid=""
role_exit_statuses="$work/remote-role-exit-statuses.jsonl"
: >"$role_exit_statuses"
terminal=$(cat "$work/expected-terminal")
product_transit=false
fault=""
warmup_cycles=0
if [ -s "$work/topology.json" ]; then
  product_transit=true
  fault=$(cat "$work/expected-fault")
  warmup_cycles=$(cat "$work/expected-warmup-cycles")
fi
cleanup() {
  status=$?
  trap - EXIT INT TERM
  for pid in $pids; do kill "$pid" 2>/dev/null || true; done
  for pid in $pids; do wait "$pid" 2>/dev/null || true; done
  for path in $stream_paths; do rm -f "$path"; done
  exit "$status"
}
trap cleanup EXIT INT TERM
start() {
  name=$1
  shift
  "$@" >"$work/$name.log" 2>"$work/$name.err" &
  pid=$!
  pids="$pids $pid"
  started_pid=$pid
}
start_stream() {
  name=$1
  shift
  stream="$work/$name.stdout"
  mkfifo "$stream"
  cat "$stream" >"$work/$name.log" &
  reader=$!
  pids="$pids $reader"
  stream_paths="$stream_paths $stream"
  "$@" >"$stream" 2>"$work/$name.err" &
  pid=$!
  pids="$pids $pid"
  started_pid=$pid
}
wait_file() {
  path=$1
  tries=0
  while [ "$tries" -lt 400 ]; do
    if [ -s "$path" ]; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
wait_fault_file() {
  path=$1
  tries=0
  while [ "$tries" -lt 20000 ]; do
    if [ -s "$path" ]; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
wait_completion_file() {
  path=$1
  tries=0
  while [ "$tries" -lt 20000 ]; do
    if [ -s "$path" ]; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
wait_source() {
  log=$1
  tries=0
  while [ "$tries" -lt 200 ]; do
    if grep -F '"kind":"source-ready"' "$log" >/dev/null 2>&1; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
wait_node_ready() {
  log=$1
  tries=0
  while [ "$tries" -lt 400 ]; do
    if grep -F '"state":"READY"' "$log" >/dev/null 2>&1; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
require_alive() {
  for pid in "$@"; do kill -0 "$pid" 2>/dev/null; done
}
record_role_exit() {
  name=$1
  pid=$2
  observed=$3
  expected=$4
  phase=$5
  printf '{"schema":"ardents-h4-8-a11-remote-role-exit-v1","role":"%s","pid":%s,"exit_status":%s,"expected_status":%s,"phase":"%s"}\n' \
    "$name" "$pid" "$observed" "$expected" "$phase" >>"$role_exit_statuses"
  [ "$observed" -eq "$expected" ]
}
wait_role() {
  name=$1
  pid=$2
  expected=$3
  phase=$4
  role_status=0
  wait "$pid" || role_status=$?
  record_role_exit "$name" "$pid" "$role_status" "$expected" "$phase"
}
write_fault_injection() {
  fault_name=$1
  target_role=$2
  target_pid=$3
  signal_name=$4
  ready_marker=$5
  printf '{"schema":"ardents-h4-8-a11-fault-injection-v1","fault":"%s","target_role":"%s","target_pid":%s,"signal":"%s","warmup_cycles":%s,"ready_marker":"%s","product_node_live":true,"carrier_relay_live":true}\n' \
    "$fault_name" "$target_role" "$target_pid" "$signal_name" "$warmup_cycles" "$ready_marker" >"$work/fault-injection.json.tmp"
  mv "$work/fault-injection.json.tmp" "$work/fault-injection.json"
}
start source-a "$work/ardents-node" source --config "$work/source-a-plan.json"
source_a=$started_pid
start source-b "$work/ardents-node" source --config "$work/source-b-plan.json"
source_b=$started_pid
wait_source "$work/source-a.log"
wait_source "$work/source-b.log"
if [ "$product_transit" = true ]; then
  (while :; do touch "$work/rendezvous-node-clock.observation"; sleep 0.1; done) &
  rendezvous_clock=$!
  pids="$pids $rendezvous_clock"
  start_stream rendezvous-node "$work/ardents-node" node --config "$work/rendezvous-node-plan.json"
  rendezvous_node=$started_pid
  printf '%s\n' "$rendezvous_node" >"$work/rendezvous-node.pid"
  wait_node_ready "$work/rendezvous-node.log"
  require_alive "$rendezvous_node"
  start carrier-relay "$work/reference-c2" carrier-relay "$work/reference-c2.json"
  carrier_relay=$started_pid
  printf '%s\n' "$carrier_relay" >"$work/carrier-relay.pid"
  wait_file "$work/carrier-relay-ready.json"
  require_alive "$rendezvous_node" "$carrier_relay"
  transit_roles="initiator introduction responder"
else
  transit_roles="rendezvous initiator introduction responder"
fi
for role in $transit_roles; do
  start "$role" "$work/reference-c2" "$role" "$work/reference-c2.json"
  transit_pids="$transit_pids $started_pid"
  transit_processes="$transit_processes $role:$started_pid"
done
for role in $transit_roles; do wait_file "$work/ready/$role"; done
start gateway "$work/reference-c2" gateway "$work/reference-c2.json"
gateway=$started_pid
start publisher "$work/reference-c2" publisher "$work/reference-c2.json"
publisher=$started_pid
printf '%s\n' "$publisher" >"$work/publisher.pid"
wait_file "$work/publication.json"
start alpha-gateway "$work/reference-c2" alpha-gateway "$work/reference-c2.json"
alpha_gateway=$started_pid
wait_file "$work/alpha-gateway-ready.json"
start alpha-relay "$work/reference-c2" alpha-relay "$work/reference-c2.json"
alpha_relay=$started_pid
wait_file "$work/alpha-relay-ready.json"
wait_file "$work/ready/gateway"
start publisher-app "$work/reference-c2" publisher-app "$work/reference-c2.json"
publisher_app=$started_pid
printf '%s\n' "$publisher_app" >"$work/publisher-app.pid"
if [ "$terminal" = endpoint-stop ]; then
  if [ "$product_transit" = true ]; then
    (wait_fault_file "$work/publisher-crash-ready"
     require_alive "$publisher" "$publisher_app" "$rendezvous_node" "$carrier_relay"
     write_fault_injection publisher-endpoint-loss publisher "$publisher" KILL publisher-crash-ready
     kill -KILL "$publisher"
     printf 'injected\n' >"$work/publisher-endpoint-kill-injected") &
  else
    (wait_fault_file "$work/publisher-crash-ready"; require_alive "$publisher" "$publisher_app"; kill -KILL "$publisher") &
  fi
  fault_pid=$!
  pids="$pids $fault_pid"
elif [ "$product_transit" = true ] && [ "$terminal" = application-reset ]; then
  (wait_fault_file "$work/publisher-application-fault-ready"
   require_alive "$publisher" "$publisher_app" "$rendezvous_node" "$carrier_relay" "$gateway" "$alpha_gateway" "$alpha_relay"
   for pid in $transit_pids; do require_alive "$pid"; done
   write_fault_injection publisher-application-loss publisher-app "$publisher_app" RESET publisher-application-fault-ready
   printf '{"schema":"ardents-h4-8-a11-publisher-application-reset-v1","fault":"publisher-application-loss","pid":%s,"action":"RESET","publisher_live_before":true,"publisher_app_live_before":true,"rendezvous_node_live_before":true,"carrier_relay_live_before":true,"transit_roles_live_before":true,"injected_after_ready":true}\n' "$publisher_app" >"$work/publisher-application-reset.json"
   printf 'inject\n' >"$work/publisher-application-fault-release") &
  fault_pid=$!
  pids="$pids $fault_pid"
elif [ "$fault" = carrier-loss ]; then
  (wait_fault_file "$work/transit-fault-ready"
   require_alive "$publisher" "$publisher_app" "$rendezvous_node" "$carrier_relay" "$gateway" "$alpha_gateway" "$alpha_relay"
   for pid in $transit_pids; do require_alive "$pid"; done
   printf 'reset\n' >"$work/carrier-relay-reset"
   wait_fault_file "$work/carrier-relay-reset.json"
   require_alive "$rendezvous_node" "$carrier_relay"
   write_fault_injection carrier-loss carrier-relay "$carrier_relay" RESET transit-fault-ready) &
  fault_pid=$!
  pids="$pids $fault_pid"
elif [ "$fault" = product-node-loss ]; then
  (wait_fault_file "$work/transit-fault-ready"
   require_alive "$publisher" "$publisher_app" "$rendezvous_node" "$carrier_relay" "$gateway" "$alpha_gateway" "$alpha_relay"
   for pid in $transit_pids; do require_alive "$pid"; done
   write_fault_injection product-node-loss rendezvous-node "$rendezvous_node" KILL transit-fault-ready
   kill -KILL "$rendezvous_node"
   printf 'injected\n' >"$work/rendezvous-node-kill-injected") &
  fault_pid=$!
  pids="$pids $fault_pid"
else
  fault_pid=""
fi
wait_completion_file "$work/complete"
if [ -n "$fault_pid" ]; then wait "$fault_pid"; fi
publisher_waited=false
if [ "$product_transit" = true ] && [ "$terminal" = endpoint-stop ]; then
  wait_file "$work/publisher-endpoint-kill-injected"
  publisher_status=0
  wait "$publisher" || publisher_status=$?
  record_role_exit publisher "$publisher" "$publisher_status" 137 fault
  [ "$publisher_status" -eq 137 ]
  printf '{"schema":"ardents-h4-8-a11-publisher-endpoint-kill-v1","fault":"publisher-endpoint-loss","pid":%s,"signal":"KILL","exit_status":137,"publisher_live_before":true,"publisher_app_live_before":true,"rendezvous_node_live_before":true,"carrier_relay_live_before":true,"injected_after_ready":true}\n' "$publisher" >"$work/publisher-endpoint-kill.json"
  publisher_waited=true
fi
if [ "$product_transit" = true ]; then
  if [ "$fault" = product-node-loss ]; then
    wait_file "$work/rendezvous-node-kill-injected"
    rendezvous_status=0
    wait "$rendezvous_node" || rendezvous_status=$?
    record_role_exit rendezvous-node "$rendezvous_node" "$rendezvous_status" 137 fault
    [ "$rendezvous_status" -eq 137 ]
    printf '{"schema":"ardents-h4-8-a11-rendezvous-kill-v1","fault":"product-node-loss","pid":%s,"signal":"KILL","exit_status":137,"rendezvous_node_live_before":true,"carrier_relay_live_before":true,"publisher_live_before":true,"publisher_app_live_before":true,"injected_after_ready":true}\n' "$rendezvous_node" >"$work/rendezvous-node-kill.json"
  else
    require_alive "$rendezvous_node"
    kill -TERM "$rendezvous_node"
    wait_role rendezvous-node "$rendezvous_node" 0 completion
    awk '/"state":"DRAINING"/{draining=1} /"state":"WITHDRAWN"/{if(draining) withdrawn=1} END{exit !withdrawn}' "$work/rendezvous-node.log"
  fi
fi
for item in "publisher:$publisher" "publisher-app:$publisher_app" "gateway:$gateway" "alpha-gateway:$alpha_gateway" "alpha-relay:$alpha_relay"; do
  name=${item%%:*}
  pid=${item#*:}
  if [ "$name" = publisher ] && [ "$publisher_waited" = true ]; then continue; fi
  expected=0
  phase=completion
  case "$terminal:$fault:$name" in
    endpoint-stop::publisher)
      expected=137
      phase=fault
      ;;
    application-reset::publisher-app|endpoint-stop::publisher-app|:carrier-loss:publisher-app|:product-node-loss:publisher-app)
      expected=2
      phase=terminal
      ;;
  esac
  wait_role "$name" "$pid" "$expected" "$phase"
done
for item in $transit_processes; do
  name=${item%%:*}
  pid=${item#*:}
  wait_role "$name" "$pid" 0 completion
done
if [ "$product_transit" = true ]; then wait_role carrier-relay "$carrier_relay" 0 completion; fi
kill -TERM "$source_a" "$source_b"
wait_role source-a "$source_a" 0 cleanup
wait_role source-b "$source_b" 0 cleanup
`
}

func TestH43RemoteRunnerHasPOSIXShellSyntax(t *testing.T) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal("Docker is unavailable for H4-3B remote-runner syntax validation")
	}
	command := exec.Command(docker, "run", "--rm", "-i", h43RemoteImage, "/bin/sh", "-n")
	command.Stdin = strings.NewReader(h43RemoteRunner())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("H4-3B remote runner shell syntax: %v\n%s", err, output)
	}
}

func TestH48A11RemoteRunnerRetainsExactProductTransitAndFaultReceipts(t *testing.T) {
	runner := h43RemoteRunner()
	for _, required := range []string{
		`start_stream rendezvous-node "$work/ardents-node" node --config "$work/rendezvous-node-plan.json"`,
		`mkfifo "$stream"`, `cat "$stream" >"$work/$name.log"`,
		`start carrier-relay "$work/reference-c2" carrier-relay "$work/reference-c2.json"`,
		`rendezvous-node.pid`, `carrier-relay.pid`, `publisher.pid`, `publisher-app.pid`, `carrier-relay-ready.json`, `topology.json`,
		`transit-fault-ready`, `fault-injection.json`, `carrier-relay-reset.json`, `rendezvous-node-kill.json`,
		`publisher-application-fault-ready`, `publisher-application-reset.json`, `publisher-endpoint-kill.json`,
		`kill -KILL "$publisher"`, `kill -KILL "$rendezvous_node"`,
		`[ "$publisher_status" -eq 137 ]`, `[ "$rendezvous_status" -eq 137 ]`,
		`remote-role-exit-statuses.jsonl`, `wait_role source-a "$source_a" 0 cleanup`, `wait_role source-b "$source_b" 0 cleanup`,
		`wait_completion_file "$work/complete"`,
		`wait_role "$name" "$pid" "$expected" "$phase"`,
		`awk '/"state":"DRAINING"/{draining=1} /"state":"WITHDRAWN"/{if(draining) withdrawn=1}`,
	} {
		if !strings.Contains(runner, required) {
			t.Fatalf("A11 remote runner is missing %q", required)
		}
	}
	if !strings.Contains(runner, `transit_roles="initiator introduction responder"`) ||
		!strings.Contains(runner, `if [ "$product_transit" = true ]; then`) {
		t.Fatal("A11 product topology did not exclude the fixture Rendezvous")
	}
}
