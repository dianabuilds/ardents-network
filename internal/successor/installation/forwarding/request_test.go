package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const forwardingRequestVector = "{\"schema\":\"ardents-forwarding-installation-v1\",\"invocation\":\"forwarding\",\"profile\":\"ardents-forwarding-process-v1\",\"installation_root\":\"/opt/ardents-forwarding\",\"state_root\":\"/var/lib/ardents-node-state\",\"participation_root\":\"/var/lib/ardents-node-participation\",\"spend_root\":\"/var/lib/ardents-node-spend\",\"hosting_root\":\"/var/lib/ardents-node-hosting\",\"credential_directory\":\"/etc/ardents-node\",\"plan\":\"/var/lib/ardents-delivery/forwarding.json\",\"certificate\":\"/var/lib/ardents-delivery/certificate.pem\",\"private_key\":\"/var/lib/ardents-delivery/private-key.pem\",\"clock\":\"/var/lib/ardents-clock/observation\"}\n"

func TestForwardingRequestRequiresEveryCompletionSocketToFit(t *testing.T) {
	r, err := DecodeRequest([]byte(forwardingRequestVector))
	if err != nil {
		t.Fatal(err)
	}
	// Independent native limit and longest declared restart name, including the
	// trailing NUL. Initial-only validation missed the later retirement socket.
	suffix := "/forwarding-invocation-00000063/start-completion.socket"
	for _, length := range []int{107, 108} {
		r.InstallationRoot = "/opt/" + strings.Repeat("x", length-len(suffix)-len("/opt/"))
		raw, _ := json.Marshal(r)
		_, err := DecodeRequest(append(raw, '\n'))
		if (err == nil) != (length == 107) {
			t.Fatal("completion path capacity was not admitted before effects", length, err)
		}
	}
	r.InstallationRoot = "/var/lib/forwarding-coherent-initial/runtime/installation"
	raw, _ := json.Marshal(r)
	if _, err := DecodeRequest(append(raw, '\n')); err == nil {
		t.Fatal("actual installed225 root with oversized retirement socket was accepted")
	}
}

func TestForwardingRequestExactInventoryAndNamespaces(t *testing.T) {
	r, err := DecodeRequest([]byte(forwardingRequestVector))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil || !bytes.Equal(append(raw, '\n'), []byte(forwardingRequestVector)) {
		t.Fatal("canonical independent request", err)
	}
	for _, mutation := range []struct{ before, after string }{
		{"\"forwarding\"", "\"source\""}, {"ardents-forwarding-process-v1", "ardents-source-process-v1"},
		{"\"profile\":", "\"profile\":\"ardents-forwarding-process-v1\",\"profile\":"},
		{"\"plan\":", "\"unknown\":0,\"plan\":"},
		{"/var/lib/ardents-node-spend", "/var/lib/ardents-node-state/spend"},
		{"/var/lib/ardents-node-hosting", "/var/lib/ardents-node-state"},
		{"/opt/ardents-forwarding", "/opt/../ardents-forwarding"},
		{"/etc/ardents-node", "/etc/ardents%node"},
		{"/var/lib/ardents-clock/observation", "/var/lib/ardents-node-hosting/clock"},
	} {
		if _, err := DecodeRequest([]byte(strings.Replace(forwardingRequestVector, mutation.before, mutation.after, 1))); err == nil {
			t.Fatal("foreign/overlapping request accepted", mutation)
		}
	}
	for _, hidden := range []string{"/tmp", "/var/tmp", "/home", "/root", "/run/user"} {
		for _, field := range []string{"installation_root", "state_root", "participation_root", "spend_root", "hosting_root", "credential_directory", "clock"} {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(forwardingRequestVector), &fields); err != nil {
				t.Fatal(err)
			}
			// Preserve canonical field order through the typed value after changing
			// exactly one declaration, so namespace denial is the causal oracle.
			fields[field], _ = json.Marshal(hidden + "/runtime-input")
			changed, _ := json.Marshal(fields)
			var candidate Request
			if err := json.Unmarshal(changed, &candidate); err != nil {
				t.Fatal(err)
			}
			changed, _ = json.Marshal(candidate)
			if _, err := DecodeRequest(append(changed, '\n')); err == nil {
				t.Fatal("fixed namespace hides runtime input", field, hidden)
			}
		}
	}
	for _, raw := range [][]byte{nil, bytes.TrimSpace([]byte(forwardingRequestVector)), []byte(forwardingRequestVector + "\n"), bytes.Repeat([]byte("x"), (16<<10)+1)} {
		if _, err := DecodeRequest(raw); err == nil {
			t.Fatal("noncanonical request accepted")
		}
	}
}

func TestForwardingRequestReservesEveryFiniteJournalBeforeEffects(t *testing.T) {
	original, err := DecodeRequest([]byte(forwardingRequestVector))
	if err != nil {
		t.Fatal(err)
	}
	// Independent names include both ends of the retained inventory. Inspecting
	// every number also prevents an off-by-one from admitting an intermediate
	// attempt as a live clock, delivery input or semantic owner.
	reserved := []string{original.InstallationRoot + ".forwarding-initial-journal", original.InstallationRoot + ".forwarding-initial-pending", original.InstallationRoot + ".forwarding-first-start-journal"}
	reserved = append(reserved, original.InstallationRoot+".forwarding-hosting", original.InstallationRoot+".forwarding-hosting-journal", original.InstallationRoot+".forwarding-hosting-pending")
	for n := 1; n < 64; n++ {
		reserved = append(reserved, original.InstallationRoot+fmt.Sprintf(".forwarding-invocation-%08d-journal", n))
	}
	for n := 0; n < 64; n++ {
		reserved = append(reserved, original.InstallationRoot+fmt.Sprintf(".forwarding-retirement-%08d-journal", n))
	}
	for _, path := range reserved {
		for _, nested := range []string{path, path + "/leaf"} {
			for _, mutate := range []func(*Request){
				func(r *Request) { r.StateRoot = nested }, func(r *Request) { r.ParticipationRoot = nested },
				func(r *Request) { r.SpendRoot = nested }, func(r *Request) { r.HostingRoot = nested },
				func(r *Request) { r.CredentialDirectory = nested }, func(r *Request) { r.Plan = nested },
				func(r *Request) { r.Certificate = nested }, func(r *Request) { r.PrivateKey = nested },
				func(r *Request) { r.Clock = nested },
			} {
				candidate := original
				mutate(&candidate)
				raw, err := json.Marshal(candidate)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRequest(append(raw, '\n')); !errors.Is(err, ErrBinding) {
					t.Fatal("journal namespace collision did not refuse before effects", nested, err)
				}
			}
		}
	}
}
