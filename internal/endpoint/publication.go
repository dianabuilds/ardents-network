package endpoint

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

type connectionEndpoint interface {
	Admit([32]byte, broker.Surface) ([32]byte, error)
	Publish(context.Context, PublicationRequest) (PublicationResult, error)
	Connect(context.Context, OutboundConnectionRequest) (RuntimeResult, error)
	Accept(context.Context, InboundConnectionRequest) (RuntimeResult, error)
}

func publishCurrent(endpoint connectionEndpoint, resources func(string, int) uint32, plan endpointPlan, principal [32]byte, at time.Time,
	deadline time.Duration, ready func()) (RuntimeResult, error) {
	listener, err := listenLocal(plan.AdministrationSocket, deadline)
	if err != nil {
		return RuntimeResult{}, err
	}
	resources("timer", 1)
	defer resources("timer", -1)
	resources("control-file", 1)
	defer resources("control-file", -1)
	defer func() { _ = listener.Close(); _ = os.Remove(plan.AdministrationSocket) }()
	if ready != nil {
		ready()
	}
	administrator, err := listener.Accept()
	if err != nil {
		return RuntimeResult{}, err
	}
	resources("accepted-ipc", 1)
	defer resources("accepted-ipc", -1)
	defer administrator.Close()
	resources("timer", 1)
	defer resources("timer", -1)
	operation, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	resources("timer", 1)
	defer resources("timer", -1)
	request, err := ReadControl(operation, administrator, 8)
	if err != nil || string(request) != "publish\n" {
		err = errors.Join(err, errors.New("administration request is malformed, partial, or oversized"))
		return RuntimeResult{}, err
	}
	credential, private, err := publicationInputs(plan)
	if err != nil {
		return RuntimeResult{}, err
	}
	defer clear(private)
	session, err := admit(endpoint, principal, "administration", at)
	if err != nil {
		return RuntimeResult{}, err
	}
	publicationResult, err := endpoint.Publish(operation, PublicationRequest{
		Principal: principal, Capability: session, Credential: credential,
		InstancePrivate: private, IntroductionSocket: plan.IntroductionSocket, At: at})
	if err != nil {
		return RuntimeResult{}, err
	}
	if err := os.WriteFile(plan.PublicationFile, publicationResult.Record, 0o600); err != nil {
		return RuntimeResult{}, err
	}
	resources("control-file", 1)
	_, err = administrator.Write([]byte("published\n"))
	return RuntimeResult{Class: publicationResult.Class, Reason: publicationResult.Reason,
		AuthenticatedTarget: publicationResult.AuthenticatedTarget,
		Generation:          publicationResult.Generation, IntroductionReceipt: publicationResult.IntroductionReceipt,
		IntroductionAcknowledgement: publicationResult.IntroductionAcknowledgement}, err
}
func publicationInputs(plan endpointPlan) (publication.Credential, ed25519.PrivateKey, error) {
	var credential publication.Credential
	if err := planfile.Decode(plan.CredentialFile, 8<<10, &credential); err != nil {
		return credential, nil, err
	}
	raw, err := planfile.Read(plan.InstanceKeyFile, ed25519.PrivateKeySize*2)
	if err != nil || len(raw) != ed25519.PrivateKeySize*2 {
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
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, errors.Join(err, listener.Close(), os.Remove(path))
	}
	if err := listener.SetDeadline(time.Now().Add(deadline)); err != nil {
		return nil, errors.Join(err, listener.Close(), os.Remove(path))
	}
	return listener, nil
}

func deliverResult(output io.Writer, result RuntimeResult) error {
	return Write(output, Result{Class: result.Class, Reason: result.Reason,
		AuthenticatedTarget: result.AuthenticatedTarget, AcceptedBytes: result.AcceptedBytes,
		ReceivedBytes: result.ReceivedBytes})
}

func admit(endpoint connectionEndpoint, principal [32]byte, surface string, at time.Time) ([32]byte, error) {
	if at.IsZero() {
		return [32]byte{}, errors.New("local admission time is absent")
	}
	return endpoint.Admit(principal, broker.Surface(surface))
}
