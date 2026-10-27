package forwarding

import (
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"strings"
	"testing"
)

func TestForwardingLoadedPolicyRejectsEveryChangedProperty(t *testing.T) {
	r := Request{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation", SpendRoot: "/var/lib/ardents-spend", HostingRoot: "/var/lib/ardents-hosting"}
	g := strings.Repeat("a", 64)
	u, s := forwardingLoadedVector(t)
	if err := verifyForwardingStopped(u, s, r, g); err != nil {
		t.Fatal("independent complete stopped vector", err)
	}
	for label, p := range map[string]systemd.Properties{"unit": u, "service": s} {
		for name, v := range p {
			delete(p, name)
			if verifyForwardingStopped(u, s, r, g) == nil {
				t.Fatal("missing property accepted", label, name)
			}
			p[name] = systemd.Value{Type: "v", Data: v.Data}
			if verifyForwardingStopped(u, s, r, g) == nil {
				t.Fatal("wrong wire signature accepted", label, name)
			}
			p[name] = v
		}
	}
	for _, change := range []struct {
		p         systemd.Properties
		name, sig string
		value     any
	}{
		{u, "ActiveState", "s", "active"}, {u, "DropInPaths", "as", []string{"/etc/systemd/system/ardents-forwarding.service.d/foreign.conf"}},
		{u, "InvocationID", "ay", [16]byte{}}, {u, "InvocationID", "ay", []int{1}}, {u, "InvocationID", "ay", nil},
		{u, "Job", "(uo)", []any{1, "/org/freedesktop/systemd1/job/1"}}, {s, "MemoryMax", "t", 1073741825}, {s, "User", "s", "root"},
		{s, "Environment", "as", []string{"GOMAXPROCS=2", "GOMEMLIMIT=536870912"}},
	} {
		old := change.p[change.name]
		raw, _ := json.Marshal(change.value)
		change.p[change.name] = systemd.Value{Type: change.sig, Data: raw}
		if verifyForwardingStopped(u, s, r, g) == nil {
			t.Fatal("causal policy change accepted", change.name)
		}
		change.p[change.name] = old
	}
}

func TestForwardingStartSelectorsRequireExactInvocationAndOwnJob(t *testing.T) {
	unit := systemd.Properties{"InvocationID": {Type: "ay", Data: json.RawMessage(`[9,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}, "Job": {Type: "(uo)", Data: json.RawMessage(`[73,"/org/freedesktop/systemd1/job/73"]`)}}
	service := systemd.Properties{"MainPID": {Type: "u", Data: json.RawMessage(`7317`)}}
	pid, id, err := runningInvocation(unit, service)
	if err != nil || pid != 7317 || id != [16]byte{9} {
		t.Fatal("exact detached invocation grammar refused", err)
	}
	for _, raw := range []string{`[]`, `[9]`, `null`, `"CQAAAAAAAAAAAAAAAAAAAA=="`, `[9,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1]`, `[256,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`, `[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`} {
		unit["InvocationID"] = systemd.Value{Type: "ay", Data: json.RawMessage(raw)}
		if _, _, err := runningInvocation(unit, service); err == nil {
			t.Fatal("truncated/foreign invocation admitted", raw)
		}
	}
	if !ownQueuedJob(unit, "/org/freedesktop/systemd1/job/73") {
		t.Fatal("matching original queued job refused")
	}
	for _, object := range []string{"", "/", "/org/freedesktop/systemd1/job/0", "/org/freedesktop/systemd1/job/073", "/org/freedesktop/systemd1/job/74", "/org/freedesktop/systemd1/job/-73"} {
		if ownQueuedJob(unit, object) {
			t.Fatal("foreign or noncanonical job admitted", object)
		}
	}
}

func TestForwardingRunningPolicyRequiresOriginalProcessAndEveryProtectionFact(t *testing.T) {
	r := Request{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation", SpendRoot: "/var/lib/ardents-spend", HostingRoot: "/var/lib/ardents-hosting"}
	g := strings.Repeat("a", 64)
	const pid uint32 = 7317
	id := [16]byte{9}
	u, s := forwardingLoadedVector(t)
	set := func(p systemd.Properties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		p[name] = systemd.Value{Type: signature, Data: raw}
	}
	set(u, "ActiveState", "s", "active")
	set(u, "SubState", "s", "running")
	set(u, "InvocationID", "ay", id)
	set(u, "TriggeredBy", "as", []string{})
	for _, name := range []string{"MainPID", "ExecMainPID"} {
		set(s, name, "u", pid)
	}
	for _, name := range []string{"ControlPID", "NRestarts"} {
		set(s, name, "u", uint32(0))
	}
	set(s, "DynamicUser", "b", false)
	set(s, "ControlGroup", "s", "/system.slice/ardents-forwarding.service")
	set(s, "StandardInput", "s", "null")
	set(s, "ExecMainStartTimestamp", "t", uint64(30))
	set(s, "ExecMainStartTimestampMonotonic", "t", uint64(40))
	program := "/opt/ardents/generations/" + g + "/ardents-node-linux-amd64"
	set(s, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "forwarding", "start-installed", "/opt/ardents/binding.json"}, []string{"no-env-expand"}, uint64(10), uint64(20), uint64(0), uint64(0), pid, int32(0), int32(0)}})
	// Detached properties test their grammar only; they supply no native process,
	// manager, root completion or successful Startup proof.
	if err := verifyForwardingRunning(u, s, r, g, pid, id); err != nil {
		t.Fatal("complete running property vector refused", err)
	}
	if verifyForwardingRunning(u, s, r, g, pid+1, id) == nil || verifyForwardingRunning(u, s, r, g, pid, [16]byte{10}) == nil || verifyForwardingStopped(u, s, r, g) == nil {
		t.Fatal("running property facts generalized another process, invocation or phase")
	}
	for _, p := range []systemd.Properties{u, s} {
		for name, original := range p {
			delete(p, name)
			if verifyForwardingRunning(u, s, r, g, pid, id) == nil {
				t.Fatal("missing running/protection property accepted", name)
			}
			p[name] = systemd.Value{Type: "v", Data: original.Data}
			if verifyForwardingRunning(u, s, r, g, pid, id) == nil {
				t.Fatal("foreign running wire type accepted", name)
			}
			p[name] = original
		}
	}
}

func forwardingLoadedVector(t *testing.T) (systemd.Properties, systemd.Properties) {
	t.Helper()
	var u, s systemd.Properties
	const unit = `{"Id":{"type":"s","data":"ardents-forwarding.service"},"LoadState":{"type":"s","data":"loaded"},"ActiveState":{"type":"s","data":"inactive"},"SubState":{"type":"s","data":"dead"},"FragmentPath":{"type":"s","data":"/etc/systemd/system/ardents-forwarding.service"},"UnitFileState":{"type":"s","data":"static"},"DropInPaths":{"type":"as","data":[]},"Job":{"type":"(uo)","data":[0,"/"]},"InvocationID":{"type":"ay","data":[]}}`
	const service = `{"User":{"type":"s","data":"ardents-node"},"Group":{"type":"s","data":"ardents-node"},"Type":{"type":"s","data":"exec"},"WorkingDirectory":{"type":"s","data":"/"},"ProtectHome":{"type":"s","data":"yes"},"ProtectSystem":{"type":"s","data":"strict"},"KillMode":{"type":"s","data":"control-group"},"Restart":{"type":"s","data":"no"},"RootDirectory":{"type":"s","data":""},"RootImage":{"type":"s","data":""},"ExitType":{"type":"s","data":"main"},"RestartMode":{"type":"s","data":"normal"},"StandardOutput":{"type":"s","data":"journal"},"StandardError":{"type":"s","data":"journal"},"ControlGroup":{"type":"s","data":""},"CapabilityBoundingSet":{"type":"t","data":0},"AmbientCapabilities":{"type":"t","data":0},"LimitCORE":{"type":"t","data":0},"LimitCORESoft":{"type":"t","data":0},"LimitNOFILE":{"type":"t","data":1024},"LimitNOFILESoft":{"type":"t","data":1024},"CPUQuotaPerSecUSec":{"type":"t","data":1000000},"CPUQuotaPeriodUSec":{"type":"t","data":100000},"MemoryHigh":{"type":"t","data":805306368},"MemoryMax":{"type":"t","data":1073741824},"TasksMax":{"type":"t","data":256},"TimeoutStopUSec":{"type":"t","data":30000000},"NoNewPrivileges":{"type":"b","data":true},"PrivateTmp":{"type":"b","data":true},"ProtectControlGroups":{"type":"b","data":true},"ProtectKernelTunables":{"type":"b","data":true},"ProtectKernelModules":{"type":"b","data":true},"ProtectKernelLogs":{"type":"b","data":true},"RestrictSUIDSGID":{"type":"b","data":true},"LockPersonality":{"type":"b","data":true},"MemoryAccounting":{"type":"b","data":true},"CPUAccounting":{"type":"b","data":true},"TasksAccounting":{"type":"b","data":true},"MainPID":{"type":"u","data":0},"ExecMainPID":{"type":"u","data":0},"UMask":{"type":"u","data":63},"RemainAfterExit":{"type":"b","data":false},"SupplementaryGroups":{"type":"as","data":[]},"ReadWritePaths":{"type":"as","data":["/var/lib/ardents-state","/var/lib/ardents-participation","/var/lib/ardents-spend","/var/lib/ardents-hosting"]},"Environment":{"type":"as","data":["GOMAXPROCS=1","GOMEMLIMIT=536870912"]},"ExecCondition":{"type":"a(sasbttttuii)","data":[]},"ExecStartPre":{"type":"a(sasbttttuii)","data":[]},"ExecStartPost":{"type":"a(sasbttttuii)","data":[]},"ExecReload":{"type":"a(sasbttttuii)","data":[]},"ExecStop":{"type":"a(sasbttttuii)","data":[]},"ExecStopPost":{"type":"a(sasbttttuii)","data":[]},"RestrictAddressFamilies":{"type":"(bas)","data":[true,["AF_UNIX","AF_INET","AF_INET6"]]},"ExecStartEx":{"type":"a(sasasttttuii)","data":[["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64",["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64","forwarding","start-installed","/opt/ardents/binding.json"],["no-env-expand"],0,0,0,0,0,0,0]]}}`
	if json.Unmarshal([]byte(unit), &u) != nil || json.Unmarshal([]byte(service), &s) != nil {
		t.Fatal("invalid independent vector")
	}
	for _, name := range []string{"ExecMainStartTimestamp", "ExecMainStartTimestampMonotonic", "ExecMainExitTimestamp", "ExecMainExitTimestampMonotonic"} {
		s[name] = systemd.Value{Type: "t", Data: json.RawMessage(`0`)}
	}
	for _, name := range []string{"ExecMainCode", "ExecMainStatus"} {
		s[name] = systemd.Value{Type: "i", Data: json.RawMessage(`0`)}
	}
	return u, s
}
