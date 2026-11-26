package connection

// Refuse returns a typed refusal for a server-side Interface implementation.
func Refuse(outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	return SetupRefusalError{outcome: outcome}
}
