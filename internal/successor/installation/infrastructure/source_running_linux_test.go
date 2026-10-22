package infrastructure

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

func TestSourceQueuedJobMatchesOnlyOriginalNativeReply(t *testing.T) {
	unit := systemd.Properties{"Job": {Type: "(uo)", Data: json.RawMessage(`[17,"/org/freedesktop/systemd1/job/17"]`)}}
	if !sourceOwnQueuedJob(unit, "/org/freedesktop/systemd1/job/17") {
		t.Fatal("own asynchronous queued job was not retained")
	}
	for _, object := range []string{"", "/", "/org/freedesktop/systemd1/job/0", "/org/freedesktop/systemd1/job/017", "/org/freedesktop/systemd1/job/18", "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice"} {
		if sourceOwnQueuedJob(unit, object) {
			t.Fatal("foreign or noncanonical job admitted pending wait", object)
		}
	}
	for _, value := range []systemd.Value{
		{Type: "(so)", Data: json.RawMessage(`["17","/org/freedesktop/systemd1/job/17"]`)},
		{Type: "(uo)", Data: json.RawMessage(`[18,"/org/freedesktop/systemd1/job/17"]`)},
		{Type: "(uo)", Data: json.RawMessage(`[0,"/"]`)},
		{Type: "(uo)", Data: json.RawMessage(`[17,"/org/freedesktop/systemd1/job/17",0]`)},
	} {
		unit["Job"] = value
		if sourceOwnQueuedJob(unit, "/org/freedesktop/systemd1/job/17") {
			t.Fatal("wrong native job tuple admitted wait", value)
		}
	}
}

func TestSourceRunningPolicyRequiresOriginalInvocationAndCompleteProtection(t *testing.T) {
	r := SourceRequest{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation"}
	u, s := sourceLoadedVector(t)
	id := [16]byte{1, 2, 3}
	put := func(p systemd.Properties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		p[name] = systemd.Value{Type: signature, Data: raw}
	}
	put(u, "ActiveState", "s", "active")
	put(u, "SubState", "s", "running")
	put(u, "InvocationID", "ay", id)
	put(u, "TriggeredBy", "as", []string{})
	put(s, "MainPID", "u", uint32(71))
	put(s, "ExecMainPID", "u", uint32(71))
	put(s, "ControlPID", "u", uint32(0))
	put(s, "NRestarts", "u", uint32(0))
	put(s, "DynamicUser", "b", false)
	put(s, "ControlGroup", "s", "/system.slice/ardents-source.service")
	put(s, "StandardInput", "s", "null")
	// Literal native tuple: start realtime/monotonic, no exit clocks,
	// exact PID, zero wait code/status. This is a policy vector, no process proof.
	s["ExecStartEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: json.RawMessage(`[["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64",["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64","source","start-installed","/opt/ardents/binding.json"],["no-env-expand"],100,200,0,0,71,0,0]]`)}
	generation := strings.Repeat("a", 64)
	if err := verifySourceRunning(u, s, r, generation, 71, id); err != nil {
		t.Fatal("complete independent running policy vector", err)
	}
	pid, gotID, err := sourceInvocation(u, s)
	if err != nil || pid != 71 || gotID != id {
		t.Fatal("typed invocation facts differ", pid, gotID, err)
	}
	for label, p := range map[string]systemd.Properties{"unit": u, "service": s} {
		for name, value := range p {
			delete(p, name)
			if verifySourceRunning(u, s, r, generation, 71, id) == nil {
				t.Fatal("incomplete running fact accepted", label, name)
			}
			p[name] = systemd.Value{Type: "v", Data: value.Data}
			if verifySourceRunning(u, s, r, generation, 71, id) == nil {
				t.Fatal("wrong native running signature accepted", label, name)
			}
			p[name] = value
		}
	}
	if verifySourceRunning(u, s, r, generation, 72, id) == nil || verifySourceRunning(u, s, r, generation, 71, [16]byte{4}) == nil || verifySourceStopped(u, s, r, generation) == nil {
		t.Fatal("another process/invocation or phase accepted")
	}
	for _, value := range []json.RawMessage{json.RawMessage(`[]`), json.RawMessage(`"AQID"`), json.RawMessage(`[1,2,3]`), json.RawMessage(`[256,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)} {
		u["InvocationID"] = systemd.Value{Type: "ay", Data: value}
		if _, _, err := sourceInvocation(u, s); err == nil {
			t.Fatal("non-native invocation bytes accepted", string(value))
		}
	}
}

func TestSourceCompletedStopRequiresOriginalExecutionAndCleanExit(t *testing.T) {
	r := SourceRequest{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation"}
	u, s := sourceLoadedVector(t)
	id := [16]byte{1, 2, 3}
	put := func(p systemd.Properties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		p[name] = systemd.Value{Type: signature, Data: raw}
	}
	put(u, "InvocationID", "ay", id)
	put(u, "TriggeredBy", "as", []string{})
	put(s, "MainPID", "u", uint32(0))
	put(s, "ExecMainPID", "u", uint32(71))
	put(s, "ControlPID", "u", uint32(0))
	put(s, "NRestarts", "u", uint32(0))
	put(s, "DynamicUser", "b", false)
	put(s, "ControlGroup", "s", "")
	put(s, "StandardInput", "s", "null")
	s["ExecStartEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: json.RawMessage(`[["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64",["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64","source","start-installed","/opt/ardents/binding.json"],["no-env-expand"],100,200,300,400,71,1,0]]`)}
	started := &sourceStartHistory{id: id, archive: sourceStartArchive{PID: 71, StartedRealtime: 100, StartedMonotonic: 200}}
	execution := sourceExecution{100, 200, 300, 400, 71, 1, 0}
	generation := strings.Repeat("a", 64)
	if err := verifySourceCompletedStop(u, s, r, generation, started, execution); err != nil {
		t.Fatal("independent complete stop vector refused", err)
	}
	for _, wrong := range []sourceExecution{
		{100, 200, 300, 400, 72, 1, 0},
		{101, 200, 300, 400, 71, 1, 0},
		{100, 201, 300, 400, 71, 1, 0},
		{100, 200, 0, 0, 71, 1, 0},
		{100, 200, 300, 400, 71, 1, 2},
		{100, 200, 300, 400, 71, 2, 9},
	} {
		if verifySourceCompletedStop(u, s, r, generation, started, wrong) == nil {
			t.Fatal("foreign, crashed or failed completion accepted", wrong)
		}
	}
	put(u, "InvocationID", "ay", [16]byte{9})
	if verifySourceCompletedStop(u, s, r, generation, started, execution) == nil {
		t.Fatal("another native invocation supplied original retirement")
	}
}
