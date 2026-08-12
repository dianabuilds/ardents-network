package node

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

const nodeProbeHeader = 4 + 1 + 6*32 + 2

type nodeProbePlan struct {
	Schema      string          `json:"schema"`
	NetworkID   string          `json:"network_id"`
	EpochDigest string          `json:"epoch_digest"`
	Nodes       []nodeProbeNode `json:"nodes"`
}

type nodeProbeNode struct {
	Address          string `json:"address"`
	NodeID           string `json:"node_id"`
	AssignmentDigest string `json:"assignment_digest"`
	ServerKeyDigest  string `json:"server_key_digest"`
}

func readNodeProbePlan(path string) (nodeProbePlan, error) {
	raw, err := byteio.ReadFile(path, 32<<10)
	if err != nil {
		return nodeProbePlan{}, err
	}
	var plan nodeProbePlan
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil || plan.Schema != "ardents-h3-node-probe-plan-v1" || len(plan.Nodes) != 2 {
		return nodeProbePlan{}, errors.New("node probe plan is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nodeProbePlan{}, errors.New("node probe plan contains trailing JSON")
	}
	for _, encoded := range []string{plan.NetworkID, plan.EpochDigest, plan.Nodes[0].NodeID,
		plan.Nodes[0].AssignmentDigest, plan.Nodes[0].ServerKeyDigest, plan.Nodes[1].NodeID,
		plan.Nodes[1].AssignmentDigest, plan.Nodes[1].ServerKeyDigest} {
		if _, err := decodeNodeProbeDigest(encoded); err != nil {
			return nodeProbePlan{}, err
		}
	}
	return plan, nil
}

func runNodeProbeMatrix(address string, config *tls.Config, plan nodeProbePlan, node int, deadline time.Time) error {
	request, err := nodeProbeRequest(plan, node, 1)
	if err != nil {
		return err
	}
	if err := exchangeNodeProbe(address, config, request, deadline, true); err != nil {
		return errors.New("authorized role probe did not return the frozen response: " + err.Error())
	}
	if err := exchangeNodeProbe(address, config, request, deadline, false); err != nil {
		return errors.New("role probe replay was not rejected: " + err.Error())
	}
	for offset := 0; offset < 5; offset++ {
		invalid, buildErr := nodeProbeRequest(plan, node, byte(offset+2))
		if buildErr != nil {
			return buildErr
		}
		invalid[5+offset*32] ^= 0xff
		if err := exchangeNodeProbe(address, config, invalid, deadline, false); err != nil {
			return errors.New("role probe accepted a mismatched frozen field: " + err.Error())
		}
	}
	return nil
}

func nodeProbeRequest(plan nodeProbePlan, node int, nonceByte byte) ([]byte, error) {
	network, networkErr := decodeNodeProbeDigest(plan.NetworkID)
	epoch, epochErr := decodeNodeProbeDigest(plan.EpochDigest)
	nodeID, nodeErr := decodeNodeProbeDigest(plan.Nodes[node].NodeID)
	assignment, assignmentErr := decodeNodeProbeDigest(plan.Nodes[node].AssignmentDigest)
	if err := errors.Join(networkErr, epochErr, nodeErr, assignmentErr); err != nil {
		return nil, err
	}
	profile := sha256.Sum256([]byte("h3-role-probe-v1"))
	nonce := [32]byte{31: nonceByte}
	payload := sha256.Sum256([]byte("ardents-h3-node-probe-payload-v1"))
	request := make([]byte, nodeProbeHeader+32)
	copy(request, "ARNP")
	request[4] = 1
	offset := 5
	for _, value := range [][32]byte{network, profile, epoch, nodeID, assignment, nonce} {
		copy(request[offset:offset+32], value[:])
		offset += 32
	}
	binary.BigEndian.PutUint16(request[offset:], 32)
	copy(request[nodeProbeHeader:], payload[:])
	return request, nil
}

func exchangeNodeProbe(address string, config *tls.Config, request []byte, deadline time.Time, expectResponse bool) error {
	connection, err := tls.DialWithDialer(&net.Dialer{Deadline: deadline}, "tcp", address, config)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(deadline)
	if _, err := connection.Write(request); err != nil {
		return err
	}
	response := make([]byte, len(request))
	_, err = io.ReadFull(connection, response)
	if !expectResponse {
		if err == nil {
			return errors.New("unexpected success response")
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			return errors.New("rejection exceeded its deadline")
		}
		return nil
	}
	if err != nil || string(response[:4]) != "ARNS" || response[4] != 1 || !equalNodeProbeResponse(request, response) {
		return errors.New("response bytes do not match the independent expectation")
	}
	return nil
}

func equalNodeProbeResponse(request, response []byte) bool {
	if len(response) != len(request) || string(response[5:nodeProbeHeader]) != string(request[5:nodeProbeHeader]) {
		return false
	}
	digest := sha256.Sum256(request[nodeProbeHeader:])
	return string(response[nodeProbeHeader:]) == string(digest[:])
}

func decodeNodeProbeDigest(encoded string) ([32]byte, error) {
	var digest [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(digest) {
		return digest, errors.New("node probe digest is invalid")
	}
	copy(digest[:], raw)
	return digest, nil
}
