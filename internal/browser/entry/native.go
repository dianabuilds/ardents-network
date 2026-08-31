package browserentry

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	maximumNativeMessageSize = 4096
	// OperationLoopbackProxyPort asks for the current fresh-proved loopback
	// proxy port. It does not disclose the proxy credential.
	OperationLoopbackProxyPort = "loopback-proxy-port"
	// OperationLoopbackProxyAuthentication asks for the exact same fresh-proved
	// proxy port and its one-process HTTP Basic password.
	OperationLoopbackProxyAuthentication = "loopback-proxy-authentication"
	// ProxyUsername is fixed because the random password, not a user name,
	// authenticates the local Browser Entry handoff.
	ProxyUsername = "ardents"
	// ProbePath is the loopback-only AlphaProxy liveness route used solely by
	// the native host before it returns a port to Firefox.
	ProbePath = "/.well-known/ardents-browser-entry/v1"
	// ProbeHeader carries the current Endpoint-local random probe capability.
	// The native host sends it and the live proxy must return the exact value.
	ProbeHeader = "X-Ardents-Browser-Entry"
)

// ServeNativeHost processes one Firefox native-messaging request. It can
// return the current verified loopback proxy port or, after the same fresh
// proof, the bounded Basic-authentication answer for that port. It cannot
// expose a name, target, route, Service credential, or arbitrary URL.
func ServeNativeHost(input io.Reader, output io.Writer, statePath string) error {
	requestRaw, err := readNativeMessage(input)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(&boundedReader{reader: requestRaw})
	decoder.DisallowUnknownFields()
	var request struct {
		Operation string `json:"operation"`
	}
	if err := decoder.Decode(&request); err != nil || (request.Operation != OperationLoopbackProxyPort && request.Operation != OperationLoopbackProxyAuthentication) {
		return errors.New("browser Entry native request is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("browser Entry native request is invalid")
	}
	state, err := readState(statePath)
	if err != nil {
		return err
	}
	capability, err := base64.RawStdEncoding.DecodeString(state.ProbeCapability)
	if err != nil || len(capability) != 32 || !probeLoopback(state.Port, capability) {
		return errors.New("browser Entry loopback proxy is unavailable")
	}
	response, err := nativeResponse(request.Operation, state)
	if err != nil {
		return err
	}
	return writeNativeMessage(output, response)
}

func nativeResponse(operation string, state state) ([]byte, error) {
	if operation == OperationLoopbackProxyPort {
		return json.Marshal(struct {
			Port uint16 `json:"port"`
		}{Port: state.Port})
	}
	credential, err := base64.RawStdEncoding.DecodeString(state.ProxyCredential)
	if err != nil || len(credential) != 32 {
		return nil, errors.New("browser Entry state is invalid")
	}
	return json.Marshal(struct {
		Port     uint16 `json:"port"`
		Password string `json:"password"`
	}{Port: state.Port, Password: hex.EncodeToString(credential)})
}

func readNativeMessage(input io.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(input, length[:]); err != nil {
		return nil, errors.New("browser Entry native request is incomplete")
	}
	size := binary.LittleEndian.Uint32(length[:])
	if size < 2 || size > maximumNativeMessageSize {
		return nil, errors.New("browser Entry native request is invalid")
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(input, body); err != nil {
		return nil, errors.New("browser Entry native request is incomplete")
	}
	return body, nil
}

func writeNativeMessage(output io.Writer, body []byte) error {
	if len(body) == 0 || len(body) > maximumNativeMessageSize {
		return errors.New("browser Entry native response is invalid")
	}
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(body)))
	if _, err := output.Write(length[:]); err != nil {
		return err
	}
	_, err := output.Write(body)
	return err
}

func probeLoopback(port uint16, capability []byte) bool {
	if port < 1024 || len(capability) != 32 {
		return false
	}
	address := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return false
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprintf(connection, "GET %s HTTP/1.1\r\nHost: %s\r\n%s: %s\r\nConnection: close\r\n\r\n",
		ProbePath, address, ProbeHeader, hex.EncodeToString(capability)); err != nil {
		return false
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodGet})
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode == http.StatusNoContent && response.Header.Get(ProbeHeader) == hex.EncodeToString(capability)
}
