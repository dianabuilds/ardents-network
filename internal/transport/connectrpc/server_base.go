package connectrpc

import (
	"net/http"

	dataapi "ardents/internal/data/api"
	diagapi "ardents/internal/diagnostics/api"
	discoveryapi "ardents/internal/discovery/api"
	hostingapi "ardents/internal/hosting/api"
	nodeapi "ardents/internal/node/api"
	runtimeconfig "ardents/internal/runtime/config"
	workloadapi "ardents/internal/workload/api"
	"ardents/proto/ardents/v1/ardentsv1connect"

	"connectrpc.com/connect"
)

type Dependencies struct {
	Node          nodeapi.RuntimeService
	Discovery     discoveryapi.Service
	Diagnostics   diagapi.Service
	Workload      workloadapi.Service
	Hosting       hostingapi.Service
	Data          dataapi.Service
	Configuration runtimeconfig.Service
	Audit         diagapi.EventWriter
}

type Server struct {
	node          nodeapi.RuntimeService
	discovery     discoveryapi.Service
	diagnostics   diagapi.Service
	workload      workloadapi.Service
	hosting       hostingapi.Service
	data          dataapi.Service
	configuration runtimeconfig.Service
	auth          AuthConfig
	audit         diagapi.EventWriter
}

func New(deps Dependencies, auth AuthConfig) *Server {
	return &Server{
		node:          deps.Node,
		discovery:     deps.Discovery,
		diagnostics:   deps.Diagnostics,
		workload:      deps.Workload,
		hosting:       deps.Hosting,
		data:          deps.Data,
		configuration: deps.Configuration,
		auth:          auth,
		audit:         deps.Audit,
	}
}

func NewHandler(deps Dependencies, auth AuthConfig) (string, http.Handler, error) {
	if err := auth.validate(); err != nil {
		return "", nil, err
	}
	server := New(deps, auth)
	path, handler := ardentsv1connect.NewArdentsServiceHandler(
		server,
		connect.WithInterceptors(newAccessInterceptor(auth, deps.Audit)),
	)
	return path, handler, nil
}

func (s *Server) call(header http.Header) (callContext, error) {
	return s.auth.callContext(header)
}
