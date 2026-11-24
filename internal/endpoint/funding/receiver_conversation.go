package funding

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// RunLocalReceiver retains and joins the original allowance and authority.
func RunLocalReceiver(ctx context.Context, root, profile string, network *state.AuthorityDeclaration, receiverIdentity receiving.Receiver, notAfter time.Time, input io.ReadCloser, out, diagnostic io.Writer) (code int) {
	receiver, err := openLocalReceiver(ctx, root, profile, network, receiverIdentity, notAfter)
	if err != nil {
		switch {
		case errors.Is(err, errInvalidLocalReceiver):
			return 2
		case errors.Is(err, errLocalReceiverCanceled):
			return 130
		default:
			return 1
		}
	}
	o := receiver.allowance
	defer func() {
		if receiver.closeAuthority() != nil {
			code = 1
		}
	}()
	defer func() {
		if o.Close() != nil {
			code = 1
		}
		_ = json.NewEncoder(diagnostic).Encode(map[string]any{"operation": "admission.receiver", "phase": "closed", "exit_code": code})
	}()
	return admission.RunConsole(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		return o.HandleLocalCommand(ctx, raw)
	})
}
