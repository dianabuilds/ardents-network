package transfer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestIdentityIsUniqueUnderConcurrency(t *testing.T) {
	const count = 256
	ids := make(chan string, count)
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() {
			id, err := requestIdentity("blob-fetch")
			require.NoError(t, err)
			ids <- id
		})
	}
	workers.Wait()
	close(ids)

	seen := make(map[string]bool, count)
	for id := range ids {
		require.False(t, seen[id], "duplicate request identity")
		seen[id] = true
	}
	require.Len(t, seen, count)
}
