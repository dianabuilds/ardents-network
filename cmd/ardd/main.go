package main

import (
	"net/http"

	"ardents/internal/daemon"
	"ardents/internal/localapi"
	localauth "ardents/internal/localapi/auth"
	"ardents/internal/observability"
)

func main() {
	daemon.Run(newLocalAPIHandler, newOperatorSurface)
}

func newLocalAPIHandler(process daemon.Owners, cfg daemon.LocalAPIConfig) (string, http.Handler, error) {
	runtime := process.Node
	if len(cfg.Capabilities) == 0 {
		cfg.Capabilities = localauth.OperatorActions()
	}
	auth := localauth.Config{Token: cfg.Token, SubjectID: cfg.SubjectID, Capabilities: cfg.Capabilities,
		ExpiresAt: cfg.ExpiresAt, TargetNode: cfg.TargetNode, TargetPrincipal: cfg.TargetID}
	return localapi.NewHandler(localapi.Dependencies{
		Node:             runtime,
		Discovery:        runtime,
		DiscoveryRecords: process.DiscoveryCommands,
		Network:          runtime,
		Diagnostics:      process.Diagnostics,
		Workload:         process.Workloads,
		Hosting:          process.Hosting,
		Content:          process.Content,
		Sources:          process.Content,
		Transfers:        process.Transfers,
		Data:             process.ContentCommands,
		DataFetch:        runtime,
		Configuration:    runtime,
		Audit:            process.Events,
	}, auth)
}

func newOperatorSurface(process daemon.Owners, token string) (daemon.OperatorSurface, error) {
	return observability.NewSurface(observability.Dependencies{
		Runtime: process.Node, Diagnostics: process.Diagnostics, Workloads: process.Workloads,
		Hosting: process.Hosting, Data: process.Content, Transfers: process.Transfers,
	}, token)
}
