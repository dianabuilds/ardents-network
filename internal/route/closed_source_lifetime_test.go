//go:build linux

package route

import (
	"net"
	"testing"
	"testing/synctest"
	"time"
)

func TestClosedSourceChannelsRetiresAfterExactIdleWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		defer peer.Close()
		owner := newClosedSourceChannelOwner(local, time.Now().Add(10*time.Minute), local.Close)
		owner.start()
		defer owner.Close()
		time.Sleep(119 * time.Second)
		select {
		case <-owner.done:
			t.Fatal("source prefix expired before idle bound")
		default:
		}
		time.Sleep(time.Second)
		synctest.Wait()
		select {
		case <-owner.done:
		default:
			t.Fatal("idle source retained parent workers")
		}
		if _, err := owner.open(t.Context(), sourceIssuerOpen(time.Now().UTC().Add(time.Minute)), time.Now().Add(10*time.Second)); err == nil {
			t.Fatal("expired source accepted new work")
		}
	})
}

func TestClosedSourceChannelsIdleWindowStartsAfterAdmittedChildEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		defer peer.Close()
		end := time.Now().UTC().Add(10 * time.Minute)
		owner := newClosedSourceChannelOwner(local, end, local.Close)
		owner.start()
		defer owner.Close()
		frames := make(chan error, 1)
		go func() {
			for range 2 {
				if _, err := ReadClosedLaneFrame(peer); err != nil {
					frames <- err
					return
				}
			}
			frames <- nil
		}()
		lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
		if err != nil {
			t.Fatal(err)
		}
		// This state-machine fixture models actual successful receiving ADMIT.
		// Endpoint network tests exercise the authentic TLS/admission producer.
		if err := lane.activate(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Minute)
		select {
		case <-owner.done:
			t.Fatal("live child counted as idle")
		default:
		}
		if err := lane.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-frames; err != nil {
			t.Fatal(err)
		}
		time.Sleep(119 * time.Second)
		select {
		case <-owner.done:
			t.Fatal("source did not retain post-work idle window")
		default:
		}
		time.Sleep(time.Second)
		synctest.Wait()
		select {
		case <-owner.done:
		default:
			t.Fatal("post-work idle source retained workers")
		}
	})
}

func TestClosedSourcePrefixAutonomousRetirementJoinsNestedReader(t *testing.T) {
	for _, parentLifetime := range []time.Duration{time.Minute, 10 * time.Minute} {
		t.Run(parentLifetime.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				local, peer := net.Pipe()
				defer peer.Close()
				end := time.Now().UTC().Add(parentLifetime)
				retirement := &closedRoleRetirement{transport: local}
				child := newClosedRoleChildStream(local, end, retirement.close, nil)
				prefix := &ClosedSourcePrefix{connection: child, child: child, retirement: retirement,
					stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
				prefix.channels = newClosedSourceChannelOwner(child, end, retirement.close)
				prefix.channels.start()
				go prefix.finishAfterChannels()
				defer prefix.Close()
				time.Sleep(min(parentLifetime, 120*time.Second))
				synctest.Wait()
				select {
				case <-prefix.Done():
				default:
					t.Fatal("autonomous retirement did not join prefix")
				}
				select {
				case <-child.done:
				default:
					t.Fatal("retired prefix left nested reader running")
				}
				if err := prefix.Close(); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}
