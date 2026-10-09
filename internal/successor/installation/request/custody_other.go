//go:build !linux

package request

import "context"

// Unsupported hosts have no physical custody representation or accepting
// reader. Keeping the private marker permits the same portable Document grammar.
type Origin struct{}

func readOwned(context.Context, string, bool) (Document, error) {
	return Document{}, ErrNativeUnavailable
}
