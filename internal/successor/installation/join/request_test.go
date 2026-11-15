package join

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const requestVector = "{\"schema\":\"ardents-join-installation-v1\",\"invocation\":\"join\",\"profile\":\"ardents-join-process-v1\",\"installation_root\":\"/opt/ardents-join\",\"state_root\":\"/var/lib/ardents-node-state\",\"participation_root\":\"/var/lib/ardents-node-participation\",\"spend_root\":\"/var/lib/ardents-node-spend\",\"hosting_root\":\"/var/lib/ardents-node-hosting\",\"credential_directory\":\"/etc/ardents-node\",\"plan\":\"/var/lib/ardents-delivery/join.json\",\"certificate\":\"/var/lib/ardents-delivery/certificate.pem\",\"private_key\":\"/var/lib/ardents-delivery/private-key.pem\",\"clock\":\"/var/lib/ardents-clock/observation\"}\n"

func TestRequestCanonicalRolePathsAndRuntimeNamespace(t *testing.T) {
	r, err := DecodeRequest([]byte(requestVector))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil || !bytes.Equal(append(raw, '\n'), []byte(requestVector)) {
		t.Fatal("independent canonical request", err)
	}
	for _, change := range []struct{ from, to string }{
		{"\"join\"", "\"forwarding\""}, {"ardents-join-process-v1", "ardents-forwarding-process-v1"},
		{"\"profile\":", "\"profile\":\"ardents-join-process-v1\",\"profile\":"},
		{"\"plan\":", "\"extra\":0,\"plan\":"},
		{"/var/lib/ardents-node-spend", "/var/lib/ardents-node-state/spend"},
		{"/var/lib/ardents-node-hosting", "/var/lib/ardents-node-state"},
		{"/opt/ardents-join", "/opt/../ardents-join"}, {"/etc/ardents-node", "/etc/ardents%node"},
		{"/var/lib/ardents-clock/observation", "/var/lib/ardents-node-hosting/clock"},
	} {
		if _, err := DecodeRequest([]byte(strings.Replace(requestVector, change.from, change.to, 1))); err == nil {
			t.Fatal("foreign, overlapping or noncanonical request", change)
		}
	}
	mutations := []func(*Request, string){
		func(r *Request, path string) { r.InstallationRoot = path }, func(r *Request, path string) { r.StateRoot = path },
		func(r *Request, path string) { r.ParticipationRoot = path }, func(r *Request, path string) { r.SpendRoot = path },
		func(r *Request, path string) { r.HostingRoot = path }, func(r *Request, path string) { r.CredentialDirectory = path },
		func(r *Request, path string) { r.Clock = path },
	}
	for _, hidden := range []string{"/tmp", "/var/tmp", "/home", "/root", "/run/user"} {
		for _, mutate := range mutations {
			changed := r
			mutate(&changed, hidden+"/hidden-runtime-input")
			raw, _ := json.Marshal(changed)
			if _, err := DecodeRequest(append(raw, '\n')); err == nil {
				t.Fatal("fixed namespace hides a runtime path", hidden)
			}
		}
	}
	for _, raw := range [][]byte{nil, bytes.TrimSpace([]byte(requestVector)), []byte(requestVector + "\n"), bytes.Repeat([]byte("x"), (16<<10)+1)} {
		if _, err := DecodeRequest(raw); err == nil {
			t.Fatal("noncanonical request accepted")
		}
	}
}

