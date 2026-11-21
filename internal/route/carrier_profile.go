package route

// CarrierPermitted reports the executable Carrier of the selected closed release.
// Recognizing a historical signed profile does not grant transport admission.
// This predicate never changes a profile or selects a fallback.
func CarrierPermitted(profile string) bool {
	return profile == "ardents-carrier-tcp-tls-v2"
}
