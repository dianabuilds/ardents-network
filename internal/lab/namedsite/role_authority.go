package namedsite

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

func runAuthorityRole(ctx context.Context, configPath, adminSocket, gatewaySocket, evidenceDirectory string) error {
	var config authorityRoleConfig
	if err := readStrictRoleConfig(configPath, &config); err != nil {
		return err
	}
	fixture, err := config.fixture()
	if err != nil {
		return err
	}
	if filepath.Dir(adminSocket) == filepath.Dir(gatewaySocket) {
		return errors.New("authority UDS principals are not separated")
	}
	adminListener, err := listenRoleSocket(ctx, adminSocket)
	if err != nil {
		return err
	}
	defer closeRoleSocket(adminListener, adminSocket)
	gatewayListener, err := listenRoleSocket(ctx, gatewaySocket)
	if err != nil {
		return err
	}
	defer closeRoleSocket(gatewayListener, gatewaySocket)
	errorsFound := make(chan error, 2)
	go func() { errorsFound <- serveAuthorityAdmin(adminListener, fixture, config.AdminRequests) }()
	go func() { errorsFound <- serveAuthorityGateway(gatewayListener, fixture, 2) }()
	for range 2 {
		if err := <-errorsFound; err != nil {
			return err
		}
	}
	return writeBoundedJSON(filepath.Join(evidenceDirectory, "authority.json"), map[string]any{
		"schema_version": "gatec-authority-evidence/v1", "status": "completed", "private_keys_exported": false,
		"administration_requests": config.AdminRequests, "gateway_signing_requests": 2,
	})
}

func serveAuthorityAdmin(listener *net.UnixListener, fixture *authorityFixture, requests int) error {
	if requests != 1 && requests != 2 {
		return errors.New("authority administration request count is invalid")
	}
	for range requests {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return err
		}
		var request authorityRequest
		decodeErr := json.NewDecoder(connection).Decode(&request)
		var response authorityResponse
		if decodeErr == nil && request.Operation == "issue" {
			credential := fixture.credential
			response = authorityResponse{Target: fixture.target, InstanceGeneration: fixture.instanceGeneration, Credential: &credential}
		} else if decodeErr == nil && request.Operation == "validate" && request.Credential != nil {
			accepted := verifyCredential(*request.Credential, fixture.servicePublic, fixture.runID, fixture.networkID, fixture.target, hex.EncodeToString(fixture.instancePublic), fixture.instanceGeneration, time.Now()) == nil
			response.Accepted = &accepted
		} else {
			_ = connection.Close()
			return errors.New("authority administration request is invalid")
		}
		encodeErr := json.NewEncoder(connection).Encode(response)
		_ = connection.Close()
		if encodeErr != nil {
			return encodeErr
		}
	}
	return nil
}

func serveAuthorityGateway(listener *net.UnixListener, fixture *authorityFixture, requests int) error {
	for range requests {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return err
		}
		var request authorityRequest
		decodeErr := json.NewDecoder(connection).Decode(&request)
		nonce, nonceErr := hex.DecodeString(request.Nonce)
		if decodeErr != nil || nonceErr != nil || len(nonce) != 32 || request.Operation != "sign" {
			_ = connection.Close()
			return errors.New("authority signing request is invalid")
		}
		deadline := time.Unix(request.DeadlineUnix, 0)
		var record []byte
		if request.Type == "name" {
			record, err = fixture.signedNameRecord(nonce, deadline)
		} else if request.Type == "reachability" {
			record, err = fixture.signedDescriptor(nonce, deadline)
		} else {
			err = errors.New("authority signing type is invalid")
		}
		if err == nil {
			err = json.NewEncoder(connection).Encode(authorityResponse{Record: record})
		}
		_ = connection.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func runAdministrationRole(ctx context.Context, configPath, authoritySocket, evidenceDirectory string) error {
	var config administrationRoleConfig
	if err := readStrictRoleConfig(configPath, &config); err != nil || config.Schema != roleConfigSchema {
		return errors.New("administration role configuration is invalid")
	}
	connection, err := dialRoleSocketRetry(ctx, authoritySocket)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := json.NewEncoder(connection).Encode(authorityRequest{Operation: "issue"}); err != nil {
		return err
	}
	var response authorityResponse
	if err := json.NewDecoder(connection).Decode(&response); err != nil || response.Target == "" || response.InstanceGeneration == 0 || response.Credential == nil {
		return errors.New("administration did not receive an issued Instance handle")
	}
	_ = connection.Close()
	supersededAttempted := config.SupersededCredential != nil
	supersededRejected := false
	if config.SupersededCredential != nil {
		validation, err := dialRoleSocketRetry(ctx, authoritySocket)
		if err != nil {
			return err
		}
		if err := json.NewEncoder(validation).Encode(authorityRequest{Operation: "validate", Credential: config.SupersededCredential}); err != nil {
			_ = validation.Close()
			return err
		}
		var checked authorityResponse
		decodeErr := json.NewDecoder(validation).Decode(&checked)
		_ = validation.Close()
		supersededRejected = decodeErr == nil && checked.Accepted != nil && !*checked.Accepted
		if !supersededRejected {
			return errors.New("superseded Instance publication was accepted")
		}
	}
	return writeAtomicBoundedJSON(filepath.Join(evidenceDirectory, "publication.json"), map[string]any{
		"schema_version": "gatec-publication/v1", "status": "published", "target": response.Target,
		"instance_generation": response.InstanceGeneration, "authority_received": false, "instance_private_key_received": false,
		"superseded_publication_attempted": supersededAttempted, "superseded_publication_rejected": supersededRejected,
	})
}

func listenRoleSocket(ctx context.Context, path string) (*net.UnixListener, error) {
	parent := filepath.Dir(path)
	if info, err := os.Lstat(parent); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("role socket parent is not a real directory")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := listener.SetDeadline(deadline); err != nil {
			closeRoleSocket(listener, path)
			return nil, err
		}
	}
	if err := os.Chmod(path, 0o666); err != nil {
		closeRoleSocket(listener, path)
		return nil, err
	}
	return listener, nil
}

func closeRoleSocket(listener *net.UnixListener, path string) {
	_ = listener.Close()
	removeOwnedSocket(path)
}

func dialRoleSocket(ctx context.Context, path string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", path)
}
