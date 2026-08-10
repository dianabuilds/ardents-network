package siteexperiment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
	"github.com/dianabuilds/ardents-network/internal/nativecircuit"
)

func runRouteAttempt(ctx context.Context, identity experimentrun.Layout, fixture *authorityFixture, sequence int, applicationImage, toolImage, referenceImage, retained string) (runErr error) {
	_, repositoryRoot, runDirectory, _, err := identity.OwnedPaths(true, true)
	if err != nil {
		return err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	reference, serviceSocket, err := startReferenceApplication(ctx, repositoryRoot, runDirectory, referenceImage, hex.EncodeToString(nonce), sequence)
	if err != nil {
		if reference != nil {
			_ = reference.close()
		}
		return err
	}
	defer func() { runErr = errors.Join(runErr, reference.close()) }()
	userDirectory := filepath.Join(runDirectory, "route-user")
	if err := os.MkdirAll(userDirectory, 0o700); err != nil {
		return err
	}
	userSocket := filepath.Join(userDirectory, "app.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: userSocket, Net: "unix"})
	if err != nil {
		return err
	}
	defer listener.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}
	_ = listener.SetDeadline(deadline)
	root, chain, key := fixture.routeIdentity()
	routeDone := make(chan error, 1)
	go func() {
		_, err := nativecircuit.RunAttached(ctx, identity, applicationImage, toolImage, userSocket, serviceSocket, root, chain, key)
		routeDone <- err
	}()
	route, err := listener.AcceptUnix()
	if err != nil {
		return errors.Join(err, <-routeDone)
	}
	resolver, err := openActiveResolver(fixture, time.Now())
	if err != nil {
		_ = route.Close()
		return err
	}
	defer resolver.close()
	operation, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := exerciseApplicationInterface(operation, route, resolver, fixture, nonce); err != nil {
		return errors.Join(err, <-routeDone)
	}
	if err := <-routeDone; err != nil {
		return err
	}
	return retainAttemptEvidence(retained, sequence)
}

func exerciseApplicationInterface(ctx context.Context, route net.Conn, resolver *activeResolver, fixture *authorityFixture, workloadNonce []byte) error {
	client, gateway := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- serveClientConnection(ctx, gateway, func(operation context.Context, _ connectRequest) (connectionResult, io.ReadWriteCloser, error) {
			name, descriptor, err := resolver.resolve(operation, time.Now())
			if err != nil {
				return connectionResult{}, nil, err
			}
			return connectionResult{Target: name.Target, NameGeneration: name.NameGeneration, NameRevision: name.NameRevision, InstanceGeneration: descriptor.InstanceGeneration}, route, nil
		})
	}()
	if err := writeConnectRequest(client); err != nil {
		return err
	}
	response, err := readConnectResponse(client)
	if err != nil || response.Status != "connected" || response.Target != fixture.target || response.InstanceGeneration != fixture.instanceGeneration {
		return errors.New("application Interface did not return the authenticated binding")
	}
	_, workloadErr := executeHTTPWorkload(client, workloadNonce)
	_ = client.Close()
	return errors.Join(workloadErr, <-done)
}

func retainAttemptEvidence(retained string, sequence int) error {
	source := filepath.Join(retained, "native-run.json")
	data, err := os.ReadFile(source)
	if err != nil || len(data) > 4*1024*1024 {
		return errors.New("native attached evidence is missing or unbounded")
	}
	directory := filepath.Join(retained, "attempts")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, formatAttempt(sequence)+".json"), data, 0o600)
}

func formatAttempt(sequence int) string {
	const digits = "000"
	value := []byte(digits)
	for index := len(value) - 1; index >= 0; index-- {
		value[index] = byte('0' + sequence%10)
		sequence /= 10
	}
	return string(value)
}
