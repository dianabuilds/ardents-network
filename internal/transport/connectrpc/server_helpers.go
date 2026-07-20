package connectrpc

import (
	"net/http"

	identityapi "ardents/internal/identity/api"

	"connectrpc.com/connect"
)

type callContext = identityapi.CallContext

// respond keeps unary handlers as thin adapters: auth and API error mapping
// are shared here, while each call site still owns its response projection.
func respond[T any](s *Server, header http.Header, invoke func(callContext) (*T, *rpcError)) (*connect.Response[T], error) {
	call, err := s.call(header)
	if err != nil {
		return nil, err
	}
	msg, rpcErr := invoke(call)
	if rpcErr != nil {
		return nil, toConnectError(rpcErr)
	}
	return connect.NewResponse(msg), nil
}
