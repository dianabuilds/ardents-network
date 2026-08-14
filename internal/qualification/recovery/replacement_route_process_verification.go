package recovery

var replacementRouteProcessRoles = [...]string{"client", "publisher"}

func verifyReplacementRouteProcesses(cell replacementCell, scope hostScopeEvidence,
	identities map[string]bool) Result {
	if len(cell.HostProcesses) != len(replacementEndpointProcessRoles)+len(replacementRouteProcessRoles) {
		return invalid("S4.2 managed process evidence is incomplete")
	}
	for _, role := range replacementRouteProcessRoles {
		process, ok := cell.HostProcesses[role]
		if !ok || !validProcessObservation(process, scope) ||
			process.ObservedAtNanos < cell.HostStartedAtNanos ||
			process.ObservedAtNanos > cell.HostStartedAtNanos+cell.TerminalNanos ||
			identities[process.Host.Identity] {
			return invalid("S4.2 client or publisher Route process observation is invalid")
		}
		identities[process.Host.Identity] = true
	}
	return Result{Verdict: "pass"}
}
