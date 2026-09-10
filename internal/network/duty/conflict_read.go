package duty

import (
	"context"
	"errors"
	"time"
)

// ReadConflict performs one bounded query without retaining the root lease.
func ReadConflict(root string, clock func() time.Time, identity, family [32]byte) (bool, error) {
	roles, err := OpenOperation(context.Background(), Config{Root: root, Clock: clock})
	if err != nil {
		return false, err
	}
	conflict, queryErr := roles.Conflict(identity, family)
	return conflict, errors.Join(queryErr, roles.Close())
}
