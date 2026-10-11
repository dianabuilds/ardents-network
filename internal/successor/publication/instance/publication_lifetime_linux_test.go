package instance

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
)

func TestNativePublicationLifetimeDeniesBeforeOriginalBindingDrain(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(context.Background(), config)
	if err != nil {
		root.Close()
		t.Fatal(err)
	}
	binding, err := root.Bind(context.Background(), history)
	if err != nil {
		root.Close()
		history.Close()
		t.Fatal(err)
	}
	lifetime, err := binding.BeginPublication(context.Background())
	if err != nil {
		root.Close()
		history.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { lifetime.Close(); binding.Close(); root.Close(); history.Close() })
	if _, err := binding.BeginPublication(context.Background()); err == nil {
		t.Fatal("second Publisher borrowed original private binding")
	}
	if err := binding.Consume(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := make(chan error, 1)
	go func() { joined <- binding.Close() }()
	select {
	case <-lifetime.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("binding did not synchronously deny original Publisher")
	}
	if _, err := lifetime.Credential(); err == nil {
		t.Fatal("withdrawn lifetime supplied authority")
	}
	root.mu.Lock()
	retained := len(binding.private) == 64
	root.mu.Unlock()
	if !retained {
		t.Fatal("original private key erased before Publisher joined")
	}
	select {
	case err := <-joined:
		t.Fatal("binding returned before borrower join", err)
	default:
	}
	lifetime.Close()
	select {
	case err := <-joined:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("binding did not join original borrower")
	}
	root.mu.Lock()
	retained = len(binding.private) != 0
	root.mu.Unlock()
	if retained {
		t.Fatal("joined binding retained private material")
	}
}
