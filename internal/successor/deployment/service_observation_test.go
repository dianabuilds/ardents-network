package deployment

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

func TestInfrastructureServiceKeepsRoleAndPathBoundaries(t *testing.T) {
	const unit = `{"Id":{"type":"s","data":"ardents-forwarding.service"},"LoadState":{"type":"s","data":"loaded"},"ActiveState":{"type":"s","data":"inactive"},"SubState":{"type":"s","data":"dead"},"FragmentPath":{"type":"s","data":"/etc/systemd/system/ardents-forwarding.service"},"UnitFileState":{"type":"s","data":"static"},"DropInPaths":{"type":"as","data":[]},"Job":{"type":"(uo)","data":[0,"/"]},"InvocationID":{"type":"ay","data":[]}}`
	const service = `{"User":{"type":"s","data":"ardents-node"},"Group":{"type":"s","data":"ardents-node"},"Type":{"type":"s","data":"exec"},"WorkingDirectory":{"type":"s","data":"/"},"ProtectHome":{"type":"s","data":"yes"},"ProtectSystem":{"type":"s","data":"strict"},"KillMode":{"type":"s","data":"control-group"},"Restart":{"type":"s","data":"no"},"RootDirectory":{"type":"s","data":""},"RootImage":{"type":"s","data":""},"ExitType":{"type":"s","data":"main"},"RestartMode":{"type":"s","data":"normal"},"StandardOutput":{"type":"s","data":"journal"},"StandardError":{"type":"s","data":"journal"},"ControlGroup":{"type":"s","data":""},"CapabilityBoundingSet":{"type":"t","data":0},"AmbientCapabilities":{"type":"t","data":0},"LimitCORE":{"type":"t","data":0},"LimitCORESoft":{"type":"t","data":0},"LimitNOFILE":{"type":"t","data":1024},"LimitNOFILESoft":{"type":"t","data":1024},"CPUQuotaPerSecUSec":{"type":"t","data":1000000},"CPUQuotaPeriodUSec":{"type":"t","data":100000},"MemoryHigh":{"type":"t","data":805306368},"MemoryMax":{"type":"t","data":1073741824},"TasksMax":{"type":"t","data":256},"TimeoutStopUSec":{"type":"t","data":30000000},"NoNewPrivileges":{"type":"b","data":true},"PrivateTmp":{"type":"b","data":true},"ProtectControlGroups":{"type":"b","data":true},"ProtectKernelTunables":{"type":"b","data":true},"ProtectKernelModules":{"type":"b","data":true},"ProtectKernelLogs":{"type":"b","data":true},"RestrictSUIDSGID":{"type":"b","data":true},"LockPersonality":{"type":"b","data":true},"MemoryAccounting":{"type":"b","data":true},"CPUAccounting":{"type":"b","data":true},"TasksAccounting":{"type":"b","data":true},"MainPID":{"type":"u","data":0},"ExecMainPID":{"type":"u","data":0},"UMask":{"type":"u","data":63},"RemainAfterExit":{"type":"b","data":false},"SupplementaryGroups":{"type":"as","data":[]},"ReadWritePaths":{"type":"as","data":["/var/lib/ardents-state","/var/lib/ardents-participation","/var/lib/ardents-spend","/var/lib/ardents-hosting"]},"Environment":{"type":"as","data":["GOMAXPROCS=1","GOMEMLIMIT=536870912"]},"ExecCondition":{"type":"a(sasbttttuii)","data":[]},"ExecStartPre":{"type":"a(sasbttttuii)","data":[]},"ExecStartPost":{"type":"a(sasbttttuii)","data":[]},"ExecReload":{"type":"a(sasbttttuii)","data":[]},"ExecStop":{"type":"a(sasbttttuii)","data":[]},"ExecStopPost":{"type":"a(sasbttttuii)","data":[]},"RestrictAddressFamilies":{"type":"(bas)","data":[true,["AF_UNIX","AF_INET","AF_INET6"]]},"ExecStartEx":{"type":"a(sasasttttuii)","data":[["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64",["/opt/ardents/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64","forwarding","start-installed","/opt/ardents/binding.json"],["no-env-expand"],0,0,0,0,0,0,0]]}}`
	for _, role := range []string{"source", "forwarding", "join", "descriptor", "introduction"} {
		t.Run(role, func(t *testing.T) {
			var u, s systemdproperty.Properties
			if json.Unmarshal([]byte(unit), &u) != nil || json.Unmarshal([]byte(service), &s) != nil {
				t.Fatal("invalid independent vector")
			}
			set := func(p systemdproperty.Properties, name, sig string, value any) {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				p[name] = systemdproperty.Value{Type: sig, Data: data}
			}
			name := "ardents-" + role + ".service"
			set(u, "Id", "s", name)
			set(u, "FragmentPath", "s", "/etc/systemd/system/"+name)
			account := "ardents-node"
			paths := []string{"/var/lib/ardents-state", "/var/lib/ardents-participation", "/var/lib/ardents-spend", "/var/lib/ardents-hosting"}
			if role == "source" {
				account = "ardents-source"
				paths = paths[:2]
			}
			if role == "descriptor" || role == "introduction" {
				paths = append(paths, "/var/lib/ardents-"+role)
			}
			set(s, "User", "s", account)
			set(s, "Group", "s", account)
			set(s, "ReadWritePaths", "as", paths)
			generation := strings.Repeat("a", 64)
			program := "/opt/ardents/generations/" + generation + "/ardents-node-linux-amd64"
			set(s, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, role, "start-installed", "/opt/ardents/binding.json"}, []string{"no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}})
			check := func(r string, p []string) bool {
				_, _, ok := CheckInfrastructureService(u, s, r, "/opt/ardents", generation, "ardents-node-linux-amd64", p)
				return ok
			}
			if !check(role, paths) {
				t.Fatal("valid fixed role refused")
			}
			if check("unknown", paths) || check("foreign", paths) {
				t.Fatal("unknown role accepted")
			}
			for _, other := range []string{"source", "forwarding", "join", "descriptor", "introduction"} {
				if other != role && check(other, paths) {
					t.Fatal("foreign role accepted", other)
				}
			}
			for i := range paths {
				altered := append([]string(nil), paths...)
				altered[i] += "-foreign"
				if check(role, altered) {
					t.Fatal("foreign write root accepted", i)
				}
			}
			set(s, "ExecStartPre", "a(sasbttttuii)", []string{"foreign"})
			if check(role, paths) {
				t.Fatal(fmt.Sprintf("%s accepted a pre-start hook", role))
			}
		})
	}
}
