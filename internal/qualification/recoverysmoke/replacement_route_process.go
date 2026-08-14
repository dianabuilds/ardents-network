package recoverysmoke

import (
	"context"
	"fmt"
)

var replacementRouteProcessRoles = [...]string{"client", "publisher"}

func observeReplacementRouteProcesses(ctx context.Context, observer hostProcessAdapter,
	identities map[string]string) (map[string]processObservationEvidence, error) {
	result := make(map[string]processObservationEvidence, len(replacementRouteProcessRoles))
	for _, role := range replacementRouteProcessRoles {
		identity, ok := identities[role]
		if !ok {
			return nil, fmt.Errorf("resolve %s Route process: service identity is missing", role)
		}
		observed, err := observeProcessEvidence(ctx, observer, role, role)
		if err != nil {
			return nil, fmt.Errorf("resolve %s Route process: %w", role, err)
		}
		if observed.Host.Identity != identity {
			return nil, fmt.Errorf("resolve %s Route process: adapter identity changed", role)
		}
		result[role] = observed
	}
	return result, nil
}
