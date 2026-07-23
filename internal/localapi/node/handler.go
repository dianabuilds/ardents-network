package node

import (
	"context"

	"ardents/internal/daemon"
)

type Runtime interface {
	Start(context.Context) error
	Stop(context.Context) error
	Snapshot() daemon.SystemSnapshot
	Subscribe(context.Context) <-chan daemon.Event
	GetNodeRuntime() daemon.RuntimeSnapshot
	NodeFeatures() daemon.NodeFeaturesSnapshot
}

type RuntimeHandler struct {
	service Runtime
}

func NewHandler(service Runtime) *RuntimeHandler {
	return &RuntimeHandler{service: service}
}
