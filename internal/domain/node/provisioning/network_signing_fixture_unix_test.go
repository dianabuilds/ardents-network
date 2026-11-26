//go:build linux || darwin

package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"testing"
	"time"
)

// Shared signing fixtures compose only portable preparation helpers, so the
// Unix-family signing codec tests and the Linux signing test reuse one oracle
// (#859). They supply no private material beyond the deterministic test seed.
// Their consumers are the Linux and macOS signing tests, so the fixture keeps
// the Unix-family boundary rather than becoming dead on other platforms.
func signingRecordFixture(t *testing.T) (map[string]any, ed25519.PrivateKey, []byte, time.Time) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	p := preparationRecordPlan(key.Public().(ed25519.PublicKey))
	_, message, err := networkdraft.Prepare("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	reference, _ := networkdraft.ParseTime("2033-05-22T05:30:00Z")
	return p, key, message, reference
}

func signingDeclaration(t *testing.T, purpose string, plan any, reference time.Time) []byte {
	t.Helper()
	field := purpose
	if purpose == "node-record" {
		field = "record"
	}
	return preparationJSON(t, map[string]any{"schema": "ardents-initial-" + purpose + "-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), field: plan})
}
