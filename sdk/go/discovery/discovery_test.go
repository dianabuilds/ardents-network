package discovery_test

import (
	"context"
	"reflect"
	"testing"

	"ardents/sdk/go/discovery"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryDomainTypesAreSDKOwned(t *testing.T) {
	require.Equal(t, reflect.TypeFor[discovery.Query]().PkgPath(), "ardents/sdk/go/discovery")
	require.Equal(t, reflect.TypeFor[discovery.Target]().PkgPath(), "ardents/sdk/go/discovery")
	require.NotEqual(t, reflect.TypeFor[discovery.Query](), reflect.TypeFor[applicationv1.ResolveServiceRequest]())
	require.NotEqual(t, reflect.TypeFor[discovery.Target](), reflect.TypeFor[applicationv1.ResolvedServiceTarget]())
	require.Equal(t, discovery.Scheme("https"), discovery.SchemeHTTPS)
	require.Equal(t, discovery.Scheme("http"), discovery.SchemeHTTP)
	require.Equal(t, discovery.Scheme("tcp"), discovery.SchemeTCP)

	var service discovery.Service = serviceStub{}
	targets, err := service.Resolve(context.Background(), discovery.Query{})
	require.NoError(t, err)
	require.Nil(t, targets)
}

type serviceStub struct{}

func (serviceStub) Resolve(context.Context, discovery.Query) ([]discovery.Target, error) {
	return nil, nil
}
