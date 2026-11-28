package localread_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document/localread"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func TestDocumentDiagnosticsContainOnlyLocalClass(t *testing.T) {
	secret := errors.New("fixture destination or private response")
	for _, test := range []struct {
		err  error
		want string
	}{
		{secret, "document operation unavailable"},
		{errors.Join(secret, context.Canceled), "read cancelled"},
		{errors.Join(secret, context.DeadlineExceeded), "read timed out"},
		{errors.Join(secret, endpointstream.ErrInvalidLocalInput), "document destination or local input is invalid"},
	} {
		if got := localread.Failure(test.err); got != test.want {
			t.Fatal("diagnostic leaked remote or local input")
		}
	}
}
