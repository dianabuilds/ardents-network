package recoverysmoke

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCollectServiceIDsMapsEveryServiceAndRetainsEveryError(t *testing.T) {
	services := []string{"client", "publisher", "responder"}
	identities, err := collectServiceIDs(context.Background(), services,
		func(_ context.Context, service string) (string, error) { return "id-" + service, nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range services {
		if identities[service] != "id-"+service {
			t.Fatalf("identity for %s = %q", service, identities[service])
		}
	}
	clientErr, publisherErr := errors.New("client lookup"), errors.New("publisher lookup")
	_, err = collectServiceIDs(context.Background(), services, func(_ context.Context, service string) (string, error) {
		switch service {
		case "client":
			return "", clientErr
		case "publisher":
			return "", publisherErr
		default:
			return "id-" + service, nil
		}
	})
	if !errors.Is(err, clientErr) || !errors.Is(err, publisherErr) ||
		!strings.Contains(err.Error(), "client service identity") ||
		!strings.Contains(err.Error(), "publisher service identity") {
		t.Fatalf("joined identity errors are incomplete: %v", err)
	}
}

func TestCollectServiceIDsPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := collectServiceIDs(ctx, []string{"client", "publisher"},
		func(ctx context.Context, _ string) (string, error) { return "", ctx.Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause was lost: %v", err)
	}
}
