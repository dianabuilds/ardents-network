package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/console"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

// RunLocalHolder retains and joins one local holder conversation.
func RunLocalHolder(ctx context.Context, root, profile string, network *state.AuthorityDeclaration, role admission.AllocationRole, options Config, input io.ReadCloser, out, diagnostic io.Writer, preparation executionruntime.Permission) (code int) {

	holder, err := openLocalHolder(ctx, root, profile, network, role, options, preparation)
	if err != nil {
		if errors.Is(err, ErrInvalidLocalConfiguration) {
			return 2
		}
		if errors.Is(err, ErrLocalConfigurationCanceled) {
			return 130
		}
		if holder != nil {
			_ = holder.finishConversation(1, diagnostic)
		}
		return 1
	}
	conversation := holder.Session
	defer func() { code = holder.finishConversation(code, diagnostic) }()
	return console.Run(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		return conversation.HandleLocalCommand(ctx, raw, preparation != nil)
	})
}

func (holder *localHolder) finishConversation(code int, diagnostic io.Writer) int {
	if holder.Session != nil && holder.Session.Close() != nil {
		code = 1
	}
	if holder.closeAuthority() != nil {
		code = 1
	}
	_ = json.NewEncoder(diagnostic).Encode(map[string]any{"operation": "admission.holder", "phase": "closed", "exit_code": code})
	return code
}
