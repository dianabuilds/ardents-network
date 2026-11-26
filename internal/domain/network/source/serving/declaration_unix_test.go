//go:build linux || darwin

package serving

import (
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func TestServingDeclarationSeparatesStateFromParticipation(t *testing.T) {
	root := t.TempDir()
	plan := servingdeclaration.ServingDeclaration{
		Network: &servingdeclaration.ServingNetwork{
			Root: filepath.Join(root, "state"), ClockObservationFile: filepath.Join(root, "clock"),
			NetworkID: [32]byte{1}, ProfileAuthority: [32]byte{2},
			Threshold: 1, Authorities: [][32]byte{{3}},
		},
		ParticipationRoot: filepath.Join(root, "participation"),
		Certificate:       filepath.Join(root, "cert"), PrivateKey: filepath.Join(root, "key"),
		ClientRoot: filepath.Join(root, "client"), ProcessProfile: processresource.SourceProfile,
		ClientPins: [][32]byte{{4}}, Address: "127.0.0.1:4848",
	}
	if !ValidDeclaration(plan) {
		t.Fatal("independent roots and bounded authority refused")
	}
	plan.Network.Threshold++
	if ValidDeclaration(plan) {
		t.Fatal("threshold greater than authority inventory admitted")
	}
	plan.Network.Threshold--
	plan.ParticipationRoot = filepath.Join(plan.Network.Root, "nested")
	if ValidDeclaration(plan) {
		t.Fatal("participation nested inside State admitted")
	}
}
