package recoverysmoke

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func TestCollectTrafficObserversStartsPairBeforeEitherCompletes(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	completed := make(chan trafficObservers, 1)
	go func() {
		result, err := collectTrafficObservers(context.Background(),
			map[string]string{"client": "client-route", "publisher": "publisher-route"},
			func(_ context.Context, route string) (string, recovery.ObserverProcess, error) {
				started <- route
				<-release
				return "observer-" + route, recovery.ObserverProcess{ContainerID: "observer-" + route}, nil
			})
		if err != nil {
			t.Errorf("collect pair: %v", err)
		}
		completed <- result
	}()
	seen := map[string]bool{}
	for range 2 {
		select {
		case route := <-started:
			seen[route] = true
		case <-time.After(time.Second):
			t.Fatal("traffic observers were not started concurrently")
		}
	}
	close(release)
	result := <-completed
	if !seen["client-route"] || !seen["publisher-route"] ||
		result.ids[0] != "observer-client-route" || result.ids[1] != "observer-publisher-route" {
		t.Fatalf("observer pair is incomplete: %#v, %#v", seen, result.ids)
	}
}

func TestCollectTrafficObserversRetainsBothRoleFailures(t *testing.T) {
	clientErr, publisherErr := errors.New("client start"), errors.New("publisher start")
	_, err := collectTrafficObservers(context.Background(),
		map[string]string{"client": "client-route", "publisher": "publisher-route"},
		func(_ context.Context, route string) (string, recovery.ObserverProcess, error) {
			if route == "client-route" {
				return "", recovery.ObserverProcess{}, clientErr
			}
			return "", recovery.ObserverProcess{}, publisherErr
		})
	if !errors.Is(err, clientErr) || !errors.Is(err, publisherErr) ||
		!strings.Contains(err.Error(), "client Route") || !strings.Contains(err.Error(), "publisher Route") {
		t.Fatalf("observer pair errors are incomplete: %v", err)
	}
}
