package cli

import (
	"context"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
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
	case tuiWorkloads:
		return "", false
	}
	return "", false
}

func (a *app) executeTUIAction(ctx context.Context, action tuiAction) (string, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	switch action {
	case tuiActionNodeStart:
		resp, err := a.client.Service().StartNode(callCtx, client.Request(a.cfg.Token, &ardentsv1.StartNodeRequest{}))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("node start -> %s", resp.Msg.GetStatus().GetState()), nil
	case tuiActionNodeStop:
		resp, err := a.client.Service().StopNode(callCtx, client.Request(a.cfg.Token, &ardentsv1.StopNodeRequest{}))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("node stop -> %s", resp.Msg.GetStatus().GetState()), nil
	default:
		return "", fmt.Errorf("unsupported tui action %q", action)
	}
}
