package camouflage

import "testing"

func TestServerCapacityIsBoundToResourceProfile(t *testing.T) {
	for _, item := range []struct {
		profile               string
		wantSessions, wantRaw uint16
	}{{"h3-s-v1", 4, 32}, {"h3-s-v1-strong", 16, 128}} {
		got, err := serverCapacity(item.profile)
		if err != nil || got.sessions != item.wantSessions || got.rawSockets != item.wantRaw {
			t.Fatalf("capacity %s=%+v,%v want=%d/%d", item.profile, got, err, item.wantSessions, item.wantRaw)
		}
	}
	if _, err := serverCapacity("h3-np1-v1"); err == nil {
		t.Fatal("endpoint resource profile was accepted for a Bridge server")
	}
}
