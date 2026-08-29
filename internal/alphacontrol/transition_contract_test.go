package alphacontrol

import "testing"

func TestH46BTransitionContractsKeepAuthoritiesSeparate(t *testing.T) {
	contracts := H46BTransitionContracts()
	if len(contracts) != 4 {
		t.Fatalf("transition contract count = %d, want 4", len(contracts))
	}
	want := []struct {
		domain   TransitionDomain
		selected bool
	}{
		{DomainReleaseSafety, true},
		{DomainNetworkEpoch, true},
		{DomainCompatibility, true},
		{DomainNamespaceMaterialization, false},
	}
	for index, expected := range want {
		contract := contracts[index]
		if contract.Domain != expected.domain || contract.Selected != expected.selected {
			t.Fatalf("contract %d = %+v, want domain %q selected=%t", index, contract, expected.domain, expected.selected)
		}
		for field, value := range map[string]string{
			"authority root":   contract.AuthorityRoot,
			"predecessor":      contract.Predecessor,
			"freshness":        contract.Freshness,
			"rotation":         contract.Rotation,
			"revocation":       contract.Revocation,
			"rollback floor":   contract.RollbackFloor,
			"emergency action": contract.EmergencyAction,
			"user failure":     contract.UserFailure,
			"evidence":         contract.Evidence,
		} {
			if value == "" {
				t.Fatalf("%s %s is absent", expected.domain, field)
			}
		}
	}
	if contracts[0].AuthorityRoot == contracts[1].AuthorityRoot || contracts[0].AuthorityRoot == contracts[2].AuthorityRoot || contracts[1].AuthorityRoot == contracts[2].AuthorityRoot {
		t.Fatalf("selected transition contracts collapse authority roots: %+v", contracts)
	}
	if contracts[3].EmergencyAction != "do not materialize, release, or reclaim a Namespace" {
		t.Fatalf("Namespace emergency action = %q", contracts[3].EmergencyAction)
	}
}
