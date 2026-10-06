package bootstrap

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

// Only mechanical credit controls inject time. Production samples under the
// duty lock; this helper supplies no Network or transport authority.
func (c *Claim) charge(now time.Time, n uint64) error {
	if c == nil || c.state == nil || c.state.adjacency == nil || c.state.adjacency.state == nil || c.state.adjacency.state.budget == nil {
		return errors.New("bootstrap claim absent")
	}
	b := c.state.adjacency.state.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	return c.chargeLocked(now, n)
}

func TestBootstrapPrepaidTerminationSurvivesExhaustionOnce(t *testing.T) {
	b := NewBudget()
	terminal, err := b.HoldTermination(time.Now().Add(10 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	if b.live != 0 || b.credit != (uint64(131072)-17)*creditScale {
		t.Fatal("terminal was not debited before a live claim", b.live, b.credit)
	}
	// All remaining credit is exhausted while the held CLOSE is still owned.
	b.credit = 0
	now := b.last
	b.mu.Unlock()
	b.mu.Lock()
	if err := terminal.emitLocked(now); err != nil {
		t.Fatal("exhaustion removed prepaid terminal", err)
	}
	b.mu.Unlock()
	if err := terminal.Emit(); err == nil {
		t.Fatal("terminal output permit was copied or renewed")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.credit != 0 || b.live != 0 || b.held != 0 {
		t.Fatal("terminal emission refunded output or created a claim")
	}
}

func TestBootstrapUnavailableTerminationGrantsNoOutput(t *testing.T) {
	for _, b := range []*Budget{nil, {}} {
		if terminal, err := b.HoldTermination(time.Now().Add(time.Second)); err == nil || terminal != nil {
			t.Fatal("absent duty output owner granted a terminal")
		}
	}
	b := NewBudget()
	if terminal, err := b.HoldTermination(time.Now().Add(-time.Second)); err == nil || terminal != nil {
		t.Fatal("expired bound granted terminal output")
	}
}

func TestBootstrapHeldTerminalCannotAccumulateAnotherFullBurst(t *testing.T) {
	for _, emitted := range []bool{true, false} {
		t.Run(map[bool]string{true: "emitted", false: "joined-without-emission"}[emitted], func(t *testing.T) {
			now := time.Unix(100, 0)
			b := newBudget(now)
			b.mu.Lock()
			terminal, err := b.holdTerminationLocked(now, now.Add(10*time.Second))
			b.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			claim, err := b.Adjacency().reserve(now, now.Add(10*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			later := now.Add(time.Second)
			// 17 held bytes plus 131055 new bytes exhaust the 131072-byte
			// burst, even after enough time to replenish the available space.
			if err := claim.charge(later, 131055); err != nil {
				t.Fatal(err)
			}
			if err := claim.charge(later, 1); err == nil {
				t.Fatal("held output coexisted with an independently full burst")
			}
			b.mu.Lock()
			if emitted {
				err = terminal.emitLocked(later)
			} else {
				terminal.releaseLocked(later)
			}
			b.mu.Unlock()
			if err != nil {
				t.Fatal("held terminal was not available", err)
			}
			copy := *terminal
			if err := copy.Emit(); err == nil {
				t.Fatal("copied terminal reissued its permit")
			}
			copy.ReleaseAfterJoin()
			if err := claim.charge(later, 1); err == nil {
				t.Fatal("emission or joined release refunded the debit")
			}
			if err := claim.charge(later.Add(60*time.Microsecond), 1); err != nil {
				t.Fatal("subsequent fractional refill lost", err)
			}
		})
	}
}

func TestBootstrapCopiedClaimsAndAdjacenciesKeepOriginalOwnership(t *testing.T) {
	b := NewBudget()
	a := b.Adjacency()
	var claims []*Claim
	for range 4 {
		c, err := a.Reserve(time.Now().Add(10 * time.Second))
		if err != nil {
			t.Fatal(err)
		}
		claims = append(claims, c)
	}
	adjacencyCopy := *a
	if _, err := adjacencyCopy.Reserve(time.Now().Add(time.Second)); err == nil {
		t.Fatal("adjacency copy multiplied physical capacity")
	}
	claimCopy := *claims[0]
	claims[0].ReleaseAfterJoin()
	claimCopy.ReleaseAfterJoin()
	if _, err := claimCopy.Restriction(); err == nil {
		t.Fatal("copied released claim recovered its restriction")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.live != 3 || a.state.live != 3 {
		t.Fatal("copied claim returned another physical reservation", b.live, a.state.live)
	}
}

func TestBootstrapPublicConcurrentOutputDoesNotInventClockRollback(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	b := NewBudget()
	var claims []*Claim
	for range 4 {
		a := b.Adjacency()
		for range 4 {
			c, err := a.Reserve(time.Now().Add(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			claims = append(claims, c)
			defer c.ReleaseAfterJoin()
		}
	}
	start := make(chan struct{})
	results := make(chan error, len(claims))
	var workers sync.WaitGroup
	for _, c := range claims {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for range 1000 {
				if err := c.ChargeOutput(1); err != nil {
					results <- err
					return
				}
			}
			results <- nil
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	// 16,000 bytes fit the initial 131,072-byte burst without any refill.
	// Every original claim remains live; scheduling cannot become clock loss.
	for err := range results {
		if err != nil {
			t.Fatal("live output within burst refused", err)
		}
	}
}

func TestBootstrapConcurrentClaimsAndOutputShareOneDutyBudget(t *testing.T) {
	now := time.Now()
	b := newBudget(now)
	results := make(chan *Claim, 64)
	var workers sync.WaitGroup
	for range 8 {
		a := b.Adjacency()
		for range 8 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				if c, err := a.reserve(now, now.Add(10*time.Second)); err == nil {
					results <- c
				}
			}()
		}
	}
	workers.Wait()
	close(results)
	var claims []*Claim
	for c := range results {
		claims = append(claims, c)
	}
	if len(claims) != 16 {
		t.Fatal("concurrent duty capacity", len(claims))
	}
	output := make(chan error, len(claims))
	for _, c := range claims {
		workers.Add(1)
		go func() { defer workers.Done(); output <- c.charge(now, 10000) }()
	}
	workers.Wait()
	close(output)
	accepted := 0
	for err := range output {
		if err == nil {
			accepted++
		}
	}
	if accepted != 13 {
		t.Fatal("concurrent output exceeded or lost the 128-KiB burst", accepted)
	}
	if err := claims[0].charge(now, 1072); err != nil {
		t.Fatal("remaining shared credit", err)
	}
	if err := claims[0].charge(now, 1); err == nil {
		t.Fatal("concurrent output duplicated credit")
	}
	for _, c := range claims {
		for range 2 {
			workers.Add(1)
			go func() { defer workers.Done(); c.ReleaseAfterJoin() }()
		}
	}
	workers.Wait()
	var replacements []*Claim
	for range 4 {
		a := b.Adjacency()
		for range 4 {
			c, err := a.reserve(now, now.Add(time.Second))
			if err != nil {
				t.Fatal("joined returns lost capacity", err)
			}
			replacements = append(replacements, c)
		}
	}
	if _, err := b.Adjacency().reserve(now, now.Add(time.Second)); err == nil {
		t.Fatal("concurrent repeated return multiplied capacity")
	}
	for _, c := range replacements {
		c.ReleaseAfterJoin()
	}
}

func TestBootstrapDutyAndAdjacencyRetainClaimsUntilJoin(t *testing.T) {
	now := time.Now()
	b := newBudget(now)
	var claims []*Claim
	var adjacencies []*Adjacency
	for range 4 {
		a := b.Adjacency()
		adjacencies = append(adjacencies, a)
		for range 4 {
			claim, err := a.reserve(now, now.Add(time.Minute))
			if err != nil || claim.Deadline() != now.Add(10*time.Second) {
				t.Fatal("original lane bound", err)
			}
			claims = append(claims, claim)
		}
		if _, err := a.reserve(now, now.Add(time.Second)); err == nil {
			t.Fatal("fifth adjacent lane accepted")
		}
	}
	fresh := b.Adjacency()
	if _, err := fresh.reserve(now, now.Add(time.Second)); err == nil {
		t.Fatal("new connection multiplied duty capacity")
	}
	adjacencies[0].Seal()
	if _, err := adjacencies[0].reserve(now, now.Add(time.Second)); err == nil {
		t.Fatal("sealed adjacency admitted work")
	}
	if _, err := fresh.reserve(now, now.Add(time.Second)); err == nil {
		t.Fatal("seal reclaimed unjoined physical capacity")
	}
	claims[0].ReleaseAfterJoin()
	claims[0].ReleaseAfterJoin()
	replacement, err := fresh.reserve(now, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.reserve(now, now.Add(time.Second)); err == nil {
		t.Fatal("repeated release returned a sibling's claim")
	}
	replacement.ReleaseAfterJoin()
	for _, c := range claims {
		c.ReleaseAfterJoin()
	}
}

func TestBootstrapFractionalOutputSurvivesRefusalAndNewConnections(t *testing.T) {
	now := time.Unix(100, 0)
	b := newBudget(now)
	first, err := b.Adjacency().reserve(now, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.charge(now, 128<<10); err != nil {
		t.Fatal(err)
	}
	for _, elapsed := range []time.Duration{20 * time.Microsecond, 40 * time.Microsecond} {
		if err := first.charge(now.Add(elapsed), 1); err == nil {
			t.Fatal("fractional credit rounded up")
		}
	}
	if err := first.charge(now.Add(60*time.Microsecond), 1); err != nil {
		t.Fatal("refusals discarded fractional credit", err)
	}
	second, err := b.Adjacency().reserve(now, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := second.charge(now.Add(60*time.Microsecond), 1); err == nil {
		t.Fatal("new adjacency created a burst")
	}
	first.ReleaseAfterJoin()
	if err := second.charge(now.Add(60*time.Microsecond), 1); err == nil {
		t.Fatal("joined release refunded output")
	}
	if err := second.charge(now.Add(120*time.Microsecond), 1); err != nil {
		t.Fatal("fractional duty refill lost", err)
	}
	if err := second.charge(now.Add(119*time.Microsecond), 1); err == nil {
		t.Fatal("clock rollback accepted")
	}
	second.ReleaseAfterJoin()
}

func TestBootstrapExpiryAndSharedQueuedWork(t *testing.T) {
	now := time.Now()
	b := newBudget(now)
	a := b.Adjacency()
	c, err := a.reserve(now, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if c.Deadline() != now.Add(time.Second) {
		t.Fatal("short original bound extended")
	}
	if err := c.charge(now.Add(time.Second), 1); err == nil {
		t.Fatal("expired lane output accepted")
	}
	if a.state.live != 1 || b.live != 1 {
		t.Fatal("expiry returned unjoined capacity")
	}
	if _, err := c.Restriction(); err != nil {
		t.Fatal(err)
	}
	a.Seal()
	if _, err := c.Restriction(); err == nil {
		t.Fatal("sealed claim supplied a restriction")
	}
	c.ReleaseAfterJoin()
	if err := c.ChargeOutput(1); err == nil {
		t.Fatal("released claim emitted")
	}
	queues := b.Queues()
	release, err := queues.HoldChild(256 << 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Queues().HoldChild(1); err == nil {
		t.Fatal("queued work exceeded duty cap")
	}
	release()
	releaseAgain, err := b.Queues().HoldChild(256 << 10)
	if err != nil {
		t.Fatal("joined return did not restore the shared queue capacity", err)
	}
	releaseAgain()
}

func TestBootstrapBurstCeilingDiscardsSurplusAndZeroBudgetRefuses(t *testing.T) {
	now := time.Unix(100, 0)
	b := newBudget(now)
	first, err := b.Adjacency().reserve(now, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.charge(now, 128<<10); err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Minute)
	second, err := b.Adjacency().reserve(later, later.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := second.charge(later, 128<<10); err != nil {
		t.Fatal(err)
	}
	if err := second.charge(later, 1); err == nil {
		t.Fatal("surplus beyond burst ceiling retained")
	}
	if err := first.charge(later, 1); err == nil {
		t.Fatal("expired original claim revived")
	}
	first.ReleaseAfterJoin()
	second.ReleaseAfterJoin()
	if _, err := (&Budget{}).Adjacency().Reserve(time.Now().Add(time.Second)); err == nil {
		t.Fatal("unconstructed budget admitted work")
	}
}
