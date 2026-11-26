package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	provisionfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestPublicNetworkPreparationPortable(t *testing.T) {
	recordKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	defer clear(recordKey)
	recordPlan := provisionfixture.RecordPlan(recordKey.Public().(ed25519.PublicKey))
	name, unsigned, err := networkdraft.Prepare("prepare-initial-node-record", provisionfixture.PlanJSON(t, recordPlan))
	if err != nil || name != "node-record.signing-input" || len(unsigned) == 0 {
		t.Fatalf("portable Record preparation failed: %q: %v", name, err)
	}
	epochPlan, epochKey, _, signedEpoch, _ := provisionfixture.EpochPlan(t)
	defer clear(epochKey)
	name, unsigned, err = networkdraft.Prepare("prepare-initial-epoch", provisionfixture.PlanJSON(t, epochPlan))
	if err != nil || name != "epoch.unsigned" || len(unsigned) >= len(signedEpoch.Raw) ||
		!bytes.Equal(unsigned, signedEpoch.Raw[:len(unsigned)]) {
		t.Fatalf("portable Epoch does not match the signed fixture prefix: %q: %v", name, err)
	}
	profilePlan, profileKey, body, _ := provisionfixture.ProfilePlan(t, time.Unix(1800003600, 0).UTC().Add(time.Minute))
	defer clear(profileKey)
	name, unsigned, err = networkdraft.Prepare("prepare-closed-profile", provisionfixture.PlanJSON(t, profilePlan))
	if err != nil || name != "profile.unsigned" || !bytes.Equal(unsigned, body) {
		t.Fatalf("portable profile differs from the independent body: %q: %v", name, err)
	}
}
