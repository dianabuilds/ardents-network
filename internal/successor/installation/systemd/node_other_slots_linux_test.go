package systemd

import "testing"

func TestOtherNodeSlotsRetainExactRoleRoster(t *testing.T) {
	all := []string{"forwarding", "issuer", "join", "descriptor", "introduction"}
	for _, purpose := range []fixedUnitPurpose{joinControlPurpose, descriptorControlPurpose, introductionControlPurpose} {
		names := otherNodeSlotNames(purpose)
		if len(names) != 9 {
			t.Fatal("wrong absent-slot count", purpose, names)
		}
		seen := make(map[string]bool)
		for _, name := range names {
			if seen[name] {
				t.Fatal("duplicate absence request", name)
			}
			seen[name] = true
		}
		for _, role := range all {
			unit, socket := "ardents-"+role+".service", "ardents-"+role+".socket"
			if !seen[socket] || seen[unit] != (unit != purpose.unit()) {
				t.Fatal("incorrect fixed absence roster", purpose, role, names)
			}
		}
	}
	if len(otherNodeSlotNames(sourceControlPurpose)) != 0 || len(otherNodeSlotNames(forwardingControlPurpose)) != 0 {
		t.Fatal("unadmitted role gained this observation")
	}
}
