package spending

import "errors"

const (
	freshRootAvailable uint8 = iota + 1
	freshRootTaken
	freshRootInitializing
	freshRootFinished
)

// FreshRoot is a one-use fact of successful exclusive journal creation and
// durability. Retained opens never issue it. It grants no token or peer rights
// and exposes neither the spend journal nor its live lease.
type FreshRoot struct{ owner *Ledger }

// RootInitialization retains the original fresh owner's lifecycle across a
// companion root's independent filesystem effects. Check precedes effects;
// Complete must succeed before that companion is made available to consumers.
// An interrupted initialization cannot be retried with the same fact.
type RootInitialization struct{ owner *Ledger }

// TakeFreshRoot transfers the creation fact once, before any admission attempt.
func (ledger *Ledger) TakeFreshRoot() (*FreshRoot, error) {
	if ledger == nil {
		return nil, errors.New("fresh receiving root unavailable")
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.closed || ledger.failure != nil || ledger.freshRootPhase != freshRootAvailable {
		return nil, errors.New("fresh receiving root unavailable")
	}
	ledger.freshRootPhase = freshRootTaken
	return &FreshRoot{owner: ledger}, nil
}

// InvalidateFreshRoot permanently retires creation evidence on any receiving
// attempt, including one refused before token verification or durable spending.
func (ledger *Ledger) InvalidateFreshRoot() {
	if ledger == nil {
		return
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.freshRootPhase = 0
}

// Begin consumes the fact for exactly the original public receiving binding.
// A failed consumption also spends the fact; it cannot select another binding.
func (fact *FreshRoot) Begin(binding Binding) (*RootInitialization, error) {
	if fact == nil || fact.owner == nil {
		return nil, errors.New("fresh receiving root unavailable")
	}
	ledger := fact.owner
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.closed || ledger.failure != nil || ledger.freshRootPhase != freshRootTaken {
		return nil, errors.New("fresh receiving root unavailable")
	}
	ledger.freshRootPhase = freshRootFinished
	if binding != ledger.binding {
		return nil, errors.New("fresh receiving root binding differs")
	}
	ledger.freshRootPhase = freshRootInitializing
	return &RootInitialization{owner: ledger}, nil
}

// Check refuses stale owners before each initialization effect. A concurrent
// admission or close during independent I/O is caught again by Complete.
func (initialization *RootInitialization) Check() error {
	return initialization.check(false)
}

// Complete consumes the final handoff. Failed or repeated completion never
// authorizes publication, even if the filesystem already contains complete bytes.
func (initialization *RootInitialization) Complete() error {
	return initialization.check(true)
}

func (initialization *RootInitialization) check(complete bool) error {
	if initialization == nil || initialization.owner == nil {
		return errors.New("receiving root initialization unavailable")
	}
	ledger := initialization.owner
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.closed || ledger.failure != nil || ledger.freshRootPhase != freshRootInitializing {
		return errors.New("receiving root initialization unavailable")
	}
	if complete {
		ledger.freshRootPhase = freshRootFinished
	}
	return nil
}
