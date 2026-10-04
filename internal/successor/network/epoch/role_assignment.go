package epoch

func assignedDomain(epoch epochEnvelope, family string) (string, error) {
	domains := make([]string, len(epoch.domains))
	for index, domain := range epoch.domains {
		domains[index] = domain.id
	}
	return selectEpochDomain(epoch.networkID, epoch.number, epoch.assignmentSeed, family, domains)
}
