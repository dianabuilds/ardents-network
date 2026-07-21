package localapi

import (
	"net/http"

	runtimeconfig "ardents/internal/config"
	diagapi "ardents/internal/diagnostics"
	localauth "ardents/internal/localapi/auth"
	configurationapi "ardents/internal/localapi/configuration"
	contenthandler "ardents/internal/localapi/content"
	diagnosticshandler "ardents/internal/localapi/diagnostics"
	networkhandler "ardents/internal/localapi/network"
	nodehandler "ardents/internal/localapi/node"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	transferhandler "ardents/internal/localapi/transfer"
	workloadhandler "ardents/internal/localapi/workload"

	"connectrpc.com/connect"
)

type Dependencies struct {
	Node             nodehandler.Runtime
	Discovery        networkhandler.Discovery
	DiscoveryRecords networkhandler.Records
	Network          networkhandler.Status
	Diagnostics      diagapi.Service
	Workload         workloadhandler.Runtime
	Hosting          workloadhandler.Hosting
	Content          contenthandler.Reader
	Sources          transferhandler.Sources
	Transfers        transferhandler.Records
	Data             contenthandler.Commands
	DataFetch        transferhandler.Fetcher
	Configuration    runtimeconfig.Service
	Audit            diagapi.EventWriter
}

type Server struct {
	*contenthandler.QueryHandler
	*configurationapi.Controller
	*diagnosticshandler.Endpoint
	*networkhandler.API
	*nodehandler.RuntimeHandler
	*transferhandler.Handler
	*workloadhandler.Service
}

func New(deps Dependencies, auth localauth.Config) (*Server, error) {
	contentAPI, err := contenthandler.NewHandler(deps.Content, deps.Data, auth)
	if err != nil {
		return nil, err
	}
	return &Server{
		QueryHandler:   contentAPI,
		Controller:     configurationapi.NewHandler(deps.Configuration, auth),
		Endpoint:       diagnosticshandler.NewHandler(deps.Diagnostics, auth),
		API:            networkhandler.NewHandler(deps.Discovery, deps.DiscoveryRecords, deps.Network, auth),
		RuntimeHandler: nodehandler.NewHandler(deps.Node, auth),
		Handler:        transferhandler.NewHandler(deps.Sources, deps.Transfers, deps.DataFetch, auth),
		Service:        workloadhandler.NewHandler(deps.Workload, deps.Hosting, auth),
	}, nil
}

func NewHandler(deps Dependencies, auth localauth.Config) (string, http.Handler, error) {
	if err := auth.Validate(); err != nil {
		return "", nil, err
	}
	server, err := New(deps, auth)
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	interceptor := connect.WithInterceptors(newAccessInterceptor(auth, deps.Audit))
	register := func(path string, handler http.Handler) { mux.Handle(path, handler) }
	register(ardentsv1connect.NewNodeServiceHandler(server, interceptor))
	register(ardentsv1connect.NewConfigurationServiceHandler(server, interceptor))
	register(ardentsv1connect.NewNetworkServiceHandler(server, interceptor))
	register(ardentsv1connect.NewWorkloadServiceHandler(server, interceptor))
	register(ardentsv1connect.NewContentServiceHandler(server, interceptor))
	register(ardentsv1connect.NewTransferServiceHandler(server, interceptor))
	register(ardentsv1connect.NewRetentionServiceHandler(server, interceptor))
	register(ardentsv1connect.NewDiagnosticsServiceHandler(server, interceptor))
	// The inner mux owns the exact bounded-service routes. The outer daemon
	// server must mount it at the root; net/http does not treat
	// "/ardents.v1." as a prefix pattern.
	return "/", mux, nil
}
