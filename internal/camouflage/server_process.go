package camouflage

import (
	"context"
	"time"
)

func startServerProcess(ctx context.Context, binary, stateRoot, bind, nextLeg, publicURL string,
	deadline, parent time.Time,
) (*candidateChild, error) {
	environment := []string{
		"TOR_PT_MANAGED_TRANSPORT_VER=1",
		"TOR_PT_SERVER_TRANSPORTS=webtunnel",
		"TOR_PT_SERVER_BINDADDR=webtunnel-" + bind,
		"TOR_PT_SERVER_TRANSPORT_OPTIONS=webtunnel:url=" + publicURL,
		"TOR_PT_ORPORT=" + nextLeg,
		"TOR_PT_STATE_LOCATION=" + stateRoot,
		"TOR_PT_EXIT_ON_STDIN_CLOSE=1",
	}
	child, _, err := startCandidateProcess(ctx, binary, environment, deadline, parent, readServerReadiness)
	return child, err
}
