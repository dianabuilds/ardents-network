package browserentry

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	stateSchema      = "ardents-browser-entry-state-v2"
	maximumStateSize = 512
)

// Publisher owns the one local state file from which the Firefox native host
// may learn the Endpoint-created loopback proxy port. Its probe capability
// proves that the port still belongs to the current Endpoint proxy before the
// host returns a value. Its separate proxy credential is disclosed only after
// the same fresh proof and only to answer that loopback proxy's HTTP
// authentication challenge. Neither value is a Service credential or a
// destination selector.
type Publisher struct {
	path            string
	probeCapability [32]byte
	proxyCredential [32]byte

	mu     sync.Mutex
	closed bool
}

type state struct {
	Schema          string `json:"schema"`
	Port            uint16 `json:"port"`
	ProbeCapability string `json:"probeCapability"`
	ProxyCredential string `json:"proxyCredential"`
}

// OpenPublisher claims one absolute state-file location and removes a stale
// regular state file left by a stopped Endpoint. Its parent is local operator
// configuration, not a security authority.
func OpenPublisher(path string) (*Publisher, error) {
	if !filepath.IsAbs(path) || filepath.Base(path) == "." || filepath.Base(path) == string(filepath.Separator) {
		return nil, errors.New("browser Entry state path is invalid")
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("browser Entry state parent is invalid")
	}
	publisher := &Publisher{path: path}
	if _, err := rand.Read(publisher.probeCapability[:]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(publisher.proxyCredential[:]); err != nil {
		return nil, err
	}
	if err := publisher.clearStale(); err != nil {
		return nil, err
	}
	return publisher, nil
}

// Capability returns the one process-local proof expected from this
// Publisher's AlphaProxy probe endpoint. It is never an HTTP proxy password.
func (publisher *Publisher) Capability() [32]byte {
	if publisher == nil {
		return [32]byte{}
	}
	return publisher.probeCapability
}

// ProxyCredential returns the one process-local secret expected by this
// Publisher's AlphaProxy HTTP proxy authentication. The native host may
// disclose its hexadecimal representation only after a fresh liveness probe.
func (publisher *Publisher) ProxyCredential() [32]byte {
	if publisher == nil {
		return [32]byte{}
	}
	return publisher.proxyCredential
}

// Publish atomically makes one active unprivileged loopback port available to
// the native host. It records no Service Name, Target, route, or application
// data.
func (publisher *Publisher) Publish(port uint16) error {
	if publisher == nil || port < 1024 {
		return errors.New("browser Entry loopback port is invalid")
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.closed {
		return errors.New("browser Entry publisher is closed")
	}
	value := state{
		Schema:          stateSchema,
		Port:            port,
		ProbeCapability: base64.RawStdEncoding.EncodeToString(publisher.probeCapability[:]),
		ProxyCredential: base64.RawStdEncoding.EncodeToString(publisher.proxyCredential[:]),
	}
	return writeState(publisher.path, value)
}

// Clear withdraws the active port before a proxy is stopped. A later native
// request consequently fails locally rather than falling back to another path.
func (publisher *Publisher) Clear() error {
	if publisher == nil {
		return nil
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return publisher.clearStale()
}

// Close clears the state file and rejects later publication attempts.
func (publisher *Publisher) Close() error {
	if publisher == nil {
		return nil
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.closed {
		return nil
	}
	publisher.closed = true
	return publisher.clearStale()
}

func (publisher *Publisher) clearStale() error {
	info, err := os.Lstat(publisher.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("browser Entry state file is invalid")
	}
	return os.Remove(publisher.path)
}

func writeState(path string, value state) error {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || len(raw) > maximumStateSize {
		return errors.New("encode browser Entry state")
	}
	temporary := path + ".next"
	if info, statErr := os.Lstat(temporary); statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("browser Entry temporary state is invalid")
		}
		if err := os.Remove(temporary); err != nil {
			return err
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func readState(path string) (state, error) {
	if !filepath.IsAbs(path) {
		return state{}, errors.New("browser Entry state path is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > maximumStateSize {
		return state{}, errors.New("browser Entry state is unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return state{}, err
	}
	decoder := json.NewDecoder(&boundedReader{reader: raw})
	decoder.DisallowUnknownFields()
	var value state
	if err := decoder.Decode(&value); err != nil || value.Schema != stateSchema || value.Port < 1024 {
		return state{}, errors.New("browser Entry state is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return state{}, errors.New("browser Entry state is invalid")
	}
	capability, err := base64.RawStdEncoding.DecodeString(value.ProbeCapability)
	if err != nil || len(capability) != 32 {
		return state{}, errors.New("browser Entry state is invalid")
	}
	credential, err := base64.RawStdEncoding.DecodeString(value.ProxyCredential)
	if err != nil || len(credential) != 32 {
		return state{}, errors.New("browser Entry state is invalid")
	}
	return value, nil
}

// boundedReader makes JSON decoding accept exactly the retained byte slice
// without letting a future caller accidentally substitute an unbounded stream.
type boundedReader struct{ reader []byte }

func (reader *boundedReader) Read(destination []byte) (int, error) {
	if len(reader.reader) == 0 {
		return 0, io.EOF
	}
	count := copy(destination, reader.reader)
	reader.reader = reader.reader[count:]
	return count, nil
}
