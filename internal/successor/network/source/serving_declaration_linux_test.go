package source

import (
	"bytes"
	"strings"
	"testing"
)

func TestServingDeclarationClosedByteGrammar(t *testing.T) {
	// Independent literal oracle; no production encoder supplies expected bytes.
	key := "[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]"
	raw := `{"network":{"root":"/future/state","network_id":` + key + `,"authorities":[` + key + `],"threshold":1,"profile_authority":` + key + `,"clock_observation_file":"/input/clock"},"process_profile":"ardents-source-process-v1","participation_root":"/future/participation","address":"127.0.0.1:4567","certificate":"/future/credentials/certificate.pem","private_key":"/future/credentials/private-key.pem","client_root":"/future/credentials/client-ca.pem","client_pins":[` + key + `]}`
	plan, err := DecodeServingDeclaration([]byte(raw))
	if err != nil || plan.Network == nil || plan.Network.Root != "/future/state" || plan.Network.NetworkID != [32]byte{1} || plan.Address != "127.0.0.1:4567" || len(plan.ClientPins) != 1 {
		t.Fatal("independent complete declaration refused", plan, err)
	}
	for name, changed := range map[string]string{
		"case":              strings.Replace(raw, `"network"`, `"Network"`, 1),
		"nested-case":       strings.Replace(raw, `"threshold"`, `"Threshold"`, 1),
		"duplicate":         `{"address":"other",` + raw[1:],
		"nested-duplicate":  strings.Replace(raw, `"threshold":1`, `"threshold":1,"threshold":1`, 1),
		"null":              strings.Replace(raw, `"threshold":1`, `"threshold":null`, 1),
		"missing":           strings.Replace(raw, `"threshold":1,`, "", 1),
		"unknown":           `{"extra":0,` + raw[1:],
		"trailing":          raw + " {}",
		"short-key":         strings.Replace(raw, key, "[1]", 1),
		"long-key":          strings.Replace(raw, key, strings.TrimSuffix(key, "]")+",0]", 1),
		"encoded-key":       strings.Replace(raw, key, `"AQ=="`, 1),
		"null-pins":         strings.Replace(raw, `"client_pins":[`+key+`]`, `"client_pins":null`, 1),
		"empty-authorities": strings.Replace(raw, `"authorities":[`+key+`]`, `"authorities":[]`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeServingDeclaration([]byte(changed)); err == nil {
				t.Fatal("invalid declaration accepted")
			}
		})
	}
	if _, err := DecodeServingDeclaration(bytes.Repeat([]byte(" "), 16<<10+1)); err == nil {
		t.Fatal("unbounded input accepted")
	}
	// Numeric policy belongs to State/Installation, not the shared byte codec.
	if _, err := DecodeServingDeclaration([]byte(strings.Replace(raw, `"threshold":1`, `"threshold":0`, 1))); err != nil {
		t.Fatal("codec acquired runtime policy", err)
	}
}
