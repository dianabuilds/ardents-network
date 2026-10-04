package duty

import (
	"context"
	"errors"
	"time"
)

const operationLeaseWait = time.Second

// OpenOperation waits for a short competing transaction, bounded by the earlier
// of ctx and one second. It retains Open's exclusive lease and full generation
// verification. The caller must Close the returned store; ctx bounds acquisition
// only. An incomplete initial root claim still fails before waiting; this does
// not adopt a competing initializer. Long-lived owners use Open and continue
// to refuse duplicate ownership.
func OpenOperation(ctx context.Context, input Config) (*store, error) {
	if ctx == nil {
		return nil, errors.New("local role operation context is absent")
	}
	bounded, cancel := context.WithTimeout(ctx, operationLeaseWait)
	defer cancel()
	return open(input, bounded)
}

func acquireOperationLease(ctx context.Context, root string) (rootLease, error) {
	for {
		if err := ctx.Err(); err != nil {
			return rootLease{}, err
		}
		lease, err := acquireRootLease(root)
		if err == nil {
			if stopped := ctx.Err(); stopped != nil {
				return rootLease{}, errors.Join(stopped, lease.release())
			}
			return lease, nil
		}
		if !rootLeaseBusy(err) {
			return rootLease{}, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return rootLease{}, ctx.Err()
		case <-timer.C:
		}
	}
}
