//go:build linux || darwin

package networksigning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	provisionfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNetworkSigningIndependentProfileAndAuthority(t *testing.T) {
	now := time.Unix(1800003600, 0).UTC().Add(time.Minute)
	p, key, body, want := provisionfixture.ProfilePlan(t, now)
	defer clear(key)
	raw := provisionfixture.ProfileSigningDeclaration(t, p, key.Public().(ed25519.PublicKey), now)
	digest := sha256.Sum256(body)
	pin := hex.EncodeToString(digest[:])
	admit := func(raw, body []byte, pin string) *networkSigning {
		t.Helper()
		s, e := admitNetworkSigning("sign-closed-profile", raw, body, pin)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s := admit(raw, body, pin)
	if _, e := admitNetworkSigning("sign-closed-profile", provisionfixture.ProfileSigningDeclaration(t, p, ed25519.PublicKey(bytes.Repeat([]byte{0}, 32)), now), body, pin); e == nil {
		t.Fatal("zero State pin accepted")
	}
	permission := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{83}, 32))
	if _, e := admitNetworkSigning("sign-closed-profile", provisionfixture.ProfileSigningDeclaration(t, p, permission.Public().(ed25519.PublicKey), now), body, pin); e == nil {
		t.Fatal("Admission authority aliased as State signer")
	}
	clear(permission)
	outputs, err := s.sign(key, now)
	if err != nil || len(outputs) != 1 || outputs[0].name != "profile.bin" || !bytes.Equal(outputs[0].raw, want) {
		t.Fatal("independent profile vector differs", err)
	}
	for _, fault := range []string{"prepared", "pin", "duplicate", "unknown", "mixed", "trailing", "wrong purpose", "missing authority", "zero authority", "authority shape", "fraction", "foreign context", "bad spki", "duplicate node", "long interval"} {
		t.Run(fault, func(t *testing.T) {
			r, b, pin := bytes.Clone(raw), bytes.Clone(body), pin
			var q map[string]any
			_ = json.Unmarshal(r, &q)
			switch fault {
			case "prepared":
				b[0] ^= 1
			case "pin":
				pin = provisionfixture.PlanIdentifier(99)
			case "duplicate":
				r = bytes.Replace(r, []byte(`"schema":`), []byte(`"schema":"x","schema":`), 1)
			case "unknown":
				q["private_key"] = true
				r = provisionfixture.PlanJSON(t, q)
			case "mixed":
				q["epoch"] = p
				r = provisionfixture.PlanJSON(t, q)
			case "trailing":
				r = append(r, []byte(" null")...)
			case "wrong purpose":
				q["schema"] = "ardents-initial-epoch-signing-plan-v1"
				r = provisionfixture.PlanJSON(t, q)
			case "missing authority":
				delete(q, "state_authority_key")
				r = provisionfixture.PlanJSON(t, q)
			case "zero authority":
				q["state_authority_key"] = provisionfixture.PlanIdentifier(0)
				r = provisionfixture.PlanJSON(t, q)
			case "authority shape":
				q["state_authority_key"] = []int{1}
				r = provisionfixture.PlanJSON(t, q)
			case "fraction":
				q["reference_time"] = now.Add(time.Nanosecond).Format(time.RFC3339Nano)
				r = provisionfixture.PlanJSON(t, q)
			default:
				profile := q["profile"].(map[string]any)
				switch fault {
				case "foreign context":
					profile["network_id"] = provisionfixture.PlanIdentifier(99)
				case "bad spki":
					profile["token_keys"].([]any)[0].(map[string]any)["spki"] = "00"
				case "duplicate node":
					n := profile["nodes"].([]any)[0]
					profile["nodes"] = []any{n, n}
				case "long interval":
					profile["not_after"] = now.Add(7 * time.Hour).Format(time.RFC3339)
				}
				r = provisionfixture.PlanJSON(t, q)
			}
			if _, e := admitNetworkSigning("sign-closed-profile", r, b, pin); e == nil {
				t.Fatal("invalid profile binding accepted")
			}
			if _, e := admit(raw, body, hex.EncodeToString(digest[:])).sign(key, now); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
	for _, n := range []byte{82, 83, 84} {
		foreign := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{n}, 32))
		if _, e := s.sign(foreign, now); e == nil {
			t.Fatal("Node/Admission/foreign private key accepted")
		}
		if n != 83 {
			wrong := admit(provisionfixture.ProfileSigningDeclaration(t, p, foreign.Public().(ed25519.PublicKey), now), body, pin)
			if _, e := wrong.sign(key, now); e == nil {
				t.Fatal("independent State pin ignored")
			}
		}
		clear(foreign)
	}
	for _, at := range []time.Time{now.Add(-time.Minute - time.Nanosecond), now.Add(5*time.Minute + time.Nanosecond), s.until} {
		if _, e := s.sign(key, at); e == nil {
			t.Fatal("expired original time accepted")
		}
	}
}
