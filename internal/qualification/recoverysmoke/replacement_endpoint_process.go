package recoverysmoke

import (
	"context"
	"fmt"
)

var replacementEndpointProcessRoles = [...]string{
	"client-endpoint", "publisher-endpoint", "client-app", "publisher-app",
}

func observeReplacementEndpointProcesses(ctx context.Context, observer hostProcessAdapter,
	identities map[string]string) (map[string]processObservationEvidence, error) {
	result := make(map[string]processObservationEvidence, len(replacementEndpointProcessRoles))
	for _, role := range replacementEndpointProcessRoles {
		identity, ok := identities[role]
		if !ok {
			return nil, fmt.Errorf("resolve %s process: service identity is missing", role)
		}
		observed, err := observeProcessEvidence(ctx, observer, role, role)
		if err != nil {
			return nil, fmt.Errorf("resolve %s process: %w", role, err)
		}
		if observed.Host.Identity != identity {
			return nil, fmt.Errorf("resolve %s process: adapter identity changed", role)
		}
		result[role] = observed
	}
	return result, nil
}
