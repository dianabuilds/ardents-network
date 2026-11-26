//go:build linux && amd64 && document_worker_installed

package worker

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

// TestInstalledStreamWorkerActivation proves the trusted Stream activation
// boundary against the real root-installed qualification inventory and system
// manager (#654 3c-A). The root driver installs the fixed stream artifact
// (packaging/stream-qualification-worker/install.py) and pins the temporary
// ardents-endpoint.service whose original non-root MainPID runs this binary.
// For each role it exercises ActivateStream's full confinement observation,
// the text inventory's refusal of the qualification unit, ObserveInstanceStream
// re-observation of the pinned identity, the fixed stream INIT/READY exchange
// over the successor attachment, and joined cleanup of the original cgroup.
//
// This is component evidence for the worker activation boundary with the
// installed qualification artifact. It is NOT the NET-14 workload measurement
// (3c-C runner) and NOT a whole-host privacy qualification.
func TestInstalledStreamWorkerActivation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint MainPID required")
	}
	for _, role := range []string{"reader", "publisher"} {
		t.Run(role, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			activation, err := ActivateStream(ctx, role)
			if err != nil {
				t.Fatalf("stream activation refused: %v", err)
			}
			instance := activation.Instance
			if instance.Role != role || !strings.HasPrefix(instance.Name, "ardents-stream-qualification-"+role+"@") ||
				instance.PID == 0 || instance.UID == 0 || instance.Invocation == [16]byte{} ||
				!strings.HasPrefix(instance.Cgroup, "/ardents.slice/ardents-qualification.slice/ardents-qualification-owner.slice/") {
				t.Fatalf("stream instance identity is invalid: %+v", instance)
			}
			// The ordinary text inventory must never admit the qualification unit.
			if _, err := ObserveInstance(ctx, instance.Name, role); err == nil {
				t.Fatal("text inventory admitted the qualification unit")
			}
			observed, err := ObserveInstanceStream(ctx, instance.Name, role)
			if err != nil {
				t.Fatalf("stream re-observation refused: %v", err)
			}
			if observed.PID != instance.PID || observed.UID != instance.UID ||
				observed.Cgroup != instance.Cgroup || observed.Invocation != instance.Invocation {
				t.Fatal("stream re-observation changed the pinned identity")
			}
			// Trusted qualification INIT dispatch: fresh invocation nonce and
			// workload seed, one predeclared profile. The ordinary text runtime
			// is not involved; this test is the trusted launch path for 3c-A.
			var nonce, seed [32]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(seed[:]); err != nil {
				t.Fatal(err)
			}
			streamRole := streamqualification.ReaderRole
			if role == "publisher" {
				streamRole = streamqualification.PublisherRole
			}
			if err := activation.Attachment.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := streamqualification.InitializeWorker(activation.Attachment, streamqualification.Init{
				Role: streamRole, Profile: streamqualification.ClientToPublisher, Nonce: nonce, Seed: seed,
			}); err != nil {
				t.Fatalf("stream INIT/READY exchange refused: %v", err)
			}
			var firstFailure error
			cleanup, err := NewCleanup(instance, func(err error) { firstFailure = err })
			if err != nil {
				t.Fatalf("stream cleanup pin refused: %v", err)
			}
			if err := activation.Attachment.Close(); err != nil {
				t.Fatal(err)
			}
			if err := cleanup.Close(); err != nil {
				t.Fatalf("stream cleanup join refused: %v", err)
			}
			if firstFailure != nil {
				t.Fatalf("first stream cleanup failure: %v", firstFailure)
			}
		})
	}
}
