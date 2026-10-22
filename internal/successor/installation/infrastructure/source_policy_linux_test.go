package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

func TestSourceAccountDeltaPreservesForeignBytesAndLockedIdentity(t *testing.T) {
	before := []byte("root:x:0:0:root:/root:/bin/bash\n")
	for _, v := range []struct{ path, row string }{
		{"/etc/passwd", "ardents-source:x:999:999::/nonexistent:/usr/sbin/nologin"},
		{"/etc/group", "ardents-source:x:999:"},
		{"/etc/shadow", "ardents-source:!:20000::::::"},
		{"/etc/gshadow", "ardents-source:!::"},
	} {
		after := append(bytes.Clone(before), []byte(v.row+"\n")...)
		if err := sourceAccountDelta(v.path, before, after, 999, 999); err != nil {
			t.Fatal(v.path, err)
		}
		for _, bad := range [][]byte{after[:len(after)-1], append(bytes.Clone(after), []byte(v.row+"\n")...), bytes.Replace(after, []byte("root:x"), []byte("root:!"), 1), bytes.Replace(after, []byte(v.row), []byte(strings.ReplaceAll(v.row, "999", "998")), 1)} {
			if bytes.Equal(bad, after) {
				continue
			}
			if sourceAccountDelta(v.path, before, bad, 999, 999) == nil {
				t.Fatalf("accepted substituted account/preimage: %s", v.path)
			}
		}
	}
	for _, row := range []string{"ardents-source::20000::::::", "ardents-source:$6$hash:20000::::::", "ardents-source:!:20000:0:99999:7:::"} {
		if sourceAccountDelta("/etc/shadow", before, append(bytes.Clone(before), []byte(row+"\n")...), 999, 999) == nil {
			t.Fatal("unlocked/aging shadow accepted")
		}
	}
}

func TestSourceLoadedPolicyRejectsEveryChangedProperty(t *testing.T) {
	r := SourceRequest{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation"}
	g := strings.Repeat("a", 64)
	u, s := sourceLoadedVector(t)
	if err := verifySourceStopped(u, s, r, g); err != nil {
		t.Fatal("independent complete stopped vector", err)
	}
	for label, p := range map[string]systemd.Properties{"unit": u, "service": s} {
		for name, v := range p {
			delete(p, name)
			if verifySourceStopped(u, s, r, g) == nil {
				t.Fatal("missing property accepted", label, name)
			}
			p[name] = systemd.Value{Type: "v", Data: v.Data}
			if verifySourceStopped(u, s, r, g) == nil {
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
		{u, "ActiveState", "s", "active"}, {u, "DropInPaths", "as", []string{"/etc/systemd/system/ardents-source.service.d/foreign.conf"}},
		{u, "InvocationID", "ay", [16]byte{}}, {u, "InvocationID", "ay", []int{1}}, {u, "InvocationID", "ay", nil},
		{u, "Job", "(uo)", []any{1, "/org/freedesktop/systemd1/job/1"}}, {s, "MemoryMax", "t", 1073741825}, {s, "User", "s", "root"},
		{s, "Environment", "as", []string{"GOMAXPROCS=2", "GOMEMLIMIT=536870912"}},
	} {
		old := change.p[change.name]
		raw, _ := json.Marshal(change.value)
		change.p[change.name] = systemd.Value{Type: change.sig, Data: raw}
		if verifySourceStopped(u, s, r, g) == nil {
			t.Fatal("causal policy change accepted", change.name)
		}
		change.p[change.name] = old
	}
}

func sourceLoadedVector(t *testing.T) (systemd.Properties, systemd.Properties) {
	t.Helper()
	var u, s systemd.Properties
	const unit = `{"Id":{"type":"s","data":"ardents-source.service"},"LoadState":{"type":"s","data":"loaded"},"ActiveState":{"type":"s","data":"inactive"},"SubState":{"type":"s","data":"dead"},"FragmentPath":{"type":"s","data":"/etc/systemd/system/ardents-source.service"},"UnitFileState":{"type":"s","data":"static"},"DropInPaths":{"type":"as","data":[]},"Job":{"type":"(uo)","data":[0,"/"]},"InvocationID":{"type":"ay","data":[]}}`
	const service = `{"User":{"type":"s","data":"ardents-source"},"Group":{"type":"s","data":"ardents-source"},"Type":{"type":"s","data":"exec"},"WorkingDirectory":{"type":"s","data":"/"},"ProtectHome":{"type":"s","data":"yes"},"ProtectSystem":{"type":"s","data":"strict"},"KillMode":{"type":"s","data":"control-group"},"Restart":{"type":"s","data":"no"},"RootDirectory":{"type":"s","data":""},"RootImage":{"type":"s","data":""},"ExitType":{"type":"s","data":"main"},"RestartMode":{"type":"s","data":"normal"},"StandardOutput":{"type":"s","data":"journal"},"StandardError":{"type":"s","data":"journal"},"ControlGroup":{"type":"s","data":""},"CapabilityBoundingSet":{"type":"t","data":0},"AmbientCapabilities":{"type":"t","data":0},"LimitCORE":{"type":"t","data":0},"LimitCORESoft":{"type":"t","data":0},"LimitNOFILE":{"type":"t","data":1024},"LimitNOFILESoft":{"type":"t","data":1024},"CPUQuotaPerSecUSec":{"type":"t","data":1000000},"CPUQuotaPeriodUSec":{"type":"t","data":100000},"MemoryHigh":{"type":"t","data":805306368},"MemoryMax":{"type":"t","data":1073741824},"TasksMax":{"type":"t","data":256},"TimeoutStopUSec":{"type":"t","data":30000000},"NoNewPrivileges":{"type":"b","data":true},"PrivateTmp":{"type":"b","data":true},"ProtectControlGroups":{"type":"b","data":true},"ProtectKernelTunables":{"type":"b","data":true},"ProtectKernelModules":{"type":"b","data":true},"ProtectKernelLogs":{"type":"b","data":true},"RestrictSUIDSGID":{"type":"b","data":true},"LockPersonality":{"type":"b","data":true},"MemoryAccounting":{"type":"b","data":true},"CPUAccounting":{"type":"b","data":true},"TasksAccounting":{"type":"b","data":true},"MainPID":{"type":"u","data":0},"ExecMainPID":{"type":"u","data":0},"UMask":{"type":"u","data":63},"RemainAfterExit":{"type":"b","data":false},"SupplementaryGroups":{"type":"as","data":[]},"ReadWritePaths":{"type":"as","data":["/var/lib/ardents-state","/var/lib/ardents-participation"]},"Environment":{"type":"as","data":["GOMAXPROCS=1","GOMEMLIMIT=536870912"]},"ExecCondition":{"type":"a(sasbttttuii)","data":[]},"ExecStartPre":{"type":"a(sasbttttuii)","data":[]},"ExecStartPost":{"type":"a(sasbttttuii)","data":[]},"ExecReload":{"type":"a(sasbttttuii)","data":[]},"ExecStop":{"type":"a(sasbttttuii)","data":[]},"ExecStopPost":{"type":"a(sasbttttuii)","data":[]},"RestrictAddressFamilies":{"type":"(bas)","data":[true,["AF_UNIX","AF_INET","AF_INET6"]]},"ExecStartEx":{"type":"a(sasasttttuii)","data":[["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64",["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64","source","start-installed","/opt/ardents/binding.json"],["no-env-expand"],0,0,0,0,0,0,0]]}}`
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
