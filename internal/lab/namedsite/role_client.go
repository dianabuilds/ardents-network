package namedsite

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/openpcc/ohttp"
)

func runHTTPClientRole(ctx context.Context, socketPath, nonceHex, evidenceDirectory string) error {
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil || len(nonce) != 32 || hex.EncodeToString(nonce) != nonceHex {
		return errors.New("HTTP Client nonce is invalid")
	}
	connection, err := dialRoleSocketRetry(ctx, socketPath)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := writeConnectRequest(connection); err != nil {
		return err
	}
	response, err := readConnectResponse(connection)
	if err != nil || response.Status != "connected" {
		return errors.New("HTTP Client did not receive an authenticated connection")
	}
	workload, err := executeHTTPWorkload(connection, nonce)
	if err != nil {
		return err
	}
	return writeBoundedJSON(filepath.Join(evidenceDirectory, "http-client.json"), map[string]any{
		"schema_version": "gatec-http-client-evidence/v1", "status": "completed", "target": response.Target,
		"name_generation": response.NameGeneration, "name_revision": response.NameRevision, "instance_generation": response.InstanceGeneration,
		"response_bytes": workload.ResponseBytes, "eof": workload.EOF,
	})
}

func runClientEndpointRole(ctx context.Context, configPath, evidenceDirectory string) error {
	var config clientRoleConfig
	if err := readStrictRoleConfig(configPath, &config); err != nil || config.Schema != roleConfigSchema || config.Target == "" || config.InstanceGeneration == 0 {
		return errors.New("client Endpoint role configuration is invalid")
	}
	keyBytes, err := readRoleFileRetry(ctx, "/gateway/key-config.bin")
	if err != nil {
		return err
	}
	var key ohttp.KeyConfig
	if err := key.UnmarshalBinary(keyBytes); err != nil {
		return err
	}
	transport, err := newOHTTPTransport(key, "http://relay:8080", &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		return err
	}
	clientSocket, routeSocket := "/client/app.sock", "/route/app.sock"
	clientListener, err := listenRoleSocket(ctx, clientSocket)
	if err != nil {
		return err
	}
	defer closeRoleSocket(clientListener, clientSocket)
	routeListener, err := listenRoleSocket(ctx, routeSocket)
	if err != nil {
		return err
	}
	defer closeRoleSocket(routeListener, routeSocket)
	route, err := routeListener.AcceptUnix()
	if err != nil {
		return err
	}
	client, err := clientListener.AcceptUnix()
	if err != nil {
		_ = route.Close()
		return err
	}
	connect := func(operation context.Context, _ connectRequest) (connectionResult, io.ReadWriteCloser, error) {
		now := time.Now()
		nameData, nameNonce, err := resolveMessage(operation, transport, "name", "site.reference", config.RunID, config.NetworkID, now)
		if err != nil {
			return connectionResult{}, nil, connectionFailure{class: "indeterminate"}
		}
		name, err := verifyNameRecord(nameData, config.NamePublic, config.RunID, config.NetworkID, nameNonce, now)
		if err != nil {
			return connectionResult{}, nil, connectionFailure{class: "authentication_failed"}
		}
		descriptorData, descriptorNonce, err := resolveMessage(operation, transport, "reachability", name.Target, config.RunID, config.NetworkID, now)
		if err != nil {
			return connectionResult{}, nil, connectionFailure{class: "route_unavailable"}
		}
		descriptor, err := verifyDescriptor(descriptorData, config.ServicePublic, config.RunID, config.NetworkID, descriptorNonce, name.Target, config.InstanceGeneration, now)
		if err != nil {
			return connectionResult{}, nil, connectionFailure{class: "authentication_failed"}
		}
		return connectionResult{Target: name.Target, NameGeneration: name.NameGeneration, NameRevision: name.NameRevision, InstanceGeneration: descriptor.InstanceGeneration}, route, nil
	}
	err = serveClientConnection(ctx, client, connect)
	if err != nil {
		return err
	}
	return writeBoundedJSON(filepath.Join(evidenceDirectory, "client-endpoint.json"), map[string]any{
		"schema_version": "gatec-client-endpoint-evidence/v1", "status": "completed", "application_interface": applicationInterfaceSchema,
		"resolution_via_relay": true, "direct_fallback": false,
	})
}

func dialRoleSocketRetry(ctx context.Context, path string) (net.Conn, error) {
	for {
		connection, err := dialRoleSocket(ctx, path)
		if err == nil {
			return connection, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func readRoleFileRetry(ctx context.Context, path string) ([]byte, error) {
	for {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 && len(data) <= 64*1024 {
			return data, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
