//go:build ignore

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cloudflare/circl/hpke"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/openpcc/ohttp"
)

var (
	syntheticName       = []byte("reference.ard")
	syntheticDescriptor = []byte("synthetic-descriptor")
	syntheticTarget     = identifier("synthetic-target")
)

func runIssuer(cell string, state selection) error {
	key, adapter, err := newOHTTPGateway()
	if err != nil {
		return err
	}
	public := signer().Public().(ed25519.PublicKey)
	result := issuerResult{}
	var once sync.Once
	var server *http.Server
	inner := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		payload, err := io.ReadAll(io.LimitReader(request.Body, requestSize+1))
		if err == nil {
			payload, err = unpad(payload, requestSize)
		}
		result.PlaintextForbidden = bytes.Contains(payload, syntheticName) || bytes.Contains(payload, syntheticTarget[:]) || bytes.Contains(payload, syntheticDescriptor)
		input, valid := decodeRequest(payload)
		if err != nil || !valid || !matchesSelection(input, state) {
			writeResponse(writer, issuanceResponse{Class: rejected})
			return
		}
		grantInput := route.TransitGrant{IssuerID: sha256.Sum256(public), GrantID: randomIdentifier(), NetworkID: input.Network,
			Digest: input.Digest, AttachmentID: input.Attachment, TransitNodeID: input.Introduction, ClientKeyDigest: input.ClientKey,
			Epoch: input.Epoch, TransitRole: input.Role, NotAfter: input.NotAfter}
		if cell == "wrong-key" {
			grantInput.ClientKeyDigest = identifier("issuer-wrong-client-key")
		}
		grant, issueErr := route.IssueTransitGrant(grantInput, signer())
		if issueErr != nil {
			writeResponse(writer, issuanceResponse{Class: rejected})
			return
		}
		result.Accepted = true
		writeResponse(writer, issuanceResponse{Class: accepted, Grant: grant})
	})
	middleware := ohttp.Middleware(adapter, inner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server = &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		result.RemoteAddress = request.RemoteAddr
		result.AdmissionForwarded = request.Header.Get("X-Entry-Admission") != ""
		middleware.ServeHTTP(writer, request)
		once.Do(func() { go closeSoon(server) })
	})}
	fmt.Printf("READY %s %s\n", listener.Addr().String(), encode(key))
	err = server.Serve(listener)
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	print(result)
	return nil
}

func runInitiator(issuer string, proof []byte, expected int) error {
	if issuer == "" || len(proof) == 0 || expected < 1 || expected > 2 {
		return errors.New("Initiator input is invalid")
	}
	result := initiatorResult{}
	used, seen := false, 0
	var mu sync.Mutex
	var server *http.Server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server = &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		seen++
		result.EndpointAddress = request.RemoteAddr
		admitted := bytes.Equal(decode(request.Header.Get("X-Entry-Admission")), proof) && !used
		if admitted {
			used = true
		} else if used {
			result.ReplayRefused = true
		}
		last := seen == expected
		mu.Unlock()
		if !admitted {
			http.Error(writer, "admission refused", http.StatusForbidden)
			if last {
				go closeSoon(server)
			}
			return
		}
		body, readErr := io.ReadAll(io.LimitReader(request.Body, 8193))
		if readErr != nil || len(body) == 0 || len(body) > 8192 {
			http.Error(writer, "opaque envelope refused", http.StatusBadRequest)
			return
		}
		forward, err := http.NewRequest(http.MethodPost, issuer+"/credential", bytes.NewReader(body))
		if err == nil {
			forward.Header.Set("Content-Type", ohttp.RequestMediaType)
			response, requestErr := (&http.Client{Timeout: 10 * time.Second}).Do(forward)
			if requestErr == nil {
				defer response.Body.Close()
				writer.Header().Set("Content-Type", response.Header.Get("Content-Type"))
				forwarded, copyErr := io.Copy(writer, io.LimitReader(response.Body, 8193))
				if copyErr == nil && forwarded > 0 && response.StatusCode == http.StatusOK {
					mu.Lock()
					result.Forwarded++
					mu.Unlock()
				} else {
					http.Error(writer, "issuer refused", http.StatusBadGateway)
				}
			}
		}
		if last {
			go closeSoon(server)
		}
	})}
	fmt.Printf("READY %s\n", listener.Addr().String())
	err = server.Serve(listener)
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	print(result)
	return nil
}

