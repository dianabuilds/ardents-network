package channel

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Exercise actual OPEN framing and both allocators. These physical protocol
// fixtures supply no successful Network, Admission, Hosting or TLS authority.
func TestChildCapacityWorkAndReservedControls(t *testing.T) {
	for _, node := range []bool{false, true} {
		for _, purpose := range []ardp.Purpose{ardp.PurposeIssuer, ardp.PurposeReachability, ardp.PurposeIntroduction, ardp.PurposeSubmission} {
			t.Run(capacityCase(node, purpose), func(t *testing.T) {
				pair := newCapacityPair(t, node)
				work := make([]capacityChild, 256)
				for i := range work {
					work[i] = pair.open(t, ardp.PurposeDataJoin)
				}
				pair.refuse(t, ardp.PurposeDataJoin)
				controls := []capacityChild{pair.open(t, purpose), pair.open(t, purpose)}
				pair.refuse(t, purpose)
				pair.refuse(t, ardp.PurposeForwarding)
				pair.counts(t, 258)

				// Reserved controls cannot consume a subsequently freed work slot.
				pair.finish(t, work[0])
				work[0] = pair.open(t, ardp.PurposeDataJoin)
				pair.refuse(t, ardp.PurposeDataJoin)
				pair.refuse(t, purpose)
				pair.counts(t, 258)

				if err := controls[0].local.Close(); err != nil {
					t.Fatal(err)
				}
				pair.refuse(t, purpose) // CLOSE alone cannot return the local claim.
				pair.finish(t, controls[0])
				pair.refuse(t, ardp.PurposeDataJoin)
				controls[0] = pair.open(t, purpose)
				pair.refuse(t, purpose)
				pair.counts(t, 258)
			})
		}
	}
}

func TestChildCapacityControlUsesOrdinarySlotFirst(t *testing.T) {
	pair := newCapacityPair(t, false)
	pair.open(t, ardp.PurposeIssuer)
	for range 255 {
		pair.open(t, ardp.PurposeDataJoin)
	}
	pair.refuse(t, ardp.PurposeDataJoin)
	pair.open(t, ardp.PurposeReachability)
	pair.open(t, ardp.PurposeIntroduction)
	pair.refuse(t, ardp.PurposeSubmission)
	pair.counts(t, 258)
}

func TestChildCapacityReceiverRefusesExcessOPEN(t *testing.T) {
	for _, node := range []bool{false, true} {
		for _, excess := range []ardp.Purpose{ardp.PurposeDataJoin, ardp.PurposeForwarding, ardp.PurposeName, ardp.PurposeIssuer} {
			t.Run(capacityCase(node, excess), func(t *testing.T) {
				local, remote := net.Pipe()
				end := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
				accepted := make(chan struct{}, 1)
				budget := NewBudget(64 << 20)
				s := New(t.Context(), local, end, 32<<20, nil, false, budget, func(ctx context.Context, _ *Lane, _ []byte) error {
					accepted <- struct{}{}
					<-ctx.Done()
					return nil
				})
				t.Cleanup(func() {
					s.Retire(nil)
					_ = remote.Close()
					_ = s.Close()
					assertCapacityBudget(t, budget, 0)
				})
				var id uint32 = 1
				write := func(purpose ardp.Purpose) error {
					err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: capacityBody(end, node, purpose)})
					id += 2
					return err
				}
				count := 256
				if excess == ardp.PurposeIssuer {
					count += 2
				}
				for i := range count {
					purpose := ardp.PurposeDataJoin
					if i >= 256 {
						purpose = ardp.PurposeIntroduction
					}
					if err := write(purpose); err != nil {
						t.Fatal("valid OPEN refused", i, err)
					}
					select {
					case <-accepted:
					case <-s.Done():
						t.Fatal("valid OPEN retired receiving parent", i)
					case <-time.After(2 * time.Second):
						t.Fatal("valid OPEN did not reach receiving handler", i)
					}
				}
				assertCapacityBudget(t, budget, uint32(count))
				// A hostile peer bypasses the opening allocator. Preserve the
				// receiver's existing terminal refusal for an excess OPEN.
				_ = write(excess)
				select {
				case <-s.Done():
				case <-time.After(2 * time.Second):
					t.Fatal("excess OPEN did not retire receiving parent")
				}
				select {
				case <-accepted:
					t.Fatal("excess OPEN reached receiving handler")
				default:
				}
				if s.Close() == nil {
					t.Fatal("excess OPEN lost its refusal")
				}
			})
		}
	}
}

