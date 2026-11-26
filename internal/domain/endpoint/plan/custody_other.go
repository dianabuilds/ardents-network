//go:build !linux && !darwin

package plan

import "context"

// Origin has no physical custody representation on unsupported hosts, which
// have no accepting reader. The private marker preserves the portable Document grammar.
type Origin struct{}

func readOwned(context.Context, string, bool) (Document, error) {
	return Document{}, ErrNativeUnavailable
}
