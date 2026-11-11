//go:build network_signing_native

package systemd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Actual original broker/manager replies only. This measures physical request
// setup, never Startup, process, State, Node or installed admission authority.
func BenchmarkForwardingManagerNativeRequestObservation(b *testing.B) {
	marker, err := os.ReadFile("/etc/forwarding-initial-component-marker")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-forwarding-coherent-preparation-v1\n" {
		b.Fatal("invalid environment: original stopped Forwarding fixture required", err)
	}
	control, err := OpenForwardingControl(b.Context())
	if err != nil {
		_ = control.Close()
		b.Fatal("original manager unavailable", err)
	}
	defer func() {
		if err := control.Close(); err != nil {
			b.Fatal("original reference close", err)
		}
	}()
	client := control.original.reference.client
	read := func(ctx context.Context) error {
		for range 45 {
			var owner string
			if err := client.owner(ctx, &owner); err != nil {
				return err
			}
			if owner != client.manager {
				return ErrObservation
			}
		}
		return client.transport.socket.SetDeadline(time.Time{})
	}
	for _, name := range []string{"per-request", "one-observation"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ctx, cancel := context.WithTimeout(b.Context(), 3*time.Second)
				var err error
				if name == "per-request" {
					err = read(ctx)
				} else {
					err = client.observeRequests(ctx, read)
				}
				canceled := ctx.Err()
				cancel()
				if err = errors.Join(err, canceled); err != nil {
					b.Fatal("fresh original reply failed", err)
				}
			}
		})
	}
}