func TestRequestReservesFiniteJournalsAndCompletionPaths(t *testing.T) {
	r, err := DecodeRequest([]byte(requestVector))
	if err != nil {
		t.Fatal(err)
	}
	reserved := []string{r.InstallationRoot + ".join-initial-journal", r.InstallationRoot + ".join-initial-pending", r.InstallationRoot + ".join-hosting", r.InstallationRoot + ".join-hosting-journal", r.InstallationRoot + ".join-hosting-pending", r.InstallationRoot + ".join-first-start-journal"}
	for n := 0; n < 64; n++ {
		if n != 0 {
			reserved = append(reserved, r.InstallationRoot+fmt.Sprintf(".join-invocation-%08d-journal", n))
		}
		reserved = append(reserved, r.InstallationRoot+fmt.Sprintf(".join-retirement-%08d-journal", n))
	}
	for _, path := range reserved {
		for _, candidate := range []string{path, path + "/leaf"} {
			changed := r
			changed.Clock = candidate
			raw, _ := json.Marshal(changed)
			if _, err := DecodeRequest(append(raw, '\n')); !errors.Is(err, ErrBinding) {
				t.Fatal("future journal admitted as live clock", candidate, err)
			}
			if err := r.CheckPreparationOutput(candidate); !errors.Is(err, ErrBinding) {
				t.Fatal("public output can occupy future journal", candidate, err)
			}
		}
	}
	const longest = "/join-invocation-00000063/start-completion.socket"
	for _, size := range []int{107, 108} {
		changed := r
		changed.InstallationRoot = "/opt/" + strings.Repeat("x", size-len(longest)-len("/opt/"))
		raw, _ := json.Marshal(changed)
		_, err := DecodeRequest(append(raw, '\n'))
		if (err == nil) != (size == 107) {
			t.Fatal("original completion namespace does not fit", size, err)
		}
	}
	if err := r.CheckPreparationOutput("/var/lib/join-public-output"); err != nil {
		t.Fatal("separate preparation output", err)
	}
}

func TestFixedJoinTemplateHasOnePurposeAndUnchangedProtection(t *testing.T) {
	unit := Template
	for _, line := range []string{
		"User=ardents-node", "Group=ardents-node", "SupplementaryGroups=", "Restart=no", "KillMode=control-group", "NoNewPrivileges=yes",
		"CapabilityBoundingSet=", "AmbientCapabilities=", "PrivateTmp=yes", "ProtectHome=yes", "ProtectSystem=strict",
		"CPUQuota=100%", "CPUQuotaPeriodSec=100ms", "MemoryHigh=805306368", "MemoryMax=1073741824", "TasksMax=256", "LimitNOFILE=1024",
		"Environment=GOMAXPROCS=1 GOMEMLIMIT=536870912",
		"ExecStart=:@ARDENTS_JOIN_PROGRAM@ join start-installed @ARDENTS_JOIN_BINDING@",
		"ReadWritePaths=@ARDENTS_JOIN_WRITE_PATHS@",
	} {
		if strings.Count(unit, "\n"+line+"\n") != 1 {
			t.Fatal("fixed JOIN property differs", line)
		}
	}
	if strings.Contains(unit, "forwarding") || strings.Contains(unit, "ExecStartPre=") || strings.Contains(unit, "DynamicUser=") || strings.Count(unit, "ExecStart=") != 1 {
		t.Fatal("foreign or additional command in fixed JOIN template")
	}
}

func TestJoinRenderingBindsExactCommandAndFourRoots(t *testing.T) {
	r, err := DecodeRequest([]byte(requestVector))
	if err != nil {
		t.Fatal(err)
	}
	generation := strings.Repeat("1", 64)
	actual, err := renderUnit([]byte(Template), r, generation)
	if err != nil {
		t.Fatal(err)
	}
	unit := string(actual)
	wanted := "ExecStart=:\"/opt/ardents-join/generations/" + generation + "/ardents-node-linux-amd64\" join start-installed \"/opt/ardents-join/binding.json\"\n"
	writes := "ReadWritePaths=\"/var/lib/ardents-node-state\" \"/var/lib/ardents-node-participation\" \"/var/lib/ardents-node-spend\" \"/var/lib/ardents-node-hosting\"\n"
	if !strings.Contains(unit, wanted) || !strings.Contains(unit, writes) || strings.Contains(unit, "@ARDENTS_") {
		t.Fatal("rendered fixed JOIN command/root binding differs", unit)
	}
	for _, bad := range []string{"", strings.Repeat("1", 63), strings.Repeat("G", 64)} {
		if _, err := renderUnit([]byte(Template), r, bad); err == nil {
			t.Fatal("invalid generation accepted")
		}
	}
	if _, err := renderUnit([]byte(Template+"ExecStartPre=/bin/true\n"), r, generation); err == nil {
		t.Fatal("unaccepted template gained a command")
	}
	r.InstallationRoot = "/opt/%n"
	if _, err := renderUnit([]byte(Template), r, generation); err == nil {
		t.Fatal("unit expansion in path admitted")
	}
}
