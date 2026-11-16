package endpoint

import (
	"encoding/hex"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

func TestReaderDeclarationDoesNotAcquirePublisherGrant(t *testing.T) {
	encode := func(n byte) string { key := [32]byte{n}; return hex.EncodeToString(key[:]) }
	plan := HeadlessPlan{Role: "reader", BrokerID: encode(1), ConnectionPrincipal: encode(2), AdministrationPrincipal: "not-an-identity"}
	config, err := installedLocalConfig(plan)
	if err != nil || len(config.Grants) != 1 || config.Grants[0].Surface != execution.Connection {
		t.Fatal(config, err)
	}
	plan.Role = ""
	if _, err := installedLocalConfig(plan); err == nil {
		t.Fatal("dual-role declaration ignored invalid Publisher identity")
	}
}

func TestNetworkDeclarationProjectsPinsWithoutOpeningPaths(t *testing.T) {
	encode := func(n byte) string { key := [32]byte{n}; return hex.EncodeToString(key[:]) }
	plan := HeadlessPlan{NetworkStateRoot: "/absent/state", TimeConfidenceFile: "/absent/clock", NetworkID: encode(1), ClosedProfileAuthority: encode(2), NetworkAuthorities: []string{encode(3)}, NetworkThreshold: 1}
	config, err := plan.Network()
	if err != nil || config.Root != plan.NetworkStateRoot || config.NetworkID != [32]byte{1} || len(config.Authorities) != 1 {
		t.Fatalf("projection failed: %+v, %v", config, err)
	}
	plan.NetworkAuthorities = append(plan.NetworkAuthorities, plan.NetworkAuthorities[0])
	if _, err := plan.Network(); err == nil {
		t.Fatal("duplicate authority pin accepted")
	}
	plan.NetworkAuthorities = plan.NetworkAuthorities[:1]
	plan.NetworkStateRoot = "/absent/../state"
	if _, err := plan.Network(); err == nil {
		t.Fatal("noncanonical declared path accepted")
	}
}
