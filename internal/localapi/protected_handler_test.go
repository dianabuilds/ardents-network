package localapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtectedMuxRoutesIdentityAndAllProductCalls(t *testing.T) {
	principalCalls, identityCalls := 0, 0
	principal := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { principalCalls++ })
	identity := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { identityCalls++ })
	handler := newProtectedMux("/ardents.v1.IdentityService/", identity, principal)
	for _, path := range []string{
		"/ardents.v1.NodeService/GetNodeStatus",
		"/ardents.v1.ConfigurationService/GetEffectiveConfiguration",
		"/ardents.v1.NetworkService/GetNetworkStatus",
		"/ardents.v1.DiagnosticsService/GetDiagnostics",
		"/ardents.v1.WorkloadService/GetWorkloadStatus",
		"/ardents.v1.ContentService/GetBlob",
		"/ardents.v1.TransferService/GetTransfer",
		"/ardents.v1.RetentionService/PinBlob",
	} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.Header.Set("Authorization", "ArdentsOperatorSession invalid-but-no-fallback")
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	require.Equal(t, 8, principalCalls)
	require.Zero(t, identityCalls)

	request := httptest.NewRequest(http.MethodPost, "/ardents.v1.IdentityService/BeginSession", nil)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	require.Equal(t, 1, identityCalls)
}
