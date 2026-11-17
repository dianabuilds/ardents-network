//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The privileged pre-start check binds the exact launch declaration digest before
// any file, account, manager or Release observation. A wrong pin refuses before
// effects and yields no output; it never reaches the runtime or an accepting receipt.
func TestEndpointPlatformCheckRefusesWrongPinBeforeEffects(t *testing.T) {
	launch := endpointLaunchFixture()
	raw, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = executeEndpointPlatformCheck(t.Context(), []string{"check", "/etc/ardents/endpoint.json", strings.Repeat("0", 64)}, &out, &platformFiles{}, launch, raw)
	if err == nil || out.Len() != 0 {
		t.Fatalf("wrong pin reached effects: err=%v output=%q", err, out.String())
	}
}

// The platform unit derives from the enrolled fixed template by injecting exactly one
// privileged pre-start check bound to the original enrolled executable and this launch
// declaration digest. The transitional base keeps no ExecStartPre; the enrolled
// inventory is not widened.
func TestRenderEndpointPlatformUnitInjectsSolePrivilegedCheck(t *testing.T) {
	pin := strings.Repeat("a", 64)
	base := []byte("[Unit]\nDescription=Ardents protected text Endpoint\n\n[Service]\nExecStart=:\"/gen/ardents-linux-amd64\" endpoint start-installed \"/root\"\nRestart=no\n")
	unit, err := renderEndpointPlatformUnit(base, "/dist/ardents-linux-amd64", "/etc/ardents/endpoint.json", pin)
	if err != nil {
		t.Fatal(err)
	}
	want := "ExecStartPre=+:/dist/ardents-linux-amd64 platform check /etc/ardents/endpoint.json " + pin + "\n"
	if !bytes.Contains(unit, []byte(want)) {
		t.Fatalf("missing privileged pre-start check:\n%s", unit)
	}
	if bytes.Count(unit, []byte("ExecStartPre=")) != 1 || !bytes.Contains(unit, []byte("ExecStart=:")) {
		t.Fatalf("pre-start check not sole or ExecStart lost:\n%s", unit)
	}
	if bytes.Contains(base, []byte("ExecStartPre=")) {
		t.Fatal("base template mutated")
	}
	for name, change := range map[string]func(*[]byte, *string, *string, *string){
		"existing-pre": func(b *[]byte, c, m, p *string) {
			*b = bytes.Replace(*b, []byte("ExecStart="), []byte("ExecStartPre=:/x\nExecStart="), 1)
		},
		"no-execstart":    func(b *[]byte, c, m, p *string) { *b = []byte("[Service]\nRestart=no\n") },
		"bad-pin":         func(b *[]byte, c, m, p *string) { *p = strings.Repeat("z", 64) },
		"short-pin":       func(b *[]byte, c, m, p *string) { *p = "abc" },
		"unsafe-program":  func(b *[]byte, c, m, p *string) { *c = "/dist/ardents %i" },
		"unsafe-manifest": func(b *[]byte, c, m, p *string) { *m = "/etc/ardents/endpoint.json\nExecStart=/bin/sh" },
	} {
		t.Run(name, func(t *testing.T) {
			b := append([]byte(nil), base...)
			c, m, p := "/dist/ardents-linux-amd64", "/etc/ardents/endpoint.json", pin
			change(&b, &c, &m, &p)
			if _, err := renderEndpointPlatformUnit(b, c, m, p); err == nil {
				t.Fatal("unsafe platform unit rendering accepted")
			}
		})
	}
}
