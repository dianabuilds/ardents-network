package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func publishCurrent(endpoint *serviceconn.Endpoint, plan endpointPlan, principal [32]byte, at time.Time,
	deadline time.Duration, ready func()) (serviceconn.Result, error) {
	listener, err := listenLocal(plan.AdministrationSocket, deadline)
	if err != nil {
		return serviceconn.Result{}, err
	}
	defer func() { _ = listener.Close(); _ = os.Remove(plan.AdministrationSocket) }()
	if ready != nil {
		ready()
	}
	administrator, err := listener.Accept()
	if err != nil {
		return serviceconn.Result{}, err
	}
	defer administrator.Close()
	operation, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	request, err := applicationipc.ReadControl(operation, administrator, 8)
	if err != nil || string(request) != "publish\n" {
		err = errors.Join(err, errors.New("administration request is malformed, partial, or oversized"))
		return serviceconn.Result{}, err
	}
	credential, private, err := publicationInputs(plan)
	if err != nil {
		return serviceconn.Result{}, err
	}
	defer clear(private)
	session, err := admit(endpoint, principal, "administration", at)
	if err != nil {
		return serviceconn.Result{}, err
	}
	result, err := endpoint.Do(operation, serviceconn.Request{Action: "publish",
		Principal: principal, Session: session, Credential: credential,
		InstancePrivate: private, IntroductionSocket: plan.IntroductionSocket, At: at})
	if err != nil {
		return serviceconn.Result{}, err
	}
	if err := os.WriteFile(plan.PublicationFile, result.Publication, 0o600); err != nil {
		return serviceconn.Result{}, err
	}
	_, err = administrator.Write([]byte("published\n"))
	return result, err
}
func publicationInputs(plan endpointPlan) (serviceconn.Credential, ed25519.PrivateKey, error) {
	var credential serviceconn.Credential
	file, err := os.Open(plan.CredentialFile)
	if err != nil {
		return credential, nil, err
	}
	decoder := json.NewDecoder(io.LimitReader(file, 8<<10))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&credential)
	if err == nil && decoder.Decode(&struct{}{}) != io.EOF {
		err = errors.New("credential file contains multiple JSON values")
	}
	_ = file.Close()
	if err != nil {
		return credential, nil, err
	}
	keyFile, err := os.Open(plan.InstanceKeyFile)
	if err != nil {
		return credential, nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(keyFile, ed25519.PrivateKeySize*2+1))
	closeErr := keyFile.Close()
	if readErr != nil || closeErr != nil || len(raw) != ed25519.PrivateKeySize*2 {
		return credential, nil, errors.New("instance Key file is invalid")
	}
	private := make([]byte, ed25519.PrivateKeySize)
	if _, err := hex.Decode(private, raw); err != nil {
		return credential, nil, err
	}
	return credential, ed25519.PrivateKey(private), nil
}
func listenLocal(path string, deadline time.Duration) (*net.UnixListener, error) {
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	_ = listener.SetDeadline(time.Now().Add(deadline))
	return listener, nil
}
func deliverResult(output io.Writer, result serviceconn.Result) error {
	return applicationipc.Write(output, applicationipc.Result{Class: result.Class, Reason: result.Reason,
		AuthenticatedTarget: result.AuthenticatedTarget, AcceptedBytes: result.AcceptedBytes,
		ReceivedBytes: result.ReceivedBytes})
}
