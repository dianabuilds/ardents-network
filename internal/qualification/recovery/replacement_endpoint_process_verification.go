package recovery

var replacementEndpointProcessRoles = [...]string{
	"client-endpoint", "publisher-endpoint", "client-app", "publisher-app",
}

func verifyReplacementEndpointProcesses(cell replacementCell, scope hostScopeEvidence,
	identities map[string]bool) Result {
	for _, role := range replacementEndpointProcessRoles {
		process, ok := cell.HostProcesses[role]
		if !ok || !validProcessObservation(process, scope) ||
			process.ObservedAtNanos < cell.HostStartedAtNanos ||
			process.ObservedAtNanos > cell.HostStartedAtNanos+cell.TerminalNanos ||
			identities[process.Host.Identity] {
			return invalid("S4.2 Endpoint or Application process observation is invalid")
		}
		identities[process.Host.Identity] = true
	}
	return Result{Verdict: "pass"}
}
