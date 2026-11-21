//go:build linux

package launch

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// platformCheckBinding projects the sole privileged F-25 pre-start argv from the
// loaded ExecStartPreEx. These observations exercise the projection grammar only;
// they supply no native manager, launch declaration or running process proof.
func platformBindingService(t *testing.T, entry []any) managerProperties {
	t.Helper()
	raw, err := json.Marshal([][]any{entry})
	if err != nil {
		t.Fatal(err)
	}
	return managerProperties{"ExecStartPreEx": systemd.Value{Type: "a(sasasttttuii)", Data: raw}}
}

func platformBindingEntry() []any {
	program := "/distribution/ardents-linux-amd64"
	manifest := "/etc/ardents/endpoint.json"
	pin := strings.Repeat("ab", 32)
	argv := []string{program, "platform", "check", manifest, pin}
	return []any{program, argv, []string{"privileged", "no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}
}

func TestPlatformCheckBindingProjectsSolePrivilegedArgv(t *testing.T) {
	service := platformBindingService(t, platformBindingEntry())
	program, manifest, pin, err := platformCheckBinding(service)
	if err != nil {
		t.Fatal(err)
	}
	if program != "/distribution/ardents-linux-amd64" || manifest != "/etc/ardents/endpoint.json" || pin != strings.Repeat("ab", 32) {
		t.Fatalf("projected binding differs: %q %q %q", program, manifest, pin)
	}
}

func TestPlatformCheckBindingRefusesForeignOrMalformedPreStart(t *testing.T) {
	for name, change := range map[string]func(managerProperties){
		"missing": func(s managerProperties) { delete(s, "ExecStartPreEx") },
		"wrong-type": func(s managerProperties) {
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasbttttuii)", Data: json.RawMessage(`[]`)}
		},
		"empty": func(s managerProperties) {
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: json.RawMessage(`[]`)}
		},
		"two-entries": func(s managerProperties) {
			raw, _ := json.Marshal([][]any{platformBindingEntry(), platformBindingEntry()})
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: raw}
		},
		"short-entry": func(s managerProperties) {
			raw, _ := json.Marshal([][]any{{"/distribution/ardents-linux-amd64"}})
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: raw}
		},
		"short-argv": func(s managerProperties) {
			entry := platformBindingEntry()
			entry[1] = []string{"/distribution/ardents-linux-amd64", "platform", "check"}
			raw, _ := json.Marshal([][]any{entry})
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: raw}
		},
		"foreign-verb": func(s managerProperties) {
			entry := platformBindingEntry()
			entry[1] = []string{"/distribution/ardents-linux-amd64", "platform", "start", "/etc/ardents/endpoint.json", strings.Repeat("ab", 32)}
			raw, _ := json.Marshal([][]any{entry})
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: raw}
		},
		"program-mismatch": func(s managerProperties) {
			entry := platformBindingEntry()
			entry[0] = "/foreign/ardents-linux-amd64"
			raw, _ := json.Marshal([][]any{entry})
			s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: raw}
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := platformBindingService(t, platformBindingEntry())
			change(service)
			if _, _, _, err := platformCheckBinding(service); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign or malformed pre-start binding accepted", name, err)
			}
		})
	}
}
