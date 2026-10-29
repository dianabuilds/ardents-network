//go:build installation_native

package completion

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func TestInstallationNativeForwardingACKRequiresWholeNewFrame(t *testing.T) {
	for _, mode := range []string{"fresh", "retained", "legacy", "unknown", "changed", "cancel-tail"} {
		t.Run(mode, func(t *testing.T) {
			directory, listener, request := completionFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			info, err := os.Lstat(directory)
			if err != nil {
				t.Fatal(err)
			}
			exchange, err := ConnectForwarding(ctx, directory, info, 65534, request)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = exchange.Close() })
			done := make(chan error, 1)
			prefixSent := make(chan struct{})
			cancelJoined := make(chan struct{})
			go func() {
				peer, err := listener.AcceptUnix()
				if err != nil {
					done <- err
					return
				}
				var actual [160]byte
				_, err = io.ReadFull(peer, actual[:])
				if err == nil && actual != request {
					err = ErrBinding
				}
				if err == nil {
					reply := append([]byte(nil), request[:]...)
					if mode != "legacy" && mode != "cancel-tail" {
						history := uint8(0)
						if mode == "retained" {
							history = 1
						} else if mode == "unknown" {
							history = 2
						}
						reply = append(reply, history)
					}
					if mode == "changed" {
						reply[0] = '2'
					}
					_, err = peer.Write(reply)
					close(prefixSent)
					if mode == "cancel-tail" {
						<-ctx.Done()
					}
				}
				done <- errors.Join(err, peer.Close())
			}()
			if mode == "cancel-tail" {
				go func() {
					defer close(cancelJoined)
					select {
					case <-prefixSent:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			history, waitErr := exchange.Wait()
			closeErr := exchange.Close()
			peerErr := <-done
			if mode == "cancel-tail" {
				<-cancelJoined
			}
			if peerErr != nil {
				t.Fatal("actual reply writer failed", peerErr)
			}
			switch mode {
			case "fresh", "retained":
				want := uint8(0)
				if mode == "retained" {
					want = 1
				}
				if waitErr != nil || closeErr != nil || history != want {
					t.Fatal("complete physical ACK unavailable", history, waitErr, closeErr)
				}
			case "legacy":
				if !errors.Is(waitErr, io.ErrUnexpectedEOF) {
					t.Fatal("legacy frame transferred history choice", history, waitErr)
				}
			case "cancel-tail":
				if !errors.Is(waitErr, context.Canceled) {
					t.Fatal("missing final byte concealed cancellation", history, waitErr)
				}
			default:
				if !errors.Is(waitErr, ErrBinding) {
					t.Fatal("foreign new frame accepted", history, waitErr)
				}
			}
		})
	}
}
