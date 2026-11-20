//go:build linux && amd64 && text_worker_installed

package runtime

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

// TestInstalledStreamQualificationSeam proves the trusted Stream qualification
// runtime seams against the real root-installed qualification inventory and
// system manager (#654 3c-B1). The root driver installs the fixed stream
// artifact (packaging/stream-qualification-worker/install.py) and pins the
// temporary ardents-endpoint.service whose original non-root MainPID runs this
// binary. For each role it exercises LaunchStreamQualification (stream worker
// activation + fixed INIT/READY + installed reobservation), the ordinary text
// inventory's refusal of the qualification unit, the one-use stream handoff
// guards, the bridge entry over the retained attachment (fast-fail on an
// incomplete stream set), and joined original-cgroup cleanup retaining the
// first failure.
//
// This is component evidence for the qualification runtime seams. The positive
// 64/256-stream NET-14 workload through authenticated Service streams is 3c-B2/
// 3c-C, NOT established here; no whole-host privacy qualification is claimed.
func TestInstalledStreamQualificationSeam(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint MainPID required")
	}
	for _, role := range []string{"reader", "publisher"} {
		t.Run(role, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			var seed [32]byte
			if _, err := rand.Read(seed[:]); err != nil {
				t.Fatal(err)
			}
			owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: [32]byte{2}, Surface: execution.Connection}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			invocation, err := owner.LaunchStreamQualification(ctx, [32]byte{2}, role, streamqualification.ClientToPublisher, seed)
			if err != nil {
				t.Fatalf("stream qualification launch refused: %v", err)
			}
			instance := invocation.activation.Instance
			if instance.Role != role || !strings.HasPrefix(instance.Name, "ardents-stream-qualification-"+role+"@") ||
				instance.PID == 0 || instance.UID == 0 || instance.Invocation == [16]byte{} ||
				!strings.HasPrefix(instance.Cgroup, "/ardents.slice/ardents-qualification.slice/ardents-qualification-owner.slice/") {
				t.Fatalf("stream qualification instance identity is invalid: %+v", instance)
			}
			// The ordinary text inventory must never admit the qualification unit.
			if _, err := worker.ObserveInstance(ctx, instance.Name, role); err == nil {
				t.Fatal("text inventory admitted the qualification unit")
			}
			operation, err := invocation.BeginOperation(ctx)
			if err != nil {
				t.Fatalf("qualification operation refused: %v", err)
			}
			contract, err := operation.BindStreamContract(64<<20, 64<<20)
			if err != nil {
				t.Fatalf("qualification stream contract refused: %v", err)
			}
			// A contract bound to no operation is refused before any handoff.
			if _, err := operation.RunStreamConnections(ctx, StreamContract{}, nil, nil); err == nil {
				t.Fatal("absent stream contract admitted")
			}
			// The bridge entry runs over the retained attachment and fast-fails on
			// an incomplete stream set; the handoff still occurred and cleanup joins.
			if _, err := operation.RunStreamConnections(ctx, contract, nil, nil); err == nil {
				t.Fatal("incomplete qualification stream set admitted")
			}
			// The one transfer attempt is consumed; a second handoff is refused.
			if _, err := operation.RunStreamConnections(ctx, contract, nil, nil); err == nil {
				t.Fatal("second qualification stream handoff admitted")
			}
			operation.Close()
			closeErr := invocation.Close()
			if closeErr == nil {
				t.Fatal("failed qualification workload reported success")
			}
			cleanup, completed := invocation.CleanupResult()
			if !completed {
				t.Fatal("qualification cleanup did not join")
			}
			if cleanup != nil {
				t.Fatalf("qualification physical cleanup failed: %v", cleanup)
			}
		})
	}
}
