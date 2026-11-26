package launch

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

func TestPlatformSourceCompletionInspectionUsesExactRetainedPlan(t *testing.T) {
	base := t.TempDir()
	plan := servingdeclaration.ServingDeclaration{
		Network: &servingdeclaration.ServingNetwork{
			Root: filepath.Join(base, "state"), ClockObservationFile: filepath.Join(base, "clock"),
			NetworkID: [32]byte{1}, ProfileAuthority: [32]byte{2},
			Threshold: 1, Authorities: [][32]byte{{3}},
		},
		ParticipationRoot: filepath.Join(base, "participation"),
		Certificate:       filepath.Join(base, "cert"), PrivateKey: filepath.Join(base, "key"),
		ClientRoot: filepath.Join(base, "client"), ProcessProfile: processguard.SourceProfile,
		ClientPins: [][32]byte{{4}}, Address: "127.0.0.1:4848",
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	config, err := sourceInspectionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if config.Root != plan.Network.Root || config.LocalRoleStateRoot != plan.ParticipationRoot || config.ClockObservationFile != plan.Network.ClockObservationFile || config.Source.ServeAddress != "" || config.Source.Sources[0].Address != "" || config.Source.Sources[1].Address != "" || config.AutomaticRefreshInterval != 0 {
		t.Fatal("inspection widened or changed runtime plan")
	}
	plan.ParticipationRoot = plan.Network.Root
	raw, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sourceInspectionConfig(raw); err == nil {
		t.Fatal("overlapping semantic histories accepted")
	}
}
