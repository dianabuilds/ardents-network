package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	applicationauth "ardents/internal/applicationapi/auth"
	applicationcontent "ardents/internal/applicationapi/content"
	contentdomain "ardents/internal/content"
	"ardents/internal/daemon"
	"ardents/internal/localapi"
	localauth "ardents/internal/localapi/auth"
	"ardents/internal/observability"
	"ardents/internal/provision"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := provision.Run(os.Args[2:], os.Stdout); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ardentsd init: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "application-credential" && os.Args[2] == "renew" {
		if err := provision.RenewApplicationCredential(os.Args[3:], os.Stdout); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ardentsd application-credential renew: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := daemon.Run(newLocalAPIHandler, newApplicationAPIHandler, newOperatorSurface); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ardentsd: %v\n", err)
		os.Exit(1)
	}
}

func newApplicationAPIHandler(process daemon.Owners, cfg daemon.ApplicationAPIConfig) (string, http.Handler, error) {
	authorizer, err := applicationauth.New(applicationauth.Config{
		Token: cfg.Token, Subject: cfg.Subject, Capabilities: cfg.Capabilities, ExpiresAt: cfg.ExpiresAt,
		Audit: func(decision applicationauth.Decision) {
			process.Events.RecordEvent("application-interface", "authorization", decision.Subject,
				"Application request authorization "+decision.Outcome, "application.authorization."+decision.Outcome,
				map[string]any{"action": decision.Action})
		},
	})
	if err != nil {
		return "", nil, err
	}
	return applicationcontent.NewHTTPHandler(applicationContentStore{owners: process}, authorizer)
}

type applicationContentStore struct{ owners daemon.Owners }

func (s applicationContentStore) PublishBlob(command contentdomain.PublishBlobCommand) (contentdomain.Blob, error) {
	return s.owners.ContentCommands.PublishBlob(command)
}

func (s applicationContentStore) GetBlob(id string) (contentdomain.Blob, bool) {
	return s.owners.Content.GetBlob(id)
}

func (s applicationContentStore) GetBlobPayload(id string) ([]byte, error) {
	return s.owners.Content.GetBlobPayload(id)
}

func (s applicationContentStore) FetchBlob(ctx context.Context, id string) (contentdomain.Blob, error) {
	return s.owners.Node.FetchBlob(ctx, id)
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
