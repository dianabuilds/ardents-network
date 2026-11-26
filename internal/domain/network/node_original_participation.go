package network

// OriginalNodeRestriction projects authenticated assignment facts for one
// private original-invocation producer. Its live exclusion survives work expiry
// until physical join and producer release; the unchanged bound still denies
// work. Assignment-specific Restriction and retained legacy facts stay separate.
// The caller cannot select a restriction class or mint another fence record.
func (facts NodeParticipationFacts) OriginalNodeRestriction(phase string) (LocalRestriction, error) {
	restriction, err := facts.Restriction(phase)
	if err != nil {
		return LocalRestriction{}, err
	}
	restriction.Class = "node-duty"
	return restriction, nil
}