func runEndpoint(cell, initiator string, keyConfig, proof []byte, state selection) error {
	if initiator == "" || len(keyConfig) == 0 || len(proof) == 0 {
		return errors.New("Endpoint input is invalid")
	}
	fmt.Println("READY endpoint")
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	public := private.Public().(ed25519.PublicKey)
	input := issuanceRequest{Network: state.Network, Digest: state.Digest, Introduction: state.Introduction, Attachment: randomIdentifier(),
		ClientKey: sha256.Sum256(public), Epoch: state.Epoch, Role: route.IntroductionRole, NotAfter: state.NotAfter}
	switch cell {
	case "node-substitution":
		input.Introduction = identifier("substituted-introduction")
	case "expiry-substitution":
		input.NotAfter = input.NotAfter.Add(time.Second)
	}
	first, err := exchange(initiator, keyConfig, proof, input, cell == "target")
	if err != nil {
		return err
	}
	result := endpointResult{Accepted: first.Class == accepted, Refused: first.Class == rejected}
	if first.Class == accepted {
		grant, verifyErr := route.VerifyTransitGrant(first.Grant, signer().Public().(ed25519.PublicKey))
		result.GrantExact = verifyErr == nil && grant.NetworkID == input.Network && grant.Digest == input.Digest && grant.Epoch == input.Epoch &&
			grant.TransitNodeID == input.Introduction && grant.TransitRole == input.Role && grant.AttachmentID == input.Attachment &&
			grant.ClientKeyDigest == input.ClientKey && grant.NotAfter.Equal(input.NotAfter)
	}
	if cell == "replay-admission" {
		second, secondErr := exchange(initiator, keyConfig, proof, input, false)
		result.ReplayRefused = secondErr == nil && second.Class == rejected
	}
	print(result)
	return nil
}

func exchange(initiator string, config, proof []byte, input issuanceRequest, target bool) (issuanceResponse, error) {
	key := ohttp.KeyConfig{}
	if err := key.UnmarshalBinary(config); err != nil {
		return issuanceResponse{}, err
	}
	transport, err := ohttp.NewTransport(key, "https://issuer.invalid/credential")
	if err != nil {
		return issuanceResponse{}, err
	}
	payload, err := pad(encodeRequest(input, target), requestSize)
	if err != nil {
		return issuanceResponse{}, err
	}
	inner, err := http.NewRequest(http.MethodPost, "http://ohttp.invalid/credential", bytes.NewReader(payload))
	if err != nil {
		return issuanceResponse{}, err
	}
	outer, decapsulator, err := transport.Encapsulate(inner)
	if err != nil {
		return issuanceResponse{}, err
	}
	defer outer.Body.Close()
	body, err := io.ReadAll(io.LimitReader(outer.Body, 8193))
	if err != nil || len(body) == 0 {
		return issuanceResponse{}, errors.New("OHTTP request is unavailable")
	}
	request, err := http.NewRequest(http.MethodPost, initiator+"/credential", bytes.NewReader(body))
	if err != nil {
		return issuanceResponse{}, err
	}
	request.Header.Set("Content-Type", ohttp.RequestMediaType)
	request.Header.Set("X-Entry-Admission", encode(proof))
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return issuanceResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusForbidden {
		return issuanceResponse{Class: rejected}, nil
	}
	if response.StatusCode != http.StatusOK {
		return issuanceResponse{}, errors.New("Initiator exchange is unavailable")
	}
	wrapped, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(wrapped) == 0 {
		return issuanceResponse{}, errors.New("OHTTP response is unavailable")
	}
	plain, err := decapsulator.Decapsulate(context.Background(), &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{ohttp.ResponseMediaType}}, Body: io.NopCloser(bytes.NewReader(wrapped)), Request: outer})
	if err != nil {
		return issuanceResponse{}, err
	}
	defer plain.Body.Close()
	decoded, err := io.ReadAll(io.LimitReader(plain.Body, responseSize+1))
	if err != nil {
		return issuanceResponse{}, err
	}
	return decodeResponse(decoded)
}

func newOHTTPGateway() ([]byte, *ohttp.Gateway, error) {
	kem := hpke.KEM_P256_HKDF_SHA256
	public, secret, err := kem.Scheme().GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	config := ohttp.KeyConfig{KeyID: 1, KemID: kem, PublicKey: public,
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}}}
	raw, err := config.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	gateway, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secret, KeyConfig: config})
	return raw, gateway, err
}