func TestChildCapacityFailedControlOPENReturnsReserve(t *testing.T) {
	for _, failure := range []string{"output-refusal", "cancellation"} {
		t.Run(failure, func(t *testing.T) {
			physical := &heldPhysicalDeadline{lifecycleConn: newLifecycleConn(false), entered: make(chan struct{}), resume: make(chan struct{})}
			physical.writes = make(chan []byte, 2048)
			end := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
			budget := NewBudget(64 << 20)
			refused := errors.New("original output capacity refusal")
			var refuse bool // Output and mutation are serialized by the session lock.
			s := Prepare(t.Context(), physical, end, 32<<20, nil, false, budget, Handlers{Output: func(uint64) error {
				if refuse {
					return refused
				}
				return nil
			}})
			var resume sync.Once
			t.Cleanup(func() {
				resume.Do(func() { close(physical.resume) })
				_ = s.Close()
				assertCapacityBudget(t, budget, 0)
			})
			open := func(purpose ardp.Purpose) (*Lane, error) {
				return s.Open(t.Context(), t.Context(), capacityBody(end, false, purpose))
			}
			for range 256 {
				if _, err := open(ardp.PurposeDataJoin); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := open(ardp.PurposeIssuer); err != nil {
				t.Fatal("first reserved control refused", err)
			}
			physical.mu.Lock()
			before := len(physical.output)
			physical.mu.Unlock()
			if failure == "output-refusal" {
				refuse = true
				if lane, err := open(ardp.PurposeSubmission); lane != nil || !errors.Is(err, refused) {
					t.Fatal("control OPEN did not retain output refusal", lane, err)
				}
				refuse = false
			} else {
				physical.armed = true
				caller, cancel := context.WithCancel(t.Context())
				defer cancel()
				result := make(chan error, 1)
				go func() {
					lane, err := s.Open(caller, caller, capacityBody(end, false, ardp.PurposeSubmission))
					if lane != nil {
						err = errors.Join(err, errors.New("canceled control returned a lane"), lane.Close())
						lane.Finish()
					}
					result <- err
				}()
				select {
				case <-physical.entered:
				case <-time.After(2 * time.Second):
					t.Fatal("reserved OPEN did not reach physical deadline")
				}
				s.mu.Lock()
				lane := s.active
				s.mu.Unlock()
				cancel()
				waitFramingState(t, lane, func() bool { return lane.localClosed && !lane.cleanupEnd.IsZero() })
				resume.Do(func() { close(physical.resume) })
				if err := lifecycleResult(t, result); !errors.Is(err, context.Canceled) {
					t.Fatal("original control cancellation lost", err)
				}
			}
			assertCapacityBudget(t, budget, 257)
			physical.mu.Lock()
			after := len(physical.output)
			physical.mu.Unlock()
			if after != before {
				t.Fatal("unemitted control failure emitted OPEN or CLOSE", before, after)
			}
			if !s.Live() {
				t.Fatal("unemitted control failure retired healthy parent")
			}
			if _, err := open(ardp.PurposeSubmission); err != nil {
				t.Fatal("failed OPEN retained reserved capacity", err)
			}
			if lane, err := open(ardp.PurposeReachability); lane != nil || err == nil {
				t.Fatal("failed OPEN returned reserved capacity twice", lane, err)
			}
			assertCapacityBudget(t, budget, 258)
		})
	}
}

func TestChildCapacityReceivingRefusalReturnsReserve(t *testing.T) {
	local, remote := net.Pipe()
	end := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	accepted := make(chan struct{}, 1)
	joined := make(chan struct{})
	budget := NewBudget(64 << 20)
	s := Prepare(t.Context(), local, end, 32<<20, nil, false, budget, Handlers{
		PrepareOpen: func(l *Lane, _ []byte) error {
			if l.id == 513 { // First control after 256 work OPENs.
				if err := l.RetainUntilFinish(func() { close(joined) }); err != nil {
					return err
				}
				return errors.New("original receiving preparation refused")
			}
			return nil
		},
		Open: func(ctx context.Context, _ *Lane, _ []byte) error {
			accepted <- struct{}{}
			<-ctx.Done()
			return nil
		},
	})
	s.Start()
	t.Cleanup(func() {
		s.Retire(nil)
		_ = remote.Close()
		_ = s.Close()
		assertCapacityBudget(t, budget, 0)
	})
	for i := range 259 {
		purpose := ardp.PurposeDataJoin
		if i >= 256 {
			purpose = ardp.PurposeIssuer
		}
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOpen, Lane: uint32(2*i + 1), Body: capacityBody(end, true, purpose)}); err != nil {
			t.Fatal("valid receiving OPEN failed", i, err)
		}
		if i == 256 {
			if err := remote.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			frame, err := ardp.ReadFrame(remote)
			if err != nil || frame.Kind != ardp.KindClose || frame.Lane != 513 || len(frame.Body) != 1 || frame.Body[0] != 1 {
				t.Fatal("receiving preparation lost original refusal", frame, err)
			}
			select {
			case <-joined:
			case <-time.After(2 * time.Second):
				t.Fatal("refused receiving child did not join")
			}
			assertCapacityBudget(t, budget, 256)
			continue
		}
		select {
		case <-accepted:
		case <-s.Done():
			t.Fatal("refused receiving child retained control capacity", i)
		case <-time.After(2 * time.Second):
			t.Fatal("valid receiving OPEN did not reach handler", i)
		}
	}
	if !s.Live() {
		t.Fatal("child preparation refusal retired healthy parent")
	}
	assertCapacityBudget(t, budget, 258)
}

