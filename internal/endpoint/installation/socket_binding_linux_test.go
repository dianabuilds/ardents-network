//go:build linux

package installation

import (
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func TestInstalledSocketRefusesForeignOwnershipAndStopBinding(t *testing.T) {
	put := func(target worker.Properties, key, signature string, value any) {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		target[key] = worker.Value{Type: signature, Data: body}
	}
	for _, role := range []string{"reader", "publisher"} {
		t.Run(role, func(t *testing.T) {
			unit, socket := worker.Properties{}, worker.Properties{}
			name := "ardents-text-" + role + ".socket"
			for key, value := range map[string]string{"Id": name, "LoadState": "loaded", "ActiveState": "active", "SubState": "listening", "FragmentPath": "/etc/systemd/system/" + name} {
				put(unit, key, "s", value)
			}
			put(unit, "DropInPaths", "as", []string{})
			put(unit, "PartOf", "as", []string{"ardents-endpoint.service"})
			put(socket, "Accept", "b", true)
			put(socket, "RemoveOnStop", "b", true)
			put(socket, "SocketUser", "s", "ardents-endpoint")
			put(socket, "SocketGroup", "s", "ardents-endpoint")
			put(socket, "SocketMode", "u", uint32(0600))
			if err := verifyInstalledSocket(unit, socket, role); err != nil {
				t.Fatal(err)
			}
			for _, change := range []struct {
				key, signature string
				unit           bool
				value          any
			}{
				{"PartOf", "as", true, []string{}}, {"ActiveState", "s", true, "inactive"},
				{"DropInPaths", "as", true, []string{"/run/override.conf"}}, {"SocketUser", "s", false, "nobody"},
				{"SocketMode", "u", false, uint32(0644)}, {"Accept", "b", false, false}, {"RemoveOnStop", "b", false, false},
			} {
				changedUnit, changedSocket := cloneProperties(unit), cloneProperties(socket)
				target := changedSocket
				if change.unit {
					target = changedUnit
				}
				put(target, change.key, change.signature, change.value)
				if err := verifyInstalledSocket(changedUnit, changedSocket, role); err == nil {
					t.Fatalf("unsafe %s accepted", change.key)
				}
			}
		})
	}
}
