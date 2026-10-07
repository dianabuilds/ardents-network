//go:build !linux

package installation

import "context"

// Unsupported hosts have no physical custody representation or accepting
// reader. Keeping the private marker permits the same portable Request grammar.
type requestCustody struct{}

func readOwnedRequest(context.Context, string, bool) (Request, error) {
	return Request{}, ErrNativeUnavailable
}
