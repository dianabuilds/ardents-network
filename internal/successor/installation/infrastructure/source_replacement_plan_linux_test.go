package infrastructure

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Detached declarations isolate the allowed configuration delta only; these
// values supply no State, trust, TLS, Release or native effect permission.
func TestSourceForwardPlanAllowsListenerChangeAndRetainsAuthority(t *testing.T) {
	plan := map[string]any{"network": map[string]any{"root": "/var/lib/state", "network_id": [32]byte{1}, "authorities": [][32]byte{{2}}, "threshold": 1, "profile_authority": [32]byte{3}, "clock_observation_file": "/var/lib/clock"},
		"process_profile": "ardents-source-process-v1", "participation_root": "/var/lib/participation", "address": "127.0.0.1:4567", "certificate": "/var/lib/credentials/certificate.pem", "private_key": "/var/lib/credentials/private-key.pem", "client_root": "/var/lib/credentials/client-ca.pem", "client_pins": [][32]byte{{4}}}
	old, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	next := bytes.Replace(old, []byte("127.0.0.1:4567"), []byte("127.0.0.1:4568"), 1)
	if err := checkSourceForwardPlan(old, next); err != nil {
		t.Fatal("permitted actual listener change", err)
	}
	for name, change := range map[string][2]string{
		"State-history":         {"/var/lib/state", "/var/lib/new-state"},
		"participation-history": {"/var/lib/participation", "/var/lib/new-participation"},
		"clock":                 {"/var/lib/clock", "/var/lib/new-clock"},
		"profile":               {"ardents-source-process-v1", "other-profile"},
		"key":                   {"private-key.pem", "replacement-key.pem"},
		"CA":                    {"client-ca.pem", "replacement-ca.pem"},
		"threshold":             {`"threshold":1`, `"threshold":2`},
	} {
		t.Run(name, func(t *testing.T) {
			altered := bytes.Replace(next, []byte(change[0]), []byte(change[1]), 1)
			if bytes.Equal(altered, next) {
				t.Fatal("control did not alter declaration")
			}
			if err := checkSourceForwardPlan(old, altered); err == nil {
				t.Fatal("changed authority/history/protection admitted")
			}
		})
	}
}