func encodeRequest(value issuanceRequest, target bool) []byte {
	out := binary.BigEndian.AppendUint16(nil, 1)
	out = append(out, requestKind)
	for _, field := range [][32]byte{value.Network, value.Digest, value.Introduction, value.Attachment, value.ClientKey} {
		out = append(out, field[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, value.Epoch)
	out = append(out, value.Role)
	out = binary.BigEndian.AppendUint64(out, uint64(value.NotAfter.Unix()))
	if target {
		out = append(out, syntheticTarget[:]...)
	}
	return out
}

func decodeRequest(raw []byte) (issuanceRequest, bool) {
	const length = 2 + 1 + 5*32 + 8 + 1 + 8
	if len(raw) != length || binary.BigEndian.Uint16(raw) != 1 || raw[2] != requestKind {
		return issuanceRequest{}, false
	}
	value := issuanceRequest{Epoch: binary.BigEndian.Uint64(raw[163:]), Role: raw[171], NotAfter: time.Unix(int64(binary.BigEndian.Uint64(raw[172:])), 0).UTC()}
	for index, destination := range []*[32]byte{&value.Network, &value.Digest, &value.Introduction, &value.Attachment, &value.ClientKey} {
		copy(destination[:], raw[3+32*index:35+32*index])
	}
	return value, value.Network != [32]byte{} && value.Digest != [32]byte{} && value.Introduction != [32]byte{} && value.Attachment != [32]byte{} &&
		value.ClientKey != [32]byte{} && value.Epoch != 0 && value.Role == route.IntroductionRole && value.NotAfter.Unix() > 0
}

func writeResponse(writer http.ResponseWriter, value issuanceResponse) {
	payload := binary.BigEndian.AppendUint16(nil, 1)
	payload = append(payload, responseKind, value.Class)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(value.Grant)))
	payload = append(payload, value.Grant...)
	payload, err := pad(payload, responseSize)
	if err == nil {
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write(payload)
	}
}

func decodeResponse(raw []byte) (issuanceResponse, error) {
	raw, err := unpad(raw, responseSize)
	if err != nil || len(raw) < 6 || binary.BigEndian.Uint16(raw) != 1 || raw[2] != responseKind || (raw[3] != accepted && raw[3] != rejected) {
		return issuanceResponse{}, errors.New("issuer response is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[4:]))
	if len(raw) != 6+length || (raw[3] == accepted) != (length > 0) {
		return issuanceResponse{}, errors.New("issuer response is invalid")
	}
	return issuanceResponse{Class: raw[3], Grant: append([]byte(nil), raw[6:]...)}, nil
}

func matchesSelection(input issuanceRequest, state selection) bool {
	return input.Network == state.Network && input.Digest == state.Digest && input.Introduction == state.Introduction && input.Epoch == state.Epoch &&
		input.Role == route.IntroductionRole && input.NotAfter.Equal(state.NotAfter)
}

func pad(raw []byte, size int) ([]byte, error) {
	if len(raw) == 0 || len(raw) > size-2 {
		return nil, errors.New("fixed message capacity exceeded")
	}
	out := make([]byte, size)
	binary.BigEndian.PutUint16(out, uint16(len(raw)))
	copy(out[2:], raw)
	return out, nil
}

func unpad(raw []byte, size int) ([]byte, error) {
	if len(raw) != size {
		return nil, errors.New("fixed message capacity is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw))
	if length == 0 || length > size-2 {
		return nil, errors.New("fixed message length is invalid")
	}
	for _, value := range raw[2+length:] {
		if value != 0 {
			return nil, errors.New("fixed message padding is invalid")
		}
	}
	return append([]byte(nil), raw[2:2+length]...), nil
}

func assertCell(cell string, endpoint endpointResult, initiator initiatorResult, issuer issuerResult) error {
	if issuer.AdmissionForwarded || issuer.RemoteAddress == "" || issuer.RemoteAddress == initiator.EndpointAddress {
		return errors.New("issuer disclosure boundary failed")
	}
	switch cell {
	case "exact":
		if issuer.PlaintextForbidden || !endpoint.Accepted || !endpoint.GrantExact || !issuer.Accepted || initiator.Forwarded != 1 {
			return errors.New("exact credential exchange failed")
		}
	case "target":
		if !issuer.PlaintextForbidden || !endpoint.Refused || issuer.Accepted || initiator.Forwarded != 1 {
			return errors.New("issuer Target refusal failed")
		}
	case "node-substitution", "expiry-substitution":
		if issuer.PlaintextForbidden || !endpoint.Refused || issuer.Accepted || initiator.Forwarded != 1 {
			return errors.New("issuer substitution refusal failed")
		}
	case "wrong-key":
		if issuer.PlaintextForbidden || !endpoint.Accepted || endpoint.GrantExact || !issuer.Accepted || initiator.Forwarded != 1 {
			return errors.New("Endpoint key binding refusal failed")
		}
	case "replay-admission":
		if issuer.PlaintextForbidden || !endpoint.Accepted || !endpoint.GrantExact || !endpoint.ReplayRefused || !initiator.ReplayRefused || initiator.Forwarded != 1 || !issuer.Accepted {
			return errors.New("Initiator admission replay refusal failed")
		}
	}
	return nil
}

func selected(deadline time.Time) selection {
	return selection{Network: identifier("network"), Digest: identifier("epoch-digest"), Introduction: identifier("introduction"), Epoch: 7, NotAfter: deadline}
}

func signer() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("r-118-experiment-state-authority"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func identifier(label string) [32]byte { return sha256.Sum256([]byte(label)) }
func randomIdentifier() [32]byte       { var value [32]byte; _, _ = rand.Read(value[:]); return value }
func closeSoon(server *http.Server)    { time.Sleep(20 * time.Millisecond); _ = server.Close() }
