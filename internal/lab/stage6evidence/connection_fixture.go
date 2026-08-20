package stage6evidence

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type evidenceEndpoint interface {
	Do(context.Context, serviceconn.Request) (serviceconn.Result, error)
}

type connectionFixture struct {
	now                           time.Time
	network, client, publisher    [32]byte
	admin                         [32]byte
	authority, introduction       ed25519.PrivateKey
	instance                      ed25519.PrivateKey
	credential                    serviceconn.Credential
	clientEndpoint, publisherNode evidenceEndpoint
	publication                   []byte
}

func newConnectionFixture() (connectionFixture, error) {
	value := connectionFixture{now: time.Unix(2_000_000_000, 0), network: [32]byte{41},
		client: [32]byte{42}, publisher: [32]byte{43}, admin: [32]byte{44},
		authority: evidenceKey("connection-authority"), introduction: evidenceKey("connection-introduction"),
		instance: evidenceKey("connection-instance")}
	var authority, introduction, instance [32]byte
	copy(authority[:], value.authority.Public().(ed25519.PublicKey))
	copy(introduction[:], value.introduction.Public().(ed25519.PublicKey))
	copy(instance[:], value.instance.Public().(ed25519.PublicKey))
	credential, err := (serviceconn.Credential{AuthorityPublic: authority, InstancePublic: instance, Generation: 1,
		NotBefore: value.now.Add(-time.Minute).Unix(), NotAfter: value.now.Add(time.Minute).Unix(),
		NetworkID: value.network, Capabilities: 3}).Issue(value.authority)
	if err != nil {
		return value, err
	}
	value.credential = credential
	value.clientEndpoint, err = serviceconn.New(serviceconn.Setup{NetworkID: value.network, BrokerID: [32]byte{45},
		AuthorityPublic: authority[:], IntroductionPublic: introduction[:], ConnectionPrincipal: value.client})
	if err != nil {
		return value, err
	}
	value.publisherNode, err = serviceconn.New(serviceconn.Setup{NetworkID: value.network, BrokerID: [32]byte{46},
		AuthorityPublic: authority[:], IntroductionPublic: introduction[:], ConnectionPrincipal: value.publisher,
		AdministrationPrincipal: value.admin})
	if err != nil {
		return value, err
	}
	admin, err := admitConnection(value.publisherNode, "administration", value.admin, value.now)
	if err != nil {
		return value, err
	}
	published, err := value.publisherNode.Do(context.Background(), serviceconn.Request{Action: "publish",
		Principal: value.admin, Session: admin, Credential: value.credential, InstancePrivate: value.instance,
		IntroductionAcknowledgement: value.acknowledgement(), At: value.now})
	if err != nil {
		return value, err
	}
	value.publication = published.Publication
	return value, nil
}

func admitConnection(endpoint evidenceEndpoint, surface string, principal [32]byte, at time.Time) ([32]byte, error) {
	result, err := endpoint.Do(context.Background(), serviceconn.Request{Action: "admit", Surface: surface,
		Principal: principal, At: at})
	return result.Session, err
}

func (value connectionFixture) acknowledgement() []byte {
	body := make([]byte, 149)
	copy(body[:4], "ASIA")
	body[4] = 1
	copy(body[5:37], value.credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], value.credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(value.credential.NotAfter))
	copy(body[53:85], value.network[:])
	body[85] = 46
	body[117] = 1
	return append(body, ed25519.Sign(value.introduction,
		append([]byte("ardents-h3-introduction-ack-v1\x00"), body...))...)
}
