package tui

import (
	"context"
	"fmt"

	"ardents/internal/cli/client"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type tuiAction string

const (
	tuiActionNodeStart tuiAction = "node.start"
	tuiActionNodeStop  tuiAction = "node.stop"
)

func tuiActionForKey(section tuiSection, key string) (tuiAction, bool) {
	switch section {
	case tuiNode:
		switch key {
		case "s":
			return tuiActionNodeStart, true
		case "x":
			return tuiActionNodeStop, true
		}
	case tuiNetwork, tuiWorkloads, tuiData, tuiDiagnostics:
		return "", false
	}
	return "", false
}

func (a *Command) executeTUIAction(ctx context.Context, action tuiAction) (string, error) {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	switch action {
	case tuiActionNodeStart:
		resp, err := a.ctx.Client.Service().StartNode(callCtx, client.Request(&ardentsv1.StartNodeRequest{}))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("node start -> %s", resp.Msg.GetStatus().GetState()), nil
	case tuiActionNodeStop:
		resp, err := a.ctx.Client.Service().StopNode(callCtx, client.Request(&ardentsv1.StopNodeRequest{}))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("node stop -> %s", resp.Msg.GetStatus().GetState()), nil
	default:
		return "", fmt.Errorf("unsupported tui action %q", action)
	}
}
