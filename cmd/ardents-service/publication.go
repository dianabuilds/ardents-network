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

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func publishCurrent(endpoint *serviceconn.Endpoint, plan endpointPlan, at time.Time, deadline time.Duration, ready func()) error {
	listener, err := listenLocal(plan.AdministrationSocket, deadline)
	if err != nil {
		return err
	}
	defer closeLocal(listener, plan.AdministrationSocket)
	if ready != nil {
		ready()
	}
	administrator, err := listener.Accept()
	if err != nil {
		return err
	}
	defer administrator.Close()
	request := make([]byte, 8)
	if _, err := io.ReadFull(administrator, request); err != nil || string(request) != "publish\n" {
		return errors.New("administration request is malformed")
	}
	credential, private, acknowledgement, err := publicationInputs(plan)
	if err != nil {
		return err
	}
	defer erase(private)
	session, err := admit(endpoint, endpointPrincipal(plan.AdministrationPrincipal), "administration", at)
	if err != nil {
		return err
	}
	result, err := endpoint.Do(context.Background(), serviceconn.Request{Action: "publish",
		Principal: endpointPrincipal(plan.AdministrationPrincipal), Session: session, Credential: credential,
		InstancePrivate: private, IntroductionAcknowledgement: acknowledgement, At: at})
	if err != nil {
		return err
	}
	if err := os.WriteFile(plan.PublicationFile, result.Publication, 0o600); err != nil {
		return err
	}
	_, err = administrator.Write([]byte("published\n"))
	return err
}

func publicationInputs(plan endpointPlan) (serviceconn.Credential, ed25519.PrivateKey, [32]byte, error) {
	var credential serviceconn.Credential
	file, err := os.Open(plan.CredentialFile)
	if err != nil {
		return credential, nil, [32]byte{}, err
	}
	decoder := json.NewDecoder(io.LimitReader(file, 8<<10))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&credential)
	if err == nil && decoder.Decode(&struct{}{}) != io.EOF {
		err = errors.New("credential file contains multiple JSON values")
	}
	_ = file.Close()
	if err != nil {
		return credential, nil, [32]byte{}, err
	}
	keyFile, err := os.Open(plan.InstanceKeyFile)
	if err != nil {
		return credential, nil, [32]byte{}, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(keyFile, ed25519.PrivateKeySize*2+1))
	closeErr := keyFile.Close()
	if readErr != nil || closeErr != nil || len(raw) != ed25519.PrivateKeySize*2 {
		return credential, nil, [32]byte{}, errors.New("instance Key file is invalid")
	}
	private := make([]byte, ed25519.PrivateKeySize)
	if _, err := hex.Decode(private, raw); err != nil {
		return credential, nil, [32]byte{}, err
	}
	var acknowledgement [32]byte
	err = fixedHex(plan.IntroductionAcknowledgement, acknowledgement[:])
	return credential, ed25519.PrivateKey(private), acknowledgement, err
}

func endpointPrincipal(encoded string) [32]byte {
	var value [32]byte
	_ = fixedHex(encoded, value[:])
	return value
}

func listenLocal(path string, deadline time.Duration) (*net.UnixListener, error) {
	if path == "" {
		return nil, errors.New("local socket path is empty")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	_ = listener.SetDeadline(time.Now().Add(deadline))
	return listener, nil
}

func closeLocal(listener *net.UnixListener, path string) {
	_ = listener.Close()
	_ = os.Remove(path)
}

func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
