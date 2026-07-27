package localapi

import (
	"net/http"

	runtimeconfig "ardents/internal/config"
	diagapi "ardents/internal/diagnostics"
	identityaccess "ardents/internal/identity/access"
	authorityhandler "ardents/internal/localapi/authority"
	configurationapi "ardents/internal/localapi/configuration"
	contenthandler "ardents/internal/localapi/content"
	diagnosticshandler "ardents/internal/localapi/diagnostics"
	identityhandler "ardents/internal/localapi/identity"
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
	Authority        authorityhandler.Service
}

func NewPrincipalHandler(deps Dependencies, access *identityaccess.Service, node string, peer [32]byte, source identityaccess.SourceKey) (http.Handler, error) {
	server, err := newOperatorServer(deps)
	if err != nil {
		return nil, err
	}
	interceptor, err := identityhandler.NewOperatorInterceptor(identityhandler.OperatorInterceptorConfig{Access: access, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: CanonicalizeOperatorResource})
	if err != nil {
		return nil, err
	}
	option := connect.WithInterceptors(interceptor)
	mux := http.NewServeMux()
	register := func(path string, handler http.Handler) { mux.Handle(path, handler) }
	register(ardentsv1connect.NewNodeServiceHandler(server, option))
	register(ardentsv1connect.NewAuthorityServiceHandler(server, option))
	register(ardentsv1connect.NewConfigurationServiceHandler(server, option))
	register(ardentsv1connect.NewNetworkServiceHandler(server, option))
	register(ardentsv1connect.NewDiagnosticsServiceHandler(server, option))
	register(ardentsv1connect.NewWorkloadServiceHandler(server, option))
	register(ardentsv1connect.NewContentServiceHandler(server, option))
	register(ardentsv1connect.NewTransferServiceHandler(server, option))
	register(ardentsv1connect.NewRetentionServiceHandler(server, option))
	return mux, nil
}

type Server struct {
	*authorityhandler.AuthorityEndpoint
	*contenthandler.QueryHandler
	*configurationapi.Controller
	*diagnosticshandler.Endpoint
	*networkhandler.API
	*nodehandler.RuntimeHandler
	*transferhandler.Handler
	*workloadhandler.Service
}

func newOperatorServer(deps Dependencies) (*Server, error) {
	authorityAPI, err := authorityhandler.NewHandler(deps.Authority)
	if err != nil {
		return nil, err
	}
	contentAPI, err := contenthandler.NewHandler(deps.Content, deps.Data)
	if err != nil {
		return nil, err
	}
	return &Server{
		AuthorityEndpoint: authorityAPI,
		QueryHandler:      contentAPI,
		Controller:        configurationapi.NewHandler(deps.Configuration),
		Endpoint:          diagnosticshandler.NewHandler(deps.Diagnostics),
		API:               networkhandler.NewHandler(deps.Discovery, deps.DiscoveryRecords, deps.Network),
		RuntimeHandler:    nodehandler.NewHandler(deps.Node),
		Handler:           transferhandler.NewHandler(deps.Sources, deps.Transfers, deps.DataFetch),
		Service:           workloadhandler.NewHandler(deps.Workload, deps.Hosting),
	}, nil
}
