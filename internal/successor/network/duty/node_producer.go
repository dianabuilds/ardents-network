package duty

import (
	"context"
	"crypto/rand"
	"errors"
	"path/filepath"
	"sync"
)

// NodeProducer retains one original local participation caller. Its private
// producer is never supplied by a caller or adopted from durable observations.
// The consumer supplies independently authenticated Node facts and must retain
// this handle until all physical users have joined. This is not State authority.
type NodeProducer struct{ original *nodeProducer }

type nodeProducer struct {
	mu            sync.Mutex
	caller        context.Context
	config        Config
	producer      [32]byte
	participation NodeParticipation
	phase         NodePhase
	generation    uint64
	current       string
	closed        bool
	closeErr      error
}

// OpenNodeProducer commits prepared participation in a short transaction, then
// releases the journal lease. A canceled original caller cannot publish a live
// producer. Ambiguous failures retain durable conflict rather than inventing
// successful removal; they grant no runtime admission.
func OpenNodeProducer(ctx context.Context, config Config, participation NodeParticipation) (*NodeProducer, error) {
	if ctx == nil || !filepath.IsAbs(config.Root) || filepath.Clean(config.Root) != config.Root {
		return nil, errors.New("original Node participation caller or root unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := nodeParticipationRestriction(participation, NodePrepared); err != nil {
		return nil, err
	}
	original := &nodeProducer{caller: ctx, config: config, participation: participation, phase: NodePrepared}
	if _, err := rand.Read(original.producer[:]); err != nil {
		return nil, err
	}
	store, err := OpenOperation(ctx, config)
	if err != nil {
		return nil, err
	}
	err = store.RetainNode(original.producer, participation, NodePrepared)
	if err == nil {
		err = original.match(store)
	}
	err = errors.Join(err, store.Close())
	if err != nil {
		return nil, err
	}
	original.config.Create = false
	handle := &NodeProducer{original: original}
	if err := original.permit(); err != nil {
		// No physical consumer has been returned; removal is safe after this
		// preparation transaction joins, with an independent cleanup caller.
		return nil, errors.Join(err, handle.Close(context.Background()))
	}
	return handle, nil
}

// Check reopens the short transaction and verifies this exact original producer
// and retained restriction. Missing or substituted history cannot recreate it.
func (handle *NodeProducer) Check() error {
	if handle == nil || handle.original == nil {
		return errors.New("original Node producer absent")
	}
	original := handle.original
	original.mu.Lock()
	defer original.mu.Unlock()
	if err := original.permit(); err != nil {
		return err
	}
	store, err := OpenOperation(original.caller, original.config)
	if err != nil {
		return err
	}
	err = errors.Join(original.match(store), store.Close())
	if err == nil {
		return original.permit()
	}
	return errors.Join(err, original.permit())
}

// Live advances only the prepared original producer. Repeating it does not
// renew validity; a canceled, removed or substituted producer cannot rebind.
func (handle *NodeProducer) Live() error {
	if handle == nil || handle.original == nil {
		return errors.New("original Node producer absent")
	}
	original := handle.original
	original.mu.Lock()
	defer original.mu.Unlock()
	if err := original.permit(); err != nil {
		return err
	}
	store, err := OpenOperation(original.caller, original.config)
	if err != nil {
		return err
	}
	err = original.match(store)
	if err == nil {
		err = store.RetainNode(original.producer, original.participation, NodeLive)
		if err == nil {
			original.phase = NodeLive
			err = original.match(store)
		}
	}
	err = errors.Join(err, store.Close())
	if err == nil {
		return original.permit()
	}
	return errors.Join(err, original.permit())
}

func (original *nodeProducer) permit() error {
	if err := original.caller.Err(); err != nil {
		return err
	}
	if original.closed {
		return errors.New("original Node producer retired")
	}
	now := original.config.Clock()
	if !now.Before(original.participation.EpochUntil) || !now.Before(original.participation.RecordUntil) {
		return errors.New("original Node participation expired")
	}
	return nil
}

// Close removes only the original producer, once, after the consumer's actual
// physical join. A cleanup caller bounds journal acquisition independently of
// the canceled runtime caller. Failed removal remains failure for every copy;
// it does not report withdrawn or remove another producer's Source record.
func (handle *NodeProducer) Close(ctx context.Context) error {
	if handle == nil || handle.original == nil {
		return nil
	}
	original := handle.original
	original.mu.Lock()
	defer original.mu.Unlock()
	if original.closed {
		return original.closeErr
	}
	original.closed = true
	store, err := OpenOperation(ctx, original.config)
	if err == nil {
		err = original.match(store)
		if err == nil {
			err = store.Remove(original.producer)
		}
		err = errors.Join(err, store.Close())
	}
	original.closeErr = err
	return err
}

func (original *nodeProducer) match(store *store) error {
	expected, err := nodeParticipationRestriction(original.participation, original.phase)
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil {
		return errors.New("original Node participation history unavailable")
	}
	if store.state.Generation < original.generation || store.state.Generation == original.generation && store.current != original.current {
		return errors.New("original Node participation history rolled back or conflicted")
	}
	var count int
	for _, record := range store.state.Duties {
		if record.Producer != original.producer {
			continue
		}
		count++
		if record.Identity != expected.Identity || record.Family != expected.Family || record.Class != expected.Class ||
			record.State != expected.State || record.NotAfter != expected.NotAfter.Unix() {
			return errors.New("original Node participation changed")
		}
	}
	if count != 1 {
		return errors.New("original Node participation absent")
	}
	original.generation, original.current = store.state.Generation, store.current
	return nil
}
