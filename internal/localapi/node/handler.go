package node

import (
	"context"

	"ardents/internal/daemon"
	localauth "ardents/internal/localapi/auth"
)

type Runtime interface {
	Start(context.Context) error
	Stop(context.Context) error
	Snapshot() daemon.SystemSnapshot
	Subscribe(context.Context) <-chan daemon.Event
	GetNodeRuntime() daemon.RuntimeSnapshot
	Capabilities() daemon.CapabilitiesSnapshot
}

type RuntimeHandler struct {
	service Runtime
	auth    localauth.Config
}

func NewHandler(service Runtime, auth localauth.Config) *RuntimeHandler {
	return &RuntimeHandler{service: service, auth: auth}
}