func TestChildCapacityControlsDoNotExpandPrincipalLimit(t *testing.T) {
	budget := NewBudget(64 << 20)
	end := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	var sessions []*Session
	var firstWork []*Lane
	for range 4 {
		physical := newLifecycleConn(false)
		physical.writes = make(chan []byte, 2048)
		s := New(t.Context(), physical, end, 32<<20, nil, false, budget, nil)
		sessions = append(sessions, s)
		t.Cleanup(func() { _ = s.Close() })
		for i := range 256 {
			lane, err := s.Open(t.Context(), t.Context(), capacityBody(end, true, ardp.PurposeDataJoin))
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				firstWork = append(firstWork, lane)
			}
		}
	}
	assertCapacityBudget(t, budget, 1024)
	openControl := func() (*Lane, error) {
		return sessions[0].Open(t.Context(), t.Context(), capacityBody(end, true, ardp.PurposeIssuer))
	}
	if lane, err := openControl(); lane != nil || err == nil {
		t.Fatal("reserved control bypassed principal limit", lane, err)
	}
	if err := firstWork[1].Close(); err != nil {
		t.Fatal(err)
	}
	firstWork[1].Finish()
	control, err := openControl()
	if err != nil {
		t.Fatal("principal refusal leaked a reserved slot", err)
	}
	assertCapacityBudget(t, budget, 1024)
	if err := control.Close(); err != nil {
		t.Fatal(err)
	}
	control.Finish()
	control.Finish()
	assertCapacityBudget(t, budget, 1023)
	for _, s := range sessions {
		_ = s.Close()
	}
	assertCapacityBudget(t, budget, 0)
}

type capacityChild struct {
	local  *Lane
	remote *Lane
	joined <-chan struct{}
}

type capacityPair struct {
	local, remote *Session
	accepted      chan capacityChild
	end           time.Time
	node          bool
}

func newCapacityPair(t *testing.T, node bool) *capacityPair {
	t.Helper()
	left, right := net.Pipe()
	p := &capacityPair{accepted: make(chan capacityChild, 1), end: time.Now().Add(time.Minute).UTC().Truncate(time.Second), node: node}
	p.local = New(t.Context(), left, p.end, 32<<20, nil, false, NewBudget(64<<20), nil)
	p.remote = New(t.Context(), right, p.end, 32<<20, nil, false, NewBudget(64<<20), func(ctx context.Context, lane *Lane, _ []byte) error {
		joined := make(chan struct{})
		if err := lane.RetainUntilFinish(func() { close(joined) }); err != nil {
			return err
		}
		p.accepted <- capacityChild{remote: lane, joined: joined}
		<-ctx.Done()
		return nil
	})
	t.Cleanup(func() {
		p.local.Retire(nil)
		p.remote.Retire(nil)
		_ = p.local.Close()
		_ = p.remote.Close()
		assertCapacityBudget(t, p.local.queues, 0)
		assertCapacityBudget(t, p.remote.queues, 0)
	})
	return p
}

func (p *capacityPair) open(t *testing.T, purpose ardp.Purpose) capacityChild {
	t.Helper()
	lane, err := p.local.Open(t.Context(), t.Context(), capacityBody(p.end, p.node, purpose))
	if err != nil {
		t.Fatal("valid child OPEN refused", purpose, err)
	}
	select {
	case child := <-p.accepted:
		child.local = lane
		return child
	case <-p.remote.Done():
		t.Fatal("valid child OPEN retired receiving parent", purpose)
	case <-time.After(2 * time.Second):
		t.Fatal("valid child OPEN did not reach receiving handler", purpose)
	}
	return capacityChild{}
}

func (p *capacityPair) refuse(t *testing.T, purpose ardp.Purpose) {
	t.Helper()
	lane, err := p.local.Open(t.Context(), t.Context(), capacityBody(p.end, p.node, purpose))
	if lane != nil || err == nil {
		t.Fatal("excess child OPEN accepted", purpose, lane, err)
	}
	if !p.local.Live() || !p.remote.Live() {
		t.Fatal("local capacity refusal retired a healthy parent")
	}
}

func (p *capacityPair) finish(t *testing.T, child capacityChild) {
	t.Helper()
	if err := child.local.Close(); err != nil {
		t.Fatal("child CLOSE failed", err)
	}
	child.local.Finish()
	select {
	case <-child.joined:
	case <-time.After(2 * time.Second):
		t.Fatal("original receiving child did not join")
	}
	child.local.Finish()
	child.remote.Finish()
}

func (p *capacityPair) counts(t *testing.T, count uint32) {
	t.Helper()
	assertCapacityBudget(t, p.local.queues, count)
	assertCapacityBudget(t, p.remote.queues, count)
}

func assertCapacityBudget(t *testing.T, budget *Budget, count uint32) {
	t.Helper()
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.children != count || budget.used != 0 {
		t.Fatal("physical principal accounting differs", budget.children, budget.used, "want children", count)
	}
}

func capacityBody(end time.Time, node bool, purpose ardp.Purpose) []byte {
	return ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: uint8(purpose), Deadline: end}, node)
}

func capacityCase(node bool, purpose ardp.Purpose) string {
	envelope := "endpoint-"
	if node {
		envelope = "node-"
	}
	return envelope + string(rune('0'+purpose))
}
