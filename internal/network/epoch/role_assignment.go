package epoch

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
)

func verifyDomainSummaries(epoch epochEnvelope, families map[string][2]uint32) error {
	computed := make(map[string][2]uint32, len(epoch.domains))
	for _, domain := range epoch.domains {
		computed[domain.id] = [2]uint32{}
	}
	for family, summary := range families {
		domain, err := assignedDomain(epoch, family)
		if err != nil {
			return err
		}
		current := computed[domain]
		current[0] += summary[0]
		current[1] += summary[1]
		computed[domain] = current
	}
	for _, expected := range epoch.domains {
		actual := computed[expected.id]
		if uint16(actual[0]) != expected.count || actual[1] != expected.capacity {
			return errors.New("role domain summaries are inconsistent")
		}
	}
	return nil
}

func assignedDomain(epoch epochEnvelope, family string) (string, error) {
	domains := make([]string, len(epoch.domains))
	for index, domain := range epoch.domains {
		domains[index] = domain.id
	}
	return assignment.Select(epoch.networkID, epoch.number, epoch.assignmentSeed, family, domains)
}
