package endpoint

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func resolutionRelayHandler(gatewayURL string, client *http.Client) func(net.Conn) error {
	return func(connection net.Conn) error {
		setup, err := readInitiatorFixtureRecord(connection, 10)
		if err == nil {
			err = writeInitiatorFixtureReady(connection, setup, 11)
		}
		envelope := []byte(nil)
		if err == nil {
			envelope, err = readInitiatorFixtureEnvelope(connection, 12)
		}
		response, chunked := []byte(nil), false
		if err == nil {
			response, chunked, err = forwardInitiatorFixtureOHTTP(context.Background(), gatewayURL, client, envelope)
		}
		if err == nil {
			framing := route.ResolutionOHTTPResponse
			if chunked {
				framing = route.ResolutionOHTTPChunkedResponse
			}
			err = writeInitiatorFixtureResponse(connection, 13, framing, response)
		}
		return err
	}
}

func credentialRelayHandler(issuerURL string, client *http.Client) func(net.Conn) error {
	return func(connection net.Conn) error {
		setup, err := readInitiatorFixtureRecord(connection, 14)
		if err == nil {
			err = writeInitiatorFixtureReady(connection, setup, 15)
		}
		envelope := []byte(nil)
		if err == nil {
			envelope, err = readInitiatorFixtureEnvelope(connection, 16)
		}
		response := []byte(nil)
		if err == nil {
			response, _, err = forwardInitiatorFixtureOHTTP(context.Background(), issuerURL, client, envelope)
		}
		if err == nil {
			err = writeInitiatorFixtureResponse(connection, 17, route.CredentialOHTTPResponse, response)
		}
		return err
	}
}

const (
	initiatorFixtureMagic                    = "ardents-interactive-route-v2\x00"
	initiatorFixtureOHTTPRequestMediaType    = "message/ohttp-req"
	initiatorFixtureOHTTPResponseMediaType   = "message/ohttp-res"
	initiatorFixtureOHTTPChunkedResponseType = "message/ohttp-chunked-res"
)

func readInitiatorFixtureRecord(reader io.Reader, kind byte) ([]byte, error) {
	header := make([]byte, len(initiatorFixtureMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if string(header[:len(initiatorFixtureMagic)]) != initiatorFixtureMagic {
		return nil, errors.New("Initiator fixture record magic is invalid")
	}
	length := int(binary.BigEndian.Uint16(header[len(initiatorFixtureMagic):]))
	if length == 0 || length > 16<<10 {
		return nil, errors.New("Initiator fixture record length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	if len(body) < 4 || binary.BigEndian.Uint16(body[:2]) != 2 || body[2] != kind || int(body[3])+4 > len(body) ||
		string(body[4:4+int(body[3])]) != route.Profile {
		return nil, errors.New("Initiator fixture record header is invalid")
	}
	return append(header, body...), nil
}

func writeInitiatorFixtureReady(writer io.Writer, setup []byte, readyKind byte) error {
	ready := append([]byte(nil), setup...)
	ready[len(initiatorFixtureMagic)+2+2] = readyKind
	_, err := writer.Write(ready)
	return err
}

func readInitiatorFixtureEnvelope(reader io.Reader, kind byte) ([]byte, error) {
	raw, err := readInitiatorFixtureRecord(reader, kind)
	if err != nil {
		return nil, err
	}
	body := raw[len(initiatorFixtureMagic)+2:]
	offset := 4 + int(body[3])
	if offset+2 > len(body) {
		return nil, errors.New("Initiator fixture envelope is incomplete")
	}
	length := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if length == 0 || offset+length != len(body) {
		return nil, errors.New("Initiator fixture envelope length is invalid")
	}
	return append([]byte(nil), body[offset:]...), nil
}

func writeInitiatorFixtureResponse(writer io.Writer, kind, framing byte, payload []byte) error {
	if len(payload) == 0 || len(payload) > 16<<10 {
		return errors.New("Initiator fixture response is invalid")
	}
	body := make([]byte, 0, 4+len(route.Profile)+3+len(payload))
	body = binary.BigEndian.AppendUint16(body, 2)
	body = append(body, kind, byte(len(route.Profile)))
	body = append(body, route.Profile...)
	body = append(body, framing)
	body = binary.BigEndian.AppendUint16(body, uint16(len(payload)))
	body = append(body, payload...)
	raw := append([]byte(initiatorFixtureMagic), 0, 0)
	binary.BigEndian.PutUint16(raw[len(initiatorFixtureMagic):], uint16(len(body)))
	raw = append(raw, body...)
	_, err := writer.Write(raw)
	return err
}

func forwardInitiatorFixtureOHTTP(ctx context.Context, origin string, client *http.Client, envelope []byte) ([]byte, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, origin, bytes.NewReader(envelope))
	if err != nil {
		return nil, false, err
	}
	request.Header.Set("Content-Type", initiatorFixtureOHTTPRequestMediaType)
	response, err := client.Do(request)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	contentType := response.Header.Get("Content-Type")
	if err != nil || response.StatusCode != http.StatusOK || len(body) == 0 || len(body) > 16<<10 ||
		(contentType != initiatorFixtureOHTTPResponseMediaType && contentType != initiatorFixtureOHTTPChunkedResponseType) {
		return nil, false, errors.New("Initiator fixture OHTTP response is invalid")
	}
	return body, contentType == initiatorFixtureOHTTPChunkedResponseType, nil
}
