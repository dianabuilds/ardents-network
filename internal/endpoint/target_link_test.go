package endpoint

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestEndpointTargetFromLinkBindsConfiguredNetwork(t *testing.T) {
	endpoint := &endpoint{network: targetLinkBytes(1)}
	link := targetlink.Link{Network: endpoint.network, Target: targetLinkBytes(33)}
	text, err := targetlink.Encode(link)
	if err != nil {
		t.Fatal(err)
	}
	got, err := endpoint.TargetFromLink(text)
	if err != nil {
		t.Fatal(err)
	}
	if got != link.Target {
		t.Fatalf("Target = %x, want %x", got, link.Target)
	}
}

func TestEndpointTargetFromLinkRejectsAnotherNetwork(t *testing.T) {
	endpoint := &endpoint{network: targetLinkBytes(1)}
	text, err := targetlink.Encode(targetlink.Link{Network: targetLinkBytes(2), Target: targetLinkBytes(33)})
	if err != nil {
		t.Fatal(err)
	}
	if target, err := endpoint.TargetFromLink(text); !errors.Is(err, ErrTargetLinkNetwork) || target != ([32]byte{}) {
		t.Fatalf("TargetFromLink = (%x, %v)", target, err)
	}
}

func targetLinkBytes(start byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = start + byte(index)
	}
	return result
}
