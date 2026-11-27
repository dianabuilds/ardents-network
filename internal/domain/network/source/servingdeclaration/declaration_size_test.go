package servingdeclaration

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIndependentValidOversizedDeclarationRefused(t *testing.T) {
	p := ServingDeclaration{Network: &ServingNetwork{Root: "/state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: "/clock"}, ProcessProfile: "ardents-source-process-v1", ParticipationRoot: "/roles", Address: "127.0.0.1:1", Certificate: "/cert", PrivateKey: "/key", ClientRoot: "/ca", ClientPins: [][32]byte{{3}}}
	body, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	p.ParticipationRoot += strings.Repeat("x", (16<<10)-len(body))
	limit, e := json.Marshal(p)
	if e != nil || len(limit) != 16<<10 {
		t.Fatal("invalid exact limit fixture", e, len(limit))
	}
	if _, e := DecodeServingDeclaration(limit); e != nil {
		t.Fatal("well-formed at-limit object refused", e)
	}
	p.ParticipationRoot += "x"
	oversized, e := json.Marshal(p)
	if e != nil || len(oversized) != (16<<10)+1 {
		t.Fatal("invalid oversized fixture", e, len(oversized))
	}
	if _, e := DecodeServingDeclaration(oversized); e == nil {
		t.Fatal("accepted complete well-shaped JSON one byte above 16KiB")
	}
}
